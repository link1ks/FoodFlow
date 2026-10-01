package app

import (
	"foodflow/internal/core"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"strings"
)

func (a *App) resetPassword(c *gin.Context) {
	var in struct {
		Phone, Code, Password string
		Challenge             string `json:"challenge_id"`
		Confirm               bool   `json:"confirm"`
	}
	if !input(c, &in) {
		return
	}
	phone, ok := normalizePhone(in.Phone)
	if !ok || !in.Confirm || len(in.Password) < 8 || len(in.Password) > 72 {
		fail(c, 400, "须确认重置，并提供已验证手机号及 8 至 72 字节新密码")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(c, 503, "重置暂不可用")
		return
	}
	tx, err := a.DB.Begin(c)
	if err != nil {
		fail(c, 503, "重置暂不可用")
		return
	}
	defer tx.Rollback(c)
	var id string
	if tx.QueryRow(c, "SELECT id FROM users WHERE phone=$1 AND phone_verified AND merged_into IS NULL FOR UPDATE", phone).Scan(&id) != nil {
		fail(c, 400, "手机号或验证码不可用于重置")
		return
	}
	if err = a.verifyCode(c, tx, in.Challenge, phone, "reset", "", in.Code); err != nil {
		fail(c, 400, err.Error())
		return
	}
	if _, err = tx.Exec(c, "UPDATE users SET password_hash=$1,auth_version=auth_version+1 WHERE id=$2", string(hash), id); err == nil {
		_, err = tx.Exec(c, "DELETE FROM sessions WHERE user_id=$1", id)
	}
	if err != nil || tx.Commit(c) != nil {
		fail(c, 503, "重置暂不可用")
		return
	}
	c.Status(204)
}

// The first trial supports complementary email-only + verified-phone-only
// accounts. Keeping one email and one phone avoids silently dropping identifiers
// or introducing ambiguous alias login. All historical actor IDs are retained.
func (a *App) mergeAccount(c *gin.Context) {
	var in struct {
		Account        string `json:"source_account"`
		SourcePassword string `json:"source_password"`
		Password       string `json:"password"`
		Code           string `json:"code"`
		Challenge      string `json:"challenge_id"`
		Confirm        bool   `json:"confirm"`
	}
	if !input(c, &in) {
		return
	}
	if !in.Confirm || len(in.Password) > 72 || len(in.SourcePassword) > 72 {
		fail(c, 400, "须明确确认合并，并提供两个账号的密码")
		return
	}
	email, phone := "", ""
	if strings.Contains(in.Account, "@") {
		email = strings.ToLower(strings.TrimSpace(in.Account))
	} else {
		var ok bool
		phone, ok = normalizePhone(in.Account)
		if !ok {
			fail(c, 400, "来源账号无效")
			return
		}
	}
	tx, err := a.DB.Begin(c)
	if err != nil {
		fail(c, 503, "合并暂不可用")
		return
	}
	defer tx.Rollback(c)
	var source string
	if tx.QueryRow(c, "SELECT id FROM users WHERE merged_into IS NULL AND (($1<>'' AND email=$1) OR ($2<>'' AND phone=$2))", email, phone).Scan(&source) != nil || source == uid(c) {
		fail(c, 400, "来源账号或密码不正确")
		return
	}
	// Lock both users in stable order; concurrent merges/replacements serialize.
	rows, err := tx.Query(c, "SELECT id,COALESCE(email,''),COALESCE(phone,''),phone_verified,password_hash FROM users WHERE id::text=ANY($1::text[]) AND merged_into IS NULL ORDER BY id FOR UPDATE", []string{source, uid(c)})
	if err != nil {
		fail(c, 503, "合并暂不可用")
		return
	}
	type identity struct {
		email, phone, hash string
		verified           bool
	}
	accounts := map[string]identity{}
	for rows.Next() {
		var id string
		var v identity
		if err = rows.Scan(&id, &v.email, &v.phone, &v.verified, &v.hash); err != nil {
			break
		}
		accounts[id] = v
	}
	rows.Close()
	if err != nil || rows.Err() != nil || len(accounts) != 2 {
		fail(c, 409, "账号状态已变更，请重新登录")
		return
	}
	target, origin := accounts[uid(c)], accounts[source]
	if bcrypt.CompareHashAndPassword([]byte(target.hash), []byte(in.Password)) != nil || bcrypt.CompareHashAndPassword([]byte(origin.hash), []byte(in.SourcePassword)) != nil {
		fail(c, 401, "账号或密码不正确")
		return
	}
	combinedEmail, combinedPhone := "", ""
	switch {
	case target.email != "" && target.phone == "" && origin.email == "" && origin.phone != "" && origin.verified:
		combinedEmail, combinedPhone = target.email, origin.phone
	case target.email == "" && target.phone != "" && target.verified && origin.email != "" && origin.phone == "":
		combinedEmail, combinedPhone = origin.email, target.phone
	default:
		fail(c, 409, "当前仅支持邮箱账号与已验证手机号账号合并；账号信息不能冲突")
		return
	}
	if err = a.verifyCode(c, tx, in.Challenge, combinedPhone, "merge", uid(c), in.Code); err != nil {
		fail(c, 400, err.Error())
		return
	}
	// Release identities on the retained source tombstone, then transfer them.
	if _, err = tx.Exec(c, "UPDATE users SET email=NULL,phone=NULL,phone_verified=false,merged_into=$1,auth_version=auth_version+1 WHERE id=$2", uid(c), source); err == nil {
		_, err = tx.Exec(c, "UPDATE users SET email=$1,phone=$2,phone_verified=true,auth_version=auth_version+1 WHERE id=$3", combinedEmail, combinedPhone, uid(c))
	}
	if err == nil {
		_, err = tx.Exec(c, `INSERT INTO members(household_id,user_id,role)
 SELECT household_id,$1,role FROM members WHERE user_id=$2
 ON CONFLICT(household_id,user_id) DO UPDATE SET role=CASE
 WHEN members.role='owner' OR EXCLUDED.role='owner' THEN 'owner'
 WHEN members.role='editor' OR EXCLUDED.role='editor' THEN 'editor' ELSE 'viewer' END`, uid(c), source)
	}
	if err == nil {
		_, err = tx.Exec(c, "UPDATE households SET owner_id=$1 WHERE owner_id=$2", uid(c), source)
	}
	if err == nil {
		_, err = tx.Exec(c, "DELETE FROM members WHERE user_id=$1", source)
	}
	if err == nil {
		_, err = tx.Exec(c, "DELETE FROM sessions WHERE user_id::text=ANY($1::text[])", []string{uid(c), source})
	}
	if err == nil {
		_, err = tx.Exec(c, "UPDATE jobs SET cancel_requested=true,status='cancelled',lease_token=NULL,lease_until=NULL,updated_at=now() WHERE created_by=$1 AND status IN ('queued','running')", source)
	}
	if err == nil {
		_, err = tx.Exec(c, "INSERT INTO account_merges(id,source_id,target_id) VALUES($1,$2,$3)", core.ID(), source, uid(c))
	}
	if err != nil || tx.Commit(c) != nil {
		fail(c, 503, "合并暂不可用，未完成的操作已回滚")
		return
	}
	c.Status(204)
}
