package context_harness

import (
	"io"
	"testing"

	"codenerd/internal/core"
)

func newSeededKernel(t *testing.T) *core.RealKernel {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	kernel.SetSchemas(
		"Decl active_issue(ID).\n" +
			"Decl issue_description(ID, Text).\n" +
			"Decl issue_mentioned_file(ID, File, Tier).\n" +
			"Decl issue_error_type(ID, Type).\n" +
			"Decl symbol_graph(Caller, Callee, Kind).\n" +
			"Decl dependency_link(From, To, Kind).\n" +
			"Decl project_pattern(Pattern, Value).\n" +
			"Decl file_topology(File, State, Exists).")
	kernel.SetPolicy("")
	return kernel
}

func TestFactSeeder_IssueAndGraphs(t *testing.T) {
	kernel := newSeededKernel(t)
	fs := NewFactSeeder(kernel)

	if err := fs.SeedIssueContext("ISSUE-1", "null deref", []string{"a.go", "b.go"}, []string{"NilPointer"}); err != nil {
		t.Fatalf("SeedIssueContext: %v", err)
	}
	files, _ := kernel.Query("issue_mentioned_file")
	if len(files) != 2 {
		t.Errorf("expected 2 mentioned-file facts, got %d", len(files))
	}

	if err := fs.SeedSymbolGraph(map[string][]string{"main": {"helper", "logger"}}); err != nil {
		t.Fatalf("SeedSymbolGraph: %v", err)
	}
	edges, _ := kernel.Query("symbol_graph")
	if len(edges) != 2 {
		t.Errorf("expected 2 symbol_graph edges, got %d", len(edges))
	}

	if err := fs.SeedDependencyLinks(map[string][]string{"a.go": {"b.go"}}); err != nil {
		t.Fatalf("SeedDependencyLinks: %v", err)
	}
	if err := fs.SeedProjectPatterns(map[string]string{"layout": "monorepo"}); err != nil {
		t.Fatalf("SeedProjectPatterns: %v", err)
	}
	if err := fs.SeedFileTopology([]string{"a.go", "b.go", "c.go"}); err != nil {
		t.Fatalf("SeedFileTopology: %v", err)
	}
	topo, _ := kernel.Query("file_topology")
	if len(topo) != 3 {
		t.Errorf("expected 3 file_topology facts, got %d", len(topo))
	}
}

func TestFileLogger_Writers(t *testing.T) {
	fl, err := NewFileLogger(t.TempDir(), io.Discard)
	if err != nil {
		t.Fatalf("NewFileLogger: %v", err)
	}
	defer fl.Close()

	// Every category writer must be non-nil and accept a write.
	writers := map[string]io.Writer{
		"jit":         fl.GetJITWriter(),
		"activation":  fl.GetActivationWriter(),
		"compression": fl.GetCompressionWriter(),
		"piggyback":   fl.GetPiggybackWriter(),
		"summary":     fl.GetSummaryWriter(),
		"feedback":    fl.GetFeedbackWriter(),
	}
	for name, w := range writers {
		if w == nil {
			t.Errorf("%s writer is nil", name)
			continue
		}
		if _, err := w.Write([]byte("log line\n")); err != nil {
			t.Errorf("%s writer write failed: %v", name, err)
		}
	}
}
