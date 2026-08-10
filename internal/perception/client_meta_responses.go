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
// equivalent surface, so they are untouched.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// metaResponsesPath is the Responses endpoint, appended to the client base URL.
const metaResponsesPath = "/responses"

// metaReasoningInclude asks Meta to return reasoning as encrypted blocks that
// this client replays on the next turn. Documented as incompatible with
// previous_response_id, which is why this client never sets that field.
const metaReasoningInclude = "reasoning.encrypted_content"

// =============================================================================
// WIRE TYPES
// =============================================================================

// metaResponsesRequest is a POST /v1/responses body.
//
// `input` is deliberately []any: the array mixes message items, reasoning
// replay items, function_call items and function_call_output items, which share
// no common field set beyond "type". A typed union would cost more than it
// buys for a payload this client both builds and never re-reads.
type metaResponsesRequest struct {
	Model  string `json:"model"`
	Input  []any  `json:"input"`
	Stream bool   `json:"stream,omitempty"`

	// Include carries reasoning across turns. Never set alongside
	// previous_response_id — the API rejects the combination.
	Include []string `json:"include,omitempty"`

	Reasoning *metaReasoningConfig `json:"reasoning,omitempty"`

	// MaxOutputTokens shares its budget with input tokens and has a documented
	// floor of 16.
	MaxOutputTokens int `json:"max_output_tokens,omitempty"`

	Tools      []metaResponsesTool `json:"tools,omitempty"`
	ToolChoice string              `json:"tool_choice,omitempty"`

	// Store is a pointer so that "false" is transmitted. The field defaults to
	// true server-side, and a plain bool would make the off state omitempty
	// away — silently persisting every request.
	Store *bool `json:"store,omitempty"`

	// Truncation accepts only "disabled".
	Truncation string `json:"truncation,omitempty"`
}

type metaReasoningConfig struct {
	// Effort is minimal | low | medium | high | xhigh. Muse Spark rejects
	// "none", so callers must not send it.
	Effort string `json:"effort,omitempty"`
}

// metaResponsesTool is a function tool in the Responses shape. Note the flat
// layout: name, description and parameters sit on the tool itself rather than
// nested under a "function" key as they are in Chat Completions.
type metaResponsesTool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// metaResponsesReply is a POST /v1/responses response.
type metaResponsesReply struct {
	ID     string              `json:"id"`
	Status string              `json:"status"`
	Model  string              `json:"model"`
	Output []metaResponsesItem `json:"output"`
	Usage  *metaResponsesUsage `json:"usage,omitempty"`
	Error  *metaResponsesError `json:"error,omitempty"`
}

type metaResponsesError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

type metaResponsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`

	OutputTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
	InputTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

// metaResponsesItem is one entry in the output array. The meaningful fields
// depend on Type: "message" carries Content, "reasoning" carries
// EncryptedContent, "function_call" carries Name/Arguments/CallID.
type metaResponsesItem struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Role   string `json:"role,omitempty"`
	Status string `json:"status,omitempty"`

	Content []metaResponsesContent `json:"content,omitempty"`

	// Reasoning replay. Summary is echoed back verbatim; it is usually empty
	// and this client does not read it.
	Summary          []any  `json:"summary,omitempty"`
	EncryptedContent string `json:"encrypted_content,omitempty"`

	// function_call
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	CallID    string `json:"call_id,omitempty"`
}

type metaResponsesContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// =============================================================================
// INPUT CONSTRUCTION
// =============================================================================

// metaInputText builds a message item whose content is a single text block.
//
// The block type differs by role and is not interchangeable: user and system
// text is "input_text", assistant text replayed back to the model is
// "output_text". Sending "input_text" on an assistant turn is rejected.
func metaInputText(role, text string) map[string]any {
	blockType := "input_text"
	if role == "assistant" {
		blockType = "output_text"
	}
	return map[string]any{
		"role": role,
		"content": []map[string]any{
			{"type": blockType, "text": text},
		},
	}
}

// metaReasoningItem rebuilds a reasoning block for replay. The encrypted
// content is opaque and is passed straight back.
func metaReasoningItem(id, encrypted string) map[string]any {
	return map[string]any{
		"type":              "reasoning",
		"id":                id,
		"summary":           []any{},
		"encrypted_content": encrypted,
	}
}

// metaFunctionCallItem replays an assistant tool call. call_id must match the
// function_call_output that answers it, or the API rejects the pair.
func metaFunctionCallItem(callID, name, arguments string) map[string]any {
	if strings.TrimSpace(arguments) == "" {
		arguments = "{}"
	}
	return map[string]any{
		"type":      "function_call",
		"call_id":   callID,
		"name":      name,
		"arguments": arguments,
	}
}

// metaFunctionOutputItem carries a tool result back to the model.
func metaFunctionOutputItem(callID, output string) map[string]any {
	return map[string]any{
		"type":    "function_call_output",
		"call_id": callID,
		"output":  output,
	}
}

// metaToolsFromDefinitions converts codeNERD tool definitions into Responses
// tools. The shape is flatter than Chat Completions: no nested "function" key.
func metaToolsFromDefinitions(tools []ToolDefinition) []metaResponsesTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]metaResponsesTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, metaResponsesTool{
			Type:        "function",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		})
	}
	return out
}

// metaInputFromHistory converts codeNERD's conversation history into Responses
// input items.
//
// Reasoning replay is the point of this function. When a prior assistant turn
// carried reasoning blocks, they are emitted BEFORE that turn's tool calls, in
// the order Meta returned them — the model reads its own prior thinking and
// then the calls it made, which is the ordering that keeps a tool loop coherent.
//
// Tool results become separate function_call_output items rather than user
// messages, and each is paired to its call by call_id.
func metaInputFromHistory(systemPrompt string, history []types.Message, reasoning map[string][]metaResponsesItem) []any {
	input := make([]any, 0, len(history)+4)

	if strings.TrimSpace(systemPrompt) != "" {
		// "developer" is Meta's preferred role for instructions; "system" is
		// accepted but documented as the older spelling.
		input = append(input, metaInputText("developer", systemPrompt))
	}

	for i, msg := range history {
		switch msg.Role {
		case "assistant":
			// Replay this turn's reasoning first, keyed by turn index.
			for _, r := range reasoning[metaTurnKey(i)] {
				input = append(input, metaReasoningItem(r.ID, r.EncryptedContent))
			}
			if strings.TrimSpace(msg.Text) != "" {
				input = append(input, metaInputText("assistant", msg.Text))
			}
			for _, tc := range msg.ToolCalls {
				// ToolCall.Input is a decoded map; the wire wants the JSON
				// text the model originally emitted. A marshal failure must
				// not drop the call — an unpaired function_call_output on the
				// next turn is rejected by the API, so send "{}" and let the
				// pairing survive.
				args := "{}"
				if len(tc.Input) > 0 {
					if encoded, err := json.Marshal(tc.Input); err == nil {
						args = string(encoded)
					} else {
						logging.Get(logging.CategoryAPI).Warn(
							"meta responses: could not marshal args for tool %s (%v); sending {}", tc.Name, err)
					}
				}
				input = append(input, metaFunctionCallItem(tc.ID, tc.Name, args))
			}

		default:
			// Tool results are their own item type and must not be folded into
			// the user text, or the model cannot pair them with their calls.
			for _, tr := range msg.ToolResults {
				input = append(input, metaFunctionOutputItem(tr.ToolUseID, tr.Content))
			}
			if strings.TrimSpace(msg.Text) != "" {
				input = append(input, metaInputText("user", msg.Text))
			}
		}
	}

	return input
}

// metaTurnKey names a history position for the reasoning cache.
func metaTurnKey(i int) string { return fmt.Sprintf("turn:%d", i) }

// =============================================================================
// TRANSPORT
// =============================================================================

// supportsResponsesAPI reports whether this client should use the Responses
// surface. Meta only — DashScope and Moonshot have no equivalent.
func (c *OpenAICompatClient) supportsResponsesAPI() bool {
	return c.vendor == ProviderMeta
}

// executeResponses performs one POST to the Responses endpoint.
func (c *OpenAICompatClient) executeResponses(ctx context.Context, reqBody metaResponsesRequest) (*metaResponsesReply, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal responses request: %w", err)
	}

	c.throttle()

	url := strings.TrimSuffix(c.baseURL, "/") + metaResponsesPath
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("build responses request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("responses request failed after %v: %w", time.Since(start).Round(time.Millisecond), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read responses body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Surface the body: Meta's errors name the offending field, and losing
		// that turns a one-line fix into a guessing game.
		return nil, fmt.Errorf("responses HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var reply metaResponsesReply
	if err := json.Unmarshal(body, &reply); err != nil {
		return nil, fmt.Errorf("decode responses body: %w", err)
	}
	if reply.Error != nil && strings.TrimSpace(reply.Error.Message) != "" {
		return nil, fmt.Errorf("responses error (%s): %s", reply.Error.Type, reply.Error.Message)
	}

	if reply.Usage != nil {
		logging.Get(logging.CategoryAPI).Debug(
			"meta responses: in=%d out=%d reasoning=%d cached=%d status=%s in %v",
			reply.Usage.InputTokens, reply.Usage.OutputTokens,
			reply.Usage.OutputTokensDetails.ReasoningTokens,
			reply.Usage.InputTokensDetails.CachedTokens,
			reply.Status, time.Since(start).Round(time.Millisecond),
		)
	}

	return &reply, nil
}

// newResponsesRequest builds a request with this client's configured reasoning
// effort and output ceiling applied.
func (c *OpenAICompatClient) newResponsesRequest(ctx context.Context, input []any, thinking bool) metaResponsesRequest {
	store := false
	req := metaResponsesRequest{
		Model: c.ModelForContext(ctx),
		Input: input,
		// Replay reasoning rather than chaining server-side history.
		Include: []string{metaReasoningInclude},
		// With replay, server-side persistence buys nothing and leaves
		// conversation state on the vendor for no reason.
		Store:      &store,
		Truncation: "disabled",
	}

	if c.maxOutputTokens > 0 {
		req.MaxOutputTokens = c.maxOutputTokens
	}

	effort := c.reasoningEffortOverride
	if effort == "" && thinking {
		effort = c.reasoningEffortForContext(ctx)
	}
	// "none" is rejected by Muse Spark, so it is never forwarded.
	if effort != "" && effort != "none" {
		req.Reasoning = &metaReasoningConfig{Effort: effort}
	}

	return req
}

// =============================================================================
// RESPONSE PARSING
// =============================================================================

// metaTextFromReply concatenates the assistant text across message items.
func metaTextFromReply(reply *metaResponsesReply) string {
	var sb strings.Builder
	for _, item := range reply.Output {
		if item.Type != "message" {
			continue
		}
		for _, block := range item.Content {
			if block.Type == "output_text" && block.Text != "" {
				sb.WriteString(block.Text)
			}
		}
	}
	return sb.String()
}

// metaReasoningFromReply collects reasoning items for replay on the next turn.
func metaReasoningFromReply(reply *metaResponsesReply) []metaResponsesItem {
	var out []metaResponsesItem
	for _, item := range reply.Output {
		if item.Type == "reasoning" && item.EncryptedContent != "" {
			out = append(out, item)
		}
	}
	return out
}

// completeWithToolResultsViaResponses runs one tool-loop turn on the Responses
// surface, replaying the reasoning Meta produced on earlier turns.
//
// The replay cache is keyed by the assistant turn's position in history and
// held per client. It is deliberately not persisted: reasoning blocks belong to
// one conversation, and a stale block replayed into an unrelated turn would be
// worse than no replay at all. A cold start simply replays nothing and behaves
// exactly like Chat Completions did.
func (c *OpenAICompatClient) completeWithToolResultsViaResponses(
	ctx context.Context,
	systemPrompt string,
	history []types.Message,
	tools []ToolDefinition,
) (*LLMToolResponse, error) {
	c.reasoningMu.Lock()
	cache := make(map[string][]metaResponsesItem, len(c.reasoningCache))
	for k, v := range c.reasoningCache {
		cache[k] = v
	}
	c.reasoningMu.Unlock()

	input := metaInputFromHistory(systemPrompt, history, cache)

	req := c.newResponsesRequest(ctx, input, c.enableThinking)
	req.Tools = metaToolsFromDefinitions(tools)
	if len(req.Tools) > 0 {
		req.ToolChoice = "auto"
	}

	reply, err := c.executeResponses(ctx, req)
	if err != nil {
		return nil, err
	}

	// Record this turn's reasoning against the position the assistant reply
	// will occupy, so the next call replays it in the right place.
	if reasoning := metaReasoningFromReply(reply); len(reasoning) > 0 {
		c.reasoningMu.Lock()
		if c.reasoningCache == nil {
			c.reasoningCache = make(map[string][]metaResponsesItem)
		}
		c.reasoningCache[metaTurnKey(len(history))] = reasoning
		c.reasoningMu.Unlock()
		logging.Get(logging.CategoryAPI).Debug(
			"meta responses: cached %d reasoning block(s) for replay at %s",
			len(reasoning), metaTurnKey(len(history)))
	}

	return metaToolResponseFromReply(reply), nil
}

// metaToolResponseFromReply converts a Responses reply into codeNERD's
// vendor-neutral tool response.
func metaToolResponseFromReply(reply *metaResponsesReply) *LLMToolResponse {
	out := &LLMToolResponse{Text: metaTextFromReply(reply)}
	for _, item := range reply.Output {
		if item.Type != "function_call" {
			continue
		}
		// Arguments arrive as a JSON string; ToolCall.Input is the decoded map.
		// A call whose arguments will not parse is still returned, with empty
		// input rather than none at all: the executor can report a tool that
		// failed on its arguments, but a silently dropped call leaves the loop
		// waiting for a result that never comes.
		input := map[string]any{}
		if trimmed := strings.TrimSpace(item.Arguments); trimmed != "" {
			if err := json.Unmarshal([]byte(trimmed), &input); err != nil {
				logging.Get(logging.CategoryAPI).Warn(
					"meta responses: tool %s returned unparseable arguments (%v)", item.Name, err)
				input = map[string]any{}
			}
		}
		out.ToolCalls = append(out.ToolCalls, types.ToolCall{
			ID:    item.CallID,
			Name:  item.Name,
			Input: input,
		})
	}
	return out
}
