package main

import (
	"strings"
	"testing"
)

func TestDiagnosticOutputIsBounded(t *testing.T) {
	var buffer boundedOutput
	if n, err := buffer.Write([]byte(strings.Repeat("a", 100000))); n != 100000 || err != nil {
		t.Fatal("writer must acknowledge all subprocess bytes")
	}
	buffer.Write([]byte("last failure"))
	if len(buffer.data) > 64<<10 || !strings.HasSuffix(string(buffer.data), "last failure") {
		t.Fatal("diagnostic tail is unbounded or loses the last error")
	}
}

func TestImportRules(t *testing.T) {
	for _, x := range []struct {
		path, value string
		reject      bool
	}{
		{"internal/engine/matcher/x.go", "github.com/jackc/pgx/v5", true},
		{"internal/core/x.go", "net/http", true},
		{"internal/engine/scheduler/x.go", "foodflow/internal/core", false},
		{"internal/insights/x.go", "foodflow/internal/dbgen", true},
		{"internal/insights/x.go", "foodflow/internal/events", false},
		{"internal/app/x.go", "github.com/twmb/franz-go/pkg/kgo", true},
	} {
		if (importViolation(x.path, x.value) != "") != x.reject {
			t.Fatalf("bad rule for %s %s", x.path, x.value)
		}
	}
}
