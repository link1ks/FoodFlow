package agent

import (
	"context"
	"os"
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
	// Resolve before removing PATH. CPython uses argv[0] to populate
	// sys.executable, which the CLI regression uses to start another interpreter.
	interpreter, err := exec.LookPath(pythonExecutable())
	if err != nil {
		t.Fatal(err)
	}
	interpreter, err = filepath.Abs(interpreter)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, interpreter, "-I", "-B",
		filepath.Join(filepath.Dir(file), "..", "..", "scripts", "test-data-tools.py"), suite)
	cmd.Env = pythonEnvironment()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Offline data contracts failed: %v\n%s", err, output)
	}
}

func TestPythonNutritionImportContracts(t *testing.T) {
	// Exercise a bare executable name even on machines with a project venv.
	// The child still receives only pythonEnvironment(), without PATH.
	interpreter, err := exec.LookPath(pythonExecutable())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(interpreter)+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FOODFLOW_PYTHON", filepath.Base(interpreter))
	runDataToolContracts(t, "NutritionTests")
}

func TestPythonEvaluationAnalysisContracts(t *testing.T) {
	runDataToolContracts(t, "EvaluationTests")
}
