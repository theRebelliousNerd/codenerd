package perception

import (
	"testing"
)

// TestUnderstandingToIntent_FullActionTable pins the exact (verb, category)
// the adapter emits for every action_type in its system prompt table.
//
// The mapping is a cross-subsystem contract, not a cosmetic choice:
//   - the verb must exist in the taxonomy corpus, which resolves the
//     turn's persona (/shard is fail-closed: unknown verbs compile with
//     no persona atoms at all);
//   - the category gates CodeDOM edits (codedom_edit.mg derives
//     next_action(/edit_element) only for /mutation);
//   - the verb must have an action_mapping in delegation.mg unless its
//     terminal response is prose (see the delegation contract test).
func TestUnderstandingToIntent_FullActionTable(t *testing.T) {
	tr := &UnderstandingTransducer{}
	cases := []struct {
		action   string
		domain   string
		semantic string
		verb     string
		category string
	}{
		{"investigate", "testing", "causation", "/debug", "/query"},
		{"investigate", "security", "causation", "/analyze", "/query"},
		{"implement", "general", "mechanism", "/create", "/mutation"},
		{"modify", "general", "mechanism", "/fix", "/mutation"},
		{"refactor", "architecture", "mechanism", "/refactor", "/mutation"},
		{"verify", "testing", "state", "/test", "/query"},
		{"explain", "general", "definition", "/explain", "/query"},
		{"research", "general", "definition", "/research", "/query"},
		{"configure", "configuration", "mechanism", "/configure", "/mutation"},
		{"attack", "security", "mechanism", "/assault", "/mutation"},
		{"revert", "git", "temporal", "/git", "/mutation"},
		{"review", "security", "state", "/security", "/query"},
		{"review", "general", "state", "/review", "/query"},
		{"remember", "general", "instruction", "/remember", "/instruction"},
		{"forget", "general", "instruction", "/forget", "/instruction"},
		{"chat", "general", "state", "/converse", "/query"},
		{"deploy", "configuration", "mechanism", "/deploy", "/mutation"},
		{"migrate", "dependencies", "mechanism", "/migrate", "/mutation"},
		{"optimize", "performance", "mechanism", "/optimize", "/mutation"},
		{"document", "documentation", "mechanism", "/document", "/mutation"},
		{"benchmark", "performance", "quantification", "/benchmark", "/query"},
		{"profile", "performance", "mechanism", "/profile", "/query"},
		{"audit", "security", "state", "/audit", "/query"},
		{"scaffold", "architecture", "mechanism", "/scaffold", "/mutation"},
		{"lint", "general", "state", "/lint", "/query"},
		{"format", "general", "mechanism", "/format", "/mutation"},
	}
	for _, c := range cases {
		u := &Understanding{
			ActionType:   c.action,
			Domain:       c.domain,
			SemanticType: c.semantic,
			Confidence:   0.9,
		}
		got := tr.understandingToIntent(u)
		if got.Verb != c.verb || got.Category != c.category {
			t.Errorf("action=%q domain=%q: got %s %s, want %s %s",
				c.action, c.domain, got.Category, got.Verb, c.category, c.verb)
		}
	}
}

// TestUnderstandingToIntent_MutationSetMatchesCorpus pins the alignment the
// adapter comment promises: every action the adapter calls /mutation must
// map to a verb the taxonomy corpus also files under /mutation, and the
// preset mirror (categoryForIntentVerb) must agree on the shared verbs.
// A drift here silently changes which turns can edit through CodeDOM.
func TestUnderstandingToIntent_MutationSetMatchesCorpus(t *testing.T) {
	tr := &UnderstandingTransducer{}
	corpusCat := map[string]string{}
	for _, e := range GetVerbCorpus() {
		corpusCat[e.Verb] = e.Category
	}
	mutationActions := []string{
		"implement", "modify", "refactor", "attack", "revert", "configure",
		"migrate", "optimize", "document", "scaffold", "format", "deploy",
	}
	for _, action := range mutationActions {
		u := &Understanding{ActionType: action, Domain: "general", SemanticType: "mechanism"}
		got := tr.understandingToIntent(u)
		if got.Category != "/mutation" {
			t.Errorf("action %q: category %s, want /mutation", action, got.Category)
		}
		cat, ok := corpusCat[got.Verb]
		if !ok {
			t.Errorf("action %q: verb %s missing from taxonomy corpus", action, got.Verb)
			continue
		}
		// /configure is the one deliberate exception: the corpus files it
		// under /instruction (conversational setup guidance) while the
		// adapter treats a classified setup ACTION as a mutation so config
		// edits can flow through the /mutation machinery.
		if got.Verb == "/configure" {
			continue
		}
		if cat != "/mutation" {
			t.Errorf("action %q: verb %s is %s in corpus, want /mutation", action, got.Verb, cat)
		}
	}
}

// TestUnderstandingToIntent_NoOrphanVerb pins that every verb the adapter can
// emit is a member of the taxonomy corpus. Membership (not the shard value)
// is the contract: /shard is a fail-closed regime dimension in
// jit_compiler.mg, so a corpus-known /none verb and a missing verb both
// compile with no persona — but only the known verb is visible to the
// classifier candidates, the firewall's isVerbKnown, and the VERB TAXONOMY
// prompt section. A verb outside the corpus is invisible to all of them.
func TestUnderstandingToIntent_NoOrphanVerb(t *testing.T) {
	tr := &UnderstandingTransducer{}
	inCorpus := map[string]bool{}
	for _, e := range GetVerbCorpus() {
		inCorpus[e.Verb] = true
	}
	actions := []string{
		"investigate", "implement", "modify", "refactor", "verify", "explain",
		"research", "configure", "attack", "revert", "review", "remember",
		"forget", "chat", "deploy", "migrate", "optimize", "document",
		"benchmark", "profile", "audit", "scaffold", "lint", "format",
		"something-the-model-invented",
	}
	seen := map[string]bool{}
	for _, action := range actions {
		for _, domain := range []string{"general", "testing", "security"} {
			u := &Understanding{ActionType: action, Domain: domain, SemanticType: "mechanism"}
			verb := tr.understandingToIntent(u).Verb
			seen[verb] = true
			if !inCorpus[verb] {
				t.Errorf("action %q (domain %q): verb %s is not in the taxonomy corpus",
					action, domain, verb)
			}
		}
	}
	// Fail-closed: unknown actions collapse to /explain, never to a novel verb.
	u := &Understanding{ActionType: "teleport", Domain: "general", SemanticType: "mechanism"}
	got := tr.understandingToIntent(u)
	if got.Verb != "/explain" || got.Category != "/query" {
		t.Errorf("unknown action: got %s %s, want /query /explain", got.Category, got.Verb)
	}
	t.Logf("adapter emits %d distinct verbs, all corpus-covered", len(seen))
}

// TestTaxonomyCorpus_AdapterVerbsResolvePersonas pins the shard half of the
// contract: mutation verbs the adapter emits must resolve a real persona so
// their turns compile with that shard's atoms, while conversational and
// memory verbs must resolve /none (GetShardTypeForVerb reports "" for both
// /none and missing — the membership half above is what tells them apart).
func TestTaxonomyCorpus_AdapterVerbsResolvePersonas(t *testing.T) {
	withPersona := map[string]string{
		"/deploy":  "coder",
		"/migrate": "coder", "/optimize": "coder", "/document": "coder",
		"/scaffold": "coder", "/format": "coder", "/create": "coder",
		"/fix": "coder", "/refactor": "coder", "/git": "coder", "/debug": "coder",
		"/test": "tester", "/benchmark": "tester", "/profile": "tester",
		"/review": "reviewer", "/audit": "reviewer", "/lint": "reviewer",
		"/analyze": "reviewer", "/security": "reviewer",
		"/research": "researcher",
	}
	for verb, want := range withPersona {
		if got := GetShardTypeForVerb(verb); got != want {
			t.Errorf("GetShardTypeForVerb(%s)=%q, want %q", verb, got, want)
		}
	}
	withoutPersona := []string{"/explain", "/converse", "/remember", "/forget", "/configure", "/assault"}
	for _, verb := range withoutPersona {
		if got := GetShardTypeForVerb(verb); got != "" {
			t.Errorf("GetShardTypeForVerb(%s)=%q, want \"\" (/none)", verb, got)
		}
	}
}

// TestUnderstandingToIntent_NormalizesAction pins whitespace/case tolerance:
// the LLM does not always return clean lowercase action strings.
func TestUnderstandingToIntent_NormalizesAction(t *testing.T) {
	tr := &UnderstandingTransducer{}
	u := &Understanding{ActionType: "  IMPLEMENT ", Domain: " general ", SemanticType: "mechanism"}
	got := tr.understandingToIntent(u)
	if got.Verb != "/create" || got.Category != "/mutation" {
		t.Errorf("padded action: got %s %s, want /mutation /create", got.Category, got.Verb)
	}
}

// TestIsInterrogativeSemanticType_QuestionForms pins the backup IsQuestion
// signal against the semantic-type table in the adapter's system prompt:
// recommendation ("Should I ...?") and state ("Is X ...?") are question
// forms. Hypothetical stays out: what-if turns route by mode=dream, and
// instruction stays out: directives are commands.
func TestIsInterrogativeSemanticType_QuestionForms(t *testing.T) {
	tr := &UnderstandingTransducer{}
	questions := []string{
		"definition", "causation", "mechanism", "location", "temporal",
		"attribution", "selection", "existence", "quantification",
		"recommendation", "state",
	}
	for _, s := range questions {
		u := &Understanding{ActionType: "explain", SemanticType: s}
		if got := tr.understandingToIntent(u); !got.IsQuestion {
			t.Errorf("semantic %q: IsQuestion=false, want true (backup signal)", s)
		}
	}
	notQuestions := []string{"hypothetical", "instruction", "nonsense"}
	for _, s := range notQuestions {
		u := &Understanding{ActionType: "explain", SemanticType: s}
		if got := tr.understandingToIntent(u); got.IsQuestion {
			t.Errorf("semantic %q: IsQuestion=true, want false", s)
		}
	}
}

// TestIsBackupQuestionSignal_MutationDominates pins that the WHAT dominates
// the HOW: a mutation action keeps IsQuestion false even with an
// interrogative semantic form, while inspection actions take the backup.
func TestIsBackupQuestionSignal_MutationDominates(t *testing.T) {
	tr := &UnderstandingTransducer{}
	cases := []struct {
		action   string
		semantic string
		want     bool
	}{
		{"modify", "state", false}, // existing contract: action requests stay non-question
		{"implement", "recommendation", false},
		{"optimize", "state", false},
		{"review", "state", true}, // verdict-seeking questions stay questions
		{"investigate", "state", true},
		{"verify", "state", true},
		{"explain", "recommendation", true},
	}
	for _, c := range cases {
		u := &Understanding{ActionType: c.action, SemanticType: c.semantic}
		if got := tr.understandingToIntent(u); got.IsQuestion != c.want {
			t.Errorf("action=%q semantic=%q: IsQuestion=%v, want %v",
				c.action, c.semantic, got.IsQuestion, c.want)
		}
	}
	// The explicit model signal always wins over the dominance rule.
	u := &Understanding{ActionType: "modify", SemanticType: "state", Signals: Signals{IsQuestion: true}}
	if got := tr.understandingToIntent(u); !got.IsQuestion {
		t.Error("explicit IsQuestion=true was overridden by the dominance rule")
	}
}

// TestUnderstandingToIntent_InstructionSemanticWins pins that an instruction
// semantic type forces /instruction even for code-writing actions: "always
// write tests first" is a standing directive, not a mutation turn.
func TestUnderstandingToIntent_InstructionSemanticWins(t *testing.T) {
	tr := &UnderstandingTransducer{}
	u := &Understanding{ActionType: "implement", Domain: "general", SemanticType: "instruction"}
	got := tr.understandingToIntent(u)
	if got.Category != "/instruction" {
		t.Errorf("instruction semantic: category %s, want /instruction", got.Category)
	}
	if got.Verb != "/create" {
		t.Errorf("instruction semantic: verb %s, want /create (verb still follows action)", got.Verb)
	}
}
