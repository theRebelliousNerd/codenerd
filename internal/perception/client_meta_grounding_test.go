package perception

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/types"
)

const metaGroundingFixture = `{
  "id": "resp_test",
  "status": "completed",
  "model": "muse-spark-1.3-contributor",
  "output": [
    {"type": "reasoning", "id": "rs_1", "encrypted_content": "ciphertext-not-for-the-caller", "summary": [],
      "content": [{"type": "output_text", "text": "secret reasoning", "annotations": [
        {"type": "url_citation", "url": "https://evil.example/hidden", "title": "hidden", "start_index": 0, "end_index": 1}
      ]}]},
    {"type": "web_search_call", "id": "ws_1", "status": "completed", "results": [
      {"type": "text_result", "title": "codeNERD", "url": "https://example.com/codenerd", "snippet": "logic-first agent"},
      {"type": "text_result", "title": "other", "url": "https://example.com/other", "snippet": "another page"},
      {"type": "text_result", "title": "bad", "url": "javascript:alert(1)", "snippet": "dropped"}
    ]},
    {"type": "message", "role": "assistant", "status": "completed", "content": [
      {"type": "output_text", "text": "codeNERD separates creativity from control.", "annotations": [
        {"type": "url_citation", "url": "https://example.com/codenerd", "title": "codeNERD", "start_index": 0, "end_index": 8},
        {"type": "url_citation", "url": "not a url", "title": "nope", "start_index": 0, "end_index": 1}
      ]}
    ]}
  ],
  "usage": {"input_tokens": 12, "output_tokens": 34, "total_tokens": 46}
}`

const metaResponsesOK = `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"grounded answer"}]}],"usage":{"input_tokens":2,"output_tokens":3,"total_tokens":5}}`

const metaChatOK = `{"choices":[{"message":{"role":"assistant","content":"plain answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

func metaCaptureServer(t *testing.T, reply string) (*httptest.Server, *[]string, *[][]byte) {
	t.Helper()
	var paths []string
	var bodies [][]byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		paths = append(paths, r.URL.Path)
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/responses") {
			_, _ = w.Write([]byte(reply))
			return
		}
		_, _ = w.Write([]byte(metaChatOK))
	}))
	return srv, &paths, &bodies
}

func decodeMetaBody(t *testing.T, raw []byte) (metaResponsesRequest, map[string]any) {
	t.Helper()
	var req metaResponsesRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("decode request: %v\n%s", err, raw)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	return req, doc
}

func assertWebSearchOn(t *testing.T, raw []byte, size string) {
	t.Helper()
	req, doc := decodeMetaBody(t, raw)
	var saw bool
	for _, tool := range req.Tools {
		if tool.Type != "web_search" {
			continue
		}
		saw = true
		if tool.SearchContextSize != size {
			t.Errorf("search_context_size = %q, want %q", tool.SearchContextSize, size)
		}
	}
	if !saw {
		t.Fatalf("tools = %+v, want a web_search tool", req.Tools)
	}
	include, _ := doc["include"].([]any)
	if !jsonStringListHas(include, "web_search_call.results") {
		t.Errorf("include = %v, want web_search_call.results", include)
	}
	if _, ok := doc["response_format"]; ok {
		t.Errorf("responses body carries response_format, which this surface does not use: %s", raw)
	}
}

func TestMetaGrounding_EnabledUsesResponsesAndDisabledDoesNot(t *testing.T) {
	srv, paths, bodies := metaCaptureServer(t, metaResponsesOK)
	defer srv.Close()

	on := newTestCompatClient(t, ProviderMeta, srv.URL)
	on.SetEnableWebSearch(true)
	on.searchContextSize = "medium"
	got, err := on.Complete(context.Background(), "what is codeNERD?")
	if err != nil {
		t.Fatalf("enabled Complete: %v", err)
	}
	if got != "grounded answer" {
		t.Errorf("text = %q", got)
	}
	if len(*paths) != 1 || (*paths)[0] != "/responses" {
		t.Fatalf("paths = %v, want one /responses", *paths)
	}
	assertWebSearchOn(t, (*bodies)[0], "medium")

	*paths = nil
	*bodies = nil
	off := newTestCompatClient(t, ProviderMeta, srv.URL)
	got, err = off.Complete(context.Background(), "what is codeNERD?")
	if err != nil {
		t.Fatalf("disabled Complete: %v", err)
	}
	if got != "plain answer" {
		t.Errorf("disabled text = %q", got)
	}
	if len(*paths) != 1 || (*paths)[0] != "/chat/completions" {
		t.Fatalf("disabled paths = %v, want /chat/completions", *paths)
	}
	if strings.Contains(string((*bodies)[0]), "web_search") {
		t.Errorf("disabled chat body carried web_search: %s", (*bodies)[0])
	}
}

func TestMetaGrounding_FirstToolTurnAndFollowUp(t *testing.T) {
	srv, paths, bodies := metaCaptureServer(t, metaResponsesOK)
	defer srv.Close()
	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	c.SetEnableWebSearch(true)
	c.searchContextSize = "high"
	tools := []ToolDefinition{{
		Name:        "lookup",
		Description: "look something up",
		InputSchema: map[string]any{"type": "object"},
	}}
	if _, err := c.CompleteWithTools(context.Background(), "system", "question", tools); err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}
	if (*paths)[0] != "/responses" {
		t.Fatalf("first tool turn path = %q, want /responses", (*paths)[0])
	}
	assertWebSearchOn(t, (*bodies)[0], "high")
	req, _ := decodeMetaBody(t, (*bodies)[0])
	var sawFunction bool
	for _, tool := range req.Tools {
		if tool.Type == "function" && tool.Name == "lookup" {
			sawFunction = true
		}
	}
	if !sawFunction {
		t.Fatalf("function tool missing beside web_search: %+v", req.Tools)
	}

	*paths = nil
	*bodies = nil
	off := newTestCompatClient(t, ProviderMeta, srv.URL)
	if _, err := off.CompleteWithTools(context.Background(), "system", "question", tools); err != nil {
		t.Fatalf("disabled CompleteWithTools: %v", err)
	}
	if (*paths)[0] != "/chat/completions" {
		t.Fatalf("disabled first tool turn path = %q, want /chat/completions", (*paths)[0])
	}

	history := []types.Message{{Role: "user", Text: "question"}}
	*paths = nil
	*bodies = nil
	if _, err := c.CompleteWithToolResults(context.Background(), "system", history, tools); err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	if (*paths)[0] != "/responses" {
		t.Fatalf("follow-up path = %q", (*paths)[0])
	}
	assertWebSearchOn(t, (*bodies)[0], "high")

	*paths = nil
	*bodies = nil
	if _, err := off.CompleteWithToolResults(context.Background(), "system", history, tools); err != nil {
		t.Fatalf("disabled follow-up: %v", err)
	}
	if (*paths)[0] != "/responses" {
		t.Fatalf("search-off follow-up left /responses: %v", *paths)
	}
	req, doc := decodeMetaBody(t, (*bodies)[0])
	for _, tool := range req.Tools {
		if tool.Type == "web_search" {
			t.Fatal("search-off tool follow-up attached web_search")
		}
	}
	include, _ := doc["include"].([]any)
	if jsonStringListHas(include, "web_search_call.results") {
		t.Errorf("search-off follow-up included web_search_call.results: %v", include)
	}
}

func assertNoLeakedTrace(t *testing.T, text string) {
	t.Helper()
	for _, bad := range []string{"ciphertext", "secret", "evil.example"} {
		if strings.Contains(text, bad) {
			t.Errorf("answer contains %q: %s", bad, text)
		}
	}
}

func assertFixtureSources(t *testing.T, sources []string) {
	t.Helper()
	want := []string{"https://example.com/codenerd", "https://example.com/other"}
	if len(sources) != len(want) {
		t.Fatalf("sources = %v, want %v", sources, want)
	}
	for i := range want {
		if sources[i] != want[i] {
			t.Fatalf("sources = %v, want %v", sources, want)
		}
	}
	for _, u := range sources {
		if strings.Contains(u, "evil.example") || strings.HasPrefix(u, "javascript:") {
			t.Errorf("source %q is not an http(s) citation or result", u)
		}
	}
}

func TestMetaGrounding_ParsesCitationsAndSearchResults(t *testing.T) {
	var reply metaResponsesReply
	if err := json.Unmarshal([]byte(metaGroundingFixture), &reply); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	text := strings.TrimSpace(metaAssistantText(&reply))
	assertNoLeakedTrace(t, text)
	if text != "codeNERD separates creativity from control." {
		t.Fatalf("text = %q", text)
	}
	got := metaGroundingFromReply(&reply)
	if len(got.Citations) != 1 || got.Citations[0].URL != "https://example.com/codenerd" || got.Citations[0].Title != "codeNERD" {
		t.Fatalf("citations = %+v", got.Citations)
	}
	if got.Citations[0].StartIndex != 0 || got.Citations[0].EndIndex != 8 {
		t.Fatalf("citation span = %d:%d", got.Citations[0].StartIndex, got.Citations[0].EndIndex)
	}
	if len(got.Results) != 2 {
		t.Fatalf("results = %+v", got.Results)
	}
	if got.Results[0].URL != "https://example.com/codenerd" || got.Results[0].Snippet != "logic-first agent" || got.Results[0].Type != "text_result" {
		t.Fatalf("first result = %+v", got.Results[0])
	}
	if got.Results[1].URL != "https://example.com/other" || got.Results[1].Title != "other" {
		t.Fatalf("second result = %+v", got.Results[1])
	}
	assertFixtureSources(t, got.Sources)

	srv, paths, _ := metaCaptureServer(t, metaGroundingFixture)
	defer srv.Close()

	searcher := newTestCompatClient(t, ProviderMeta, srv.URL)
	searcher.searchContextSize = "medium"
	res, err := searcher.GroundedWebSearch(context.Background(), "what is codeNERD?")
	if err != nil {
		t.Fatalf("GroundedWebSearch: %v", err)
	}
	assertNoLeakedTrace(t, res.Text)
	if res.Text != text {
		t.Fatalf("GroundedWebSearch text = %q", res.Text)
	}
	if len(res.Citations) != 1 || res.Citations[0].URL != got.Citations[0].URL {
		t.Fatalf("GroundedWebSearch citations = %+v", res.Citations)
	}
	if len(res.Results) != 2 || res.Results[0].URL != got.Results[0].URL || res.Results[1].URL != got.Results[1].URL {
		t.Fatalf("GroundedWebSearch results = %+v", res.Results)
	}
	if res.Usage.InputTokens != 12 || res.Usage.OutputTokens != 34 || res.Usage.TotalTokens != 46 {
		t.Fatalf("usage = %+v", res.Usage)
	}
	assertFixtureSources(t, searcher.GetLastGroundingSources())
	copied := searcher.GetLastGroundingSources()
	copied[0] = "mutated"
	if again := searcher.GetLastGroundingSources(); again[0] == "mutated" {
		t.Fatal("GetLastGroundingSources returned its internal slice")
	}
	if (*paths)[0] != "/responses" {
		t.Fatalf("GroundedWebSearch path = %v", *paths)
	}

	*paths = nil
	on := newTestCompatClient(t, ProviderMeta, srv.URL)
	on.SetEnableWebSearch(true)
	completed, err := on.Complete(context.Background(), "what is codeNERD?")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	assertNoLeakedTrace(t, completed)
	assertFixtureSources(t, on.GetLastGroundingSources())

	*paths = nil
	followed, err := on.CompleteWithToolResults(context.Background(), "system", []types.Message{{Role: "user", Text: "what is codeNERD?"}}, nil)
	if err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	assertNoLeakedTrace(t, followed.Text)
	assertFixtureSources(t, followed.GroundingSources)
	assertFixtureSources(t, on.GetLastGroundingSources())
	if (*paths)[len(*paths)-1] != "/responses" {
		t.Fatalf("paths = %v", *paths)
	}
}

func TestMetaGrounding_ReservedToolNames(t *testing.T) {
	srv, paths, bodies := metaCaptureServer(t, metaResponsesOK)
	defer srv.Close()
	tools := []ToolDefinition{{
		Name:        "browser.search",
		Description: "reserved by web search",
		InputSchema: map[string]any{"type": "object"},
	}}

	on := newTestCompatClient(t, ProviderMeta, srv.URL)
	on.SetEnableWebSearch(true)
	if _, err := on.CompleteWithTools(context.Background(), "system", "question", tools); err == nil {
		t.Fatal("browser.search was accepted beside web_search")
	} else if !strings.Contains(err.Error(), "browser.search") {
		t.Fatalf("reserved-name error = %v", err)
	}
	if len(*paths) != 0 {
		t.Fatalf("reserved name still hit the server: %v", *paths)
	}

	off := newTestCompatClient(t, ProviderMeta, srv.URL)
	if _, err := off.CompleteWithTools(context.Background(), "system", "question", tools); err != nil {
		t.Fatalf("search-off browser.search: %v", err)
	}
	if len(*paths) != 1 || (*paths)[0] != "/chat/completions" {
		t.Fatalf("search-off paths = %v", *paths)
	}
	if strings.Contains(string((*bodies)[0]), `"type":"web_search"`) || strings.Contains(string((*bodies)[0]), `"type": "web_search"`) {
		t.Errorf("search-off body carried a web_search tool: %s", (*bodies)[0])
	}
}

func TestMetaGrounding_PiggybackUsesJSONObjectOnResponses(t *testing.T) {
	srv, paths, bodies := metaCaptureServer(t, metaResponsesOK)
	defer srv.Close()
	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	c.SetEnableWebSearch(true)
	c.searchContextSize = "low"
	if _, err := c.CompleteWithSystem(context.Background(), "return a control_packet envelope", "hi"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if len(*paths) != 1 || (*paths)[0] != "/responses" {
		t.Fatalf("paths = %v", *paths)
	}
	assertWebSearchOn(t, (*bodies)[0], "low")
	req, _ := decodeMetaBody(t, (*bodies)[0])
	if req.Text == nil || req.Text.Format == nil || req.Text.Format.Type != "json_object" {
		t.Fatalf("text format = %+v, want json_object", req.Text)
	}
}

func TestMetaGrounding_EmptyRetryStaysOnResponses(t *testing.T) {
	var n int
	var paths []string
	var bodies [][]byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		paths = append(paths, r.URL.Path)
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		n++
		if strings.HasSuffix(r.URL.Path, "/chat/completions") {
			_, _ = w.Write([]byte(metaChatOK))
			return
		}
		if n == 1 {
			_, _ = w.Write([]byte(`{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":""}]}],"usage":{"input_tokens":1,"output_tokens":10,"total_tokens":11}}`))
			return
		}
		_, _ = w.Write([]byte(metaResponsesOK))
	}))
	defer srv.Close()

	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	c.SetEnableWebSearch(true)
	got, err := c.Complete(context.Background(), "question")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got != "grounded answer" {
		t.Fatalf("text = %q", got)
	}
	if len(paths) != 2 {
		t.Fatalf("paths = %v, want two /responses calls", paths)
	}
	for _, p := range paths {
		if p != "/responses" {
			t.Fatalf("empty retry left /responses: %v", paths)
		}
	}
	req, _ := decodeMetaBody(t, bodies[1])
	if req.Reasoning == nil || req.Reasoning.Effort != "minimal" {
		t.Fatalf("retry reasoning = %+v, want effort minimal", req.Reasoning)
	}
}

func TestMetaGrounding_StreamingIsOneResponsesDelta(t *testing.T) {
	srv, paths, bodies := metaCaptureServer(t, metaResponsesOK)
	defer srv.Close()
	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	c.SetEnableWebSearch(true)
	c.searchContextSize = "medium"

	content, errs := c.CompleteWithStreaming(context.Background(), "", "question", false)
	var got string
	for delta := range content {
		got += delta
	}
	for err := range errs {
		t.Fatalf("stream: %v", err)
	}
	if got != "grounded answer" {
		t.Fatalf("deltas = %q", got)
	}
	if len(*paths) != 1 || (*paths)[0] != "/responses" {
		t.Fatalf("paths = %v", *paths)
	}
	assertWebSearchOn(t, (*bodies)[0], "medium")
	if strings.Contains(string((*bodies)[0]), `"stream":true`) {
		t.Errorf("grounded stream sent stream:true: %s", (*bodies)[0])
	}
}

func TestMetaGrounding_DashScopeIgnoresTheSwitch(t *testing.T) {
	srv, paths, bodies := metaCaptureServer(t, metaResponsesOK)
	defer srv.Close()
	c := newTestCompatClient(t, ProviderDashScope, srv.URL)
	c.SetEnableWebSearch(true)
	c.searchContextSize = "high"
	if c.SupportsGrounding() {
		t.Fatal("DashScope reported grounding")
	}
	if c.IsWebSearchEnabled() {
		t.Fatal("SetEnableWebSearch enabled search on DashScope")
	}
	got, err := c.Complete(context.Background(), "question")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got != "plain answer" {
		t.Fatalf("text = %q", got)
	}
	if len(*paths) != 1 || (*paths)[0] != "/chat/completions" {
		t.Fatalf("paths = %v", *paths)
	}
	if strings.Contains(string((*bodies)[0]), "web_search") {
		t.Errorf("DashScope body carried web_search: %s", (*bodies)[0])
	}
}

func TestMetaGrounding_FactoryAppliesConfig(t *testing.T) {
	nilMeta, err := newCompatClient(&ProviderConfig{
		Provider: ProviderMeta,
		APIKey:   "k",
		Model:    metaContributorModel,
	})
	if err != nil {
		t.Fatalf("nil Meta: %v", err)
	}
	nilClient := nilMeta.(*OpenAICompatClient)
	if !nilClient.IsWebSearchEnabled() || nilClient.searchContextSize != "medium" {
		t.Fatalf("nil Meta resolved enabled=%v size=%q, want true/medium", nilClient.IsWebSearchEnabled(), nilClient.searchContextSize)
	}
	if !nilClient.SupportsGrounding() {
		t.Fatal("Meta client reported no grounding capability")
	}

	off := false
	explicit, err := newCompatClient(&ProviderConfig{
		Provider: ProviderMeta,
		APIKey:   "k",
		Model:    metaContributorModel,
		Meta:     &config.MetaProviderConfig{EnableWebSearch: &off, SearchContextSize: "low"},
	})
	if err != nil {
		t.Fatalf("explicit false: %v", err)
	}
	explicitClient := explicit.(*OpenAICompatClient)
	if explicitClient.IsWebSearchEnabled() || explicitClient.searchContextSize != "low" {
		t.Fatalf("explicit Meta resolved enabled=%v size=%q, want false/low", explicitClient.IsWebSearchEnabled(), explicitClient.searchContextSize)
	}

	empty, err := newCompatClient(&ProviderConfig{
		Provider: ProviderMeta,
		APIKey:   "k",
		Model:    metaContributorModel,
		Meta:     &config.MetaProviderConfig{},
	})
	if err != nil {
		t.Fatalf("empty Meta: %v", err)
	}
	emptyClient := empty.(*OpenAICompatClient)
	if !emptyClient.IsWebSearchEnabled() || emptyClient.searchContextSize != "medium" {
		t.Fatalf("empty Meta resolved enabled=%v size=%q, want true/medium", emptyClient.IsWebSearchEnabled(), emptyClient.searchContextSize)
	}

	if _, err := newCompatClient(&ProviderConfig{
		Provider: ProviderMeta,
		APIKey:   "k",
		Model:    metaContributorModel,
		Meta:     &config.MetaProviderConfig{SearchContextSize: "huge"},
	}); err == nil {
		t.Fatal("search_context_size huge was accepted")
	}

	dash, err := newCompatClient(&ProviderConfig{
		Provider: ProviderDashScope,
		APIKey:   "k",
		Model:    "test-model",
		Meta:     config.DefaultMetaProviderConfig(),
	})
	if err != nil {
		t.Fatalf("dashscope: %v", err)
	}
	dashClient := dash.(*OpenAICompatClient)
	if dashClient.IsWebSearchEnabled() || dashClient.SupportsGrounding() || dashClient.searchContextSize != "" {
		t.Fatalf("dashscope took the meta block: enabled=%v capable=%v size=%q", dashClient.IsWebSearchEnabled(), dashClient.SupportsGrounding(), dashClient.searchContextSize)
	}

	class, err := NewClassificationClientFromConfig(&ProviderConfig{
		Provider: ProviderMeta,
		APIKey:   "k",
		Model:    metaContributorModel,
		Meta:     config.DefaultMetaProviderConfig(),
	})
	if err != nil || class == nil {
		t.Fatalf("classification client = %v, %v", class, err)
	}
	classClient := broker.Base(class).(*OpenAICompatClient)
	if classClient.IsWebSearchEnabled() {
		t.Fatal("classification client has web search on")
	}

	slotOff, err := newSecondarySlotClient(&config.UserConfig{
		MetaAPIKey: "mk",
		Meta:       &config.MetaProviderConfig{EnableWebSearch: &off, SearchContextSize: "high"},
	}, "worker", &config.SecondaryLLMConfig{Provider: "meta", Model: metaContributorModel})
	if err != nil {
		t.Fatalf("secondary explicit: %v", err)
	}
	slotClient := broker.Base(slotOff).(*OpenAICompatClient)
	if slotClient.IsWebSearchEnabled() || slotClient.searchContextSize != "high" {
		t.Fatalf("secondary explicit resolved enabled=%v size=%q, want false/high", slotClient.IsWebSearchEnabled(), slotClient.searchContextSize)
	}

	slotDefault, err := newSecondarySlotClient(&config.UserConfig{
		MetaAPIKey: "mk",
	}, "worker", &config.SecondaryLLMConfig{Provider: "meta", Model: metaContributorModel})
	if err != nil {
		t.Fatalf("secondary default: %v", err)
	}
	slotDefaultClient := broker.Base(slotDefault).(*OpenAICompatClient)
	if !slotDefaultClient.IsWebSearchEnabled() || slotDefaultClient.searchContextSize != "medium" {
		t.Fatalf("secondary nil Meta resolved enabled=%v size=%q, want true/medium", slotDefaultClient.IsWebSearchEnabled(), slotDefaultClient.searchContextSize)
	}

	pc, err := ProviderConfigFromUserConfig(&config.UserConfig{
		Provider:   "meta",
		MetaAPIKey: "k",
		Model:      metaContributorModel,
	})
	if err != nil {
		t.Fatalf("ProviderConfigFromUserConfig: %v", err)
	}
	if pc.Meta == nil || !pc.Meta.WebSearchEnabled() || pc.Meta.ResolvedSearchContextSize() != "medium" {
		t.Fatalf("ProviderConfig.Meta = %+v, want search on at medium", pc.Meta)
	}
}

// metaCommentaryFixture is the shape a grounded muse-spark-1.3-contributor run
// actually returned on 2026-09-29: progress notes between searches arrive as
// message items with phase "commentary", and only the final answer (no phase)
// carries the url_citation.
const metaCommentaryFixture = `{
  "id": "resp_x", "status": "completed",
  "output": [
    {"type": "reasoning", "id": "rs_1", "summary": [], "status": "completed"},
    {"type": "message", "id": "rs_2", "role": "assistant", "phase": "commentary", "status": "completed",
     "content": [{"type": "output_text", "text": "I'll search the web for the latest release.", "annotations": []}]},
    {"type": "web_search_call", "id": "ws_1", "status": "completed",
     "results": [{"type": "text_result", "title": "Downloads", "url": "https://example.com/downloads", "snippet": "3.14.7"}]},
    {"type": "message", "id": "rs_3", "role": "assistant", "phase": "commentary", "status": "completed",
     "content": [{"type": "output_text", "text": "The search points to 3.14.7; I'll confirm it.", "annotations": []}]},
    {"type": "web_search_call", "id": "ws_2", "status": "completed", "results": []},
    {"type": "message", "id": "msg_1", "role": "assistant", "status": "completed",
     "content": [{"type": "output_text", "text": "The latest release is 3.14.7.",
       "annotations": [{"type": "url_citation", "url": "https://example.com/downloads", "title": "Downloads", "start_index": 0, "end_index": 29}]}]}
  ]
}`

func TestMetaGrounding_CommentaryIsNotTheAnswer(t *testing.T) {
	var reply metaResponsesReply
	if err := json.Unmarshal([]byte(metaCommentaryFixture), &reply); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	const want = "The latest release is 3.14.7."
	if got := metaAssistantText(&reply); got != want {
		t.Fatalf("metaAssistantText = %q, want %q (commentary must not reach the answer)", got, want)
	}
	if got := metaTextFromReply(&reply); got != want {
		t.Fatalf("metaTextFromReply = %q, want %q", got, want)
	}
	g := metaGroundingFromReply(&reply)
	if len(g.Citations) != 1 || g.Citations[0].URL != "https://example.com/downloads" {
		t.Fatalf("citations = %+v", g.Citations)
	}
	if len(g.Results) != 1 {
		t.Fatalf("results = %+v", g.Results)
	}
}

func TestMetaGrounding_SeparateAnswerMessagesAreNotGlued(t *testing.T) {
	reply := metaResponsesReply{Output: []metaResponsesItem{
		{Type: "message", Role: "assistant", Content: []metaResponsesContent{{Type: "output_text", Text: "First part."}}},
		{Type: "message", Role: "assistant", Content: []metaResponsesContent{{Type: "output_text", Text: "Second part."}}},
	}}
	if got := metaAssistantText(&reply); got != "First part.\n\nSecond part." {
		t.Fatalf("metaAssistantText = %q", got)
	}
}
