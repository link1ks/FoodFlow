package agent

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestPythonRAGIndexAndGroundingContracts(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, pythonExecutable(), "-I", "-B", filepath.Join(filepath.Dir(file), "python", "test_rag.py"))
	cmd.Env = pythonEnvironment()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Python RAG regression: %v\n%s", err, output)
	}
}

func TestPythonRAGBatchBudgetContracts(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, pythonExecutable(), "-I", "-B", filepath.Join(filepath.Dir(file), "python", "test_rag_batch.py"))
	cmd.Env = pythonEnvironment()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Python batch budget regression: %v\n%s", err, output)
	}
}
