package app

import (
	"context"
	"errors"
	"foodflow/internal/agent"
	"foodflow/internal/core"
	"github.com/gin-gonic/gin"
	"os"
	"regexp"
	"strconv"
	"time"
)

// Amounts are exact milli-CNY policy allowances, not provider invoice amounts.
type allowanceConfig struct {
	Enabled              bool
	Monthly, Text, Image int64
	Daily, Concurrent    int
}

var allowanceAmount = regexp.MustCompile(`^[0-9]+(\.[0-9]{1,3})?$`)

func loadAllowanceConfig() (allowanceConfig, error) {
	cfg := allowanceConfig{Enabled: os.Getenv("AI_ALLOWANCE_ENABLED") == "true", Daily: 20, Concurrent: 1}
	for _, v := range []struct {
		Key, Default string
		Target       *int64
	}{{"AI_MONTHLY_ALLOWANCE_CNY", "5", &cfg.Monthly}, {"AI_TEXT_CALL_ALLOWANCE_CNY", "0.10", &cfg.Text}, {"AI_IMAGE_CALL_ALLOWANCE_CNY", "0.50", &cfg.Image}} {
		raw := os.Getenv(v.Key)
		if raw == "" {
			raw = v.Default
		}
		if !allowanceAmount.MatchString(raw) {
			return cfg, errors.New("invalid AI allowance amount")
		}
		amount, err := core.Quantity(raw)
		if err != nil || amount <= 0 || amount > 1000000000 {
			return cfg, errors.New("invalid AI allowance amount")
		}
		*v.Target = amount
	}
	for _, v := range []struct {
		Key    string
		Target *int
	}{{"AI_DAILY_CALL_LIMIT", &cfg.Daily}, {"AI_CONCURRENT_CALL_LIMIT", &cfg.Concurrent}} {
		if raw := os.Getenv(v.Key); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > 1000 {
				return cfg, errors.New("invalid AI call limit")
			}
			*v.Target = n
		}
	}
	return cfg, nil
}
func paidAIEnabled() bool { cfg, err := loadAllowanceConfig(); return err == nil && cfg.Enabled }
func (a *App) aiAllowance(c *gin.Context) {
	cfg, err := loadAllowanceConfig()
	if err != nil {
		fail(c, 503, "AI 额度配置不可用")
		return
	}
	var global, house int64
	var daily, active int
	err = a.DB.QueryRow(c, `SELECT
 COALESCE(sum(ceiling_milli) FILTER (WHERE month=date_trunc('month',now() AT TIME ZONE 'Asia/Shanghai')::date),0),
 COALESCE(sum(ceiling_milli) FILTER (WHERE month=date_trunc('month',now() AT TIME ZONE 'Asia/Shanghai')::date AND household_id=$1),0),
 count(*) FILTER (WHERE (created_at AT TIME ZONE 'Asia/Shanghai')::date=(now() AT TIME ZONE 'Asia/Shanghai')::date),
 count(*) FILTER (WHERE state IN ('pending','uncertain')) FROM ai_call_allowances`, hid(c)).Scan(&global, &house, &daily, &active)
	if err != nil {
		fail(c, 503, "AI 额度暂不可用")
		return
	}
	c.JSON(200, gin.H{"enabled": cfg.Enabled, "monthly_limit": core.Format(cfg.Monthly), "household_reserved": core.Format(house), "service_reserved": core.Format(global), "text_call_allowance": core.Format(cfg.Text), "image_call_allowance": core.Format(cfg.Image), "daily_calls": daily, "daily_limit": cfg.Daily, "active_or_uncertain": active, "concurrent_limit": cfg.Concurrent})
}
func (a *App) allowanceGuard(j claimed, kind string) agent.CallGuard {
	return func(ctx context.Context, requestBytes int) (func(bool), error) {
		cfg, err := loadAllowanceConfig()
		if err != nil {
			return nil, err
		}
		if !cfg.Enabled {
			return nil, errors.New("收费 AI 未启用额度保护，调用已阻止")
		}
		if kind == "text" && requestBytes > 32<<10 || kind == "image" && requestBytes > 7<<20 {
			return nil, errors.New("AI 请求超出大小限制")
		}
		amount := cfg.Text
		if kind == "image" {
			amount = cfg.Image
		}
		tx, err := a.DB.Begin(ctx)
		if err != nil {
			return nil, errors.New("AI 额度服务不可用")
		}
		defer tx.Rollback(ctx)
		var locked bool
		if err = tx.QueryRow(ctx, "SELECT id FROM ai_allowance_lock WHERE id=true FOR UPDATE").Scan(&locked); err != nil {
			return nil, errors.New("AI 额度锁不可用")
		}
		var valid bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs j JOIN members m ON m.household_id=j.household_id AND m.user_id=j.created_by
 WHERE j.id=$1 AND j.household_id=$2 AND j.created_by=$3 AND j.lease_token=$4 AND j.lease_until>now() AND j.status='running' AND NOT j.cancel_requested AND m.role IN ('owner','editor'))`, j.ID, j.Household, j.Creator, j.Token).Scan(&valid)
		if err != nil || !valid {
			return nil, errors.New("AI 任务权限或租约已失效")
		}
		var used, house int64
		var daily, active int
		err = tx.QueryRow(ctx, `SELECT
 COALESCE(sum(ceiling_milli) FILTER (WHERE month=date_trunc('month',now() AT TIME ZONE 'Asia/Shanghai')::date),0),
 COALESCE(sum(ceiling_milli) FILTER (WHERE month=date_trunc('month',now() AT TIME ZONE 'Asia/Shanghai')::date AND household_id=$1),0),
 count(*) FILTER (WHERE (created_at AT TIME ZONE 'Asia/Shanghai')::date=(now() AT TIME ZONE 'Asia/Shanghai')::date),
 count(*) FILTER (WHERE state IN ('pending','uncertain')) FROM ai_call_allowances`, j.Household).Scan(&used, &house, &daily, &active)
		if err != nil {
			return nil, errors.New("AI 额度查询失败")
		}
		if used > cfg.Monthly-amount || house > cfg.Monthly-amount || daily >= cfg.Daily || active >= cfg.Concurrent {
			return nil, errors.New("AI 预扣额度、每日次数或并发上限已达到；待核查请求不会自动释放")
		}
		if _, err = tx.Exec(ctx, `INSERT INTO ai_call_allowances(job_id,household_id,kind,month,ceiling_milli,state) VALUES($1,$2,$3,date_trunc('month',now() AT TIME ZONE 'Asia/Shanghai')::date,$4,'pending')`, j.ID, j.Household, kind, amount); err != nil {
			return nil, errors.New("AI 任务已预扣或无法预扣；不会重复发送")
		}
		if tx.Commit(ctx) != nil {
			return nil, errors.New("AI 预扣提交失败；调用已阻止")
		}
		return func(knownComplete bool) {
			finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			state := "uncertain"
			if knownComplete {
				state = "completed"
			}
			// A crash or settlement failure retains the slot and full debit. No refunds.
			_, _ = a.DB.Exec(finalCtx, "UPDATE ai_call_allowances SET state=$1,finished_at=now() WHERE job_id=$2 AND state='pending'", state, j.ID)
		}, nil
	}
}
