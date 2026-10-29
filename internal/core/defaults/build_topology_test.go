package defaults

import (
	"os"
	"testing"

	"codenerd/internal/mangle"
)

// build_topology.mg had no live-engine coverage, which is how both of its
// defects survived: every campaign run reported them and every campaign run
// proceeded anyway.
//
// F-TOPO-1: /research, /test and /ops were accepted by the Go normalizer
// (campaign.allowedPhaseCategories) but absent from build_phase_type, so those
// phases derived no phase_precedence, has_phase_category/1 was false, and the
// kernel raised "missing_category" against a valid plan.
//
// F-TOPO-2: suspicious_gap fired on ScoreDown > ScoreUp -- which is exactly a
// correctly ordered dependency -- so task_topology_warning "skips_layer" was
// raised for every well-formed task in every campaign.
func loadTopologyEngine(t *testing.T, edb string) *mangle.Engine {
	t.Helper()

	eng, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	t.Cleanup(func() { eng.Close() })

	for _, f := range []string{"schemas_campaign.mg", "build_topology.mg"} {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if err := eng.LoadSchemaString(string(content)); err != nil {
			t.Fatalf("load %s: %v", f, err)
		}
	}

	if edb != "" {
		if err := eng.LoadSchemaString(edb); err != nil {
			t.Fatalf("load EDB: %v", err)
		}
	}

	// LoadSchemaString installs clauses; it does not run the fixpoint. Without
	// this every derived predicate reads as empty, which would make these tests
	// pass for the wrong reason.
	if err := eng.RecomputeRules(); err != nil {
		t.Fatalf("recompute rules: %v", err)
	}
	return eng
}

// derivedFirstArgs returns the first argument of every fact of a predicate.
func derivedFirstArgs(t *testing.T, eng *mangle.Engine, predicate string) map[string]bool {
	t.Helper()

	out := map[string]bool{}
	facts, err := eng.GetFacts(predicate)
	if err != nil {
		// An underived predicate is empty, not an error condition for these tests.
		return out
	}
	for _, f := range facts {
		if len(f.Args) == 0 {
			continue
		}
		if s, ok := f.Args[0].(string); ok {
			out[s] = true
		}
	}
	return out
}

// The exact shape the degraded scaffold produces: research -> scaffold -> test.
// Two of its three phases used to report missing_category.
func TestBuildTopology_ScaffoldPhasesAllHaveCategory(t *testing.T) {
	eng := loadTopologyEngine(t, `
phase_category("p_research", /research).
phase_category("p_impl", /scaffold).
phase_category("p_test", /test).
campaign_phase("p_research", "c1", "Research & Planning", 0, "/pending", "profile").
campaign_phase("p_impl", "c1", "Implementation", 1, "/pending", "profile").
campaign_phase("p_test", "c1", "Testing & Review", 2, "/pending", "profile").
`)

	has := derivedFirstArgs(t, eng, "has_phase_category")
	for _, phase := range []string{"p_research", "p_impl", "p_test"} {
		if !has[phase] {
			t.Errorf("has_phase_category(%q) did not derive; its category has no build_phase_type entry, "+
				"so the kernel reports missing_category against a valid plan", phase)
		}
	}

	if errs := derivedFirstArgs(t, eng, "validation_error"); len(errs) != 0 {
		t.Errorf("validation_error derived for %v; a plan whose phases all carry known categories "+
			"must validate clean", errs)
	}
}

// Every category the Go normalizer can emit must order against every other.
func TestBuildTopology_EveryAllowedCategoryDerivesPrecedence(t *testing.T) {
	eng := loadTopologyEngine(t, `
phase_category("p1", /research).
phase_category("p2", /scaffold).
phase_category("p3", /domain_core).
phase_category("p4", /data_layer).
phase_category("p5", /service).
phase_category("p6", /transport).
phase_category("p7", /integration).
phase_category("p8", /test).
phase_category("p9", /ops).
`)

	prec := derivedFirstArgs(t, eng, "phase_precedence")
	for _, phase := range []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9"} {
		if !prec[phase] {
			t.Errorf("phase_precedence(%q, _) did not derive", phase)
		}
	}
}

// A dependency on the immediately preceding layer is the normal, correct case.
// It used to raise suspicious_gap, which raised task_topology_warning for every
// task in the phase.
func TestBuildTopology_AdjacentDependencyIsNotSuspicious(t *testing.T) {
	eng := loadTopologyEngine(t, `
phase_category("p_domain", /domain_core).
phase_category("p_data", /data_layer).
phase_dependency("p_data", "p_domain", /hard).
`)

	if gaps := derivedFirstArgs(t, eng, "suspicious_gap"); len(gaps) != 0 {
		t.Errorf("suspicious_gap derived %v for a data_layer -> domain_core dependency; "+
			"that is adjacent and correctly ordered", gaps)
	}
	if v := derivedFirstArgs(t, eng, "architectural_violation"); len(v) != 0 {
		t.Errorf("architectural_violation derived %v for a correctly ordered dependency", v)
	}
}

// Skipping two or more layers is what the warning is actually for.
func TestBuildTopology_LargeSkipIsSuspicious(t *testing.T) {
	eng := loadTopologyEngine(t, `
phase_category("p_scaffold", /scaffold).
phase_category("p_integration", /integration).
phase_dependency("p_integration", "p_scaffold", /hard).
`)

	gaps := derivedFirstArgs(t, eng, "suspicious_gap")
	if !gaps["p_integration"] {
		t.Error("suspicious_gap did not derive for integration -> scaffold, which skips " +
			"domain_core, data_layer, service and transport")
	}
}

// Depending on a later layer is the inversion the whole file exists to catch.
func TestBuildTopology_InvertedDependencyIsAViolation(t *testing.T) {
	eng := loadTopologyEngine(t, `
phase_category("p_domain", /domain_core).
phase_category("p_transport", /transport).
phase_dependency("p_domain", "p_transport", /hard).
`)

	v := derivedFirstArgs(t, eng, "architectural_violation")
	if !v["p_domain"] {
		t.Error("architectural_violation did not derive for domain_core depending on transport")
	}
}

// phase_synonym exists so an LLM answering "testing" still sorts correctly. It
// was unreachable for years because Go normalization ran first and collapsed
// every alias to /service; campaign.phaseCategorySynonyms now resolves them, and
// this asserts the kernel half still works for anything that reaches it raw.
func TestBuildTopology_SynonymsResolveToALayer(t *testing.T) {
	eng := loadTopologyEngine(t, `
phase_category("p_alias", "testing").
`)

	if !derivedFirstArgs(t, eng, "phase_precedence")["p_alias"] {
		t.Error(`phase_precedence did not derive from the alias "testing" via phase_synonym`)
	}
}
