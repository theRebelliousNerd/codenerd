package perception

// Provider fidelity: what each request surface can actually carry.
//
// A conversation turn is an ordered list of typed blocks — prose, signed
// reasoning, tool calls, tool results — and every provider takes a different
// subset of that back on the wire. Those limits were written down four times,
// once in the doc comment of each mapper, in prose. Prose is where a limit goes
// to become a claim: the comment above MapTypesHistoryToOpenAIMessages says
// thinking is dropped, and until this file existed nothing anywhere checked
// that it still was.
//
// This is the same shape as the rest of this branch's gates. The table is the
// declaration; provider_fidelity_test.go runs the real mappers over one
// interleaved turn and holds each of them to it. A mapper that quietly starts
// replaying signatures fails, and so does one that quietly stops.

// Surface names one provider request format. It is the format, not the vendor:
// OpenAI ships two of these and they have different fidelity, which is exactly
// the distinction a vendor-keyed table would lose.
type Surface string

const (
	// SurfaceAnthropicMessages is the Anthropic /v1/messages content-block
	// format. It is the reference surface: everything survives.
	SurfaceAnthropicMessages Surface = "anthropic_messages"
	// SurfaceOpenAIChatCompletions is /v1/chat/completions and every
	// OpenAI-compatible endpoint that clones it (DeepSeek, Qwen, Groq,
	// SuperGrok's OAuth surface, local servers).
	SurfaceOpenAIChatCompletions Surface = "openai_chat_completions"
	// SurfaceOpenAIResponses is the OpenAI Responses API item list, which
	// Meta's Muse Spark surface also speaks.
	SurfaceOpenAIResponses Surface = "openai_responses"
	// SurfaceGeminiContents is Gemini's generateContent parts list.
	SurfaceGeminiContents Surface = "gemini_contents"
)

// Fidelity is what one surface preserves when a turn is sent back to it.
//
// Every field is about the REQUEST direction. Reading a response is the easy
// half — a provider tells you what it produced — and the expensive mistakes are
// all on the way back in, because that is where a turn the model never produced
// gets handed to it as its own history.
//
// Three fields, not four. The fourth difference the mapper comments raise —
// whether a tool result's error flag is a protocol field (Anthropic) or data
// smuggled inside the payload (Gemini) — is a nesting difference, not a
// presence one, so no probe short of walking the decoded request can see it,
// and no caller decides anything differently either way. It stays prose in
// blocks_gemini.go, where it describes the one surface it is about. A boolean
// nothing checks and nothing reads is the exact failure this file exists to
// fix, and adding one here would be a fast way to reintroduce it.
type Fidelity struct {
	// KeepsOrder is whether prose emitted between two tool calls stays between
	// them.
	//
	// Where this is false the wire format has one content field per assistant
	// turn, so "I'll check two things" / call A / "now the caller" / call B
	// arrives as both sentences welded together followed by both calls. The
	// model reads its own last turn as something it did not say.
	KeepsOrder bool

	// ReplaysReasoning is whether a thinking block can be sent back at all.
	//
	// This is the field with a price on it. Where it is false, reasoning is
	// generated fresh every turn of a tool loop and thrown away every turn:
	// the provider will bill for it and has no request-side field to accept it
	// back. Some of these surfaces RETURN reasoning (Qwen's
	// reasoning_content); none of them take it.
	ReplaysReasoning bool

	// CarriesToolIDs is whether the tool-use identifier exists on the wire.
	//
	// Where it is false the surface pairs a result to its call by tool NAME
	// (Gemini), and the ids the rest of codeNERD pairs on are minted locally
	// from position. Two concurrent calls to the same tool are then
	// indistinguishable to the provider, which is a property of the protocol
	// and not something a client can fix.
	CarriesToolIDs bool
}

// BlockFidelity is the machine-readable form of every "what this surface cannot
// represent" note in this package.
//
// Ask it before assuming a turn survives. In particular, read
// BlockFidelity[surface].ReplaysReasoning before paying for extended thinking
// inside a multi-turn tool loop.
//
// An undeclared surface answers false to everything, because that is Go's zero
// value for the struct, and here the zero value is also the safe answer: a
// caller declines to pay for reasoning it could have replayed, rather than
// sending a signature to an endpoint that rejects the whole request over it.
// That is worth stating because it is load-bearing and invisible — there is no
// comma-ok anywhere near the call site to warn a reader that a miss is being
// interpreted rather than reported.
var BlockFidelity = map[Surface]Fidelity{
	SurfaceAnthropicMessages: {
		KeepsOrder:       true,
		ReplaysReasoning: true,
		CarriesToolIDs:   true,
	},
	SurfaceOpenAIChatCompletions: {
		// One content string and one tool_calls array per assistant turn.
		// Splitting the turn to preserve position would put a tool_calls
		// message somewhere other than immediately before its results, which
		// the endpoint rejects, so the order is collapsed on purpose.
		KeepsOrder: false,
		// No request-side reasoning field exists on this surface at all.
		ReplaysReasoning: false,
		CarriesToolIDs:   true,
	},
	SurfaceOpenAIResponses: {
		// An item list, so position is native.
		KeepsOrder: true,
		// Reasoning items replay by encrypted_content.
		ReplaysReasoning: true,
		CarriesToolIDs:   true,
	},
	SurfaceGeminiContents: {
		// A parts list, so position is native.
		KeepsOrder: true,
		// A part carries Thought and a ThoughtSignature, replayed verbatim.
		ReplaysReasoning: true,
		// Pairing is by function name; ids are minted positionally here.
		CarriesToolIDs: false,
	},
}
