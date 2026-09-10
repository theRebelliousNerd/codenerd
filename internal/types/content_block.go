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
