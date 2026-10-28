package prompt_evolution

import (
	"context"
	"errors"
	"testing"
)

func TestPinScopeValid(t *testing.T) {
	for _, s := range AllPinScopes() {
		if !s.Valid() {
			t.Errorf("AllPinScopes returned invalid scope %q", s)
		}
	}
	for _, s := range []PinScope{"", "exact", "MODEL", "family"} {
		if s.Valid() {
			t.Errorf("scope %q should be invalid", s)
		}
	}
}

func TestPinSelectorsByScope(t *testing.T) {
	pin := ServingPin{Provider: "Anthropic", Model: "anthropic/claude-opus-4-20260501"}

	tests := []struct {
		scope         PinScope
		wantProviders []string
		wantModels    []string
	}{
		{PinScopeModel, []string{"anthropic"}, []string{"claude_opus_4_20260501"}},
		{PinScopeModelFamily, []string{"anthropic"}, []string{"claude_opus_4"}},
		{PinScopeProvider, []string{"anthropic"}, nil},
	}

	for _, tc := range tests {
		t.Run(string(tc.scope), func(t *testing.T) {
			providers, models := pinSelectors(tc.scope, pin)

			if !equalStrings(providers, tc.wantProviders) {
				t.Errorf("providers = %v, want %v", providers, tc.wantProviders)
			}
			if !equalStrings(models, tc.wantModels) {
				t.Errorf("models = %v, want %v", models, tc.wantModels)
			}
		})
	}
}

// Rule 1: the default is the exact model, and family is an explicit opt-in.
func TestDefaultPinScopeIsExactModel(t *testing.T) {
	if DefaultPinScope != PinScopeModel {
		t.Fatalf("DefaultPinScope = %q, want %q", DefaultPinScope, PinScopeModel)
	}
	if PinScope("none").Valid() {
		t.Error("there must be no scope that leaves atoms unpinned")
	}
}

// Rule 3: no pin, no atom. Each scope refuses evidence missing what it needs,
// with a typed error.
func TestRequirePinRefusesEvidenceWithoutPin(t *testing.T) {
	full := ServingPin{Provider: "anthropic", Model: "claude-opus-4-20260501"}
	tests := []struct {
		name    string
		scope   PinScope
		role    string
		pin     ServingPin
		missing string
	}{
		{"no pin at all", PinScopeModel, "/coder", ServingPin{}, "provider"},
		{"no model, exact scope", PinScopeModel, "/coder", ServingPin{Provider: "anthropic"}, "model"},
		{"no model, family scope", PinScopeModelFamily, "/coder", ServingPin{Provider: "anthropic"}, "model"},
		{"no provider", PinScopeModel, "/coder", ServingPin{Model: "claude-opus-4"}, "provider"},
		{"no role", PinScopeModel, " ", full, "role"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := requirePin(tc.scope, tc.role, tc.pin)
			if !errors.Is(err, ErrUnpinnedEvidence) {
				t.Fatalf("err = %v, want ErrUnpinnedEvidence", err)
			}
			var typed *UnpinnedEvidenceError
			if !errors.As(err, &typed) || typed.Missing != tc.missing {
				t.Errorf("err = %#v, want Missing=%q", err, tc.missing)
			}
		})
	}
	if err := requirePin(PinScopeProvider, "/coder", ServingPin{Provider: "anthropic"}); err != nil {
		t.Errorf("provider scope needs no model: %v", err)
	}
	if err := requirePin(PinScopeModel, "/coder", full); err != nil {
		t.Errorf("full pin refused: %v", err)
	}
}

type scriptedLLM struct {
	response string
	calls    int
}

func (l *scriptedLLM) Complete(_ context.Context, _ string) (string, error) {
	l.calls++
	return l.response, nil
}

func (l *scriptedLLM) CompleteWithSystem(_ context.Context, _, _ string) (string, error) {
	l.calls++
	return l.response, nil
}

const widenedAtomYAML = "```yaml\n" +
	"- id: evolved/methodology/check_errors\n" +
	"  category: methodology\n" +
	"  shard_types: [\"/coder\", \"/tester\", \"/reviewer\"]\n" +
	"  content: |\n" +
	"    Always check the error.\n" +
	"```\n"

func oneFailure() []*JudgeVerdict {
	return []*JudgeVerdict{{TaskID: "t1", Verdict: "FAIL", Explanation: "ignored an error"}}
}

// Rules 1 and 2 end to end: the generated atom is pinned to the exact model
// and to the source role only, whatever roles the model suggested.
func TestGenerateFromFailuresPinsExactModelAndSourceRole(t *testing.T) {
	llm := &scriptedLLM{response: widenedAtomYAML}
	ag := NewAtomGenerator(llm, nil)

	atoms, err := ag.GenerateFromFailures(context.Background(), oneFailure(), "/coder",
		string(ProblemDebugging), ServingPin{Provider: "anthropic", Model: "claude-opus-4-20260501"})
	if err != nil || len(atoms) != 1 {
		t.Fatalf("atoms=%d err=%v", len(atoms), err)
	}
	atom := atoms[0].Atom
	if !equalStrings(atom.Models, []string{"claude_opus_4_20260501"}) {
		t.Errorf("Models = %v, want the exact model", atom.Models)
	}
	if !equalStrings(atom.Providers, []string{"anthropic"}) {
		t.Errorf("Providers = %v", atom.Providers)
	}
	if !equalStrings(atom.ShardTypes, []string{"coder"}) {
		t.Errorf("ShardTypes = %v, want exactly the source role", atom.ShardTypes)
	}
	if atoms[0].PinScope != PinScopeModel {
		t.Errorf("PinScope = %q, want %q", atoms[0].PinScope, PinScopeModel)
	}
}

// Rule 3 end to end: no model call is bought, and the error is typed.
func TestGenerateFromFailuresRefusesUnpinnedEvidence(t *testing.T) {
	llm := &scriptedLLM{response: widenedAtomYAML}
	ag := NewAtomGenerator(llm, nil)

	atoms, err := ag.GenerateFromFailures(context.Background(), oneFailure(), "/coder",
		string(ProblemDebugging), ServingPin{})
	if !errors.Is(err, ErrUnpinnedEvidence) || atoms != nil {
		t.Fatalf("atoms=%v err=%v, want ErrUnpinnedEvidence", atoms, err)
	}
	if llm.calls != 0 {
		t.Errorf("model was called %d times for unpinnable evidence", llm.calls)
	}
}

// The generated atom must carry selectors in the canonical vocabulary, since an
// atom pinned in raw vendor spelling would look pinned and never match.
func TestConvertToPromptAtomAppliesPin(t *testing.T) {
	ag := &AtomGenerator{pinScope: PinScopeModelFamily}
	def := atomDefinition{
		ID:       "evolved/test",
		Category: "methodology",
		Content:  "Always check the error.",
	}

	atom := ag.convertToPromptAtom(def, "/coder", ServingPin{
		Provider: "anthropic",
		Model:    "claude-opus-4-20260501",
	})
	if atom == nil {
		t.Fatal("convertToPromptAtom returned nil")
	}

	if !equalStrings(atom.Providers, []string{"anthropic"}) {
		t.Errorf("atom.Providers = %v, want [anthropic]", atom.Providers)
	}
	if !equalStrings(atom.Models, []string{"claude_opus_4"}) {
		t.Errorf("atom.Models = %v, want [claude_opus_4]", atom.Models)
	}
}

// Grouping granularity must equal pin granularity, or one atom is generated
// from failures spanning two models and pinned to whichever came first.
func TestGroupKeyMatchesPinGranularity(t *testing.T) {
	may := &ExecutionRecord{
		ProblemType: "debugging", ShardType: "/coder",
		Provider: "anthropic", Model: "claude-opus-4-20260501",
	}
	sept := &ExecutionRecord{
		ProblemType: "debugging", ShardType: "/coder",
		Provider: "anthropic", Model: "claude-opus-4-20260901",
	}
	other := &ExecutionRecord{
		ProblemType: "debugging", ShardType: "/coder",
		Provider: "openai", Model: "gpt-4o",
	}

	// Family scope: two snapshots of one model share a group.
	if GroupKeyFor(may, PinScopeModelFamily) != GroupKeyFor(sept, PinScopeModelFamily) {
		t.Error("family scope split two snapshots of one model into different groups")
	}
	// Exact scope: they must not.
	if GroupKeyFor(may, PinScopeModel) == GroupKeyFor(sept, PinScopeModel) {
		t.Error("model scope merged two distinct snapshots")
	}
	// Different vendors never share a group unless pinning is off.
	for _, scope := range AllPinScopes() {
		if GroupKeyFor(may, scope) == GroupKeyFor(other, scope) {
			t.Errorf("scope %q merged two providers", scope)
		}
	}
}

// The group's token and the resulting atom's selector must be equal by
// construction; this pins that invariant down rather than trusting it.
func TestGroupTokenEqualsGeneratedSelector(t *testing.T) {
	rec := &ExecutionRecord{
		ProblemType: "debugging", ShardType: "/coder",
		Provider: "Anthropic", Model: "anthropic/claude-opus-4-20260501",
	}

	for _, scope := range []PinScope{PinScopeModelFamily, PinScopeModel, PinScopeProvider} {
		key := GroupKeyFor(rec, scope)
		providers, models := pinSelectors(scope, servingPinFor([]*ExecutionRecord{rec}))

		gotProvider := ""
		if len(providers) > 0 {
			gotProvider = providers[0]
		}
		if gotProvider != key.Provider {
			t.Errorf("scope %q: atom provider %q != group provider %q", scope, gotProvider, key.Provider)
		}

		gotModel := ""
		if len(models) > 0 {
			gotModel = models[0]
		}
		if gotModel != key.Model {
			t.Errorf("scope %q: atom model %q != group model %q", scope, gotModel, key.Model)
		}
	}
}

func TestServingPinForSkipsRecordsWithoutProvenance(t *testing.T) {
	pin := servingPinFor([]*ExecutionRecord{
		nil,
		{ProblemType: "debugging"},
		{Provider: "anthropic", Model: "claude-opus-4"},
	})

	if pin.Provider != "anthropic" || pin.Model != "claude-opus-4" {
		t.Errorf("servingPinFor = %+v, want anthropic/claude-opus-4", pin)
	}

	if got := servingPinFor(nil); !got.IsZero() {
		t.Errorf("servingPinFor(nil) = %+v, want zero", got)
	}
}

// An omitted scope must default rather than silently disabling pinning.
func TestEvolverConfigPinScopeDefaults(t *testing.T) {
	cfg := DefaultEvolverConfig()
	cfg.AtomPinScope = ""

	validated, err := validateEvolverConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if validated.AtomPinScope != PinScopeModel {
		t.Errorf("AtomPinScope = %q, want %q", validated.AtomPinScope, PinScopeModel)
	}
}

func TestEvolverConfigRejectsUnknownPinScope(t *testing.T) {
	cfg := DefaultEvolverConfig()
	cfg.AtomPinScope = "sometimes"

	if _, err := validateEvolverConfig(cfg); err == nil {
		t.Fatal("expected an error for an unknown pin scope")
	}
}

func TestNewAtomGeneratorDefaultsToExactModel(t *testing.T) {
	if ag := NewAtomGenerator(nil, nil); ag.pinScope != PinScopeModel {
		t.Errorf("pinScope = %q, want %q", ag.pinScope, PinScopeModel)
	}
	// An unknown scope falls back to the default, never to a looser one.
	if ag := NewAtomGeneratorWithPinScope(nil, nil, "bogus"); ag.pinScope != PinScopeModel {
		t.Errorf("pinScope = %q, want fallback %q", ag.pinScope, PinScopeModel)
	}
	// Family stays available when asked for.
	if ag := NewAtomGeneratorWithPinScope(nil, nil, PinScopeModelFamily); ag.pinScope != PinScopeModelFamily {
		t.Errorf("explicit family scope lost: %q", ag.pinScope)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
