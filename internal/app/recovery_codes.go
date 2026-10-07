package app

import (
	"crypto/rand"
	"encoding/hex"
	"foodflow/internal/core"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strings"
	"time"
)

func (a *App) recoveryStatus(c *gin.Context) {
	var count int
	var expiry *time.Time
	if err := a.DB.QueryRow(c, "SELECT count(*),max(r.expires_at) FROM recovery_codes r JOIN users u ON u.id=r.user_id WHERE r.user_id=$1 AND r.expires_at>now() AND r.auth_version=u.auth_version AND u.merged_into IS NULL", uid(c)).Scan(&count, &expiry); err != nil {
		fail(c, 503, "恢复码状态暂不可用")
		return
	}
	c.JSON(200, gin.H{"remaining": count, "expires_at": expiry})
}
func (a *App) issueRecoveryCodes(c *gin.Context) {
	var in struct {
		Password string `json:"password"`
		Confirm  bool   `json:"confirm"`
	}
	if !input(c, &in) {
		return
	}
	if !in.Confirm || len(in.Password) > 72 {
		fail(c, 400, "请确认重新生成恢复码并提供当前密码")
		return
	}
	tx, err := a.DB.Begin(c)
	if err != nil {
		fail(c, 503, "恢复码暂不可用")
		return
	}
	defer tx.Rollback(c)
	var hash string
	var version int64
	if tx.QueryRow(c, "SELECT password_hash,auth_version FROM users WHERE id=$1 AND merged_into IS NULL FOR UPDATE", uid(c)).Scan(&hash, &version) != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		fail(c, 400, "当前密码不可用于生成恢复码")
		return
	}
	if _, err = tx.Exec(c, "DELETE FROM recovery_codes WHERE user_id=$1", uid(c)); err != nil {
		fail(c, 503, "恢复码暂不可用")
		return
	}
	var expiry time.Time
	if err = tx.QueryRow(c, "SELECT now()+interval '180 days'").Scan(&expiry); err != nil {
		fail(c, 503, "恢复码暂不可用")
		return
	}
	codes := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		var raw [16]byte
		if _, err = rand.Read(raw[:]); err != nil {
			fail(c, 503, "恢复码暂不可用")
			return
		}
		value := strings.ToUpper(hex.EncodeToString(raw[:]))
		if _, err = tx.Exec(c, "INSERT INTO recovery_codes(user_id,code_hash,expires_at,auth_version) VALUES($1,$2,$3,$4)", uid(c), core.Hash(value), expiry, version); err != nil {
			fail(c, 503, "恢复码暂不可用")
			return
		}
		codes = append(codes, value[:8]+"-"+value[8:16]+"-"+value[16:24]+"-"+value[24:])
	}
	if tx.Commit(c) != nil {
		fail(c, 503, "恢复码暂不可用")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"codes": codes, "expires_at": expiry})
}
func normalizeRecoveryCode(value string) (string, bool) {
	value = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), "-", ""))
	if len(value) != 32 {
		return "", false
	}
	_, err := hex.DecodeString(value)
	return value, err == nil
}
func (a *App) recoverWithCode(c *gin.Context) {
	var in struct {
		Account  string `json:"account"`
		Code     string `json:"code"`
		Password string `json:"password"`
		Confirm  bool   `json:"confirm"`
	}
	if !input(c, &in) {
		return
	}
	code, ok := normalizeRecoveryCode(in.Code)
	if !ok || !in.Confirm || len(in.Password) < 8 || len(in.Password) > 72 {
		fail(c, 400, "账号或恢复码不可用于重置；请确认重置并提供 8 至 72 字节新密码")
		return
	}
	// Always perform the password work before looking up the account.
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(c, 503, "重置暂不可用")
		return
	}
	account := strings.ToLower(strings.TrimSpace(in.Account))
	email, phone := "", ""
	if strings.Contains(account, "@") {
		email = account
	} else {
		phone, _ = normalizePhone(account)
	}
	tx, err := a.DB.Begin(c)
	if err != nil {
		fail(c, 503, "重置暂不可用")
		return
	}
	defer tx.Rollback(c)
	var id string
	invalid := func() { fail(c, http.StatusBadRequest, "账号或恢复码不可用于重置") }
	if tx.QueryRow(c, "SELECT id FROM users WHERE merged_into IS NULL AND (($1<>'' AND email=$1) OR ($2<>'' AND phone=$2)) FOR UPDATE", email, phone).Scan(&id) != nil {
		invalid()
		return
	}
	var valid bool
	if tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM recovery_codes r JOIN users u ON u.id=r.user_id WHERE r.user_id=$1 AND r.code_hash=$2 AND r.expires_at>now() AND r.auth_version=u.auth_version)", id, core.Hash(code)).Scan(&valid) != nil || !valid {
		invalid()
		return
	}
	if _, err = tx.Exec(c, "UPDATE users SET password_hash=$1,auth_version=auth_version+1 WHERE id=$2", string(hash), id); err == nil {
		_, err = tx.Exec(c, "DELETE FROM sessions WHERE user_id=$1", id)
	}
	if err == nil {
		_, err = tx.Exec(c, "DELETE FROM recovery_codes WHERE user_id=$1", id)
	}
	if err != nil || tx.Commit(c) != nil {
		fail(c, 503, "重置暂不可用")
		return
	}
	c.Status(204)
}
