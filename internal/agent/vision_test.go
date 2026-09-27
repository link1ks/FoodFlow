package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVisionAdapterAndSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("invalid vision request")
		}
		var request struct {
			Messages []struct {
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if e := json.NewDecoder(r.Body).Decode(&request); e != nil || len(request.Messages) != 2 || !strings.Contains(string(request.Messages[1].Content), "data:image/png;base64,") {
			t.Errorf("image not sent as data URL: %v", e)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"name\":\"番茄\",\"category\":\"蔬菜\"}"}}]}`))
	}))
	defer server.Close()
	result, e := (VisionModel{Endpoint: server.URL, Name: "vision", Key: "test-key"}).Recognize(context.Background(), []byte("png"), "image/png")
	if e != nil || result.Name != "番茄" {
		t.Fatalf("unexpected suggestion %+v: %v", result, e)
	}
	for _, raw := range []string{`{"name":"番茄","quantity":"2"}`, `{"name":""}`, `{"name":"番茄"}{"name":"土豆"}`} {
		if _, e := ParseImageSuggestion(raw); e == nil {
			t.Fatalf("unsafe result accepted: %s", raw)
		}
	}
}
