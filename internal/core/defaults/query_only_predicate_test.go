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
// GO-QUERIED, NEVER-PRODUCED BUDGET
// =============================================================================
// The sibling of TestStarvedPredicateBudget, for the half it cannot see.
//
// That gate finds predicates a RULE BODY reads and nothing produces. Its
// producer test is "no Go code that names it", which is deliberately
// over-approximate — and a kernel.Query("x") names x. So a predicate whose only
// Go mention is the query reading it counts as produced by that query, and the
// gate looks straight past the case where Go asks the kernel a question nothing
// can ever answer.
//
// That is not a hypothetical hole. populateTestState queried test_result every
// turn to decide whether the session was in TDD repair mode. Nothing in this
// repository asserts test_result. The query returned empty forever, which is
// indistinguishable from "no tests have run" — a normal state — so there was
// never anything to investigate, and an entire operational mode was
// unreachable with no test failing and no line logged.
//
// A Go query with no producer always returns zero rows. Whatever the caller
// does with zero rows, it does always. The failure is silence, which is why it
// needs a gate rather than a bug report.
//
// Like its sibling this is a budget, not a target of zero. Some of these are
// genuinely someone else's job to assert — an operator-supplied fact, an
// optional integration, a predicate a shard may volunteer. Adding a line is an
// admission; before adding one, ask whether the producer is genuinely external
// or whether the wiring is simply missing, which is what test_result was.

const queryOnlyBaselinePath = "testdata/query_only_predicates.txt"

var (
	// Matches kernel.Query("x") and kernel.QueryCallback("x").
	goQueryRe = regexp.MustCompile(`\.Query(?:Callback)?\(\s*"([a-z_][a-zA-Z0-9_]*)"`)
	// Matches core.Fact{Predicate: "x"} — Go asserting a fact directly.
	goPredicateFieldRe = regexp.MustCompile(`Predicate:\s*"([a-z_][a-zA-Z0-9_]*)"`)
	// Matches a predicate written as Mangle source inside a Go string, which is
	// how the init scan emits profile.mg: `project_language(/go).`
	goMangleTextRe = regexp.MustCompile(`[a-z_][a-zA-Z0-9_]*\s*\(`)
)

// goQueriedAndProduced scans non-test Go under internal/ and cmd/ for the
// predicates it queries and the predicates it produces.
//
// Production is over-approximated on purpose, and in the same direction and for
// the same reason as the sibling gate: a false "produced" costs one missed
// finding, while a false "never produced" costs someone an afternoon hunting a
// bug that is not there. Anything that looks like Mangle source inside a string
// literal counts, because that is how generated corpora are written.
func goQueriedAndProduced(t *testing.T, root string) (queried map[string][]string, produced map[string]struct{}) {
	t.Helper()
	queried = make(map[string][]string)
	produced = make(map[string]struct{}, 4096)

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
			text := string(data)
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			rel = filepath.ToSlash(rel)

			for _, m := range goQueryRe.FindAllStringSubmatch(text, -1) {
				queried[m[1]] = append(queried[m[1]], rel)
			}
			for _, m := range goPredicateFieldRe.FindAllStringSubmatch(text, -1) {
				produced[m[1]] = struct{}{}
			}
			// Mangle emitted as text. Scanning every string literal would be
			// too coarse even for an over-approximation, so this looks only at
			// literals that contain a '(' and a '.', the shape of a fact.
			for _, lit := range goStringLiteralBodies(text) {
				if !strings.Contains(lit, "(") {
					continue
				}
				for _, m := range goMangleTextRe.FindAllString(lit, -1) {
					produced[strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(m), "("))] = struct{}{}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", sub, err)
		}
	}
	return queried, produced
}

// goStringLiteralBodies returns the contents of double-quoted and backquoted
// literals. Deliberately simple: it is feeding an over-approximation, so a
// literal it splits wrongly costs at most an extra name in the produced set.
func goStringLiteralBodies(text string) []string {
	var out []string
	for _, m := range regexp.MustCompile("`[^`]*`").FindAllString(text, -1) {
		out = append(out, strings.Trim(m, "`"))
	}
	for _, m := range regexp.MustCompile(`"(?:[^"\\\n]|\\.)*"`).FindAllString(text, -1) {
		out = append(out, strings.Trim(m, `"`))
	}
	return out
}

func currentQueryOnlyPredicates(t *testing.T) (names []string, sites map[string][]string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := repoRootFrom(t, cwd)

	_, mangleProduced, _ := mangleCorpus(t, filepath.Join(root, "internal", "core", "defaults"))
	queried, goProduced := goQueriedAndProduced(t, root)

	sites = make(map[string][]string)
	for pred, where := range queried {
		if _, ok := mangleProduced[pred]; ok {
			continue
		}
		if _, ok := goProduced[pred]; ok {
			continue
		}
		names = append(names, pred)
		sort.Strings(where)
		sites[pred] = where
	}
	sort.Strings(names)
	return names, sites
}

// TestGoQueriedPredicateBudget fails when Go starts querying a predicate
// nothing produces, and when one on the list gains a producer without leaving
// the list.
func TestGoQueriedPredicateBudget(t *testing.T) {
	current, sites := currentQueryOnlyPredicates(t)

	// Opt-in through the environment rather than a flag, matching the sibling
	// gate: a gate that rewrites its own baseline on failure is not a gate.
	if os.Getenv("CODENERD_UPDATE_QUERY_ONLY") == "1" {
		writeQueryOnlyBaseline(t, current, sites)
		t.Logf("rewrote %s with %d predicate(s)", queryOnlyBaselinePath, len(current))
		return
	}

	data, err := os.ReadFile(queryOnlyBaselinePath)
	if err != nil {
		t.Fatalf("read %s: %v\nRegenerate with: CODENERD_UPDATE_QUERY_ONLY=1 go test ./internal/core/defaults/ -run TestGoQueriedPredicateBudget",
			queryOnlyBaselinePath, err)
	}
	baseline := make(map[string]struct{})
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		baseline[strings.Fields(line)[0]] = struct{}{}
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

	for _, p := range added {
		t.Errorf("Go queries %s and nothing produces it: %s.\n"+
			"That query returns zero rows on every call, forever, and zero rows is "+
			"indistinguishable from the legitimate empty case -- so whatever the caller "+
			"does with an empty result, it does always, silently. Either assert the fact "+
			"somewhere, or record it here with the reason its producer is external.",
			p, strings.Join(sites[p], ", "))
	}
	if len(wired) > 0 {
		t.Errorf("%d predicate(s) on the list now have a producer and must be removed from %s: %v\n"+
			"Regenerate with: CODENERD_UPDATE_QUERY_ONLY=1 go test ./internal/core/defaults/ -run TestGoQueriedPredicateBudget",
			len(wired), queryOnlyBaselinePath, wired)
	}
}

func writeQueryOnlyBaseline(t *testing.T, names []string, sites map[string][]string) {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("# Predicates Go queries that nothing produces.\n")
	sb.WriteString("#\n")
	sb.WriteString("# Each of these returns zero rows on every call. Zero rows is also what a\n")
	sb.WriteString("# legitimately empty kernel returns, so the caller cannot tell the two apart\n")
	sb.WriteString("# and neither can anyone reading its output.\n")
	sb.WriteString("#\n")
	sb.WriteString("# Regenerate: CODENERD_UPDATE_QUERY_ONLY=1 go test ./internal/core/defaults/ -run TestGoQueriedPredicateBudget\n")
	sb.WriteString("# See query_only_predicate_test.go for why this is a budget rather than a\n")
	sb.WriteString("# target of zero.\n")
	sb.WriteString("#\n")
	sb.WriteString("# Adding a line is an admission, not a fix. Ask first whether the producer is\n")
	sb.WriteString("# genuinely external -- an operator-supplied fact, an optional integration --\n")
	sb.WriteString("# or whether the wiring is simply missing, which is what test_result was.\n")
	sb.WriteString("#\n")
	sb.WriteString("# Format: predicate<TAB>the file(s) querying it.\n")
	for _, n := range names {
		sb.WriteString(n)
		sb.WriteString("\t")
		sb.WriteString(strings.Join(sites[n], " "))
		sb.WriteString("\n")
	}
	if err := os.WriteFile(queryOnlyBaselinePath, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", queryOnlyBaselinePath, err)
	}
}
