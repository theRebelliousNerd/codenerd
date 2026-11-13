package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"codenerd/internal/broker"
	working "codenerd/internal/context"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// What a working request is made of, measured per request (GAP-SESSION-
// REPAIR-COST-01). Live 2026-11-12 one self-repair fix turn sent 1,631,694
// input tokens over 17 model calls -- about 96k a call -- with three repair
// attempts, and nothing recorded which part of each request the tokens went
// to. Every working request now logs its parts at INFO and hands them to the
// broker, whose receipt (.nerd/meter/receipts.jsonl) records them beside the
// provider's own token count.

// repairPayloadKind is the working-record kind of a repair round's prompt:
// the failing output, the tests and the diff one attempt was sent. One kind
// for every repair round, so the policy's working_superseded makes each
// payload superseded by the next (same entity, revision and kind, later step).
const repairPayloadKind = "repair_payload"

// retiredPayloadPrefix opens the pointer a retired repair payload reads as.
const retiredPayloadPrefix = "Repair payload retired:"

// retiredPayloadHandle is what a repair round's prompt reads as once a later
// round's supersedes it: its size and its recall handle, in the pipeline's
// marker, byte-identical from one request to the next.
func retiredPayloadHandle(text, handle string) string {
	return fmt.Sprintf("%s [harness: an earlier repair attempt's prompt -- the failing output, tests and diff it was sent -- stood here; the latest attempt's prompt below supersedes it and summarizes every attempt before it.] %s recall_context id=%q returns it from offset 0, or in offset/limit pages.",
		retiredPayloadPrefix,
		types.DroppedNotice(len(text), len(text), "chars of this repair prompt; a later attempt's prompt supersedes it", ""),
		handle)
}

// payloadCall is the ledger row of a repair payload's storage id: a payload
// rides a user turn, not a tool result, so it has no tool call of its own.
func payloadCall(id string) string { return "payload:" + id }

func payloadDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// notePayload records a repair round's prompt as a working observation before
// the round sends it: recallable by its handle like any archived result, and a
// row of the context ledger, which the policy retires once a later payload
// supersedes it (working_retire). The prompt is known by its text, which the
// round appends to its history as one user turn.
func (loop *workingLoop) notePayload(ctx context.Context, text string) error {
	if loop == nil || loop.set == nil || text == "" {
		return nil
	}
	digest := payloadDigest(text)
	if _, ok := loop.payloads[digest]; ok {
		return nil
	}
	step := time.Now().UnixNano()
	revision := loop.set.Revision("")
	idSum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d", repairPayloadKind, digest, step)))
	id := hex.EncodeToString(idSum[:])
	record := working.WorkingRecord{ID: id, Entity: "", Revision: revision, Kind: repairPayloadKind, Step: step, Body: text}
	if err := loop.set.Save(ctx, record); err != nil {
		return fmt.Errorf("persist repair payload: %w", err)
	}
	handle, err := loop.set.Handle(ctx, id)
	if err != nil {
		return fmt.Errorf("repair payload handle: %w", err)
	}
	if loop.payloads == nil {
		loop.payloads = map[string]string{}
	}
	if loop.handles == nil {
		loop.handles = map[string]string{}
	}
	loop.payloads[digest] = id
	loop.handles[id] = handle
	return nil
}

// payloadOf is the storage id of the repair payload a user turn's text is,
// or "".
func (loop *workingLoop) payloadOf(m types.Message) string {
	if loop == nil || len(loop.payloads) == 0 || m.Role != "user" || m.Text == "" || len(m.ToolResults) > 0 {
		return ""
	}
	return loop.payloads[payloadDigest(m.Text)]
}

// composition measures an assembled working request by part, in bytes.
// messages is what prepareWorkingRequest returned for history: loop.prior,
// the anchor, then history from the ledger's start, in order.
func (loop *workingLoop) composition(system string, history, messages []types.Message, definitions []types.ToolDefinition) broker.Composition {
	c := broker.Composition{System: len(system)}
	if len(definitions) > 0 {
		if raw, err := json.Marshal(definitions); err == nil {
			c.Catalog = len(raw)
		}
	}
	if loop == nil {
		for _, m := range messages {
			text, results, reasoning, calls := messageBytes(m)
			c.Prior += text + results + reasoning + calls
		}
		return c
	}
	views := map[string]int{}
	for _, s := range loop.snapshots {
		if !s.collapsed {
			views[s.key] += len(s.text)
		}
	}
	// harness splits the text the ledger appended to the message at key into
	// views and notes, and returns what is left of the message's own text.
	harness := func(key string, text int) int {
		appended := len(loop.appended[key])
		if appended == 0 {
			return text
		}
		v := min(views[key], appended)
		c.Views += v
		c.Notes += appended - v
		return max(text-appended, 0)
	}
	start := ledgerStart(history, loop.task)
	anchor := len(loop.prior)
	for i, m := range messages {
		text, results, reasoning, calls := messageBytes(m)
		switch {
		case i < anchor:
			c.Prior += text + results + reasoning + calls
		case i == anchor:
			c.Task += harness(anchorLedgerKey, text) + results
		default:
			h := start + i - anchor - 1
			if m.Role == "assistant" {
				c.Assistant += text + calls
				c.Reasoning += reasoning
				c.ToolResults += results
				continue
			}
			c.ToolResults += results
			own := harness(ledgerKey(h), text)
			if h >= 0 && h < len(history) {
				if id := loop.payloadOf(history[h]); id != "" {
					c.RepairPayload += own
					if loop.evicted[payloadCall(id)] {
						c.RetiredPayloads++
					} else {
						c.RepairPayloads++
					}
					continue
				}
			}
			c.Notes += own
		}
	}
	return c
}

// messageBytes is a message's bytes by block kind: text, tool results,
// replayed reasoning (its text and signature), and tool calls (name and
// encoded arguments).
func messageBytes(m types.Message) (text, results, reasoning, calls int) {
	for _, b := range m.Content() {
		switch b.Kind {
		case types.BlockToolResult:
			results += len(b.Text)
		case types.BlockThinking:
			reasoning += len(b.Text) + len(b.Signature)
		case types.BlockToolUse:
			raw, _ := json.Marshal(b.Input)
			calls += len(b.Name) + len(raw)
		case types.BlockServerTool:
			calls += len(b.Raw)
		default:
			text += len(b.Text)
		}
	}
	return text, results, reasoning, calls
}

// logRequestComposition writes one working request's parts and, once it
// returned, what the provider counted, at INFO in the session log.
func logRequestComposition(ctx context.Context, c broker.Composition, resp *types.LLMToolResponse, err error) {
	phase := string(broker.PhaseFromContext(ctx))
	if phase == "" {
		phase = "round"
	}
	counted := "provider usage: none"
	switch {
	case err != nil:
		counted = "provider usage: none (the call failed)"
	case resp != nil:
		counted = fmt.Sprintf("provider usage: input_tokens=%d cached_tokens=%d output_tokens=%d", resp.Usage.InputTokens, resp.Usage.CachedContentTokens, resp.Usage.OutputTokens)
	}
	logging.Get(logging.CategorySession).Info("Working request (%s): %s; %s", phase, c, counted)
}
