package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- SSE transport ----------------------------------------------------------

// rawSSEServer is the legacy two-endpoint binding written from the
// specification: GET opens the event stream and announces an endpoint, POSTs to
// that endpoint are acknowledged with 202, and every JSON-RPC response travels
// on the stream. onPost decides what to push for each message received.
type rawSSEServer struct {
	*httptest.Server
	mu     sync.Mutex
	events chan string
	bodies []string
	onPost func(req rawRPC, push func(frame string))
}

func newRawSSEServer(t *testing.T, onPost func(req rawRPC, push func(frame string))) *rawSSEServer {
	t.Helper()
	s := &rawSSEServer{events: make(chan string, 64), onPost: onPost}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)
			fmt.Fprint(w, "event: endpoint\ndata: /messages\n\n")
			flusher.Flush()
			for {
				select {
				case frame := <-s.events:
					fmt.Fprint(w, frame)
					flusher.Flush()
				case <-r.Context().Done():
					return
				}
			}
		}
		raw, _ := io.ReadAll(r.Body)
		var req rawRPC
		_ = json.Unmarshal(raw, &req)
		s.mu.Lock()
		s.bodies = append(s.bodies, string(raw))
		s.mu.Unlock()
		s.onPost(req, func(frame string) { s.events <- frame })
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *rawSSEServer) postBodies() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.bodies...)
}

func messageFrame(payload string) string { return "event: message\ndata: " + payload + "\n\n" }

func handshakeThen(rest func(req rawRPC, push func(frame string))) func(rawRPC, func(string)) {
	return func(req rawRPC, push func(string)) {
		switch req.Method {
		case "initialize":
			push(messageFrame(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, goodInitialize)))
		case "notifications/initialized":
		default:
			rest(req, push)
		}
	}
}

func connectSSE(t *testing.T, s *rawSSEServer) *SSETransport {
	t.Helper()
	transport := NewSSETransportWithHeaders(s.URL, 2*time.Second, nil)
	if err := transport.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = transport.Disconnect() })
	return transport
}

func TestSSEInitialize_ObjectCapabilitiesAndInitializedNotification(t *testing.T) {
	s := newRawSSEServer(t, handshakeThen(func(rawRPC, func(string)) {}))
	transport := connectSSE(t, s)
	caps, err := transport.GetCapabilities(context.Background())
	if err != nil || !caps.Tools || !caps.Resources || caps.Prompts {
		t.Fatalf("caps=%+v err=%v", caps, err)
	}
	bodies := s.postBodies()
	if len(bodies) != 2 || !strings.Contains(bodies[1], "notifications/initialized") || strings.Contains(bodies[1], `"id"`) {
		t.Fatalf("handshake POSTs = %v; want initialize then an id-less initialized notification", bodies)
	}
}

func TestSSEInitialize_BooleanCapabilityFormIsRefused(t *testing.T) {
	s := newRawSSEServer(t, func(req rawRPC, push func(string)) {
		if req.Method == "initialize" {
			push(messageFrame(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":true}}}`, req.ID)))
		}
	})
	transport := NewSSETransportWithHeaders(s.URL, 2*time.Second, nil)
	err := transport.Connect(context.Background())
	if err == nil || !strings.Contains(err.Error(), "must be an object") {
		t.Fatalf("Connect error = %v", err)
	}
	if transport.IsConnected() {
		t.Fatal("connected with an unusable capability observation")
	}
}

// A server-initiated request is not the answer to ours, even if it reuses the
// same numeric ID; delivering it would make a request body the call's result.
func TestSSEServerRequestWithCollidingIDIsNotDeliveredAsAResponse(t *testing.T) {
	s := newRawSSEServer(t, handshakeThen(func(req rawRPC, push func(string)) {
		push(messageFrame(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"method":"roots/list"}`, req.ID)))
		push(messageFrame(`{"jsonrpc":"2.0","method":"notifications/message","params":{"level":"info"}}`))
		push(messageFrame(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"tools":[{"name":"real","inputSchema":{"type":"object"}}]}}`, req.ID)))
	}))
	tools, err := connectSSE(t, s).ListTools(context.Background())
	if err != nil || len(tools) != 1 || tools[0].Name != "real" {
		t.Fatalf("tools=%+v err=%v", tools, err)
	}
}

func TestSSEDuplicateTerminalResponseIsIgnored(t *testing.T) {
	s := newRawSSEServer(t, handshakeThen(func(req rawRPC, push func(string)) {
		push(messageFrame(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"first"}]}}`, req.ID)))
		push(messageFrame(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"second"}]}}`, req.ID)))
	}))
	result, err := connectSSE(t, s).CallTool(context.Background(), "t", nil)
	if err != nil || !result.Success || !strings.Contains(string(result.Output), "first") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

// SSE permits a message event with no event: line (the default type) and a
// data field with no space after the colon. The old parser dropped the first
// such event entirely.
func TestSSEEventWithoutTypeLineIsAMessage(t *testing.T) {
	s := newRawSSEServer(t, handshakeThen(func(req rawRPC, push func(string)) {
		push(fmt.Sprintf("data:{\"jsonrpc\":\"2.0\",\"id\":%s,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"untyped\"}]}}\n\n", req.ID))
	}))
	result, err := connectSSE(t, s).CallTool(context.Background(), "t", nil)
	if err != nil || !result.Success || !strings.Contains(string(result.Output), "untyped") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestSSEIsErrorResultIsAToolFailure(t *testing.T) {
	s := newRawSSEServer(t, handshakeThen(func(req rawRPC, push func(string)) {
		push(messageFrame(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"quota exceeded"}],"isError":true}}`, req.ID)))
	}))
	result, err := connectSSE(t, s).CallTool(context.Background(), "t", nil)
	if err != nil || result.Success || !result.ToolError || result.Error != "quota exceeded" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestSSEOversizedEventEndsTheStreamWithItsCause(t *testing.T) {
	old := maxWireMessageBytes
	maxWireMessageBytes = 4096
	t.Cleanup(func() { maxWireMessageBytes = old })
	s := newRawSSEServer(t, handshakeThen(func(req rawRPC, push func(string)) {
		push(messageFrame(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"tools":[],"pad":"%s"}}`, req.ID, strings.Repeat("x", 16384))))
	}))
	transport := connectSSE(t, s)
	done := make(chan error, 1)
	go func() { _, err := transport.ListTools(context.Background()); done <- err }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "wire limit") {
			t.Fatalf("err = %v, want the wire-limit cause", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("an oversized event left the call hanging")
	}
}

func TestSSERequestIDsAreNeverReusedAcrossReconnect(t *testing.T) {
	s := newRawSSEServer(t, handshakeThen(func(req rawRPC, push func(string)) {
		push(messageFrame(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"tools":[]}}`, req.ID)))
	}))
	transport := connectSSE(t, s)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, err := transport.ListTools(ctx); err != nil {
			t.Fatal(err)
		}
	}
	_ = transport.Disconnect()
	if err := transport.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := transport.ListTools(ctx); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, body := range s.postBodies() {
		var req rawRPC
		_ = json.Unmarshal([]byte(body), &req)
		if len(req.ID) == 0 {
			continue
		}
		if seen[string(req.ID)] {
			t.Fatalf("request id %s reused: %v", req.ID, s.postBodies())
		}
		seen[string(req.ID)] = true
	}
}

// --- shared parsers ---------------------------------------------------------

func TestScanSSEFollowsTheFieldGrammar(t *testing.T) {
	stream := ": comment\r\n" +
		"data: one\r\ndata: two\r\n\r\n" + // multi-line data, CRLF endings, default type
		"event: endpoint\ndata:/rpc/1\n\n" + // no space after the colon
		"id: 7\nretry: 100\nevent: custom\ndata: x\n\n" +
		"event: empty\n\n" + // no data: not dispatched
		"event: unterminated\ndata: lost" // never completed: not dispatched
	type ev struct{ name, data string }
	var got []ev
	if err := scanSSE(strings.NewReader(stream), 1<<20, func(name, data string) bool {
		got = append(got, ev{name, data})
		return false
	}); err != nil {
		t.Fatal(err)
	}
	want := []ev{{"message", "one\ntwo"}, {"endpoint", "/rpc/1"}, {"custom", "x"}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func TestScanSSERefusesOversizedInput(t *testing.T) {
	long := "data: " + strings.Repeat("a", 5000) + "\n\n"
	if err := scanSSE(strings.NewReader(long), 1024, func(string, string) bool { return false }); err == nil || !strings.Contains(err.Error(), "wire limit") {
		t.Fatalf("oversized line: err=%v", err)
	}
	multi := strings.Repeat("data: "+strings.Repeat("b", 500)+"\n", 10) + "\n"
	if err := scanSSE(strings.NewReader(multi), 2048, func(string, string) bool { return false }); err == nil || !strings.Contains(err.Error(), "wire limit") {
		t.Fatalf("oversized event across lines: err=%v", err)
	}
}

func TestMessageKindsAreClassifiedByShape(t *testing.T) {
	cases := []struct {
		raw  string
		want rpcKind
	}{
		{`{"jsonrpc":"2.0","id":1,"result":{}}`, rpcResponse},
		{`{"jsonrpc":"2.0","id":1,"error":{"code":1,"message":"m"}}`, rpcResponse},
		{`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"parse"}}`, rpcResponse},
		{`{"jsonrpc":"2.0","id":1,"method":"ping"}`, rpcRequest},
		{`{"jsonrpc":"2.0","method":"notifications/x"}`, rpcNotification},
		{`{"jsonrpc":"2.0","id":1}`, rpcInvalid},
		{`{"jsonrpc":"2.0","id":1,"result":{},"error":{"code":1,"message":"m"}}`, rpcInvalid},
	}
	for _, tc := range cases {
		msg, err := decodeRPCMessage([]byte(tc.raw))
		if err != nil {
			t.Fatalf("%s: %v", tc.raw, err)
		}
		if got := msg.kind(); got != tc.want {
			t.Errorf("%s classified as %v, want %v", tc.raw, got, tc.want)
		}
	}
	for _, bad := range []string{`{"id":1,"result":{}}`, `{"jsonrpc":"1.0","id":1,"result":{}}`, `[{"jsonrpc":"2.0"}]`, `nope`} {
		if _, err := decodeRPCMessage([]byte(bad)); err == nil {
			t.Errorf("%s decoded without error", bad)
		}
	}
	if id, ok := (&rpcMessage{ID: json.RawMessage(`"7"`)}).intID(); ok {
		t.Errorf("string id accepted as %d", id)
	}
	if id, ok := (&rpcMessage{ID: json.RawMessage(`1.5`)}).intID(); ok {
		t.Errorf("fractional id accepted as %d", id)
	}
}

// --- stdio transport --------------------------------------------------------

// The stdio server is this test binary re-executed: TestStdioHelperServer is a
// newline-delimited JSON-RPC server selected by MCP_STDIO_MODE.
func TestStdioHelperServer(t *testing.T) {
	mode := os.Getenv("MCP_STDIO_MODE")
	if mode == "" {
		t.Skip("helper process only")
	}
	in := bufio.NewReader(os.Stdin)
	out := os.Stdout
	var seen []string
	for {
		line, err := in.ReadBytes('\n')
		if err != nil {
			return
		}
		var req rawRPC
		_ = json.Unmarshal(line, &req)
		seen = append(seen, req.Method)
		write := func(result string) { fmt.Fprintf(out, `{"jsonrpc":"2.0","id":%s,"result":%s}`+"\n", req.ID, result) }
		switch req.Method {
		case "initialize":
			if mode == "boolean-caps" {
				write(`{"protocolVersion":"2024-11-05","capabilities":{"tools":true}}`)
			} else {
				write(goodInitialize)
			}
		case "notifications/initialized":
		case "tools/list":
			switch mode {
			case "oversize":
				fmt.Fprintf(out, `{"jsonrpc":"2.0","id":%s,"result":{"tools":[],"pad":"%s"}}`+"\n", req.ID, strings.Repeat("x", 1<<16))
			case "exit-on-list":
				os.Exit(0)
			default:
				write(`{"tools":[{"name":"` + strings.Join(seen, "|") + `","inputSchema":{"type":"object"}}]}`)
			}
		case "tools/call":
			fmt.Fprintf(out, `{"jsonrpc":"2.0","method":"notifications/progress","params":{"progress":1}}`+"\n")
			fmt.Fprintf(out, `{"jsonrpc":"2.0","id":%s,"method":"roots/list"}`+"\n", req.ID)
			write(`{"content":[{"type":"text","text":"boom"}],"isError":true}`)
		}
	}
}

func stdioTransport(t *testing.T, mode string) *StdioTransport {
	t.Helper()
	t.Setenv("MCP_STDIO_MODE", mode)
	return NewStdioTransport(os.Args[0] + " -test.run=^TestStdioHelperServer$")
}

func TestStdioHandshakeObservesObjectCapabilitiesAndNotifies(t *testing.T) {
	transport := stdioTransport(t, "ok")
	if err := transport.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = transport.Disconnect() })
	caps, err := transport.GetCapabilities(context.Background())
	if err != nil || !caps.Tools || !caps.Resources || caps.Prompts {
		t.Fatalf("caps=%+v err=%v", caps, err)
	}
	// The server recorded the order it saw: the notification precedes every
	// request that follows the handshake.
	tools, err := transport.ListTools(context.Background())
	if err != nil || len(tools) != 1 || tools[0].Name != "initialize|notifications/initialized|tools/list" {
		t.Fatalf("tools=%+v err=%v", tools, err)
	}
}

func TestStdioBooleanCapabilityFormIsRefusedAtConnect(t *testing.T) {
	transport := stdioTransport(t, "boolean-caps")
	err := transport.Connect(context.Background())
	if err == nil || !strings.Contains(err.Error(), "must be an object") {
		t.Fatalf("Connect error = %v", err)
	}
	if transport.IsConnected() {
		t.Fatal("connected after a refused handshake")
	}
}

func TestStdioIsErrorResultIsAToolFailureAndServerRequestsAreNotAnswers(t *testing.T) {
	transport := stdioTransport(t, "ok")
	if err := transport.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = transport.Disconnect() })
	result, err := transport.CallTool(context.Background(), "t", nil)
	if err != nil || result.Success || !result.ToolError || result.Error != "boom" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestStdioOversizedMessageFailsPendingCallsWithItsCause(t *testing.T) {
	old := maxWireMessageBytes
	maxWireMessageBytes = 4096
	t.Cleanup(func() { maxWireMessageBytes = old })
	transport := stdioTransport(t, "oversize")
	if err := transport.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = transport.Disconnect() })
	done := make(chan error, 1)
	go func() { _, err := transport.ListTools(context.Background()); done <- err }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "wire limit") {
			t.Fatalf("err = %v, want the wire-limit cause", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("an oversized message left the call hanging")
	}
	if transport.IsConnected() {
		t.Fatal("transport still reports connected after its stream was lost")
	}
}

func TestStdioServerExitWakesPendingCalls(t *testing.T) {
	transport := stdioTransport(t, "exit-on-list")
	if err := transport.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = transport.Disconnect() })
	done := make(chan error, 1)
	go func() { _, err := transport.ListTools(context.Background()); done <- err }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "connection closed") {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a dead server left the call hanging")
	}
}

func TestStdioReconnectAfterDisconnectIsClean(t *testing.T) {
	transport := stdioTransport(t, "ok")
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := transport.Connect(ctx); err != nil {
			t.Fatalf("Connect #%d: %v", i, err)
		}
		if _, err := transport.ListTools(ctx); err != nil {
			t.Fatalf("ListTools #%d: %v", i, err)
		}
		if err := transport.Disconnect(); err != nil {
			t.Fatal(err)
		}
	}
}
