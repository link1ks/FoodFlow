package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPythonAgentContracts(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, pythonExecutable(), "-I", "-B", filepath.Join(filepath.Dir(file), "python", "test_runtime.py"))
	cmd.Env = pythonEnvironment()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Python offline contracts failed: %v\n%s", err, output)
	}
}

func TestPythonAgentCancellationPreservesUncertainDebit(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
	}))
	defer server.Close()
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	known := true
	result := make(chan error, 1)
	go func() {
		_, err := (OpenAIModel{Endpoint: server.URL, Name: "fixture", Key: "fictional", Guard: func(context.Context, int) (func(bool), error) {
			return func(v bool) { known = v }, nil
		}}).Generate(ctx, nil)
		result <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("provider not reached")
	}
	started := time.Now()
	cancel()
	select {
	case err := <-result:
		if err == nil || known || time.Since(started) > 3*time.Second {
			t.Fatal("cancel did not preserve uncertainty or reap process", err, known)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("Python process did not exit on cancellation")
	}
}

func TestPythonAgentGuardMatchesExactUTF8RequestAndRejectsUnsafeRecipe(t *testing.T) {
	var expected atomic.Int64
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.ContentLength != expected.Load() {
			t.Error("guard charged a different request size", r.ContentLength, expected.Load())
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": `{"recipe_id":"invented"}`}}}})
	}))
	defer server.Close()
	known := false
	model := OpenAIModel{Endpoint: server.URL, Name: "fixture", Key: "fictional", Task: "plan",
		Context: map[string]any{"allowed_recipes": []map[string]string{{"ID": "known"}}, "user_preference": "番茄"},
		Guard: func(_ context.Context, n int) (func(bool), error) {
			expected.Store(int64(n))
			return func(v bool) { known = v }, nil
		}}
	if _, err := model.Generate(context.Background(), nil); err == nil || !known || calls.Load() != 1 {
		t.Fatal("unsafe selection accepted or debit state lost", err, known, calls.Load())
	}
	model.Guard = func(context.Context, int) (func(bool), error) { return nil, errors.New("denied") }
	if _, err := model.Generate(context.Background(), nil); err == nil || calls.Load() != 1 {
		t.Fatal("denied request reached provider")
	}
}

func TestPythonAgentEnvironmentDoesNotInheritApplicationSecrets(t *testing.T) {
	t.Setenv("DATABASE_URL", "private-fixture")
	t.Setenv("MODEL_API_KEY", "private-fixture")
	t.Setenv("PYTHONPATH", "private-fixture")
	t.Setenv("HTTP_PROXY", "private-fixture")
	for _, entry := range pythonEnvironment() {
		if strings.Contains(entry, "private-fixture") {
			t.Fatal("private application environment inherited")
		}
	}
}
