package app

import (
	"context"
	"errors"
	"foodflow/internal/agent"
	"foodflow/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"sort"
	"strings"
	"time"
)

// Both queues share the same policy lock, global money/daily/concurrency totals.
const paidCalls = `(SELECT household_id,month,ceiling_milli,state,created_at FROM ai_call_allowances
 UNION ALL SELECT NULL::uuid,month,ceiling_milli,state,created_at FROM community_review_calls) calls`

func normalizeCommunityIngredients(ctx context.Context, tx pgx.Tx, d *communityDraft) (string, error) {
	for i := range d.Ingredients {
		item := &d.Ingredients[i]
		var unit string
		if tx.QueryRow(ctx, "SELECT name,default_unit FROM ingredient_catalog WHERE id=$1", item.CatalogID).Scan(&item.Name, &unit) != nil {
			return "", communityInvalid("食材目录不存在")
		}
		quantity, _ := core.Quantity(item.Quantity)
		quantity, err := core.Convert(quantity, item.Unit, unit)
		if err != nil || quantity > 100000000 {
			return "", communityInvalid("食材单位或用量与目录不兼容，请使用 " + unit)
		}
		item.Unit = unit
		item.Quantity = core.Format(quantity)
	}
	items := append([]communityIngredient{}, d.Ingredients...)
	sort.Slice(items, func(i, j int) bool { return items[i].CatalogID < items[j].CatalogID })
	return core.Hash(string(core.JSON(struct {
		Items    []communityIngredient
		Steps    []string
		Servings int
	}{items, d.Steps, d.Servings}))), nil
}

func (a *App) communityEdit(c *gin.Context) {
	if !communityID(c.Param("post")) {
		fail(c, 400, "动态编号无效")
		return
	}
	var d communityDraft
	if !communityBind(c, &d) {
		return
	}
	if err := validateCommunityDraft(&d); err != nil {
		fail(c, 400, err.Error())
		return
	}
	if d.RemoveCover && d.CoverBase64 != "" {
		fail(c, 400, "请选择替换或移除封面")
		return
	}
	replacement, err := communityCover(d.CoverBase64)
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	a.communityWrite(c, d, func(tx pgx.Tx) (any, error) {
		var author, status, mime string
		var cover []byte
		var version int
		err := tx.QueryRow(c, "SELECT author_id::text,review_status,cover,cover_type,review_version FROM community_posts WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", c.Param("post")).Scan(&author, &status, &cover, &mime, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, communityFault{404, "动态不存在"}
		}
		if err != nil {
			return nil, err
		}
		if author != uid(c) {
			return nil, communityFault{403, "只能修改自己的待审动态"}
		}
		if status != "pending" && status != "rejected" && status != "unavailable" {
			return nil, communityFault{409, "已公开或正在审核的动态不能修改，请等待审核完成"}
		}
		fingerprint, err := normalizeCommunityIngredients(c, tx, &d)
		if err != nil {
			return nil, err
		}
		var duplicate bool
		if err = tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM community_posts WHERE author_id=$1 AND fingerprint=$2 AND id<>$3)", author, fingerprint, c.Param("post")).Scan(&duplicate); err != nil {
			return nil, err
		}
		if duplicate {
			return nil, communityFault{409, "这份配方已经分享过了"}
		}
		if d.RemoveCover {
			cover = nil
			mime = ""
		} else if len(replacement) > 0 {
			cover = replacement
			mime = "image/jpeg"
		}
		_, err = tx.Exec(c, `UPDATE community_posts SET title=$2,story=$3,servings=$4,minutes=$5,tags=$6,ingredients=$7,steps=$8,fingerprint=$9,cover=$10,cover_type=$11,
 review_status='pending',review_version=review_version+1,review_note='',review_token=NULL,review_until=NULL,reviewed_at=NULL,cover_reviewed=false WHERE id=$1`, c.Param("post"), d.Title, d.Story, d.Servings, d.Minutes, d.Tags, core.JSON(d.Ingredients), d.Steps, fingerprint, cover, mime)
		return gin.H{"id": c.Param("post"), "review_status": "pending", "review_version": version + 1}, err
	})
}

type communityClaim struct {
	id, author, token string
	version           int
	submission        agent.CommunitySubmission
}

func (a *App) communityReviewGuard(job communityClaim) agent.CallGuard {
	return func(ctx context.Context, requestBytes int) (func(bool), error) {
		cfg, err := loadAllowanceConfig()
		if err != nil || !cfg.Enabled {
			return nil, errors.New("review allowance unavailable")
		}
		kind, amount := "text", cfg.Text
		if len(job.submission.Cover) > 0 {
			kind, amount = "image", cfg.Image
		}
		if requestBytes > 7<<20 || kind == "text" && requestBytes > 96<<10 {
			return nil, errors.New("review request too large")
		}
		tx, err := a.DB.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, "SELECT id FROM ai_allowance_lock WHERE id=true FOR UPDATE"); err != nil {
			return nil, err
		}
		var valid bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM community_posts p JOIN users u ON u.id=p.author_id WHERE p.id=$1 AND p.author_id=$2 AND p.review_version=$3 AND p.review_token=$4 AND p.review_until>now() AND p.review_status='reviewing' AND p.deleted_at IS NULL AND u.merged_into IS NULL)`, job.id, job.author, job.version, job.token).Scan(&valid)
		if err != nil || !valid {
			return nil, errors.New("review permission or lease expired")
		}
		var used, personal int64
		var daily, active int
		err = tx.QueryRow(ctx, `SELECT COALESCE(sum(ceiling_milli) FILTER(WHERE month=date_trunc('month',now() AT TIME ZONE 'Asia/Shanghai')::date),0),
 count(*) FILTER(WHERE (created_at AT TIME ZONE 'Asia/Shanghai')::date=(now() AT TIME ZONE 'Asia/Shanghai')::date),count(*) FILTER(WHERE state IN ('pending','uncertain')) FROM `+paidCalls).Scan(&used, &daily, &active)
		if err != nil {
			return nil, err
		}
		if err = tx.QueryRow(ctx, "SELECT COALESCE(sum(ceiling_milli),0) FROM community_review_calls WHERE author_id=$1 AND month=date_trunc('month',now() AT TIME ZONE 'Asia/Shanghai')::date", job.author).Scan(&personal); err != nil {
			return nil, err
		}
		if used > cfg.Monthly-amount || personal > cfg.Monthly-amount || daily >= cfg.Daily || active >= cfg.Concurrent {
			return nil, errors.New("review allowance reached")
		}
		if _, err = tx.Exec(ctx, `INSERT INTO community_review_calls(post_id,version,author_id,kind,month,ceiling_milli,state) VALUES($1,$2,$3,$4,date_trunc('month',now() AT TIME ZONE 'Asia/Shanghai')::date,$5,'pending')`, job.id, job.version, job.author, kind, amount); err != nil {
			return nil, errors.New("review revision already dispatched")
		}
		if tx.Commit(ctx) != nil {
			return nil, errors.New("review dispatch reservation failed")
		}
		return func(complete bool) {
			finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			state := "uncertain"
			if complete {
				state = "completed"
			}
			_, _ = a.DB.Exec(finalCtx, "UPDATE community_review_calls SET state=$1,finished_at=now() WHERE post_id=$2 AND version=$3 AND state='pending'", state, job.id, job.version)
		}, nil
	}
}

// Process one committed immutable revision. Each call uses bounded network time
// outside DB transactions. A lease alone never permits an ambiguous paid resend.
func (a *App) ProcessCommunityReview(ctx context.Context) (bool, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE community_review_calls r SET state='uncertain' WHERE r.state='pending' AND EXISTS(SELECT 1 FROM community_posts p WHERE p.id=r.post_id AND p.review_version=r.version AND p.review_status='reviewing' AND p.review_until<now())`); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE community_posts p SET review_status=CASE WHEN EXISTS(SELECT 1 FROM community_review_calls r WHERE r.post_id=p.id AND r.version=p.review_version) THEN 'unavailable' ELSE 'pending' END,
 review_note='上次审核未能完成，内容尚未公开。请稍后重新提交或联系管理员。',review_token=NULL,review_until=NULL WHERE review_status='reviewing' AND review_until<now()`); err != nil {
		return false, err
	}
	var job communityClaim
	err = tx.QueryRow(ctx, `SELECT id::text,author_id::text,review_version,title,story,servings,minutes,tags,ingredients,steps,cover FROM community_posts WHERE review_status='pending' AND deleted_at IS NULL ORDER BY created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&job.id, &job.author, &job.version, &job.submission.Title, &job.submission.Story, &job.submission.Servings, &job.submission.Minutes, &job.submission.Tags, &job.submission.Ingredients, &job.submission.Steps, &job.submission.Cover)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, tx.Commit(ctx)
	}
	if err != nil {
		return false, err
	}
	job.token = core.ID()
	if _, err = tx.Exec(ctx, "UPDATE community_posts SET review_status='reviewing',review_token=$2,review_until=now()+interval '90 seconds' WHERE id=$1", job.id, job.token); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	reviewer := a.communityReviewer
	if reviewer == nil {
		reviewer = agent.CommunityReviewFromEnv(len(job.submission.Cover) > 0)
	}
	callCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	result, reviewErr := reviewer.Review(callCtx, job.submission, a.communityReviewGuard(job))
	cancel()
	// Validate even injected fixtures; only canonical verdicts can change visibility.
	if reviewErr == nil {
		result, reviewErr = agent.ParseCommunityReview(string(core.JSON(result)), len(job.submission.Cover) > 0)
	}
	status, note := "unavailable", "初审暂时不可用，内容尚未公开。请稍后重新提交；仍未恢复时联系管理员。"
	if reviewErr == nil {
		status = result.Decision
		if status == "approved" {
			note = "配方初审通过。初审不替代实际烹饪安全确认。"
			if result.ImageChecked {
				note = "配方与封面初审通过。初审不替代实际烹饪安全确认。"
			}
		} else {
			notes := []string{}
			for _, issue := range result.Issues {
				labels := map[string]string{"title": "标题", "story": "心得", "ingredients": "食材", "steps": "步骤", "cover": "封面"}
				notes = append(notes, labels[issue.Field]+"："+issue.Reason)
			}
			note = strings.Join(notes, "\n")
		}
	}
	finalCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer finishCancel()
	finishTx, err := a.DB.Begin(finalCtx)
	if err != nil {
		return true, err
	}
	defer finishTx.Rollback(finalCtx)
	if _, err = finishTx.Exec(finalCtx, "SELECT pg_advisory_xact_lock(733033)"); err != nil {
		return true, err
	}
	tag, err := finishTx.Exec(finalCtx, `UPDATE community_posts SET review_status=$5,review_note=$6,cover_reviewed=$7,reviewed_at=now(),review_token=NULL,review_until=NULL
 WHERE id=$1 AND review_version=$2 AND review_token=$3 AND author_id=$4 AND review_status='reviewing' AND review_until>now() AND deleted_at IS NULL`, job.id, job.version, job.token, job.author, status, note, reviewErr == nil && result.ImageChecked)
	if err != nil {
		return true, err
	}
	if tag.RowsAffected() > 0 && status == "approved" {
		if err = communityAward(finalCtx, finishTx, job.author, 5, "publish", "publish:"+job.id, 5); err != nil {
			return true, err
		}
	}
	return true, finishTx.Commit(finalCtx)
}

func (a *App) WorkCommunityReviews(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		processed, err := a.ProcessCommunityReview(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Error("community preliminary review failed")
		}
		if processed && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
