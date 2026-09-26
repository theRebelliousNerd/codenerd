package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// Generic LSP client.
//
// lsp/README.md sketched "Phase 3: gopls integration" as a GoplsClient type.
// This is that, generalized: the protocol work (framing, request correlation,
// notification fan-in) is language-agnostic, so a single Client drives gopls,
// rust-analyzer, pyright or anything else that speaks LSP over stdio. The
// session's critic grounding starts one per language a turn wrote
// (internal/session/lsp_diagnostics.go).
//
// The transport is an io.ReadWriteCloser rather than an *exec.Cmd so the client
// is testable without any language server installed: a test drives it over an
// in-memory pipe. StartServer is the thin subprocess wrapper on top.

// Transport is a bidirectional LSP byte stream (a subprocess's stdio, a socket,
// or an in-memory pipe in tests).
type Transport interface {
	io.ReadWriteCloser
}

// Client speaks LSP to one language server.
type Client struct {
	lang      string // Mangle atom form, e.g. "/go"
	transport Transport
	enc       *bufio.Writer
	dec       *bufio.Reader

	mu       sync.Mutex
	nextID   int64
	pending  map[int64]chan json.RawMessage
	errs     map[int64]error
	closed   bool
	writeMu  sync.Mutex
	diagsMu  sync.Mutex
	diags    map[string][]Diagnostic
	doneCh   chan struct{}
	readOnce sync.Once
}

// Diagnostic is a language-agnostic diagnostic from a server.
type Diagnostic struct {
	URI      string
	Line     int
	Severity int
	Message  string
}

// Location is a resolved position in a file.
type Location struct {
	URI  string
	Line int
	Col  int
}

// NewClient wraps a transport. lang is the Mangle atom the projected facts are
// tagged with ("/go", "/rust", ...).
func NewClient(lang string, transport Transport) *Client {
	if !strings.HasPrefix(lang, "/") {
		lang = "/" + lang
	}
	c := &Client{
		lang:      lang,
		transport: transport,
		enc:       bufio.NewWriter(transport),
		dec:       bufio.NewReader(transport),
		pending:   make(map[int64]chan json.RawMessage),
		errs:      make(map[int64]error),
		diags:     make(map[string][]Diagnostic),
		doneCh:    make(chan struct{}),
	}
	c.readOnce.Do(func() { go c.readLoop() })
	return c
}

// StartServer launches a language server binary and returns a Client bound to
// its stdio. A missing binary is reported as a plain error so callers can
// degrade to AST-only intelligence instead of failing the session — no
// environment is required to have gopls installed.
func StartServer(ctx context.Context, lang, binary string, args ...string) (*Client, error) {
	path, err := exec.LookPath(binary)
	if err != nil {
		return nil, fmt.Errorf("language server %q not found on PATH: %w", binary, err)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// A server's stderr is its own chatter, not diagnostics, and a TUI owns
	// the terminal it would otherwise write over.
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", binary, err)
	}
	logging.World("LSP: started %s for %s (pid %d)", binary, lang, cmd.Process.Pid)
	return NewClient(lang, &processTransport{in: stdin, out: stdout, cmd: cmd}), nil
}

type processTransport struct {
	in  io.WriteCloser
	out io.ReadCloser
	cmd *exec.Cmd
}

func (p *processTransport) Read(b []byte) (int, error)  { return p.out.Read(b) }
func (p *processTransport) Write(b []byte) (int, error) { return p.in.Write(b) }
func (p *processTransport) Close() error {
	_ = p.in.Close()
	_ = p.out.Close()
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	return p.cmd.Wait()
}

// Close shuts the transport down. In-flight callers are released with an error
// rather than left blocked forever on a channel nobody will write to.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	for id, ch := range c.pending {
		c.errs[id] = fmt.Errorf("lsp client closed")
		close(ch)
	}
	c.pending = make(map[int64]chan json.RawMessage)
	c.mu.Unlock()
	close(c.doneCh)
	return c.transport.Close()
}

// ---------------------------------------------------------------------------
// Wire protocol
// ---------------------------------------------------------------------------

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *Client) writeMessage(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := fmt.Fprintf(c.enc, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	if _, err := c.enc.Write(body); err != nil {
		return err
	}
	return c.enc.Flush()
}

// readLoop demultiplexes responses to their waiting caller and accumulates
// server-pushed diagnostics.
func (c *Client) readLoop() {
	for {
		body, err := readFrame(c.dec)
		if err != nil {
			c.mu.Lock()
			for id, ch := range c.pending {
				c.errs[id] = err
				close(ch)
			}
			c.pending = make(map[int64]chan json.RawMessage)
			c.mu.Unlock()
			return
		}
		var msg rpcMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			logging.WorldWarn("LSP: undecodable message: %v", err)
			continue
		}
		switch {
		case msg.ID != nil && msg.Method == "":
			c.mu.Lock()
			ch, ok := c.pending[*msg.ID]
			if ok {
				delete(c.pending, *msg.ID)
				if msg.Error != nil {
					c.errs[*msg.ID] = fmt.Errorf("lsp error %d: %s", msg.Error.Code, msg.Error.Message)
				}
			}
			c.mu.Unlock()
			if ok {
				ch <- msg.Result
				close(ch)
			}
		case msg.Method == "textDocument/publishDiagnostics":
			c.handleDiagnostics(msg.Params)
		case msg.ID != nil:
			// Server->client request. Answering null keeps servers that ask for
			// configuration or registration from blocking their own startup.
			_ = c.writeMessage(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": nil})
		}
	}
}

// readFrame reads one Content-Length framed LSP message.
func readFrame(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if name, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("bad Content-Length %q: %w", value, err)
			}
			length = n
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("message without Content-Length header")
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// call issues a request and waits for its response.
func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, fmt.Errorf("lsp client closed")
	}
	c.nextID++
	id := c.nextID
	ch := make(chan json.RawMessage, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	req := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	if err := c.writeMessage(req); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case res, ok := <-ch:
		c.mu.Lock()
		err := c.errs[id]
		delete(c.errs, id)
		c.mu.Unlock()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("lsp request %s cancelled", method)
		}
		return res, nil
	}
}

func (c *Client) notify(method string, params any) error {
	return c.writeMessage(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

// ---------------------------------------------------------------------------
// LSP methods
// ---------------------------------------------------------------------------

// Initialize performs the LSP handshake for a workspace root.
func (c *Client) Initialize(ctx context.Context, root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	_, err = c.call(ctx, "initialize", map[string]any{
		"processId": os.Getpid(),
		"rootUri":   pathToURI(abs),
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"publishDiagnostics": map[string]any{"relatedInformation": false},
				"definition":         map[string]any{"linkSupport": false},
				"references":         map[string]any{},
			},
		},
	})
	if err != nil {
		return err
	}
	return c.notify("initialized", map[string]any{})
}

// DidOpen tells the server about a file's contents. Servers only publish
// diagnostics for documents they have been told about.
func (c *Client) DidOpen(path, languageID, text string) error {
	return c.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":        pathToURI(path),
			"languageId": languageID,
			"version":    1,
			"text":       text,
		},
	})
}

// Shutdown performs the polite LSP shutdown/exit sequence, then closes.
func (c *Client) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, _ = c.call(shutdownCtx, "shutdown", nil)
	_ = c.notify("exit", nil)
	return c.Close()
}

// WaitForDiagnostics blocks until the server has published diagnostics for path
// or the context expires. Diagnostics arrive as unsolicited notifications, so
// without a wait the caller races the server and usually projects nothing.
func (c *Client) WaitForDiagnostics(ctx context.Context, path string) ([]Diagnostic, error) {
	uri := pathToURI(path)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		c.diagsMu.Lock()
		d, ok := c.diags[uri]
		c.diagsMu.Unlock()
		if ok {
			return d, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.doneCh:
			return nil, fmt.Errorf("lsp client closed")
		case <-ticker.C:
		}
	}
}

func (c *Client) handleDiagnostics(params json.RawMessage) {
	var payload struct {
		URI         string `json:"uri"`
		Diagnostics []struct {
			Range struct {
				Start struct {
					Line int `json:"line"`
				} `json:"start"`
			} `json:"range"`
			Severity int    `json:"severity"`
			Message  string `json:"message"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		logging.WorldWarn("LSP: bad publishDiagnostics payload: %v", err)
		return
	}
	out := make([]Diagnostic, 0, len(payload.Diagnostics))
	for _, d := range payload.Diagnostics {
		out = append(out, Diagnostic{
			URI: payload.URI,
			// LSP lines are 0-based; every world fact is 1-based, and mixing the
			// two silently points reviewers at the wrong line.
			Line:     d.Range.Start.Line + 1,
			Severity: d.Severity,
			Message:  d.Message,
		})
	}
	c.diagsMu.Lock()
	c.diags[payload.URI] = out
	c.diagsMu.Unlock()
}

// ---------------------------------------------------------------------------
// Fact projection
// ---------------------------------------------------------------------------

// SeverityWord names an LSP diagnostic severity worth reporting: an error or
// a warning. Information and hints report false. An omitted severity (0) is
// read as an error, which is how the protocol leaves it to the client.
func SeverityWord(sev int) (string, bool) {
	switch sev {
	case 0, 1:
		return "error", true
	case 2:
		return "warning", true
	default:
		return "", false
	}
}
