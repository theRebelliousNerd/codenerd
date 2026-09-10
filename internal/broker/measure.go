package broker

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// Per-element structural overheads, in characters. These stand in for the JSON
// framing, role markers, and content-block wrappers a provider adds around each
// element on the wire.
//
// Their exact values matter far less than they appear to, because the
// calibrator learns a chars-per-token ratio over the output of measure() and
// then predicts from the output of measure(). Any systematic bias these
// constants introduce is absorbed into the learned ratio and cancels. What they
// must do is scale correctly with the shape of a request — a 50-message history
// costs more framing than a 2-message one — so that the ratio stays stable as
// request shape changes.
const (
	perMessageOverheadChars   = 16
	perToolCallOverheadChars  = 24
	perToolResultOverheadChar = 24
	perToolDefOverheadChars   = 32
	systemBlockOverheadChars  = 16
)

// measure returns the character size of an assembled request, broken out by
// segment. It counts runes rather than bytes because token/rune ratios are far
// more stable across languages than token/byte ratios.
//
// This is the single definition of "how big is this request" in the codebase.
// Both prediction and calibration go through it, which is what lets the learned
// ratio absorb framing overhead instead of fighting it.
func measure(req *Request) Segments {
	var seg Segments

	if req == nil {
		return seg
	}

	if req.System != "" {
		seg.System = utf8.RuneCountInString(req.System) + systemBlockOverheadChars
	}
	if req.User != "" {
		seg.User = utf8.RuneCountInString(req.User) + perMessageOverheadChars
	}

	for i := range req.Messages {
		msg := &req.Messages[i]
		n := perMessageOverheadChars + utf8.RuneCountInString(msg.Role) + utf8.RuneCountInString(msg.Text)

		for j := range msg.ToolCalls {
			call := &msg.ToolCalls[j]
			n += perToolCallOverheadChars +
				utf8.RuneCountInString(call.ID) +
				utf8.RuneCountInString(call.Name) +
				jsonChars(call.Input)
		}

		for j := range msg.ToolResults {
			res := &msg.ToolResults[j]
			n += perToolResultOverheadChar +
				utf8.RuneCountInString(res.ToolUseID) +
				utf8.RuneCountInString(res.Content)
		}

		seg.History += n
	}

	for i := range req.Tools {
		tool := &req.Tools[i]
		seg.Tools += perToolDefOverheadChars +
			utf8.RuneCountInString(tool.Name) +
			utf8.RuneCountInString(tool.Description) +
			jsonChars(tool.InputSchema)
	}

	return seg
}

// jsonChars returns the rune length of v's JSON encoding, or a conservative
// fallback when v cannot be marshalled.
//
// The fallback matters: a tool schema containing an unmarshallable value would
// otherwise be measured as zero, and a request could be admitted on a count
// that omitted it entirely. Undercounting is the dangerous direction, so an
// encoding failure yields a deliberately large number rather than nothing.
func jsonChars(v any) int {
	if v == nil {
		return 0
	}
	data, ok := jsonBytes(v)
	if !ok {
		return unmarshallableSchemaChars
	}
	return utf8.RuneCount(data)
}

// jsonBytes marshals v deterministically, reporting whether it succeeded.
//
// Determinism holds because encoding/json sorts map keys, which is what lets
// the same schema fingerprint identically across runs.
//
// On failure it falls back to Go's own syntax rendering rather than to a shared
// constant. A shared constant would make two different unmarshallable schemas
// hash alike, which would splice two genuinely distinct prefixes into one epoch
// and overstate cache reuse — an error in the direction that flatters the
// caching bet, which is the direction never to fail in.
func jsonBytes(v any) ([]byte, bool) {
	if v == nil {
		return nil, true
	}
	data, err := json.Marshal(v)
	if err != nil {
		return []byte(fmt.Sprintf("unmarshallable:%#v", v)), false
	}
	return data, true
}

// unmarshallableSchemaChars is charged for a value that will not marshal. It is
// large on purpose: fail toward refusing a request rather than toward silently
// undercounting one.
const unmarshallableSchemaChars = 4096

// splitProportional distributes an authoritative total across segments in
// proportion to their measured character sizes.
//
// Provider counting endpoints return a single number for the whole request, so
// per-segment attribution cannot be measured directly without paying for one
// API call per segment. The total stays authoritative; the split is declared
// proportional in the Segments doc comment and used for observability only.
func splitProportional(total int, chars Segments) Segments {
	sum := chars.Total()
	if sum <= 0 || total <= 0 {
		return Segments{}
	}

	scale := func(part int) int { return int(int64(total) * int64(part) / int64(sum)) }

	out := Segments{
		System:  scale(chars.System),
		History: scale(chars.History),
		User:    scale(chars.User),
	}
	// Give the remainder to Tools so the segments always sum to exactly total.
	// Integer division would otherwise lose up to three tokens per receipt, and
	// a receipt whose parts do not add up invites exactly the kind of doubt
	// this package exists to remove.
	out.Tools = total - out.System - out.History - out.User
	if out.Tools < 0 {
		out.Tools = 0
	}
	return out
}
