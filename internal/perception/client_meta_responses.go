package perception

// Meta Model API — Responses surface.
//
// Why this file exists at all, when client_openai_compat.go already speaks to
// Meta perfectly well:
//
// Chat Completions throws Muse Spark's reasoning away between turns. Meta's own
// documentation is blunt about the consequence — each turn "reasons from
// scratch, so in multi-step and agentic loops the model can lose the thread of
// its own prior thinking and behave erratically: repeating work it already did,
// contradicting earlier steps, or dropping task context mid-loop."
//
// That is a description of codeNERD's tool loop failing. Measured 2026-08-10:
// tasks re-read the same file repeatedly, hit the 24-iteration ceiling at 72
// tool calls, and in one case added three imports and never wrote the function
// body they were for. The model was not the weak part; its reasoning was being
// discarded between every tool call and re-derived from the transcript.
//
// The Responses surface carries reasoning across turns. `meta:<model>` defaults
// to it; Chat Completions is the fallback, and codeNERD had been using the
// fallback.
//
// STATELESS REPLAY, NOT previous_response_id
//
// Meta offers two ways to carry reasoning. `previous_response_id` chains to
// server-stored history, and `include: ["reasoning.encrypted_content"]` returns
// the reasoning as opaque blocks the caller replays itself. They are mutually
// exclusive — the API rejects both together.
//
// This client uses the second. codeNERD's tool loop already rebuilds the whole
// conversation every turn (session.Executor owns the history), so replay fits
// the existing shape without introducing server-side state that a retry, a
// crash, or a second process could desynchronise. The reasoning blocks are
// opaque ciphertext; this client stores and returns them without inspecting
// them.
//
// SCOPE
//
// Meta only. DashScope and Moonshot share client_openai_compat.go and have no
// equivalent surface, so they are untouched. The protocol code itself (wire
// types, input, reply parsing, transport) is shared with OpenAI's client in
// responses_*.go; this file keeps what is Meta's.
//
// WEB SEARCH
//
// Meta's web_search tool exists only on this surface. When the client has
// search enabled, plain completions, streaming, and the first tool turn are
// built by the same request function the later tool turns already use, with
// the web_search tool appended and include set to web_search_call.results.
// Search off leaves plain completions and the first tool turn on Chat
// Completions. An explicit GroundedWebSearch always sends the tool, even
// when the switch is off: the caller asked for a search.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// =============================================================================
// TRANSPORT
// =============================================================================

// supportsResponsesAPI reports whether this client should use the Responses
// surface. Meta only — DashScope and Moonshot have no equivalent.
func (c *OpenAICompatClient) supportsResponsesAPI() bool {
	return c.vendor == ProviderMeta
}

// executeResponses performs one POST to the Responses endpoint.
// op is the usage bucket for this one request (chat, tool_gen). A search
// that keeps its own sanitized transport does not come through here, so it
// is not counted twice.
func (c *OpenAICompatClient) executeResponses(ctx context.Context, reqBody responsesRequest, op string) (*responsesReply, error) {
	c.throttle()
	start := time.Now()
	reply, err := postResponses(ctx, responsesEndpoint{
		httpClient: c.httpClient, baseURL: c.baseURL, apiKey: c.apiKey, provider: c.vendor,
	}, reqBody)
	if err != nil {
		return nil, err
	}
	if reply.Usage != nil {
		// Reasoning tokens are billed as output and reported inside it; the
		// sub-count travels with the report so both consumers see it.
		reportUsage(ctx, c.model, c.vendor, op, reply.Usage.metadata())
		logResponsesUsage(c.vendor, reply, start)
	}
	return reply, nil
}

// metaPromptCacheRetention asks for the longer of the two retention options.
// Muse Spark rejects "none", so the only real choice is between the in-memory
// default and this.
const metaPromptCacheRetention = "24h"

// newResponsesRequest builds a request with this client's configured reasoning
// effort (or a shard profile's, effortFor) and output ceiling applied.
func (c *OpenAICompatClient) newResponsesRequest(ctx context.Context, input []any) responsesRequest {
	store := false
	req := responsesRequest{
		Model: c.ModelForContext(ctx),
		Input: input,
		// Replay reasoning rather than chaining server-side history.
		Include: []string{responsesReasoningInclude},
		// With replay, server-side persistence buys nothing and leaves
		// conversation state on the vendor for no reason.
		Store:      &store,
		Truncation: "disabled",
		// Long conversations against a large, stable prefix are exactly the
		// case extended retention exists for; the in-memory default was being
		// evicted mid-conversation.
		PromptCacheRetention: metaPromptCacheRetention,
	}

	if c.maxOutputTokens > 0 {
		req.MaxOutputTokens = c.maxOutputTokens
	}

	if effort := c.effortFor(ctx); effort != "" {
		req.Reasoning = &responsesReasoning{Effort: effort}
	}

	return req
}

// =============================================================================
// COMPLETION PATHS
// =============================================================================

// completeWithToolResultsViaResponses runs one tool-loop turn on the Responses
// surface, replaying the reasoning Meta produced on earlier turns.
//
// Reasoning is replayed only from the turn's own blocks: a BlockThinking block
// carrying a Signature becomes a reasoning replay item, in position. A text-only
// assistant turn replays no reasoning. There is no side cache and no second
// source of truth.
func (c *OpenAICompatClient) completeWithToolResultsViaResponses(
	ctx context.Context,
	systemPrompt string,
	history []types.Message,
	tools []ToolDefinition,
) (*LLMToolResponse, error) {
	// The Chat path validates Meta's 400-triggering payload limits before
	// sending; this path must too, or a bad call_id dies at the vendor.
	if err := c.validateMetaHistory(tools, history); err != nil {
		return nil, err
	}

	c.clearGroundingSources()
	input := responsesInputFromHistory(systemPrompt, history)

	req := c.newResponsesRequest(ctx, input)
	req.Tools = responsesToolsFromDefinitions(tools)
	if len(req.Tools) > 0 {
		req.ToolChoice = "auto"
	}
	// The cache key hashes the tool catalog, so the search tool has to be
	// on the request before the key is computed. attachWebSearch is a no-op
	// while search is off, which leaves this turn's body as it was.
	if err := c.attachWebSearch(&req, tools); err != nil {
		return nil, err
	}
	req.PromptCacheKey = promptCacheKey(req.Model, req.Tools)

	reply, err := c.executeResponses(ctx, req, usageOpToolGen)
	if err != nil {
		return nil, err
	}
	// A failed or cancelled run carries no usable output. Returning it as a
	// success would hand the executor an empty turn it mistakes for a final
	// answer, so this is an error carrying the vendor's own message.
	// Sources reach callers through the client (GetLastGroundingSources), the
	// one channel every grounding reader uses.
	captured := responsesGroundingFromReply(reply)
	c.recordGroundingSources(captured.Sources)
	if err := c.responsesTerminalError(reply, "CompleteWithToolResults"); err != nil {
		return nil, err
	}
	return responsesToolResponseFromReply(reply), nil
}

// validateMetaHistory guards the Responses path the way validateMetaTools
// guards the Chat path: Meta rejects out-of-range call_ids and malformed
// function names with HTTP 400 on this surface too, and this path never
// builds OpenAIMessage values, so the Chat-shaped validator cannot see it.
// Failing here keeps a vendor 400 with an opaque body from killing the turn.
func (c *OpenAICompatClient) validateMetaHistory(tools []ToolDefinition, history []types.Message) error {
	if c.vendor != ProviderMeta {
		return nil
	}
	for _, t := range tools {
		if err := validateMetaToolName(t.Name); err != nil {
			return err
		}
	}
	for _, m := range history {
		for _, tc := range m.ToolCalls {
			if err := validateMetaCallID(tc.ID); err != nil {
				return fmt.Errorf("assistant tool call %s: %w", tc.ID, err)
			}
		}
		for _, tr := range m.ToolResults {
			if err := validateMetaCallID(tr.ToolUseID); err != nil {
				return fmt.Errorf("tool result %s: %w", tr.ToolUseID, err)
			}
		}
	}
	return nil
}

// metaReservedSearchTools are the names Meta reserves for the web_search
// tool. A function tool with one of these names is legal on its own and a
// 400 once web_search is on the same request, so the check runs only when
// the search tool is about to be attached.
var metaReservedSearchTools = map[string]struct{}{
	"browser.search": {},
	"browser.open":   {},
	"browser.find":   {},
}

func (c *OpenAICompatClient) metaWebSearchActive() bool {
	if c == nil || c.vendor != ProviderMeta {
		return false
	}
	c.mu.Lock()
	on := c.enableWebSearch
	c.mu.Unlock()
	return on
}

// attachWebSearch adds the search tool when this client has grounding on.
// It is a no-op otherwise, which is what keeps a search-off Meta request
// byte-for-byte on its previous path.
func (c *OpenAICompatClient) attachWebSearch(req *responsesRequest, tools []ToolDefinition) error {
	if !c.metaWebSearchActive() {
		return nil
	}
	return c.forceWebSearch(req, tools)
}

// forceWebSearch attaches web_search whether or not the config switch is on.
// GroundedWebSearch uses it: that call is a search even when ordinary
// completions are not.
func (c *OpenAICompatClient) forceWebSearch(req *responsesRequest, tools []ToolDefinition) error {
	if err := rejectReservedSearchTools(tools); err != nil {
		return err
	}
	req.Tools = append(req.Tools, c.webSearchTool())
	req.Include = appendWebSearchInclude(req.Include)
	return nil
}

func rejectReservedSearchTools(tools []ToolDefinition) error {
	for _, t := range tools {
		if _, reserved := metaReservedSearchTools[t.Name]; reserved {
			return fmt.Errorf("tool name %q is reserved by Meta web search (browser.search, browser.open, browser.find)", t.Name)
		}
	}
	return nil
}

func (c *OpenAICompatClient) webSearchTool() responsesTool {
	tool := responsesTool{Type: "web_search"}
	if c != nil {
		if size := strings.TrimSpace(c.searchContextSize); size != "" {
			tool.SearchContextSize = size
		}
	}
	return tool
}

// SupportsGrounding reports the capability, not the current switch. A Meta
// client can ground; DashScope and Moonshot share this Go type and cannot.
func (c *OpenAICompatClient) SupportsGrounding() bool {
	return c != nil && c.vendor == ProviderMeta
}

// SetEnableWebSearch implements types.GroundingController. Non-Meta vendors
// ignore it: the method set is shared, the wire is not.
func (c *OpenAICompatClient) SetEnableWebSearch(enable bool) {
	if c == nil || c.vendor != ProviderMeta {
		return
	}
	c.mu.Lock()
	c.enableWebSearch = enable
	c.mu.Unlock()
}

// IsWebSearchEnabled implements types.GroundingController.
func (c *OpenAICompatClient) IsWebSearchEnabled() bool {
	if !c.SupportsGrounding() {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enableWebSearch
}

// SetEnableURLContext is a no-op. Meta has no URL-context tool; the method
// exists so this client satisfies the same controller Gemini does.
func (c *OpenAICompatClient) SetEnableURLContext(bool) {}

// SetURLContextURLs is a no-op. See SetEnableURLContext.
func (c *OpenAICompatClient) SetURLContextURLs([]string) {}

// IsURLContextEnabled is always false. Meta does not serve URL context.
func (c *OpenAICompatClient) IsURLContextEnabled() bool { return false }

// GetLastGroundingSources returns the URLs the previous completion cited or
// retrieved. The slice is a copy.
func (c *OpenAICompatClient) GetLastGroundingSources() []string {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.lastGroundingSources) == 0 {
		return nil
	}
	return append([]string(nil), c.lastGroundingSources...)
}

func (c *OpenAICompatClient) clearGroundingSources() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.lastGroundingSources = nil
	c.mu.Unlock()
}

func (c *OpenAICompatClient) recordGroundingSources(sources []string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	if len(sources) == 0 {
		c.lastGroundingSources = nil
	} else {
		c.lastGroundingSources = append([]string(nil), sources...)
	}
	c.mu.Unlock()
}

// responsesTerminalError reports a run that did not finish as an answer.
// A length stop carries the partial text on the truncation error; the broker
// restates the request rather than accepting a cut answer.
func (c *OpenAICompatClient) responsesTerminalError(reply *responsesReply, method string) error {
	return responsesTerminal(reply, c.vendor, c.model, method, c.maxOutputTokens)
}

// postResponsesCompletion is one plain (no function tools) Responses call.
// forceEffort replaces the request's reasoning effort; "minimal" is the
// empty-completion retry, and it wins over a configured override the same
// way the Chat path overwrites reasoning_effort for that retry.
func (c *OpenAICompatClient) postResponsesCompletion(ctx context.Context, systemPrompt, userPrompt string, forceEffort string) (string, *responsesReply, error) {
	req := c.newResponsesRequest(ctx, responsesInputFromHistory(systemPrompt, []types.Message{{Role: "user", Text: userPrompt}}))
	if forceEffort != "" {
		if forceEffort == "none" {
			req.Reasoning = nil
		} else {
			req.Reasoning = &responsesReasoning{Effort: forceEffort}
		}
	}
	if isPiggybackPrompt(ctx, systemPrompt, userPrompt) {
		req.Text = &responsesText{Format: &responsesTextFormat{Type: "json_object"}}
	}
	if err := c.attachWebSearch(&req, nil); err != nil {
		return "", nil, err
	}
	req.PromptCacheKey = promptCacheKey(req.Model, req.Tools)
	reply, err := c.executeResponses(ctx, req, usageOpChat)
	if err != nil {
		return "", nil, err
	}
	return responsesTextFromReply(reply), reply, nil
}

// completeWithSystemGrounded is CompleteWithSystem for a Meta client that
// has web search on. The retry stays on this surface: falling back to Chat
// Completions would drop the search tool the caller turned on.
func (c *OpenAICompatClient) completeWithSystemGrounded(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	start := time.Now()
	c.clearGroundingSources()

	text, reply, err := c.postResponsesCompletion(ctx, systemPrompt, userPrompt, "")
	if err != nil {
		logging.PerceptionError("[%s] CompleteWithSystem failed after %v: %v", c.vendor, time.Since(start), err)
		return "", err
	}
	if term := c.responsesTerminalError(reply, "CompleteWithSystem"); term != nil {
		c.recordGroundingSources(responsesGroundingFromReply(reply).Sources)
		return "", term
	}
	out := strings.TrimSpace(text)
	billed := reply.Usage.outputTokens()
	// Same shape as the Chat path's empty retry: tokens were billed and no
	// text came back, so the budget went to reasoning. This surface does not
	// return a readable reasoning trace, so the token count is the signal.
	// A zero-token empty body is a real empty answer and is not retried.
	if out == "" && !allowEmptyCompletion(ctx) && billed > 0 {
		logging.PerceptionWarn(
			"[%s] empty content (0 chars of reasoning, %d completion tokens); retrying once with minimal reasoning",
			c.vendor, billed)
		retryText, retryReply, retryErr := c.postResponsesCompletion(ctx, systemPrompt, userPrompt, "minimal")
		if retryErr == nil && c.responsesTerminalError(retryReply, "CompleteWithSystem") == nil {
			if retried := strings.TrimSpace(retryText); retried != "" {
				c.recordGroundingSources(responsesGroundingFromReply(retryReply).Sources)
				logging.PerceptionWarn("[%s] retry with minimal reasoning recovered %d chars", c.vendor, len(retried))
				return retried, nil
			}
		}
	}
	if out == "" {
		if allowEmptyCompletion(ctx) {
			c.recordGroundingSources(responsesGroundingFromReply(reply).Sources)
			logging.PerceptionDebug("[%s] empty completion allowed by caller (model=%s status=%q output_tokens=%d)",
				c.vendor, c.model, reply.Status, billed)
			return "", nil
		}
		return "", fmt.Errorf("%s returned an empty completion (model=%s status=%q reasoning_chars=0 output_tokens=%d); "+
			"if status is \"completed\" with 0 content the completion budget was likely consumed by reasoning",
			c.vendor, c.model, reply.Status, billed)
	}
	c.recordGroundingSources(responsesGroundingFromReply(reply).Sources)
	logging.Perception("[%s] CompleteWithSystem: model=%s completed in %v response_len=%d",
		c.vendor, c.model, time.Since(start), len(out))
	return out, nil
}
