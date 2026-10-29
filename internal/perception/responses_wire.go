package perception

// Responses API wire types, shared by every client that speaks
// POST /v1/responses: OpenAI's own client (client_openai.go) and the Meta
// surface of OpenAICompatClient (client_meta_responses.go). Vendor quirks
// live in those clients; the shapes here are the protocol's.

import "codenerd/internal/types"

// responsesPath is the Responses endpoint, appended to the client base URL.
const responsesPath = "/responses"

// responsesReasoningInclude asks for reasoning as encrypted blocks the client
// replays on the next turn. Documented as incompatible with
// previous_response_id, which is why no codeNERD client sets that field.
// OpenAI now returns encrypted_content by default under store:false; models
// before that change still need the include, and the newer ones accept it.
const responsesReasoningInclude = "reasoning.encrypted_content"

// responsesWebSearchResultsInclude asks a web_search_call item to carry the pages
// it retrieved. Without it the item is only a marker and the pages are lost.
const responsesWebSearchResultsInclude = "web_search_call.results"

// responsesWebSearchSourcesInclude asks a web_search_call item to list every
// URL the search consulted under action.sources. OpenAI documents this one;
// it is a superset of the cited URLs the answer's annotations carry.
const responsesWebSearchSourcesInclude = "web_search_call.action.sources"

// responsesRequest is a POST /v1/responses body.
//
// `input` is deliberately []any: the array mixes message items, reasoning
// replay items, function_call items and function_call_output items, which share
// no common field set beyond "type". A typed union would cost more than it
// buys for a payload this client both builds and never re-reads.
type responsesRequest struct {
	Model  string `json:"model"`
	Input  []any  `json:"input"`
	Stream bool   `json:"stream,omitempty"`

	// Include carries reasoning across turns. Never set alongside
	// previous_response_id — the API rejects the combination.
	Include []string `json:"include,omitempty"`

	Reasoning *responsesReasoning `json:"reasoning,omitempty"`

	// MaxOutputTokens shares its budget with input tokens and has a documented
	// floor of 16.
	MaxOutputTokens int `json:"max_output_tokens,omitempty"`

	Tools      []responsesTool `json:"tools,omitempty"`
	ToolChoice string          `json:"tool_choice,omitempty"`

	// Store is a pointer so that "false" is transmitted. The field defaults to
	// true server-side, and a plain bool would make the off state omitempty
	// away — silently persisting every request. Note that store:false does not
	// disable prompt caching: cached prefixes are still reused and
	// cached_tokens still reports hits. It only turns off response retrieval.
	Store *bool `json:"store,omitempty"`

	// PromptCacheRetention asks for extended prefix retention. The default,
	// in_memory, is evicted under memory pressure, which showed up in practice
	// as cache hits alternating with total misses on a ~35k-token prefix --
	// roughly half of every conversation re-billed at the uncached rate. "24h"
	// requests retention for up to a day. It is a hint, not a guarantee: the
	// server still evicts under load.
	//
	// Models from OpenAI's GPT-5.6 generation on replace it with
	// PromptCacheOptions and reject it; a client sends one or the other.
	PromptCacheRetention string `json:"prompt_cache_retention,omitempty"`

	// PromptCacheOptions is the newer cache control: mode implicit|explicit
	// and ttl 30m|12h. A 12h write bills 2.5x the input rate (30m: 1.25x),
	// so the lifetime is the workspace's choice, not a client constant.
	PromptCacheOptions *responsesPromptCacheOptions `json:"prompt_cache_options,omitempty"`

	// PromptCacheKey routes requests that share a prefix to the same cache.
	// Without it the server infers grouping, which scatters when several slots
	// (main, worker, planner) issue interleaved requests carrying the same
	// system prompt and tool definitions.
	PromptCacheKey string `json:"prompt_cache_key,omitempty"`

	// Truncation accepts only "disabled".
	Truncation string `json:"truncation,omitempty"`

	// ServiceTier picks the processing tier (auto, default, flex, fast,
	// priority, ultrafast). Empty omits it: the project's own default.
	ServiceTier string `json:"service_tier,omitempty"`

	// Temperature is sent only to models that take sampling controls. Reasoning
	// models reject it whenever effort is not "none", so it is a pointer the
	// client sets deliberately rather than a zero that omitempty hides.
	Temperature *float64 `json:"temperature,omitempty"`

	// Text carries structured output and verbosity. Chat Completions uses
	// response_format; this surface does not have that field. Meta takes
	// only json_object (its strict json_schema 400s on additionalProperties);
	// OpenAI takes the strict json_schema form.
	Text *responsesText `json:"text,omitempty"`
}

// responsesPromptCacheOptions is prompt_cache_options.
type responsesPromptCacheOptions struct {
	Mode string `json:"mode,omitempty"`
	TTL  string `json:"ttl,omitempty"`
}

// responsesText is the Responses structured-output envelope.
type responsesText struct {
	Format *responsesTextFormat `json:"format,omitempty"`
	// Verbosity is low | medium | high: how much the answer says, separate
	// from how hard the model thinks.
	Verbosity string `json:"verbosity,omitempty"`
}

// responsesTextFormat is text.format. json_object sets Type alone; the
// json_schema form carries Name, Schema and Strict flat on the format, with
// no nested json_schema wrapper as Chat Completions has.
type responsesTextFormat struct {
	Type   string         `json:"type,omitempty"`
	Name   string         `json:"name,omitempty"`
	Schema map[string]any `json:"schema,omitempty"`
	Strict *bool          `json:"strict,omitempty"`
}

// responsesReasoning is the reasoning block. Which efforts a model accepts is
// the vendor's: Muse Spark rejects none and max; OpenAI's GPT-6 Astra and
// GPT-6.1 Sol reject none. Clients send the configured effort as written.
type responsesReasoning struct {
	Effort string `json:"effort,omitempty"`
	// Mode is standard | pro (OpenAI, GPT-5.6 and GPT-6 families).
	Mode string `json:"mode,omitempty"`
	// Context is auto | current_turn | all_turns: whether earlier turns'
	// replayed reasoning is used. Empty is the model's default.
	Context string `json:"context,omitempty"`
	// Summary is auto | concise | detailed: ask for readable summaries of
	// the reasoning, which stream as response.reasoning_summary_text.delta.
	Summary string `json:"summary,omitempty"`
}

// responsesTool is one Responses tool. Function tools are flat: name,
// description and parameters sit on the tool rather than under a "function"
// key. A web_search tool sets Type and, when configured, SearchContextSize;
// the function fields stay empty and omitempty drops them.
type responsesTool struct {
	Type              string         `json:"type"`
	Name              string         `json:"name,omitempty"`
	Description       string         `json:"description,omitempty"`
	Parameters        map[string]any `json:"parameters,omitempty"`
	SearchContextSize string         `json:"search_context_size,omitempty"`
}

// responsesReply is a POST /v1/responses response.
type responsesReply struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Model  string          `json:"model"`
	Output []responsesItem `json:"output"`
	Usage  *responsesUsage `json:"usage,omitempty"`
	Error  *responsesError `json:"error,omitempty"`
	// ServiceTier is the tier that actually served the request, which can
	// differ from the one asked for (a ramp limit downgrades fast to default).
	ServiceTier string `json:"service_tier,omitempty"`
	// IncompleteDetails says why a Status of "incomplete" stopped; the reason
	// this client acts on is "max_output_tokens".
	IncompleteDetails *responsesIncompleteDetails `json:"incomplete_details,omitempty"`
}

type responsesIncompleteDetails struct {
	Reason string `json:"reason"`
}

type responsesError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

type responsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`

	// The detail blocks are pointers so a category the response did not
	// carry stays unknown rather than reading as an observed zero.
	OutputTokensDetails *responsesOutputTokensDetails `json:"output_tokens_details,omitempty"`
	InputTokensDetails  *responsesInputTokensDetails  `json:"input_tokens_details,omitempty"`

	// PromptTokens and CompletionTokens are the Chat Completions names. A
	// response that uses them and leaves the Responses names at zero still
	// has to be billed; GroundedWebSearch's usage fallback depends on it.
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
}

// resolved returns input, output, and total tokens, preferring the Responses
// names and falling back to the Chat Completions names when both of those
// are zero.
func (u *responsesUsage) resolved() (in, out, total int) {
	if u == nil {
		return 0, 0, 0
	}
	in, out = u.InputTokens, u.OutputTokens
	if in == 0 && out == 0 {
		in, out = u.PromptTokens, u.CompletionTokens
	}
	total = u.TotalTokens
	if total == 0 && (in != 0 || out != 0) {
		total = in + out
	}
	return in, out, total
}

type responsesOutputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

type responsesInputTokensDetails struct {
	// CachedTokens is billed cache reads. On models that bill cache writes it
	// excludes the tokens charged as writes.
	CachedTokens int `json:"cached_tokens"`
	// CacheWriteTokens is the input written to a prompt cache on this request,
	// billed at the write rate. A pointer: models that cache for free never
	// report it, and that is unknown, not an observed zero.
	CacheWriteTokens *int `json:"cache_write_tokens,omitempty"`
}

// reasoningTokens is the reasoning subset of output, 0 when unreported.
func (u *responsesUsage) reasoningTokens() int {
	if u == nil || u.OutputTokensDetails == nil {
		return 0
	}
	return u.OutputTokensDetails.ReasoningTokens
}

// cachedTokens is the cache-read subset of input, 0 when unreported.
func (u *responsesUsage) cachedTokens() int {
	if u == nil || u.InputTokensDetails == nil {
		return 0
	}
	return u.InputTokensDetails.CachedTokens
}

// metadata normalizes the block. Responses reports reasoning inside
// output_tokens and cached tokens inside input_tokens, the categories
// types.UsageMetadata uses, so nothing is folded. Reasoning-model turns are
// mostly reasoning: the sub-count is what shows it.
func (u *responsesUsage) metadata() types.UsageMetadata {
	if u == nil {
		return types.UsageMetadata{}
	}
	in, out, total := u.resolved()
	m := types.UsageMetadata{InputTokens: in, OutputTokens: out, TotalTokens: total}
	if in > 0 || out > 0 {
		m.Known |= types.UsageTokens
	}
	if u.OutputTokensDetails != nil {
		m.ThinkingTokens = u.OutputTokensDetails.ReasoningTokens
		m.Known |= types.UsageReasoning
	}
	if u.InputTokensDetails != nil {
		m.CachedContentTokens = u.InputTokensDetails.CachedTokens
		m.Known |= types.UsageCacheRead
		if w := u.InputTokensDetails.CacheWriteTokens; w != nil {
			m.CacheWriteTokens = *w
			m.Known |= types.UsageCacheWrite
		}
	}
	return m
}

func (u *responsesUsage) outputTokens() int {
	_, out, _ := u.resolved()
	return out
}

func (u *responsesUsage) groundedUsage() types.GroundedUsage {
	in, out, total := u.resolved()
	return types.GroundedUsage{InputTokens: in, OutputTokens: out, TotalTokens: total}
}

// responsesItem is one entry in the output array. The meaningful fields
// depend on Type: "message" carries Content, "reasoning" carries
// EncryptedContent, "function_call" carries Name/Arguments/CallID.
type responsesItem struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Role   string `json:"role,omitempty"`
	Status string `json:"status,omitempty"`
	// Phase is "commentary" on progress notes a model emits between tool calls
	// or searches ("I'll search for..."). The final answer carries no phase
	// (Meta) or "final_answer" (OpenAI GPT-6).
	Phase string `json:"phase,omitempty"`

	Content []responsesContent `json:"content,omitempty"`

	// Reasoning replay. Summary is echoed back verbatim; it is usually empty
	// and this client does not read it.
	Summary          []any  `json:"summary,omitempty"`
	EncryptedContent string `json:"encrypted_content,omitempty"`

	// function_call
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	CallID    string `json:"call_id,omitempty"`

	// web_search_call. Present only when the request included
	// web_search_call.results. Reasoning items do not carry this.
	Results []responsesWebSearchResult `json:"results,omitempty"`

	// Action is the web_search_call's action. With
	// web_search_call.action.sources included, Sources lists every URL the
	// search consulted.
	Action *responsesWebSearchAction `json:"action,omitempty"`
}

// responsesWebSearchAction is a web_search_call's action block.
type responsesWebSearchAction struct {
	Type    string                     `json:"type,omitempty"`
	Query   string                     `json:"query,omitempty"`
	Sources []responsesWebSearchSource `json:"sources,omitempty"`
}

// responsesWebSearchSource is one consulted URL.
type responsesWebSearchSource struct {
	Type string `json:"type,omitempty"`
	URL  string `json:"url"`
}

// responsesWebSearchResult is one retrieved page. Type is documented as
// "text_result"; a hit with a usable URL is kept whatever the type says,
// because dropping it on a spelling would hide a page the model used.
type responsesWebSearchResult struct {
	Type    string `json:"type"`
	Title   string `json:"title,omitempty"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

type responsesContent struct {
	Type        string                `json:"type"`
	Text        string                `json:"text,omitempty"`
	Annotations []responsesAnnotation `json:"annotations,omitempty"`
}

type responsesAnnotation struct {
	Type       string `json:"type"`
	URL        string `json:"url"`
	Title      string `json:"title,omitempty"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
}
