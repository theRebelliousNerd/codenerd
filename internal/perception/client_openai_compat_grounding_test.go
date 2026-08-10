package perception

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

func TestGroundedWebSearch_ExactPayload(t *testing.T) {
	var gotBody metaGroundedRequest
	var gotPath string
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode grounded request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[]}]}],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}`))
	}))
	defer srv.Close()

	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	// Use default modelContributorModel; ensure model is forwarded.
	res, err := c.GroundedWebSearch(context.Background(), "what is codeNERD?")
	if err != nil {
		t.Fatalf("GroundedWebSearch: %v", err)
	}
	if res.Text != "hello" {
		t.Errorf("Text = %q, want hello", res.Text)
	}
	// Verify wire contract: POST c.baseURL + /responses
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/responses" {
		t.Errorf("path = %q, want /responses", gotPath)
	}
	// model
	if gotBody.Model != metaContributorModel {
		t.Errorf("model = %q, want %q", gotBody.Model, metaContributorModel)
	}
	// input one user item with input_text query
	if len(gotBody.Input) != 1 {
		t.Fatalf("input len = %d, want 1", len(gotBody.Input))
	}
	if gotBody.Input[0].Role != "user" {
		t.Errorf("input role = %q, want user", gotBody.Input[0].Role)
	}
	if len(gotBody.Input[0].Content) != 1 {
		t.Fatalf("input content len = %d, want 1", len(gotBody.Input[0].Content))
	}
	if gotBody.Input[0].Content[0].Type != "input_text" {
		t.Errorf("input content type = %q, want input_text", gotBody.Input[0].Content[0].Type)
	}
	if gotBody.Input[0].Content[0].Text != "what is codeNERD?" {
		t.Errorf("input text = %q", gotBody.Input[0].Content[0].Text)
	}
	// tools one web_search
	if len(gotBody.Tools) != 1 {
		t.Fatalf("tools len = %d, want 1", len(gotBody.Tools))
	}
	if gotBody.Tools[0].Type != "web_search" {
		t.Errorf("tool type = %q, want web_search", gotBody.Tools[0].Type)
	}
	// reasoning.effort default xhigh when no override nor hint
	if gotBody.Reasoning == nil {
		t.Fatal("reasoning is nil, want effort xhigh")
	}
	if gotBody.Reasoning.Effort != "xhigh" {
		t.Errorf("reasoning.effort = %q, want xhigh", gotBody.Reasoning.Effort)
	}
	// stream false
	if gotBody.Stream != false {
		t.Errorf("stream = %v, want false", gotBody.Stream)
	}
	encoded, _ := json.Marshal(gotBody)
	if !strings.Contains(string(encoded), `"stream":false`) {
		t.Errorf("payload must contain stream:false, got %s", encoded)
	}
}

func TestGroundedWebSearch_ReasoningEffortOverride(t *testing.T) {
	var gotEffort string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req metaGroundedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Reasoning != nil {
			gotEffort = req.Reasoning.Effort
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok","annotations":[]}]}]}`))
	}))
	defer srv.Close()

	cfg := DefaultOpenAICompatConfig(ProviderMeta, "test-key")
	cfg.BaseURL = srv.URL
	cfg.ReasoningEffort = "high"
	cfg.Timeout = 5 * time.Second
	c, err := NewOpenAICompatClient(cfg)
	if err != nil {
		t.Fatalf("NewOpenAICompatClient: %v", err)
	}
	// unhinted should use override
	_, err = c.GroundedWebSearch(context.Background(), "q1")
	if err != nil {
		t.Fatalf("GroundedWebSearch: %v", err)
	}
	if gotEffort != "high" {
		t.Errorf("reasoning.effort = %q, want high (override)", gotEffort)
	}
	// hinted should still use override
	ctx := context.WithValue(context.Background(), types.CtxKeyModelCapability, types.CapabilityHighSpeed)
	_, err = c.GroundedWebSearch(ctx, "q2")
	if err != nil {
		t.Fatalf("GroundedWebSearch: %v", err)
	}
	if gotEffort != "high" {
		t.Errorf("with capability hint, reasoning.effort = %q, want high (override wins)", gotEffort)
	}
}

func TestGroundedWebSearch_ReasoningEffortFromContext(t *testing.T) {
	var efforts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req metaGroundedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Reasoning != nil {
			efforts = append(efforts, req.Reasoning.Effort)
		} else {
			efforts = append(efforts, "")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok","annotations":[]}]}]}`))
	}))
	defer srv.Close()

	c := newTestCompatClient(t, ProviderMeta, srv.URL)

	// unhinted -> xhigh
	_, _ = c.GroundedWebSearch(context.Background(), "q")
	// high_reasoning -> high
	ctx := context.WithValue(context.Background(), types.CtxKeyModelCapability, types.CapabilityHighReasoning)
	_, _ = c.GroundedWebSearch(ctx, "q")
	// balanced -> medium
	ctx2 := context.WithValue(context.Background(), types.CtxKeyModelCapability, types.CapabilityBalanced)
	_, _ = c.GroundedWebSearch(ctx2, "q")
	// high_speed -> low
	ctx3 := context.WithValue(context.Background(), types.CtxKeyModelCapability, types.CapabilityHighSpeed)
	_, _ = c.GroundedWebSearch(ctx3, "q")

	want := []string{"xhigh", "high", "medium", "low"}
	if len(efforts) != len(want) {
		t.Fatalf("efforts len %d, want %d", len(efforts), len(want))
	}
	for i, w := range want {
		if efforts[i] != w {
			t.Errorf("effort[%d] = %q, want %q", i, efforts[i], w)
		}
	}
}

func TestGroundedWebSearch_CitationsAndUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output":[
				{"type":"message","role":"assistant","content":[
					{"type":"output_text","text":"hello world","annotations":[
						{"type":"url_citation","url":"https://example.com","title":"Example","start_index":0,"end_index":5},
						{"type":"other","url":"https://ignored.com","title":"Ignored","start_index":0,"end_index":1}
					]},
					{"type":"output_text","text":" more","annotations":[
						{"type":"url_citation","url":"https://example2.com","title":"","start_index":6,"end_index":10}
					]}
				]}
			],
			"usage":{"input_tokens":11,"output_tokens":22,"total_tokens":33}
		}`))
	}))
	defer srv.Close()

	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	res, err := c.GroundedWebSearch(context.Background(), "q")
	if err != nil {
		t.Fatalf("GroundedWebSearch: %v", err)
	}
	if res.Text != "hello world more" {
		t.Errorf("Text = %q, want 'hello world more'", res.Text)
	}
	if len(res.Citations) != 2 {
		t.Fatalf("citations len = %d, want 2 (only url_citation)", len(res.Citations))
	}
	if res.Citations[0].URL != "https://example.com" || res.Citations[0].Title != "Example" || res.Citations[0].StartIndex != 0 || res.Citations[0].EndIndex != 5 {
		t.Errorf("citation[0] = %+v", res.Citations[0])
	}
	if res.Citations[1].URL != "https://example2.com" || res.Citations[1].StartIndex != 6 || res.Citations[1].EndIndex != 10 {
		t.Errorf("citation[1] = %+v", res.Citations[1])
	}
	if res.Usage.InputTokens != 11 || res.Usage.OutputTokens != 22 || res.Usage.TotalTokens != 33 {
		t.Errorf("usage = %+v, want 11/22/33", res.Usage)
	}
}

func TestGroundedWebSearch_ReasoningSuppression(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output":[
				{"type":"reasoning","role":"assistant","content":[{"type":"output_text","text":"secret reasoning trace","annotations":[{"type":"url_citation","url":"https://evil.com","title":"bad","start_index":0,"end_index":1}]}]},
				{"type":"web_search_call","role":"assistant","content":[]},
				{"type":"message","role":"assistant","content":[{"type":"output_text","text":"visible answer","annotations":[{"type":"url_citation","url":"https://good.com","title":"Good","start_index":0,"end_index":7}]}]}
			],
			"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}
		}`))
	}))
	defer srv.Close()

	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	res, err := c.GroundedWebSearch(context.Background(), "q")
	if err != nil {
		t.Fatalf("GroundedWebSearch: %v", err)
	}
	if res.Text != "visible answer" {
		t.Errorf("Text = %q, want 'visible answer'", res.Text)
	}
	if strings.Contains(res.Text, "secret") {
		t.Error("reasoning trace leaked into Text")
	}
	if len(res.Citations) != 1 || res.Citations[0].URL != "https://good.com" {
		t.Errorf("citations = %+v, want only good.com", res.Citations)
	}
	for _, cit := range res.Citations {
		if cit.URL == "https://evil.com" {
			t.Error("citation from reasoning item must be suppressed")
		}
	}
}

func TestGroundedWebSearch_NonMetaFailClosed(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	for _, vendor := range []Provider{ProviderDashScope, ProviderMoonshot} {
		c := newTestCompatClient(t, vendor, srv.URL)
		_, err := c.GroundedWebSearch(context.Background(), "hello")
		if err == nil {
			t.Errorf("vendor %s: expected fail-closed error", vendor)
		}
		if !strings.Contains(err.Error(), "only supported for provider") {
			t.Errorf("vendor %s: error = %q, want provider-only message", vendor, err.Error())
		}
	}
	if hit {
		t.Error("non-Meta request hit HTTP server, must fail closed before HTTP")
	}
}

func TestGroundedWebSearch_BlankQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("blank query must not hit HTTP")
		w.WriteHeader(200)
	}))
	defer srv.Close()
	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	for _, q := range []string{"", "   ", "\n\t"} {
		_, err := c.GroundedWebSearch(context.Background(), q)
		if err == nil {
			t.Errorf("blank query %q: expected error", q)
		}
		if !strings.Contains(err.Error(), "must not be blank") {
			t.Errorf("blank query %q: err = %q", q, err.Error())
		}
	}
}

func TestGroundedWebSearch_ErrorsDoNotExposeReasoningOrKey(t *testing.T) {
	const secretReasoning = "super secret reasoning trace 12345"
	apiKey := "test-key"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		// Simulate a server that echoes API key and reasoning in message (must not be forwarded).
		_, _ = w.Write([]byte(`{"error":{"message":"` + secretReasoning + ` key=` + apiKey + `","code":"internal_error","type":"server_error"}}`))
	}))
	defer srv.Close()

	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	_, err := c.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, secretReasoning) {
		t.Errorf("error message exposed simulated reasoning: %q", msg)
	}
	if strings.Contains(msg, apiKey) {
		t.Errorf("error message exposed API key: %q", msg)
	}
	if !strings.Contains(msg, "status 500") {
		t.Errorf("error should contain status, got %q", msg)
	}
}

func TestGroundedWebSearch_Errors_StatusCodes(t *testing.T) {
	cases := []struct {
		status int
		body   string
	}{
		{400, `{"error":{"message":"bad request"}}`},
		{404, `{"error":{"message":"not found"}}`},
		{500, `internal error`},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		c := newTestCompatClient(t, ProviderMeta, srv.URL)
		_, err := c.GroundedWebSearch(context.Background(), "q")
		srv.Close()
		if err == nil {
			t.Errorf("status %d: expected error", tc.status)
		}
		if !strings.Contains(err.Error(), "status") {
			t.Errorf("status %d: err = %q", tc.status, err.Error())
		}
	}
}

func TestGroundedWebSearch_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[]}`))
	}))
	defer srv.Close()

	cfg := DefaultOpenAICompatConfig(ProviderMeta, "test-key")
	cfg.BaseURL = srv.URL
	cfg.Timeout = 50 * time.Millisecond
	c, err := NewOpenAICompatClient(cfg)
	if err != nil {
		t.Fatalf("NewOpenAICompatClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err = c.GroundedWebSearch(ctx, "q")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	// The error should wrap context deadline.
	if !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "context") {
		t.Logf("timeout err = %v", err)
	}
	// Ensure no reasoning trace in error.
	if strings.Contains(err.Error(), "reasoning") {
		t.Error("timeout error must not expose reasoning")
	}
}

func TestGroundedWebSearch_OversizedResponse(t *testing.T) {
	large := strings.Repeat("x", 11*1024*1024) // >10MiB
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(large))
	}))
	defer srv.Close()

	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	_, err := c.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Fatal("expected oversized response error")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("err = %q, want too large", err.Error())
	}
}

func TestGroundedWebSearch_OversizedQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("oversized query must not hit HTTP")
		w.WriteHeader(200)
	}))
	defer srv.Close()
	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	oversized := strings.Repeat("a", 10001)
	_, err := c.GroundedWebSearch(context.Background(), oversized)
	if err == nil {
		t.Fatal("expected oversized query error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("err = %q, want exceeds", err.Error())
	}
}

func TestGroundedWebSearch_ImplementsInterface(t *testing.T) {
	var c any = &OpenAICompatClient{}
	if _, ok := c.(types.GroundedWebSearcher); !ok {
		t.Error("OpenAICompatClient does not implement GroundedWebSearcher")
	}
	// Also via types.
	var _ types.GroundedWebSearcher = (*OpenAICompatClient)(nil)
}

func TestGroundedWebSearch_NilClient(t *testing.T) {
	var c *OpenAICompatClient
	_, err := c.GroundedWebSearch(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected nil client error")
	}
	if !strings.Contains(err.Error(), "client is nil") {
		t.Errorf("err = %q", err.Error())
	}
}

func TestGroundedWebSearch_UsageFallback(t *testing.T) {
	// Vendor may use prompt_tokens/completion_tokens naming.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ans","annotations":[]}]}],
			"usage":{"prompt_tokens":7,"completion_tokens":8,"total_tokens":15}
		}`))
	}))
	defer srv.Close()
	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	res, err := c.GroundedWebSearch(context.Background(), "q")
	if err != nil {
		t.Fatalf("GroundedWebSearch: %v", err)
	}
	if res.Usage.InputTokens != 7 || res.Usage.OutputTokens != 8 || res.Usage.TotalTokens != 15 {
		t.Errorf("usage fallback = %+v", res.Usage)
	}
}

func TestGroundedWebSearch_ModelFromContext(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req metaGroundedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotModel = req.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok","annotations":[]}]}]}`))
	}))
	defer srv.Close()

	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	ctx := context.WithValue(context.Background(), types.CtxKeyModelName, "muse-spark-1.2")
	_, err := c.GroundedWebSearch(ctx, "q")
	if err != nil {
		t.Fatalf("GroundedWebSearch: %v", err)
	}
	if gotModel != metaContributorModel {
		t.Errorf("model = %q, want normalized %q", gotModel, metaContributorModel)
	}
}

func TestGroundedWebSearch_Non200DoesNotLeakRawBody(t *testing.T) {
	const secretReasoning = "super secret reasoning trace 12345"
	const apiKey = "test-key"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(secretReasoning + " " + apiKey + " raw body that must not appear"))
	}))
	defer srv.Close()
	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	_, err := c.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, secretReasoning) {
		t.Errorf("non-200 error exposed raw reasoning: %q", msg)
	}
	if strings.Contains(msg, apiKey) {
		t.Errorf("non-200 error exposed API key: %q", msg)
	}
	if strings.Contains(msg, "raw body") {
		t.Errorf("non-200 error exposed raw body: %q", msg)
	}
	if !strings.Contains(msg, "status 400") {
		t.Errorf("error must contain status, got %q", msg)
	}
}

func TestGroundedWebSearch_Non200StructuredCodeType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","type":"throttle","message":"ignored secret message"}}`))
	}))
	defer srv.Close()
	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	_, err := c.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "code=rate_limited") {
		t.Errorf("expected code, got %q", msg)
	}
	if !strings.Contains(msg, "type=throttle") {
		t.Errorf("expected type, got %q", msg)
	}
	if strings.Contains(msg, "ignored secret") {
		t.Errorf("error must not expose message body, got %q", msg)
	}
}

func TestGroundedWebSearch_Non200CodeBounded(t *testing.T) {
	longCode := strings.Repeat("c", 500)
	longType := strings.Repeat("t", 500)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": longCode, "type": longType, "message": "secret"}})
	}))
	defer srv.Close()
	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	_, err := c.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, longCode) {
		t.Errorf("code not bounded, len msg %d", len(msg))
	}
	if strings.Contains(msg, longType) {
		t.Errorf("type not bounded, len msg %d", len(msg))
	}
	if !strings.Contains(msg, "status 500") {
		t.Errorf("must contain status, got %q", msg)
	}
}

func TestGroundedWebSearch_NilHttpClientGuarded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok","annotations":[]}]}]}`))
	}))
	defer srv.Close()
	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	c.httpClient = nil
	ctx := context.Background()
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		t.Fatal("test context should have no deadline")
	}
	res, err := c.GroundedWebSearch(ctx, "hello")
	if err != nil {
		t.Fatalf("nil httpClient GroundedWebSearch: %v", err)
	}
	if res.Text != "ok" {
		t.Errorf("Text = %q, want ok", res.Text)
	}
}

func TestGroundedWebSearch_200ErrorObjectWithoutMessageFails(t *testing.T) {
	cases := []string{
		`{"output":[],"error":{"code":"upstream_failed","type":"server_error"}}`,
		`{"output":[],"error":{"code":"bad","type":"invalid"}}`,
		`{"output":[],"error":{"code":"only_code"}}`,
		`{"output":[],"error":{"type":"only_type"}}`,
		`{"output":[],"error":{}}`,
	}
	for i, body := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		c := newTestCompatClient(t, ProviderMeta, srv.URL)
		_, err := c.GroundedWebSearch(context.Background(), "q")
		srv.Close()
		if err == nil {
			t.Errorf("case %d body %s: expected error for 200 with error object", i, body)
		} else if !strings.Contains(err.Error(), "api error") {
			t.Errorf("case %d: err = %q, want api error", i, err.Error())
		}
		if err != nil && strings.Contains(err.Error(), "secret") {
			t.Errorf("case %d leaked message", i)
		}
	}
}

func TestGroundedWebSearch_200NoOutputTextFails(t *testing.T) {
	bodies := []string{
		`{"output":[]}`,
		`{"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"   ","annotations":[]}]}]}`,
		`{"output":[{"type":"reasoning","content":[{"type":"output_text","text":"secret","annotations":[]}]}]}`,
		`{"output":[{"type":"web_search_call","role":"assistant"}]}`,
		`{"output":[{"type":"message","role":"user","content":[{"type":"output_text","text":"wrong role","annotations":[]}]}]}`,
		`{"output":[{"type":"message","role":"assistant","content":[{"type":"other","text":"not output_text"}]}]}`,
	}
	for i, body := range bodies {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		c := newTestCompatClient(t, ProviderMeta, srv.URL)
		_, err := c.GroundedWebSearch(context.Background(), "q")
		srv.Close()
		if err == nil {
			t.Errorf("case %d body %s: expected error for missing output_text", i, body)
		} else if !strings.Contains(err.Error(), "no output_text") {
			t.Errorf("case %d: err = %q, want no output_text", i, err.Error())
		}
	}
}

func TestGroundedWebSearch_CitationsOnlyHttpHttps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"answer","annotations":[
				{"type":"url_citation","url":"https://good.com/a","title":"good","start_index":0,"end_index":1},
				{"type":"url_citation","url":"http://good2.com/b","title":"","start_index":1,"end_index":2},
				{"type":"url_citation","url":"ftp://bad.com/c","title":"ftp","start_index":2,"end_index":3},
				{"type":"url_citation","url":"javascript:alert(1)","title":"xss","start_index":3,"end_index":4},
				{"type":"url_citation","url":"data:text/plain,hello","title":"data","start_index":4,"end_index":5},
				{"type":"url_citation","url":"//no-scheme.com","title":"noscheme","start_index":5,"end_index":6},
				{"type":"url_citation","url":"","title":"empty","start_index":6,"end_index":7},
				{"type":"url_citation","url":"   ","title":"blank","start_index":7,"end_index":8},
				{"type":"url_citation","url":"https://","title":"no host","start_index":8,"end_index":9}
			]}]}]
		}`))
	}))
	defer srv.Close()
	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	res, err := c.GroundedWebSearch(context.Background(), "q")
	if err != nil {
		t.Fatalf("GroundedWebSearch: %v", err)
	}
	if len(res.Citations) != 2 {
		t.Fatalf("citations len = %d, want 2 (only http/https)", len(res.Citations))
	}
	if res.Citations[0].URL != "https://good.com/a" {
		t.Errorf("citation[0] = %+v", res.Citations[0])
	}
	if res.Citations[1].URL != "http://good2.com/b" {
		t.Errorf("citation[1] = %+v", res.Citations[1])
	}
}

func TestGroundedWebSearch_MaliciousCodeTypeContainingApiKeyIsDropped(t *testing.T) {
	const apiKey = "test-key"
	cases := []struct {
		name string
		body string
	}{
		{"non200_code_has_key", `{"error":{"code":"oops-` + apiKey + `-leaked","type":"server_error","message":"secret"}}`},
		{"non200_type_has_key", `{"error":{"code":"bad","type":"type-` + apiKey + `","message":"secret"}}`},
		{"non200_flat_code_has_key", `{"code":"flat-` + apiKey + `","type":"throttle"}`},
		{"non200_both_have_key", `{"error":{"code":"` + apiKey + `","type":"` + apiKey + `","message":"secret"}}`},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(tc.body))
		}))
		c := newTestCompatClient(t, ProviderMeta, srv.URL)
		_, err := c.GroundedWebSearch(context.Background(), "q")
		srv.Close()
		if err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
		msg := err.Error()
		if strings.Contains(msg, apiKey) {
			t.Errorf("%s: error leaked api key via code/type: %q", tc.name, msg)
		}
		if !strings.Contains(msg, "status 500") {
			t.Errorf("%s: missing status: %q", tc.name, msg)
		}
	}
	// Also cover 200 error-object path.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[],"error":{"code":"x-` + apiKey + `","type":"y-` + apiKey + `"}}`))
	}))
	defer srv.Close()
	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	_, err := c.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Fatal("200 malicious error object: expected error")
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Errorf("200 path leaked api key: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "api error") {
		t.Errorf("200 path missing api error: %q", err.Error())
	}
}

func TestGroundedWebSearch_MaliciousCodeTypeWithIllegalCharsIsDropped(t *testing.T) {
	cases := []struct {
		body      string
		forbidden string // raw malicious code that must not appear in error
	}{
		{`{"error":{"code":"bad code with spaces","type":"server_error"}}`, "bad code with spaces"},
		{`{"error":{"code":"code/with/slash","type":"t"}}`, "code/with/slash"},
		{`{"error":{"code":"code:test-key:extra","type":"ok"}}`, "code:test-key:extra"},
		{`{"error":{"code":"oops; rm -rf /","type":"ok"}}`, "oops; rm -rf /"},
	}
	for i, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(tc.body))
		}))
		c := newTestCompatClient(t, ProviderMeta, srv.URL)
		_, err := c.GroundedWebSearch(context.Background(), "q")
		srv.Close()
		if err == nil {
			t.Fatalf("case %d: expected error", i)
		}
		msg := err.Error()
		if strings.Contains(msg, tc.forbidden) {
			t.Errorf("case %d: sanitized code %q must be dropped, got %q", i, tc.forbidden, msg)
		}
		if strings.Contains(msg, "test-key") {
			t.Errorf("case %d: key leaked via illegal code: %q", i, msg)
		}
	}
}

func TestGroundedWebSearch_TransportErrorDoesNotLeakApiKeyAndPreservesContext(t *testing.T) {
	const apiKey = "test-key"
	c := newTestCompatClient(t, ProviderMeta, "http://example.invalid")

	// Custom RoundTripper whose error string contains the API key.
	c.httpClient.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("boom transport failure key=" + apiKey + " secret-reasoning-trace")
	})

	_, err := c.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Fatal("expected transport error")
	}
	msg := err.Error()
	if strings.Contains(msg, apiKey) {
		t.Errorf("transport error leaked api key: %q", msg)
	}
	if strings.Contains(msg, "secret-reasoning") {
		t.Errorf("transport error leaked provider text: %q", msg)
	}
	if !strings.Contains(msg, "transport error") {
		t.Errorf("transport error should be generic, got %q", msg)
	}
	// Must not preserve wrapped provider text as cause that could be inspected for key leakage.
	if strings.Contains(err.Error(), "boom") {
		t.Errorf("transport error must not wrap arbitrary provider text, got %q", err.Error())
	}

	// Context cancellation identity must still be preserved.
	c2 := newTestCompatClient(t, ProviderMeta, "http://example.invalid")
	c2.httpClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		// Simulate a transport that wraps context.Canceled.
		return nil, context.Canceled
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c2.GroundedWebSearch(ctx, "q")
	if err == nil {
		t.Fatal("expected context canceled error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("canceled transport should preserve context.Canceled identity, got %v", err)
	}

	// Deadline identity preserved via Do returning DeadlineExceeded directly.
	c3 := newTestCompatClient(t, ProviderMeta, "http://example.invalid")
	c3.httpClient.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})
	_, err = c3.GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Fatal("expected deadline error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("deadline transport should preserve DeadlineExceeded identity, got %v", err)
	}
}

// roundTripperFunc is a helper for transport-error tests.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// trackingReadCloser wraps an io.ReadCloser and records Close calls for lifecycle assertions.
type trackingReadCloser struct {
	io.ReadCloser
	closed *bool
	closes *int
}

func (t *trackingReadCloser) Close() error {
	*t.closed = true
	*t.closes++
	return t.ReadCloser.Close()
}

func TestGroundedWebSearch_ResponseBodyClosedOnSuccess(t *testing.T) {
	closed := false
	closes := 0
	body := `{"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[]}]}]}`
	c := newTestCompatClient(t, ProviderMeta, "http://example.invalid")
	c.httpClient.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		rc := io.NopCloser(strings.NewReader(body))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       &trackingReadCloser{ReadCloser: rc, closed: &closed, closes: &closes},
			Header:     make(http.Header),
		}, nil
	})
	res, err := c.GroundedWebSearch(context.Background(), "hello")
	if err != nil {
		t.Fatalf("GroundedWebSearch success close tracking: %v", err)
	}
	if res.Text != "hello" {
		t.Errorf("Text = %q, want hello", res.Text)
	}
	if !closed {
		t.Error("response Body was not closed on success path")
	}
	if closes != 1 {
		t.Errorf("Close calls = %d, want 1 on success path", closes)
	}
}

func TestGroundedWebSearch_ResponseBodyClosedOnErrorPaths(t *testing.T) {
	large := strings.Repeat("x", 11*1024*1024)
	cases := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "non-200", statusCode: http.StatusInternalServerError, body: `{"error":{"code":"internal_error","type":"server_error"}}`},
		{name: "malformed json", statusCode: http.StatusOK, body: `not json {`},
		{name: "oversized", statusCode: http.StatusOK, body: large},
		{name: "hollow output", statusCode: http.StatusOK, body: `{"output":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			closed := false
			closes := 0
			body := tc.body
			c := newTestCompatClient(t, ProviderMeta, "http://example.invalid")
			c.httpClient.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
				rc := io.NopCloser(strings.NewReader(body))
				return &http.Response{
					StatusCode: tc.statusCode,
					Body:       &trackingReadCloser{ReadCloser: rc, closed: &closed, closes: &closes},
					Header:     make(http.Header),
				}, nil
			})
			_, err := c.GroundedWebSearch(context.Background(), "q")
			if err == nil {
				t.Fatalf("%s: expected error", tc.name)
			}
			if !closed {
				t.Errorf("%s: response Body was not closed on error path", tc.name)
			}
			if closes != 1 {
				t.Errorf("%s: Close calls = %d, want 1", tc.name, closes)
			}
		})
	}
}
