package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/types"
	"codenerd/internal/world/lsp"
)

// fakeLanguageServer answers initialize and shutdown, and on didOpen publishes
// one error and one hint for the opened document.
func fakeLanguageServer(t *testing.T, conn net.Conn) {
	t.Helper()
	serveLanguageServer(t, conn, func(int) bool { return true })
}

// serveLanguageServer is fakeLanguageServer with a gate on publishing.
// publish reports whether this didOpen (0-based) gets diagnostics. A file
// the server never publishes for is how a request that does not finish is
// exercised without waiting out the default bound.
func serveLanguageServer(t *testing.T, conn net.Conn, publish func(opened int) bool) {
	t.Helper()
	r := bufio.NewReader(conn)
	send := func(v any) {
		body, _ := json.Marshal(v)
		fmt.Fprintf(conn, "Content-Length: %d\r\n\r\n%s", len(body), body)
	}
	opened := 0
	for {
		length := 0
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if v, ok := strings.CutPrefix(line, "Content-Length:"); ok {
				length, _ = strconv.Atoi(strings.TrimSpace(v))
			}
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		var msg struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.Unmarshal(body, &msg)
		switch msg.Method {
		case "initialize":
			send(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": map[string]any{"capabilities": map[string]any{}}})
		case "shutdown":
			send(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": nil})
		case "textDocument/didOpen":
			n := opened
			opened++
			if !publish(n) {
				continue
			}
			var p struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &p)
			send(map[string]any{"jsonrpc": "2.0", "method": "textDocument/publishDiagnostics", "params": map[string]any{
				"uri": p.TextDocument.URI,
				"diagnostics": []any{
					map[string]any{"range": map[string]any{"start": map[string]any{"line": 2}}, "severity": 1, "message": `"undefined_name" is not defined`},
					map[string]any{"range": map[string]any{"start": map[string]any{"line": 0}}, "severity": 4, "message": "unused import"},
				},
			}})
		}
	}
}

func withFakeLanguageServer(t *testing.T, started *[]string) {
	t.Helper()
	prev := startLanguageServer
	startLanguageServer = func(_ context.Context, lang, binary string, _ ...string) (*lsp.Client, error) {
		*started = append(*started, binary)
		clientSide, serverSide := net.Pipe()
		go fakeLanguageServer(t, serverSide)
		t.Cleanup(func() { _ = serverSide.Close() })
		return lsp.NewClient(lang, clientSide), nil
	}
	t.Cleanup(func() { startLanguageServer = prev })
}

// A Python write is checked by its language server and the critic is handed
// the errors and warnings, not the hints.
func TestLSPDiagnostics_GroundsAPythonTurn(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "app.py"), []byte("import os\n\nundefined_name()\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var started []string
	withFakeLanguageServer(t, &started)

	got := lspDiagnostics(context.Background(), ws, []string{"app.py", "README.md", "main.go"})
	want := `app.py:3: error: "undefined_name" is not defined`
	if got != want {
		t.Fatalf("diagnostics = %q, want %q", got, want)
	}
	if len(started) != 1 || started[0] != "pyright-langserver" {
		t.Fatalf("servers started = %v, want only pyright-langserver", started)
	}
}

// No written file a server checks, no server; an absent server is silence.
func TestLSPDiagnostics_AbsentServerOrNoFilesIsSilent(t *testing.T) {
	ws := t.TempDir()
	var started []string
	withFakeLanguageServer(t, &started)
	if got := lspDiagnostics(context.Background(), ws, []string{"main.go", "notes.md"}); got != "" || len(started) != 0 {
		t.Fatalf("a turn with no Python or TypeScript started %v and reported %q", started, got)
	}

	startLanguageServer = func(context.Context, string, string, ...string) (*lsp.Client, error) {
		return nil, errors.New("not on PATH")
	}
	if err := os.WriteFile(filepath.Join(ws, "app.ts"), []byte("let x: number = 'a'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := lspDiagnostics(context.Background(), ws, []string{"app.ts"}); got != "" {
		t.Fatalf("an absent server reported %q", got)
	}
}

// Every Python file the turn wrote is opened. Stopping at eight hid the rest.
func TestLSPDiagnostics_ChecksEveryPythonFile(t *testing.T) {
	ws := t.TempDir()
	var paths []string
	for i := 0; i < 9; i++ {
		name := fmt.Sprintf("app%d.py", i)
		if err := os.WriteFile(filepath.Join(ws, name), []byte("import os\n\nundefined_name()\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, name)
	}
	var started []string
	withFakeLanguageServer(t, &started)
	got := lspDiagnostics(context.Background(), ws, paths)
	if strings.Count(got, "error:") != len(paths) {
		t.Fatalf("got %d error lines, want %d:\n%s", strings.Count(got, "error:"), len(paths), got)
	}
	for _, name := range paths {
		want := name + `:3: error: "undefined_name" is not defined`
		if !strings.Contains(got, want) {
			t.Errorf("missing %s\n%s", want, got)
		}
	}
}

// A server that publishes for the first file and never for the later ones
// costs the rest of that one request. The file it diagnosed stays; the ones
// it did not are named. The wait is the request bound, not the section default.
func TestLSPDiagnostics_UnfinishedServerNamesUncheckedFiles(t *testing.T) {
	ws := t.TempDir()
	for _, name := range []string{"app0.py", "app1.py", "app2.py"} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte("import os\n\nundefined_name()\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prev := startLanguageServer
	startLanguageServer = func(_ context.Context, lang, _ string, _ ...string) (*lsp.Client, error) {
		clientSide, serverSide := net.Pipe()
		go serveLanguageServer(t, serverSide, func(opened int) bool { return opened == 0 })
		t.Cleanup(func() { _ = serverSide.Close() })
		return lsp.NewClient(lang, clientSide), nil
	}
	t.Cleanup(func() { startLanguageServer = prev })

	ctx := withDiagnosticTimeout(context.Background(), 250*time.Millisecond)
	start := time.Now()
	got := lspDiagnostics(ctx, ws, []string{"app0.py", "app1.py", "app2.py"})
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("the request waited %s; session.lsp_timeout did not bound it", elapsed)
	}
	if !strings.Contains(got, `app0.py:3: error: "undefined_name" is not defined`) {
		t.Fatalf("the file the server did diagnose was dropped: %q", got)
	}
	if !types.IsClamped(got) {
		t.Fatalf("files the server never published for were reported as clean: %q", got)
	}
	idx := strings.Index(got, "files not diagnosed")
	if idx < 0 {
		t.Fatalf("the omission was not named: %q", got)
	}
	omitted := got[idx:]
	for _, name := range []string{"app1.py", "app2.py"} {
		if !strings.Contains(omitted, name) {
			t.Errorf("unchecked file %s was not named: %q", name, got)
		}
	}
	if strings.Contains(omitted, "app0.py") {
		t.Errorf("a diagnosed file was listed as unchecked: %q", got)
	}
}

// A non-Go uplift is not re-verified with the Go toolchain: in a workspace
// with no go.mod, `go build ./...` fails, and the recheck would undo every
// uplift the review asked for. The workspace's own test gate measures it.
func TestRecheckUplift_NonGoTurnIsNotGoVerified(t *testing.T) {
	var ran atomic.Bool
	goToolchain := func(context.Context, string, []string, string, []string) ([]byte, error) {
		ran.Store(true)
		return []byte("go: go.mod file not found in current directory"), errors.New("exit status 1")
	}
	stubVerifySeams(t, time.Minute, time.Minute, goToolchain, goToolchain)

	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "app.py"), []byte("def f():\n    return 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := &ExecutionResult{WrittenPaths: []string{"app.py"}}
	if err := recheckUplift(context.Background(), ws, result, snapshotTurnFiles(ws, result)); err != nil {
		t.Fatalf("recheckUplift: %v", err)
	}
	if ran.Load() {
		t.Error("the Go toolchain ran for a turn that wrote no Go")
	}
	if got, _ := os.ReadFile(filepath.Join(ws, "app.py")); string(got) != "def f():\n    return 2\n" {
		t.Fatalf("the uplift was undone: %q", got)
	}
}
