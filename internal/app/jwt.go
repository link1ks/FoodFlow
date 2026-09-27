package app

import (
	"fmt"
	"foodflow/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"strings"
	"time"
)

func (a *App) signSession(id string) (string, time.Time, error) {
	now := time.Now()
	expiry := now.Add(7 * 24 * time.Hour)
	claims := jwt.RegisteredClaims{Issuer: "foodflow", Subject: id, Audience: jwt.ClaimStrings{"foodflow-web"}, ExpiresAt: jwt.NewNumericDate(expiry), IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now), ID: core.ID()}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.jwtKey)
	return value, expiry, err
}

func (a *App) verifySession(value string) (string, error) {
	claims := new(jwt.RegisteredClaims)
	t, err := jwt.ParseWithClaims(value, claims, func(*jwt.Token) (any, error) { return a.jwtKey, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("foodflow"), jwt.WithAudience("foodflow-web"), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !t.Valid || claims.Subject == "" || claims.ID == "" {
		return "", fmt.Errorf("invalid token")
	}
	return claims.Subject, nil
}

func normalizePhone(value string) (string, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "+86")
	if len(value) != 11 || value[0] != '1' || value[1] < '3' || value[1] > '9' {
		return "", false
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	return "+86" + value, true
}

func (a *App) limitAuth(c *gin.Context) {
	var attempts int
	err := a.DB.QueryRow(c, `INSERT INTO auth_rate_limits(ip_hash,window_start,attempts) VALUES($1,now(),1)
 ON CONFLICT(ip_hash) DO UPDATE SET attempts=CASE WHEN auth_rate_limits.window_start<now()-interval '15 minutes' THEN 1 ELSE auth_rate_limits.attempts+1 END,
 window_start=CASE WHEN auth_rate_limits.window_start<now()-interval '15 minutes' THEN now() ELSE auth_rate_limits.window_start END RETURNING attempts`, core.Hash(c.ClientIP())).Scan(&attempts)
	if err != nil {
		fail(c, 503, "登录服务暂不可用")
		return
	}
	if attempts > 60 {
		c.Header("Retry-After", "900")
		fail(c, 429, "尝试次数过多，请15分钟后再试")
		return
	}
	c.Next()
}
