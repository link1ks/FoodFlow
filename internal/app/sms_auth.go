package app

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"foodflow/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
	"math/big"
	"strings"
)

func (a *App) codeHash(id, code string) string {
	m := hmac.New(sha256.New, a.jwtKey)
	m.Write([]byte("sms:" + id + ":" + code))
	return hex.EncodeToString(m.Sum(nil))
}
func (a *App) sendSMS(c *gin.Context)     { a.sendCode(c, false) }
func (a *App) sendBindSMS(c *gin.Context) { a.sendCode(c, true) }
func (a *App) sendCode(c *gin.Context, binding bool) {
	if a.sms == nil {
		fail(c, 503, "短信服务未配置，请使用密码登录")
		return
	}
	var x struct{ Phone, Purpose string }
	if !input(c, &x) {
		return
	}
	phone, ok := normalizePhone(x.Phone)
	if !ok {
		fail(c, 400, "手机号格式不正确")
		return
	}
	actor := ""
	if binding {
		x.Purpose = "bind"
		actor = uid(c)
	} else if x.Purpose != "register" && x.Purpose != "login" {
		fail(c, 400, "验证码用途无效")
		return
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		fail(c, 503, "验证码生成失败")
		return
	}
	code := fmt.Sprintf("%06d", n.Int64())
	id := core.ID()
	tx, err := a.DB.Begin(c)
	if err != nil {
		fail(c, 503, "短信服务暂不可用")
		return
	}
	defer tx.Rollback(c)
	// A single transaction reserves all budgets; failed/ambiguous sends also count.
	for _, bucket := range []struct {
		key      string
		limit    int
		cooldown bool
	}{{"global", 200, false}, {"ip:" + core.Hash(c.ClientIP()), 20, false}, {phone, 5, true}} {
		tag, e := tx.Exec(c, `INSERT INTO sms_send_limits(phone,last_sent,day,count) VALUES($1,now(),(now() AT TIME ZONE 'Asia/Shanghai')::date,1)
 ON CONFLICT(phone) DO UPDATE SET last_sent=now(),day=EXCLUDED.day,count=CASE WHEN sms_send_limits.day=EXCLUDED.day THEN sms_send_limits.count+1 ELSE 1 END
 WHERE (sms_send_limits.day<>EXCLUDED.day OR sms_send_limits.count<$2) AND (NOT $3 OR sms_send_limits.last_sent<now()-interval '60 seconds')`, bucket.key, bucket.limit, bucket.cooldown)
		if e != nil {
			fail(c, 503, "短信服务暂不可用")
			return
		}
		if tag.RowsAffected() == 0 {
			fail(c, 429, "发送过于频繁或今日额度已用完，请稍后再试")
			return
		}
	}
	_, err = tx.Exec(c, `UPDATE sms_challenges SET used_at=now() WHERE phone=$1 AND purpose=$2 AND actor=$3 AND used_at IS NULL`, phone, x.Purpose, actor)
	if err != nil {
		fail(c, 503, "短信服务暂不可用")
		return
	}
	_, err = tx.Exec(c, `INSERT INTO sms_challenges(id,phone,purpose,actor,code_hash,state,expires_at) VALUES($1,$2,$3,$4,$5,'pending',now()+interval '5 minutes')`, id, phone, x.Purpose, actor, a.codeHash(id, code))
	if err != nil || tx.Commit(c) != nil {
		fail(c, 503, "短信服务暂不可用")
		return
	}
	if err = a.sms.Send(c, phone, code); err != nil {
		_, _ = a.DB.Exec(c, "UPDATE sms_challenges SET state='failed' WHERE id=$1", id)
		fail(c, 502, "短信发送未成功确认，请稍后重试")
		return
	}
	if _, err = a.DB.Exec(c, "UPDATE sms_challenges SET state='sent' WHERE id=$1", id); err != nil {
		fail(c, 503, "验证码状态保存失败，请稍后重试")
		return
	}
	c.JSON(200, gin.H{"challenge_id": id, "expires_in": 300, "retry_after": 60})
}

// Wrong guesses commit the attempt counter; successful verification and business
// mutation share the same transaction, preventing replay and concurrent reuse.
func (a *App) verifyCode(c *gin.Context, tx pgx.Tx, id, phone, purpose, actor, code string) error {
	var hash string
	var valid bool
	err := tx.QueryRow(c, `SELECT code_hash,state='sent' AND used_at IS NULL AND expires_at>now() AND attempts<5 FROM sms_challenges WHERE id::text=$1 AND phone=$2 AND purpose=$3 AND actor=$4 FOR UPDATE`, id, phone, purpose, actor).Scan(&hash, &valid)
	if err != nil || !valid {
		return errors.New("验证码无效、已使用或已过期")
	}
	if len(code) != 6 || !hmac.Equal([]byte(hash), []byte(a.codeHash(id, code))) {
		_, err = tx.Exec(c, "UPDATE sms_challenges SET attempts=attempts+1 WHERE id::text=$1", id)
		if err == nil {
			_ = tx.Commit(c)
		}
		return errors.New("验证码不正确")
	}
	_, err = tx.Exec(c, "UPDATE sms_challenges SET used_at=now() WHERE id::text=$1", id)
	return err
}

func (a *App) smsLogin(c *gin.Context) {
	var x struct {
		Phone, Code string
		Challenge   string `json:"challenge_id"`
	}
	if !input(c, &x) {
		return
	}
	phone, ok := normalizePhone(x.Phone)
	if !ok {
		fail(c, 401, "手机号或验证码不正确")
		return
	}
	tx, err := a.DB.Begin(c)
	if err != nil {
		fail(c, 503, "登录暂不可用")
		return
	}
	defer tx.Rollback(c)
	if err = a.verifyCode(c, tx, x.Challenge, phone, "login", "", x.Code); err != nil {
		fail(c, 401, err.Error())
		return
	}
	var id string
	err = tx.QueryRow(c, "SELECT id FROM users WHERE phone=$1 AND phone_verified=true", phone).Scan(&id)
	if err != nil {
		_ = tx.Commit(c)
		fail(c, 401, "请先注册，或使用密码登录后验证绑定手机号")
		return
	}
	if tx.Commit(c) != nil {
		fail(c, 503, "登录暂不可用")
		return
	}
	a.issue(c, id)
}

func (a *App) bindPhone(c *gin.Context) {
	var x struct {
		Phone, Code, Password string
		Challenge             string `json:"challenge_id"`
	}
	if !input(c, &x) {
		return
	}
	phone, ok := normalizePhone(x.Phone)
	if !ok || len(x.Password) > 72 {
		fail(c, 400, "手机号或密码格式不正确")
		return
	}
	tx, err := a.DB.Begin(c)
	if err != nil {
		fail(c, 503, "绑定暂不可用")
		return
	}
	defer tx.Rollback(c)
	var hash, old string
	if tx.QueryRow(c, "SELECT password_hash,COALESCE(phone,'') FROM users WHERE id=$1 FOR UPDATE", uid(c)).Scan(&hash, &old) != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(x.Password)) != nil {
		fail(c, 401, "当前密码不正确")
		return
	}
	if old != "" && old != phone {
		fail(c, 409, "已绑定其他手机号，暂不支持在线换绑")
		return
	}
	if err = a.verifyCode(c, tx, x.Challenge, phone, "bind", uid(c), x.Code); err != nil {
		fail(c, 400, err.Error())
		return
	}
	if _, err = tx.Exec(c, "UPDATE users SET phone=$1,phone_verified=true WHERE id=$2", phone, uid(c)); err != nil {
		fail(c, 409, "手机号不可绑定，请使用原账号登录")
		return
	}
	if tx.Commit(c) != nil {
		fail(c, 503, "绑定暂不可用")
		return
	}
	c.Status(204)
}

func resolveAccount(account, email, phone string) (string, string, bool) {
	if strings.TrimSpace(account) != "" {
		if email != "" || phone != "" {
			return "", "", false
		}
		if strings.Contains(account, "@") {
			email = account
		} else {
			phone = account
		}
	}
	return email, phone, true
}
