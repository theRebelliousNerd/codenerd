package northstar

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

func linkFactArgs(facts []types.Fact, predicate string) [][]any {
	var out [][]any
	for _, f := range facts {
		if f.Predicate == predicate {
			out = append(out, f.Args)
		}
	}
	return out
}

func linkedVision() *Vision {
	return &Vision{
		Mission: "m", Problem: "p", VisionStmt: "v",
		Personas: []Persona{
			{Name: "Operator", Needs: []string{"audit trail"}},
			{Name: "Maintainer"},
		},
		Capabilities: []Capability{
			{ID: "cap_1", Description: "projection", Timeline: "now", Priority: "critical", Serves: []string{"Operator", "persona_Maintainer"}},
			{ID: "cap_2", Description: "orphan", Timeline: "later", Priority: "low"},
		},
		Risks: []Risk{
			{ID: "risk_1", Description: "drift", Likelihood: "high", Impact: "high", Mitigation: "reconcile on boot"},
		},
		Requirements: []Requirement{
			{ID: "req_1", Type: "functional", Description: "single authority", Priority: "must_have",
				Supports: []string{"cap_1"}, Addresses: []string{"risk_1"}},
		},
	}
}

func TestToFacts_WhenCapabilityServesPersona_ShouldEmitNorthstarServes(t *testing.T) {
	facts := linkedVision().ToFacts()

	serves := linkFactArgs(facts, "northstar_serves")
	if len(serves) != 2 {
		t.Fatalf("northstar_serves count = %d, want 2 (%v)", len(serves), serves)
	}
	seen := map[string]bool{}
	for _, args := range serves {
		seen[args[0].(string)+"->"+args[1].(string)] = true
	}
	if !seen["cap_1->persona_Operator"] {
		t.Error("missing northstar_serves(cap_1, persona_Operator): a bare persona name must resolve to its fact ID")
	}
	if !seen["cap_1->persona_Maintainer"] {
		t.Error("missing northstar_serves(cap_1, persona_Maintainer): an already-encoded ID must pass through")
	}
}

func TestToFacts_WhenRequirementLinks_ShouldEmitSupportsAndAddresses(t *testing.T) {
	facts := linkedVision().ToFacts()

	supports := linkFactArgs(facts, "northstar_supports")
	if len(supports) != 1 || supports[0][0] != "req_1" || supports[0][1] != "cap_1" {
		t.Errorf("northstar_supports = %v, want [[req_1 cap_1]]", supports)
	}
	addresses := linkFactArgs(facts, "northstar_addresses")
	if len(addresses) != 1 || addresses[0][0] != "req_1" || addresses[0][1] != "risk_1" {
		t.Errorf("northstar_addresses = %v, want [[req_1 risk_1]]", addresses)
	}
}

func TestToFacts_WhenLinkTargetMissing_ShouldNotEmitDanglingFact(t *testing.T) {
	v := linkedVision()
	v.Capabilities[0].Serves = append(v.Capabilities[0].Serves, "Ghost")
	v.Requirements[0].Supports = append(v.Requirements[0].Supports, "cap_missing")
	v.Requirements[0].Addresses = append(v.Requirements[0].Addresses, "risk_missing")

	facts := v.ToFacts()

	for _, args := range linkFactArgs(facts, "northstar_serves") {
		if args[1] == "persona_Ghost" {
			t.Error("emitted northstar_serves against a persona that does not exist; unserved_persona would be silently wrong")
		}
	}
	for _, args := range linkFactArgs(facts, "northstar_supports") {
		if args[1] == "cap_missing" {
			t.Error("emitted northstar_supports against a capability that does not exist")
		}
	}
	for _, args := range linkFactArgs(facts, "northstar_addresses") {
		if args[1] == "risk_missing" {
			t.Error("emitted northstar_addresses against a risk that does not exist")
		}
	}
}

func TestToFacts_WhenRiskHasMitigation_ShouldEncodeTextNotAConstant(t *testing.T) {
	facts := linkedVision().ToFacts()

	mitigations := linkFactArgs(facts, "northstar_mitigation")
	if len(mitigations) != 1 {
		t.Fatalf("northstar_mitigation count = %d, want 1", len(mitigations))
	}
	atom, ok := mitigations[0][1].(types.MangleAtom)
	if !ok {
		t.Fatalf("strategy arg type = %T, want types.MangleAtom", mitigations[0][1])
	}
	if string(atom) == "/mitigation" {
		t.Fatal("strategy is still the constant /mitigation; every risk's mitigation unifies with every other")
	}
	if !strings.HasPrefix(string(atom), "/mit_reconcile_on_boot_") {
		t.Errorf("strategy atom = %q, want a slug derived from the mitigation text", atom)
	}

	texts := linkFactArgs(facts, "northstar_mitigation_text")
	if len(texts) != 1 {
		t.Fatalf("northstar_mitigation_text count = %d, want 1", len(texts))
	}
	if got := string(texts[0][1].(types.MangleString)); got != "reconcile on boot" {
		t.Errorf("mitigation text = %q, want the operator's own words", got)
	}
}

func TestMitigationStrategyAtom_WhenTextsDiffer_ShouldProduceDistinctAtoms(t *testing.T) {
	a := MitigationStrategyAtom("rotate the signing key every 30 days")
	b := MitigationStrategyAtom("rotate the signing key every 90 days")
	if a == b {
		t.Fatalf("two different mitigations collided on %q", a)
	}
	if MitigationStrategyAtom("same text") != MitigationStrategyAtom("same text") {
		t.Error("atom encoding is not deterministic; kernel facts would churn on every boot")
	}
}

func TestMitigationStrategyAtom_WhenTextIsUnslugable_ShouldStillBeAValidName(t *testing.T) {
	for _, in := range []string{"", "   ", "日本語のみ", "!!!", strings.Repeat("very long mitigation text ", 20)} {
		atom := string(MitigationStrategyAtom(in))
		if !strings.HasPrefix(atom, "/mit_") {
			t.Errorf("input %q produced %q, want a /mit_ prefixed name", in, atom)
		}
		if strings.Count(atom, "/") != 1 || strings.ContainsAny(atom, " \t\n") {
			t.Errorf("input %q produced %q, which is not a valid Mangle name constant", in, atom)
		}
	}
}

func TestToFacts_EveryEmittedPredicate_ShouldBeInTheRetractSet(t *testing.T) {
	// A predicate that ToFacts emits but refreshKernelFacts does not retract
	// survives a vision change and leaves stale facts in the kernel.
	for _, f := range linkedVision().ToFacts() {
		if _, ok := northstarPredicatesMap[f.Predicate]; !ok {
			t.Errorf("predicate %q is emitted but never retracted on vision refresh", f.Predicate)
		}
	}
}
