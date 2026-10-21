package agent

// The harness owns audit/provenance policy; Python owns all RAG computation.
import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"os/exec"
	"time"
)

//go:embed python/rag.py
var ragRuntime string

const RetrievalVersion = "python-sqlite-cosine-v2"

func RunRAG(request, result any) error {
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	if len(raw) > 256<<20 {
		return errors.New("RAG input exceeds bound")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, pythonExecutable(), "-I", "-B", "-c", ragRuntime)
	cmd.Env = pythonEnvironment()
	cmd.Stdin = bytes.NewReader(raw)
	output, err := cmd.Output()
	if err != nil {
		return errors.New("Python RAG unavailable or rejected index/request")
	}
	return json.Unmarshal(output, result)
}
