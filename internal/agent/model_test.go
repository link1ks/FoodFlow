package agent

import (
	"context"
	"github.com/cloudwego/eino/schema"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDemoAndSchema(t *testing.T) {
	m := DemoModel{RecipeID: "abc"}
	msg, e := m.Generate(context.Background(), nil)
	if e != nil {
		t.Fatal(e)
	}
	id, e := ParseRecipeID(msg.Content)
	if e != nil || id != "abc" {
		t.Fatal(id, e)
	}
	if _, e = ParseRecipeID(`{"recipe_id":"abc","inventory":999}`); e == nil {
		t.Fatal("unknown field accepted")
	}
	if _, e = ParseRecipeID(`{"recipe_id":"abc"}{"recipe_id":"def"}`); e == nil {
		t.Fatal("multiple objects accepted")
	}
}
func TestOpenAIAdapter(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("invalid model request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"recipe_id\":\"known\"}"}}]}`))
	}))
	defer s.Close()
	m := OpenAIModel{Endpoint: s.URL, Name: "test-model", Key: "test-key"}
	answer, e := m.Generate(context.Background(), []*schema.Message{schema.UserMessage("pick")})
	if e != nil {
		t.Fatal(e)
	}
	id, e := ParseRecipeID(answer.Content)
	if e != nil || id != "known" {
		t.Fatal(id, e)
	}
}
