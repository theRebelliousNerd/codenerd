package atomjudge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/prompt"
	"codenerd/internal/typesafe"
)

// readings answers the atomicity questions with fixed Score positions and
// covers probabilities.
func readings(known, concerns, padding, covers float64) *fakeAsker {
	return &fakeAsker{answer: func(name string, _ typesafe.Question) string {
		score := func(v float64) string {
			return fmt.Sprintf(`{"type":"score","score":%g,"confidence":0.5,"legend":{"0":"a"},"probabilities":{"0":1}}`, v)
		}
		switch {
		case name == QuestionKnown:
			return score(known)
		case name == QuestionConcerns:
			return score(concerns)
		case name == QuestionPadding:
			return score(padding)
		case strings.HasPrefix(name, questionCovers):
			return fmt.Sprintf(`{"type":"noul","noul":%g}`, covers)
		}
		return `{}`
	}}
}

func coder(id, content string) *prompt.PromptAtom {
	a := atom(id, content)
	a.ShardTypes = []string{"/coder"}
	return a
}

func TestAdmission_TruthTable(t *testing.T) {
	existing := []*prompt.PromptAtom{coder("existing/wrap", "Wrap returned errors with %w so callers can inspect them.")}
	cand := coder("cand/wrap", "Wrap errors with %w when returning them so callers can use errors.Is.")
	cases := []struct {
		name                             string
		known, concerns, padding, covers float64
		want                             string
	}{
		{"clean atom", 1.0, 0.4, 0.5, 0.1, Admitted},
		{"covered by a reaching atom", 1.0, 0.4, 0.5, 0.9, Duplicate},
		{"three to five instructions", 1.0, 1.6, 0.5, 0.1, SeveralConcerns},
		{"two instructions admit", 1.0, 1.2, 0.5, 0.1, Admitted},
		{"all of it known", 3.7, 0.4, 0.5, 0.1, KnownByDefault},
		{"most of it known admits", 3.2, 0.4, 0.5, 0.1, Admitted},
		{"half padding", 1.0, 0.4, 2.6, 0.1, Padding},
		{"unsettled covers admits", 1.0, 0.4, 0.5, 0.5, Admitted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := Admission{Asker: readings(c.known, c.concerns, c.padding, c.covers), MaxChars: 1500, Yes: 0.7, Compare: 3}
			v, err := a.Check(context.Background(), cand, existing)
			if err != nil {
				t.Fatal(err)
			}
			if v.Decision != c.want {
				t.Fatalf("decision %s, want %s", v, c.want)
			}
			if v.Admit() != (c.want == Admitted) {
				t.Fatalf("Admit() = %v for %s", v.Admit(), v)
			}
		})
	}
}

// An existing atom scoped narrower than the candidate never makes it a
// duplicate: the candidate reaches calls the existing atom does not.
func TestAdmission_NarrowerExistingAtomIsNotADuplicate(t *testing.T) {
	existing := coder("existing/wrap", "Wrap returned errors with %w.")
	existing.Languages = []string{"/go"}
	cand := coder("cand/wrap", "Wrap returned errors with %w.")
	cand.Languages = []string{"/go", "/rust"}
	a := Admission{Asker: readings(1, 0.3, 0.3, 0.95), MaxChars: 1500, Yes: 0.7, Compare: 3}
	v, err := a.Check(context.Background(), cand, []*prompt.PromptAtom{existing})
	if err != nil {
		t.Fatal(err)
	}
	if v.Decision != Admitted {
		t.Fatalf("decision %s, want admitted", v)
	}
	if !Reaches(cand, existing) {
		t.Fatal("the wider atom must reach the narrower atom's calls")
	}
}

// Size is code's check and runs before Jev is asked.
func TestAdmission_OversizeIsRefusedWithoutAsking(t *testing.T) {
	f := readings(0, 0, 0, 0)
	a := Admission{Asker: f, MaxChars: 10, Yes: 0.7}
	v, err := a.Check(context.Background(), coder("c", "far more than ten characters"), nil)
	if err != nil || v.Decision != Oversize || v.Admit() {
		t.Fatalf("verdict %s err %v", v, err)
	}
	if len(f.bodies) != 0 {
		t.Fatal("Jev was asked about an oversize atom")
	}
}

// A failed Jev call is never a "no": the atom is admitted as unjudged with
// the failure kept as its reason.
func TestAdmission_JevFailureAdmitsUnjudged(t *testing.T) {
	a := Admission{Asker: &fakeAsker{err: errors.New("503 upstream")}, MaxChars: 1500, Yes: 0.7}
	v, err := a.Check(context.Background(), coder("c", "Wrap errors."), nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.Decision != Unjudged || !v.Admit() || !strings.Contains(v.Reason, "503 upstream") {
		t.Fatalf("verdict %s", v)
	}
	off := Admission{MaxChars: 1500}
	if v, _ := off.Check(context.Background(), coder("c", "Wrap errors."), nil); v.Decision != Unjudged {
		t.Fatalf("Jev off: verdict %s", v)
	}
}

func TestAdmission_ConfigResolvesBoundThresholdAndSwitch(t *testing.T) {
	f := readings(0, 0, 0, 0)
	on := AdmissionFor(f, &config.TypeSafeConfig{JudgmentYesPercent: 80}, 1200, 3)
	if on.Asker == nil || on.Yes != 0.8 || on.MaxChars != 1200 {
		t.Fatalf("resolved %+v", on)
	}
	no := false
	off := AdmissionFor(f, &config.TypeSafeConfig{AtomChecks: &no}, 1200, 3)
	if off.Asker != nil {
		t.Fatal("atom_checks off must leave Jev out")
	}
	if _, err := (Admission{}).Check(context.Background(), coder("c", "x"), nil); err == nil {
		t.Fatal("a missing bound must be an error, not a default")
	}
}

func TestAtomicityRequest_ComparesByIndexWithTextOnly(t *testing.T) {
	cand := coder("cand", "Wrap errors with %w.")
	other := coder("lib/errors/wrap", "Use %w to wrap.")
	state, qs, err := AtomicityRequest(cand, []*prompt.PromptAtom{other})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := qs[questionCovers+"0"]; !ok {
		t.Fatalf("questions %v lack covers_0", qs)
	}
	if g := state["guidance"].(map[string]any); len(g) != 1 || g["text"] != cand.Content {
		t.Fatalf("guidance state carries more than the text: %v", g)
	}
	if _, ok := state["existing_0"]; !ok {
		t.Fatal("state lacks existing_0")
	}
}

func TestNearest_RanksLexicalNeighboursAndSkipsSelf(t *testing.T) {
	cand := atom("c", "wrap returned errors with fmt errorf and the w verb")
	near := atom("n", "wrap errors with fmt errorf w verb so callers inspect them")
	far := atom("f", "table driven tests name each case")
	got := Nearest(cand, []*prompt.PromptAtom{far, cand, near}, 2)
	if len(got) == 0 || got[0].ID != "n" {
		t.Fatalf("nearest %v", got)
	}
	for _, a := range got {
		if a.ID == "c" {
			t.Fatal("the candidate compared with itself")
		}
	}
}
