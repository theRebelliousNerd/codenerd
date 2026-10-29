package perception

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/types"
)

// OpenAI hosted web search: the web_search tool on the Responses surface.
// openai.enable_web_search attaches it to every completion and tool turn;
// GroundedWebSearch always attaches it, since that call is a search.
var (
	_ types.GroundedWebSearcher = (*OpenAIClient)(nil)
	_ types.GroundingController = (*OpenAIClient)(nil)
	_ types.GroundingCapable    = (*OpenAIClient)(nil)
)

// webSearchTool is the hosted tool with the configured context size.
func (c *OpenAIClient) webSearchTool() responsesTool {
	return responsesTool{Type: "web_search", SearchContextSize: strings.TrimSpace(c.options.SearchContextSize)}
}

// SupportsGrounding reports the capability: every OpenAI client can search.
func (c *OpenAIClient) SupportsGrounding() bool { return c != nil }

// SetEnableWebSearch implements types.GroundingController.
func (c *OpenAIClient) SetEnableWebSearch(enable bool) {
	c.mu.Lock()
	c.enableWebSearch = enable
	c.mu.Unlock()
}

// IsWebSearchEnabled implements types.GroundingProvider.
func (c *OpenAIClient) IsWebSearchEnabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enableWebSearch
}

// SetEnableURLContext is a no-op: OpenAI has no URL-context tool.
func (c *OpenAIClient) SetEnableURLContext(bool) {}

// SetURLContextURLs is a no-op. See SetEnableURLContext.
func (c *OpenAIClient) SetURLContextURLs([]string) {}

// IsURLContextEnabled is always false.
func (c *OpenAIClient) IsURLContextEnabled() bool { return false }

// GetLastGroundingSources returns a copy of the URLs the previous completion
// cited or consulted.
func (c *OpenAIClient) GetLastGroundingSources() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.lastGroundingSources) == 0 {
		return nil
	}
	return append([]string(nil), c.lastGroundingSources...)
}

func (c *OpenAIClient) clearGroundingSources() { c.recordGroundingSources(nil) }

func (c *OpenAIClient) recordGroundingSources(sources []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(sources) == 0 {
		c.lastGroundingSources = nil
		return
	}
	c.lastGroundingSources = append([]string(nil), sources...)
}

// SupportsGroundedWebSearch implements types.GroundedWebSearcher.
func (c *OpenAIClient) SupportsGroundedWebSearch() bool { return c != nil }

// GroundedWebSearch answers a query with a web-grounded completion, with the
// citations and consulted pages. It uses the slot's effort and options like
// any other request, and the sanitized transport Meta's search uses.
func (c *OpenAIClient) GroundedWebSearch(ctx context.Context, query string) (*types.GroundedWebSearchResult, error) {
	if c == nil {
		return nil, fmt.Errorf("grounded search: client is nil")
	}
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, fmt.Errorf("grounded web search query must not be blank")
	}
	if len(trimmed) > maxGroundedQueryChars {
		return nil, fmt.Errorf("grounded web search query exceeds %d characters", maxGroundedQueryChars)
	}
	if err := c.admit(ctx, "", userTurn(trimmed), nil); err != nil {
		return nil, err
	}
	c.clearGroundingSources()
	timeout := c.httpClient.Timeout
	if timeout <= 0 || timeout > maxGroundedTimeout {
		timeout = maxGroundedTimeout
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	req := c.newRequest(ctx, []any{responsesInputText("user", trimmed)})
	c.finalize(&req, true)

	reply, err := postGroundedSearch(ctx, c.endpoint(), req, timeout, c.throttle)
	if err != nil {
		return nil, err
	}
	reportUsage(ctx, req.Model, ProviderOpenAI, usageOpSearch, reply.Usage.metadata())
	// Status only: the vendor's error message never reaches a search error.
	if reply.Status != "" && reply.Status != "completed" {
		return nil, fmt.Errorf("openai grounded search: run %s", reply.Status)
	}
	result, sources, err := groundedResultFromReply(ProviderOpenAI, reply)
	if err != nil {
		return nil, err
	}
	c.recordGroundingSources(sources)
	return result, nil
}
