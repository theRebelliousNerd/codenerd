package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeServer speaks just enough LSP to exercise the client end to end: framing,
// request/response correlation, and unsolicited notifications. It exists so the
// client is verifiable with no language server installed — the environment this
// runs in has no gopls, and a client that is only "tested" by being pointed at a
// binary that may not exist is not tested at all.
type fakeServer struct {
	conn       net.Conn
	t          *testing.T
	diagURI    string
	gotInit    chan struct{}
	gotDidOpen chan string
}

func startFakeServer(t *testing.T, diagURI string) (*Client, *fakeServer) {
	t.Helper()
	clientSide, serverSide := net.Pipe()
	fs := &fakeServer{
		conn:       serverSide,
		t:          t,
		diagURI:    diagURI,
		gotInit:    make(chan struct{}, 1),
		gotDidOpen: make(chan string, 1),
	}
	go fs.serve()
	c := NewClient("/go", clientSide)
	t.Cleanup(func() { _ = c.Close() })
	return c, fs
}

func (f *fakeServer) serve() {
	r := bufio.NewReader(f.conn)
	w := bufio.NewWriter(f.conn)
	send := func(v any) {
		body, _ := json.Marshal(v)
		fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(body))
		w.Write(body)
		w.Flush()
	}
	for {
		body, err := readFrame(r)
		if err != nil {
			return
		}
		var msg struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(body, &msg); err != nil {
			return
		}
		switch msg.Method {
		case "initialize":
			select {
			case f.gotInit <- struct{}{}:
			default:
			}
			send(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": map[string]any{"capabilities": map[string]any{}}})
		case "textDocument/didOpen":
			var p struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &p)
			select {
			case f.gotDidOpen <- p.TextDocument.URI:
			default:
			}
			// Diagnostics arrive unsolicited, after the fact, exactly as a real
			// server publishes them.
			send(map[string]any{
				"jsonrpc": "2.0",
				"method":  "textDocument/publishDiagnostics",
				"params": map[string]any{
					"uri": f.diagURI,
					"diagnostics": []any{
						map[string]any{
							"range":    map[string]any{"start": map[string]any{"line": 4, "character": 2}},
							"severity": 1,
							"message":  "undefined: Foo",
						},
					},
				},
			})
		case "textDocument/definition":
			send(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": []any{
				map[string]any{
					"uri":   f.diagURI,
					"range": map[string]any{"start": map[string]any{"line": 9, "character": 5}},
				},
			}})
		case "textDocument/references":
			// LocationLink shape, which servers are free to use instead of
			// Location; the client must accept both.
			send(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": []any{
				map[string]any{
					"targetUri":            f.diagURI,
					"targetSelectionRange": map[string]any{"start": map[string]any{"line": 19, "character": 0}},
				},
			}})
		case "shutdown":
			send(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": nil})
		case "exit":
			return
		}
	}
}

func TestLSPClient_WhenServerPublishesDiagnostics_ShouldProjectCanonicalFacts(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "internal", "svc", "run.go")
	c, fs := startFakeServer(t, pathToURI(file))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if err := c.Initialize(ctx, root); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	select {
	case <-fs.gotInit:
	case <-ctx.Done():
		t.Fatal("server never saw initialize")
	}

	if err := c.DidOpen(file, "go", "package svc\n"); err != nil {
		t.Fatalf("DidOpen: %v", err)
	}
	diags, err := c.WaitForDiagnostics(ctx, file)
	if err != nil {
		t.Fatalf("WaitForDiagnostics: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(diags))
	}
	// LSP is 0-based; world facts are 1-based.
	if diags[0].Line != 5 {
		t.Errorf("diagnostic line = %d, want 5 (LSP line 4 is the fifth line)", diags[0].Line)
	}

	facts := c.DiagnosticFacts(root)
	if len(facts) != 1 {
		t.Fatalf("projected %d facts, want 1", len(facts))
	}
	path, _ := facts[0].Args[0].(string)
	if path != "internal/svc/run.go" {
		t.Errorf("code_diagnostic path = %q, want the canonical workspace-relative path; an absolute path joins no file_topology row", path)
	}
	if got := fmt.Sprint(facts[0].Args[2]); got != "/error" {
		t.Errorf("severity atom = %q, want /error", got)
	}
}

func TestLSPClient_WhenAskedForDefinitions_ShouldDecodeBothLocationShapes(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.go")
	c, _ := startFakeServer(t, pathToURI(file))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	defs, err := c.Definition(ctx, file, 0, 0)
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if len(defs) != 1 || defs[0].Line != 10 {
		t.Fatalf("definitions = %+v, want one at line 10", defs)
	}
	refs, err := c.References(ctx, file, 0, 0, false)
	if err != nil {
		t.Fatalf("References: %v", err)
	}
	if len(refs) != 1 || refs[0].Line != 20 {
		t.Fatalf("references = %+v, want one at line 20 (LocationLink shape)", refs)
	}

	facts := c.SymbolFacts(root, "Foo", defs, refs)
	if len(facts) != 2 {
		t.Fatalf("projected %d facts, want 2", len(facts))
	}
	if facts[0].Predicate != "symbol_defined" || facts[1].Predicate != "symbol_referenced" {
		t.Errorf("unexpected predicates: %v", facts)
	}
	if p, _ := facts[0].Args[2].(string); p != "a.go" {
		t.Errorf("symbol_defined path = %q, want canonical a.go", p)
	}
}

// TestLSPClient_WhenClosedWithRequestInFlight_ShouldNotBlockForever — a server
// that dies mid-request must fail its callers, not park them on a channel
// nothing will ever write to.
func TestLSPClient_WhenClosedWithRequestInFlight_ShouldNotBlockForever(t *testing.T) {
	clientSide, serverSide := net.Pipe()
	c := NewClient("/go", clientSide)
	go func() {
		r := bufio.NewReader(serverSide)
		_, _ = readFrame(r) // swallow the request, then hang up
		serverSide.Close()
	}()

	done := make(chan error, 1)
	go func() {
		_, err := c.Definition(t.Context(), "a.go", 1, 1)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("expected an error when the server hung up")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Definition never returned after the server disconnected")
	}
	_ = c.Close()
}

// TestStartServer_WhenBinaryMissing_ShouldReportItPlainly — the offline/no-tool
// path must be a legible error, not a panic or a silent nil client, because
// most environments have no language server installed.
func TestStartServer_WhenBinaryMissing_ShouldReportItPlainly(t *testing.T) {
	_, err := StartServer(t.Context(), "/go", "definitely-not-a-real-language-server-binary")
	if err == nil {
		t.Fatal("expected an error for a missing binary")
	}
	if !strings.Contains(err.Error(), "not found on PATH") {
		t.Errorf("error %q does not explain that the binary is missing", err)
	}
}

// TestManager_WhenExternalServerRegistered_ShouldIncludeItsDiagnostics proves
// the Manager fans external servers into the same relations as the built-in
// Mangle server.
func TestManager_WhenExternalServerRegistered_ShouldIncludeItsDiagnostics(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "x.go")
	c, _ := startFakeServer(t, pathToURI(file))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := c.Initialize(ctx, root); err != nil {
		t.Fatal(err)
	}
	if err := c.DidOpen(file, "go", "package x\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.WaitForDiagnostics(ctx, file); err != nil {
		t.Fatal(err)
	}

	m := NewManager(root)
	m.AddLanguageServer("go", c)
	if _, ok := m.LanguageServer("/go"); !ok {
		t.Fatal("registered server not retrievable")
	}
	m.indexed = true // the Mangle half is exercised by manager_test.go

	facts, err := m.ProjectToFacts()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range facts {
		if f.Predicate == "code_diagnostic" {
			if p, _ := f.Args[0].(string); p == "x.go" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("external server diagnostics were not projected: %v", facts)
	}
}
