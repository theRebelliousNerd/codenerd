package perception

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// AnthropicClient calls the direct Messages API. Inference, schema output and
// native tool loops share the same transport, cancellation and retry contract.
type AnthropicClient struct {
	apiKey          string
	baseURL         string
	httpClient      *http.Client
	maxOutputTokens int
	// maxInputTokens is the slot's input cap (max_input_tokens); 0 is none.
	// Every request entry point refuses a larger request before sending it
	// (admitInput), and InputWindowTokens reports it to the executor.
	maxInputTokens  int
	options         config.AnthropicProviderConfig
	reasoningEffort string
	configErr       error

	stateMu             sync.RWMutex
	model               string
	enableSystemCaching bool
	// lastGroundingSources are the URLs the last exchange's server tools
	// returned and its text cited.
	lastGroundingSources []string
	mu                   sync.Mutex
	lastRequest          time.Time
}

var (
	_ LLMClient                 = (*AnthropicClient)(nil)
	_ types.ToolResultsProvider = (*AnthropicClient)(nil)
	_ types.InputWindowReporter = (*AnthropicClient)(nil)
)

func DefaultAnthropicConfig(apiKey string) AnthropicConfig {
	return AnthropicConfig{APIKey: apiKey, BaseURL: "https://api.anthropic.com/v1", Timeout: 10 * time.Minute}
}

func NewAnthropicClientWithConfig(cfg AnthropicConfig) *AnthropicClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultAnthropicConfig(cfg.APIKey).BaseURL
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultAnthropicConfig(cfg.APIKey).Timeout
	}
	c := &AnthropicClient{
		apiKey: cfg.APIKey, baseURL: strings.TrimRight(cfg.BaseURL, "/"), model: strings.TrimSpace(cfg.Model),
		httpClient: NewSharedHTTPClient(cfg.Timeout), maxOutputTokens: orDefaultTokens(cfg.MaxOutputTokens, 8192),
		maxInputTokens: cfg.MaxInputTokens, options: cfg.Options, reasoningEffort: cfg.ReasoningEffort,
	}
	// Never forward the custom x-api-key header to a redirect destination.
	c.httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	c.configErr = cfg.Options.Validate()
	if c.configErr == nil {
		c.configErr = config.ValidateAnthropicEffort(cfg.ReasoningEffort)
	}
	if c.configErr == nil && cfg.Options.Thinking == "enabled" && cfg.Options.ThinkingBudgetTokens >= c.maxOutputTokens {
		c.configErr = fmt.Errorf("Anthropic thinking budget must be smaller than max_output_tokens")
	}
	if c.configErr == nil && cfg.MaxOutputTokens < 0 {
		c.configErr = fmt.Errorf("max_output_tokens must not be negative")
	}
	if c.configErr == nil && cfg.MaxInputTokens < 0 {
		c.configErr = fmt.Errorf("max_input_tokens must not be negative")
	}
	return c
}

// InputWindowTokens implements types.InputWindowReporter: the slot's
// max_input_tokens, 0 when the slot sets none.
func (c *AnthropicClient) InputWindowTokens() int { return c.maxInputTokens }

// admitInput is the client's backstop for its input cap: a request larger
// than max_input_tokens is refused before it is built or sent. The session
// executor fits its own requests to the same window by the same measure, so
// what reaches this refusal is a request that was never fitted -- a direct
// caller of the slot. It is the admission of every request that goes through
// exchange, so it counts the server web tool declarations exchange appends.
func (c *AnthropicClient) admitInput(system string, messages []types.Message, defs []types.ToolDefinition) error {
	return c.admit(system, messages, defs, c.serverTools(c.GetModel()))
}

// admit measures a request against the slot's input cap together with the
// server tool declarations it will carry (none for a streamed request, which
// declares no server tool). Those declarations are not in defs, so a count
// that left them out would be short by exactly their size.
func (c *AnthropicClient) admit(system string, messages []types.Message, defs []types.ToolDefinition, server []AnthropicTool) error {
	declared, err := serverDeclarationTokens(server)
	if err != nil {
		return fmt.Errorf("%s %s: the server tool declarations could not be measured against the slot's max_input_tokens of %d, so the request was not sent: %w",
			ProviderAnthropic, c.GetModel(), c.maxInputTokens, err)
	}
	return admitInputWindow(c.maxInputTokens, ProviderAnthropic, c.GetModel(), system, messages, defs, declared)
}

func (c *AnthropicClient) EnableSystemCaching() {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.enableSystemCaching = true
}

func (c *AnthropicClient) systemCaching() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.enableSystemCaching
}

func (c *AnthropicClient) request(ctx context.Context, system string, messages []AnthropicMessage, tools []AnthropicTool) (AnthropicRequest, error) {
	if c.configErr != nil {
		return AnthropicRequest{}, c.configErr
	}
	if c.apiKey == "" {
		return AnthropicRequest{}, fmt.Errorf("API key not configured")
	}
	model := c.GetModel()
	if model == "" {
		return AnthropicRequest{}, fmt.Errorf("no model configured for Anthropic")
	}
	// The caller owns the system prompt: a blank one sends no system field.
	if strings.TrimSpace(system) == "" {
		system = ""
	}
	req := AnthropicRequest{Model: model, MaxTokens: c.maxOutputTokens, System: system, Messages: messages, Tools: tools}
	if c.options.Thinking != "" {
		req.Thinking = &AnthropicThinking{Type: c.options.Thinking, BudgetTokens: c.options.ThinkingBudgetTokens, Display: c.options.ThinkingDisplay}
	}
	effort, err := c.effortFor(ctx)
	if err != nil {
		return AnthropicRequest{}, err
	}
	// The effort is the config's -- the slot's reasoning_effort, defaulted per
	// slot by config, or a shard profile's riding the request -- and it is sent
	// to every model that accepts one. A model that takes none is not sent it.
	if effort != "" && config.AnthropicSupportsEffort(model) {
		req.OutputConfig = &AnthropicOutputConfig{Effort: effort}
		// Older adaptive-capable models leave thinking off by default; an
		// effort should actually enable it there. An explicit
		// anthropic.thinking (disabled included) is left as configured.
		if req.Thinking == nil && config.AnthropicSupportsAdaptive(model) {
			req.Thinking = &AnthropicThinking{Type: "adaptive"}
		}
	}
	return req, nil
}

// effortFor is the effort a request is sent with: a shard profile's on the
// request context (types.WithReasoningEffort), else this slot's configured
// effort. A context effort is checked as the slot's own was at construction;
// one Anthropic cannot accept fails the request instead of reaching the wire.
func (c *AnthropicClient) effortFor(ctx context.Context) (string, error) {
	effort, ok := types.ReasoningEffortFromContext(ctx)
	if !ok {
		return c.reasoningEffort, nil
	}
	if err := config.ValidateAnthropicEffort(effort); err != nil {
		return "", fmt.Errorf("shard profile reasoning_effort %q: %w", effort, err)
	}
	return effort, nil
}

// anthropicCachePlan says where a request places its prompt-cache
// breakpoints. The endpoint caches a prefix up to each breakpoint and serves
// later requests that repeat that prefix at a tenth of the input price.
type anthropicCachePlan struct {
	// system puts one breakpoint on the system block. Tools render before
	// system, so this one marker caches both: the part of every tool-loop
	// request that never changes within a loop.
	system bool
	// tail puts one breakpoint on the last block of the newest turn, so the
	// next round reads the whole conversation so far from the cache and pays
	// full price only for what that round appended.
	tail bool
}

// toolLoopCachePlan is the plan for a tool-loop round. The loop resends the
// same tools, system prompt and growing transcript every round (about 45-55k
// tokens per round, Docs/journeys/11-idea-corpus-2026-09-26/tokens.md), so
// without breakpoints every round rebills the whole prefix at full price.
var toolLoopCachePlan = anthropicCachePlan{system: true, tail: true}

// ephemeral is the default 5-minute cache entry. A read refreshes it, and loop
// rounds start well inside five minutes of each other.
func ephemeral() *AnthropicCacheControl { return &AnthropicCacheControl{Type: "ephemeral"} }

// marshalAnthropicRequest renders the request with the plan's breakpoints.
// With no breakpoints it is the plain request, byte for byte what the client
// always sent.
func marshalAnthropicRequest(base AnthropicRequest, plan anthropicCachePlan) ([]byte, error) {
	if !plan.system && !plan.tail {
		return json.Marshal(base)
	}
	cached := anthropicCachedRequest{
		Model:        base.Model,
		MaxTokens:    base.MaxTokens,
		Messages:     base.Messages,
		Tools:        base.Tools,
		Thinking:     base.Thinking,
		OutputConfig: base.OutputConfig,
		Stream:       base.Stream,
	}
	if base.System != "" {
		block := AnthropicSystemCacheBlock{Type: "text", Text: base.System}
		if plan.system {
			block.CacheControl = ephemeral()
		}
		cached.System = []AnthropicSystemCacheBlock{block}
	}
	if plan.tail {
		cached.Messages = withTailBreakpoint(base.Messages)
	}
	return json.Marshal(cached)
}

// withTailBreakpoint returns messages with a cache breakpoint on the last
// block of the newest turn that can carry one. The caller's slice is not
// modified: a retry marshals the same request again, and the history it came
// from belongs to the executor.
//
// A bare-string turn becomes a single text block, which is the same content in
// the structured form. A thinking block cannot carry a breakpoint, so the
// marker goes on the nearest block before it.
func withTailBreakpoint(messages []AnthropicMessage) []AnthropicMessage {
	if len(messages) == 0 {
		return messages
	}
	out := make([]AnthropicMessage, len(messages))
	copy(out, messages)
	last := &out[len(out)-1]
	switch content := last.Content.(type) {
	case string:
		if content == "" {
			return out
		}
		last.Content = []AnthropicContentBlock{{Type: "text", Text: content, CacheControl: ephemeral()}}
	case []AnthropicContentBlock:
		blocks := make([]AnthropicContentBlock, len(content))
		copy(blocks, content)
		for i := len(blocks) - 1; i >= 0; i-- {
			if blocks[i].Type == "thinking" || blocks[i].Type == "redacted_thinking" {
				continue
			}
			blocks[i].CacheControl = ephemeral()
			break
		}
		last.Content = blocks
	}
	return out
}

func (c *AnthropicClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

func (c *AnthropicClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	if err := c.admitInput(system, userTurn(user), nil); err != nil {
		return "", err
	}
	req, err := c.request(ctx, system, []AnthropicMessage{{Role: "user", Content: user}}, nil)
	if err != nil {
		return "", err
	}
	return c.completeText(ctx, req, anthropicInput{history: userTurn(user)}, "CompleteWithSystem", usageOpChat)
}

func (c *AnthropicClient) completeText(ctx context.Context, req AnthropicRequest, in anthropicInput, operation, usageOperation string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.httpClient.Timeout)
	defer cancel()
	resp, err := c.exchange(ctx, req, anthropicCachePlan{system: c.systemCaching()}, in)
	if err != nil {
		return "", err
	}
	reportUsage(ctx, req.Model, ProviderAnthropic, usageOperation, resp.Usage.usageMetadata())
	var text strings.Builder
	for _, b := range resp.Content {
		if b.Type == "text" {
			text.WriteString(b.Text)
		}
	}
	result := strings.TrimSpace(text.String())
	if types.LengthStop(resp.StopReason) {
		return "", outputTruncated(ProviderAnthropic, req.Model, operation, resp.StopReason, result, req.MaxTokens, resp.Usage.OutputTokens)
	}
	if resp.StopReason == "refusal" {
		return "", &types.StreamError{Outcome: types.StreamRefused, Provider: string(ProviderAnthropic), Model: req.Model, Reason: resp.StopReason, Detail: "request declined by provider"}
	}
	if resp.StopReason != "end_turn" && resp.StopReason != "stop_sequence" {
		return "", fmt.Errorf("Anthropic text completion ended with unexpected stop_reason %q", resp.StopReason)
	}
	if result == "" {
		return "", fmt.Errorf("no text completion returned by Anthropic")
	}
	return result, nil
}

func (c *AnthropicClient) tools(defs []ToolDefinition) ([]AnthropicTool, error) {
	tools := make([]AnthropicTool, len(defs))
	for i, t := range defs {
		schema := t.InputSchema
		if c.options.StrictTools {
			var err error
			schema, _, err = prepareAnthropicSchema(t.InputSchema)
			if err != nil {
				return nil, fmt.Errorf("strict tool %q: %w", t.Name, err)
			}
		}
		tools[i] = AnthropicTool{Name: t.Name, Description: t.Description, InputSchema: schema, Strict: c.options.StrictTools}
	}
	return tools, nil
}

func (c *AnthropicClient) CompleteWithTools(ctx context.Context, system, user string, defs []ToolDefinition) (*LLMToolResponse, error) {
	if err := c.admitInput(system, userTurn(user), defs); err != nil {
		return nil, err
	}
	tools, err := c.tools(defs)
	if err != nil {
		return nil, err
	}
	req, err := c.request(ctx, system, []AnthropicMessage{{Role: "user", Content: user}}, tools)
	if err != nil {
		return nil, err
	}
	return c.completeTools(ctx, req, anthropicInput{history: userTurn(user), defs: defs}, anthropicCachePlan{system: true}, "CompleteWithTools")
}

func (c *AnthropicClient) CompleteWithToolResults(ctx context.Context, system string, history []types.Message, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if len(history) == 0 {
		return nil, fmt.Errorf("history must contain at least one message")
	}
	if err := c.admitInput(system, history, defs); err != nil {
		return nil, err
	}
	messages, err := buildAnthropicMessagesFromHistory(history)
	if err != nil {
		return nil, fmt.Errorf("invalid history: %w", err)
	}
	tools, err := c.tools(defs)
	if err != nil {
		return nil, err
	}
	req, err := c.request(ctx, system, messages, tools)
	if err != nil {
		return nil, err
	}
	return c.completeTools(ctx, req, anthropicInput{history: history, defs: defs}, toolLoopCachePlan, "CompleteWithToolResults")
}

func (c *AnthropicClient) completeTools(ctx context.Context, req AnthropicRequest, in anthropicInput, plan anthropicCachePlan, operation string) (*LLMToolResponse, error) {
	defs := in.defs
	ctx, cancel := context.WithTimeout(ctx, c.httpClient.Timeout)
	defer cancel()
	resp, err := c.exchange(ctx, req, plan, in)
	if err != nil {
		return nil, err
	}
	reportUsage(ctx, req.Model, ProviderAnthropic, usageOpFor(len(defs)), resp.Usage.usageMetadata())
	out := anthropicToolResponse(resp)
	if types.LengthStop(resp.StopReason) {
		return nil, outputTruncated(ProviderAnthropic, req.Model, operation, resp.StopReason, out.Text, req.MaxTokens, resp.Usage.OutputTokens)
	}
	if resp.StopReason == "refusal" {
		return nil, &types.StreamError{Outcome: types.StreamRefused, Provider: string(ProviderAnthropic), Model: req.Model, Reason: resp.StopReason, Detail: "request declined by provider"}
	}
	switch resp.StopReason {
	case "end_turn", "stop_sequence", "tool_use":
	default:
		return nil, fmt.Errorf("Anthropic tool completion ended with unsupported stop_reason %q", resp.StopReason)
	}
	// Unknown block types must not disappear and masquerade as a completed
	// turn. The server web tools are wired (exchange): their blocks are kept
	// verbatim for replay and never executed here. Other server-side
	// execution and compaction would need their own wiring.
	for _, b := range resp.Content {
		switch b.Type {
		case "text", "thinking", "redacted_thinking":
		case "server_tool_use", "web_search_tool_result", "web_fetch_tool_result":
		case "tool_use":
			if b.ID == "" || b.Name == "" || b.Input == nil {
				return nil, fmt.Errorf("Anthropic returned an incomplete tool_use block")
			}
			if c.options.StrictTools {
				if err := validateAnthropicToolCall(b, defs); err != nil {
					return nil, err
				}
			}
		default:
			return nil, fmt.Errorf("Anthropic returned unsupported content block %q", b.Type)
		}
	}
	if len(out.ToolCalls) != 0 && resp.StopReason != "tool_use" {
		return nil, fmt.Errorf("Anthropic tool calls without tool_use stop_reason")
	}
	if resp.StopReason == "tool_use" && len(out.ToolCalls) == 0 {
		return nil, fmt.Errorf("Anthropic tool_use stop_reason without tool calls")
	}
	return out, nil
}

func (c *AnthropicClient) SetModel(model string) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.model = strings.TrimSpace(model)
}

func (c *AnthropicClient) GetModel() string {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.model
}

// SchemaCapable reports whether the configured model takes native JSON
// outputs (config.AnthropicSupportsStructuredOutput).
func (c *AnthropicClient) SchemaCapable() bool {
	return config.AnthropicSupportsStructuredOutput(c.GetModel())
}

func (c *AnthropicClient) ShouldUsePiggybackTools() bool { return false }
func (c *AnthropicClient) ModelIdentity() (string, string) {
	return string(ProviderAnthropic), c.GetModel()
}
