package types

import (
	"errors"
	"fmt"
	"strings"
)

// OutputTruncated reports that a completion stopped because it reached the
// provider's output ceiling, not because the model finished.
//
// A provider client returns this instead of the partial text. The partial is
// never an answer: it is a document with its ending missing, and every
// consumer downstream — the articulation parser, the tool loop, the campaign
// checkpoint — would treat it as complete. Until 2026-09-10 every client did
// exactly that: the finish reason was copied into a field nothing inspected,
// and a cut answer was stored in history and shown to the user as if it were
// whole.
//
// The broker, which every inference call passes through, is the one place that
// acts on this error: it sends the same request back with an instruction to
// restate the whole answer within the limit, and only after bounded retries
// does the error reach a caller. See internal/broker/compression.go.
type OutputTruncated struct {
	Provider string
	Model    string
	Method   string
	// Reason is the provider's own word for the stop: "length",
	// "max_tokens", "MAX_TOKENS", "max_output_tokens".
	Reason string
	// LimitTokens is the ceiling in force, when the client knows it.
	LimitTokens int
	// OutputTokens is what the provider billed for the cut response, when
	// reported.
	OutputTokens int
	// Partial is what came back before the cut. Diagnostics only.
	Partial string
}

// Error implements error.
func (e *OutputTruncated) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s output truncated at the provider's ceiling", e.Method)
	if e.Provider != "" {
		fmt.Fprintf(&b, " (%s", e.Provider)
		if e.Model != "" {
			fmt.Fprintf(&b, " %s", e.Model)
		}
		b.WriteString(")")
	}
	if e.Reason != "" {
		fmt.Fprintf(&b, ": finish=%q", e.Reason)
	}
	if e.OutputTokens > 0 || e.LimitTokens > 0 {
		fmt.Fprintf(&b, " output_tokens=%d limit=%d", e.OutputTokens, e.LimitTokens)
	}
	fmt.Fprintf(&b, " partial_chars=%d", len(e.Partial))
	return b.String()
}

// AsOutputTruncated unwraps err to the truncation report, if it carries one.
func AsOutputTruncated(err error) (*OutputTruncated, bool) {
	var t *OutputTruncated
	if errors.As(err, &t) {
		return t, true
	}
	return nil, false
}

// LengthStop reports whether a provider's finish or stop reason means the
// output hit the completion ceiling. The vocabularies: OpenAI-shaped APIs say
// "length", Anthropic says "max_tokens", Gemini says "MAX_TOKENS", and the
// Responses API reports an incomplete status with reason "max_output_tokens".
func LengthStop(reason string) bool {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "length", "max_tokens", "max_output_tokens":
		return true
	}
	return false
}
