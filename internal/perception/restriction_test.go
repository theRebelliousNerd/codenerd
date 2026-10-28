package perception

import (
	"context"
	"slices"
	"testing"
)

func kindsOf(r Restriction) []RestrictionKind { return r.Kinds() }

func TestRestrictionFrom_RecognisesTheSupportedVocabulary(t *testing.T) {
	cases := []struct {
		name string
		u    Understanding
		want []RestrictionKind
	}{
		{"exact tokens", Understanding{UserConstraints: []string{"no_changes", "dry_run"}}, []RestrictionKind{RestrictDryRun, RestrictNoChanges}},
		{"spellings the model emits", Understanding{UserConstraints: []string{"Read-Only", "dry run", "NO CHANGES"}}, []RestrictionKind{RestrictDryRun, RestrictNoChanges, RestrictReadOnly}},
		{"hypothetical signal", Understanding{Signals: Signals{IsHypothetical: true}}, []RestrictionKind{RestrictHypothetical}},
		{"confirmation signal", Understanding{Signals: Signals{RequiresConfirmation: true}}, []RestrictionKind{RestrictConfirmFirst}},
		{"confirm_first constraint", Understanding{UserConstraints: []string{"confirm_first"}}, []RestrictionKind{RestrictConfirmFirst}},
		{"none", Understanding{}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := kindsOf(RestrictionFrom(&tc.u))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("kinds = %v, want %v", got, tc.want)
			}
		})
	}
	if !RestrictionFrom(nil).IsZero() {
		t.Fatal("a nil understanding carries no restriction")
	}
}

// Prose outside the vocabulary is kept for the model and the user, and is
// never reported as an enforced obligation.
func TestRestrictionFrom_UnsupportedProseIsNotEnforced(t *testing.T) {
	r := RestrictionFrom(&Understanding{UserConstraints: []string{"keep it simple", "don't touch the API"}})
	if r.Enforced() {
		t.Fatalf("prose was labelled enforced: %v", r.Kinds())
	}
	if r.IsZero() {
		t.Fatal("unrecognised constraints must stay visible as unenforced, not vanish")
	}
	if got := r.Unenforced(); len(got) != 2 {
		t.Fatalf("unenforced = %v", got)
	}
	if r.Text() != "keep it simple; don't touch the API" {
		t.Fatalf("text = %q", r.Text())
	}
}

func TestRestriction_IsAnImmutableValue(t *testing.T) {
	r := RestrictionFrom(&Understanding{UserConstraints: []string{"no_changes"}})
	got := r.Kinds()
	got[0] = RestrictConfirmFirst
	if !slices.Equal(r.Kinds(), []RestrictionKind{RestrictNoChanges}) {
		t.Fatalf("mutating the returned slice changed the restriction: %v", r.Kinds())
	}
	if r.Digest() != RestrictionFrom(&Understanding{UserConstraints: []string{"no_changes"}}).Digest() {
		t.Fatal("equal restrictions must share a digest")
	}
	if r.Digest() == RestrictionFrom(&Understanding{UserConstraints: []string{"dry_run"}}).Digest() {
		t.Fatal("a different restriction must not share a digest")
	}
}

func TestRestriction_MergeOnlyAdds(t *testing.T) {
	parent := RestrictionFrom(&Understanding{UserConstraints: []string{"no_changes"}})
	child := RestrictionFrom(&Understanding{Signals: Signals{RequiresConfirmation: true}})
	if got := kindsOf(parent.Merge(Restriction{})); !slices.Equal(got, []RestrictionKind{RestrictNoChanges}) {
		t.Fatalf("merging nothing dropped the parent's kind: %v", got)
	}
	if got := kindsOf(Restriction{}.Merge(parent)); !slices.Equal(got, []RestrictionKind{RestrictNoChanges}) {
		t.Fatalf("zero merged with parent: %v", got)
	}
	merged := parent.Merge(child)
	if !merged.Has(RestrictNoChanges) || !merged.Has(RestrictConfirmFirst) {
		t.Fatalf("a child lost a kind: %v", merged.Kinds())
	}
}

// A context-carried restriction is how a delegation inherits it; a derived
// context can add to it and cannot lift it.
func TestWithRestriction_ChildContextCannotLiftTheParents(t *testing.T) {
	parent := WithRestriction(context.Background(), RestrictionFrom(&Understanding{UserConstraints: []string{"read_only"}}))
	lifted := WithRestriction(parent, Restriction{})
	if !RestrictionFromContext(lifted).Has(RestrictReadOnly) {
		t.Fatal("deriving a context with no restriction lifted the parent's")
	}
	grown := WithRestriction(parent, RestrictionFrom(&Understanding{Signals: Signals{RequiresConfirmation: true}}))
	got := RestrictionFromContext(grown)
	if !got.Has(RestrictReadOnly) || !got.Has(RestrictConfirmFirst) {
		t.Fatalf("grown restriction = %v", got.Kinds())
	}
	if RestrictionFromContext(parent).Has(RestrictConfirmFirst) {
		t.Fatal("growing a child changed the parent's context")
	}
	if !RestrictionFromContext(context.Background()).IsZero() {
		t.Fatal("a bare context carries no restriction")
	}
}

func hypotheticalModify() *Understanding {
	return &Understanding{
		PrimaryIntent: "analyze", SemanticType: "hypothetical", ActionType: "modify", Domain: "general",
		Scope:           Scope{Level: "function", Target: "retryLoop"},
		UserConstraints: []string{"confirm_first"},
		Signals:         Signals{IsHypothetical: true},
		Confidence:      0.9,
	}
}

// The conversion every transducer uses carries the recognised limit with the
// intent: before it did, is_hypothetical and requires_confirmation stopped at
// the Understanding and the executable Intent knew nothing of them.
func TestUnderstandingToIntent_CarriesTheRestriction(t *testing.T) {
	base := NewUnderstandingTransducer(nil).(*UnderstandingTransducer)
	intent := base.understandingToIntent(hypotheticalModify())
	if intent.Category != "/mutation" {
		// The compatibility projection is a mutation: the hypothetical is a
		// separate fact about it, and the restriction is what carries it.
		t.Fatalf("category = %q; the test premise is a modify classified as a mutation", intent.Category)
	}
	if !intent.Restriction.Has(RestrictHypothetical) || !intent.Restriction.Has(RestrictConfirmFirst) {
		t.Fatalf("the hypothetical/confirm limits did not reach the Intent: %v", intent.Restriction.Kinds())
	}
}

// The Gemini transducer converts through the same function; it must not be a
// second path that drops the limit.
func TestGeminiTransducer_CarriesTheRestriction(t *testing.T) {
	g := NewUnderstandingTransducer(&textGeminiClient{response: geminiSchemaEnvelope}).(*UnderstandingTransducer)
	ctx := context.Background()
	g.initialize(ctx)
	intent := g.understandingToIntent(hypotheticalModify())
	if !intent.Restriction.Has(RestrictHypothetical) || !intent.Restriction.Has(RestrictConfirmFirst) {
		t.Fatalf("Gemini path dropped the limit: %v", intent.Restriction.Kinds())
	}
}

// IsReadOnly and the restriction read the user's words in one place.
func TestIsReadOnly_AgreesWithTheRestriction(t *testing.T) {
	for _, c := range []string{"no changes", "Read-Only", "dry_run"} {
		u := &Understanding{ActionType: "modify", UserConstraints: []string{c}}
		if !u.IsReadOnly() {
			t.Errorf("%q should make the request read-only", c)
		}
	}
	if (&Understanding{ActionType: "modify", UserConstraints: []string{"keep it simple"}}).IsReadOnly() {
		t.Error("prose outside the vocabulary must not make a modify read-only")
	}
	u := &Understanding{ActionType: "modify", UserConstraints: []string{"confirm_first"}}
	if !u.NeedsConfirmation() {
		t.Error("confirm_first must need confirmation")
	}
}
