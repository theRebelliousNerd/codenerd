package defaults

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// =============================================================================
// STARVED ATOM-VALUE BUDGET
// =============================================================================
// The value-level twin of TestStarvedPredicateBudget, and it exists because the
// predicate-level check is structurally blind to this shape.
//
// A rule body can select on a specific atom rather than on a predicate:
//
//	specific_enrichment(TaskID, /decompose) :-
//	    task_error(TaskID, /too_complex, _).
//
// task_error is produced, abundantly, so the predicate-level gate is satisfied
// and stays quiet. But Go's classifier emits /transient, /logic and /refused —
// it has never emitted /too_complex — so this rule cannot fire, and no test
// anywhere notices. Starvation at the level of a value is invisible to a check
// that works at the level of a predicate.
//
// Two of these were found by hand in one afternoon, both by following a
// vocabulary from where it is produced to where it is consumed:
//
//   - task_error: the kernel derives an enrichment strategy per failure type —
//     /unknown_api to research, /missing_context to documentation,
//     /too_complex to decompose, /domain_specific to a specialist. Go emits
//     none of those four, so every failing task falls through to the same
//     default and the adaptive retry that makes the kernel more than a retry
//     loop is inert.
//   - memory_operations: the piggyback schema admits four operations and the
//     compressor's switch handled three, so a "note" the model was told to
//     emit, and which validated on the way in, fell out of the bottom of a
//     switch with no default. (That one is now gated in internal/context.)
//
// Same posture as the sibling: this is a budget, not a target of zero. Plenty
// of entries are a rule written ahead of its producer on purpose. It demands
// the list not GROW, and that entries leave it when they are wired.
//
// The Go scan is deliberately over-approximate — an atom named anywhere in
// non-test Go counts as producible, with or without its leading slash. The cost
// of a false "produced" is one missed finding; the cost of a false "starved" is
// a failing gate sending someone after a bug that is not there.

// THE FOURTH DIRECTION, AND WHY IT IS NOT GATED HERE.
//
// The Go/Mangle boundary has four ways to be disconnected, and three now have
// a budget: Go asserting a predicate no .mg declares (undeclared_asserts.txt),
// a rule reading a predicate nothing produces (starved_predicates.txt), and a
// rule selecting an atom nothing produces (this file).
//
// The fourth is a rule that DERIVES something no rule body and no Go code ever
// reads — which is what made the enrichment example doubly dead, since nothing
// queries enrichment_strategy either. Measuring it is easy: 261 of the corpus's
// 917 rule-derived predicates have no reader in a rule body and appear in no
// non-test Go, among them build_healthy, campaign_requires_tests and
// coder_stuck.
//
// It is not gated because that measurement cannot be defended as a definition
// of dead. A derived fact can reach a consumer without being named: QueryAll
// exists, shard export moves derivations across kernels, and a predicate that
// lands in a prompt is read by the model rather than by Go. Establishing which
// of the 261 are genuinely unreachable means settling each of those paths
// first. A 261-entry baseline whose meaning is unsettled is not a gate, it is
// a number that looks like rigour, and this repo has enough of those.
//
// The number is recorded here so the next person starts from it rather than
// rediscovering it.

const starvedAtomBaselinePath = "testdata/starved_atom_values.txt"

var (
	// literalRe matches one predicate application with no nested parentheses,
	// which is the shape every selector in this corpus uses.
	atomLiteralRe = regexp.MustCompile(`\b([a-z_][a-zA-Z0-9_]*)\s*\(([^()]*)\)`)
	atomValueRe   = regexp.MustCompile(`/([a-z_][a-zA-Z0-9_]*)`)
	// goAtomRe accepts "foo" and "/foo": Go writes both forms, and the
	// auto-atomizer promotes an identifier-shaped string to a name.
	goAtomRe = regexp.MustCompile(`"/?([a-zA-Z_][a-zA-Z0-9_]*)"`)
)

// currentStarvedAtomValues returns "predicate\t/atom" for every atom a rule
// body selects on that no rule head, ground fact, or Go source names.
func currentStarvedAtomValues(t *testing.T) []string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := repoRootFrom(t, cwd)

	inBodies := map[string]map[string]struct{}{}
	inHeads := map[string]map[string]struct{}{}

	add := func(m map[string]map[string]struct{}, pred, atom string) {
		if m[pred] == nil {
			m[pred] = map[string]struct{}{}
		}
		m[pred][atom] = struct{}{}
	}

	corpus := filepath.Join(root, "internal", "core", "defaults")
	walkErr := filepath.WalkDir(corpus, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".mg") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var lines []string
		for _, l := range strings.Split(string(data), "\n") {
			lines = append(lines, strings.SplitN(l, "#", 2)[0])
		}
		for _, stmt := range statementSplit.Split(strings.Join(lines, "\n"), -1) {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "Decl") {
				continue
			}
			head, body, isRule := strings.Cut(stmt, ":-")
			// A head, or a ground fact, produces the atoms it names.
			for _, m := range atomLiteralRe.FindAllStringSubmatch(head, -1) {
				for _, a := range atomValueRe.FindAllStringSubmatch(m[2], -1) {
					add(inHeads, m[1], a[1])
				}
			}
			if !isRule {
				continue
			}
			for _, m := range atomLiteralRe.FindAllStringSubmatch(body, -1) {
				for _, a := range atomValueRe.FindAllStringSubmatch(m[2], -1) {
					add(inBodies, m[1], a[1])
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk corpus: %v", walkErr)
	}
	if len(inBodies) == 0 {
		t.Fatal("found no rule-body atom selectors in the corpus; the parser is wrong " +
			"and this gate would pass on an empty measurement")
	}

	goNames := goAtomLiterals(t, root)

	var starved []string
	for pred, atoms := range inBodies {
		for atom := range atoms {
			if _, produced := inHeads[pred][atom]; produced {
				continue
			}
			if _, inGo := goNames[atom]; inGo {
				continue
			}
			starved = append(starved, fmt.Sprintf("%s\t/%s", pred, atom))
		}
	}
	sort.Strings(starved)
	return starved
}

// goAtomLiterals returns every identifier appearing as a quoted string in
// non-test Go under internal/ and cmd/, with any leading slash stripped.
func goAtomLiterals(t *testing.T, root string) map[string]struct{} {
	t.Helper()
	out := make(map[string]struct{}, 8192)
	for _, sub := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, sub), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			for _, m := range goAtomRe.FindAllStringSubmatch(string(data), -1) {
				out[m[1]] = struct{}{}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", sub, err)
		}
	}
	return out
}

// TestStarvedAtomValueBudget fails when a rule starts selecting on an atom
// nothing can produce, and when an entry stops being starved without leaving
// the baseline.
func TestStarvedAtomValueBudget(t *testing.T) {
	current := currentStarvedAtomValues(t)

	// Regeneration is opt-in through the environment, like its sibling: a gate
	// that rewrites its own baseline on failure is not a gate.
	if os.Getenv("CODENERD_UPDATE_STARVED_ATOMS") == "1" {
		writeStarvedAtomBaseline(t, current)
		t.Logf("rewrote %s with %d entry/entries", starvedAtomBaselinePath, len(current))
		return
	}

	data, err := os.ReadFile(starvedAtomBaselinePath)
	if err != nil {
		t.Fatalf("read %s: %v\nRegenerate with: CODENERD_UPDATE_STARVED_ATOMS=1 go test ./internal/core/defaults/ -run TestStarvedAtomValueBudget",
			starvedAtomBaselinePath, err)
	}
	baseline := map[string]struct{}{}
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimRight(line, " \t")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		baseline[line] = struct{}{}
	}

	currentSet := make(map[string]struct{}, len(current))
	for _, e := range current {
		currentSet[e] = struct{}{}
	}

	var added, wired []string
	for _, e := range current {
		if _, known := baseline[e]; !known {
			added = append(added, e)
		}
	}
	for e := range baseline {
		if _, still := currentSet[e]; !still {
			wired = append(wired, e)
		}
	}
	sort.Strings(wired)

	if len(added) > 0 {
		t.Errorf("%d rule selector(s) on an atom nothing produces:\n  %s\n\n"+
			"The rule cannot fire. Either emit the atom from Go (or from another rule), "+
			"or record it here with the reason it is written ahead of its producer:\n"+
			"  CODENERD_UPDATE_STARVED_ATOMS=1 go test ./internal/core/defaults/ -run TestStarvedAtomValueBudget",
			len(added), strings.Join(prettyAtomEntries(added), "\n  "))
	}
	if len(wired) > 0 {
		t.Errorf("%d baseline entry/entries are no longer starved:\n  %s\n\n"+
			"Record the improvement so the count stays a measurement:\n"+
			"  CODENERD_UPDATE_STARVED_ATOMS=1 go test ./internal/core/defaults/ -run TestStarvedAtomValueBudget",
			len(wired), strings.Join(prettyAtomEntries(wired), "\n  "))
	}
}

func prettyAtomEntries(entries []string) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = strings.ReplaceAll(e, "\t", "  ")
	}
	return out
}

func writeStarvedAtomBaseline(t *testing.T, entries []string) {
	t.Helper()
	const header = `# Atoms a Mangle rule body selects on that nothing can produce — no rule head,
# no ground fact, no mention in non-test Go. Each of these rules cannot fire.
#
# A budget, not a target of zero: see TestStarvedAtomValueBudget for why some of
# these are deliberate, and for the two that were not.
#
# Regenerate: CODENERD_UPDATE_STARVED_ATOMS=1 go test ./internal/core/defaults/ -run TestStarvedAtomValueBudget
# Format: predicate<TAB>/atom
`
	body := header + strings.Join(entries, "\n") + "\n"
	if err := os.WriteFile(starvedAtomBaselinePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", starvedAtomBaselinePath, err)
	}
}
