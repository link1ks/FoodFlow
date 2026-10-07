package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"foodflow/internal/agent"
	"foodflow/internal/core"
	"foodflow/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const maxImageBytes = 5 << 20

type imagePayload struct {
	Key  string `json:"key"`
	MIME string `json:"mime"`
}

func imageStore() (storage.Store, error) {
	if os.Getenv("STORAGE_BACKEND") == "s3" {
		return storage.NewS3(os.Getenv("S3_ENDPOINT"), os.Getenv("S3_REGION"), os.Getenv("S3_BUCKET"), os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"))
	}
	if v := os.Getenv("STORAGE_BACKEND"); v != "" && v != "local" {
		return nil, fmt.Errorf("unsupported storage backend")
	}
	root := os.Getenv("IMAGE_STORAGE_DIR")
	if root == "" {
		root = "data/images"
	}
	return storage.Local{Root: root}, nil
}

func visionConfigured() bool {
	return agent.VisionFromEnv().Configured() && paidAIEnabled()
}

func readUploadedImage(c *gin.Context) ([]byte, string, string, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImageBytes+(1<<20))
	file, e := c.FormFile("image")
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if e != nil || file.Size <= 0 || file.Size > maxImageBytes {
		return nil, "", "", errors.New("image must be JPEG, PNG or WebP and at most 5 MiB")
	}
	r, e := file.Open()
	if e != nil {
		return nil, "", "", errors.New("image unavailable")
	}
	defer r.Close()
	data, e := io.ReadAll(io.LimitReader(r, maxImageBytes+1))
	if e != nil || len(data) == 0 || len(data) > maxImageBytes {
		return nil, "", "", errors.New("image exceeds 5 MiB")
	}
	mime := http.DetectContentType(data)
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[mime]
	if ext == "" {
		return nil, "", "", errors.New("unsupported image format")
	}
	return data, mime, ext, nil
}

func (a *App) enqueueImage(c *gin.Context) {
	if !writable(c) {
		return
	}
	if !visionConfigured() {
		fail(c, http.StatusServiceUnavailable, "image recognition requires a configured vision model; use manual entry")
		return
	}
	store, e := imageStore()
	if e != nil {
		fail(c, 503, "image storage unavailable")
		return
	}
	data, mime, ext, e := readUploadedImage(c)
	if e != nil {
		fail(c, 400, e.Error())
		return
	}
	id := core.ID()
	key := hid(c) + "/" + id + ext
	if e = store.Put(c, key, bytes.NewReader(data), mime); e != nil {
		fail(c, 500, "image storage failed")
		return
	}
	_, e = a.DB.Exec(c, "INSERT INTO jobs(id,household_id,kind,status,payload,created_by,max_attempts) VALUES($1,$2,'image','queued',$3,$4,1)", id, hid(c), core.JSON(imagePayload{key, mime}), uid(c))
	if e != nil {
		_ = store.Delete(c, key)
		fail(c, 500, "enqueue failed")
		return
	}
	c.JSON(202, gin.H{"id": id, "status": "queued"})
}

func (a *App) imagePreview(c *gin.Context) {
	var kind, status string
	var payload []byte
	e := a.DB.QueryRow(c, "SELECT kind,status,payload FROM jobs WHERE id=$1 AND household_id=$2", c.Param("job"), hid(c)).Scan(&kind, &status, &payload)
	if e != nil || kind != "image" || status != "awaiting_confirmation" {
		fail(c, 404, "image unavailable")
		return
	}
	var image imagePayload
	if json.Unmarshal(payload, &image) != nil || !strings.HasPrefix(image.Key, hid(c)+"/") {
		fail(c, 404, "image unavailable")
		return
	}
	store, e := imageStore()
	if e != nil {
		fail(c, 503, "image storage unavailable")
		return
	}
	r, e := store.Open(c, image.Key)
	if e != nil {
		fail(c, 404, "image unavailable")
		return
	}
	defer r.Close()
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(200, -1, image.MIME, r, nil)
}

type imageConfirmation struct {
	Name      string `json:"name"`
	Category  string `json:"category"`
	Unit      string `json:"unit"`
	Quantity  string `json:"quantity"`
	Location  string `json:"location"`
	BoughtOn  string `json:"bought_on"`
	ExpiresOn string `json:"expires_on"`
}

func (a *App) confirmImage(c *gin.Context) {
	if !writable(c) {
		return
	}
	var x imageConfirmation
	if !input(c, &x) {
		return
	}
	x.Name = strings.TrimSpace(x.Name)
	x.Category = strings.TrimSpace(x.Category)
	if x.Name == "" || len([]rune(x.Name)) > 100 || len([]rune(x.Category)) > 60 || len([]rune(x.Location)) > 100 {
		fail(c, 400, "invalid ingredient details")
		return
	}
	dim, _, e := core.Dimension(x.Unit)
	if e != nil {
		fail(c, 400, "unsupported unit")
		return
	}
	q, e := core.Quantity(x.Quantity)
	if e != nil {
		fail(c, 400, e.Error())
		return
	}
	for _, d := range []string{x.BoughtOn, x.ExpiresOn} {
		if d != "" {
			if _, e = time.Parse("2006-01-02", d); e != nil {
				fail(c, 400, "invalid date")
				return
			}
		}
	}
	jobID := c.Param("job")
	a.idem(c, gin.H{"job": jobID, "confirmation": x}, func(tx pgx.Tx) (any, error) {
		var status, kind string
		var result []byte
		e := tx.QueryRow(c, "SELECT kind,status,result FROM jobs WHERE id=$1 AND household_id=$2 FOR UPDATE", jobID, hid(c)).Scan(&kind, &status, &result)
		if e != nil || kind != "image" {
			return nil, conflict("image job unavailable")
		}
		if status != "awaiting_confirmation" {
			return nil, conflict("image job is not awaiting confirmation")
		}
		var suggestion agent.ImageSuggestion
		if e = json.Unmarshal(result, &suggestion); e != nil || suggestion.Name == "" {
			return nil, conflict("recognition result unavailable")
		}
		ingredient := core.ID()
		e = tx.QueryRow(c, "INSERT INTO ingredients(id,household_id,name,category,unit,dimension) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(household_id,name,unit) DO UPDATE SET category=EXCLUDED.category,archived_at=NULL RETURNING id", ingredient, hid(c), x.Name, x.Category, x.Unit, dim).Scan(&ingredient)
		if e != nil {
			return nil, e
		}
		batch := core.ID()
		expiryKind := "unknown"
		if x.ExpiresOn != "" {
			expiryKind = "user"
		}
		_, e = tx.Exec(c, "INSERT INTO batches(id,household_id,ingredient_id,quantity_milli,location,bought_on,expires_on,expiry_kind,source) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')::date,NULLIF($7,'')::date,$8,$9)", batch, hid(c), ingredient, q, x.Location, x.BoughtOn, x.ExpiresOn, expiryKind, "图片识别后用户确认")
		if e == nil {
			_, e = tx.Exec(c, "INSERT INTO stock_ledger(id,household_id,batch_id,delta_milli,reason,actor_id,ref_type,ref_id) VALUES($1,$2,$3,$4,'manual',$5,'image_job',$6)", core.ID(), hid(c), batch, q, uid(c), jobID)
		}
		if e == nil {
			_, e = tx.Exec(c, "UPDATE jobs SET status='succeeded',updated_at=now() WHERE id=$1", jobID)
		}
		if e == nil {
			_, e = tx.Exec(c, "INSERT INTO job_events(job_id,event) VALUES($1,'succeeded')", jobID)
		}
		return gin.H{"ingredient_id": ingredient, "batch_id": batch}, e
	})
	if c.Writer.Status() == 200 {
		var payload []byte
		if a.DB.QueryRow(c, "SELECT payload FROM jobs WHERE id=$1 AND household_id=$2 AND kind='image' AND status='succeeded'", jobID, hid(c)).Scan(&payload) == nil {
			var image imagePayload
			if json.Unmarshal(payload, &image) == nil && strings.HasPrefix(image.Key, hid(c)+"/") {
				if store, err := imageStore(); err == nil {
					_ = store.Delete(c, image.Key)
				}
			}
		}
	}
}

func (a *App) recognizeImage(ctx context.Context, j claimed) error {
	var payload imagePayload
	if e := json.Unmarshal(j.Payload, &payload); e != nil || payload.Key == "" || !strings.HasPrefix(payload.Key, j.Household+"/") {
		return errors.New("invalid image job payload")
	}
	var role string
	if e := a.DB.QueryRow(ctx, "SELECT role FROM members WHERE household_id=$1 AND user_id=$2", j.Household, j.Creator).Scan(&role); e != nil || role == "viewer" {
		return errors.New("image tool permission denied")
	}
	store, e := imageStore()
	if e != nil {
		return e
	}
	r, e := store.Open(ctx, payload.Key)
	if e != nil {
		return e
	}
	data, e := io.ReadAll(io.LimitReader(r, maxImageBytes+1))
	r.Close()
	if e != nil || len(data) == 0 || len(data) > maxImageBytes || http.DetectContentType(data) != payload.MIME {
		return errors.New("stored image is invalid")
	}
	_, _ = a.DB.Exec(ctx, "INSERT INTO job_events(job_id,event,detail) VALUES($1,'tools_read',$2)", j.ID, core.JSON(gin.H{"tools": []string{"image_storage_read"}}))
	model := agent.VisionFromEnv()
	model.Guard = a.allowanceGuard(j, "image")
	suggestion, e := model.Recognize(ctx, data, payload.MIME)
	if e != nil {
		return e
	}
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var cancelled bool
	if e = tx.QueryRow(ctx, "SELECT cancel_requested FROM jobs WHERE id=$1 AND lease_token=$2 AND status='running' AND lease_until>now() FOR UPDATE", j.ID, j.Token).Scan(&cancelled); e != nil || cancelled {
		return errors.New("image job cancelled or lease lost")
	}
	_, e = tx.Exec(ctx, "UPDATE jobs SET status='awaiting_confirmation',result=$1,progress=100,lease_owner=NULL,lease_token=NULL,lease_until=NULL,updated_at=now() WHERE id=$2 AND lease_token=$3", core.JSON(suggestion), j.ID, j.Token)
	if e == nil {
		_, e = tx.Exec(ctx, "INSERT INTO job_events(job_id,event) VALUES($1,'awaiting_confirmation')", j.ID)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	return e
}
