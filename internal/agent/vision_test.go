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
			MaxTokens int  `json:"max_tokens"`
			Thinking  bool `json:"enable_thinking"`
			Messages  []struct {
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if e := json.NewDecoder(r.Body).Decode(&request); e != nil || len(request.Messages) != 2 || !strings.Contains(string(request.Messages[1].Content), "data:image/png;base64,") {
			t.Errorf("image not sent as data URL: %v", e)
		}
		if len(request.Messages) == 2 && (!strings.Contains(string(request.Messages[0].Content), "Simplified Chinese") || !strings.Contains(string(request.Messages[1].Content), "简体中文")) {
			t.Error("ingredient draft request must specify the Chinese UI language")
		}
		if request.MaxTokens != 120 || request.Thinking || !strings.Contains(string(request.Messages[1].Content), `"max_pixels":1048576`) {
			t.Error("vision latency/input bounds missing")
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"scene\":\"photo\",\"foods\":[\"番茄\"]}"}}]}`))
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

func TestVisionRejectsUnrelatedUncertainAndMixedResults(t *testing.T) {
	for _, raw := range []string{
		`{"scene":"text","foods":["番茄"]}`,
		`{"scene":"drawing","foods":["番茄"]}`,
		`{"scene":"photo","foods":["番茄","黄瓜"]}`,
		`{"scene":"photo","foods":[]}`,
		`{"scene":"photo","foods":["番茄"],"quantity":5}`,
		`{"name":"椅子","category":"家具"}`,
		`{"decision":"not_ingredient","name":"椅子","category":"家具"}`,
		`{"decision":"uncertain","name":"番茄","category":"蔬菜"}`,
		`{"decision":"multiple","name":"番茄和鸡蛋","category":"食材"}`,
		`{"decision":"ingredient","name":"番茄","category":"蔬菜","quantity":5}`,
	} {
		if _, err := ParseImageSuggestion(raw); err == nil {
			t.Fatalf("unsafe image answer accepted: %s", raw)
		}
	}
}
