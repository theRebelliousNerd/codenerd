package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type sseTestResponder func(session int, request mcpRequest) (any, bool)

type sseTestServer struct {
	server    *httptest.Server
	mu        sync.Mutex
	next      int
	responses map[int]chan []byte
	requests  chan sseTestRequest
}

type sseTestRequest struct {
	session int
	request mcpRequest
}

func newSSETestServer(t *testing.T, respond sseTestResponder) *sseTestServer {
	t.Helper()

	fixture := &sseTestServer{
		responses: make(map[int]chan []byte),
		requests:  make(chan sseTestRequest, 16),
	}
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/events" {
			fixture.serveEvents(w, r)
			return
		}
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/rpc/") {
			fixture.serveRPC(w, r, respond)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (s *sseTestServer) serveEvents(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.next++
	session := s.next
	responses := make(chan []byte, 16)
	s.responses[session] = responses
	s.mu.Unlock()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "event: endpoint\ndata: /rpc/%d\n\n", session)
	flusher.Flush()

	for {
		select {
		case data := <-responses:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *sseTestServer) serveRPC(w http.ResponseWriter, r *http.Request, respond sseTestResponder) {
	session, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/rpc/"))
	if err != nil {
		http.Error(w, "bad session", http.StatusBadRequest)
		return
	}
	var request mcpRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.requests <- sseTestRequest{session: session, request: request}

	if result, ok := respond(session, request); ok {
		resultJSON, err := json.Marshal(result)
		if err != nil {
			http.Error(w, "bad fixture result", http.StatusInternalServerError)
			return
		}
		responseJSON, err := json.Marshal(mcpResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
			Result:  resultJSON,
		})
		if err != nil {
			http.Error(w, "bad fixture response", http.StatusInternalServerError)
			return
		}
		s.mu.Lock()
		responses := s.responses[session]
		s.mu.Unlock()
		if responses != nil {
			responses <- responseJSON
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *sseTestServer) url() string { return s.server.URL + "/events" }

func TestSSETransport_LongLivedStreamOutlastsRequestTimeout(t *testing.T) {
	respond := func(_ int, request mcpRequest) (any, bool) {
		switch request.Method {
		case "initialize":
			return map[string]any{"capabilities": map[string]bool{"tools": true}}, true
		case "tools/list":
			return map[string]any{"tools": []any{}}, true
		default:
			return nil, false
		}
	}
	fixture := newSSETestServer(t, respond)
	requestTimeout := 200 * time.Millisecond
	transport := NewSSETransportWithHeaders(fixture.url(), requestTimeout, nil)
	t.Cleanup(func() { _ = transport.Disconnect() })

	connectCtx, cancelConnect := context.WithCancel(context.Background())
	if err := transport.Connect(connectCtx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	cancelConnect()
	if _, err := transport.ListTools(context.Background()); err != nil {
		t.Fatalf("initial ListTools round trip: %v", err)
	}
	time.Sleep(3 * requestTimeout)
	if !transport.IsConnected() {
		t.Fatal("healthy event stream was closed by the per-request timeout")
	}
	if _, err := transport.ListTools(context.Background()); err != nil {
		t.Fatalf("ListTools after the request timeout elapsed: %v", err)
	}
}

func TestSSETransport_ReconnectUsesFreshEndpointAndRoundTrips(t *testing.T) {
	respond := func(_ int, request mcpRequest) (any, bool) {
		switch request.Method {
		case "initialize":
			return map[string]any{"capabilities": map[string]bool{"tools": true}}, true
		case "tools/list":
			return map[string]any{"tools": []any{}}, true
		case "tools/call":
			return map[string]any{"content": []any{map[string]any{"type": "text", "text": "echoed"}}}, true
		default:
			return nil, false
		}
	}
	fixture := newSSETestServer(t, respond)
	transport := NewSSETransportWithHeaders(fixture.url(), time.Second, nil)
	t.Cleanup(func() { _ = transport.Disconnect() })

	for attempt := 1; attempt <= 2; attempt++ {
		if err := transport.Connect(context.Background()); err != nil {
			t.Fatalf("Connect attempt %d: %v", attempt, err)
		}
		if _, err := transport.ListTools(context.Background()); err != nil {
			t.Fatalf("ListTools attempt %d: %v", attempt, err)
		}
		if attempt == 1 {
			if err := transport.Disconnect(); err != nil {
				t.Fatalf("Disconnect: %v", err)
			}
			drainSSERequests(fixture)
		}
		if attempt == 2 {
			result, err := transport.CallTool(context.Background(), "echo", map[string]any{"value": attempt})
			if err != nil {
				t.Fatalf("CallTool attempt %d: %v", attempt, err)
			}
			if !result.Success || !strings.Contains(string(result.Output), "echoed") {
				t.Fatalf("CallTool attempt %d result = %+v", attempt, result)
			}
		}
	}

	fixture.mu.Lock()
	streamCount := fixture.next
	fixture.mu.Unlock()
	if streamCount != 2 {
		t.Fatalf("opened %d event streams across reconnect, want 2", streamCount)
	}
	wantMethods := map[string]bool{"initialize": false, "tools/list": false, "tools/call": false}
	seenIDs := make(map[int]bool, len(wantMethods))
	for i := 0; i < len(wantMethods); i++ {
		select {
		case request := <-fixture.requests:
			if request.session != 2 {
				t.Errorf("request %q used stale event stream %d after reconnect", request.request.Method, request.session)
			}
			if _, ok := wantMethods[request.request.Method]; !ok {
				t.Errorf("unexpected request after reconnect: %q", request.request.Method)
			} else {
				wantMethods[request.request.Method] = true
			}
			if seenIDs[request.request.ID] {
				t.Errorf("request ID %d was reused on the reconnected stream", request.request.ID)
			}
			seenIDs[request.request.ID] = true
			if request.request.Method == "tools/call" {
				params, ok := request.request.Params.(map[string]any)
				if !ok || params["name"] != "echo" {
					t.Errorf("tools/call params = %#v, want echo tool", request.request.Params)
					continue
				}
				arguments, ok := params["arguments"].(map[string]any)
				if !ok || arguments["value"] != float64(2) {
					t.Errorf("tools/call arguments = %#v, want value 2", params["arguments"])
				}
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for initialize/list/call round trips")
		}
	}
	for method, seen := range wantMethods {
		if !seen {
			t.Errorf("did not observe %q on the reconnected stream", method)
		}
	}
}

func TestSSETransport_CancelledCallIsRemovedAndReturnsPromptly(t *testing.T) {
	respond := func(_ int, request mcpRequest) (any, bool) {
		if request.Method == "initialize" {
			return map[string]any{"capabilities": map[string]bool{"tools": true}}, true
		}
		return nil, false
	}
	fixture := newSSETestServer(t, respond)
	transport := NewSSETransportWithHeaders(fixture.url(), time.Second, nil)
	t.Cleanup(func() { _ = transport.Disconnect() })
	if err := transport.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := transport.call(ctx, "tools/list", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancelled call error = %v, want context deadline", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("cancelled call took %v, want prompt return", elapsed)
	}

	transport.mu.RLock()
	pending := len(transport.pending)
	transport.mu.RUnlock()
	if pending != 0 {
		t.Fatalf("pending calls after cancellation = %d, want 0", pending)
	}
}

func TestSSETransport_DisconnectReleasesPendingCall(t *testing.T) {
	respond := func(_ int, request mcpRequest) (any, bool) {
		if request.Method == "initialize" {
			return map[string]any{"capabilities": map[string]bool{"tools": true}}, true
		}
		return nil, false
	}
	fixture := newSSETestServer(t, respond)
	transport := NewSSETransportWithHeaders(fixture.url(), time.Second, nil)
	t.Cleanup(func() { _ = transport.Disconnect() })
	if err := transport.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	callDone := make(chan error, 1)
	go func() {
		_, err := transport.call(context.Background(), "tools/list", nil)
		callDone <- err
	}()
	if request := waitForSSERequest(t, fixture, "tools/list"); request.session != 1 {
		t.Fatalf("pending call used stream %d, want 1", request.session)
	}

	if err := transport.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	select {
	case err := <-callDone:
		if err == nil || !strings.Contains(err.Error(), "connection closed") {
			t.Fatalf("pending call error = %v, want connection closed", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Disconnect did not release the pending call")
	}
	transport.mu.RLock()
	pending := len(transport.pending)
	transport.mu.RUnlock()
	if pending != 0 {
		t.Fatalf("pending calls after disconnect = %d, want 0", pending)
	}
}

func TestSSETransport_ResponseAndDisconnectAreSafeConcurrently(t *testing.T) {
	transport := NewSSETransportWithHeaders("http://127.0.0.1/events", time.Second, nil)
	message := `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`
	for iteration := 0; iteration < 500; iteration++ {
		transport.mu.Lock()
		transport.generation++
		generation := transport.generation
		transport.streamActive = true
		transport.connected = true
		transport.connectionDone = make(chan struct{})
		transport.pending = map[int]chan *mcpResponse{1: make(chan *mcpResponse, 1)}
		transport.mu.Unlock()

		start := make(chan struct{})
		eventResult := make(chan any, 1)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			defer func() { eventResult <- recover() }()
			<-start
			transport.handleEvent(generation, "message", message)
		}()
		go func() {
			defer wg.Done()
			<-start
			_ = transport.Disconnect()
		}()
		close(start)
		wg.Wait()
		if recovered := <-eventResult; recovered != nil {
			t.Fatalf("response/disconnect iteration %d panicked: %v", iteration, recovered)
		}
	}
}

func waitForSSERequest(t *testing.T, fixture *sseTestServer, method string) sseTestRequest {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case request := <-fixture.requests:
			if request.request.Method == method {
				return request
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for SSE method %q", method)
			return sseTestRequest{}
		}
	}
}

func drainSSERequests(fixture *sseTestServer) {
	for {
		select {
		case <-fixture.requests:
		default:
			return
		}
	}
}
