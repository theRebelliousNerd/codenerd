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
// UNDECLARED ASSERT BUDGET
// =============================================================================
// The mirror of TestStarvedPredicateBudget, in the opposite direction.
//
// Starved: declared and read, produced by nothing.
// Undeclared: produced by Go, declared by nothing.
//
// The second is quieter than the first. Fact.ToAtom is Decl-blind, so a fact
// whose predicate was never declared converts cleanly, lands in the EDB, and
// returns nil from Assert — every signal the caller has says it worked. The
// fixpoint only derives what the program declares, so Query never returns it.
// The fact is written into a space nothing reads, and nothing anywhere fails.
//
// This surfaced from a test that asserted a hundred facts concurrently, was
// told a hundred times that each Assert succeeded, read back zero, and
// reported "Concurrency lost data" — a real-sounding diagnosis of a bug that
// was not there. Checking non-test Go against the 1839 Decls a booted kernel
// loads turned up 82 more, including whole state machines
// (campaign_paused, tdd_phase, ouroboros_phase, python_snapshot) whose facts
// have never been visible to a rule.
//
// The kernel now warns once per predicate at runtime
// (RealKernel.warnIfUndeclaredLocked). Assert deliberately does not start
// returning an error: 82 live call sites would begin failing at once, and the
// honest fix for each is a per-predicate decision — declare it and wire a
// consumer, or drop the assert — not a blanket rejection.
//
// So this is a budget, like the others. It does not demand zero. It demands
// the number not grow, and that entries leave the list when they are fixed, so
// the count stays a measurement rather than a forgotten file.

const undeclaredBaselinePath = "testdata/undeclared_asserts.txt"

var (
	undeclaredDeclRe = regexp.MustCompile(`^\s*Decl\s+([a-z_][a-zA-Z0-9_]*)\s*\(`)
	// Fact literals are written as `Predicate: "name"` throughout the tree,
	// both for core.Fact and types.Fact.
	undeclaredAssertRe = regexp.MustCompile(`Predicate:\s*"([a-z_][a-zA-Z0-9_]*)"`)
)

// declaredPredicateNames collects every predicate the .mg corpus declares.
//
// Names only, not arities. The runtime warning checks name and arity together
// because a fact at an undeclared arity is invisible for the same reason, but
// this static gate cannot know an assert's arity without evaluating the Go, and
// a name-only check still catches the whole class it is aimed at.
func declaredPredicateNames(t *testing.T, root string) map[string]struct{} {
	t.Helper()
	declared := make(map[string]struct{})
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".mg") {
			return nil
		}
		data, readErr := os.ReadFile(filepath.Clean(path))
		if readErr != nil {
			return nil
		}
		for _, line := range strings.Split(string(data), "\n") {
			if m := undeclaredDeclRe.FindStringSubmatch(line); m != nil {
				declared[m[1]] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(declared) == 0 {
		t.Fatalf("found no Decl in %s — the corpus moved or the regex broke, and this gate is checking nothing", root)
	}
	return declared
}

// assertedPredicateNames collects every predicate non-test Go asserts.
func assertedPredicateNames(t *testing.T, root string) map[string]string {
	t.Helper()
	asserted := make(map[string]string)
	for _, dir := range []string{"internal", "cmd"} {
		base := filepath.Join(root, dir)
		if _, err := os.Stat(base); err != nil {
			continue
		}
		err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") || strings.Contains(filepath.ToSlash(path), "/testdata/") {
				return nil
			}
			data, readErr := os.ReadFile(filepath.Clean(path))
			if readErr != nil {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			rel = filepath.ToSlash(rel)
			for _, m := range undeclaredAssertRe.FindAllStringSubmatch(string(data), -1) {
				if _, seen := asserted[m[1]]; !seen {
					asserted[m[1]] = rel
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", base, err)
		}
	}
	if len(asserted) == 0 {
		t.Fatal("found no asserted predicates — the regex broke, and this gate is checking nothing")
	}
	return asserted
}

func currentUndeclaredAsserts(t *testing.T) ([]string, map[string]string) {
	t.Helper()
	root := repoRootFrom(t, mustGetwd(t))
	declared := declaredPredicateNames(t, root)
	asserted := assertedPredicateNames(t, root)

	var out []string
	where := make(map[string]string)
	for name, file := range asserted {
		if _, ok := declared[name]; ok {
			continue
		}
		out = append(out, name)
		where[name] = file
	}
	sort.Strings(out)
	return out, where
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return wd
}

// TestUndeclaredAssertBudget fails when Go starts asserting a predicate no .mg
// file declares, and when one on the list stops being undeclared without being
// removed from it.
func TestUndeclaredAssertBudget(t *testing.T) {
	current, where := currentUndeclaredAsserts(t)

	// Opt-in through the environment, never a flag: a gate that rewrites its
	// own baseline on failure is not a gate.
	if os.Getenv("CODENERD_UPDATE_UNDECLARED") == "1" {
		var sb strings.Builder
		sb.WriteString("# Predicates asserted from non-test Go that no .mg file declares.\n")
		sb.WriteString("#\n")
		sb.WriteString("# Each of these is a fact the code believes it recorded. Assert returns nil,\n")
		sb.WriteString("# the fact lands in the EDB, and Query can never return it, because the\n")
		sb.WriteString("# fixpoint only derives what the program declares.\n")
		sb.WriteString("#\n")
		sb.WriteString("# Fix one by declaring it AND giving it a consumer, or by dropping the assert.\n")
		sb.WriteString("# Declaring it alone just moves it to the starved-predicate list.\n")
		sb.WriteString("#\n")
		sb.WriteString("# Regenerate: CODENERD_UPDATE_UNDECLARED=1 go test ./internal/core/defaults/ -run TestUndeclaredAssertBudget\n")
		for _, p := range current {
			sb.WriteString(p)
			sb.WriteString("\n")
		}
		if err := os.WriteFile(undeclaredBaselinePath, []byte(sb.String()), 0o600); err != nil {
			t.Fatalf("write %s: %v", undeclaredBaselinePath, err)
		}
		t.Logf("rewrote %s with %d predicate(s)", undeclaredBaselinePath, len(current))
		return
	}

	data, err := os.ReadFile(undeclaredBaselinePath)
	if err != nil {
		t.Fatalf("read %s: %v\nRegenerate with: CODENERD_UPDATE_UNDECLARED=1 go test ./internal/core/defaults/ -run TestUndeclaredAssertBudget",
			undeclaredBaselinePath, err)
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
			added = append(added, p+"  ("+where[p]+")")
		}
	}
	var fixed []string
	for p := range baseline {
		if _, still := currentSet[p]; !still {
			fixed = append(fixed, p)
		}
	}
	sort.Strings(fixed)

	if len(added) > 0 {
		t.Errorf(`%d predicate(s) are asserted from Go but declared nowhere: %v

Assert will return nil and the fact will not be readable by any rule or query.
Declare the predicate and give it a consumer, or drop the assert. Declaring it
without a consumer only moves it to the starved-predicate list.

If it is deliberate, record it with:
  CODENERD_UPDATE_UNDECLARED=1 go test ./internal/core/defaults/ -run TestUndeclaredAssertBudget
and say in the commit why the fact is written but never read.`, len(added), added)
	}

	if len(fixed) > 0 {
		t.Errorf(`%d predicate(s) are no longer undeclared: %v

Good — that is the number going down. Refresh the baseline so it stays a real
measurement:
  CODENERD_UPDATE_UNDECLARED=1 go test ./internal/core/defaults/ -run TestUndeclaredAssertBudget`, len(fixed), fixed)
	}
}
