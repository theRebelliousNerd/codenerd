package broker

import (
	"bytes"
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

const (
	// anthropicCountPath is the provider's token counting endpoint. It is
	// stateless, is not billed as inference, and returns a count for the exact
	// model requested — token counts are model-specific, so the model passed
	// here must be the model that will serve the request.
	anthropicCountPath = "/messages/count_tokens"

	// anthropicCountTimeout bounds the pre-flight count.
	//
	// This is a latency guard, not a correctness one. The count happens on the
	// hot path of a user-visible turn; an exact number that arrives ten seconds
	// late is worse than a calibrated estimate that arrives now. On timeout the
	// counter degrades to the estimator and says so in the Confidence, and
	// admission policy decides whether that is acceptable.
	anthropicCountTimeout = 5 * time.Second

	// countCacheSize bounds the memoized counts. System prompts and tool
	// schemas repeat almost verbatim across the turns of a session, so a small
	// cache removes most of the round trips without holding meaningful memory.
	countCacheSize = 512
)

// AnthropicCounter counts tokens using Anthropic's count_tokens endpoint.
//
// Fidelity caveat, stated because it would otherwise be an invisible source of
// error: this builds a counting request that mirrors what the Anthropic client
// sends, but the two are assembled by different code. Where they diverge the
// count is slightly off — still exact for the body submitted, and still orders
// of magnitude closer than dividing characters by four. If the client's request
// shape changes, this must change with it; the parity test in
// counter_anthropic_test.go exists to make that break loudly.
type AnthropicCounter struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	fallback   *EstimatingCounter

	mu    sync.Mutex
	cache map[string]*list.Element
	order *list.List
}

type countCacheEntry struct {
	key   string
	count Count
}

// NewAnthropicCounter builds a counter against the given credentials. The
// fallback is used when the endpoint is unreachable, slow, or unauthorized, and
// its lower Confidence propagates to the caller rather than being hidden.
func NewAnthropicCounter(apiKey, baseURL string, httpClient *http.Client, fallback *EstimatingCounter) *AnthropicCounter {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: anthropicCountTimeout}
	}
	if fallback == nil {
		fallback = NewEstimatingCounter(nil)
	}
	return &AnthropicCounter{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: httpClient,
		fallback:   fallback,
		cache:      make(map[string]*list.Element, countCacheSize),
		order:      list.New(),
	}
}

// Count implements Counter.
func (a *AnthropicCounter) Count(ctx context.Context, req *Request) (Count, error) {
	if req == nil {
		return Count{}, errNilRequest
	}
	if a.apiKey == "" {
		// No credential means no endpoint. Degrade rather than fail: an
		// estimate that says it is an estimate keeps the turn moving, and a
		// caller that genuinely needs exactness sets RequireExact and is
		// refused at admission instead.
		return a.fallback.Count(ctx, req)
	}

	body, err := a.buildCountBody(req)
	if err != nil {
		logging.Get(logging.CategoryAPI).Warn("broker: count_tokens body build failed, estimating: %v", err)
		return a.fallback.Count(ctx, req)
	}

	key := cacheKey(body)
	if cached, ok := a.lookup(key); ok {
		return cached, nil
	}

	total, err := a.postCount(ctx, body)
	if err != nil {
		logging.Get(logging.CategoryAPI).Debug("broker: count_tokens unavailable, estimating: %v", err)
		return a.fallback.Count(ctx, req)
	}

	count := Count{
		Tokens:     total,
		Segments:   splitProportional(total, measure(req)),
		Confidence: ConfidenceExact,
		Source:     "anthropic.count_tokens",
		Model:      req.Model,
	}
	a.store(key, count)
	return count, nil
}

// Observe forwards observations to the fallback estimator.
//
// The exact path does not need calibration, but the fallback does, and the
// fallback is what serves every request made while the endpoint is unreachable.
// Keeping it trained during the good times is what makes it useful during the
// bad ones.
func (a *AnthropicCounter) Observe(obs Observation) { a.fallback.Observe(obs) }

// Fallback exposes the estimator, for observability and tests.
func (a *AnthropicCounter) Fallback() *EstimatingCounter { return a.fallback }

func (a *AnthropicCounter) postCount(ctx context.Context, body []byte) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, anthropicCountTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+anthropicCountPath, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("build count request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("count request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return 0, fmt.Errorf("read count response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("count endpoint status %d: %s", resp.StatusCode, truncateForLog(payload))
	}

	var decoded struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return 0, fmt.Errorf("decode count response: %w", err)
	}
	if decoded.InputTokens <= 0 {
		return 0, fmt.Errorf("count endpoint returned %d input tokens", decoded.InputTokens)
	}
	return decoded.InputTokens, nil
}

// anthropicCountRequest mirrors the subset of the Messages API shape that
// affects token count.
type anthropicCountRequest struct {
	Model    string                  `json:"model"`
	System   string                  `json:"system,omitempty"`
	Messages []anthropicCountMessage `json:"messages"`
	Tools    []anthropicCountTool    `json:"tools,omitempty"`
}

type anthropicCountMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicCountTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

func (a *AnthropicCounter) buildCountBody(req *Request) ([]byte, error) {
	out := anthropicCountRequest{Model: req.Model, System: req.System}

	for i := range req.Tools {
		tool := &req.Tools[i]
		schema := tool.InputSchema
		if schema == nil {
			// The endpoint rejects a tool with no schema. An empty object
			// preserves the tool's contribution to the count without inventing
			// parameters that are not there.
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out.Tools = append(out.Tools, anthropicCountTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schema,
		})
	}

	for i := range req.Messages {
		msg := &req.Messages[i]
		converted, ok := convertCountMessage(msg)
		if ok {
			out.Messages = append(out.Messages, converted)
		}
	}

	if req.User != "" {
		out.Messages = append(out.Messages, anthropicCountMessage{Role: "user", Content: req.User})
	}

	// The endpoint requires at least one message, and requires the first to be
	// a user turn. A count request that violates either is rejected outright,
	// which would silently push every such call onto the estimator.
	if len(out.Messages) == 0 {
		return nil, fmt.Errorf("no countable messages")
	}
	if out.Messages[0].Role != "user" {
		out.Messages = append([]anthropicCountMessage{{Role: "user", Content: "."}}, out.Messages...)
	}

	return json.Marshal(out)
}

// convertCountMessage renders one internal message as Anthropic content blocks.
// It returns false for a message with nothing countable in it, so an empty turn
// does not become an empty content array the endpoint will reject.
//
// It walks the message's ordered blocks so that what is counted is what the
// adapter will actually send — thinking blocks and their signatures included.
// A count taken from the flat fields would omit replayed reasoning entirely and
// report a request materially smaller than the one that gets billed.
func convertCountMessage(msg *types.Message) (anthropicCountMessage, bool) {
	role := msg.Role
	if role != "assistant" {
		role = "user"
	}

	content := msg.Content()
	if len(content) == 0 {
		return anthropicCountMessage{}, false
	}
	if len(content) == 1 && content[0].Kind == types.BlockText {
		if content[0].Text == "" {
			return anthropicCountMessage{}, false
		}
		return anthropicCountMessage{Role: role, Content: content[0].Text}, true
	}

	blocks := make([]map[string]any, 0, len(content))
	for _, b := range content {
		switch b.Kind {
		case types.BlockText:
			if b.Text == "" {
				continue
			}
			blocks = append(blocks, map[string]any{"type": "text", "text": b.Text})
		case types.BlockThinking:
			if b.Signature == "" {
				continue
			}
			if b.Redacted {
				blocks = append(blocks, map[string]any{"type": "redacted_thinking", "data": b.Signature})
				continue
			}
			blocks = append(blocks, map[string]any{
				"type": "thinking", "thinking": b.Text, "signature": b.Signature,
			})
		case types.BlockToolUse:
			input := b.Input
			if input == nil {
				input = map[string]any{}
			}
			blocks = append(blocks, map[string]any{
				"type": "tool_use", "id": b.ID, "name": b.Name, "input": input,
			})
		case types.BlockToolResult:
			blocks = append(blocks, map[string]any{
				"type": "tool_result", "tool_use_id": b.ToolUseID,
				"content": b.Text, "is_error": b.IsError,
			})
		}
	}
	if len(blocks) == 0 {
		return anthropicCountMessage{}, false
	}
	return anthropicCountMessage{Role: role, Content: blocks}, true
}

func cacheKey(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func (a *AnthropicCounter) lookup(key string) (Count, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	elem, ok := a.cache[key]
	if !ok {
		return Count{}, false
	}
	a.order.MoveToFront(elem)
	return elem.Value.(*countCacheEntry).count, true
}

func (a *AnthropicCounter) store(key string, count Count) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if elem, ok := a.cache[key]; ok {
		elem.Value.(*countCacheEntry).count = count
		a.order.MoveToFront(elem)
		return
	}

	elem := a.order.PushFront(&countCacheEntry{key: key, count: count})
	a.cache[key] = elem

	for a.order.Len() > countCacheSize {
		oldest := a.order.Back()
		if oldest == nil {
			break
		}
		a.order.Remove(oldest)
		delete(a.cache, oldest.Value.(*countCacheEntry).key)
	}
}

func truncateForLog(b []byte) string {
	const max = 200
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}

var _ CalibratingCounter = (*AnthropicCounter)(nil)
