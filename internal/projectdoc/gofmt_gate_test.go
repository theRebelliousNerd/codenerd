package projectdoc

import "testing"

// gofmt fell through ClassifyShellEffect's lists into the default
// UnknownMutating and was refused, so codeNERD could not check the formatting
// of its own output — the one verification a Go agent most needs. Observed
// live: "shell-effect gate BLOCKED run_command effect=unknown_mutating".
//
// The point of these cases is the split. Allowing the gofmt binary wholesale
// would hand the model an in-place file rewriter through a gate that exists to
// decide whether writes are permitted, so -l and -d (which only report) are
// read-only while -w (which rewrites) goes down the mutation path.
func TestGofmtShellEffectClassification(t *testing.T) {
	cases := []struct {
		command string
		want    ShellEffectKind
	}{
		// Reporting only: safe to run unprompted.
		{"gofmt -l internal/campaign/", ShellEffectReadOnly},
		{"gofmt -l .", ShellEffectReadOnly},
		{"gofmt -d internal/campaign/x.go", ShellEffectReadOnly},

		// Rewrites files: must not be classified read-only.
		{"gofmt -w internal/campaign/x.go", ShellEffectMutating},
		{"gofmt -l -w .", ShellEffectMutating},
		{"go fmt ./...", ShellEffectMutating},

		// Unrecognised shape still falls through to the default deny rather
		// than being waved through because the binary is named gofmt.
		{"gofmt", ShellEffectUnknownMutating},
		{"gofmt --some-future-flag .", ShellEffectUnknownMutating},
	}

	for _, tc := range cases {
		if got := ClassifyShellEffect(tc.command); got != tc.want {
			t.Errorf("ClassifyShellEffect(%q) = %v, want %v", tc.command, got, tc.want)
		}
	}
}

// The commands codeNERD actually runs to verify its own work must stay
// permitted; this is the regression that would bite hardest.
func TestGoVerificationCommandsStayPermitted(t *testing.T) {
	for _, command := range []string{
		"go test ./...",
		"go test ./internal/campaign/ -run Callsite -count=1 -v",
		"go vet ./...",
		"go build ./...",
	} {
		if got := ClassifyShellEffect(command); got != ShellEffectVerification {
			t.Errorf("ClassifyShellEffect(%q) = %v, want verification", command, got)
		}
	}
}
