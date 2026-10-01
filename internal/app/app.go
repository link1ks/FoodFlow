package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"foodflow/internal/cache"
	"foodflow/internal/core"
	"foodflow/internal/dbgen"
	"foodflow/internal/sms"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type App struct {
	DB           *pgxpool.Pool
	jwtKey       []byte
	sms          sms.Sender
	catalogCache *cache.Catalog
}

var requestCount atomic.Uint64

func New(db *pgxpool.Pool) *App {
	key := os.Getenv("JWT_SECRET")
	if len(key) < 32 {
		panic("JWT_SECRET must contain at least 32 bytes")
	}
	a := &App{DB: db, jwtKey: []byte(key), sms: sms.FromEnv()}
	if url := os.Getenv("REDIS_URL"); url != "" {
		var err error
		a.catalogCache, err = cache.New(url)
		if err != nil {
			panic("invalid REDIS_URL")
		}
	}
	return a
}
func (a *App) Close() {
	if a.catalogCache != nil {
		_ = a.catalogCache.Client.Close()
	}
}
func fail(c *gin.Context, code int, msg string) { c.AbortWithStatusJSON(code, gin.H{"error": msg}) }
func input(c *gin.Context, v any) bool {
	if e := c.ShouldBindJSON(v); e != nil {
		fail(c, 400, e.Error())
		return false
	}
	return true
}
func uid(c *gin.Context) string { return c.GetString("user_id") }
func hid(c *gin.Context) string { return c.Param("household") }
func (a *App) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		start := time.Now()
		requestCount.Add(1)
		c.Next()
		slog.Info("http_request", "method", c.Request.Method, "path", c.FullPath(), "status", c.Writer.Status(), "duration_ms", time.Since(start).Milliseconds())
	})
	r.Use(func(c *gin.Context) {
		origin := os.Getenv("APP_ORIGIN")
		if origin == "" {
			origin = "http://localhost:5173"
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
		}
	})
	r.GET("/health/live", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/health/ready", func(c *gin.Context) {
		if e := a.DB.Ping(c); e != nil {
			fail(c, 503, "database unavailable")
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.GET("/metrics", func(c *gin.Context) {
		metrics := fmt.Sprintf("# HELP foodflow_http_requests_total HTTP requests handled.\n# TYPE foodflow_http_requests_total counter\nfoodflow_http_requests_total %d\n", requestCount.Load())
		if a.catalogCache != nil {
			metrics += fmt.Sprintf("foodflow_catalog_cache_hits_total %d\nfoodflow_catalog_cache_misses_total %d\nfoodflow_catalog_cache_errors_total %d\n", a.catalogCache.Hits.Load(), a.catalogCache.Misses.Load(), a.catalogCache.Errors.Load())
		}
		c.Data(200, "text/plain; version=0.0.4", []byte(metrics))
	})
	r.POST("/api/register", a.limitAuth, a.register)
	r.POST("/api/login", a.limitAuth, a.login)
	r.GET("/api/auth/capabilities", func(c *gin.Context) { c.JSON(200, gin.H{"sms_enabled": a.sms != nil}) })
	r.POST("/api/auth/sms/code", a.limitAuth, a.sendSMS)
	r.POST("/api/auth/sms/login", a.limitAuth, a.smsLogin)
	r.POST("/api/auth/password/reset", a.limitAuth, a.resetPassword)
	v := r.Group("/api")
	v.Use(a.auth)
	v.POST("/logout", a.logout)
	v.GET("/me", a.me)
	v.POST("/me/phone/code", a.limitAuth, a.sendBindSMS)
	v.POST("/me/phone", a.limitAuth, a.bindPhone)
	v.POST("/me/merge", a.limitAuth, a.mergeAccount)
	v.PATCH("/me", a.updateMe)
	v.GET("/households", a.households)
	v.POST("/households", a.createHousehold)
	v.POST("/invites/accept", a.acceptInvite)
	v.GET("/recipes", a.recipes)
	v.GET("/ingredient-catalog", a.ingredientCatalog)
	v.GET("/capabilities", func(c *gin.Context) {
		c.JSON(200, gin.H{"vision_enabled": visionConfigured(), "model_enabled": os.Getenv("MODEL_ENDPOINT") != "" && os.Getenv("MODEL_API_KEY") != "" && os.Getenv("MODEL_NAME") != "", "insights_enabled": os.Getenv("INSIGHTS_URL") != "" && len(os.Getenv("INSIGHTS_SERVICE_TOKEN")) >= 32})
	})
	h := v.Group("/households/:household")
	h.Use(a.membership)
	h.GET("", a.household)
	h.PATCH("", a.updateHousehold)
	h.GET("/members", a.members)
	h.POST("/invites", a.invite)
	h.PATCH("/members/:user", a.memberRole)
	h.GET("/inventory", a.inventory)
	h.GET("/ledger", a.ledger)
	h.GET("/today", a.today)
	h.POST("/ingredients", a.addIngredient)
	h.DELETE("/ingredients/:ingredient", a.archiveIngredient)
	h.POST("/catalog-stock", a.catalogStock)
	h.POST("/ingredients/:ingredient/image", a.uploadIngredientImage)
	h.GET("/ingredients/:ingredient/image", a.ingredientImage)
	h.DELETE("/ingredients/:ingredient/image", a.deleteIngredientImage)
	h.POST("/stock", a.stock)
	h.PATCH("/batches/:batch/quantity", a.correctBatchQuantity)
	h.POST("/batches/:batch/cost", a.recordBatchCost)
	h.GET("/batches/:batch/cost", a.batchCost)
	h.GET("/batches/:batch/nutrition-basis", a.batchNutrition)
	h.POST("/batches/:batch/nutrition-basis", a.confirmBatchNutrition)
	h.GET("/meals/:meal/record", a.mealRecord)
	h.GET("/meals/:meal/pipeline", a.mealPipeline)
	h.POST("/meals/:meal/pipeline", a.createPipeline)
	h.POST("/pipelines/:session/steps/:step", a.progressPipeline)
	h.POST("/recipe-options", a.recipeOptions)
	h.POST("/clear-fridge", a.clearFridge)
	h.GET("/multi-meal", a.listMultiMeals)
	h.POST("/multi-meal", a.createMultiMeal)
	h.POST("/multi-meal/:proposal", a.actMultiMeal)
	h.GET("/nutrition", a.nutritionRadar)
	h.GET("/monthly-report", a.monthlyReport)
	h.GET("/insights", a.kitchenInsights)
	h.GET("/pantry", a.pantry)
	h.POST("/pantry", a.enablePantry)
	h.POST("/pantry/:pantry", a.updatePantry)
	h.GET("/expiring", a.expiring)
	h.POST("/plans", a.createPlan)
	h.GET("/plans", a.plans)
	h.GET("/plans/:plan", a.plan)
	h.PATCH("/plans/:plan/meals/:meal", a.editMeal)
	h.POST("/plans/:plan/confirm", a.confirmPlan)
	h.POST("/plans/:plan/reject", a.rejectPlan)
	h.POST("/plans/:plan/consume", a.consumePlan)
	h.POST("/plans/:plan/meals/:meal/complete", a.completeMeal)
	h.GET("/shopping", a.shopping)
	h.GET("/prices/benchmarks", a.benchmarks)
	h.GET("/prices/dashboard", a.priceDashboard)
	h.PATCH("/shopping/:item", a.editShopping)
	h.DELETE("/shopping/:item", a.deleteShopping)
	h.POST("/shopping/:item/stock", a.stockShopping)
	h.POST("/jobs/plan", a.enqueuePlan)
	h.POST("/jobs/advice", a.enqueueAdvice)
	h.POST("/jobs/image", a.enqueueImage)
	h.GET("/jobs/:job/image", a.imagePreview)
	h.POST("/jobs/:job/image/confirm", a.confirmImage)
	h.GET("/jobs", a.jobs)
	h.GET("/jobs/:job", a.job)
	h.POST("/jobs/:job/cancel", a.cancelJob)
	h.POST("/jobs/:job/retry", a.retryJob)
	h.GET("/events", a.events)
	h.GET("/reminders", a.reminders)
	return r
}
func token() string { var b [32]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (a *App) register(c *gin.Context) {
	var x struct {
		Account, Email, Phone, Password, Name, Code string
		Challenge                                   string `json:"challenge_id"`
	}
	if !input(c, &x) {
		return
	}
	var resolved bool
	x.Email, x.Phone, resolved = resolveAccount(x.Account, x.Email, x.Phone)
	if !resolved {
		fail(c, 400, "请只输入一个账号")
		return
	}
	x.Email = strings.ToLower(strings.TrimSpace(x.Email))
	phone := ""
	if x.Phone != "" {
		var ok bool
		phone, ok = normalizePhone(x.Phone)
		if !ok {
			fail(c, 400, "请输入有效的中国大陆手机号")
			return
		}
	}
	if (x.Email == "") == (phone == "") || (x.Email != "" && !strings.Contains(x.Email, "@")) || len(x.Password) < 8 || len(x.Password) > 72 || strings.TrimSpace(x.Name) == "" || len([]rune(x.Name)) > 80 {
		fail(c, 400, "请提供手机号或邮箱、称呼及8至72字节密码")
		return
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(x.Password), bcrypt.DefaultCost)
	if e != nil {
		fail(c, 500, "password error")
		return
	}
	id := core.ID()
	tx, e := a.DB.Begin(c)
	if e != nil {
		fail(c, 503, "注册暂不可用")
		return
	}
	defer tx.Rollback(c)
	if phone != "" {
		if e = a.verifyCode(c, tx, x.Challenge, phone, "register", "", x.Code); e != nil {
			fail(c, 400, e.Error())
			return
		}
	}
	_, e = tx.Exec(c, "INSERT INTO users(id,email,phone,password_hash,name,phone_verified) VALUES($1,NULLIF($2,''),NULLIF($3,''),$4,$5,$6)", id, x.Email, phone, string(hash), strings.TrimSpace(x.Name), phone != "")
	if e != nil {
		var pgErr *pgconn.PgError
		if errors.As(e, &pgErr) && pgErr.Code == "23505" {
			fail(c, 409, "该账号已注册")
		} else {
			fail(c, 503, "registration unavailable")
		}
		return
	}
	if tx.Commit(c) != nil {
		fail(c, 503, "注册暂不可用")
		return
	}
	a.issue(c, id)
}
func (a *App) login(c *gin.Context) {
	var x struct{ Account, Email, Phone, Password string }
	if !input(c, &x) {
		return
	}
	var resolved bool
	x.Email, x.Phone, resolved = resolveAccount(x.Account, x.Email, x.Phone)
	if !resolved {
		fail(c, 400, "请只输入一个账号")
		return
	}
	var id, hash string
	var authVersion int64
	phone := ""
	if x.Phone != "" {
		var ok bool
		phone, ok = normalizePhone(x.Phone)
		if !ok {
			fail(c, 401, "手机号、邮箱或密码错误")
			return
		}
	}
	if (strings.TrimSpace(x.Email) == "") == (phone == "") || len(x.Password) > 72 {
		fail(c, 401, "手机号、邮箱或密码错误")
		return
	}
	e := a.DB.QueryRow(c, "SELECT id,password_hash,auth_version FROM users WHERE merged_into IS NULL AND (($1<>'' AND email=$1) OR ($2<>'' AND phone=$2))", strings.ToLower(strings.TrimSpace(x.Email)), phone).Scan(&id, &hash, &authVersion)
	if e != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(x.Password)) != nil {
		fail(c, 401, "手机号、邮箱或密码错误")
		return
	}
	a.issueVersion(c, id, authVersion)
}
func (a *App) issue(c *gin.Context, id string) {
	var version int64
	if err := a.DB.QueryRow(c, "SELECT auth_version FROM users WHERE id=$1 AND merged_into IS NULL", id).Scan(&version); err != nil {
		fail(c, 401, "账号状态已变更，请重新登录")
		return
	}
	a.issueVersion(c, id, version)
}
func (a *App) issueVersion(c *gin.Context, id string, version int64) {
	t, expiry, e := a.signSession(id)
	if e != nil {
		fail(c, 500, "token signing failed")
		return
	}
	tag, e := a.DB.Exec(c, "INSERT INTO sessions(token_hash,user_id,expires_at,auth_version) SELECT $1,id,$3,auth_version FROM users WHERE id=$2 AND auth_version=$4 AND merged_into IS NULL", core.Hash(t), id, expiry, version)
	if e != nil {
		fail(c, 500, "session error")
		return
	}
	if tag.RowsAffected() != 1 {
		fail(c, 401, "账号状态已变更，请重新登录")
		return
	}
	c.JSON(200, gin.H{"token": t, "user_id": id, "token_type": "Bearer", "expires_at": expiry})
}
func (a *App) auth(c *gin.Context) {
	t := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if t == "" || !strings.HasPrefix(c.GetHeader("Authorization"), "Bearer ") {
		fail(c, 401, "login required")
		return
	}
	var id string
	e := a.DB.QueryRow(c, "SELECT s.user_id FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now() AND s.auth_version=u.auth_version AND u.merged_into IS NULL", core.Hash(t)).Scan(&id)
	if e != nil {
		fail(c, 401, "invalid session")
		return
	}
	if strings.Contains(t, ".") {
		subject, err := a.verifySession(t)
		if err != nil || subject != id {
			fail(c, 401, "invalid token")
			return
		}
	}
	c.Set("user_id", id)
	c.Next()
}
func (a *App) logout(c *gin.Context) {
	t := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if _, err := a.DB.Exec(c, "DELETE FROM sessions WHERE token_hash=$1", core.Hash(t)); err != nil {
		fail(c, 503, "退出失败，请重试")
		return
	}
	c.Status(204)
}
func (a *App) me(c *gin.Context) {
	var name, email, phone, tz string
	var verified bool
	var days int
	e := a.DB.QueryRow(c, "SELECT name,COALESCE(email,''),COALESCE(phone,''),phone_verified,timezone,remind_days FROM users WHERE id=$1", uid(c)).Scan(&name, &email, &phone, &verified, &tz, &days)
	if e != nil {
		fail(c, 500, "user unavailable")
		return
	}
	c.JSON(200, gin.H{"id": uid(c), "name": name, "email": email, "phone": phone, "phone_verified": verified, "timezone": tz, "remind_days": days})
}
func (a *App) updateMe(c *gin.Context) {
	var x struct {
		Name, Timezone string
		RemindDays     int `json:"remind_days"`
	}
	if !input(c, &x) {
		return
	}
	if strings.TrimSpace(x.Name) == "" || x.RemindDays < 0 || x.RemindDays > 30 {
		fail(c, 400, "invalid profile")
		return
	}
	if _, e := time.LoadLocation(x.Timezone); e != nil {
		fail(c, 400, "invalid timezone")
		return
	}
	_, e := a.DB.Exec(c, "UPDATE users SET name=$1,timezone=$2,remind_days=$3 WHERE id=$4", x.Name, x.Timezone, x.RemindDays, uid(c))
	if e != nil {
		fail(c, 500, "profile update failed")
		return
	}
	c.Status(204)
}
func (a *App) membership(c *gin.Context) {
	var house, user pgtype.UUID
	if house.Scan(hid(c)) != nil || user.Scan(uid(c)) != nil {
		fail(c, 404, "household not found")
		return
	}
	role, e := dbgen.New(a.DB).Membership(c, dbgen.MembershipParams{HouseholdID: house, UserID: user})
	if e != nil {
		fail(c, 404, "household not found")
		return
	}
	c.Set("role", role)
	c.Next()
}
func writable(c *gin.Context) bool {
	if c.GetString("role") == "viewer" {
		fail(c, 403, "editor role required")
		return false
	}
	return true
}
func owner(c *gin.Context) bool {
	if c.GetString("role") != "owner" {
		fail(c, 403, "owner role required")
		return false
	}
	return true
}
func (a *App) households(c *gin.Context) {
	rows, e := a.DB.Query(c, "SELECT h.id,h.name,m.role,h.servings FROM households h JOIN members m ON m.household_id=h.id WHERE m.user_id=$1 ORDER BY h.created_at", uid(c))
	if e != nil {
		fail(c, 500, "query failed")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, name, role string
		var servings int
		if rows.Scan(&id, &name, &role, &servings) == nil {
			out = append(out, gin.H{"id": id, "name": name, "role": role, "servings": servings})
		}
	}
	c.JSON(200, out)
}
func (a *App) createHousehold(c *gin.Context) {
	var x struct {
		Name                string   `json:"name"`
		Servings            int      `json:"servings"`
		Preferences         string   `json:"preferences"`
		ExcludedIngredients []string `json:"excluded_ingredients"`
	}
	if !input(c, &x) {
		return
	}
	if strings.TrimSpace(x.Name) == "" || x.Servings < 1 || x.Servings > 20 {
		fail(c, 400, "invalid household")
		return
	}
	tx, e := a.DB.Begin(c)
	if e != nil {
		fail(c, 500, "transaction failed")
		return
	}
	defer tx.Rollback(c)
	id := core.ID()
	if x.ExcludedIngredients == nil {
		x.ExcludedIngredients = []string{}
	}
	if !validExclusions(x.ExcludedIngredients) {
		fail(c, 400, "invalid excluded ingredients")
		return
	}
	_, e = tx.Exec(c, "INSERT INTO households(id,name,owner_id,servings,preferences,excluded_ingredients,timezone) VALUES($1,$2,$3,$4,$5,$6,(SELECT timezone FROM users WHERE id=$3))", id, x.Name, uid(c), x.Servings, x.Preferences, x.ExcludedIngredients)
	if e == nil {
		_, e = tx.Exec(c, "INSERT INTO members(household_id,user_id,role) VALUES($1,$2,'owner')", id, uid(c))
	}
	if e != nil {
		fail(c, 500, "create failed")
		return
	}
	if e = tx.Commit(c); e != nil {
		fail(c, 500, "commit failed")
		return
	}
	c.JSON(201, gin.H{"id": id})
}
func (a *App) household(c *gin.Context) {
	var name, pref string
	var excluded []string
	var servings int
	e := a.DB.QueryRow(c, "SELECT name,servings,preferences,excluded_ingredients FROM households WHERE id=$1", hid(c)).Scan(&name, &servings, &pref, &excluded)
	if e != nil {
		fail(c, 404, "not found")
		return
	}
	c.JSON(200, gin.H{"id": hid(c), "name": name, "servings": servings, "preferences": pref, "excluded_ingredients": excluded, "role": c.GetString("role")})
}
func (a *App) updateHousehold(c *gin.Context) {
	if !owner(c) {
		return
	}
	var x struct {
		Name                string   `json:"name"`
		Servings            int      `json:"servings"`
		Preferences         string   `json:"preferences"`
		ExcludedIngredients []string `json:"excluded_ingredients"`
	}
	if !input(c, &x) {
		return
	}
	if x.Name == "" || x.Servings < 1 || x.Servings > 20 {
		fail(c, 400, "invalid settings")
		return
	}
	if x.ExcludedIngredients == nil {
		x.ExcludedIngredients = []string{}
	}
	if !validExclusions(x.ExcludedIngredients) {
		fail(c, 400, "invalid excluded ingredients")
		return
	}
	_, e := a.DB.Exec(c, "UPDATE households SET name=$1,servings=$2,preferences=$3,excluded_ingredients=$4 WHERE id=$5", x.Name, x.Servings, x.Preferences, x.ExcludedIngredients, hid(c))
	if e != nil {
		fail(c, 500, "update failed")
		return
	}
	c.Status(204)
}
func (a *App) members(c *gin.Context) {
	rows, e := a.DB.Query(c, "SELECT u.id,u.name,COALESCE(u.email,''),m.role FROM members m JOIN users u ON u.id=m.user_id WHERE m.household_id=$1", hid(c))
	if e != nil {
		fail(c, 500, "query failed")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, name, email, role string
		if rows.Scan(&id, &name, &email, &role) == nil {
			out = append(out, gin.H{"id": id, "name": name, "email": email, "role": role})
		}
	}
	c.JSON(200, out)
}
func (a *App) invite(c *gin.Context) {
	if !owner(c) {
		return
	}
	var x struct{ Email, Role string }
	if !input(c, &x) {
		return
	}
	if x.Role != "editor" && x.Role != "viewer" {
		fail(c, 400, "invalid role")
		return
	}
	code := token()
	id := core.ID()
	_, e := a.DB.Exec(c, "INSERT INTO invites(id,household_id,email,role,code_hash,expires_at) VALUES($1,$2,$3,$4,$5,now()+interval '7 days')", id, hid(c), strings.ToLower(strings.TrimSpace(x.Email)), x.Role, core.Hash(code))
	if e != nil {
		fail(c, 500, "invite failed")
		return
	}
	c.JSON(201, gin.H{"code": code, "expires_in_days": 7})
}
func (a *App) acceptInvite(c *gin.Context) {
	var x struct{ Code string }
	if !input(c, &x) {
		return
	}
	tx, e := a.DB.Begin(c)
	if e != nil {
		fail(c, 500, "transaction failed")
		return
	}
	defer tx.Rollback(c)
	var id, h, role, email string
	e = tx.QueryRow(c, "SELECT id,household_id,role,email FROM invites WHERE code_hash=$1 AND accepted_at IS NULL AND expires_at>now() FOR UPDATE", core.Hash(x.Code)).Scan(&id, &h, &role, &email)
	if e != nil {
		fail(c, 404, "invite unavailable")
		return
	}
	var actual string
	_ = tx.QueryRow(c, "SELECT email FROM users WHERE id=$1", uid(c)).Scan(&actual)
	if actual != email {
		fail(c, 403, "invite belongs to another email")
		return
	}
	_, e = tx.Exec(c, "INSERT INTO members(household_id,user_id,role) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", h, uid(c), role)
	if e == nil {
		_, e = tx.Exec(c, "UPDATE invites SET accepted_at=now() WHERE id=$1", id)
	}
	if e != nil {
		fail(c, 500, "accept failed")
		return
	}
	if e = tx.Commit(c); e != nil {
		fail(c, 500, "commit failed")
		return
	}
	c.JSON(200, gin.H{"household_id": h})
}
func (a *App) memberRole(c *gin.Context) {
	if !owner(c) {
		return
	}
	var x struct{ Role string }
	if !input(c, &x) {
		return
	}
	if x.Role != "editor" && x.Role != "viewer" {
		fail(c, 400, "invalid role")
		return
	}
	tag, e := a.DB.Exec(c, "UPDATE members SET role=$1 WHERE household_id=$2 AND user_id=$3 AND role<>'owner'", x.Role, hid(c), c.Param("user"))
	if e != nil {
		fail(c, 500, "update failed")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(c, 404, "member unavailable")
		return
	}
	c.Status(204)
}
func txError(c *gin.Context, e error) {
	if errors.Is(e, pgx.ErrNoRows) {
		fail(c, 404, "not found")
	} else {
		fail(c, http.StatusInternalServerError, "database error")
	}
}
func validExclusions(items []string) bool {
	if len(items) > 20 {
		return false
	}
	for _, v := range items {
		if strings.TrimSpace(v) == "" || len(v) > 60 {
			return false
		}
	}
	return true
}
