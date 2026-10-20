package app

import (
	"context"
	"encoding/json"
	"fmt"
	"foodflow/internal/agent"
	"foodflow/internal/core"
	"foodflow/internal/nutrition"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"os"
	"sort"
	"strings"
)

type adviceRequest struct {
	Selected   []string `json:"selected_ingredient_ids"`
	Servings   int      `json:"servings"`
	MaxMinutes int      `json:"max_minutes"`
	Goal       string   `json:"goal"`
}
type adviceStock struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Unit      string            `json:"unit"`
	Quantity  string            `json:"quantity"`
	Nutrition nutrition.Profile `json:"nutrition"`
}

func (a *App) adviceInventory(ctx context.Context, house string, ids []string) ([]adviceStock, error) {
	rows, err := a.DB.Query(ctx, `SELECT i.id,i.name,i.unit,COALESCE(c.name,i.name),COALESCE(sum(b.quantity_milli) FILTER(WHERE b.condition='normal' AND (b.expires_at IS NULL OR b.expires_at>now())),0)
 FROM ingredients i LEFT JOIN ingredient_catalog c ON c.id=i.catalog_id LEFT JOIN batches b ON b.ingredient_id=i.id AND b.household_id=i.household_id
 WHERE i.household_id=$1 AND i.archived_at IS NULL AND i.id::text=ANY($2::text[]) GROUP BY i.id,c.name ORDER BY i.id`, house, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adviceStock
	for rows.Next() {
		var v adviceStock
		var canonical string
		var quantity int64
		if err := rows.Scan(&v.ID, &v.Name, &v.Unit, &canonical, &quantity); err != nil {
			return nil, err
		}
		if quantity <= 0 {
			return nil, fmt.Errorf("所选食材已无有效库存，请重新选择")
		}
		v.Quantity = core.Format(quantity)
		v.Nutrition = nutrition.Lookup(canonical)
		out = append(out, v)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	if len(out) != len(ids) {
		return nil, fmt.Errorf("所选食材不可用或不属于当前家庭")
	}
	return out, nil
}
func (a *App) enqueueAdvice(c *gin.Context) {
	if !writable(c) {
		return
	}
	var req adviceRequest
	if !input(c, &req) {
		return
	}
	key := c.GetHeader("Idempotency-Key")
	if len(key) < 8 || len(key) > 120 || len(req.Selected) < 1 || len(req.Selected) > 30 || req.Servings < 1 || req.Servings > 20 || req.MaxMinutes < 1 || req.MaxMinutes > 240 || len([]rune(req.Goal)) > 500 {
		fail(c, 400, "请选择1–30种食材，并检查人数、时间与建议目标")
		return
	}
	sort.Strings(req.Selected)
	for i, id := range req.Selected {
		if len(id) != 36 || (i > 0 && id == req.Selected[i-1]) {
			fail(c, 400, "食材选择无效或重复")
			return
		}
	}
	req.Goal = strings.TrimSpace(req.Goal)
	hash := core.Hash(string(core.JSON(req)))
	// Replay before checking live stock: a previously accepted request stays replayable.
	var existing, oldHash string
	err := a.DB.QueryRow(c, "SELECT id,request_hash FROM jobs WHERE household_id=$1 AND created_by=$2 AND request_key=$3", hid(c), uid(c), key).Scan(&existing, &oldHash)
	if err == nil {
		if oldHash != hash {
			fail(c, 409, "此请求编号已用于不同的建议")
			return
		}
		c.JSON(202, gin.H{"id": existing})
		return
	}
	if err != pgx.ErrNoRows {
		fail(c, 500, "任务读取失败")
		return
	}
	if _, err := a.adviceInventory(c, hid(c), req.Selected); err != nil {
		fail(c, 400, err.Error())
		return
	}
	id := core.ID()
	err = a.DB.QueryRow(c, `INSERT INTO jobs(id,household_id,kind,status,payload,created_by,request_key,request_hash) VALUES($1,$2,'advice','queued',$3,$4,$5,$6)
 ON CONFLICT(household_id,created_by,request_key) WHERE request_key IS NOT NULL DO UPDATE SET request_hash=jobs.request_hash RETURNING id,request_hash`, id, hid(c), core.JSON(req), uid(c), key, hash).Scan(&id, &oldHash)
	if err != nil {
		fail(c, 500, "建议任务创建失败")
		return
	}
	if oldHash != hash {
		fail(c, 409, "此请求编号已用于不同的建议")
		return
	}
	c.JSON(202, gin.H{"id": id})
}

type adviceRecipe struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Minutes     int      `json:"minutes"`
	Ingredients []string `json:"ingredients"`
}

func (a *App) runAdvice(ctx context.Context, j claimed) error {
	var live bool
	if err := a.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM jobs WHERE id=$1 AND lease_token=$2 AND status='running' AND NOT cancel_requested AND lease_until>now())", j.ID, j.Token).Scan(&live); err != nil {
		return err
	}
	if !live {
		return fmt.Errorf("任务已取消或租约失效")
	}
	var req adviceRequest
	if err := json.Unmarshal(j.Payload, &req); err != nil {
		return err
	}
	var pref, role string
	var excluded []string
	err := a.DB.QueryRow(ctx, "SELECT h.preferences,h.excluded_ingredients,m.role FROM households h JOIN members m ON m.household_id=h.id WHERE h.id=$1 AND m.user_id=$2 AND (h.kind='family' OR h.owner_id=m.user_id)", j.Household, j.Creator).Scan(&pref, &excluded, &role)
	if err != nil || role == "viewer" {
		return fmt.Errorf("家庭权限已变化，无法生成建议")
	}
	stock, err := a.adviceInventory(ctx, j.Household, req.Selected)
	if err != nil {
		return err
	}
	selectedNames := []string{}
	for _, v := range stock {
		selectedNames = append(selectedNames, v.Name)
	}
	rows, err := a.DB.Query(ctx, `SELECT r.id,r.title,r.minutes,array_agg(ri.name ORDER BY ri.name) FROM recipes r JOIN recipe_items ri ON ri.recipe_id=r.id
 WHERE r.minutes<=$1 AND NOT EXISTS(SELECT 1 FROM recipe_items x WHERE x.recipe_id=r.id AND x.name=ANY($2::text[]))
 AND EXISTS(SELECT 1 FROM recipe_items x WHERE x.recipe_id=r.id AND x.name=ANY($3::text[])) GROUP BY r.id ORDER BY r.minutes,r.id LIMIT 20`, req.MaxMinutes, excluded, selectedNames)
	if err != nil {
		return err
	}
	recipes := []adviceRecipe{}
	allowed := map[string]bool{}
	for rows.Next() {
		var r adviceRecipe
		if err := rows.Scan(&r.ID, &r.Title, &r.Minutes, &r.Ingredients); err != nil {
			rows.Close()
			return err
		}
		recipes = append(recipes, r)
		allowed[r.ID] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	_, err = a.DB.Exec(ctx, "INSERT INTO job_events(job_id,event,detail) VALUES($1,'nutrition_tools_read',$2)", j.ID, core.JSON(gin.H{"selected_count": len(stock), "candidate_count": len(recipes), "nutrition_source": "USDA SR Legacy 2018; missing values remain unknown"}))
	if err != nil {
		return err
	}
	agentContext := gin.H{"goal": req.Goal, "servings": req.Servings, "household_preferences": pref, "excluded_ingredients": excluded, "selected_inventory": stock, "allowed_recipes": recipes}
	var result agent.Advice
	mode := "demo"
	if os.Getenv("MODEL_ENDPOINT") != "" && os.Getenv("MODEL_NAME") != "" && os.Getenv("MODEL_API_KEY") != "" && paidAIEnabled() {
		mode = "model"
		model := agent.OpenAIModel{Endpoint: os.Getenv("MODEL_ENDPOINT"), Name: os.Getenv("MODEL_NAME"), Key: os.Getenv("MODEL_API_KEY"), MaxTokens: 1600, Guard: a.allowanceGuard(j, "text"), Task: "advice", Context: agentContext}
		prompt := core.JSON(agentContext)
		if len(prompt) > 48000 {
			return fmt.Errorf("建议上下文超出调用预算，请减少所选食材或偏好长度")
		}
		if _, err := a.DB.Exec(ctx, "INSERT INTO job_events(job_id,event) VALUES($1,'advice_model_call_started')", j.ID); err != nil {
			return err
		}
		answer, err := model.Generate(ctx, nil)
		if err != nil {
			return err
		}
		result, err = agent.ParseAdvice(answer.Content, allowed)
		if err != nil {
			return err
		}
	}
	if mode == "demo" {
		result, err = agent.DemoAdvice(ctx, agentContext, allowed)
		if err != nil {
			return err
		}
	}
	chosen := []adviceRecipe{}
	for _, id := range result.RecipeIDs {
		for _, r := range recipes {
			if r.ID == id {
				chosen = append(chosen, r)
			}
		}
	}
	// Completion is fenced by lease and cancellation. No inventory/menu writes occur.
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	tag, err := tx.Exec(ctx, `UPDATE jobs SET status='succeeded',progress=100,result=$1,lease_token=NULL,lease_until=NULL,lease_owner=NULL,updated_at=now()
 WHERE id=$2 AND lease_token=$3 AND lease_until>now() AND status='running' AND NOT cancel_requested
 AND EXISTS(SELECT 1 FROM members m JOIN households h ON h.id=m.household_id WHERE m.household_id=$4 AND m.user_id=$5 AND m.role!='viewer' AND (h.kind='family' OR h.owner_id=m.user_id))`, core.JSON(gin.H{"mode": mode, "summary": result.Summary, "tips": result.Tips, "recipes": chosen, "selected_inventory": stock, "servings": req.Servings}), j.ID, j.Token, j.Household, j.Creator)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("任务已取消、租约或权限已失效")
	}
	_, err = tx.Exec(ctx, "INSERT INTO job_events(job_id,event) VALUES($1,'advice_succeeded')", j.ID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
