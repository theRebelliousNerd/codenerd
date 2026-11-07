package campaign

import (
	"strings"
	"testing"
)

func TestWitnessInvalidatedNamesEveryChangedFile(t *testing.T) {
	err := witnessInvalidated([]string{".nerd/agents/coder.md", "cmd/nerd/main.go", "internal/campaign/x_test.go"})
	want := "witness invalidated: 3 file(s) changed: .nerd/agents/coder.md, cmd/nerd/main.go, internal/campaign/x_test.go"
	if got := err.Error(); !strings.HasSuffix(got, want) {
		t.Fatalf("error = %q, want suffix %q", got, want)
	}
}
