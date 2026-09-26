package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
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
		err := c.Initialize(t.Context(), "/ws")
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("expected an error when the server hung up")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Initialize never returned after the server disconnected")
	}
	_ = c.Close()
}
