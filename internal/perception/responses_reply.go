package perception

// Responses API reply parsing, shared by every Responses client: answer
// text, tool calls with their reasoning blocks in emitted order, terminal
// status, and grounding sources.

import (
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// responsesTextFromReply is the answer text of a reply: every non-commentary
// message item, in order, separated by a blank line.
func responsesTextFromReply(reply *responsesReply) string {
	return responsesAnswerText(reply, false)
}

// responsesAnswerText joins the output_text of the reply's answer messages.
// Commentary-phase messages are progress notes between searches, not the
// answer: concatenated in, they run into the answer mid-sentence. Separate
// answer messages are kept whole and separated, never glued together.
func responsesAnswerText(reply *responsesReply, assistantOnly bool) string {
	if reply == nil {
		return ""
	}
	var parts []string
	for _, item := range reply.Output {
		if item.Type != "message" || item.Phase == "commentary" {
			continue
		}
		if assistantOnly && item.Role != "" && item.Role != "assistant" {
			continue
		}
		var sb strings.Builder
		for _, block := range item.Content {
			if block.Type == "output_text" && block.Text != "" {
				sb.WriteString(block.Text)
			}
		}
		if sb.Len() > 0 {
			parts = append(parts, sb.String())
		}
	}
	return strings.Join(parts, "\n\n")
}

// responsesTerminalFailure reports whether a Responses status is a terminal
// failure: the vendor produced nothing usable, so surfacing an error is the
// only honest outcome. Anything else falls through to normal parsing.
func responsesTerminalFailure(status string) bool {
	return status == "failed" || status == "cancelled"
}

// shareReasoningTokens gives each reasoning block of a reply its share of the
// reply's reasoning tokens, by ciphertext length; the last takes what is left,
// so the shares add up to the count. The provider counts reasoning once per
// reply, and the next turn replays each item: the broker measures a replayed
// item by its share, because the length of its encrypted_content is no
// measure of what replaying it costs (ladder run R1-4c).
func shareReasoningTokens(blocks []types.ContentBlock, total int) {
	if total <= 0 {
		return
	}
	var idx []int
	chars := 0
	for i, b := range blocks {
		if b.Kind == types.BlockThinking {
			idx = append(idx, i)
			chars += len(b.Signature)
		}
	}
	given := 0
	for k, i := range idx {
		share := total - given
		if k < len(idx)-1 {
			if chars > 0 {
				share = total * len(blocks[i].Signature) / chars
			} else {
				share = total / len(idx)
			}
		}
		blocks[i].Tokens = share
		given += share
	}
}

// responsesToolResponseFromReply converts a Responses reply into codeNERD's
// vendor-neutral tool response.
//
// The output array is ordered, so Blocks records the real order the model
// emitted — reasoning, text and calls interleaved as they came — with each
// reasoning item's encrypted_content kept verbatim as the block signature.
// That is what lets the next turn replay this one in position rather than
// looking it up by history index.
func responsesToolResponseFromReply(reply *responsesReply) *LLMToolResponse {
	out := &LLMToolResponse{Text: responsesTextFromReply(reply), StopReason: "end_turn"}
	for _, item := range reply.Output {
		switch item.Type {
		case "reasoning":
			if item.EncryptedContent == "" {
				continue
			}
			block := types.RedactedThinkingBlock(item.EncryptedContent)
			block.ID = item.ID
			out.Blocks = append(out.Blocks, block)
		case "message":
			for _, c := range item.Content {
				if c.Text != "" {
					out.Blocks = append(out.Blocks, types.TextBlock(c.Text))
				}
			}
		}
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
					"responses: tool %s returned unparseable arguments (%v)", item.Name, err)
				input = map[string]any{}
			}
		}
		out.ToolCalls = append(out.ToolCalls, types.ToolCall{
			ID:    item.CallID,
			Name:  item.Name,
			Input: input,
		})
		out.Blocks = append(out.Blocks, types.ToolUseBlock(item.CallID, item.Name, input))
	}
	// The Chat path normalizes "tool_calls" to "tool_use"; the Responses path
	// never set a stop reason at all, leaving schedulers and logs blind.
	if len(out.ToolCalls) > 0 {
		out.StopReason = "tool_use"
	}
	if reply.Usage != nil {
		out.Usage = reply.Usage.metadata()
		shareReasoningTokens(out.Blocks, reply.Usage.reasoningTokens())
	}
	return out
}

func appendWebSearchInclude(include []string) []string {
	for _, v := range include {
		if v == responsesWebSearchResultsInclude {
			return include
		}
	}
	return append(include, responsesWebSearchResultsInclude)
}

func responsesHTTPURL(raw string) (string, bool) {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "", false
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Host == "" {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	return u, true
}

// responsesGroundingCapture is the citations, retrieved pages, and deduped source
// URLs of one Responses reply. Sources list citation URLs first, then result
// URLs, each URL once.
type responsesGroundingCapture struct {
	Citations []types.GroundedCitation
	Results   []types.GroundedSearchResult
	Sources   []string
}

// responsesGroundingFromReply reads url_citation annotations on assistant message
// text and web_search_call results. Reasoning items are skipped: a URL in
// the reasoning trace is not a source the caller is allowed to see.
func responsesGroundingFromReply(reply *responsesReply) responsesGroundingCapture {
	g := responsesGroundingCapture{
		Citations: []types.GroundedCitation{},
		Results:   []types.GroundedSearchResult{},
	}
	if reply == nil {
		return g
	}
	seen := map[string]struct{}{}
	addSource := func(raw string) {
		u, ok := responsesHTTPURL(raw)
		if !ok {
			return
		}
		if _, dup := seen[u]; dup {
			return
		}
		seen[u] = struct{}{}
		g.Sources = append(g.Sources, u)
	}
	for _, item := range reply.Output {
		if item.Type != "message" {
			continue
		}
		if item.Role != "" && item.Role != "assistant" {
			continue
		}
		for _, content := range item.Content {
			if content.Type != "output_text" {
				continue
			}
			for _, ann := range content.Annotations {
				if ann.Type != "url_citation" {
					continue
				}
				u, ok := responsesHTTPURL(ann.URL)
				if !ok {
					continue
				}
				g.Citations = append(g.Citations, types.GroundedCitation{
					URL:        u,
					Title:      ann.Title,
					StartIndex: ann.StartIndex,
					EndIndex:   ann.EndIndex,
				})
				addSource(u)
			}
		}
	}
	seenResult := map[string]struct{}{}
	for _, item := range reply.Output {
		if item.Type != "web_search_call" {
			continue
		}
		for _, hit := range item.Results {
			u, ok := responsesHTTPURL(hit.URL)
			if !ok {
				continue
			}
			seenResult[u] = struct{}{}
			g.Results = append(g.Results, types.GroundedSearchResult{
				Type:    hit.Type,
				Title:   hit.Title,
				URL:     u,
				Snippet: hit.Snippet,
			})
			addSource(u)
		}
		// OpenAI lists the URLs a search consulted under action.sources
		// (include web_search_call.action.sources). Each becomes a result
		// with no snippet; a URL already listed as a result is not listed
		// twice. A cited URL is still a result: citing a page does not mean
		// it was not retrieved.
		if item.Action != nil {
			for _, src := range item.Action.Sources {
				u, ok := responsesHTTPURL(src.URL)
				if !ok {
					continue
				}
				if _, dup := seenResult[u]; dup {
					continue
				}
				seenResult[u] = struct{}{}
				g.Results = append(g.Results, types.GroundedSearchResult{Type: src.Type, URL: u})
				addSource(u)
			}
		}
	}
	return g
}

// responsesAssistantText is the visible answer. Reasoning and search-call items
// are not text, and a non-assistant message is not the answer.
func responsesAssistantText(reply *responsesReply) string {
	return responsesAnswerText(reply, true)
}

// responsesRefusal is the provider's explicit refusal in a reply: the text of
// every "refusal" content block on an assistant message, joined. A refusal
// arrives on a run whose status is "completed", so status alone never shows
// it; ok is false when the reply carries none.
func responsesRefusal(reply *responsesReply) (text string, ok bool) {
	if reply == nil {
		return "", false
	}
	var parts []string
	for _, item := range reply.Output {
		if item.Type != "message" || (item.Role != "" && item.Role != "assistant") {
			continue
		}
		for _, c := range item.Content {
			if c.Type == "refusal" {
				ok = true
				if t := strings.TrimSpace(c.Refusal); t != "" {
					parts = append(parts, t)
				}
			}
		}
	}
	return strings.Join(parts, "\n"), ok
}

// responsesTerminal reports a run that did not finish as an answer: a failed
// or cancelled status, an incomplete one, or an explicit refusal. A refusal
// is the typed StreamRefused outcome, the same one every other provider
// reports, so no caller mistakes it for an empty answer and the fallback
// client never reroutes it to another model. A length stop is the typed
// truncation error carrying the partial text, so the broker restates the
// request rather than accepting a cut answer. Any other incomplete reason
// (content_filter) is a failure too: an incomplete run is never a success.
func responsesTerminal(reply *responsesReply, provider Provider, model, method string, limit int) error {
	if reply == nil {
		return fmt.Errorf("no completion returned")
	}
	if responsesTerminalFailure(reply.Status) {
		detail := ""
		if reply.Error != nil && strings.TrimSpace(reply.Error.Message) != "" {
			detail = ": " + strings.TrimSpace(reply.Error.Message)
		}
		return fmt.Errorf("%s responses run %s%s", provider, reply.Status, detail)
	}
	if refusal, refused := responsesRefusal(reply); refused {
		detail := "request declined by provider"
		if refusal != "" {
			detail += ": " + refusal
		}
		return &types.StreamError{Outcome: types.StreamRefused, Provider: string(provider), Model: model, Reason: "refusal", Detail: detail}
	}
	if reply.Status != "incomplete" {
		return nil
	}
	reason := ""
	if reply.IncompleteDetails != nil {
		reason = reply.IncompleteDetails.Reason
	}
	if types.LengthStop(reason) {
		produced := 0
		if reply.Usage != nil {
			produced = reply.Usage.outputTokens()
		}
		return outputTruncated(provider, model, method, reason, responsesTextFromReply(reply), limit, produced)
	}
	return fmt.Errorf("%s responses run incomplete (reason=%q)", provider, reason)
}
