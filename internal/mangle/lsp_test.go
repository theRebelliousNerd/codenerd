package mangle

import (
	"testing"
)

func TestNewLSPServer(t *testing.T) {
	// We can pass a nil Engine just to test the LSPServer initialization,
	// or create a dummy engine if Engine has a NewEngine method.
	// For this test, nil is sufficient to check map initializations.
	server := NewLSPServer(nil)

	if server == nil {
		t.Fatal("Expected NewLSPServer to return a non-nil object")
	}

	if server.engine != nil {
		t.Errorf("Expected engine to be nil, got %v", server.engine)
	}

	if server.documents == nil {
		t.Error("Expected documents map to be initialized")
	}

	if server.definitions == nil {
		t.Error("Expected definitions map to be initialized")
	}

	if server.references == nil {
		t.Error("Expected references map to be initialized")
	}

	if server.diagnostics == nil {
		t.Error("Expected diagnostics map to be initialized")
	}

	if server.hover == nil {
		t.Error("Expected hover map to be initialized")
	}
}

func TestLSPServer_OpenCloseDocument(t *testing.T) {
	server := NewLSPServer(nil)

	uri := "file:///test/doc.mg"
	content := "foo(bar)."
	version := 1

	// Test OpenDocument
	server.OpenDocument(uri, content, version)

	server.mu.RLock()
	doc, exists := server.documents[uri]
	server.mu.RUnlock()

	if !exists {
		t.Fatalf("Expected document with URI %s to be opened", uri)
	}

	if doc.URI != uri {
		t.Errorf("Expected document URI %s, got %s", uri, doc.URI)
	}

	if doc.Content != content {
		t.Errorf("Expected document Content %s, got %s", content, doc.Content)
	}

	if doc.Version != version {
		t.Errorf("Expected document Version %d, got %d", version, doc.Version)
	}

	if len(doc.Lines) == 0 {
		t.Errorf("Expected document Lines to be populated")
	} else if doc.Lines[0] != content {
		t.Errorf("Expected first line to be %s, got %s", content, doc.Lines[0])
	}

	// Test CloseDocument
	server.CloseDocument(uri)

	server.mu.RLock()
	_, existsAfterClose := server.documents[uri]
	server.mu.RUnlock()

	if existsAfterClose {
		t.Errorf("Expected document with URI %s to be removed after CloseDocument", uri)
	}
}

// TODO: Add Negative Test - GoToDefinition with line < 0 or line >= len(doc.Lines) to catch slice panics.
// TODO: Add Negative Test - GetHover with col < 0 or col > len(line) bounds checking.
// TODO: Add Negative Test - OpenDocument with empty string content and empty URI.
// TODO: Add Boundary Test - Symbol extraction involving multibyte UTF-8 characters and emojis.
// TODO: Add Boundary Test - Extreme line lengths (e.g., 500,000 chars) to check for ReDoS in indexDocumentLocked.
// TODO: Add Boundary Test - Deeply nested and unbalanced parentheses in countArity.
// TODO: Add State Conflict Test - Calling GetDiagnostics on a URI immediately after CloseDocument is called.
// TODO: Add State Conflict Test - Out-of-order version numbers in OpenDocument to ensure stale data doesn't overwrite new data.
// TODO: Add Concurrency Test - 100+ goroutines reading (FindReferences) while writing (OpenDocument) to trigger race conditions.
// TODO: Add Edge Case Test - Ensure LSPServer handles nil engine gracefully during GetHover and IndexWorkspace.
