package app

import (
	"bytes"
	"context"
	"encoding/json"
	"foodflow/internal/core"
	"foodflow/internal/engine/intake"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestVoiceS3CorrectionRequiresReviewAndCleansAudio(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	var mu sync.Mutex
	objects := map[string][]byte{}
	operations := map[string]int{}
	objectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		operations[r.Method]++
		switch r.Method {
		case http.MethodPut:
			objects[r.URL.Path], _ = io.ReadAll(r.Body)
		case http.MethodGet:
			data, ok := objects[r.URL.Path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(data)
		case http.MethodDelete:
			delete(objects, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer objectServer.Close()
	modelCalls := 0
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		modelCalls++
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"鸡蛋三个，不对，鸡蛋四个。"}}]}`))
	}))
	defer model.Close()
	for key, value := range map[string]string{
		"REDIS_URL": testRedis(t), "STORAGE_BACKEND": "s3", "S3_ENDPOINT": objectServer.URL,
		"S3_REGION": "fixture", "S3_BUCKET": "fixture", "S3_ACCESS_KEY": "fixture", "S3_SECRET_KEY": "fixture",
		"SPEECH_MODEL_ENDPOINT": model.URL, "SPEECH_MODEL_NAME": "fixture", "SPEECH_MODEL_API_KEY": "fixture",
		"AI_ALLOWANCE_ENABLED": "true", "AI_MONTHLY_ALLOWANCE_CNY": "5",
	} {
		t.Setenv(key, value)
	}
	a, err := NewAPI(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	server := httptest.NewServer(a.Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "password123", "name": "Review fixture"})
	must(t, code, 200, v)
	token := get(v, "token")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "Review fixture", "servings": 2})
	must(t, code, 201, v)
	root := "/households/" + get(v, "id")
	wav := make([]byte, 80)
	copy(wav, "RIFF")
	copy(wav[8:], "WAVE")
	key := core.ID()
	upload := func() string {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, _ := writer.CreateFormFile("audio", "review.wav")
		_, _ = part.Write(wav)
		_ = writer.WriteField("consent", "true")
		_ = writer.Close()
		req, _ := http.NewRequest("POST", server.URL+"/api"+root+"/jobs/voice", &body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var result map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&result)
		must(t, resp.StatusCode, 200, result)
		return get(result, "id")
	}
	job := upload()
	if upload() != job {
		t.Fatal("replay created another job")
	}
	j, err := a.claim(ctx, "review-worker")
	if err != nil || j.ID != job {
		t.Fatal("claim failed")
	}
	a.runJob(ctx, j, "review-worker")
	var status string
	var raw []byte
	if err := pool.QueryRow(ctx, "SELECT status,result FROM jobs WHERE id=$1", job).Scan(&status, &raw); err != nil {
		t.Fatal(err)
	}
	var result intake.Result
	if json.Unmarshal(raw, &result) != nil || status != "awaiting_confirmation" || len(result.Items) != 2 {
		t.Fatal("missing editable correction draft")
	}
	items := []map[string]any{}
	for _, item := range result.Items {
		if item.Quantity != "" || item.Warning == "" {
			t.Fatal("correction quantity prefilled")
		}
		items = append(items, map[string]any{"catalog_id": item.CatalogID, "quantity": item.Quantity})
	}
	code, v = h.call("POST", root+"/jobs/"+job+"/voice/confirm", token, core.ID(), map[string]any{"confirm": true, "items": items})
	must(t, code, 400, v)
	var batches, ledger, outbox int
	if err := pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM batches),(SELECT count(*) FROM stock_ledger),(SELECT count(*) FROM stock_outbox)").Scan(&batches, &ledger, &outbox); err != nil || batches != 0 || ledger != 0 || outbox != 0 {
		t.Fatal("unresolved corrections mutated inventory")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(objects) != 0 || operations["PUT"] != 1 || operations["GET"] != 1 || operations["DELETE"] != 1 || modelCalls != 1 {
		t.Fatal("S3 cleanup or same-key/model replay behavior changed")
	}
}
