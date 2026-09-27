package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"foodflow/internal/core"
	"foodflow/internal/dbgen"
	engine "foodflow/internal/engine/multimeal"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"math"
	"time"
)

type multiSlot struct {
	Day  string `json:"day"`
	Meal string `json:"meal"`
}
type multiRequest struct {
	Slots      []multiSlot `json:"slots"`
	Servings   int         `json:"servings"`
	MaxMinutes int         `json:"max_minutes"`
}
type multiItem struct {
	Key, Name, Unit string
	Quantity        int64
}
type multiRecipe struct {
	ID, Title         string
	Servings, Minutes int
	Items             []multiItem
}
type multiBatch struct {
	ID, Key, Name, Unit string
	ExpiryKind          string
	Quantity            int64
	Expires             *time.Time
}
type multiSnapshot struct {
	Recipes  []multiRecipe
	Batches  []multiBatch
	Excluded []string
	Timezone string
}
type multiUse struct {
	BatchID    string `json:"batch_id"`
	Name       string `json:"name"`
	Unit       string `json:"unit"`
	Quantity   string `json:"quantity"`
	ExpiryKind string `json:"expiry_kind"`
}
type multiMeal struct {
	multiSlot
	RecipeID string     `json:"recipe_id"`
	Title    string     `json:"title"`
	Minutes  int        `json:"minutes"`
	Uses     []multiUse `json:"uses"`
	Rollover []multiUse `json:"rollover"`
	AtRisk   []multiUse `json:"at_risk"`
}
type multiResult struct {
	Meals     []multiMeal `json:"meals"`
	Truncated bool        `json:"truncated"`
	Nodes     int         `json:"nodes"`
	Score     float64     `json:"score"`
}

// All SQL and conversions are completed before entering the search engine.
func loadMultiSnapshot(c *gin.Context, tx pgx.Tx) (multiSnapshot, error) {
	s := multiSnapshot{}
	err := tx.QueryRow(c, `SELECT excluded_ingredients,timezone FROM households WHERE id=$1 FOR NO KEY UPDATE`, hid(c)).Scan(&s.Excluded, &s.Timezone)
	if err != nil {
		return s, err
	}
	var raw []byte
	err = tx.QueryRow(c, `SELECT COALESCE(jsonb_agg(v ORDER BY v->>'ID'),'[]'::jsonb) FROM (
 SELECT jsonb_build_object('ID',r.id,'Title',r.title,'Servings',r.servings,'Minutes',r.minutes,'Items',
 (SELECT jsonb_agg(jsonb_build_object('Key',COALESCE(ri.catalog_id::text,ic.id::text,ri.name),'Name',ri.name,'Unit',ri.unit,'Quantity',ri.quantity_milli) ORDER BY ri.name,ri.unit) FROM recipe_items ri LEFT JOIN ingredient_catalog ic ON ic.name=ri.name WHERE ri.recipe_id=r.id)) v
 FROM recipes r WHERE NOT EXISTS(SELECT 1 FROM recipe_items ri WHERE ri.recipe_id=r.id AND ri.name=ANY($1::text[])) ORDER BY r.id LIMIT 21) selected`, s.Excluded).Scan(&raw)
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal(raw, &s.Recipes); err != nil {
		return s, err
	}
	if len(s.Recipes) > 20 {
		return s, conflict("当前规划最多支持 20 道候选菜谱")
	}
	rows, err := tx.Query(c, `SELECT b.id,COALESCE(i.catalog_id::text,ic.id::text,i.name),i.name,i.unit,b.quantity_milli,b.expires_at,b.expiry_kind
 FROM batches b JOIN ingredients i ON i.id=b.ingredient_id AND i.household_id=b.household_id LEFT JOIN ingredient_catalog ic ON ic.name=i.name
 WHERE b.household_id=$1 AND b.quantity_milli>0 AND b.condition='normal' AND i.archived_at IS NULL ORDER BY b.id FOR UPDATE OF b`, hid(c))
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var b multiBatch
		if err = rows.Scan(&b.ID, &b.Key, &b.Name, &b.Unit, &b.Quantity, &b.Expires, &b.ExpiryKind); err != nil {
			return s, err
		}
		s.Batches = append(s.Batches, b)
		if len(s.Batches) > 128 {
			return s, conflict("当前规划最多支持 128 个有效批次")
		}
	}
	return s, rows.Err()
}
func multiHash(s multiSnapshot) string {
	b, _ := json.Marshal(s)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func multiInput(s multiSnapshot, request multiRequest, now time.Time) (engine.Input, error) {
	in := engine.Input{Now: now.Unix(), MaxMinutes: request.MaxMinutes, MaxNodes: 50000}
	if request.Servings < 1 || request.Servings > 20 || request.MaxMinutes < 1 || request.MaxMinutes > 240 || len(request.Slots) < 1 || len(request.Slots) > 7 {
		return in, bad("需指定 1–7 餐、1–20 人及每餐 1–240 分钟")
	}
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return in, err
	}
	today := now.In(loc).Format("2006-01-02")
	last := ""
	for _, slot := range request.Slots {
		hour, ok := map[string]int{"breakfast": 8, "lunch": 12, "dinner": 18}[slot.Meal]
		if !ok {
			return in, bad("无效餐次")
		}
		day, err := time.ParseInLocation("2006-01-02", slot.Day, loc)
		if err != nil || slot.Day < today || slot.Day > now.In(loc).AddDate(0, 0, 6).Format("2006-01-02") {
			return in, bad("请选择今天起 7 天内的餐次")
		}
		order := fmt.Sprintf("%s%02d", slot.Day, hour)
		if order <= last {
			return in, bad("餐次必须按时间排列且不能重复")
		}
		last = order
		mealTime := time.Date(day.Year(), day.Month(), day.Day(), hour, 0, 0, 0, loc)
		in.Times = append(in.Times, max(now.Unix(), mealTime.Unix()))
	}
	keys := map[string]int{}
	keyIndex := func(k string) (int, error) {
		if i, ok := keys[k]; ok {
			return i, nil
		}
		if len(keys) >= 64 {
			return 0, bad("规划涉及食材单位组合超过 64 种")
		}
		i := len(keys)
		keys[k] = i
		return i, nil
	}
	for _, r := range s.Recipes {
		recipe := engine.Recipe{Minutes: r.Minutes}
		var mainQty int64
		hasMass := false
		for _, item := range r.Items {
			unit, err := canonicalRecipeUnit(item.Unit)
			if err != nil {
				return in, bad("菜谱存在无法换算的单位")
			}
			qty, err := core.Convert(item.Quantity, item.Unit, unit)
			if err != nil {
				return in, err
			}
			qty, err = core.Scale(qty, request.Servings, r.Servings)
			if err != nil {
				return in, err
			}
			index, err := keyIndex(item.Key + "\x00" + unit)
			if err != nil {
				return in, err
			}
			if recipe.Needs[index] > math.MaxInt64-qty {
				return in, bad("食材数量超出范围")
			}
			recipe.Needs[index] += qty
			// Main ingredient heuristic: first BOM item, preferring the largest mass item.
			if mainQty == 0 || (unit == "g" && (!hasMass || qty > mainQty)) {
				recipe.Main = index
				mainQty = qty
				hasMass = unit == "g"
			}
		}
		in.Recipes = append(in.Recipes, recipe)
	}
	for _, b := range s.Batches {
		unit, err := canonicalRecipeUnit(b.Unit)
		if err != nil {
			return in, bad("库存存在无法换算的单位")
		}
		qty, err := core.Convert(b.Quantity, b.Unit, unit)
		if err != nil {
			return in, err
		}
		index, err := keyIndex(b.Key + "\x00" + unit)
		if err != nil {
			return in, err
		}
		batch := engine.Batch{Ingredient: index, Quantity: qty}
		if b.Expires != nil {
			batch.Expires = b.Expires.Unix()
		}
		in.Batches = append(in.Batches, batch)
	}
	return in, nil
}
func multiOutput(s multiSnapshot, request multiRequest, in engine.Input, plan engine.Result) multiResult {
	out := multiResult{Meals: []multiMeal{}, Truncated: plan.Truncated, Nodes: plan.Nodes, Score: plan.Score}
	for i, slot := range request.Slots {
		choice := plan.Meals[i]
		meal := multiMeal{multiSlot: slot, Uses: []multiUse{}, Rollover: []multiUse{}, AtRisk: []multiUse{}}
		if choice.Recipe >= 0 {
			r := s.Recipes[choice.Recipe]
			meal.RecipeID = r.ID
			meal.Title = r.Title
			meal.Minutes = r.Minutes
		}
		for bi, b := range s.Batches {
			unit, _ := canonicalRecipeUnit(b.Unit)
			if choice.Used[bi] > 0 {
				meal.Uses = append(meal.Uses, multiUse{b.ID, b.Name, unit, core.Format(choice.Used[bi]), b.ExpiryKind})
			}
			if choice.Remaining[bi] > 0 && (b.Expires == nil || b.Expires.Unix() > in.Times[i]) {
				remaining := multiUse{b.ID, b.Name, unit, core.Format(choice.Remaining[bi]), b.ExpiryKind}
				if i+1 < len(in.Times) && b.Expires != nil && b.Expires.Unix() <= in.Times[i+1] {
					meal.AtRisk = append(meal.AtRisk, remaining)
				} else {
					meal.Rollover = append(meal.Rollover, remaining)
				}
			}
		}
		out.Meals = append(out.Meals, meal)
	}
	return out
}
func (a *App) createMultiMeal(c *gin.Context) {
	if !writable(c) {
		return
	}
	var req multiRequest
	if !input(c, &req) {
		return
	}
	a.idem(c, req, func(tx pgx.Tx) (any, error) {
		s, err := loadMultiSnapshot(c, tx)
		if err != nil {
			return nil, err
		}
		in, err := multiInput(s, req, time.Now())
		if err != nil {
			return nil, err
		}
		plan, err := engine.Plan(in)
		if err != nil {
			return nil, bad(err.Error())
		}
		result := multiOutput(s, req, in, plan)
		body, _ := json.Marshal(req)
		value, _ := json.Marshal(result)
		id := core.ID()
		_, err = tx.Exec(c, `INSERT INTO multi_meal_proposals(id,household_id,created_by,request,result,snapshot_hash) VALUES($1,$2,$3,$4,$5,$6)`, id, hid(c), uid(c), body, value, multiHash(s))
		return gin.H{"id": id, "result": result, "status": "pending"}, err
	})
}
func (a *App) listMultiMeals(c *gin.Context) {
	var data []byte
	err := a.DB.QueryRow(c, `SELECT COALESCE(jsonb_agg(v ORDER BY v->>'created_at' DESC),'[]'::jsonb) FROM (SELECT jsonb_build_object('id',id,'result',result,'request',request,'status',status,'plan_id',plan_id,'created_at',created_at,'expires_at',expires_at) v FROM multi_meal_proposals WHERE household_id=$1 ORDER BY created_at DESC LIMIT 5) p`, hid(c)).Scan(&data)
	if err != nil {
		fail(c, 500, "规划读取失败")
		return
	}
	c.Data(200, "application/json", data)
}
func (a *App) actMultiMeal(c *gin.Context) {
	if !writable(c) {
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if !input(c, &req) {
		return
	}
	if req.Action != "adopt" && req.Action != "cancel" {
		fail(c, 400, "无效操作")
		return
	}
	a.idem(c, req, func(tx pgx.Tx) (any, error) {
		// Lock household first for consistent ordering and conflicting slot adoption.
		var house string
		if err := tx.QueryRow(c, `SELECT id FROM households WHERE id=$1 FOR NO KEY UPDATE`, hid(c)).Scan(&house); err != nil {
			return nil, err
		}
		var proposalID, houseID pgtype.UUID
		if proposalID.Scan(c.Param("proposal")) != nil || houseID.Scan(hid(c)) != nil {
			return nil, bad("无效规划编号")
		}
		stored, err := dbgen.New(tx).LockMultiMealProposal(c, dbgen.LockMultiMealProposalParams{ID: proposalID, HouseholdID: houseID})
		if err != nil {
			return nil, businessError{404, "规划不存在"}
		}
		status, hash, rawRequest, rawResult, expiry := stored.Status, stored.SnapshotHash, stored.Request, stored.Result, stored.ExpiresAt.Time
		if status != "pending" {
			return nil, conflict("规划已采纳或取消")
		}
		if req.Action == "cancel" {
			_, err = tx.Exec(c, `UPDATE multi_meal_proposals SET status='cancelled' WHERE id=$1`, c.Param("proposal"))
			return gin.H{"status": "cancelled"}, err
		}
		if time.Now().After(expiry) {
			return nil, conflict("规划已超过 30 分钟，请重新生成")
		}
		var request multiRequest
		var result multiResult
		if err = json.Unmarshal(rawRequest, &request); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(rawResult, &result); err != nil {
			return nil, err
		}
		s, err := loadMultiSnapshot(c, tx)
		if err != nil {
			return nil, err
		}
		if multiHash(s) != hash {
			return nil, conflict("库存、菜谱或家庭偏好已变化，请重新生成")
		}
		in, err := multiInput(s, request, time.Now())
		if err != nil {
			return nil, err
		}
		chosen := 0
		for i, m := range result.Meals {
			if m.RecipeID == "" {
				continue
			}
			chosen++
			var clash bool
			err = tx.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM plan_meals m JOIN plans p ON p.id=m.plan_id WHERE p.household_id=$1 AND m.day=$2::date AND m.meal=$3 AND p.status IN ('confirmed','consumed'))`, hid(c), m.Day, m.Meal).Scan(&clash)
			if err != nil {
				return nil, err
			}
			if clash {
				return nil, conflict("目标餐次已有已确认菜单，请调整餐次")
			}
			for _, use := range m.Uses {
				for bi, b := range s.Batches {
					if b.ID == use.BatchID && in.Batches[bi].Expires != 0 && in.Batches[bi].Expires <= in.Times[i] {
						return nil, conflict("拟使用批次已过期，请重新生成")
					}
				}
			}
		}
		if chosen == 0 {
			return nil, conflict("没有可采纳餐次，请补充库存或调整人数")
		}
		planID := core.ID()
		_, err = tx.Exec(c, `INSERT INTO plans(id,household_id,status,created_by,confirmed_at) VALUES($1,$2,'confirmed',$3,now())`, planID, hid(c), uid(c))
		if err != nil {
			return nil, err
		}
		for _, m := range result.Meals {
			if m.RecipeID == "" {
				continue
			}
			_, err = tx.Exec(c, `INSERT INTO plan_meals(id,plan_id,day,meal,servings,recipe_id) VALUES($1,$2,$3,$4,$5,$6)`, core.ID(), planID, m.Day, m.Meal, request.Servings, m.RecipeID)
			if err != nil {
				return nil, err
			}
		}
		_, err = tx.Exec(c, `INSERT INTO shopping_lists(id,household_id,plan_id) VALUES($1,$2,$3)`, core.ID(), hid(c), planID)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(c, `UPDATE multi_meal_proposals SET status='adopted',plan_id=$2 WHERE id=$1`, c.Param("proposal"), planID)
		return gin.H{"status": "adopted", "plan_id": planID, "stock_deducted": false}, err
	})
}
