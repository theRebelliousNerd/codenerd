package session

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

func stdinReceipt(output, stderr string) types.GeneratedToolReceipt {
	r := types.GeneratedToolReceipt{Output: output, Stderr: stderr}
	r.Request.Tool.Protocol = types.GeneratedStdinV1
	return r
}

// Finding B: stderr was kept in the receipt for learning but never reached
// the model; only the decoded output did.
func TestGeneratedObservation_CarriesStderrBesideTheResult(t *testing.T) {
	got := generatedObservation(stdinReceipt("partial: 3 of 5", "open d.go: permission denied\n"))
	if !strings.HasPrefix(got, "partial: 3 of 5\n\n[generated tool stderr]\nopen d.go: permission denied") {
		t.Fatalf("observation = %q", got)
	}
	// A process that died before writing an envelope: stderr is the only
	// explanation there is.
	if got := generatedObservation(stdinReceipt("", "panic: nil map\n")); got != "[generated tool stderr]\npanic: nil map" {
		t.Fatalf("stderr-only observation = %q", got)
	}
}

func TestGeneratedObservation_LeavesTheResultAloneWithoutStderr(t *testing.T) {
	if got := generatedObservation(stdinReceipt(`{"ok":true}`, "  \n")); got != `{"ok":true}` {
		t.Fatalf("observation = %q", got)
	}
	legacy := types.GeneratedToolReceipt{Output: "out+err", Stderr: "err"}
	legacy.Request.Tool.Protocol = types.LegacyArgvV1
	if got := generatedObservation(legacy); got != "out+err" {
		t.Fatalf("legacy output (stderr already appended by the runtime) = %q", got)
	}
}

func TestGeneratedObservation_RedactsCredentialsAndMarksACutCapture(t *testing.T) {
	r := stdinReceipt("", "using api_key=sk-live-0123456789abcdef0123456789abcdef")
	r.OutputTruncated = true
	got := generatedObservation(r)
	if strings.Contains(got, "sk-live-0123456789abcdef") || !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("secret reached the model: %q", got)
	}
	if !strings.Contains(got, "capture bound was reached") {
		t.Fatalf("a cut capture was not marked: %q", got)
	}
}
