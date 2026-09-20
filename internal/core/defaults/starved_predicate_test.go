package defaults

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// =============================================================================
// STARVED PREDICATE BUDGET
// =============================================================================
// A starved predicate is one that is declared, joined by at least one rule
// body, and produced by nothing — no Mangle rule head, no ground fact, and no
// Go code that names it. Every rule that reads one derives nothing, forever,
// and no test fails.
//
// This is the defect class that keeps recurring in this codebase, and it is
// invisible by construction: both halves look correct in isolation. The 2026-09
// hardening pass found four instances the hard way.
//
//   - modified_function: impact.mg derived the entire caller-impact chain from
//     it. Its only Go references were an allow-list entry letting the model
//     volunteer the fact and a shard's owned-predicate list. Nothing derived it
//     from what the system had actually done, so the flagship "impact-
//     prioritized callers" feature never ran once in production.
//   - user_rejected_finding / user_accepted_finding: reviewer.mg's whole
//     self-correction loop hung on them. The chat commands where a person says
//     "this finding was wrong" dropped that judgement into a nil check.
//   - atom_selector: thirteen dimension rules joined it. Its only emitter was
//     called from two test files, and nothing consumed the rules' output
//     either. Removed.
//
// This test is the gate. It does not demand the list be empty — a good number
// of these are deliberate optional inputs, and emptying it is a program of
// work, not a commit. It demands the list not GROW, and that entries leave it
// when they are wired, so the count is a real measurement rather than a
// forgotten file.

// starvedBaselinePath is the checked-in inventory, one predicate per line.
const starvedBaselinePath = "testdata/starved_predicates.txt"

var (
	starvedDeclRe = regexp.MustCompile(`^\s*Decl\s+([a-z_][a-zA-Z0-9_]*)\s*\(`)
	starvedHeadRe = regexp.MustCompile(`^([a-z_][a-zA-Z0-9_]*)\s*\(`)
	starvedAtomRe = regexp.MustCompile(`\b([a-z_][a-zA-Z0-9_]*)\s*\(`)
	starvedGoRe   = regexp.MustCompile(`"([a-z_][a-zA-Z0-9_]*)"`)
	// A fact written as source text rather than a name: AssertString and
	// ParseFactString take a whole clause, so the predicate appears as
	// `"pred(` inside a format string and never as a bare `"pred"`. Missing
	// this form cost a dogfood run: turn_age_category is asserted by
	// context/compressor_metrics.go via
	// AssertString(fmt.Sprintf("turn_age_category(%q, %s)", ...)), the gate
	// could not see it, the predicate sat on the starved list as a false
	// positive, and the run -- briefed on that false premise -- added an
	// unused `const ... = "turn_age_category"` whose comment said it existed
	// so "the starved-predicate gate sees this file as the producer".
	// A gate that can be satisfied by a string literal teaches writing string
	// literals.
	starvedAssertStringRe = regexp.MustCompile(`"([a-z_][a-zA-Z0-9_]*)\(`)
	// An external-predicate registration, which is production only when the
	// corpus also declares the predicate external(). See goStringLiterals.
	starvedRegRe = regexp.MustCompile(`mkPred\(\s*"([a-z_][a-zA-Z0-9_]*)"`)
	// statementSplit ends a Mangle statement at a '.' that closes a line.
	statementSplit = regexp.MustCompile(`\.\s*(?:\n|$)`)
)

// repoRootFrom walks up from dir until it finds the directory holding go.mod.
func repoRootFrom(t *testing.T, dir string) string {
	t.Helper()
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate repo root from %s", dir)
	return ""
}

// mangleCorpus reads every .mg file under the defaults corpus and returns the
// declared predicates, the predicates some rule head or ground fact produces,
// and the predicates some rule body reads.
// It also returns the predicates whose Decl carries an external() descriptor,
// because only those reach an external handler at evaluation: RealKernel
// registers a callback only when the Decl is external (kernel_eval.go, the
// decl.IsExternal() filter), and silently drops every other registration.
func mangleCorpus(t *testing.T, corpusDir string) (declared map[string]string, produced, consumed, externalDecl map[string]struct{}) {
	t.Helper()
	declared = make(map[string]string)
	produced = make(map[string]struct{})
	consumed = make(map[string]struct{})
	externalDecl = make(map[string]struct{})

	err := filepath.WalkDir(corpusDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".mg") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		// Strip comments before parsing; '#' begins a comment in this corpus.
		var lines []string
		for _, l := range strings.Split(string(data), "\n") {
			lines = append(lines, strings.SplitN(l, "#", 2)[0])
			if m := starvedDeclRe.FindStringSubmatch(lines[len(lines)-1]); m != nil {
				declared[m[1]] = path
				if strings.Contains(lines[len(lines)-1], "external()") {
					externalDecl[m[1]] = struct{}{}
				}
			}
		}
		for _, stmt := range statementSplit.Split(strings.Join(lines, "\n"), -1) {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "Decl") {
				continue
			}
			head, body, isRule := strings.Cut(stmt, ":-")
			if m := starvedHeadRe.FindStringSubmatch(strings.TrimSpace(head)); m != nil {
				produced[m[1]] = struct{}{}
			}
			if isRule {
				for _, m := range starvedAtomRe.FindAllStringSubmatch(body, -1) {
					consumed[m[1]] = struct{}{}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	return declared, produced, consumed, externalDecl
}

// goStringLiterals returns every lowercase identifier appearing as a quoted
// string in non-test Go under internal/ and cmd/.
//
// Deliberately over-approximate: a name mentioned anywhere in Go counts as
// possibly produced. The cost of a false "produced" is one missed finding; the
// cost of a false "starved" is a failing test that sends someone hunting for a
// bug that is not there.
//
// There is one case where that is not over-approximation but a wrong answer, so
// the pieces needed to judge it are returned too. `mkPred("p", ...)` in
// BuildExternalPredicates registers a handler for p, and RealKernel keeps that
// registration only when the corpus declares p with an external() descriptor --
// kernel_eval.go filters the callback map by decl.IsExternal() and drops the
// rest without a word. Against a plain Decl the handler never runs, yet the
// registration and the handler body both put p's name in a Go string.
//
// Measured 2026-09-20: a dogfood run added four such registrations
// (target_is_large, target_is_complex, target_word_count,
// target_contains_multiple_files) against plain Decls, and this gate reported
// all four as "no longer starved". Nothing had changed -- the rules they feed
// still derive nothing, and the run's own verdict was /unverified.
//
// So `registered` is every mkPred name and `asserts` is every real kernel
// assert (the same boundary-anchored literal the undeclared gate counts). A
// predicate with a live production route -- a rule head, an Assert, or an
// external() Decl -- is produced; an inert registration is not.
func goStringLiterals(t *testing.T, root string) (all, registered, asserts map[string]struct{}) {
	t.Helper()
	out := make(map[string]struct{}, 4096)
	registered = make(map[string]struct{})
	asserts = make(map[string]struct{})
	for _, sub := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, sub), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !isProductionGo(path) {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			src := string(data)
			for _, m := range starvedGoRe.FindAllStringSubmatch(src, -1) {
				out[m[1]] = struct{}{}
			}
			for _, m := range starvedAssertStringRe.FindAllStringSubmatch(src, -1) {
				out[m[1]] = struct{}{}
				asserts[m[1]] = struct{}{}
			}
			for _, m := range starvedRegRe.FindAllStringSubmatch(src, -1) {
				registered[m[1]] = struct{}{}
			}
			// undeclaredAssertRe is the sibling gate's literal for a real
			// kernel assert, shared so the two gates agree on what writing a
			// fact looks like.
			for _, m := range undeclaredAssertRe.FindAllStringSubmatch(src, -1) {
				asserts[m[1]] = struct{}{}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", sub, err)
		}
	}
	return out, registered, asserts
}

// goProduces reports whether pred's presence in Go is evidence that something
// fills it.
//
// Every mention counts, as before, unless pred is registered as an external
// predicate. Then the registration is the claim being tested: it reaches the
// engine only behind an external() Decl, so without one the only remaining
// evidence is a real kernel assert. This is deliberately narrow -- it says
// nothing about predicates nobody registers, and a registered predicate that is
// also asserted stays produced.
func goProduces(pred string, registered, asserts, externalDecl map[string]struct{}) bool {
	if _, isReg := registered[pred]; !isReg {
		return true
	}
	if _, isExternal := externalDecl[pred]; isExternal {
		return true
	}
	_, asserted := asserts[pred]
	return asserted
}

// currentStarvedPredicates computes the live inventory.
func currentStarvedPredicates(t *testing.T) []string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := repoRootFrom(t, cwd)

	declared, produced, consumed, externalDecl := mangleCorpus(t, filepath.Join(root, "internal", "core", "defaults"))
	goNames, registered, asserts := goStringLiterals(t, root)

	var starved []string
	for pred := range consumed {
		if _, isDeclared := declared[pred]; !isDeclared {
			continue
		}
		if _, hasProducer := produced[pred]; hasProducer {
			continue
		}
		if _, inGo := goNames[pred]; inGo && goProduces(pred, registered, asserts, externalDecl) {
			continue
		}
		starved = append(starved, pred)
	}
	sort.Strings(starved)
	return starved
}

// TestStarvedPredicateBudget fails when a rule starts reading a predicate that
// nothing produces, and when a predicate on the list stops being starved
// without being removed from it.
func TestStarvedPredicateBudget(t *testing.T) {
	current := currentStarvedPredicates(t)

	// Regeneration is opt-in through the environment rather than a test flag,
	// so it cannot fire in CI by accident: a gate that rewrites its own
	// baseline on failure is not a gate.
	if os.Getenv("CODENERD_UPDATE_STARVED") == "1" {
		writeStarvedBaseline(t, current)
		t.Logf("rewrote %s with %d predicate(s)", starvedBaselinePath, len(current))
		return
	}

	data, err := os.ReadFile(starvedBaselinePath)
	if err != nil {
		t.Fatalf("read %s: %v\nRegenerate with: CODENERD_UPDATE_STARVED=1 go test ./internal/core/defaults/ -run TestStarvedPredicateBudget",
			starvedBaselinePath, err)
	}
	baseline := make(map[string]struct{})
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		baseline[line] = struct{}{}
	}

	currentSet := make(map[string]struct{}, len(current))
	for _, p := range current {
		currentSet[p] = struct{}{}
	}

	var added []string
	for _, p := range current {
		if _, known := baseline[p]; !known {
			added = append(added, p)
		}
	}
	var wired []string
	for p := range baseline {
		if _, still := currentSet[p]; !still {
			wired = append(wired, p)
		}
	}
	sort.Strings(wired)

	if len(added) > 0 {
		t.Errorf(`%d predicate(s) became starved: %v

A starved predicate is declared, joined by a rule body, and produced by
nothing — no rule head, no ground fact, no Go code naming it. Every rule that
reads one derives nothing, forever, and nothing else fails.

Either write the producer, or if the predicate is a deliberate optional input,
record it with: CODENERD_UPDATE_STARVED=1 go test ./internal/core/defaults/ -run TestStarvedPredicateBudget
and say in the commit who is expected to supply it.

Baseline: %s`,
			len(added), added, starvedBaselinePath)
	}

	if len(wired) > 0 {
		t.Errorf(`%d predicate(s) on the starved list now have a producer: %v

Good. Refresh the baseline so the count stays a real measurement:
CODENERD_UPDATE_STARVED=1 go test ./internal/core/defaults/ -run TestStarvedPredicateBudget

Baseline: %s`,
			len(wired), wired, starvedBaselinePath)
	}
}

// writeStarvedBaseline rewrites the inventory, header included.
func writeStarvedBaseline(t *testing.T, predicates []string) {
	t.Helper()
	const header = `# Starved predicates — declared, read by a rule body, produced by nothing.
#
# Regenerate: CODENERD_UPDATE_STARVED=1 go test ./internal/core/defaults/ -run TestStarvedPredicateBudget
# See starved_predicate_test.go for what this measures and why it is a gate
# rather than a target of zero.
#
# Adding to this list is an admission, not a fix. Each line is a rule chain that
# derives nothing today. Before adding one, ask whether the producer is
# genuinely someone else's job — an operator-supplied fact, an optional
# integration — or whether the wiring is simply missing, which is what every
# instance found in the 2026-09 audit turned out to be.

`
	body := header + strings.Join(predicates, "\n") + "\n"
	if err := os.WriteFile(starvedBaselinePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", starvedBaselinePath, err)
	}
}
