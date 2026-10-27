package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// anthropicMaxPauseResumes bounds how many times one turn is resumed after the
// endpoint paused it (stop_reason pause_turn) while its server tools ran. A
// turn still paused after that many resumptions is refused, not resumed
// forever.
const anthropicMaxPauseResumes = 4

// The client is a grounding provider, read through the production chain
// (ScheduledLLMCall, the tracing client and the broker forward the getters):
// chat articulation and delegation record its sources, campaign planning's
// grounding helper captures them. It takes no run-time grounding control: its
// web tools are on exactly when the slot's config opted in.
var _ types.GroundingProvider = (*AnthropicClient)(nil)

// serverTools are the server web tools a request to model declares: those the
// slot's config opted into (anthropic.web_search, anthropic.web_fetch), in the
// versions model takes (config.AnthropicWebToolVersions), each with its
// per-request cap.
func (c *AnthropicClient) serverTools(model string) []AnthropicTool {
	searchType, fetchType := config.AnthropicWebToolVersions(model)
	var tools []AnthropicTool
	if uses := c.options.WebSearchUses(); uses > 0 {
		tools = append(tools, AnthropicTool{Type: searchType, Name: "web_search", MaxUses: uses})
	}
	if uses := c.options.WebFetchUses(); uses > 0 {
		tools = append(tools, AnthropicTool{Type: fetchType, Name: "web_fetch", MaxUses: uses})
	}
	return tools
}

// exchange sends a non-streaming request with the slot's server web tools
// declared and returns the turn whole. A turn the endpoint paused while its
// server tools ran (pause_turn) is resumed by sending the paused content back
// as the assistant's turn so far, up to anthropicMaxPauseResumes times; the
// returned response carries every part's content in order and the usage of
// every request, each of which was billed. The turn's grounding sources --
// the results its searches and fetches returned and the URLs its text cites
// -- are recorded for GetLastGroundingSources, cleared first so a later call
// never reports a page it did not use.
func (c *AnthropicClient) exchange(ctx context.Context, req AnthropicRequest, plan anthropicCachePlan) (*AnthropicResponse, error) {
	c.recordGroundingSources(nil)
	if server := c.serverTools(req.Model); len(server) > 0 {
		req.Tools = append(append([]AnthropicTool(nil), req.Tools...), server...)
	}
	resp, err := c.postMessages(ctx, req, plan)
	if err != nil {
		return nil, err
	}
	for resumes := 0; resp.StopReason == "pause_turn"; resumes++ {
		if resumes == anthropicMaxPauseResumes {
			return nil, fmt.Errorf("Anthropic paused the turn %d times while its server tools ran; it was not completed", resumes+1)
		}
		next := req
		next.Messages = append(append([]AnthropicMessage(nil), req.Messages...), AnthropicMessage{Role: "assistant", Content: resp.Content})
		more, err := c.postMessages(ctx, next, plan)
		if err != nil {
			return nil, fmt.Errorf("resume the paused Anthropic turn: %w", err)
		}
		merged := *more
		merged.Content = append(append([]AnthropicContentBlock(nil), resp.Content...), more.Content...)
		merged.Usage = resp.Usage.plus(more.Usage)
		resp = &merged
	}
	sources := anthropicGroundingSources(resp.Content)
	c.recordGroundingSources(sources)
	if uses := resp.Usage.ServerToolUse; uses.WebSearchRequests > 0 || uses.WebFetchRequests > 0 {
		// The usage meter counts tokens; the per-use charge of a search
		// ($10 per 1,000) has no slot there, so the count is logged.
		logging.Perception("Anthropic server tools: model=%s web_search_requests=%d web_fetch_requests=%d sources=%d",
			req.Model, uses.WebSearchRequests, uses.WebFetchRequests, len(sources))
	}
	return resp, nil
}

// anthropicGroundingSources are the URLs a turn's server tools returned and
// its text cites, in order, each once.
func anthropicGroundingSources(content []AnthropicContentBlock) []string {
	var out []string
	seen := map[string]bool{}
	add := func(url string) {
		if url != "" && !seen[url] {
			seen[url] = true
			out = append(out, url)
		}
	}
	for _, block := range content {
		switch block.Type {
		case "web_search_tool_result", "web_fetch_tool_result":
			for _, url := range anthropicServerResults(block) {
				add(url)
			}
		case "text":
			for _, citation := range block.Citations {
				add(citation.URL)
			}
		}
	}
	return out
}

// anthropicServerResults are the URLs one server tool result carries. A web
// search result's content is a list of results on success and an error object
// on failure, so the shape is branched on, never indexed into; a fetch
// result's content is one object, a fetched page or an error. An error is
// logged: the model saw it and answers around it.
func anthropicServerResults(block AnthropicContentBlock) []string {
	content := bytes.TrimSpace(block.ServerContent)
	if len(content) == 0 {
		return nil
	}
	type result struct {
		Type      string `json:"type"`
		URL       string `json:"url"`
		ErrorCode string `json:"error_code"`
	}
	var results []result
	switch content[0] {
	case '[':
		if err := json.Unmarshal(content, &results); err != nil {
			logging.PerceptionWarn("Anthropic %s: unreadable result list: %v", block.Type, err)
			return nil
		}
	case '{':
		var one result
		if err := json.Unmarshal(content, &one); err != nil {
			logging.PerceptionWarn("Anthropic %s: unreadable result: %v", block.Type, err)
			return nil
		}
		results = []result{one}
	default:
		return nil
	}
	var urls []string
	for _, r := range results {
		if r.ErrorCode != "" {
			logging.PerceptionWarn("Anthropic %s failed: %s (%s)", block.Type, r.ErrorCode, r.Type)
			continue
		}
		urls = append(urls, r.URL)
	}
	return urls
}

func (c *AnthropicClient) recordGroundingSources(sources []string) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.lastGroundingSources = append([]string(nil), sources...)
}

// GetLastGroundingSources implements types.GroundingProvider: the URLs the
// last non-streaming request's server tools returned and its text cited.
func (c *AnthropicClient) GetLastGroundingSources() []string {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return append([]string(nil), c.lastGroundingSources...)
}

// IsWebSearchEnabled implements types.GroundingProvider: whether the slot's
// config opted into web_search.
func (c *AnthropicClient) IsWebSearchEnabled() bool { return c.options.WebSearch }

// IsURLContextEnabled implements types.GroundingProvider: Anthropic's URL
// grounding is the web_fetch tool, on when the slot's config opted in.
func (c *AnthropicClient) IsURLContextEnabled() bool { return c.options.WebFetch }
