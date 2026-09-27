package app

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"foodflow/internal/core"
	"foodflow/internal/dbgen"
	engine "foodflow/internal/engine/pipeline"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type kitchenStep struct {
	Index        int    `json:"index"`
	Dish         string `json:"dish"`
	Action       string `json:"action"`
	Source       string `json:"source"`
	Resources    uint8  `json:"resources"`
	Dependencies []int  `json:"dependencies"`
	Duration     int    `json:"duration"`
	engine.Slot
}
type kitchenSchedule struct {
	Steps       []kitchenStep `json:"steps"`
	Duration    int           `json:"duration"`
	DeadlineMet bool          `json:"deadline_met"`
	Revision    int           `json:"revision"`
}

func (a *App) mealPipeline(c *gin.Context) {
	var meal pgtype.UUID
	if meal.Scan(c.Param("meal")) != nil {
		fail(c, 400, "无效餐次")
		return
	}
	var value []byte
	err := a.DB.QueryRow(c, `SELECT jsonb_build_object('id',s.id,'status',s.status,'starts_at',s.starts_at,'target_at',s.target_at,'schedule',s.schedule,
 'progress',COALESCE((SELECT jsonb_agg(jsonb_build_object('index',step_index,'started_at',started_at,'completed_at',completed_at) ORDER BY step_index) FROM cooking_step_progress WHERE session_id=s.id),'[]'::jsonb))
 FROM cooking_sessions s WHERE s.household_id=$1 AND s.plan_meal_id=$2 AND s.status<>'cancelled' ORDER BY created_at DESC LIMIT 1`, hid(c), c.Param("meal")).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(200, gin.H{"session": nil})
		return
	}
	if err != nil {
		fail(c, 500, "备餐安排读取失败")
		return
	}
	c.JSON(200, gin.H{"session": json.RawMessage(value)})
}

func (a *App) createPipeline(c *gin.Context) {
	if !writable(c) {
		return
	}
	var in struct {
		TargetAt    time.Time `json:"target_at"`
		SecondStove bool      `json:"second_stove"`
	}
	if !input(c, &in) {
		return
	}
	a.idem(c, in, func(tx pgx.Tx) (any, error) {
		now := time.Now().UTC()
		deadline := int(in.TargetAt.Sub(now).Seconds())
		if deadline < 0 || deadline > 86400 {
			return nil, bad("出锅时间须在未来 24 小时内")
		}
		var status string
		var revision int
		err := tx.QueryRow(c, `SELECT p.status,p.revision FROM plans p JOIN plan_meals m ON m.plan_id=p.id WHERE m.id=$1 AND p.household_id=$2 FOR UPDATE OF p`, c.Param("meal"), hid(c)).Scan(&status, &revision)
		if err != nil {
			return nil, businessError{404, "餐次不存在"}
		}
		var completed, exists bool
		if err = tx.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM meal_completions WHERE plan_meal_id=$1),EXISTS(SELECT 1 FROM cooking_sessions WHERE plan_meal_id=$1 AND status<>'cancelled')`, c.Param("meal")).Scan(&completed, &exists); err != nil {
			return nil, err
		}
		if status != "confirmed" || completed {
			return nil, conflict("仅可安排尚未完成的已确认餐次")
		}
		if exists {
			return nil, conflict("本餐已有备餐安排，请刷新或取消原安排")
		}
		rows, err := tx.Query(c, `SELECT recipe_id FROM plan_meals WHERE id=$1 UNION SELECT recipe_id FROM plan_meal_dishes WHERE plan_meal_id=$1`, c.Param("meal"))
		if err != nil {
			return nil, err
		}
		ids := []pgtype.UUID{}
		for rows.Next() {
			var id pgtype.UUID
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		steps, err := dbgen.New(tx).GetRecipeSteps(c, ids)
		if err != nil {
			return nil, err
		}
		if len(steps) == 0 || len(steps) > 64 {
			return nil, conflict("菜谱尚无工序数据，或超过 64 步限制")
		}
		type stepKey struct {
			ID      pgtype.UUID
			Ordinal int32
		}
		index := map[stepKey]int{}
		dishes := map[pgtype.UUID]int{}
		for i, s := range steps {
			index[stepKey{s.RecipeID, s.Ordinal}] = i
			if _, ok := dishes[s.RecipeID]; !ok {
				dishes[s.RecipeID] = len(dishes)
			}
		}
		if len(dishes) != len(ids) {
			return nil, conflict("部分菜谱尚无工序数据，不能生成完整安排")
		}
		var work [64]engine.Step
		schedule := kitchenSchedule{Steps: make([]kitchenStep, len(steps)), Revision: revision}
		for i, s := range steps {
			resource := engine.MainStove
			switch s.Equipment {
			case "board":
				resource = engine.Board
			case "stove":
				if in.SecondStove && dishes[s.RecipeID]%2 == 1 {
					resource = engine.SecondStove
				}
			default:
				return nil, conflict("当前厨房配置仅支持案板与主副灶")
			}
			if !s.IsParallelizable {
				resource |= engine.Hands
			}
			deps := []int{}
			var mask uint64
			for _, d := range s.Dependencies {
				j, ok := index[stepKey{s.RecipeID, d}]
				if !ok {
					return nil, conflict("菜谱工序依赖不完整")
				}
				deps = append(deps, j)
				mask |= uint64(1) << j
			}
			work[i] = engine.Step{Duration: int(s.DurationSeconds), Dependencies: mask, Resources: resource}
			schedule.Steps[i] = kitchenStep{Index: i, Dish: s.Title, Action: s.Action, Source: s.Source, Resources: resource, Dependencies: deps, Duration: int(s.DurationSeconds)}
		}
		result, err := engine.Schedule(work[:len(steps)], deadline)
		if err != nil {
			return nil, conflict("工序配置无效：" + err.Error())
		}
		schedule.Duration = result.Duration
		schedule.DeadlineMet = result.DeadlineMet
		for i := range schedule.Steps {
			schedule.Steps[i].Slot = result.Slots[i]
			// API offsets share starts_at as origin, including the reverse bound.
			schedule.Steps[i].Latest -= max(0, deadline-result.Duration)
		}
		start := in.TargetAt.Add(-time.Duration(result.Duration) * time.Second)
		if start.Before(now) {
			start = now
		}
		payload, err := json.Marshal(schedule)
		if err != nil {
			return nil, err
		}
		id := core.ID()
		_, err = tx.Exec(c, `INSERT INTO cooking_sessions(id,household_id,plan_meal_id,created_by,target_at,starts_at,schedule) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, hid(c), c.Param("meal"), uid(c), in.TargetAt, start, payload)
		return gin.H{"id": id, "schedule": schedule}, err
	})
}

func (a *App) progressPipeline(c *gin.Context) {
	if !writable(c) {
		return
	}
	var in struct {
		Action string `json:"action"`
	}
	if !input(c, &in) {
		return
	}
	if in.Action != "start" && in.Action != "complete" && in.Action != "cancel" {
		fail(c, 400, "无效操作")
		return
	}
	step, err := strconv.Atoi(c.Param("step"))
	if err != nil || step < 0 || step >= 64 {
		fail(c, 400, "无效步骤")
		return
	}
	a.idem(c, in, func(tx pgx.Tx) (any, error) {
		// Match the completion lock order: plan first, then session. This serializes
		// final inventory consumption against late kitchen step writes.
		var planStatus string
		var revision int
		err := tx.QueryRow(c, `SELECT p.status,p.revision FROM plans p JOIN plan_meals m ON m.plan_id=p.id JOIN cooking_sessions s ON s.plan_meal_id=m.id WHERE s.id=$1 AND s.household_id=$2 FOR UPDATE OF p`, c.Param("session"), hid(c)).Scan(&planStatus, &revision)
		if err != nil {
			return nil, businessError{404, "备餐安排不存在"}
		}
		var status string
		var data []byte
		var completed bool
		err = tx.QueryRow(c, `SELECT s.status,s.schedule,EXISTS(SELECT 1 FROM meal_completions WHERE plan_meal_id=s.plan_meal_id) FROM cooking_sessions s WHERE s.id=$1 AND s.household_id=$2 FOR UPDATE OF s`, c.Param("session"), hid(c)).Scan(&status, &data, &completed)
		if err != nil {
			return nil, err
		}
		var schedule kitchenSchedule
		if err = json.Unmarshal(data, &schedule); err != nil {
			return nil, err
		}
		if status != "active" {
			return nil, conflict("备餐已结束或取消")
		}
		if in.Action == "cancel" {
			_, err = tx.Exec(c, `UPDATE cooking_sessions SET status='cancelled',updated_at=now() WHERE id=$1`, c.Param("session"))
			return gin.H{"status": "cancelled"}, err
		}
		if planStatus != "confirmed" || completed || revision != schedule.Revision {
			return nil, conflict("菜单已变更或完成，请取消此安排")
		}
		if step >= len(schedule.Steps) {
			return nil, bad("步骤不存在")
		}
		rows, err := tx.Query(c, `SELECT step_index,completed_at IS NOT NULL FROM cooking_step_progress WHERE session_id=$1`, c.Param("session"))
		if err != nil {
			return nil, err
		}
		var started, done [64]bool
		for rows.Next() {
			var i int
			var d bool
			if err = rows.Scan(&i, &d); err != nil {
				rows.Close()
				return nil, err
			}
			started[i] = true
			done[i] = d
		}
		rows.Close()
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		if in.Action == "start" {
			if started[step] {
				return gin.H{"status": status}, nil
			}
			for _, d := range schedule.Steps[step].Dependencies {
				if !done[d] {
					return nil, conflict("请先完成前置步骤")
				}
			}
			for i, s := range schedule.Steps {
				if started[i] && !done[i] && s.Resources&schedule.Steps[step].Resources != 0 {
					return nil, conflict("双手或设备正被其他步骤占用")
				}
			}
			_, err = tx.Exec(c, `INSERT INTO cooking_step_progress(session_id,household_id,step_index) VALUES($1,$2,$3)`, c.Param("session"), hid(c), step)
		} else {
			if !started[step] {
				return nil, conflict("请先开始本步骤")
			}
			if !done[step] {
				_, err = tx.Exec(c, `UPDATE cooking_step_progress SET completed_at=clock_timestamp() WHERE session_id=$1 AND step_index=$2`, c.Param("session"), step)
			}
			done[step] = true
			all := true
			for i := range schedule.Steps {
				all = all && done[i]
			}
			if all {
				status = "finished"
			}
		}
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(c, `UPDATE cooking_sessions SET status=$2,updated_at=now() WHERE id=$1`, c.Param("session"), status)
		return gin.H{"status": status}, err
	})
}
