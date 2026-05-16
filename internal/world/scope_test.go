package world

import (
	"testing"
)

func TestNewFileScope(t *testing.T) {
	projectRoot := "/path/to/project"
	scope := NewFileScope(projectRoot)

	if scope == nil {
		t.Fatalf("Expected NewFileScope to return a non-nil FileScope")
	}

	if scope.ProjectRoot != projectRoot {
		t.Errorf("Expected ProjectRoot to be %q, got %q", projectRoot, scope.ProjectRoot)
	}

	if scope.InScope == nil || len(scope.InScope) != 0 {
		t.Errorf("Expected InScope to be initialized and empty, got %v", scope.InScope)
	}

	if scope.Elements == nil || len(scope.Elements) != 0 {
		t.Errorf("Expected Elements to be initialized and empty, got %v", scope.Elements)
	}

	if scope.OutboundDeps == nil || len(scope.OutboundDeps) != 0 {
		t.Errorf("Expected OutboundDeps to be initialized and empty, got %v", scope.OutboundDeps)
	}

	if scope.InboundDeps == nil || len(scope.InboundDeps) != 0 {
		t.Errorf("Expected InboundDeps to be initialized and empty, got %v", scope.InboundDeps)
	}

	if scope.FileHashes == nil || len(scope.FileHashes) != 0 {
		t.Errorf("Expected FileHashes to be initialized and empty, got %v", scope.FileHashes)
	}

	if len(scope.diagnosticFacts) != 0 {
		t.Errorf("Expected diagnosticFacts to be empty, got length %d", len(scope.diagnosticFacts))
	}

	if len(scope.diagnosticFactIndex) != 0 {
		t.Errorf("Expected diagnosticFactIndex to be empty, got length %d", len(scope.diagnosticFactIndex))
	}

	if scope.parserFactory == nil {
		t.Errorf("Expected parserFactory to be initialized")
	}
}
