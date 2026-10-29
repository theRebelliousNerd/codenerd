package main

import (
	"errors"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// TestPrintDreamLearnings_ExtractsFromSuccessfulConsultations covers the gap
// this closed: `nerd dream` spent up to four LLM round-trips, printed the
// prose, and dropped every insight. Extraction is pure computation over
// consultations already in memory — the only reason it was not here is that
// nobody wired it.
func TestPrintDreamLearnings_ExtractsFromSuccessfulConsultations(t *testing.T) {
	results := []dreamResult{{
		name:     "coder",
		shardTyp: types.ShardTypeEphemeral,
		response: "Risk: migrating the schema could corrupt existing rows if the " +
			"backfill runs before the constraint is dropped.",
	}}

	consultations := dreamConsultationsFor(results)
	if len(consultations) != 1 {
		t.Fatalf("got %d consultations, want 1", len(consultations))
	}
	if consultations[0].Perspective == "" {
		t.Error("the agent's answer did not survive conversion; there is nothing to learn from")
	}

	learnings := core.NewDreamLearningCollector().ExtractLearnings("migrate the schema", consultations)
	if len(learnings) == 0 {
		t.Fatal("a consultation naming a concrete risk produced no learnings")
	}
}

// TestPrintDreamLearnings_SkipsFailedConsultations: an agent that errored has
// no perspective, and its zero-valued response would be extracted as if it
// were one.
func TestPrintDreamLearnings_SkipsFailedConsultations(t *testing.T) {
	results := []dreamResult{
		{name: "coder", err: errors.New("timeout")},
		{name: "tester", response: "Careful: this could break the fixtures."},
	}

	consultations := dreamConsultationsFor(results)
	if len(consultations) != 1 {
		t.Fatalf("got %d consultations, want only the one that answered", len(consultations))
	}
	if consultations[0].ShardName != "tester" {
		t.Errorf("kept %q, want the agent that answered", consultations[0].ShardName)
	}
}

func TestPrintDreamLearnings_NoConsultationsIsSilent(t *testing.T) {
	// Must not panic, and must not print a heading with nothing under it.
	printDreamLearnings("anything", nil)
	printDreamLearnings("anything", []dreamResult{{name: "coder", err: errors.New("failed")}})
}

// TestPrintDreamLearnings_DoesNotClaimPersistence pins the honesty of the
// output. DreamRouter refuses to route an unconfirmed learning, and a one-shot
// CLI has nobody to ask — so the command must not leave the reader believing
// their insights were saved when nothing was written.
func TestPrintDreamLearnings_DoesNotClaimPersistence(t *testing.T) {
	out, _ := captureStdout(t, func() error {
		printDreamLearnings("migrate the schema", []dreamResult{{
			name:     "coder",
			shardTyp: types.ShardTypeEphemeral,
			response: "Risk: dropping the column could corrupt rows that have not been backfilled.",
		}})
		return nil
	})

	if !strings.Contains(out, "learnable insight") {
		t.Fatalf("nothing was surfaced:\n%s", out)
	}
	if !strings.Contains(out, "Not persisted") {
		t.Errorf("the output does not say the insights were not stored:\n%s", out)
	}
}
