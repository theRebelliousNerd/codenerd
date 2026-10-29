package perception

// OpenAI API client — Responses surface (POST /v1/responses).
//
// OpenAI's current models are Responses-first: GPT-6 Astra, GPT-6.1 Sol and
// GPT-6.1 Terra call tools only through Responses, and reasoning only carries
// across turns there. This client therefore sends every request to /responses:
// plain completions, streaming, single-turn tools, the multi-turn tool loop,
// and grounded search.
//
// What rides on each request, all from .nerd/config.json (the "openai" block,
// the slot's reasoning_effort, a shard profile's override on the context):
//
//   - reasoning.effort, and reasoning.mode / context / summary when set;
//   - encrypted reasoning replayed in position on every tool turn (store:false,
//     stateless replay, the same model as the Meta Responses client);
//   - strict json_schema structured output for piggyback envelopes;
//   - prompt caching: a prompt_cache_key per model and tool catalog, plus
//     prompt_cache_options.ttl on GPT-5.6+ or prompt_cache_retention "24h"
//     on GPT-5 models before that;
//   - service_tier, text.verbosity, and the hosted web_search tool.
//
// Capability rules (which efforts a model takes, whether it samples, which
// cache control it uses, its output ceiling) are config.OpenAI* functions, so
// the load-time check and this client decide the same way. None of them
// picks a model.

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// defaultOpenAIOutputCeiling is the max_output_tokens for a model codeNERD
// has no record of. Reasoning counts against the ceiling, and OpenAI advises
// reserving at least 25k for reasoning and output; a ceiling is a cap, not a
// spend.
const defaultOpenAIOutputCeiling = 32768

// OpenAIClient implements LLMClient, ToolResultsProvider, grounding and
// grounded search for the OpenAI API.
type OpenAIClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client

	maxOutputTokens int
	maxInputTokens  int
	reasoningEffort string
	options         config.OpenAIProviderConfig

	mu                   sync.Mutex
	lastRequest          time.Time
	enableWebSearch      bool
	lastGroundingSources []string
}

// OpenAIConfig configures an OpenAI client. It names a model only because the
// workspace's config does; the factory refuses to build a client without one.
type OpenAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
	// MaxOutputTokens is the completion ceiling; zero is the model's maximum
	// (config.OpenAIMaxOutputTokens), else defaultOpenAIOutputCeiling.
	MaxOutputTokens int
	// MaxInputTokens is the slot's input cap; zero is none. A larger request
	// is refused before it is sent.
	MaxInputTokens int
	// ReasoningEffort is the slot's effort, sent to reasoning models.
	ReasoningEffort string
	// Options is the workspace's "openai" block.
	Options config.OpenAIProviderConfig
}

// DefaultOpenAIConfig returns the endpoint and timeout defaults. It names no
// model: the workspace's config does.
func DefaultOpenAIConfig(apiKey string) OpenAIConfig {
	return OpenAIConfig{
		APIKey:  apiKey,
		BaseURL: "https://api.openai.com/v1",
		// Reasoning at high effort over a large context runs for minutes.
		Timeout: 10 * time.Minute,
	}
}

// NewOpenAIClient builds a client, refusing a configuration the API would
// reject on the first request: an effort the model does not accept or an
// option outside the API's vocabulary.
func NewOpenAIClient(cfg OpenAIConfig) (*OpenAIClient, error) {
	if problems := cfg.Options.Check("openai"); len(problems) > 0 {
		return nil, fmt.Errorf("%s", problems[0].String())
	}
	if err := config.ValidateOpenAIEffort(cfg.Model, cfg.ReasoningEffort); err != nil {
		return nil, err
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultOpenAIConfig("").BaseURL
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultOpenAIConfig("").Timeout
	}
	ceiling := cfg.MaxOutputTokens
	if ceiling <= 0 {
		ceiling = config.OpenAIMaxOutputTokens(cfg.Model)
	}
	if ceiling <= 0 {
		ceiling = defaultOpenAIOutputCeiling
	}
	return &OpenAIClient{
		apiKey:          cfg.APIKey,
		baseURL:         cfg.BaseURL,
		model:           cfg.Model,
		httpClient:      NewSharedHTTPClient(cfg.Timeout),
		maxOutputTokens: ceiling,
		maxInputTokens:  cfg.MaxInputTokens,
		reasoningEffort: strings.TrimSpace(cfg.ReasoningEffort),
		options:         cfg.Options,
		enableWebSearch: cfg.Options.WebSearchEnabled(),
	}, nil
}

// SetModel changes the model used for completions.
func (c *OpenAIClient) SetModel(model string) { c.model = model }

// GetModel returns the current model.
func (c *OpenAIClient) GetModel() string { return c.model }

// ModelIdentity satisfies types.ModelIdentifier.
func (c *OpenAIClient) ModelIdentity() (string, string) { return string(ProviderOpenAI), c.model }

// InputWindowTokens reports the slot's input cap (types.InputWindowReporter).
func (c *OpenAIClient) InputWindowTokens() int { return c.maxInputTokens }

// modelFor is the model a request is sent to: a per-call override on the
// context (a shard profile's model), else the client's.
func (c *OpenAIClient) modelFor(ctx context.Context) string {
	if model, ok := types.ModelNameFromContext(ctx); ok && strings.TrimSpace(model) != "" {
		return strings.TrimSpace(model)
	}
	return c.model
}

// effortFor is the effort a request is sent with: a shard profile's on the
// context, else the slot's.
func (c *OpenAIClient) effortFor(ctx context.Context) string {
	if effort, ok := types.ReasoningEffortFromContext(ctx); ok {
		return effort
	}
	return c.reasoningEffort
}

// admit refuses a request before it is sent: a profile effort the target
// model rejects, or input over the slot's cap.
func (c *OpenAIClient) admit(ctx context.Context, system string, messages []types.Message, tools []ToolDefinition) error {
	if c.apiKey == "" {
		return fmt.Errorf("OpenAI API key not configured: set api_key_pools.openai or openai_api_key in .nerd/config.json")
	}
	if effort, ok := types.ReasoningEffortFromContext(ctx); ok {
		if err := config.ValidateOpenAIEffort(c.modelFor(ctx), effort); err != nil {
			return fmt.Errorf("shard profile reasoning_effort %q: %w", effort, err)
		}
	}
	return admitInputWindow(c.maxInputTokens, ProviderOpenAI, c.modelFor(ctx), system, messages, tools, 0)
}

// throttle spaces requests at least 100ms apart.
func (c *OpenAIClient) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elapsed := time.Since(c.lastRequest); elapsed < 100*time.Millisecond {
		time.Sleep(100*time.Millisecond - elapsed)
	}
	c.lastRequest = time.Now()
}

func (c *OpenAIClient) endpoint() responsesEndpoint {
	return responsesEndpoint{httpClient: c.httpClient, baseURL: c.baseURL, apiKey: c.apiKey, provider: ProviderOpenAI}
}

// newRequest builds a request carrying every configured option the target
// model takes. Tools, structured output and the cache key are added by the
// caller (the key hashes the final tool list).
func (c *OpenAIClient) newRequest(ctx context.Context, input []any) responsesRequest {
	model := c.modelFor(ctx)
	store := false
	req := responsesRequest{
		Model: model,
		Input: input,
		// Stateless replay: server-side persistence buys nothing when the
		// executor resends the conversation, and leaves state on the vendor.
		Store:           &store,
		Truncation:      "disabled",
		MaxOutputTokens: c.maxOutputTokens,
		ServiceTier:     strings.TrimSpace(c.options.ServiceTier),
	}

	reasoning := config.OpenAIIsReasoningModel(model)
	effort := c.effortFor(ctx)
	if reasoning {
		req.Include = []string{responsesReasoningInclude}
		r := &responsesReasoning{
			Effort:  effort,
			Mode:    strings.TrimSpace(c.options.ReasoningMode),
			Context: strings.TrimSpace(c.options.ReasoningContext),
		}
		if *r != (responsesReasoning{}) {
			req.Reasoning = r
		}
		if v := strings.TrimSpace(c.options.Verbosity); v != "" {
			req.Text = &responsesText{Verbosity: v}
		}
	}
	// Sampling controls reach only models that take them: a reasoning model
	// rejects temperature unless its effort is "none".
	if !reasoning || effort == "none" {
		t := types.TemperatureFor(ctx, 0.1)
		req.Temperature = &t
	}

	switch {
	case config.OpenAIUsesCacheOptions(model):
		if ttl := strings.TrimSpace(c.options.PromptCacheTTL); ttl != "" {
			req.PromptCacheOptions = &responsesPromptCacheOptions{TTL: ttl}
		}
	case openAIExtendedRetention(model):
		// Free on these models, and evicted far less than in_memory.
		req.PromptCacheRetention = "24h"
	}
	return req
}

// openAIExtendedRetention reports a GPT-5 model before GPT-5.6, which takes
// prompt_cache_retention "24h" at no extra charge.
func openAIExtendedRetention(model string) bool {
	major, _, ok := config.OpenAIModelVersion(model)
	return ok && major >= 5 && !config.OpenAIUsesCacheOptions(model)
}

// withStructuredOutput sets the strict PiggybackEnvelope schema.
func withStructuredOutput(req *responsesRequest) {
	env := BuildOpenAIPiggybackEnvelopeSchema().JSONSchema
	strict := env.Strict
	if req.Text == nil {
		req.Text = &responsesText{}
	}
	req.Text.Format = &responsesTextFormat{Type: "json_schema", Name: env.Name, Schema: env.Schema, Strict: &strict}
}

// finalize attaches search when on and computes the cache key over the final
// tool list.
func (c *OpenAIClient) finalize(req *responsesRequest, force bool) {
	if force || c.IsWebSearchEnabled() {
		req.Tools = append(req.Tools, c.webSearchTool())
		req.Include = appendInclude(req.Include, responsesWebSearchSourcesInclude)
	}
	req.PromptCacheKey = promptCacheKey(req.Model, req.Tools)
}

// execute posts one request and reports its usage under op.
func (c *OpenAIClient) execute(ctx context.Context, req responsesRequest, op string) (*responsesReply, error) {
	c.throttle()
	start := time.Now()
	reply, err := postResponses(ctx, c.endpoint(), req)
	if err != nil {
		return nil, err
	}
	if reply.Usage != nil {
		reportUsage(ctx, req.Model, ProviderOpenAI, op, reply.Usage.metadata())
		logResponsesUsage(ProviderOpenAI, reply, start)
	}
	return reply, nil
}

// executeStructured posts a structured-output request; a model that refuses
// the schema gets the request again without it, as the Chat client did.
func (c *OpenAIClient) executeStructured(ctx context.Context, req responsesRequest, op string) (*responsesReply, error) {
	reply, err := c.execute(ctx, req, op)
	if err == nil || req.Text == nil || req.Text.Format == nil || !isSchemaRejection(err.Error()) {
		return reply, err
	}
	logging.PerceptionWarn("[openai] structured output rejected (%v); retrying without text.format", err)
	req.Text.Format = nil
	return c.execute(ctx, req, op)
}

// withDeadline applies the client timeout to a context that has none.
func (c *OpenAIClient) withDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.httpClient.Timeout)
}

// Complete sends a prompt and returns the completion.
func (c *OpenAIClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a system prompt and a user prompt.
func (c *OpenAIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if err := c.admit(ctx, systemPrompt, userTurn(userPrompt), nil); err != nil {
		return "", err
	}
	ctx, cancel := c.withDeadline(ctx)
	defer cancel()
	start := time.Now()
	c.clearGroundingSources()

	req := c.newRequest(ctx, responsesInputFromHistory(systemPrompt, userTurn(userPrompt)))
	if isPiggybackPrompt(ctx, systemPrompt, userPrompt) {
		withStructuredOutput(&req)
	}
	c.finalize(&req, false)

	reply, err := c.executeStructured(ctx, req, usageOpChat)
	if err != nil {
		logging.PerceptionError("[openai] CompleteWithSystem failed after %v: %v", time.Since(start), err)
		return "", err
	}
	c.recordGroundingSources(responsesGroundingFromReply(reply).Sources)
	if err := responsesTerminal(reply, ProviderOpenAI, req.Model, "CompleteWithSystem", c.maxOutputTokens); err != nil {
		return "", err
	}
	out := strings.TrimSpace(responsesTextFromReply(reply))
	if out == "" && !allowEmptyCompletion(ctx) {
		return "", fmt.Errorf("openai returned an empty completion (model=%s status=%q output_tokens=%d reasoning_tokens=%d)",
			req.Model, reply.Status, reply.Usage.outputTokens(), reply.Usage.reasoningTokens())
	}
	logging.Perception("[openai] CompleteWithSystem: model=%s completed in %v response_len=%d tier=%s",
		req.Model, time.Since(start), len(out), reply.ServiceTier)
	return out, nil
}

// CompleteWithTools sends a prompt with tool definitions: the first turn of a
// tool loop, built exactly as the later turns are.
func (c *OpenAIClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error) {
	return c.CompleteWithToolResults(ctx, systemPrompt, userTurn(userPrompt), tools)
}

// CompleteWithToolResults continues a tool loop (types.ToolResultsProvider).
// Earlier turns' reasoning is replayed from the history's own thinking blocks,
// in position, so the model keeps the thread of its own prior thinking.
func (c *OpenAIClient) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []ToolDefinition) (*LLMToolResponse, error) {
	if err := c.admit(ctx, systemPrompt, history, tools); err != nil {
		return nil, err
	}
	if err := rejectHostedToolNames(tools); err != nil {
		return nil, err
	}
	ctx, cancel := c.withDeadline(ctx)
	defer cancel()
	c.clearGroundingSources()

	req := c.newRequest(ctx, responsesInputFromHistory(systemPrompt, history))
	req.Tools = responsesToolsFromDefinitions(tools)
	if len(req.Tools) > 0 {
		req.ToolChoice = "auto"
	}
	c.finalize(&req, false)

	reply, err := c.execute(ctx, req, usageOpFor(len(tools)))
	if err != nil {
		return nil, err
	}
	c.recordGroundingSources(responsesGroundingFromReply(reply).Sources)
	if err := responsesTerminal(reply, ProviderOpenAI, req.Model, "CompleteWithToolResults", c.maxOutputTokens); err != nil {
		return nil, err
	}
	return responsesToolResponseFromReply(reply), nil
}

// rejectHostedToolNames refuses a function tool named like a hosted tool the
// client may attach, which the API would reject as a duplicate.
func rejectHostedToolNames(tools []ToolDefinition) error {
	for _, t := range tools {
		if t.Name == "web_search" {
			return fmt.Errorf("tool name %q collides with OpenAI's hosted web_search tool", t.Name)
		}
	}
	return nil
}

// appendInclude adds v to include once.
func appendInclude(include []string, v string) []string {
	for _, have := range include {
		if have == v {
			return include
		}
	}
	return append(include, v)
}
