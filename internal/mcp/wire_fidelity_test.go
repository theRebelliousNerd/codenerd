package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// These tests pin the wire contract in docs/audits/mcp-versioned-wire-fidelity.md.
// Every server below speaks hand-written JSON copied from the specification's
// message shapes; none goes through this package's DTOs, so a decoder that only
// agrees with its own types cannot pass.

type rawRPC struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// stubServer is a single-endpoint JSON-RPC server whose behavior per method is
// a plain function; it records every message it receives, notifications
// included.
type stubServer struct {
	*httptest.Server
	mu       sync.Mutex
	received []rawRPC
	bodies   []string
	handler  func(w http.ResponseWriter, req rawRPC)
}

func newStubServer(t *testing.T, handler func(w http.ResponseWriter, req rawRPC)) *stubServer {
	t.Helper()
	s := &stubServer{handler: handler}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var req rawRPC
		_ = json.Unmarshal(body, &req)
		s.mu.Lock()
		s.received = append(s.received, req)
		s.bodies = append(s.bodies, string(body))
		s.mu.Unlock()
		s.handler(w, req)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *stubServer) methods() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.received))
	for _, r := range s.received {
		out = append(out, r.Method)
	}
	return out
}

// reply writes a JSON-RPC response with the request's own ID.
func reply(w http.ResponseWriter, req rawRPC, resultJSON string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, resultJSON)
}

const goodInitialize = `{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":true},"resources":{"subscribe":true},"experimental":{"vendor":{}}},"serverInfo":{"name":"spec-server","version":"2.3"}}`

// standardHandler answers the handshake and delegates other methods.
func standardHandler(rest func(w http.ResponseWriter, req rawRPC)) func(http.ResponseWriter, rawRPC) {
	return func(w http.ResponseWriter, req rawRPC) {
		switch req.Method {
		case "initialize":
			reply(w, req, goodInitialize)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		default:
			rest(w, req)
		}
	}
}

func connectHTTP(t *testing.T, s *stubServer) *HTTPTransport {
	t.Helper()
	transport := NewHTTPTransportWithHeaders(s.URL, 2*time.Second, nil)
	if err := transport.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = transport.Disconnect() })
	return transport
}

// --- capability observation -------------------------------------------------

// Spec-shaped capabilities are objects with sub-capabilities. The old decoder
// failed on them and then "succeeded" by decoding the envelope into booleans,
// reporting a server that declared tools and resources as having neither.
func TestHTTPInitialize_ObjectCapabilitiesAreObserved(t *testing.T) {
	s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {}))
	transport := connectHTTP(t, s)
	caps, err := transport.GetCapabilities(context.Background())
	if err != nil {
		t.Fatalf("GetCapabilities: %v", err)
	}
	if !caps.Tools || !caps.Resources {
		t.Fatalf("declared tools and resources observed as %+v", caps)
	}
	if caps.Prompts || caps.Logging {
		t.Fatalf("undeclared capabilities invented: %+v", caps)
	}
	if caps.ProtocolVersion != protocolVersionLegacyInit || caps.ServerName != "spec-server" {
		t.Fatalf("version/server info not carried: %+v", caps)
	}
}

func TestHTTPInitialize_EmptyObjectCapabilityIsDeclared(t *testing.T) {
	s := newStubServer(t, func(w http.ResponseWriter, req rawRPC) {
		if req.Method == "initialize" {
			reply(w, req, `{"protocolVersion":"2024-11-05","capabilities":{"prompts":{}},"serverInfo":{"name":"x","version":"1"}}`)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	caps, err := connectHTTP(t, s).GetCapabilities(context.Background())
	if err != nil || !caps.Prompts || caps.Tools {
		t.Fatalf("empty-object prompts capability: caps=%+v err=%v", caps, err)
	}
}

func TestHTTPInitialize_ServerWithNoCapabilitiesIsLegitimatelyEmpty(t *testing.T) {
	s := newStubServer(t, func(w http.ResponseWriter, req rawRPC) {
		if req.Method == "initialize" {
			reply(w, req, `{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"x","version":"1"}}`)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	caps, err := connectHTTP(t, s).GetCapabilities(context.Background())
	if err != nil || caps.Tools || caps.Resources || caps.Prompts || caps.Logging {
		t.Fatalf("empty capability set must stay empty and valid: caps=%+v err=%v", caps, err)
	}
}

func TestHTTPInitialize_MalformedEnvelopesAreRefusedNotEmptied(t *testing.T) {
	cases := map[string]struct{ result, want string }{
		"legacy boolean form":      {`{"protocolVersion":"2024-11-05","capabilities":{"tools":true}}`, `capability "tools" must be an object`},
		"capability null":          {`{"protocolVersion":"2024-11-05","capabilities":{"tools":null}}`, `capability "tools" must be an object`},
		"capabilities missing":     {`{"protocolVersion":"2024-11-05","serverInfo":{"name":"x","version":"1"}}`, "capabilities object is missing"},
		"capabilities not object":  {`{"protocolVersion":"2024-11-05","capabilities":"yes"}`, "capabilities must be an object"},
		"capabilities array":       {`{"protocolVersion":"2024-11-05","capabilities":[]}`, "capabilities must be an object"},
		"protocolVersion missing":  {`{"capabilities":{"tools":{}}}`, "protocolVersion is missing"},
		"result is not an object":  {`"ok"`, "malformed initialize result"},
		"capability list of names": {`{"protocolVersion":"2024-11-05","capabilities":{"tools":["a"]}}`, `capability "tools" must be an object`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := newStubServer(t, func(w http.ResponseWriter, req rawRPC) {
				if req.Method == "initialize" {
					reply(w, req, tc.result)
					return
				}
				w.WriteHeader(http.StatusAccepted)
			})
			transport := NewHTTPTransportWithHeaders(s.URL, 2*time.Second, nil)
			err := transport.Connect(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Connect error = %v, want it to contain %q", err, tc.want)
			}
			if transport.IsConnected() {
				t.Fatal("a transport with an unusable capability observation reports connected")
			}
			for _, method := range s.methods() {
				if method == "notifications/initialized" {
					t.Fatal("initialized notification sent for a handshake the client rejected")
				}
			}
		})
	}
}

func TestHTTPInitialize_UnsupportedVersionIsOneActionableError(t *testing.T) {
	s := newStubServer(t, func(w http.ResponseWriter, req rawRPC) {
		reply(w, req, `{"protocolVersion":"2099-01-01","capabilities":{"tools":{}}}`)
	})
	err := NewHTTPTransportWithHeaders(s.URL, 2*time.Second, nil).Connect(context.Background())
	var unsupported *UnsupportedProtocolError
	if !errors.As(err, &unsupported) || unsupported.Returned != "2099-01-01" {
		t.Fatalf("Connect error = %v, want UnsupportedProtocolError for 2099-01-01", err)
	}
	if !strings.Contains(err.Error(), protocolVersionLegacyInit) {
		t.Fatalf("error does not name the supported version: %v", err)
	}
}

func TestHTTPInitialize_SendsInitializedNotificationBeforeAnyRequest(t *testing.T) {
	s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
		reply(w, req, `{"tools":[]}`)
	}))
	transport := connectHTTP(t, s)
	if _, err := transport.ListTools(context.Background()); err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := s.methods()
	want := []string{"initialize", "notifications/initialized", "tools/list"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("message order = %v, want %v", got, want)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.Contains(s.bodies[1], `"id"`) {
		t.Fatalf("the initialized notification carries an id: %s", s.bodies[1])
	}
}

func TestHTTPInitialize_FailedInitializedNotificationFailsConnect(t *testing.T) {
	s := newStubServer(t, func(w http.ResponseWriter, req rawRPC) {
		if req.Method == "initialize" {
			reply(w, req, goodInitialize)
			return
		}
		http.Error(w, "nope", http.StatusInternalServerError)
	})
	transport := NewHTTPTransportWithHeaders(s.URL, 2*time.Second, nil)
	err := transport.Connect(context.Background())
	if err == nil || !strings.Contains(err.Error(), "initialized notification") {
		t.Fatalf("Connect error = %v, want an initialized-notification failure", err)
	}
	if transport.IsConnected() {
		t.Fatal("connected without completing the handshake")
	}
}

// --- request identity -------------------------------------------------------

func TestHTTPRequestIDsAreNeverReused(t *testing.T) {
	s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
		switch req.Method {
		case "tools/list":
			reply(w, req, `{"tools":[]}`)
		case "tools/call":
			reply(w, req, `{"content":[{"type":"text","text":"ok"}]}`)
		default:
			reply(w, req, `{}`)
		}
	}))
	transport := connectHTTP(t, s)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := transport.ListTools(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := transport.CallTool(ctx, "t", nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := transport.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, req := range s.received {
		if strings.HasPrefix(req.Method, "notifications/") {
			continue
		}
		id := string(req.ID)
		if id == "" || seen[id] {
			t.Fatalf("request id %q reused or missing across %v", id, s.received)
		}
		seen[id] = true
	}
}

func TestHTTPResponseWithWrongIDIsNeverSuccess(t *testing.T) {
	s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":9999,"result":{"content":[{"type":"text","text":"someone else's answer"}]}}`)
	}))
	transport := connectHTTP(t, s)
	result, err := transport.CallTool(context.Background(), "t", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || !strings.Contains(result.Error, "does not match request id") {
		t.Fatalf("wrong-ID response was accepted: %+v", result)
	}
	if _, err := transport.ListTools(context.Background()); err == nil {
		t.Fatal("ListTools accepted a response to a different request")
	}
}

func TestHTTPEnvelopeFaultsAreNotSuccess(t *testing.T) {
	bodies := map[string]string{
		"wrong jsonrpc version":   `{"jsonrpc":"1.0","id":%s,"result":{}}`,
		"string id":               `{"jsonrpc":"2.0","id":"%s","result":{}}`,
		"request instead":         `{"jsonrpc":"2.0","id":%s,"method":"sampling/createMessage"}`,
		"neither result nor err":  `{"jsonrpc":"2.0","id":%s}`,
		"batch array":             `[{"jsonrpc":"2.0","id":%s,"result":{}}]`,
		"result and error":        `{"jsonrpc":"2.0","id":%s,"result":{},"error":{"code":1,"message":"m"}}`,
		"notification as answer":  `{"jsonrpc":"2.0","method":"notifications/message"}`,
		"truncated json document": `{"jsonrpc":"2.0","id":%s,"result":{"conte`,
	}
	for name, template := range bodies {
		t.Run(name, func(t *testing.T) {
			s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
				w.Header().Set("Content-Type", "application/json")
				id := strings.Trim(string(req.ID), `"`)
				if strings.Contains(template, "%s") {
					fmt.Fprintf(w, template, id)
				} else {
					fmt.Fprint(w, template)
				}
			}))
			transport := connectHTTP(t, s)
			result, err := transport.CallTool(context.Background(), "t", nil)
			if err != nil {
				t.Fatal(err)
			}
			if result.Success {
				t.Fatalf("malformed response was reported as success: %+v", result)
			}
		})
	}
}

// --- response forms ---------------------------------------------------------

func TestHTTPResponseMayBeARequestScopedEventStream(t *testing.T) {
	s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
		w.Header().Set("Content-Type", "text/event-stream")
		// A progress notification precedes the terminal response; it is not
		// the answer and must not end the call.
		fmt.Fprint(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{\"progress\":1}}\n\n")
		fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":%s,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"streamed\"}]}}\n\n", req.ID)
	}))
	transport := connectHTTP(t, s)
	result, err := transport.CallTool(context.Background(), "t", nil)
	if err != nil || !result.Success || !strings.Contains(string(result.Output), "streamed") {
		t.Fatalf("event-stream response: result=%+v err=%v", result, err)
	}
}

func TestHTTPEventStreamFaultsAreNotSuccess(t *testing.T) {
	cases := map[string]string{
		"wrong id":        "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":424242,\"result\":{}}\n\n",
		"no response":     "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\"}\n\n",
		"empty stream":    "",
		"malformed event": "event: message\ndata: not json\n\n",
	}
	for name, stream := range cases {
		t.Run(name, func(t *testing.T) {
			s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, stream)
			}))
			transport := connectHTTP(t, s)
			result, err := transport.CallTool(context.Background(), "t", nil)
			if err != nil {
				t.Fatal(err)
			}
			if result.Success {
				t.Fatalf("faulty stream reported success: %+v", result)
			}
		})
	}
}

func TestHTTPUnsupportedContentTypeIsRefused(t *testing.T) {
	s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html>login</html>")
	}))
	transport := connectHTTP(t, s)
	result, _ := transport.CallTool(context.Background(), "t", nil)
	if result.Success || !strings.Contains(result.Error, "unsupported response content type") {
		t.Fatalf("html response: %+v", result)
	}
}

func TestHTTPOversizedResponseIsAnErrorNotACut(t *testing.T) {
	old := maxWireMessageBytes
	maxWireMessageBytes = 2048
	t.Cleanup(func() { maxWireMessageBytes = old })
	s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
		reply(w, req, fmt.Sprintf(`{"content":[{"type":"text","text":%q}]}`, strings.Repeat("x", 8192)))
	}))
	transport := connectHTTP(t, s)
	result, _ := transport.CallTool(context.Background(), "t", nil)
	if result.Success || !strings.Contains(result.Error, "wire limit") {
		t.Fatalf("oversized body: %+v", result)
	}
	if len(result.Output) != 0 {
		t.Fatal("a cut document was returned as output")
	}
}

func TestHTTPErrorBodyQuotedIntoErrorsIsBounded(t *testing.T) {
	s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
		http.Error(w, strings.Repeat("secret-ish ", 2000), http.StatusInternalServerError)
	}))
	transport := connectHTTP(t, s)
	result, _ := transport.CallTool(context.Background(), "t", nil)
	if result.Success || len(result.Error) > 2*maxDiagnosticBody {
		t.Fatalf("diagnostic body not bounded: %d bytes", len(result.Error))
	}
}

func TestHTTPRefusesCrossHostRedirectThatWouldForwardCredentials(t *testing.T) {
	var leaked sync.Map
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked.Store("hit", r.Header.Get("X-Api-Key"))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(other.Close)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(origin.Close)
	transport := NewHTTPTransportWithHeaders(origin.URL, 2*time.Second, map[string]string{"X-Api-Key": "hunter2"})
	err := transport.Connect(context.Background())
	if err == nil || !strings.Contains(err.Error(), "refusing redirect") {
		t.Fatalf("Connect error = %v, want a refused cross-host redirect", err)
	}
	if _, hit := leaked.Load("hit"); hit {
		t.Fatal("the credential header reached the redirect target")
	}
}

// --- tool results -----------------------------------------------------------

func TestToolResultOutcomeFollowsTheWire(t *testing.T) {
	cases := []struct {
		name        string
		result      string
		wantSuccess bool
		wantToolErr bool
		wantError   string
	}{
		{"ordinary success", `{"content":[{"type":"text","text":"fine"}]}`, true, false, ""},
		{"isError false", `{"content":[{"type":"text","text":"fine"}],"isError":false}`, true, false, ""},
		{"isError true carries remote text", `{"content":[{"type":"text","text":"disk full"},{"type":"text","text":"retry later"}],"isError":true}`, false, true, "disk full\nretry later"},
		{"isError true without text", `{"content":[{"type":"image","data":"AA==","mimeType":"image/png"}],"isError":true}`, false, true, "isError without a message"},
		{"isError wrong type", `{"content":[],"isError":"yes"}`, false, false, "isError must be a boolean"},
		{"content not an array", `{"content":"ok"}`, false, false, "content must be an array"},
		{"result not an object", `["ok"]`, false, false, "not a JSON object"},
		{"result null", `null`, false, false, "invalid response"},
		{"structured content only", `{"structuredContent":{"sum":8}}`, true, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
				reply(w, req, tc.result)
			}))
			result, err := connectHTTP(t, s).CallTool(context.Background(), "t", nil)
			if err != nil {
				t.Fatal(err)
			}
			if result.Success != tc.wantSuccess || result.ToolError != tc.wantToolErr {
				t.Fatalf("Success=%v ToolError=%v, want %v/%v (%+v)", result.Success, result.ToolError, tc.wantSuccess, tc.wantToolErr, result)
			}
			if !strings.Contains(result.Error, tc.wantError) {
				t.Fatalf("Error = %q, want it to contain %q", result.Error, tc.wantError)
			}
			if tc.result != "null" && string(result.Output) != tc.result {
				t.Fatalf("the remote's full result was not preserved: %s", result.Output)
			}
		})
	}
}

func TestJSONRPCErrorIsAFailureNotAToolError(t *testing.T) {
	s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"error":{"code":-32602,"message":"unknown tool"}}`, req.ID)
	}))
	result, err := connectHTTP(t, s).CallTool(context.Background(), "nope", nil)
	if err != nil || result.Success || result.ToolError || !strings.Contains(result.Error, "unknown tool") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

// --- catalog pagination -----------------------------------------------------

// pagedServer serves tools/resources/prompts lists in pages keyed by cursor.
func pagedServer(t *testing.T, pages map[string]string) *stubServer {
	return newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
		var params struct {
			Cursor string `json:"cursor"`
		}
		_ = json.Unmarshal(req.Params, &params)
		page, ok := pages[req.Method+"|"+params.Cursor]
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"error":{"code":-32602,"message":"bad cursor %s"}}`, req.ID, params.Cursor)
			return
		}
		reply(w, req, page)
	}))
}

func toolPage(next string, names ...string) string {
	items := make([]string, len(names))
	for i, n := range names {
		items[i] = fmt.Sprintf(`{"name":%q,"description":"d","inputSchema":{"type":"object"}}`, n)
	}
	page := `{"tools":[` + strings.Join(items, ",") + `]`
	if next != "" {
		page += fmt.Sprintf(`,"nextCursor":%q`, next)
	}
	return page + "}"
}

func TestToolListFollowsNextCursorToExhaustion(t *testing.T) {
	s := pagedServer(t, map[string]string{
		"tools/list|":   toolPage("c1", "a"),
		"tools/list|c1": toolPage("c2", "b"),
		"tools/list|c2": toolPage("", "target"),
	})
	tools, err := connectHTTP(t, s).ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var names []string
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "a,b,target" {
		t.Fatalf("tools = %v; a tool only on the last page must be discoverable", names)
	}
}

func TestResourceAndPromptListsFollowNextCursor(t *testing.T) {
	s := pagedServer(t, map[string]string{
		"resources/list|":   `{"resources":[{"uri":"file:///a"}],"nextCursor":"r2"}`,
		"resources/list|r2": `{"resources":[{"uri":"file:///b"}]}`,
		"prompts/list|":     `{"prompts":[{"name":"p1"}],"nextCursor":"q2"}`,
		"prompts/list|q2":   `{"prompts":[{"name":"p2"}]}`,
	})
	transport := connectHTTP(t, s)
	resources, err := transport.ListResources(context.Background())
	if err != nil || len(resources) != 2 || resources[1].URI != "file:///b" {
		t.Fatalf("resources = %+v err=%v", resources, err)
	}
	prompts, err := transport.ListPrompts(context.Background())
	if err != nil || len(prompts) != 2 || prompts[1].Name != "p2" {
		t.Fatalf("prompts = %+v err=%v", prompts, err)
	}
}

func TestRepeatedCursorIsIncompleteNotComplete(t *testing.T) {
	s := pagedServer(t, map[string]string{
		"tools/list|":     toolPage("loop", "a"),
		"tools/list|loop": toolPage("loop", "b"),
	})
	tools, err := connectHTTP(t, s).ListTools(context.Background())
	var incomplete *IncompleteCatalogError
	if !errors.As(err, &incomplete) {
		t.Fatalf("err = %v, want IncompleteCatalogError", err)
	}
	if len(tools) != 2 || incomplete.Collected != 2 || !strings.Contains(incomplete.Reason, "repeated") {
		t.Fatalf("known subset not preserved: tools=%d %+v", len(tools), incomplete)
	}
}

func TestLaterPageFailureKeepsEarlierPagesAsIncomplete(t *testing.T) {
	s := pagedServer(t, map[string]string{
		"tools/list|": toolPage("c1", "a", "b"),
		// c1 is not served: the server errors.
	})
	tools, err := connectHTTP(t, s).ListTools(context.Background())
	var incomplete *IncompleteCatalogError
	if !errors.As(err, &incomplete) || len(tools) != 2 || incomplete.Pages != 1 {
		t.Fatalf("tools=%d err=%v", len(tools), err)
	}
}

func TestFirstPageFailureIsAnOrdinaryError(t *testing.T) {
	s := pagedServer(t, map[string]string{})
	tools, err := connectHTTP(t, s).ListTools(context.Background())
	var incomplete *IncompleteCatalogError
	if err == nil || errors.As(err, &incomplete) || tools != nil {
		t.Fatalf("tools=%v err=%v; nothing collected is a failure, not a partial catalog", tools, err)
	}
}

func TestMalformedCatalogPagesAreRefusedNotEmptied(t *testing.T) {
	cases := map[string]string{
		"key missing":         `{"nextCursor":""}`,
		"key null":            `{"tools":null}`,
		"key not an array":    `{"tools":{"a":1}}`,
		"cursor wrong type":   `{"tools":[],"nextCursor":7}`,
		"result not object":   `[]`,
		"tool item malformed": `{"tools":[{"name":5}]}`,
	}
	for name, page := range cases {
		t.Run(name, func(t *testing.T) {
			s := pagedServer(t, map[string]string{"tools/list|": page})
			tools, err := connectHTTP(t, s).ListTools(context.Background())
			if err == nil {
				t.Fatalf("malformed page produced a catalog: %+v", tools)
			}
		})
	}
}

func TestCancelledCollectionIsIncomplete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
		var params struct {
			Cursor string `json:"cursor"`
		}
		_ = json.Unmarshal(req.Params, &params)
		if params.Cursor != "" {
			cancel() // the caller gives up while the second page is in flight
		}
		reply(w, req, toolPage("more", "a"))
	}))
	tools, err := connectHTTP(t, s).ListTools(ctx)
	var incomplete *IncompleteCatalogError
	if !errors.As(err, &incomplete) || len(tools) != 1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("tools=%d err=%v", len(tools), err)
	}
}

// --- manager-level coverage -------------------------------------------------

// A partial listing cannot show that an unlisted tool was withdrawn: the tool
// stays registered, the gap is recorded, and the control plane says so.
func TestPartialDiscoveryDoesNotWithdrawUnlistedTools(t *testing.T) {
	var mu sync.Mutex
	mode := "complete"
	s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
		var params struct {
			Cursor string `json:"cursor"`
		}
		_ = json.Unmarshal(req.Params, &params)
		mu.Lock()
		current := mode
		mu.Unlock()
		switch {
		case req.Method != "tools/list":
			reply(w, req, `{}`)
		case current == "complete":
			reply(w, req, toolPage("", "alpha", "beta"))
		case params.Cursor == "":
			reply(w, req, toolPage("again", "alpha"))
		default:
			reply(w, req, toolPage("again", "gamma")) // the cursor never advances
		}
	}))
	manager, plane, _ := newGenerationManager(t, s.URL)
	ctx := context.Background()
	if err := manager.Connect(ctx, "srv"); err != nil {
		t.Fatal(err)
	}
	if err := manager.DiscoverTools(ctx, "srv"); err != nil {
		t.Fatalf("complete discovery: %v", err)
	}
	if got := len(manager.GetAllTools()); got != 2 {
		t.Fatalf("tools after complete discovery = %d", got)
	}
	if gaps := manager.IncompleteCatalogs("srv"); len(gaps) != 0 {
		t.Fatalf("complete discovery recorded gaps: %v", gaps)
	}

	mu.Lock()
	mode = "looping"
	mu.Unlock()
	err := manager.DiscoverTools(ctx, "srv")
	var incomplete *IncompleteCatalogError
	if !errors.As(err, &incomplete) {
		t.Fatalf("DiscoverTools err = %v, want IncompleteCatalogError", err)
	}
	names := map[string]bool{}
	for _, tool := range manager.GetAllTools() {
		names[tool.Name] = true
	}
	if !names["alpha"] || !names["beta"] || !names["gamma"] {
		t.Fatalf("an unlisted tool was withdrawn by a partial listing (or a listed one lost): %v", names)
	}
	if gaps := manager.IncompleteCatalogs("srv"); len(gaps) != 1 || !strings.Contains(gaps[0], "tools") {
		t.Fatalf("incomplete catalog not recorded: %v", gaps)
	}
	atlas := plane.Atlas(ctx, AtlasOptions{})
	if len(atlas.Servers) != 1 || len(atlas.Servers[0].Incomplete) != 1 || !strings.Contains(atlas.Summary, "incomplete") {
		t.Fatalf("atlas hides the gap: %+v / %s", atlas.Servers, atlas.Summary)
	}

	mu.Lock()
	mode = "complete"
	mu.Unlock()
	if err := manager.DiscoverTools(ctx, "srv"); err != nil {
		t.Fatalf("recovery discovery: %v", err)
	}
	if gaps := manager.IncompleteCatalogs("srv"); len(gaps) != 0 {
		t.Fatalf("a complete rediscovery left a stale gap: %v", gaps)
	}
}

// A listed tool whose schema changed is still revoked by a partial listing;
// only unlisted tools are left alone.
func TestPartialDiscoveryStillRevokesChangedListedTools(t *testing.T) {
	var mu sync.Mutex
	changed := false
	s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) {
		var params struct {
			Cursor string `json:"cursor"`
		}
		_ = json.Unmarshal(req.Params, &params)
		mu.Lock()
		isChanged := changed
		mu.Unlock()
		switch {
		case req.Method != "tools/list":
			reply(w, req, `{}`)
		case !isChanged:
			reply(w, req, toolPage("", "alpha", "beta"))
		case params.Cursor == "":
			reply(w, req, `{"tools":[{"name":"alpha","description":"CHANGED","inputSchema":{"type":"object"}}],"nextCursor":"x"}`)
		default:
			reply(w, req, `{"tools":[],"nextCursor":"x"}`) // the cursor never advances
		}
	}))
	manager, _, _ := newGenerationManager(t, s.URL)
	ctx := context.Background()
	if err := manager.Connect(ctx, "srv"); err != nil {
		t.Fatal(err)
	}
	if err := manager.DiscoverTools(ctx, "srv"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	changed = true
	mu.Unlock()
	// The cursor never advances, so collection stops after the changed alpha.
	var incomplete *IncompleteCatalogError
	if err := manager.DiscoverTools(ctx, "srv"); !errors.As(err, &incomplete) {
		t.Fatalf("DiscoverTools err = %v, want IncompleteCatalogError", err)
	}
	for _, tool := range manager.GetAllTools() {
		if tool.Name == "alpha" && tool.Description != "CHANGED" {
			t.Fatalf("a listed tool kept its stale schema: %+v", tool)
		}
	}
}

// --- configured reviews survive their own server's connect ------------------

func TestConfiguredReviewsSurviveConnect(t *testing.T) {
	s := newStubServer(t, standardHandler(func(w http.ResponseWriter, req rawRPC) { reply(w, req, `{}`) }))
	if RemoteAuthorityKernelForTest == nil {
		t.Fatal("real Mangle kernel fixture is not installed")
	}
	review := ReviewedToolEffect{ToolID: "srv/write_note", SchemaHash: "abc123", Effect: RemoteWrite, TargetArgument: "path"}
	manager := NewMCPClientManager(nil, nil, map[string]MCPServerConfig{
		"srv": {ID: "srv", Enabled: true, Protocol: string(ProtocolHTTP), BaseURL: s.URL, Timeout: "5s", Reviews: []ReviewedToolEffect{review}},
	})
	manager.SetFactEmitter(NewFactEmitter(RemoteAuthorityKernelForTest(t)))
	if _, ok := manager.ReviewFor("srv/write_note"); !ok {
		t.Fatal("review not installed with the emitter")
	}
	// A review installed at runtime is bound to the connection it was made
	// against and does not outlive a reconnect.
	if err := manager.SetReviewedToolEffect(ReviewedToolEffect{ToolID: "srv/runtime_only", SchemaHash: "h", Effect: RemoteRead}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Connect(context.Background(), "srv"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	got, ok := manager.ReviewFor("srv/write_note")
	if !ok || got.SchemaHash != "abc123" || got.Effect != RemoteWrite {
		t.Fatalf("Connect cleared the operator's configured review: %+v ok=%v", got, ok)
	}
	if _, ok := manager.ReviewFor("srv/runtime_only"); ok {
		t.Fatal("a runtime review survived the reconnect it was not configured for")
	}
}

// --- connection refusal -----------------------------------------------------

func TestManagerConnectRefusesAMalformedHandshake(t *testing.T) {
	s := newStubServer(t, func(w http.ResponseWriter, req rawRPC) {
		if req.Method == "initialize" {
			reply(w, req, `{"protocolVersion":"2024-11-05","capabilities":{"tools":true}}`)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	manager, _, _ := newGenerationManager(t, s.URL)
	err := manager.Connect(context.Background(), "srv")
	if err == nil || !strings.Contains(err.Error(), "must be an object") {
		t.Fatalf("Connect error = %v", err)
	}
	if _, ok := manager.GetServer("srv"); ok {
		t.Fatal("a server with an unusable handshake was registered as connected")
	}
}
