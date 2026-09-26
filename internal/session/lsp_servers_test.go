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

	"codenerd/internal/world/lsp"
)

// fakeLanguageServer answers initialize and shutdown, and on didOpen publishes
// one error and one hint for the opened document.
func fakeLanguageServer(t *testing.T, conn net.Conn) {
	t.Helper()
	r := bufio.NewReader(conn)
	send := func(v any) {
		body, _ := json.Marshal(v)
		fmt.Fprintf(conn, "Content-Length: %d\r\n\r\n%s", len(body), body)
	}
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
