package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"foodflow/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type communityIngredient struct {
	CatalogID string `json:"catalog_id"`
	Name      string `json:"name"`
	Quantity  string `json:"quantity"`
	Unit      string `json:"unit"`
}
type communityDraft struct {
	Title         string                `json:"title"`
	Story         string                `json:"story"`
	Servings      int                   `json:"servings"`
	Minutes       int                   `json:"minutes"`
	Tags          []string              `json:"tags"`
	Ingredients   []communityIngredient `json:"ingredients"`
	Steps         []string              `json:"steps"`
	CoverBase64   string                `json:"cover_base64"`
	ConfirmPublic bool                  `json:"confirm_public"`
	ConfirmReview bool                  `json:"confirm_review"`
	RemoveCover   bool                  `json:"remove_cover"`
}
type communityPost struct {
	ID            string                `json:"id"`
	AuthorID      string                `json:"author_id"`
	AuthorName    string                `json:"author_name"`
	Badge         string                `json:"badge"`
	Frame         string                `json:"frame"`
	Title         string                `json:"title"`
	Story         string                `json:"story"`
	Servings      int                   `json:"servings"`
	Minutes       int                   `json:"minutes"`
	Tags          []string              `json:"tags"`
	Ingredients   []communityIngredient `json:"ingredients"`
	Steps         []string              `json:"steps"`
	HasCover      bool                  `json:"has_cover"`
	RecipeID      string                `json:"recipe_id"`
	CreatedAt     time.Time             `json:"created_at"`
	Likes         int                   `json:"likes"`
	Liked         bool                  `json:"liked"`
	Saved         bool                  `json:"saved"`
	WeeklyLikes   int                   `json:"weekly_likes"`
	ReviewStatus  string                `json:"review_status"`
	ReviewVersion int                   `json:"review_version"`
	ReviewNote    string                `json:"review_note"`
	CoverReviewed bool                  `json:"cover_reviewed"`
}
type communityFault struct {
	code    int
	message string
}

func (e communityFault) Error() string { return e.message }
func communityInvalid(s string) error  { return communityFault{400, s} }
func communityID(s string) bool        { var id pgtype.UUID; return id.Scan(s) == nil && id.Valid }
func communityWeek(now time.Time) time.Time {
	loc := time.FixedZone("Asia/Shanghai", 8*3600)
	n := now.In(loc)
	d := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
	return d.AddDate(0, 0, -(int(d.Weekday())+6)%7)
}
func communityAdmin(id string) bool {
	for _, v := range strings.Split(os.Getenv("COMMUNITY_ADMIN_IDS"), ",") {
		if strings.TrimSpace(v) == id && id != "" {
			return true
		}
	}
	return false
}
func communityBind(c *gin.Context, v any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 3<<20)
	if c.ShouldBindJSON(v) != nil {
		fail(c, 400, "请检查填写内容，图片及请求不能超过 3MB")
		return false
	}
	return true
}

// A separate advisory lock serializes the small initial community's writes, never kitchen inventory.
// Balance caps, replay records, awards and redemptions commit in one transaction.
func (a *App) communityWrite(c *gin.Context, request any, run func(pgx.Tx) (any, error)) {
	key := c.GetHeader("Idempotency-Key")
	if len(key) < 8 || len(key) > 128 {
		fail(c, 400, "需要有效的 Idempotency-Key")
		return
	}
	hash := core.Hash(c.Request.Method + " " + c.Request.URL.Path + string(core.JSON(request)))
	tx, e := a.DB.Begin(c)
	if e != nil {
		fail(c, 503, "社区暂时不可用")
		return
	}
	defer tx.Rollback(c)
	if _, e = tx.Exec(c, "SELECT pg_advisory_xact_lock(733033)"); e != nil {
		fail(c, 503, "社区暂时不可用")
		return
	}
	var priorHash string
	var prior []byte
	e = tx.QueryRow(c, "SELECT request_hash,response FROM community_replays WHERE user_id=$1 AND key=$2", uid(c), key).Scan(&priorHash, &prior)
	if e == nil {
		if priorHash != hash {
			fail(c, 409, "同一个请求编号不能用于不同操作")
			return
		}
		c.Data(200, "application/json", prior)
		return
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		fail(c, 503, "社区暂时不可用")
		return
	}
	result, e := run(tx)
	if e != nil {
		var f communityFault
		if errors.As(e, &f) {
			fail(c, f.code, f.message)
		} else {
			fail(c, 503, "操作未完成，请稍后重试")
		}
		return
	}
	if _, e = tx.Exec(c, "INSERT INTO community_replays(user_id,key,request_hash,response) VALUES($1,$2,$3,$4)", uid(c), key, hash, core.JSON(result)); e != nil {
		fail(c, 503, "操作未完成，请稍后重试")
		return
	}
	if e = tx.Commit(c); e != nil {
		fail(c, 503, "操作未完成，请使用原请求编号重试")
		return
	}
	c.JSON(200, result)
}
func communityAward(ctx context.Context, tx pgx.Tx, user string, amount int, reason, event string, cap int) error {
	if cap > 0 {
		var total int
		if e := tx.QueryRow(ctx, `SELECT COALESCE(sum(amount),0) FROM community_points WHERE user_id=$1 AND reason=$2 AND created_at>=date_trunc('day',now() AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai'`, user, reason).Scan(&total); e != nil {
			return e
		}
		if total+amount > cap {
			return nil
		}
	}
	_, e := tx.Exec(ctx, "INSERT INTO community_points(id,user_id,amount,reason,event_key) VALUES($1,$2,$3,$4,$5) ON CONFLICT(user_id,event_key) DO NOTHING", core.ID(), user, amount, reason, event)
	return e
}
func validateCommunityDraft(d *communityDraft) error {
	d.Title = strings.TrimSpace(d.Title)
	d.Story = strings.TrimSpace(d.Story)
	if !d.ConfirmPublic || !d.ConfirmReview {
		return communityInvalid("请确认配方与照片的智能初审及通过后公开分享")
	}
	if utf8.RuneCountInString(d.Title) < 2 || utf8.RuneCountInString(d.Title) > 60 || utf8.RuneCountInString(d.Story) > 1000 || d.Servings < 1 || d.Servings > 20 || d.Minutes < 1 || d.Minutes > 240 {
		return communityInvalid("标题 2–60 字，心得不超过 1000 字；份数 1–20，耗时 1–240 分钟")
	}
	if len(d.Ingredients) < 1 || len(d.Ingredients) > 30 || len(d.Steps) < 1 || len(d.Steps) > 20 || len(d.Tags) < 1 || len(d.Tags) > 3 {
		return communityInvalid("填写 1–30 种食材、1–20 个步骤和 1–3 个标签")
	}
	seen := map[string]bool{}
	for i := range d.Ingredients {
		v := &d.Ingredients[i]
		if !communityID(v.CatalogID) || seen[v.CatalogID] {
			return communityInvalid("食材必须来自目录且不能重复")
		}
		seen[v.CatalogID] = true
		q, e := core.Quantity(v.Quantity)
		if e != nil || q > 100000000 {
			return communityInvalid("食材用量须为正数，最多三位小数且不超过 100000")
		}
		v.Quantity = core.Format(q)
	}
	seen = map[string]bool{}
	for i, s := range d.Steps {
		s = strings.TrimSpace(s)
		if s == "" || utf8.RuneCountInString(s) > 500 {
			return communityInvalid("每一步需填写 1–500 字")
		}
		d.Steps[i] = s
	}
	for _, s := range d.Tags {
		valid := s == "家常" || s == "快手" || s == "早餐" || s == "轻食" || s == "汤羹" || s == "烘焙"
		if !valid || seen[s] {
			return communityInvalid("请选择不重复的菜谱标签")
		}
		seen[s] = true
	}
	return nil
}
func communityCover(encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, nil
	}
	b, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil || len(b) > 2<<20 {
		return nil, communityInvalid("封面需为 2MB 以内的 JPG 或 PNG")
	}
	cfg, _, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || cfg.Width > 1600 || cfg.Height > 1600 || cfg.Width < 1 || cfg.Height < 1 {
		return nil, communityInvalid("封面需为 JPG/PNG，宽高均不超过 1600 像素")
	}
	img, _, e := image.Decode(bytes.NewReader(b))
	if e != nil {
		return nil, communityInvalid("无法读取封面")
	}
	var out bytes.Buffer
	if e = jpeg.Encode(&out, img, &jpeg.Options{Quality: 85}); e != nil {
		return nil, e
	}
	if out.Len() > 2<<20 {
		return nil, communityInvalid("封面过大")
	}
	return out.Bytes(), nil
}
func (a *App) communityPublish(c *gin.Context) {
	var d communityDraft
	if !communityBind(c, &d) {
		return
	}
	if e := validateCommunityDraft(&d); e != nil {
		fail(c, 400, e.Error())
		return
	}
	cover, e := communityCover(d.CoverBase64)
	if e != nil {
		fail(c, 400, e.Error())
		return
	}
	a.communityWrite(c, d, func(tx pgx.Tx) (any, error) {
		var count int
		e := tx.QueryRow(c, `SELECT count(*) FROM community_posts WHERE author_id=$1 AND created_at>=date_trunc('day',now() AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai'`, uid(c)).Scan(&count)
		if e != nil {
			return nil, e
		}
		if count >= 2 {
			return nil, communityFault{429, "每天最多分享两道配方，明天再来吧"}
		}
		for i := range d.Ingredients {
			v := &d.Ingredients[i]
			var unit string
			if e = tx.QueryRow(c, "SELECT name,default_unit FROM ingredient_catalog WHERE id=$1", v.CatalogID).Scan(&v.Name, &unit); e != nil {
				return nil, communityInvalid("食材目录不存在")
			}
			q, _ := core.Quantity(v.Quantity)
			q, e = core.Convert(q, v.Unit, unit)
			if e != nil || q > 100000000 {
				return nil, communityInvalid("食材单位或用量与目录不兼容，请使用 " + unit)
			}
			v.Unit = unit
			v.Quantity = core.Format(q)
		}
		items := append([]communityIngredient{}, d.Ingredients...)
		sort.Slice(items, func(i, j int) bool { return items[i].CatalogID < items[j].CatalogID })
		fingerprint := core.Hash(string(core.JSON(struct {
			Items    []communityIngredient
			Steps    []string
			Servings int
		}{items, d.Steps, d.Servings})))
		var exists bool
		if e = tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM community_posts WHERE author_id=$1 AND fingerprint=$2)", uid(c), fingerprint).Scan(&exists); e != nil {
			return nil, e
		}
		if exists {
			return nil, communityFault{409, "这份配方已经分享过了"}
		}
		id := core.ID()
		mime := ""
		if len(cover) > 0 {
			mime = "image/jpeg"
		}
		_, e = tx.Exec(c, "INSERT INTO community_posts(id,author_id,title,story,servings,minutes,tags,ingredients,steps,fingerprint,cover,cover_type,review_status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'pending')", id, uid(c), d.Title, d.Story, d.Servings, d.Minutes, d.Tags, core.JSON(d.Ingredients), d.Steps, fingerprint, cover, mime)
		if e != nil {
			return nil, e
		}
		return gin.H{"id": id, "review_status": "pending", "review_version": 1}, nil
	})
}

const communitySelect = `SELECT p.id::text,p.author_id::text,u.name,COALESCE(pr.badge,''),COALESCE(pr.frame,''),p.title,p.story,p.servings,p.minutes,p.tags,p.ingredients,p.steps,p.cover IS NOT NULL,COALESCE(p.recipe_id::text,''),p.created_at,
 (SELECT count(*) FROM community_reactions r WHERE r.post_id=p.id AND r.kind='like' AND r.active),
 EXISTS(SELECT 1 FROM community_reactions r WHERE r.post_id=p.id AND r.user_id=$1 AND r.kind='like' AND r.active),
 EXISTS(SELECT 1 FROM community_reactions r WHERE r.post_id=p.id AND r.user_id=$1 AND r.kind='save' AND r.active),
 (SELECT count(*) FROM community_reactions r WHERE r.post_id=p.id AND r.kind='like' AND r.active AND r.created_at>=$2 AND r.created_at<$3),p.review_status,p.review_version,p.review_note,p.cover_reviewed
 FROM community_posts p JOIN users u ON u.id=p.author_id LEFT JOIN community_profiles pr ON pr.user_id=p.author_id `

func scanCommunityPost(row pgx.Row) (communityPost, error) {
	var p communityPost
	var ingredients []byte
	e := row.Scan(&p.ID, &p.AuthorID, &p.AuthorName, &p.Badge, &p.Frame, &p.Title, &p.Story, &p.Servings, &p.Minutes, &p.Tags, &ingredients, &p.Steps, &p.HasCover, &p.RecipeID, &p.CreatedAt, &p.Likes, &p.Liked, &p.Saved, &p.WeeklyLikes, &p.ReviewStatus, &p.ReviewVersion, &p.ReviewNote, &p.CoverReviewed)
	if e == nil {
		e = json.Unmarshal(ingredients, &p.Ingredients)
	}
	return p, e
}
func (a *App) communityFeed(c *gin.Context) {
	offset, e := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if e != nil || offset < 0 || offset > 10000 {
		fail(c, 400, "分页参数无效")
		return
	}
	filter := c.DefaultQuery("filter", "latest")
	if filter != "latest" && filter != "popular" && filter != "saved" && filter != "mine" && filter != "featured" {
		fail(c, 400, "筛选无效")
		return
	}
	start := communityWeek(time.Now())
	q := strings.TrimSpace(c.Query("q"))
	tag := c.Query("tag")
	if utf8.RuneCountInString(q) > 60 {
		fail(c, 400, "搜索词过长")
		return
	}
	rows, e := a.DB.Query(c, communitySelect+`WHERE p.deleted_at IS NULL AND ($4='' OR p.title ILIKE '%'||$4||'%' OR p.story ILIKE '%'||$4||'%') AND ($5='' OR $5=ANY(p.tags))
 AND (p.review_status IN ('legacy','approved') OR ($6='mine' AND p.author_id=$1))
 AND ($6<>'mine' OR p.author_id=$1) AND ($6<>'featured' OR p.recipe_id IS NOT NULL)
 AND ($6<>'saved' OR EXISTS(SELECT 1 FROM community_reactions r WHERE r.post_id=p.id AND r.user_id=$1 AND r.kind='save' AND r.active))
 ORDER BY CASE WHEN $6='popular' THEN (SELECT count(*) FROM community_reactions r WHERE r.post_id=p.id AND r.kind='like' AND r.active) END DESC NULLS LAST,p.created_at DESC,p.id DESC LIMIT 21 OFFSET $7`, uid(c), start, start.AddDate(0, 0, 7), q, tag, filter, offset)
	if e != nil {
		fail(c, 503, "动态暂时无法加载")
		return
	}
	defer rows.Close()
	posts := []communityPost{}
	for rows.Next() {
		p, e := scanCommunityPost(rows)
		if e != nil {
			fail(c, 503, "动态暂时无法加载")
			return
		}
		posts = append(posts, p)
	}
	if rows.Err() != nil {
		fail(c, 503, "动态暂时无法加载")
		return
	}
	more := len(posts) > 20
	if more {
		posts = posts[:20]
	}
	c.JSON(200, gin.H{"posts": posts, "has_more": more})
}
func (a *App) communityDetail(c *gin.Context) {
	if !communityID(c.Param("post")) {
		fail(c, 400, "动态编号无效")
		return
	}
	w := communityWeek(time.Now())
	p, e := scanCommunityPost(a.DB.QueryRow(c, communitySelect+"WHERE p.id=$4 AND p.deleted_at IS NULL AND (p.review_status IN ('legacy','approved') OR p.author_id=$1 OR $5)", uid(c), w, w.AddDate(0, 0, 7), c.Param("post"), communityAdmin(uid(c))))
	if errors.Is(e, pgx.ErrNoRows) {
		fail(c, 404, "动态不存在")
		return
	}
	if e != nil {
		fail(c, 503, "动态暂时无法加载")
		return
	}
	c.JSON(200, p)
}
func (a *App) communityPhoto(c *gin.Context) {
	if !communityID(c.Param("post")) {
		fail(c, 400, "动态编号无效")
		return
	}
	var b []byte
	var mime string
	e := a.DB.QueryRow(c, "SELECT cover,cover_type FROM community_posts WHERE id=$1 AND deleted_at IS NULL AND cover IS NOT NULL AND (review_status IN ('legacy','approved') OR author_id=$2 OR $3)", c.Param("post"), uid(c), communityAdmin(uid(c))).Scan(&b, &mime)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(c, 404, "封面不存在")
		return
	}
	if e != nil {
		fail(c, 503, "封面暂不可用")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(200, mime, b)
}
func (a *App) communityReact(c *gin.Context) {
	var in struct {
		Active *bool `json:"active"`
	}
	if !communityBind(c, &in) {
		return
	}
	kind := c.Param("kind")
	if in.Active == nil || !communityID(c.Param("post")) || (kind != "like" && kind != "save") {
		fail(c, 400, "互动参数无效")
		return
	}
	a.communityWrite(c, in, func(tx pgx.Tx) (any, error) {
		var author string
		e := tx.QueryRow(c, "SELECT author_id::text FROM community_posts WHERE id=$1 AND deleted_at IS NULL AND review_status IN ('legacy','approved')", c.Param("post")).Scan(&author)
		if errors.Is(e, pgx.ErrNoRows) {
			return nil, communityFault{404, "动态不存在"}
		}
		if e != nil {
			return nil, e
		}
		if kind == "like" && author == uid(c) {
			return nil, communityInvalid("不能给自己的配方点赞")
		}
		var existed bool
		if e = tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM community_reactions WHERE post_id=$1 AND user_id=$2 AND kind=$3)", c.Param("post"), uid(c), kind).Scan(&existed); e != nil {
			return nil, e
		}
		if !existed && !*in.Active {
			return gin.H{"active": false}, nil
		}
		// A first interaction remains recorded on unlike; repeat likes cannot earn points or re-enter a new week.
		_, e = tx.Exec(c, "INSERT INTO community_reactions(post_id,user_id,kind,active) VALUES($1,$2,$3,$4) ON CONFLICT(post_id,user_id,kind) DO UPDATE SET active=excluded.active", c.Param("post"), uid(c), kind, *in.Active)
		if e != nil {
			return nil, e
		}
		if kind == "like" && *in.Active && !existed {
			if e = communityAward(c, tx, author, 1, "like", "like:"+c.Param("post")+":"+uid(c), 20); e != nil {
				return nil, e
			}
		}
		return gin.H{"active": *in.Active}, nil
	})
}
func (a *App) communityRemove(c *gin.Context) {
	if !communityID(c.Param("post")) {
		fail(c, 400, "动态编号无效")
		return
	}
	a.communityWrite(c, gin.H{}, func(tx pgx.Tx) (any, error) {
		var author string
		e := tx.QueryRow(c, "SELECT author_id::text FROM community_posts WHERE id=$1", c.Param("post")).Scan(&author)
		if errors.Is(e, pgx.ErrNoRows) {
			return nil, communityFault{404, "动态不存在"}
		}
		if e != nil {
			return nil, e
		}
		if author != uid(c) && !communityAdmin(uid(c)) {
			return nil, communityFault{403, "只能撤下自己的动态"}
		}
		_, e = tx.Exec(c, "UPDATE community_posts SET deleted_at=COALESCE(deleted_at,now()),cover=NULL,cover_type='' WHERE id=$1", c.Param("post"))
		return gin.H{"removed": true}, e
	})
}

type communityReward struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Cost        int    `json:"cost"`
	Kind        string `json:"kind"`
	Owned       bool   `json:"owned"`
	Equipped    bool   `json:"equipped"`
}

func communityShop() []communityReward {
	return []communityReward{
		{ID: "seedling", Name: "新芽创作者", Description: "让每一次认真分享，都有一枚小小的印记", Cost: 30, Kind: "badge"},
		{ID: "sunshine", Name: "暖阳头像框", Description: "给你的社区头像添一圈温暖的金色", Cost: 80, Kind: "frame"},
		{ID: "amber", Name: "蜜糖主题", Description: "把社区换成柔和的蜂蜜与琥珀色", Cost: 120, Kind: "theme"},
	}
}
func (a *App) communityAccount(c *gin.Context) {
	var balance, publications, selected int
	var badge, frame, theme string
	e := a.DB.QueryRow(c, `SELECT COALESCE((SELECT sum(amount) FROM community_points WHERE user_id=$1),0),
 (SELECT count(*) FROM community_posts WHERE author_id=$1 AND deleted_at IS NULL),
 (SELECT count(*) FROM community_posts WHERE author_id=$1 AND recipe_id IS NOT NULL),
 COALESCE((SELECT badge FROM community_profiles WHERE user_id=$1),''),COALESCE((SELECT frame FROM community_profiles WHERE user_id=$1),''),COALESCE((SELECT theme FROM community_profiles WHERE user_id=$1),'forest')`, uid(c)).Scan(&balance, &publications, &selected, &badge, &frame, &theme)
	if e != nil {
		fail(c, 503, "积分暂时无法加载")
		return
	}
	rewards := communityShop()
	for i := range rewards {
		v := &rewards[i]
		if e = a.DB.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM community_rewards WHERE user_id=$1 AND reward=$2)", uid(c), v.ID).Scan(&v.Owned); e != nil {
			fail(c, 503, "积分暂时无法加载")
			return
		}
		v.Equipped = v.ID == badge || v.ID == frame || v.ID == theme
	}
	rows, e := a.DB.Query(c, "SELECT amount,reason,created_at FROM community_points WHERE user_id=$1 ORDER BY created_at DESC,id DESC LIMIT 30", uid(c))
	if e != nil {
		fail(c, 503, "积分暂时无法加载")
		return
	}
	defer rows.Close()
	ledger := []gin.H{}
	for rows.Next() {
		var n int
		var reason string
		var at time.Time
		if e = rows.Scan(&n, &reason, &at); e != nil {
			fail(c, 503, "积分暂时无法加载")
			return
		}
		ledger = append(ledger, gin.H{"amount": n, "reason": reason, "created_at": at})
	}
	if rows.Err() != nil {
		fail(c, 503, "积分暂时无法加载")
		return
	}
	c.JSON(200, gin.H{"user_id": uid(c), "is_admin": communityAdmin(uid(c)), "balance": balance, "published": publications, "selected": selected, "badge": badge, "frame": frame, "theme": theme, "rewards": rewards, "ledger": ledger})
}
func (a *App) communityRedeem(c *gin.Context) {
	var reward communityReward
	for _, v := range communityShop() {
		if v.ID == c.Param("reward") {
			reward = v
		}
	}
	if reward.ID == "" {
		fail(c, 404, "奖励不存在")
		return
	}
	a.communityWrite(c, gin.H{"reward": reward.ID}, func(tx pgx.Tx) (any, error) {
		var owned bool
		var balance int
		e := tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM community_rewards WHERE user_id=$1 AND reward=$2)", uid(c), reward.ID).Scan(&owned)
		if e != nil {
			return nil, e
		}
		if !owned {
			if e = tx.QueryRow(c, "SELECT COALESCE(sum(amount),0) FROM community_points WHERE user_id=$1", uid(c)).Scan(&balance); e != nil {
				return nil, e
			}
			if balance < reward.Cost {
				return nil, communityFault{409, "积分还不够，再分享几道拿手菜吧"}
			}
			if e = communityAward(c, tx, uid(c), -reward.Cost, "redeem", "redeem:"+reward.ID, 0); e != nil {
				return nil, e
			}
			if _, e = tx.Exec(c, "INSERT INTO community_rewards(user_id,reward) VALUES($1,$2)", uid(c), reward.ID); e != nil {
				return nil, e
			}
		}
		if e = communityEquip(c, tx, uid(c), reward); e != nil {
			return nil, e
		}
		return gin.H{"owned": true, "equipped": true}, nil
	})
}
func communityEquip(ctx context.Context, tx pgx.Tx, user string, r communityReward) error {
	if _, e := tx.Exec(ctx, "INSERT INTO community_profiles(user_id) VALUES($1) ON CONFLICT DO NOTHING", user); e != nil {
		return e
	}
	// Column names are drawn exclusively from the fixed server-side reward catalogue.
	_, e := tx.Exec(ctx, "UPDATE community_profiles SET "+r.Kind+"=$2 WHERE user_id=$1", user, r.ID)
	return e
}
func (a *App) communityEquipReward(c *gin.Context) {
	var reward communityReward
	for _, v := range communityShop() {
		if v.ID == c.Param("reward") {
			reward = v
		}
	}
	if reward.ID == "" {
		fail(c, 404, "奖励不存在")
		return
	}
	a.communityWrite(c, gin.H{}, func(tx pgx.Tx) (any, error) {
		var owned bool
		if e := tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM community_rewards WHERE user_id=$1 AND reward=$2)", uid(c), reward.ID).Scan(&owned); e != nil {
			return nil, e
		}
		if !owned {
			return nil, communityFault{403, "先兑换这件奖励"}
		}
		return gin.H{"equipped": true}, communityEquip(c, tx, uid(c), reward)
	})
}

// Weekly shortlist is frozen only after the Shanghai calendar week closes.
func finalizeCommunityWeek(ctx context.Context, tx pgx.Tx, week time.Time) error {
	tag, e := tx.Exec(ctx, "INSERT INTO community_weeks(week_start) VALUES($1) ON CONFLICT DO NOTHING", week.Format("2006-01-02"))
	if e != nil || tag.RowsAffected() == 0 {
		return e
	}
	rows, e := tx.Query(ctx, `WITH scores AS (
 SELECT p.id,p.author_id,p.created_at,count(r.user_id)::int AS votes FROM community_posts p JOIN community_reactions r ON r.post_id=p.id
 WHERE p.deleted_at IS NULL AND p.review_status IN ('legacy','approved') AND p.recipe_id IS NULL AND p.created_at<$2 AND r.kind='like' AND r.active AND r.created_at>=$1 AND r.created_at<$2
 GROUP BY p.id), ranked AS (SELECT *,row_number() OVER(PARTITION BY author_id ORDER BY votes DESC,created_at,id) AS author_rank FROM scores)
 SELECT id::text,author_id::text,votes FROM ranked WHERE author_rank=1 ORDER BY votes DESC,created_at,id LIMIT 3`, week, week.AddDate(0, 0, 7))
	if e != nil {
		return e
	}
	type winner struct {
		id, author string
		votes      int
	}
	winners := []winner{}
	for rows.Next() {
		var v winner
		if e = rows.Scan(&v.id, &v.author, &v.votes); e != nil {
			rows.Close()
			return e
		}
		winners = append(winners, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for i, v := range winners {
		if _, e = tx.Exec(ctx, "INSERT INTO community_candidates(id,week_start,post_id,author_id,rank,votes) VALUES($1,$2,$3,$4,$5,$6)", core.ID(), week.Format("2006-01-02"), v.id, v.author, i+1, v.votes); e != nil {
			return e
		}
	}
	return nil
}

// Worker calls this on boot and hourly. No model provider is involved.
func FinalizeCommunityPreviousWeek(ctx context.Context, db *pgxpool.Pool) error {
	tx, e := db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(733033)"); e != nil {
		return e
	}
	if e = finalizeCommunityWeek(ctx, tx, communityWeek(time.Now()).AddDate(0, 0, -7)); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (a *App) communityFinalize(c *gin.Context) {
	if !communityAdmin(uid(c)) {
		fail(c, 403, "仅社区管理员可以生成候选")
		return
	}
	week, e := time.ParseInLocation("2006-01-02", c.Param("week"), time.FixedZone("Asia/Shanghai", 8*3600))
	if e != nil || week.Weekday() != time.Monday || !week.Before(communityWeek(time.Now())) {
		fail(c, 400, "只能结算已经结束的完整自然周")
		return
	}
	a.communityWrite(c, gin.H{}, func(tx pgx.Tx) (any, error) { return gin.H{"finalized": true}, finalizeCommunityWeek(c, tx, week) })
}
func (a *App) communityWeekly(c *gin.Context) {
	start := communityWeek(time.Now())
	previous := start.AddDate(0, 0, -7)
	rows, e := a.DB.Query(c, `WITH scores AS (
 SELECT p.id,p.author_id,p.created_at,count(r.user_id)::int AS votes FROM community_posts p JOIN community_reactions r ON r.post_id=p.id
 WHERE p.deleted_at IS NULL AND p.review_status IN ('legacy','approved') AND p.recipe_id IS NULL AND r.kind='like' AND r.active AND r.created_at>=$2 AND r.created_at<$3
 GROUP BY p.id), ranked AS (SELECT *,row_number() OVER(PARTITION BY author_id ORDER BY votes DESC,created_at,id) AS author_rank FROM scores),
 winners AS (SELECT id FROM ranked WHERE author_rank=1 ORDER BY votes DESC,created_at,id LIMIT 3)
 `+communitySelect+`WHERE p.id IN (SELECT id FROM winners) ORDER BY 19 DESC,p.created_at,p.id`, uid(c), start, start.AddDate(0, 0, 7))
	if e != nil {
		fail(c, 503, "周榜暂不可用")
		return
	}
	current := []communityPost{}
	for rows.Next() {
		p, e := scanCommunityPost(rows)
		if e != nil {
			rows.Close()
			fail(c, 503, "周榜暂不可用")
			return
		}
		current = append(current, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		fail(c, 503, "周榜暂不可用")
		return
	}
	var finalized bool
	e = a.DB.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM community_weeks WHERE week_start=$1)", previous.Format("2006-01-02")).Scan(&finalized)
	if e != nil {
		fail(c, 503, "周榜暂不可用")
		return
	}
	cr, e := a.DB.Query(c, "SELECT id::text,post_id::text,rank,votes,status,review_note FROM community_candidates WHERE week_start=$1 ORDER BY rank", previous.Format("2006-01-02"))
	if e != nil {
		fail(c, 503, "候选暂不可用")
		return
	}
	candidates := []gin.H{}
	for cr.Next() {
		var id, post, status, note string
		var rank, votes int
		if e = cr.Scan(&id, &post, &rank, &votes, &status, &note); e != nil {
			break
		}
		candidates = append(candidates, gin.H{"id": id, "post_id": post, "rank": rank, "votes": votes, "status": status, "review_note": note})
	}
	if e == nil {
		e = cr.Err()
	}
	cr.Close()
	if e != nil {
		fail(c, 503, "候选暂不可用")
		return
	}
	c.JSON(200, gin.H{"week_start": start.Format("2006-01-02"), "week_end": start.AddDate(0, 0, 7).Format("2006-01-02"), "current": current, "previous_week": previous.Format("2006-01-02"), "finalized": finalized, "candidates": candidates})
}
func (a *App) communityReview(c *gin.Context) {
	if !communityAdmin(uid(c)) {
		fail(c, 403, "仅社区管理员可以审核食谱")
		return
	}
	var in struct {
		Accept   bool   `json:"accept"`
		Note     string `json:"note"`
		Confirm  bool   `json:"confirm"`
		Workflow []struct {
			Equipment string `json:"equipment"`
			Duration  int    `json:"duration_seconds"`
		} `json:"workflow"`
	}
	if !communityBind(c, &in) {
		return
	}
	in.Note = strings.TrimSpace(in.Note)
	if !communityID(c.Param("candidate")) || !in.Confirm || utf8.RuneCountInString(in.Note) > 300 || (!in.Accept && in.Note == "") {
		fail(c, 400, "请确认审核结果，并为未收录配方填写原因")
		return
	}
	a.communityWrite(c, in, func(tx pgx.Tx) (any, error) {
		var post, status string
		e := tx.QueryRow(c, "SELECT post_id::text,status FROM community_candidates WHERE id=$1", c.Param("candidate")).Scan(&post, &status)
		if errors.Is(e, pgx.ErrNoRows) {
			return nil, communityFault{404, "候选不存在"}
		}
		if e != nil {
			return nil, e
		}
		if status != "pending" {
			return nil, communityFault{409, "这份配方已审核"}
		}
		next := "rejected"
		recipe := ""
		if in.Accept {
			var author, name, title string
			var servings, minutes int
			var tags, steps []string
			var raw []byte
			e = tx.QueryRow(c, `SELECT p.author_id::text,u.name,p.title,p.servings,p.minutes,p.tags,p.steps,p.ingredients FROM community_posts p JOIN users u ON u.id=p.author_id WHERE p.id=$1 AND p.deleted_at IS NULL AND p.review_status IN ('legacy','approved') AND p.recipe_id IS NULL`, post).Scan(&author, &name, &title, &servings, &minutes, &tags, &steps, &raw)
			if errors.Is(e, pgx.ErrNoRows) {
				return nil, communityFault{409, "动态已撤下或已收录"}
			}
			if e != nil {
				return nil, e
			}
			var items []communityIngredient
			if e = json.Unmarshal(raw, &items); e != nil {
				return nil, e
			}
			recipe = core.ID()
			source := fmt.Sprintf("食光社区 · %s · 配方 %s · 份量与耗时由作者提供，收录后版本固定", name, post)
			if _, e = tx.Exec(c, "INSERT INTO recipes(id,title,servings,minutes,tags,steps,source) VALUES($1,$2,$3,$4,$5,$6,$7)", recipe, title, servings, minutes, tags, steps, source); e != nil {
				return nil, e
			}
			for _, v := range items {
				var catalogName, unit string
				if e = tx.QueryRow(c, "SELECT name,default_unit FROM ingredient_catalog WHERE id=$1", v.CatalogID).Scan(&catalogName, &unit); e != nil {
					return nil, communityInvalid("食材目录已变化，请重新核对")
				}
				q, err := core.Quantity(v.Quantity)
				if err != nil {
					return nil, err
				}
				if _, err = core.Convert(q, v.Unit, unit); err != nil {
					return nil, err
				}
				if _, e = tx.Exec(c, "INSERT INTO recipe_items(recipe_id,name,quantity_milli,unit,catalog_id) VALUES($1,$2,$3,$4,$5)", recipe, catalogName, q, v.Unit, v.CatalogID); e != nil {
					return nil, e
				}
			}
			// Equipment/time must be explicitly reviewed; never silently guess kitchen resources.
			if len(in.Workflow) != len(steps) {
				return nil, communityInvalid("请为每个步骤确认设备与预估耗时")
			}
			for i, v := range in.Workflow {
				if (v.Equipment != "board" && v.Equipment != "stove") || v.Duration < 1 || v.Duration > 14400 {
					return nil, communityInvalid("目前仅支持操作台与灶台，步骤耗时 1–14400 秒")
				}
				deps := []int32{}
				if i > 0 {
					deps = append(deps, int32(i-1))
				}
				kind := "cook"
				if v.Equipment == "board" {
					kind = "prep"
				}
				if _, e = tx.Exec(c, "INSERT INTO recipe_steps(recipe_id,ordinal,action,kind,duration_seconds,dependencies,equipment,is_parallelizable,source) VALUES($1,$2,$3,$4,$5,$6,$7,false,$8)", recipe, i, steps[i], kind, v.Duration, deps, v.Equipment, "社区审核预估耗时；熟透情况由使用者确认"); e != nil {
					return nil, e
				}
			}
			if _, e = tx.Exec(c, "UPDATE community_posts SET recipe_id=$2 WHERE id=$1", post, recipe); e != nil {
				return nil, e
			}
			if e = communityAward(c, tx, author, 50, "selected", "selected:"+post, 0); e != nil {
				return nil, e
			}
			next = "accepted"
		}
		_, e = tx.Exec(c, "UPDATE community_candidates SET status=$2,review_note=$3,reviewed_by=$4,reviewed_at=now() WHERE id=$1", c.Param("candidate"), next, in.Note, uid(c))
		return gin.H{"status": next, "recipe_id": recipe}, e
	})
}
