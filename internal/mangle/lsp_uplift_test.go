package mangle

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Behavioral proofs for the LSP uplift: index lifecycle, slice isolation,
// unified diagnostics, cooperative shutdown, and position honesty.

// Closing a document must retract its symbols: a closed file must not haunt
// go-to-definition or references.
func TestLSPCloseClearsIndex(t *testing.T) {
	s := NewLSPServer(nil)
	uri := "file:///test/close.mg"
	s.OpenDocument(uri, "Decl widget(X).\nwidget(a) :- widget(b).\n", 1)
	if defs := s.GoToDefinition(uri, 2, 2); len(defs) == 0 {
		t.Fatal("expected definition before close")
	}
	s.CloseDocument(uri)
	if defs := s.GetDefinitions("widget"); len(defs) != 0 {
		t.Fatalf("definitions survive close: %v", defs)
	}
	if refs := s.GetReferences("widget"); len(refs) != 0 {
		t.Fatalf("references survive close: %v", refs)
	}
	if diags := s.GetDiagnostics(uri); len(diags) != 0 {
		t.Fatalf("diagnostics survive close: %v", diags)
	}
}

// FindReferences with declarations must not corrupt the stored slice:
// repeated calls return identical results.
func TestLSPFindReferencesIsolated(t *testing.T) {
	s := NewLSPServer(nil)
	uri := "file:///test/refs.mg"
	s.OpenDocument(uri, "Decl item(X).\nitem(a).\nitem(b) :- item(a).\n", 1)
	first := s.FindReferences(uri, 3, 2, true)
	second := s.FindReferences(uri, 3, 2, true)
	if len(first) != len(second) {
		t.Fatalf("repeat call changed results: %d vs %d (append aliasing)", len(first), len(second))
	}
	stored := len(s.references["item"])
	_ = s.FindReferences(uri, 3, 2, true)
	if len(s.references["item"]) != stored {
		t.Fatal("FindReferences mutated the stored reference list")
	}
}

// Mutating a returned slice must not corrupt server state.
func TestLSPGettersCopy(t *testing.T) {
	s := NewLSPServer(nil)
	uri := "file:///test/copy.mg"
	s.OpenDocument(uri, "Decl item(X).\nitem(a).\n", 1)
	defs := s.GetDefinitions("item")
	if len(defs) == 0 {
		t.Fatal("expected definitions")
	}
	defs[0].Symbol = "CORRUPTED"
	if again := s.GetDefinitions("item"); again[0].Symbol == "CORRUPTED" {
		t.Fatal("GetDefinitions exposes the live slice")
	}
	diags := s.GetDiagnostics(uri)
	_ = diags
}

// The exit notification signals shutdown; it must not kill the host process.
func TestLSPExitIsCooperative(t *testing.T) {
	s := NewLSPServer(nil)
	resp := s.handleRequest(LSPRequest{JSONRPC: "2.0", ID: 1, Method: "exit"})
	if resp != nil {
		t.Fatalf("exit should be silent, got %+v", resp)
	}
	if !s.shutdownRequested.Load() {
		t.Fatal("exit did not request shutdown")
	}
	// We are still alive to assert this, which is the point.
}

// Repeated body predicates get their own columns, not the first one's.
func TestLSPBodyColumnsOnRepeats(t *testing.T) {
	s := NewLSPServer(nil)
	uri := "file:///test/cols.mg"
	s.OpenDocument(uri, "head(X) :- body(X), body(Y).\n", 1)
	var cols []int
	for _, r := range s.GetReferences("body") {
		if r.Kind == RefInBody {
			cols = append(cols, r.Column)
		}
	}
	if len(cols) != 2 {
		t.Fatalf("expected 2 body references, got %d", len(cols))
	}
	if cols[0] == cols[1] {
		t.Fatalf("both references share column %d", cols[0])
	}
	if cols[0] > cols[1] {
		t.Fatalf("columns out of order: %v", cols)
	}
}

// An editor attached to `nerd mangle-lsp` sees what the server computes:
// diagnostics are published after open and cleared on close, and references
// and completion are answered instead of advertised and refused.
func TestLSPServesDiagnosticsReferencesAndCompletion(t *testing.T) {
	s := NewLSPServer(nil)
	uri := "file:///ws/p.mg"
	text := "Decl edge(X, Y).\nedge(/a, /b).\nreach(X, Y) :- edge(X, Y).\nbroken(a.\n"
	open := LSPRequest{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: json.RawMessage(
		fmt.Sprintf(`{"textDocument":{"uri":%q,"text":%q,"version":1}}`, uri, text))}
	if resp := s.handleRequest(open); resp != nil {
		t.Fatalf("didOpen is a notification; got a response %+v", resp)
	}
	notes := s.notificationsFor(open)
	if len(notes) != 1 {
		t.Fatalf("didOpen owes one publishDiagnostics, got %d", len(notes))
	}
	body, _ := json.Marshal(notes[0])
	if !strings.Contains(string(body), `"method":"textDocument/publishDiagnostics"`) || !strings.Contains(string(body), `"line":3`) {
		t.Fatalf("the broken line 4 was not published: %s", body)
	}

	refs := s.handleRequest(LSPRequest{JSONRPC: "2.0", ID: 7, Method: "textDocument/references", Params: json.RawMessage(
		fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":2,"character":16},"context":{"includeDeclaration":true}}`, uri))})
	if refs == nil || refs.Error != nil {
		t.Fatalf("references refused: %+v", refs)
	}
	if locs, _ := refs.Result.([]map[string]any); len(locs) == 0 {
		t.Fatalf("no references to edge: %+v", refs.Result)
	}

	comp := s.handleRequest(LSPRequest{JSONRPC: "2.0", ID: 8, Method: "textDocument/completion", Params: json.RawMessage(
		fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":2,"character":17}}`, uri))})
	if comp == nil || comp.Error != nil {
		t.Fatalf("completion refused: %+v", comp)
	}
	items, _ := comp.Result.([]map[string]any)
	found := false
	for _, it := range items {
		found = found || it["label"] == "edge"
	}
	if !found {
		t.Fatalf("completion at \"ed\" did not offer edge: %v", items)
	}

	closeReq := LSPRequest{JSONRPC: "2.0", Method: "textDocument/didClose", Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q}}`, uri))}
	s.handleRequest(closeReq)
	body, _ = json.Marshal(s.notificationsFor(closeReq)[0])
	if !strings.Contains(string(body), `"diagnostics":[]`) {
		t.Fatalf("closing did not clear the document's diagnostics: %s", body)
	}
}
