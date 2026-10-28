package types

import "time"

// A tool earns its place by doing something the harness cannot do any other
// way, by consuming far more material than it returns, or by costing far fewer
// tokens than the work it does or saves. The types here are the measurements
// the session executor hands a ToolMeter so that judgment rests on numbers,
// not on how a tool reads in a catalog. The ledger that keeps them is
// autopoiesis.ProfileStore; the rules that classify a tool from them are
// policy/tool_value.mg.

// ToolChannel is the path a tool call arrived on. The same tool costs the same
// tokens on every channel; the channel is recorded so a tool that only one
// channel ever calls is visible as that.
type ToolChannel string

const (
	// ToolChannelNative is a call on a provider's typed tool channel.
	ToolChannelNative ToolChannel = "native"
	// ToolChannelPiggyback is a call a Piggyback client wrote into its text
	// envelope.
	ToolChannelPiggyback ToolChannel = "piggyback"
	// ToolChannelGenerated is a call to a generated (Ouroboros) tool.
	ToolChannelGenerated ToolChannel = "generated"
	// ToolChannelMCP is a call to an MCP verb (mcp_call and its siblings).
	ToolChannelMCP ToolChannel = "mcp"
)

// ToolOrigin says where a tool came from, which decides what a retirement
// proposal may be: a generated tool can be proposed for removal, a built-in
// is only ever flagged.
type ToolOrigin string

const (
	ToolOriginBuiltin   ToolOrigin = "builtin"
	ToolOriginGenerated ToolOrigin = "generated"
)

// ToolOffer is one tool's prompt cost in one compile: the tokens its catalog
// entry puts in the request, and the tokens of the included instruction atoms
// that declare they teach it (requires_tools).
type ToolOffer struct {
	Tool      string
	Origin    ToolOrigin
	SessionID string
	// CatalogTokens is the tool's definition (name, description, schema) as a
	// request carries it.
	CatalogTokens int
	// AtomTokens is the share of the included atoms' tokens that name the tool
	// in requires_tools. An atom naming several offered tools is split evenly
	// among them, so the shares of one atom never exceed its tokens.
	AtomTokens int
	// Atoms are the ids of the atoms AtomTokens came from.
	Atoms []string
}

// ToolConsumption is the material a tool call scanned to produce its result:
// the files, rows or bytes it read. A tool that does not report one is
// Known == false; the ledger records "unknown", never a guess.
type ToolConsumption struct {
	Known bool
	Bytes int64
	Items int64
}

// ToolCallMeasure is one tool call on any channel.
type ToolCallMeasure struct {
	Tool      string
	Origin    ToolOrigin
	Channel   ToolChannel
	SessionID string
	Success   bool
	Duration  time.Duration
	// CallTokens is the model-emitted call (name and arguments) in tokens,
	// counted with the broker's calibrated ruler.
	CallTokens int
	// ResultTokens and ResultBytes are what the tool returned (output plus
	// error text), before the working context pages it.
	ResultTokens int
	ResultBytes  int64
	// Consumed is what the tool reported scanning.
	Consumed ToolConsumption
}

// ToolVerification is a verification call attributed to the write that made it
// necessary: the build or test the model ran after Tool changed the tree.
type ToolVerification struct {
	// Tool is the write tool the verification is charged to.
	Tool string
	// Verifier is the verification tool that ran.
	Verifier string
	// Tokens are the verification call's own tokens (call and result).
	Tokens int
}

// ToolMeter receives the measurements. It is implemented by the tool ledger
// (autopoiesis.ProfileStore); the executor holds it as an interface because
// internal/session does not import internal/autopoiesis. Implementations must
// be safe for concurrent use and must not block a tool call on a failure: a
// ledger that cannot persist logs and drops, it does not fail the call.
type ToolMeter interface {
	NoteToolOffers(offers []ToolOffer)
	NoteToolCall(call ToolCallMeasure)
	NoteToolVerification(v ToolVerification)
}
