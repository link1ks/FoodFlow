package agent

import (
	"context"
	"errors"
	"github.com/cloudwego/eino/schema"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestCallGuardBlocksNetworkAndRedirectsCannotRepeatPaidRequests(t *testing.T) {
	var calls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer destination.Close()
	deny := func(context.Context, int) (func(bool), error) { return nil, errors.New("fixture allowance denied") }
	if _, err := (OpenAIModel{Endpoint: destination.URL, Name: "fixture", Key: "fixture", Guard: deny}).Generate(context.Background(), []*schema.Message{schema.UserMessage("fixture")}); err == nil || calls.Load() != 0 {
		t.Fatal("guard did not prevent text network call")
	}
	if _, err := (VisionModel{Endpoint: destination.URL, Name: "fixture", Key: "fixture", Guard: deny}).Recognize(context.Background(), []byte("fixture"), "image/png"); err == nil || calls.Load() != 0 {
		t.Fatal("guard did not prevent vision network call")
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL+"/chat/completions", http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	completed := false
	allow := func(context.Context, int) (func(bool), error) { return func(known bool) { completed = known }, nil }
	if _, err := (OpenAIModel{Endpoint: redirect.URL, Name: "fixture", Key: "fixture", Guard: allow}).Generate(context.Background(), nil); err == nil || calls.Load() != 0 || !completed {
		t.Fatal("redirect repeated paid call or settlement differs")
	}
	closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	endpoint := closed.URL
	closed.Close()
	completed = true
	if _, err := (OpenAIModel{Endpoint: endpoint, Name: "fixture", Key: "fixture", Guard: allow}).Generate(context.Background(), nil); err == nil || completed {
		t.Fatal("network failure treated as known complete")
	}
}
