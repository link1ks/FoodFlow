package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"foodflow/internal/agent"
	"foodflow/internal/core"
	"foodflow/internal/engine/intake"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"strings"
	"time"
)

func speechConfigured() bool { return agent.SpeechFromEnv().Configured() && paidAIEnabled() }

func audioFormat(data []byte) (string, string, error) {
	switch {
	case len(data) >= 44 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE":
		return "audio/wav", ".wav", nil
	case len(data) > 4 && bytes.Equal(data[:4], []byte{0x1a, 0x45, 0xdf, 0xa3}) && bytes.Contains(data[:min(len(data), 4096)], []byte("webm")):
		return "audio/webm", ".webm", nil
	case len(data) > 4 && string(data[:4]) == "OggS":
		return "audio/ogg", ".ogg", nil
	case len(data) > 3 && (string(data[:3]) == "ID3" || (data[0] == 0xff && data[1]&0xe0 == 0xe0)):
		return "audio/mpeg", ".mp3", nil
	default:
		return "", "", errors.New("请上传 WAV、WebM、Ogg 或 MP3 录音，最大 5 MiB")
	}
}

func (a *App) enqueueVoice(c *gin.Context) {
	if !writable(c) {
		return
	}
	if !speechConfigured() {
		fail(c, 503, "语音模型未配置或收费额度保护未启用，请使用食材目录")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agent.MaxSpeechBytes+(1<<20))
	file, err := c.FormFile("audio")
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if err != nil || file.Size <= 0 || file.Size > agent.MaxSpeechBytes {
		fail(c, 400, "录音不能为空且不能超过 5 MiB")
		return
	}
	if c.PostForm("consent") != "true" {
		fail(c, 400, "请明确同意将录音发送至语音模型")
		return
	}
	r, err := file.Open()
	if err != nil {
		fail(c, 400, "无法读取录音")
		return
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, agent.MaxSpeechBytes+1))
	if err != nil || len(data) > agent.MaxSpeechBytes {
		fail(c, 400, "录音大小无效")
		return
	}
	mime, ext, err := audioFormat(data)
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	store, err := imageStore()
	if err != nil {
		fail(c, 503, "录音暂存不可用")
		return
	}
	id := core.ID()
	key := hid(c) + "/voice-" + id + ext
	digest := core.Hash(string(data))
	written := false
	a.idem(c, gin.H{"audio_digest": digest, "mime": mime, "consent": true}, func(tx pgx.Tx) (any, error) {
		if err := store.Put(c, key, bytes.NewReader(data), mime); err != nil {
			return nil, errors.New("录音暂存失败")
		}
		written = true
		_, err := tx.Exec(c, "INSERT INTO jobs(id,household_id,kind,status,payload,created_by,max_attempts) VALUES($1,$2,'voice','queued',$3,$4,1)", id, hid(c), core.JSON(imagePayload{Key: key, MIME: mime}), uid(c))
		return gin.H{"id": id, "status": "queued"}, err
	})
	if written && c.Writer.Status() != 200 {
		// An ambiguous commit may have created the job: keep its input if so.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var exists bool
		if a.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM jobs WHERE id=$1)", id).Scan(&exists) == nil && !exists {
			_ = store.Delete(ctx, key)
		}
	}
}

func (a *App) recognizeVoice(ctx context.Context, j claimed) error {
	var p imagePayload
	if json.Unmarshal(j.Payload, &p) != nil || !strings.HasPrefix(p.Key, j.Household+"/voice-"+j.ID+".") {
		return errors.New("录音任务输入无效")
	}
	store, err := imageStore()
	if err != nil {
		return errors.New("录音暂存不可用")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = store.Delete(cleanup, p.Key)
	}()
	var allowed bool
	if err = a.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM members m JOIN households h ON h.id=m.household_id WHERE m.household_id=$1 AND m.user_id=$2 AND m.role IN ('owner','editor') AND (h.kind='family' OR h.owner_id=m.user_id))`, j.Household, j.Creator).Scan(&allowed); err != nil || !allowed {
		return errors.New("语音任务权限已失效")
	}
	r, err := store.Open(ctx, p.Key)
	if err != nil {
		return errors.New("录音已不可用，请重新录音")
	}
	data, err := io.ReadAll(io.LimitReader(r, agent.MaxSpeechBytes+1))
	r.Close()
	mime, _, formatErr := audioFormat(data)
	if err != nil || len(data) > agent.MaxSpeechBytes || formatErr != nil || mime != p.MIME {
		return errors.New("暂存录音无效")
	}
	model := agent.SpeechFromEnv()
	model.Guard = a.allowanceGuard(j, "speech")
	text, err := model.Transcribe(ctx, data, p.MIME)
	if err != nil {
		return err
	}
	rows, err := a.DB.Query(ctx, "SELECT id,name,default_unit,aliases FROM ingredient_catalog ORDER BY id")
	if err != nil {
		return errors.New("食材目录暂不可用")
	}
	catalog := []intake.CatalogItem{}
	for rows.Next() {
		var item intake.CatalogItem
		if err = rows.Scan(&item.ID, &item.Name, &item.Unit, &item.Aliases); err != nil {
			break
		}
		catalog = append(catalog, item)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return errors.New("食材目录读取失败")
	}
	result := intake.Parse(text, catalog)
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var valid bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM members m JOIN households h ON h.id=m.household_id WHERE m.household_id=$1 AND m.user_id=$2 AND m.role IN ('owner','editor') AND (h.kind='family' OR h.owner_id=m.user_id))`, j.Household, j.Creator).Scan(&valid); err != nil || !valid {
		return errors.New("语音任务权限已失效")
	}
	var cancelled bool
	if err = tx.QueryRow(ctx, "SELECT cancel_requested FROM jobs WHERE id=$1 AND lease_token=$2 AND status='running' AND lease_until>now() FOR UPDATE", j.ID, j.Token).Scan(&cancelled); err != nil || cancelled {
		return errors.New("语音任务已取消或租约失效")
	}
	_, err = tx.Exec(ctx, "UPDATE jobs SET status='awaiting_confirmation',result=$1,progress=100,lease_owner=NULL,lease_token=NULL,lease_until=NULL,updated_at=now() WHERE id=$2", core.JSON(result), j.ID)
	if err == nil {
		_, err = tx.Exec(ctx, "INSERT INTO job_events(job_id,event) VALUES($1,'awaiting_confirmation')", j.ID)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	return err
}

type voiceItem struct {
	CatalogID string `json:"catalog_id"`
	Quantity  string `json:"quantity"`
	Location  string `json:"location"`
	BoughtOn  string `json:"bought_on"`
	ExpiresOn string `json:"expires_on"`
}

func (a *App) confirmVoice(c *gin.Context) {
	if !writable(c) {
		return
	}
	var request struct {
		Confirm bool        `json:"confirm"`
		Items   []voiceItem `json:"items"`
	}
	if !input(c, &request) {
		return
	}
	if !request.Confirm || len(request.Items) < 1 || len(request.Items) > 20 {
		fail(c, 400, "请核对并确认 1–20 项食材")
		return
	}
	quantities := make([]int64, len(request.Items))
	for i, item := range request.Items {
		q, err := core.Quantity(item.Quantity)
		if err != nil || item.CatalogID == "" || len([]rune(item.Location)) > 100 {
			fail(c, 400, "请补齐食材、正数数量并核对位置")
			return
		}
		quantities[i] = q
		for _, date := range []string{item.BoughtOn, item.ExpiresOn} {
			if date != "" {
				if _, err = time.Parse("2006-01-02", date); err != nil {
					fail(c, 400, "日期无效")
					return
				}
			}
		}
	}
	job := c.Param("job")
	a.idem(c, gin.H{"job": job, "confirmation": request}, func(tx pgx.Tx) (any, error) {
		var status string
		if err := tx.QueryRow(c, "SELECT status FROM jobs WHERE id=$1 AND household_id=$2 AND kind='voice' FOR UPDATE", job, hid(c)).Scan(&status); err != nil || status != "awaiting_confirmation" {
			return nil, conflict("语音草稿已处理或不可用")
		}
		results := []gin.H{}
		for i, item := range request.Items {
			var name, category, unit string
			if err := tx.QueryRow(c, "SELECT name,category,default_unit FROM ingredient_catalog WHERE id=$1", item.CatalogID).Scan(&name, &category, &unit); err != nil {
				return nil, bad("目录食材不可用")
			}
			dim, _, err := core.Dimension(unit)
			if err != nil {
				return nil, err
			}
			ingredient := core.ID()
			err = tx.QueryRow(c, "INSERT INTO ingredients(id,household_id,name,category,unit,dimension,catalog_id) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(household_id,name,unit) DO UPDATE SET category=EXCLUDED.category,catalog_id=EXCLUDED.catalog_id,archived_at=NULL RETURNING id", ingredient, hid(c), name, category, unit, dim, item.CatalogID).Scan(&ingredient)
			if err != nil {
				return nil, err
			}
			batch := core.ID()
			expiryKind := "unknown"
			if item.ExpiresOn != "" {
				expiryKind = "user"
			}
			_, err = tx.Exec(c, "INSERT INTO batches(id,household_id,ingredient_id,quantity_milli,location,bought_on,expires_on,expiry_kind,source) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')::date,NULLIF($7,'')::date,$8,'语音识别后用户确认')", batch, hid(c), ingredient, quantities[i], item.Location, item.BoughtOn, item.ExpiresOn, expiryKind)
			if err != nil {
				return nil, err
			}
			_, err = tx.Exec(c, "INSERT INTO stock_ledger(id,household_id,batch_id,delta_milli,reason,actor_id,ref_type,ref_id) VALUES($1,$2,$3,$4,'manual',$5,'voice_job',$6)", core.ID(), hid(c), batch, quantities[i], uid(c), job)
			if err != nil {
				return nil, err
			}
			results = append(results, gin.H{"ingredient_id": ingredient, "batch_id": batch})
		}
		if _, err := tx.Exec(c, "UPDATE jobs SET status='succeeded',updated_at=now() WHERE id=$1", job); err != nil {
			return nil, err
		}
		_, err := tx.Exec(c, "INSERT INTO job_events(job_id,event) VALUES($1,'succeeded')", job)
		return gin.H{"items": results}, err
	})
}
