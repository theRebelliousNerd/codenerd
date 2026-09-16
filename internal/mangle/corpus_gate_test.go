package mangle

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/TauCeti/mangle-go/analysis"
)

// TestCorpusGate_AllNonAdversarialMangleParses walks every .mg file in the
// repo and requires it to parse with the real Mangle parser, except for
// paths that are invalid by contract:
//
//   - **/mangle-adversarial/** — the stress-tester adversarial suite; its
//     README defines it as "invalid Mangle code patterns" by design.
//   - **/.nerd/** and .nerd/** — runtime state, crash dumps
//     (debug_program_ERROR.mg), and snapshots, never repo content.
//
// A floor on the checked count keeps a broken walk from passing silently.
// The floor counts only files present on a fresh clone (tracked production
// corpus); gitignored skill mirrors add to it when present.
func TestCorpusGate_AllNonAdversarialMangleParses(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	if err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".mg") {
			files = append(files, p)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	const minChecked = 100 // tracked production corpus alone exceeds this
	checked := 0
	for _, p := range files {
		rel, _ := filepath.Rel(root, p)
		slash := filepath.ToSlash(rel)
		if isExcludedCorpusPath(slash) {
			continue
		}
		checked++
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("%s: read: %v", rel, err)
			continue
		}
		if _, err := ParseUnit(strings.NewReader(string(data))); err != nil {
			t.Errorf("%s: does not parse: %v", rel, err)
		}
	}
	t.Logf("corpus gate: checked=%d total=%d", checked, len(files))
	if checked < minChecked {
		t.Errorf("checked only %d files, want at least %d (walk broken?)", checked, minChecked)
	}
}

// TestCorpusGate_SyntacticFixturesStillFail pins the other direction: files
// under mangle-adversarial/syntactic/ exist to demonstrate syntax violations,
// so each one MUST fail to parse. If one starts parsing, either the fixture
// lost its teeth or the grammar changed — both demand attention.
func TestCorpusGate_SyntacticFixturesStillFail(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []string
	if err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".mg") &&
			strings.Contains(filepath.ToSlash(p), "mangle-adversarial/syntactic/") {
			fixtures = append(fixtures, p)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no syntactic fixtures found (walk broken?)")
	}
	for _, p := range fixtures {
		rel, _ := filepath.Rel(root, p)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("%s: read: %v", rel, err)
			continue
		}
		if _, err := ParseUnit(strings.NewReader(string(data))); err == nil {
			t.Errorf("%s: parses clean but is supposed to demonstrate a syntax violation", rel)
		}
	}
	t.Logf("syntactic fixtures pinned: %d", len(fixtures))
}

func isExcludedCorpusPath(slashRel string) bool {
	return strings.Contains(slashRel, "mangle-adversarial/") ||
		strings.Contains(slashRel, "/.nerd/") ||
		strings.HasPrefix(slashRel, ".nerd/")
}

// Adversarial fixture expectations by "category/file.mg" suffix. Mirror
// copies (.agent, .gemini, ...) share the canonical verdict: a stale mirror
// fails here until it is re-synced, which is the point.
//
//   - stageParseFail: the file must NOT parse (demonstrated notations are
//     syntax errors). Scaffolding Decls and facts must still be valid, so a
//     failure anywhere proves a demo broke the parse, not the setup.
//   - stageAnalyzeFail: the file must parse but fail AnalyzeOneUnit
//     (unbound variables, unknown functions).
//   - stageStratFail: the file must parse and analyze but fail Stratify
//     (genuine recursion-through-negation cycles).
//   - stageClean: the file must pass all three stages. Its flaws are
//     runtime properties (non-termination, unchecked fact types, wrong
//     answers) that no static gate can catch; the pin guards the
//     scaffolding against rot and documents the limit honestly.
type adversarialStage int

const (
	stageParseFail adversarialStage = iota
	stageAnalyzeFail
	stageStratFail
	stageClean
)

func (s adversarialStage) String() string {
	switch s {
	case stageParseFail:
		return "parse-fail"
	case stageAnalyzeFail:
		return "analyze-fail"
	case stageStratFail:
		return "stratify-fail"
	default:
		return "clean"
	}
}

var adversarialExpectations = map[string]adversarialStage{
	// Syntactic violations: must not parse. (Also covered by
	// TestCorpusGate_SyntacticFixturesStillFail; the table keeps one
	// authoritative map of every fixture's contract.)
	"syntactic/assignment_operators.mg":  stageParseFail,
	"syntactic/atom_string_confusion.mg": stageParseFail,
	"syntactic/inline_aggregation.mg":    stageParseFail,
	"syntactic/lowercase_vars.mg":        stageParseFail,
	"syntactic/missing_periods.mg":       stageParseFail,
	"syntactic/souffle_syntax.mg":        stageParseFail,
	"syntactic/string_predicate.mg":      stageParseFail,
	"syntactic/wrong_comments.mg":        stageParseFail,
	// Safety violations: parse, then fail binding/safety analysis.
	"safety/anonymous_misuse.mg":      stageAnalyzeFail,
	"safety/negation_order.mg":        stageAnalyzeFail,
	"safety/unbound_head_vars.mg":     stageAnalyzeFail,
	"safety/unsafe_negation.mg":       stageAnalyzeFail,
	"safety/stratification_cycles.mg": stageStratFail,
	// Type errors: unknown functions fail analysis; unchecked confusions
	// (fact types, int/float, list/scalar) are valid programs whose flaws
	// are runtime-only.
	"types/atom_vs_string.mg":         stageAnalyzeFail,
	"types/hallucinated_functions.mg": stageAnalyzeFail,
	"types/int_vs_float.mg":           stageClean,
	"types/list_in_scalar.mg":         stageClean,
	// Non-termination is a runtime property: all four must be
	// well-formed programs. Containment is by executor timeout, pinned in
	// internal/tactile/timeout_kill_test.go, not by static rejection.
	"loops/cartesian_explosion.mg":   stageClean,
	"loops/direct_self_reference.mg": stageClean,
	"loops/mutual_recursion.mg":      stageClean,
	"loops/unbounded_counter.mg":     stageClean,
	// Structure notations that do not exist here: must not parse.
	"structures/bracket_notation.mg": stageParseFail,
	"structures/dot_notation.mg":     stageParseFail,
	"structures/json_syntax.mg":      stageParseFail,
}

// TestCorpusGate_AdversarialFixturesHitDocumentedStage runs every
// mangle-adversarial fixture through the same three-stage pipeline the
// engine uses (parse, AnalyzeOneUnit, Stratify) and requires the verdict
// the table above documents. Before this gate the whole suite failed at
// line 6 with invalid Decl scaffolding, so no fixture exercised the error
// it was named for; a fixture that regresses to failing at the wrong
// stage fails here. New fixtures fail until they are classified.
func TestCorpusGate_AdversarialFixturesHitDocumentedStage(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []string
	if err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".mg") &&
			strings.Contains(filepath.ToSlash(p), "mangle-adversarial/") {
			fixtures = append(fixtures, p)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no adversarial fixtures found (walk broken?)")
	}
	for _, p := range fixtures {
		rel, _ := filepath.Rel(root, p)
		slash := filepath.ToSlash(rel)
		idx := strings.Index(slash, "mangle-adversarial/")
		key := slash[idx+len("mangle-adversarial/"):]
		want, ok := adversarialExpectations[key]
		if !ok {
			t.Errorf("%s: no documented stage; classify it in adversarialExpectations", rel)
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("%s: read: %v", rel, err)
			continue
		}
		unit, err := ParseUnit(strings.NewReader(string(data)))
		if err != nil {
			if want != stageParseFail {
				t.Errorf("%s: parses fail (%v), want %s", rel, oneLineErr(err), want)
			}
			continue
		}
		if want == stageParseFail {
			t.Errorf("%s: parses clean but is supposed to demonstrate a syntax violation", rel)
			continue
		}
		info, err := analysis.AnalyzeOneUnit(unit, nil)
		if err != nil {
			if want != stageAnalyzeFail {
				t.Errorf("%s: analysis fails (%v), want %s", rel, oneLineErr(err), want)
			}
			continue
		}
		if want == stageAnalyzeFail {
			t.Errorf("%s: analyzes clean but is supposed to fail binding/safety analysis", rel)
			continue
		}
		_, _, err = analysis.Stratify(analysis.Program{
			EdbPredicates: info.EdbPredicates,
			IdbPredicates: info.IdbPredicates,
			Rules:         info.Rules,
		})
		if err != nil {
			if want != stageStratFail {
				t.Errorf("%s: stratification fails (%v), want %s", rel, oneLineErr(err), want)
			}
			continue
		}
		if want != stageClean {
			t.Errorf("%s: passes all stages, want %s", rel, want)
		}
	}
	t.Logf("adversarial fixtures pinned: %d", len(fixtures))
}

func oneLineErr(err error) string {
	s := strings.ReplaceAll(err.Error(), "\n", " / ")
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}
