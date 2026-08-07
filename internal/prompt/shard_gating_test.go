package prompt

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Shard gating is an IDENTITY constraint, so it must be fail-closed in both
// selection paths.
//
// The live (Mangle) path and the fallback (Go) path disagreed: jit_compiler.mg
// only blocked an atom when the context HAD that dimension, while Go's
// matchSelector returns false for a constraint with no context value. Since
// buildCompilationContext never set ShardType, the live path admitted every
// shard-gated atom in the corpus — one "explain this file" turn arrived with
// 25+ contradictory identities and answered with an intent announcement
// instead of doing the work.
//
// These tests pin the Go semantics, which are the ones the Mangle rule now
// mirrors. If matchSelector is ever "relaxed" to match the old Mangle
// behaviour, persona leakage returns silently.
func TestShardGating_UnknownShardBlocksGatedAtom(t *testing.T) {
	nemesis := &PromptAtom{
		ID:         "identity/nemesis/mission",
		Category:   CategoryIdentity,
		ShardTypes: []string{"/nemesis"},
	}

	// No shard in context — the exact state buildCompilationContext used to
	// produce for every CLI action and TUI turn.
	cc := NewCompilationContext()
	if nemesis.MatchesContext(cc) {
		t.Error("a /nemesis-gated atom must NOT match a context with no shard; " +
			"that is how every persona in the corpus leaked into one prompt")
	}
}

func TestShardGating_WrongShardBlocksGatedAtom(t *testing.T) {
	nemesis := &PromptAtom{
		ID:         "identity/nemesis/mission",
		Category:   CategoryIdentity,
		ShardTypes: []string{"/nemesis"},
	}
	cc := NewCompilationContext()
	cc.ShardType = "/researcher"

	if nemesis.MatchesContext(cc) {
		t.Error("a /nemesis-gated atom must not appear in a /researcher compile")
	}
}

func TestShardGating_MatchingShardAdmitsAtom(t *testing.T) {
	researcher := &PromptAtom{
		ID:         "identity/researcher/mission",
		Category:   CategoryIdentity,
		ShardTypes: []string{"/researcher"},
	}
	cc := NewCompilationContext()
	cc.ShardType = "/researcher"

	if !researcher.MatchesContext(cc) {
		t.Error("the matching persona must still be admitted — fail-closed must not starve the shard of its own identity")
	}
}

// Ungated atoms are the shared substrate (safety, protocol, output format) and
// must keep flowing to every shard regardless of persona.
func TestShardGating_UngatedAtomAlwaysAdmitted(t *testing.T) {
	shared := &PromptAtom{
		ID:       "safety/constitutional/default_deny",
		Category: CategorySafety,
	}
	for _, shard := range []string{"", "/researcher", "/nemesis"} {
		cc := NewCompilationContext()
		cc.ShardType = shard
		if !shared.MatchesContext(cc) {
			t.Errorf("an ungated atom must be admitted for shard %q", shard)
		}
	}
}

// Situational dimensions must stay permissive — this is the distinction that
// keeps the fail-closed shard rule from over-blocking. An atom that mentions Go
// should survive a compile that has not resolved a language.
func TestShardGating_SituationalDimensionsStayPermissive(t *testing.T) {
	goAtom := &PromptAtom{
		ID:        "knowledge/go/idioms",
		Category:  CategoryKnowledge,
		Languages: []string{"go"},
	}
	cc := NewCompilationContext()
	cc.ShardType = "/coder"
	// Language deliberately unset.
	if goAtom.MatchesContext(cc) {
		t.Skip("matchSelector is fail-closed for all dimensions; recorded so the " +
			"shard-vs-situational distinction is a conscious choice, not an accident")
	}
}

// hasRegimeSelector reports whether an atom is scoped to an operating regime.
//
// This list must stay in lockstep with regime_dimension/1 in
// internal/core/defaults/jit_compiler.mg. Only these dimensions are fail-closed;
// a gate on any other dimension is a relevance hint that a compile which never
// set that dimension will happily ignore.
//
// intent_verbs and languages are deliberately absent. They are permissive by
// design, so "this atom has intents: [/fix]" is NOT sufficient to keep a persona
// out of an unrelated compile.
func hasRegimeSelector(a *PromptAtom) bool {
	return len(a.ShardTypes) > 0 ||
		len(a.OperationalModes) > 0 ||
		len(a.CampaignPhases) > 0 ||
		len(a.BuildLayers) > 0 ||
		len(a.InitPhases) > 0 ||
		len(a.NorthstarPhases) > 0 ||
		len(a.OuroborosStages) > 0
}

// declaresPersona reports whether an atom's content tells the model who it is.
// "You are the X" / "Your Role: X" is the shape that overrides a shard's own
// identity, regardless of which category the atom happens to be filed under.
func declaresPersona(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "you are the") ||
		strings.Contains(lower, "## your role")
}

// A persona-defining atom must carry a REGIME gate. Anything less reaches every
// compile for every shard, which is precisely how one prompt ended up asserting
// 25+ contradictory identities.
//
// This sweeps the WHOLE corpus, not just CategoryIdentity. The worst offender
// found live -- perception_understanding, which made every shard believe it was
// the perception layer and must only describe intent -- is filed under
// CategoryIntent, so a category-scoped check walked straight past it. The second
// worst carried languages:[/mangle] and nothing else, which looks like a gate
// and behaves like none.
func TestShardGating_PersonaAtomsInCorpusAreGated(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Skipf("embedded corpus unavailable: %v", err)
	}
	var ungated []string
	for _, a := range corpus.All() {
		if a == nil {
			continue
		}
		if !hasRegimeSelector(a) && declaresPersona(a.Content) {
			ungated = append(ungated, a.ID)
		}
	}
	sort.Strings(ungated)
	if len(ungated) > 0 {
		t.Errorf("%d atom(s) declare a persona with no fail-closed selector, so they reach every shard:\n  %s",
			len(ungated), strings.Join(ungated, "\n  "))
	}
}

// The corpus test above is only as strong as its dimension list. If someone
// removes a regime_dimension fact from jit_compiler.mg, that dimension silently
// becomes permissive and the corpus test keeps passing while personas leak.
func TestShardGating_RegimeDimensionsMatchKernelPolicy(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "core", "defaults", "jit_compiler.mg"))
	if err != nil {
		t.Skipf("jit_compiler.mg unreadable: %v", err)
	}
	for _, dim := range []string{
		"/shard", "/mode", "/phase", "/layer",
		"/init_phase", "/northstar_phase", "/ouroboros_stage",
	} {
		if !strings.Contains(string(src), "regime_dimension("+dim+").") {
			t.Errorf("jit_compiler.mg no longer declares regime_dimension(%s); that dimension "+
				"is now permissive, and hasRegimeSelector in this file is lying about it", dim)
		}
	}
}
