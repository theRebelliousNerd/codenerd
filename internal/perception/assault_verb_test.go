package perception

import (
	"context"
	"testing"
)

func TestMatchVerbFromCorpus_Assault(t *testing.T) {
	verb, _, _, _ := matchVerbFromCorpus(context.Background(), "adversarial assault internal/core")
	if verb != "/assault" {
		t.Fatalf("expected /assault, got %q", verb)
	}
}

