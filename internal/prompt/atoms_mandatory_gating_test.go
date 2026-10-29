package prompt

import (
	"testing"

	"codenerd/internal/core"
)

// personaShardForVerb is the shard dimension a turn for this verb compiles
// with. The catalog and the persona come from the same config atom: its
// policy set is the persona the factory serves, and taxonomy verbs whose
// shard is /none compile with the shard dimension unset (PolicySetBase).
// An unclassifiable verb fails the test; skipping it would hide a gate.
func personaShardForVerb(t *testing.T, provider *DefaultConfigAtomProvider, verb string) string {
	t.Helper()
	atom, ok := provider.GetAtom(verb)
	if !ok {
		t.Fatalf("no config atom for registered verb %s", verb)
	}
	got := make(map[string]bool, len(atom.Policies))
	for _, p := range atom.Policies {
		got[p] = true
	}
	var matched []string
	for _, setID := range []string{
		core.PolicySetCoder, core.PolicySetTester, core.PolicySetReviewer,
		core.PolicySetResearcher, core.PolicySetNemesis,
		core.PolicySetToolGenerator, core.PolicySetBase,
	} {
		files, ok := core.DefaultAgentPolicySetFiles(setID)
		if !ok || len(files) != len(got) {
			continue
		}
		same := true
		for _, f := range files {
			if !got[f] {
				same = false
				break
			}
		}
		if same {
			matched = append(matched, setID)
		}
	}
	if len(matched) != 1 {
		t.Fatalf("verb %s policies match %v, want exactly one agent policy set", verb, matched)
	}
	// /none verbs (explain, read, general) compile with no shard dimension.
	// matchSelector treats an empty value as "no match" against a constraint
	// and "match" against an empty selector, which is that compile.
	if matched[0] == core.PolicySetBase {
		return ""
	}
	return "/" + matched[0]
}

// catalogIndependentGuidance is the guidance that has to survive every tool
// catalog. The property is the id prefix safety/ on a mandatory atom.
// capability/codedom_safety is category capability, so the prefix does not
// reach it; it is the one named exception. The edit_file/write_file teaching
// lives on capability/codedom_whole_file, which is tool-gated on purpose.
func catalogIndependentGuidance(a *PromptAtom) bool {
	if a == nil || !a.IsMandatory {
		return false
	}
	if len(a.ID) >= len("safety/") && a.ID[:len("safety/")] == "safety/" {
		return true
	}
	return a.ID == "capability/codedom_safety"
}

// blockingDependency walks depends_on. A parent with requires_tools is dropped
// by the inclusion gate, base_prohibited propagates through atom_requires, and
// the child is dropped with it. The same drop happens through a chain.
func blockingDependency(a *PromptAtom, byID map[string]*PromptAtom, seen map[string]bool) (id, why string) {
	if a == nil {
		return "", ""
	}
	for _, dep := range a.DependsOn {
		if seen[dep] {
			continue
		}
		seen[dep] = true
		parent, ok := byID[dep]
		if !ok {
			return dep, "missing"
		}
		if len(parent.RequiresTools) > 0 {
			return parent.ID, "requires_tools"
		}
		if id, why := blockingDependency(parent, byID, seen); why != "" {
			return id, why
		}
	}
	return "", ""
}

// selectorFields names every selector dimension that can keep an atom off a
// compile. shard_types is reported separately: one safety atom uses it as a
// persona cost fence.
func selectorFields(a *PromptAtom) []string {
	type field struct {
		name   string
		values []string
	}
	fields := []field{
		{"intent_verbs", a.IntentVerbs},
		{"operational_modes", a.OperationalModes},
		{"campaign_phases", a.CampaignPhases},
		{"build_layers", a.BuildLayers},
		{"init_phases", a.InitPhases},
		{"northstar_phases", a.NorthstarPhases},
		{"ouroboros_stages", a.OuroborosStages},
		{"languages", a.Languages},
		{"frameworks", a.Frameworks},
		{"providers", a.Providers},
		{"models", a.Models},
		{"world_states", a.WorldStates},
	}
	var set []string
	for _, f := range fields {
		if len(f.values) > 0 {
			set = append(set, f.name)
		}
	}
	return set
}

// TestCapabilityAtoms_CatalogIndependentGuidanceIsNeverToolGated pins the
// inclusion-gate bug class from f704e20d. requires_tools drops an atom when
// the turn's catalog lacks a listed tool, before the atom can enter the
// prompt. Guidance that has to be present on every catalog carries an empty
// requires_tools and no depends_on edge onto an atom that has one.
//
// The set is derived from the id prefix, not a hand list of atom ids, and
// each member is admitted for every registered verb and persona. Tool-teaching
// mandatory atoms stay out of the set: capability/codedom_core, codedom_tools,
// structure_queries and codedom_impact keep empty selectors, and requires_tools
// drops them exactly where a catalog lacks a tool. An intent_verbs list copied
// from today's factory catalogs duplicates that gate and leaves out a verb
// registered later.
//
// safety/honesty/no_unverified_claims is mandatory and under safety/, so the
// tool-gate checks cover it. Its shard_types fence (claim_honesty.yaml: the
// cost stays on the shards that write and review code) is the one persona
// exception. It is still admitted for every verb of those shards.
func TestCapabilityAtoms_CatalogIndependentGuidanceIsNeverToolGated(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	byID := make(map[string]*PromptAtom)
	for _, a := range corpus.All() {
		byID[a.ID] = a
	}
	provider := NewDefaultConfigAtomProvider()
	verbs, catalogs := catalogsByVerb(t)
	if len(verbs) < 40 {
		t.Fatalf("only %d registered verbs; the factory catalog walk collapsed", len(verbs))
	}

	// Widening the reviewer catalog with edit_file would silence the gate
	// without restoring the contract. The reviewer steering says not to.
	if catalogs["/review"]["edit_file"] || catalogs["/review"]["write_file"] {
		t.Fatalf("/review catalog grants edit_file or write_file; do not widen authority so a prompt gate can pass")
	}

	selection := byID["capability/codedom_selection"]
	if selection == nil {
		t.Fatal("capability/codedom_selection missing from corpus")
	}
	if !selection.IsMandatory || !matchSelector(selection.IntentVerbs, "/review") || !matchSelector(selection.ShardTypes, "/reviewer") {
		t.Errorf("capability/codedom_selection is not admitted for /review (mandatory=%v intent=%v shard=%v)",
			selection.IsMandatory, selection.IntentVerbs, selection.ShardTypes)
	}
	for _, tool := range selection.RequiresTools {
		if tool == "edit_file" || tool == "write_file" {
			t.Errorf("capability/codedom_selection requires %s; those names belong on capability/codedom_whole_file", tool)
		}
		if !catalogs["/review"][tool] {
			t.Errorf("capability/codedom_selection requires %s, which /review withholds, so the inclusion gate drops the atom", tool)
		}
	}
	whole := byID["capability/codedom_whole_file"]
	if whole == nil || !whole.IsMandatory {
		t.Fatal("capability/codedom_whole_file missing or not mandatory")
	}
	held := map[string]bool{}
	for _, tool := range whole.RequiresTools {
		held[tool] = true
	}
	if !held["edit_file"] || !held["write_file"] {
		t.Errorf("capability/codedom_whole_file requires_tools = %v, want edit_file and write_file", whole.RequiresTools)
	}
	reviewAvail := make(map[string]struct{}, len(catalogs["/review"]))
	for tool := range catalogs["/review"] {
		reviewAvail[tool] = struct{}{}
	}
	if atomToolSatisfied(whole, reviewAvail) {
		t.Error("capability/codedom_whole_file is satisfied by the /review catalog; whole-file names would reach a catalog that does not grant them")
	}

	// These four teach tools. Empty intent_verbs lets a later verb receive
	// them as soon as its catalog holds requires_tools. A copied verb list
	// does not.
	for _, id := range []string{
		"capability/codedom_core",
		"capability/codedom_tools",
		"capability/structure_queries",
		"capability/codedom_impact",
	} {
		a := byID[id]
		if a == nil || !a.IsMandatory {
			t.Errorf("%s missing or not mandatory", id)
			continue
		}
		if len(a.IntentVerbs) != 0 {
			t.Errorf("%s intent_verbs = %v; the list duplicates requires_tools and leaves out a verb registered later", id, a.IntentVerbs)
		}
	}

	const honestyID = "safety/honesty/no_unverified_claims"
	shards := make(map[string]string, len(verbs))
	for _, verb := range verbs {
		shards[verb] = personaShardForVerb(t, provider, verb)
	}

	var always []string
	for _, a := range corpus.All() {
		// Category safety is also used by shard-local atoms (campaign/*, shards/*,
		// system/*). Those are persona guidance. The catalog-independent set is
		// the safety/ id prefix.
		if !catalogIndependentGuidance(a) {
			continue
		}
		always = append(always, a.ID)
		if len(a.RequiresTools) != 0 {
			t.Errorf("%s carries requires_tools %v; the inclusion gate drops it wherever a tool is missing", a.ID, a.RequiresTools)
		}
		if id, why := blockingDependency(a, byID, map[string]bool{a.ID: true}); why != "" {
			t.Errorf("%s depends on %s (%s); a pruned parent drops this atom", a.ID, id, why)
		}
		leaks := selectorFields(a)
		if a.ID == honestyID {
			if len(leaks) != 0 {
				t.Errorf("%s selectors %v; the persona fence is shard_types only", a.ID, leaks)
			}
			wantShards := []string{"coder", "tester", "reviewer", "researcher"}
			if len(a.ShardTypes) != len(wantShards) {
				t.Errorf("%s shard_types = %v, want %v", a.ID, a.ShardTypes, wantShards)
			}
			for _, shard := range wantShards {
				if !matchSelector(a.ShardTypes, shard) {
					t.Errorf("%s shard_types %v missing %s", a.ID, a.ShardTypes, shard)
				}
			}
			continue
		}
		if len(a.ShardTypes) != 0 {
			leaks = append(leaks, "shard_types")
		}
		if len(leaks) != 0 {
			t.Errorf("%s selectors %v; catalog-independent guidance is admitted for every registered verb and persona", a.ID, leaks)
		}
	}
	if len(always) < 7 {
		t.Fatalf("always-present set has %d atoms (%v); safety/ no longer sees the corpus", len(always), always)
	}
	for _, id := range []string{"safety/constitutional/core", "safety/constitution/prime_directive", honestyID, "capability/codedom_safety"} {
		if byID[id] == nil || !catalogIndependentGuidance(byID[id]) {
			t.Errorf("%s is not in the always-present set", id)
		}
	}

	honestyIncluded, honestyExcluded := 0, 0
	for _, verb := range verbs {
		shard := shards[verb]
		persona := shard
		if persona == "" {
			persona = "(none)"
		}
		for _, id := range always {
			a := byID[id]
			if a.ID == honestyID {
				if !matchSelector(a.IntentVerbs, verb) {
					t.Errorf("%s intent_verbs exclude %s; the cost fence is shard_types only", a.ID, verb)
				}
				if matchSelector(a.ShardTypes, shard) {
					honestyIncluded++
				} else {
					honestyExcluded++
				}
				continue
			}
			if !matchSelector(a.IntentVerbs, verb) || !matchSelector(a.ShardTypes, shard) {
				t.Errorf("%s is not admitted for %s (persona %s)", a.ID, verb, persona)
			}
		}
	}
	if honestyIncluded == 0 || honestyExcluded == 0 {
		t.Errorf("honesty persona fence included %d verb compiles and excluded %d; expected both", honestyIncluded, honestyExcluded)
	}
	// The pair the inclusion gate was shown to drop. Safety has to be on
	// /review. whole_file must not be, because /review lacks its tools and
	// its verb list.
	safety := byID["capability/codedom_safety"]
	if !matchSelector(safety.IntentVerbs, "/review") || !matchSelector(safety.ShardTypes, "/reviewer") {
		t.Error("capability/codedom_safety is not admitted for /review")
	}
	if matchSelector(whole.IntentVerbs, "/review") && atomToolSatisfied(whole, reviewAvail) {
		t.Error("capability/codedom_whole_file is admitted and tool-satisfied for /review")
	}
	t.Logf("always-present atoms: %v", always)
}
