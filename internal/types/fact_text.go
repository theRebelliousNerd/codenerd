package types

// =============================================================================
// FACT TEXT BOUNDS
// =============================================================================
// A Mangle fact argument is a value the kernel hashes, indexes and compares on
// every fixpoint pass. Storing an unbounded tool result in one — a read_file of
// a large source file, the combined output of `go test ./...` — makes the EDB
// carry megabytes per action and makes every evaluation pay to move them.
//
// It also leaks outward: facts are what `nerd query`, the glass-box explainer
// and factsnap snapshots read, and the legacy articulation path can render
// kernel facts straight into a prompt.
//
// Two call sites did exactly this:
//
//	execution_result(ActionID, Type, Target, Success, Output, Ts)
//	  internal/core/virtual_store_routing.go — the whole ActionResult.Output.
//	  No rule reads the Output argument at all; it is declared and never joined.
//
//	routing_result(CallID, Verdict, Details, Ts)
//	  internal/shards/system/router.go — the whole tool result string. The only
//	  rule that reads Details is routing_failed/2, which matches the /failure
//	  case; the /success case discards it with `_`. The ToolEventBus emission
//	  five lines below already truncated to 500 characters for display, while
//	  the fact above it took everything.

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxFactTextBytes bounds a free-text fact argument.
//
// 4 KiB is far more than any rule needs — no policy rule in the corpus does
// substring work on these arguments — and still enough that `nerd query` and
// the glass box show a useful excerpt rather than a stub. The full text is
// never lost: it stays on the ActionResult the caller already holds, where the
// session's own 16 KiB tool-result cap governs what reaches the model.
const MaxFactTextBytes = 4096

// factTextMarkerBudget is the room reserved for the truncation marker inside
// MaxFactTextBytes.
//
// Reserved rather than appended past the cap, so the returned value is always
// within the bound and truncating an already-truncated value is a no-op. Not
// reserving it made the function non-idempotent: a 4096-byte cut plus a
// 35-byte marker came back at 4131, and a value that round-tripped through the
// fact store got re-cut and re-marked on every pass. 64 bytes covers the marker
// for any input size the process can hold.
const factTextMarkerBudget = 64

// TruncateFactText bounds s for storage in a fact argument, appending a marker
// that states how much was dropped.
//
// The marker matters. A silently truncated result is one an operator reading
// the fact store — or a rule author reasoning about it — will take for the
// whole story. Truncation is on a rune boundary so the value stays valid UTF-8;
// a half-rune in a fact argument breaks the Mangle string encoding downstream.
//
// The result is always at most MaxFactTextBytes, marker included, which makes
// the function idempotent.
func TruncateFactText(s string) string {
	if len(s) <= MaxFactTextBytes {
		return s
	}
	cut := MaxFactTextBytes - factTextMarkerBudget
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	dropped := len(s) - cut
	var b strings.Builder
	b.Grow(MaxFactTextBytes)
	b.WriteString(s[:cut])
	fmt.Fprintf(&b, "\n… [truncated %d of %d bytes]", dropped, len(s))
	return b.String()
}
