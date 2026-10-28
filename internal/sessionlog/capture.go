package sessionlog

import (
	"bytes"
	"context"
	"io"
	"mime"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type captureKey struct{}

// Capture collects the wire traffic of one in-flight model call. The broker
// installs one per call when exchange logging is active; the transport and the
// exec recorders write to it.
type Capture struct {
	mu   sync.Mutex
	wire []Wire
}

// WithCapture returns a context whose transports record into c.
func WithCapture(ctx context.Context, c *Capture) context.Context {
	if c == nil {
		return ctx
	}
	return context.WithValue(ctx, captureKey{}, c)
}

// CaptureFrom returns the capture on ctx, or nil.
func CaptureFrom(ctx context.Context) *Capture {
	if ctx == nil {
		return nil
	}
	c, _ := ctx.Value(captureKey{}).(*Capture)
	return c
}

func (c *Capture) add(w Wire) {
	c.mu.Lock()
	defer c.mu.Unlock()
	w.Attempt = len(c.wire) + 1
	c.wire = append(c.wire, w)
}

// Wires returns the recorded round trips in order.
func (c *Capture) Wires() []Wire {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Wire(nil), c.wire...)
}

// WireTransport wraps base so that a request made under a context carrying a
// Capture is recorded: request and response exactly as they crossed the
// transport, credentials stripped. A request with no capture on its context
// passes straight through, so installing it everywhere costs one context
// lookup per request.
func WireTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &wireTransport{base: base}
}

// WrapClient returns a copy of hc whose transport is WireTransport over hc's
// own (http.DefaultTransport when it has none). hc is not modified.
func WrapClient(hc *http.Client) *http.Client {
	if hc == nil {
		return &http.Client{Transport: WireTransport(nil)}
	}
	if _, already := hc.Transport.(*wireTransport); already {
		return hc
	}
	out := *hc
	out.Transport = WireTransport(hc.Transport)
	return &out
}

type wireTransport struct{ base http.RoundTripper }

// UnderlyingTransport returns the transport beneath a WireTransport wrapper,
// or rt itself when it is not wrapped.
func UnderlyingTransport(rt http.RoundTripper) http.RoundTripper {
	if w, ok := rt.(*wireTransport); ok {
		return w.base
	}
	return rt
}

func (t *wireTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	capture := CaptureFrom(req.Context())
	if capture == nil {
		return t.base.RoundTrip(req)
	}
	started := time.Now()
	wire := Wire{Transport: "http", Started: started}
	wire.Request = describeRequest(req)

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		wire.DurationMS = time.Since(started).Milliseconds()
		wire.Error = err.Error()
		capture.add(wire)
		return resp, err
	}
	wr := &WireResponse{Status: resp.StatusCode, Headers: RedactHeaders(resp.Header)}
	wire.Response = wr
	if isEventStream(resp.Header.Get("Content-Type")) {
		wr.BodyNote = "event stream: the stream is not kept as events; the assembled result is the exchange's response"
		wire.DurationMS = time.Since(started).Milliseconds()
		capture.add(wire)
		return resp, nil
	}
	resp.Body = &teeBody{
		src: resp.Body,
		done: func(body []byte, readErr error) {
			wire.DurationMS = time.Since(started).Milliseconds()
			fillResponseBody(wr, resp.Header.Get("Content-Type"), body)
			if readErr != nil && readErr != io.EOF {
				wire.Error = "reading response body: " + readErr.Error()
			}
			capture.add(wire)
		},
	}
	return resp, nil
}

func describeRequest(req *http.Request) WireRequest {
	wr := WireRequest{
		Method:  req.Method,
		URL:     RedactURL(req.URL.String()),
		Headers: RedactHeaders(req.Header),
	}
	if req.Host != "" && wr.Headers != nil {
		wr.Headers["Host"] = []string{req.Host}
	}
	var body []byte
	switch {
	case req.GetBody != nil:
		if rc, err := req.GetBody(); err == nil {
			body, _ = io.ReadAll(rc)
			rc.Close()
		}
	case req.Body != nil && req.Body != http.NoBody:
		body, _ = io.ReadAll(req.Body)
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	fillRequestBody(&wr, req.Header.Get("Content-Type"), body)
	return wr
}

func fillRequestBody(wr *WireRequest, contentType string, body []byte) {
	wr.BodyBytes = len(body)
	wr.Body, wr.LinesPaths, wr.BodyNote = encodeBody(contentType, body)
}

func fillResponseBody(wr *WireResponse, contentType string, body []byte) {
	wr.BodyBytes = len(body)
	wr.Body, wr.LinesPaths, wr.BodyNote = encodeBody(contentType, body)
}

// encodeBody turns a body into its file form. JSON keeps its key order;
// other text becomes lines; binary is named, never silently dropped.
func encodeBody(contentType string, body []byte) (any, []string, string) {
	if len(body) == 0 {
		return nil, nil, ""
	}
	if tree, err := DecodeOrdered(body); err == nil {
		split, paths := SplitLines(tree)
		return split, paths, ""
	}
	if utf8.Valid(body) {
		return Lines(string(body)), nil, "text body (not JSON); stored as lines"
	}
	return nil, nil, "binary body of " + contentType + " not recorded"
}

func isEventStream(contentType string) bool {
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return strings.Contains(strings.ToLower(contentType), "event-stream")
	}
	return mt == "text/event-stream"
}

// teeBody copies what the caller reads into a buffer and reports it once the
// body ends or is closed.
type teeBody struct {
	src  io.ReadCloser
	buf  bytes.Buffer
	once sync.Once
	err  error
	done func([]byte, error)
}

func (b *teeBody) Read(p []byte) (int, error) {
	n, err := b.src.Read(p)
	b.buf.Write(p[:n])
	if err != nil {
		b.err = err
		b.finish()
	}
	return n, err
}

func (b *teeBody) Close() error {
	err := b.src.Close()
	b.finish()
	return err
}

func (b *teeBody) finish() {
	b.once.Do(func() { b.done(b.buf.Bytes(), b.err) })
}

// ExecRun is one subprocess model call: the command line, what was piped in,
// what came out.
type ExecRun struct {
	Command  string
	Args     []string
	Stdin    string
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
	Started  time.Time
	Duration time.Duration
}

// RecordExec records a CLI-backed model call into the capture on ctx. It
// stores the argument vector and stdin as the request and stdout/stderr as the
// response; the environment (where CLI engines keep their credentials) is never
// recorded. A context with no capture records nothing.
func RecordExec(ctx context.Context, r ExecRun) {
	capture := CaptureFrom(ctx)
	if capture == nil {
		return
	}
	reqTree, _ := ToTree(map[string]any{"args": r.Args, "stdin": r.Stdin})
	split, paths := SplitLines(reqTree)
	wire := Wire{
		Transport:  "exec",
		Started:    r.Started,
		DurationMS: r.Duration.Milliseconds(),
		Request: WireRequest{
			Method:     "exec",
			URL:        r.Command,
			Body:       split,
			LinesPaths: paths,
			BodyNote:   "argument vector and stdin; environment not recorded",
		},
	}
	respTree, _ := ToTree(map[string]any{"stdout": r.Stdout, "stderr": r.Stderr})
	respSplit, respPaths := SplitLines(respTree)
	wire.Response = &WireResponse{Status: r.ExitCode, Body: respSplit, LinesPaths: respPaths, BodyNote: "status is the exit code"}
	if r.Err != nil {
		wire.Error = r.Err.Error()
	}
	capture.add(wire)
}

// ExitCode is the exit code of a finished command, or -1 when it never ran.
func ExitCode(cmd *exec.Cmd) int {
	if cmd == nil || cmd.ProcessState == nil {
		return -1
	}
	return cmd.ProcessState.ExitCode()
}
