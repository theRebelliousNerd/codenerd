package perception

import "codenerd/internal/types"

// outputTruncated is the typed error a client returns when the provider
// stopped the completion at its output ceiling. The partial text travels in
// the error for diagnostics and nowhere else: a cut answer is not an answer,
// and the broker sends the request back to the model to be restated within
// the limit (internal/broker/compression.go).
// lengthStop is types.LengthStop for the clients in this package.
func lengthStop(reason string) bool { return types.LengthStop(reason) }

// orDefaultTokens returns n, or def when n is not positive. Every provider
// client's completion ceiling resolves through it: the configured
// max_output_tokens when one was given, else the client's documented default.
// Until 2026-09-10 five clients carried their ceiling as a literal in each
// request builder, unreachable from configuration.
func orDefaultTokens(n, def int) int {
	if n > 0 {
		return n
	}
	return def
}

func outputTruncated(provider Provider, model, method, reason, partial string, limit, produced int) error {
	return &types.OutputTruncated{
		Provider:     string(provider),
		Model:        model,
		Method:       method,
		Reason:       reason,
		LimitTokens:  limit,
		OutputTokens: produced,
		Partial:      partial,
	}
}
