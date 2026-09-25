package session

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// piggybackChannel is the tool-result continuation for a client that carries
// its tool calls in the Piggyback envelope rather than in native tool_use
// blocks: the two CLI engines, which have no tool channel at all, and Gemini
// while grounding is on.
//
// It lets those clients run the same tool loop the native path runs -- the
// working context and its ledger, the working policy's stops and finalize, the
// forced final answer, and every post-edit repair round -- instead of a path of
// their own. Until 2026-09-25 the Piggyback path executed the first batch of
// tool requests and returned: the model never saw a result, so a turn that had
// to read before it could write ended after the read, and the repair rounds
// had no channel to ask for a fix. The protocol atom (capability/tool_thinking,
// "Tool results are automatically injected into your context on the next
// turn") had been promising a continuation the harness did not make.
//
// A native provider gets the conversation as typed messages. This client gets
// one system prompt and one user prompt, so the request is rendered: the
// catalog of the tools the request offers joins the system prompt, and the
// conversation -- prior turns, the anchor, every call and result the ledger
// carries -- becomes the user prompt. The model's envelope comes back as text
// and is promoted to tool calls here, so the loop sees the same response shape
// a native provider returns.
type piggybackChannel struct {
	e      *Executor
	client types.LLMClient
	cfg    *config.EffectiveAgentRuntimeConfig
}

// toolResultsChannel is the continuation a turn's tool loop uses for client:
// the Piggyback channel for an envelope client, else the client's own native
// channel when it has one. The Piggyback answer comes first on purpose: the
// session adapter always claims ToolResultsProvider (it forwards or errors),
// so for a CLI engine the native claim is the one that would fail.
func (e *Executor) toolResultsChannel(client types.LLMClient, cfg *config.EffectiveAgentRuntimeConfig) (types.ToolResultsProvider, bool) {
	if client == nil {
		return nil, false
	}
	if usesPiggybackTools(client) {
		return &piggybackChannel{e: e, client: client, cfg: cfg}, true
	}
	trp, ok := client.(types.ToolResultsProvider)
	return trp, ok
}

// CompleteWithToolResults renders the request, sends it on the client's text
// channel, and promotes the envelope's tool_requests to tool calls.
func (p *piggybackChannel) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, definitions []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if p == nil || p.client == nil {
		return nil, fmt.Errorf("piggyback channel has no client")
	}
	// The catalog is the request's: the definitions the working context
	// offers this round, which the commit regime and the structural-first
	// catalog have already narrowed. An empty offer (a forced final answer on
	// a read-only intent) renders no catalog at all.
	if catalog := p.e.piggybackCatalog(p.cfg, definitions); catalog != "" {
		systemPrompt += "\n\n" + catalog
	}
	text, err := p.client.CompleteWithSystem(ctx, systemPrompt, renderPiggybackConversation(history))
	if err != nil {
		return nil, err
	}
	resp := &types.LLMToolResponse{Text: text, StopReason: "end_turn"}
	if p.e.promotePiggybackToolRequests(resp) {
		if loop := activeWorkingLoop(ctx); loop != nil {
			if calls, renamed := loop.claimCallIDs(resp.ToolCalls); renamed {
				resp.Rewrite(resp.Text, calls)
			}
			// This round calls tools, so its surface is not the turn's last
			// word and will not be shown. A notice that the constitution
			// blocked one of its mangle_updates must reach the user anyway.
			if notice := safetyNoticeOf(resp.Text); notice != "" && loop.result != nil {
				loop.result.safetyNotices = append(loop.result.safetyNotices, notice)
			}
		}
		logging.Session("Piggyback round: %d tool_request(s) promoted to tool calls", len(resp.ToolCalls))
	}
	return resp, nil
}

// safetyNoticePrefix opens the line a constitutional override puts in front
// of an envelope's surface (articulation.ApplyConstitutionalOverride).
const safetyNoticePrefix = "[SAFETY NOTICE:"

// safetyNoticeOf returns the override notice a surface opens with, or "".
func safetyNoticeOf(surface string) string {
	if !strings.HasPrefix(surface, safetyNoticePrefix) {
		return ""
	}
	line, _, _ := strings.Cut(surface, "\n")
	return strings.TrimSpace(line)
}

// withSafetyNotices puts every carried notice the response does not already
// state in front of it, once each, in the order they were raised.
func withSafetyNotices(response string, notices []string) string {
	var head []string
	for _, notice := range notices {
		if notice == "" || strings.Contains(response, notice) || slices.Contains(head, notice) {
			continue
		}
		head = append(head, notice)
	}
	if len(head) == 0 {
		return response
	}
	return strings.Join(head, "\n") + "\n\n" + response
}

// claimCallIDs makes every call's ID unique within the working loop. A native
// provider mints the ids; an envelope carries whatever the model wrote, and the
// protocol's own example is "req_1" -- which a model writes again every round.
// The loop keys observations, ledger evictions and result pairing by call ID,
// so a repeated id would pair a round's result to an earlier round's call and
// evict both together. A model id that is new is kept, so the transcript it
// reads back uses the name it chose; an empty or repeated one is replaced.
func (loop *workingLoop) claimCallIDs(calls []types.ToolCall) ([]types.ToolCall, bool) {
	if loop.callIDs == nil {
		loop.callIDs = make(map[string]bool)
	}
	out := make([]types.ToolCall, len(calls))
	renamed := false
	for i, call := range calls {
		id := strings.TrimSpace(call.ID)
		if id == "" || loop.callIDs[id] {
			for n := len(loop.callIDs) + 1; ; n++ {
				candidate := fmt.Sprintf("pb_%d", n)
				if !loop.callIDs[candidate] {
					id = candidate
					break
				}
			}
		}
		if id != call.ID {
			renamed = true
		}
		loop.callIDs[id] = true
		call.ID = id
		out[i] = call
	}
	return out, renamed
}

// piggybackCatalog renders the tool catalog for the tools this request offers:
// the persona's allowlist narrowed to the offered definitions. Rendering from
// the allowlist rather than straight from the definitions keeps one renderer
// and one set of words for both the single-shot and the loop path.
func (e *Executor) piggybackCatalog(cfg *config.EffectiveAgentRuntimeConfig, offered []types.ToolDefinition) string {
	if len(offered) == 0 || cfg == nil {
		return ""
	}
	names := make(map[string]bool, len(offered))
	for _, def := range offered {
		names[def.Name] = true
	}
	narrowed := *cfg
	narrowed.AllowedTools = make([]string, 0, len(cfg.AllowedTools))
	for _, name := range cfg.AllowedTools {
		if names[name] {
			narrowed.AllowedTools = append(narrowed.AllowedTools, name)
		}
	}
	return e.buildToolCatalogForPiggyback(&narrowed)
}

// renderPiggybackConversation renders a working request's messages as the one
// user prompt an envelope client receives, in order: each turn's prose under
// its role, each tool request with the id the model will see its result under,
// and each result with that id and whether the tool failed. Reasoning blocks
// are dropped rather than folded into the prose: this channel has no field to
// replay them in, and reasoning rendered as text would read as something the
// model said (the rule BlockFidelity holds every lossy surface to).
func renderPiggybackConversation(messages []types.Message) string {
	var b strings.Builder
	b.WriteString("Conversation so far:\n")
	for _, m := range messages {
		role := m.Role
		if role == "" {
			role = "user"
		}
		for _, block := range m.Content() {
			switch block.Kind {
			case types.BlockText:
				if strings.TrimSpace(block.Text) == "" {
					continue
				}
				fmt.Fprintf(&b, "%s: %s\n", role, block.Text)
			case types.BlockToolUse:
				args, err := json.Marshal(block.Input)
				if err != nil {
					args = []byte("{}")
				}
				fmt.Fprintf(&b, "%s tool_request id=%s tool=%s args=%s\n", role, block.ID, block.Name, args)
			case types.BlockToolResult:
				status := "ok"
				if block.IsError {
					status = "error"
				}
				fmt.Fprintf(&b, "tool_result id=%s status=%s:\n%s\n", block.ToolUseID, status, block.Text)
			}
		}
	}
	return b.String()
}
