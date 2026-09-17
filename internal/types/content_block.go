package types

import "strings"

// ContentBlockKind names one kind of native content block in a conversation
// turn. The set is the intersection every provider that has native blocks at
// all can express, plus thinking, which only some can — see BlockFidelity in
// internal/perception for who carries what.
type ContentBlockKind string

const (
	// BlockText is ordinary assistant or user prose.
	BlockText ContentBlockKind = "text"
	// BlockThinking is the model's reasoning, carrying the provider's
	// verbatim attestation in Signature. Anthropic requires the signature be
	// replayed byte-for-byte and in position or extended thinking degrades;
	// Gemini requires the same of a part's thoughtSignature; the OpenAI
	// Responses surface requires the reasoning item's encrypted_content.
	BlockThinking ContentBlockKind = "thinking"
	// BlockToolUse is a tool invocation the model requested, identified by ID.
	BlockToolUse ContentBlockKind = "tool_use"
	// BlockToolResult is the output of one tool invocation, paired to its
	// BlockToolUse by ToolUseID.
	BlockToolResult ContentBlockKind = "tool_result"
)

// ContentBlock is one ordered, typed piece of a conversation turn.
//
// The ordering between blocks is the point. A model that says "I'll check two
// things", calls tool A, says more, then calls tool B produces five blocks in
// that order, and replaying them in any other order changes what the model
// sees on its next call — which is also the prefix the provider's cache is
// keyed on.
type ContentBlock struct {
	Kind ContentBlockKind `json:"kind"`

	// Text is the body of a BlockText, the reasoning of a BlockThinking, or
	// the tool output of a BlockToolResult. It is empty on a BlockToolUse and
	// on a thinking block the provider returned encrypted (Redacted).
	Text string `json:"text,omitzero"`

	// Signature is the opaque provider blob that must be replayed verbatim and
	// in position for reasoning continuity. It is the same concept under three
	// vendor spellings: Anthropic's thinking `signature`, Gemini's per-part
	// `thoughtSignature`, and the OpenAI Responses reasoning item's
	// `encrypted_content`. Never derive, truncate, or re-encode it.
	Signature string `json:"signature,omitzero"`

	// Redacted marks a thinking block whose reasoning the provider withheld or
	// encrypted. Text is then empty and Signature carries everything the
	// provider will accept back.
	Redacted bool `json:"redacted,omitzero"`

	// ID identifies a BlockToolUse (Anthropic `id`, OpenAI `tool_call.id`).
	// On a BlockThinking it is the provider's reasoning-item id where one
	// exists (OpenAI Responses `rs_...`); providers without item ids leave it
	// empty.
	ID string `json:"id,omitzero"`

	// Name is the tool name on a BlockToolUse. Gemini pairs a function
	// response by name rather than by id, so it is also what a BlockToolResult
	// is matched on there.
	Name string `json:"name,omitzero"`

	// Input is the decoded tool arguments on a BlockToolUse.
	Input map[string]any `json:"input,omitzero"`

	// ToolUseID pairs a BlockToolResult to the BlockToolUse it answers.
	ToolUseID string `json:"tool_use_id,omitzero"`

	// IsError marks a BlockToolResult whose tool failed.
	IsError bool `json:"is_error,omitzero"`
}

// TextBlock builds a text block.
func TextBlock(text string) ContentBlock {
	return ContentBlock{Kind: BlockText, Text: text}
}

// ThinkingBlock builds a reasoning block carrying the provider's verbatim
// signature. An empty signature is legal — some providers stream reasoning
// summaries with no attestation — but such a block cannot be replayed to a
// provider that requires one, and the adapters drop it rather than send an
// unsigned thinking block the API will reject.
func ThinkingBlock(text, signature string) ContentBlock {
	return ContentBlock{Kind: BlockThinking, Text: text, Signature: signature}
}

// RedactedThinkingBlock builds a reasoning block whose text the provider
// encrypted. The signature is all there is, and it must still be replayed.
func RedactedThinkingBlock(signature string) ContentBlock {
	return ContentBlock{Kind: BlockThinking, Signature: signature, Redacted: true}
}

// ToolUseBlock builds a tool invocation block.
func ToolUseBlock(id, name string, input map[string]any) ContentBlock {
	return ContentBlock{Kind: BlockToolUse, ID: id, Name: name, Input: input}
}

// ToolResultBlock builds a tool result block paired to toolUseID.
func ToolResultBlock(toolUseID, content string, isError bool) ContentBlock {
	return ContentBlock{Kind: BlockToolResult, ToolUseID: toolUseID, Text: content, IsError: isError}
}

// ToolCall projects a tool_use block onto the flat ToolCall shape.
func (b ContentBlock) ToolCall() ToolCall {
	return ToolCall{ID: b.ID, Name: b.Name, Input: b.Input}
}

// ToolResult projects a tool_result block onto the flat ToolResult shape.
func (b ContentBlock) ToolResult() ToolResult {
	return ToolResult{ToolUseID: b.ToolUseID, Content: b.Text, IsError: b.IsError}
}

// NewMessage builds a message from ordered content blocks.
//
// The blocks are the message's content. The flat Text/ToolCalls/ToolResults
// fields are filled from them here, once, so that a caller reading the old
// surface sees a faithful — if order-free — projection of the same turn. They
// are never read back to rebuild the blocks.
func NewMessage(role string, blocks ...ContentBlock) Message {
	m := Message{Role: role}
	if len(blocks) == 0 {
		return m
	}
	m.blocks = append(make([]ContentBlock, 0, len(blocks)), blocks...)

	var text strings.Builder
	for _, b := range m.blocks {
		switch b.Kind {
		case BlockText:
			text.WriteString(b.Text)
		case BlockToolUse:
			m.ToolCalls = append(m.ToolCalls, b.ToolCall())
		case BlockToolResult:
			m.ToolResults = append(m.ToolResults, b.ToolResult())
		case BlockThinking:
			// Reasoning is deliberately absent from the flat projection. It is
			// not part of the visible answer, and folding it into Text would
			// leak it into every transcript, prompt echo and log that reads
			// the old surface.
		}
	}
	m.Text = strings.TrimSpace(text.String())
	return m
}

// NewUserMessage builds a user turn from ordered blocks.
func NewUserMessage(blocks ...ContentBlock) Message {
	return NewMessage("user", blocks...)
}

// NewAssistantMessage builds an assistant turn from ordered blocks.
func NewAssistantMessage(blocks ...ContentBlock) Message {
	return NewMessage("assistant", blocks...)
}

// Content returns the message's ordered content blocks.
//
// This is the only correct way to read a message's content. When the message
// was built from blocks they are returned as they were given. When it was
// built from the flat fields — every struct literal in the tree predating this
// representation — they are lifted in a fixed order: tool_result, then text,
// then tool_use.
//
// That order is not arbitrary and it is not a preference. It is the order the
// adapters already emitted by hand, so a legacy message keeps its existing
// wire shape byte for byte on every provider. It also happens to be the
// chronological one: answers to the previous turn's calls, then this turn's
// prose, then this turn's calls.
func (m Message) Content() []ContentBlock {
	if len(m.blocks) > 0 {
		return m.blocks
	}
	out := make([]ContentBlock, 0, 1+len(m.ToolCalls)+len(m.ToolResults))
	for _, tr := range m.ToolResults {
		out = append(out, ToolResultBlock(tr.ToolUseID, tr.Content, tr.IsError))
	}
	if m.Text != "" {
		out = append(out, TextBlock(m.Text))
	}
	for _, tc := range m.ToolCalls {
		out = append(out, ToolUseBlock(tc.ID, tc.Name, tc.Input))
	}
	return out
}

// HasNativeBlocks reports whether this message carries a real ordered block
// list rather than one lifted from the flat fields. Adapters use it to tell a
// turn whose ordering is known from one whose ordering was never recorded.
func (m Message) HasNativeBlocks() bool { return len(m.blocks) > 0 }

// AssistantMessageFrom builds the assistant turn to append to a tool-loop
// history from the response that produced it.
//
// Prefer it over composing a Message literal out of Text and ToolCalls: the
// literal is where a turn's ordering and its thinking signatures are lost, and
// the loss is invisible until a later turn replays the wrong prefix.
func AssistantMessageFrom(resp *LLMToolResponse) Message {
	if resp == nil {
		return Message{Role: "assistant"}
	}
	if len(resp.Blocks) > 0 {
		return NewAssistantMessage(resp.Blocks...)
	}
	return Message{Role: "assistant", Text: resp.Text, ToolCalls: resp.ToolCalls}
}

// Rewrite replaces the turn's prose and tool calls in BOTH views at once.
//
// LLMToolResponse is the one place in this design where the flat fields and the
// block list are both settable, and so the one place they can be given
// contradictory values. Message solved that by hiding the block list behind
// constructors; a response cannot, because adapters build it field by field as
// they parse. So the invariant is kept by making every post-parse edit go
// through here.
//
// The edit that made this necessary is piggyback promotion. It reads a control
// envelope out of the prose and turns it into tool calls, writing both to the
// flat fields — and a block-carrying response would then hold the original
// envelope text and NO tool_use blocks, so AssistantMessageFrom (which prefers
// blocks) would append an assistant turn claiming it called nothing. The next
// user turn carries tool_results for calls that are not in the transcript,
// which strict providers reject outright.
//
// Thinking blocks survive, in front. They carry provider signatures that must
// be replayed verbatim, so dropping them to rebuild the turn would lose exactly
// what the block representation exists to keep; and Anthropic requires them at
// the head of an assistant turn regardless. Everything else is rebuilt from the
// arguments, because after a rewrite the model's original ordering no longer
// describes what the turn means.
//
// A response with no blocks is left with no blocks. Nil means "this provider
// did not tell us the order", and inventing one here would turn an honest
// absence into a claim.
func (r *LLMToolResponse) Rewrite(text string, calls []ToolCall) {
	if r == nil {
		return
	}
	r.Text = text
	r.ToolCalls = calls
	if len(r.Blocks) == 0 {
		return
	}

	rebuilt := make([]ContentBlock, 0, len(r.Blocks)+len(calls))
	for _, b := range r.Blocks {
		switch b.Kind {
		case BlockText, BlockToolUse:
			// Replaced by the arguments.
		default:
			rebuilt = append(rebuilt, b)
		}
	}
	if text != "" {
		rebuilt = append(rebuilt, TextBlock(text))
	}
	for _, c := range calls {
		rebuilt = append(rebuilt, ToolUseBlock(c.ID, c.Name, c.Input))
	}
	r.Blocks = rebuilt
}

// ClearToolCalls drops the turn's tool calls from both views, keeping its prose.
//
// `resp.ToolCalls = nil` is the tempting form and it is half a change: the
// tool_use blocks stay, so a caller reading the blocks still sees the calls the
// assignment was written to retract. It allocates a fresh block slice rather
// than filtering in place, because responses are copied by value in this
// package and an in-place filter would reach through the copy into the
// original's backing array.
func (r *LLMToolResponse) ClearToolCalls() {
	if r == nil {
		return
	}
	r.Rewrite(r.Text, nil)
}

// WithToolResults returns a copy of m carrying these tool results in BOTH
// views, matched to the originals by position.
//
// The tool-loop transcript is bounded by rewriting old tool results — blanked
// to a notice, or clamped head+tail — and the obvious way to do that is to
// assign the message's ToolResults field. On a message built from the flat
// fields that is correct, because Content() lifts them. On a message built from
// blocks it changes only the projection: Content() returns the block list, so
// every adapter goes on sending the full payload and the transcript quietly
// stops being bounded. Nothing fails. The bill arrives instead.
//
// Position is a sound way to match because the flat projection is built by
// walking the blocks in order, so the n-th ToolResult is the n-th
// BlockToolResult. A caller handing back a different number of results has
// already lost that correspondence, so the block list is rebuilt from the
// arguments alone rather than half-matched.
func (m Message) WithToolResults(results []ToolResult) Message {
	if len(m.blocks) == 0 {
		m.ToolResults = results
		return m
	}

	rebuilt := make([]ContentBlock, 0, len(m.blocks))
	aligned := len(results) == len(m.ToolResults)
	seen := 0
	for _, b := range m.blocks {
		if b.Kind != BlockToolResult {
			rebuilt = append(rebuilt, b)
			continue
		}
		if aligned && seen < len(results) {
			r := results[seen]
			rebuilt = append(rebuilt, ToolResultBlock(r.ToolUseID, r.Content, r.IsError))
		}
		seen++
	}
	if !aligned {
		for _, r := range results {
			rebuilt = append(rebuilt, ToolResultBlock(r.ToolUseID, r.Content, r.IsError))
		}
	}
	return NewMessage(m.Role, rebuilt...)
}
