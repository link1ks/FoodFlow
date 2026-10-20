package agent

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func runDataToolContracts(t *testing.T, suite string) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, pythonExecutable(), "-I", "-B",
		filepath.Join(filepath.Dir(file), "..", "..", "scripts", "test-data-tools.py"), suite)
	cmd.Env = pythonEnvironment()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Offline data contracts failed: %v\n%s", err, output)
	}
}

func TestPythonNutritionImportContracts(t *testing.T) {
	runDataToolContracts(t, "NutritionTests")
}

func TestPythonEvaluationAnalysisContracts(t *testing.T) {
	runDataToolContracts(t, "EvaluationTests")
}
