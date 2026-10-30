package session

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/experts"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

type requiringConsultant struct{}

func (requiringConsultant) SpawnConsultation(context.Context, string, string) (string, error) {
	return "Keep the old column readable.\nREQUIRE TestDecoderKeepsOldColumn: readers overlap the rollout\nREQUIRE not-a-test: ignored", nil
}

func TestParseMandates(t *testing.T) {
	got := parseMandates("REQUIRE TestA_b2: why\nREQUIRE `TestB`: quoted\nREQUIRE Bench: no\nREQUIRE Test-x: no\nrequire TestC: lower")
	if len(got) != 2 || got[0] != (cardMandate{Test: "TestA_b2", Why: "why"}) || got[1].Test != "TestB" {
		t.Fatalf("parseMandates = %+v", got)
	}
}

// An expert's REQUIRE reaches the turn's prompt as a requirement only when the
// registry grants the agent mandate authority; otherwise it stays in the
// card's advice.
func TestConsultation_AnAuthorizedRequirementIsNamedToTheTurn(t *testing.T) {
	for _, granted := range []bool{false, true} {
		e, _ := routeFixture(t)
		e.SetExpertConsultant(requiringConsultant{})
		if granted {
			if err := e.kernel.Assert(types.Fact{Predicate: "shard_agent_authority", Args: []any{experts.AgentAtom("schemaexpert"), types.MangleAtom("/mandate")}}); err != nil {
				t.Fatal(err)
			}
		}
		sel := experts.Selection{Needs: []experts.Need{{Agent: "schemaexpert", Spawn: experts.SpawnAdvise, Files: []string{"a.go"}}}}
		out := e.withExpertConsultations(context.Background(), "turn prompt", perception.Intent{Verb: "/fix", Target: "a.go"}, "fix a", sel, &ExecutionResult{}, nil)
		named := strings.Contains(out, "## Requirements this turn must meet") && strings.Contains(out, "  - TestDecoderKeepsOldColumn")
		if named != granted {
			t.Fatalf("authority %v: requirement named = %v:\n%s", granted, named, out)
		}
	}
}
