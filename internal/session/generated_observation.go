package session

import (
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// generatedObservation is what the solving model receives from a generated
// tool call: the decoded result, and the process's stderr as a separately
// labelled diagnostic (Docs/audits/generated-tool-observation-fidelity.md,
// Finding B). The receipt kept stderr for durable learning while the tool
// result carried only the decoded output, so a diagnostic written to stderr --
// or the only explanation of a process that failed before writing an envelope
// -- never reached the model deciding how to respond.
//
// receipt.Output itself is not changed: validators and learning read the
// decoded result, and stderr is not part of the protocol's stdout. A present
// stderr is not a failure and an empty one is not success; the receipt's own
// error decides that. Stderr is tool-authored data, so credential-shaped
// substrings are redacted (shown as [REDACTED]) before the model sees it. The
// whole diagnostic is carried: the working context archives a large result
// behind a recall handle rather than cutting it.
func generatedObservation(r types.GeneratedToolReceipt) string {
	if r.Request.Tool.Protocol == types.LegacyArgvV1 {
		return r.Output // the legacy runtime already appends stderr to Output
	}
	stderr := strings.TrimRight(r.Stderr, "\r\n\t ")
	if strings.TrimSpace(stderr) == "" {
		return r.Output
	}
	label := "[generated tool stderr]"
	if r.OutputTruncated {
		label = "[generated tool stderr; the capture bound was reached, so this is a prefix]"
	}
	diagnostic := label + "\n" + logging.RedactSecrets(stderr)
	if r.Output == "" {
		return diagnostic
	}
	return r.Output + "\n\n" + diagnostic
}
