package world

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	working "codenerd/internal/context"
	"codenerd/internal/core"
)

// impactFixture builds a target with n same-shaped prioritized callers:
// Fn00.. with raw priority 3 (rendered as priority 100, depth 1), so every
// rendered line is the same length and the test knows the mean exactly.
func impactFixture(t *testing.T, n int) (*HolographicProvider, string) {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	if err := os.WriteFile(target, []byte("package p\n\nfunc Target() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	facts := make([]core.Fact, 0, n)
	for i := 0; i < n; i++ {
		facts = append(facts, core.Fact{
			Predicate: "context_priority_file",
			Args:      []any{fmt.Sprintf("f%d.go", i), fmt.Sprintf("Fn%02d", i), int64(3)},
		})
	}
	h := NewHolographicProvider(&stubQuerier{facts: map[string][]core.Fact{
		"context_priority_file": facts,
	}}, dir)
	return h, target
}

// budgetedSection renders through a real working set, recording the
// measurements the renderer asserted. share is the config's caller share,
// budget the session's render budget in bytes.
func budgetedSection(t *testing.T, h *HolographicProvider, target string, share, budget int) (section string, total, avg, seenBudget int) {
	t.Helper()
	spans := config.DefaultWorkingConfig()
	spans.HolographicCallerSharePercent = share
	set, err := working.NewWorkingSet(t.TempDir(), "budget", spans)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = set.Close() })
	decide := func(ctx context.Context, tgt string, n, mean, b int) (int, error) {
		total, avg, seenBudget = n, mean, b
		if tgt != target {
			t.Errorf("decider target = %q, want %q", tgt, target)
		}
		return set.DecideCallerLimit(ctx, tgt, n, mean, b)
	}
	return h.PromptSectionWithCallerBudget(context.Background(), target, budget, decide), total, avg, seenBudget
}

func countCallerLines(t *testing.T, section, prefix string) int {
	t.Helper()
	n := 0
	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(line, prefix) {
			n++
		}
	}
	return n
}

// Few callers under a budget that fits them render whole, with no remainder
// line. Two callers at 45 bytes need 90 of the 100-byte allowance (1000
// bytes at a 10% share).
func TestBudgetedCallers_FewRenderWhole(t *testing.T) {
	h, target := impactFixture(t, 2)
	section, total, avg, budget := budgetedSection(t, h, target, 10, 1000)
	if total != 2 || budget != 1000 {
		t.Fatalf("measured pool = (%d callers, %d-byte budget), want (2, 1000)", total, budget)
	}
	wantLine := "- `Fn00` — `f0.go` (priority 100, depth 1)\n"
	if avg != len(wantLine) {
		t.Fatalf("measured mean = %d bytes, want the rendered line's %d", avg, len(wantLine))
	}
	if !strings.Contains(section, wantLine) || !strings.Contains(section, "- `Fn01`") {
		t.Fatalf("both callers must render:\n%s", section)
	}
	if strings.Contains(section, "more callers") {
		t.Fatalf("a pool that fits states a remainder:\n%s", section)
	}
}

// Many callers under a small budget render the derived N: the top of the
// existing ranking, the true remainder, and the tool that reads the rest.
// Ten callers at 45 bytes need 450 of a 100-byte allowance, so 100/45 = 2
// render (Fn00, Fn01 by the name tiebreak) and 8 are omitted.
func TestBudgetedCallers_SmallBudgetRendersDerivedN(t *testing.T) {
	h, target := impactFixture(t, 10)
	section, total, avg, _ := budgetedSection(t, h, target, 10, 1000)
	if total != 10 {
		t.Fatalf("measured pool = %d callers, want 10", total)
	}
	if avg != len("- `Fn00` — `f0.go` (priority 100, depth 1)\n") {
		t.Fatalf("measured mean = %d bytes, want one rendered line", avg)
	}
	if got := countCallerLines(t, section, "- `Fn"); got != 2 {
		t.Fatalf("rendered %d caller lines, want the derived 2:\n%s", got, section)
	}
	if !strings.Contains(section, "- `Fn00`") || !strings.Contains(section, "- `Fn01`") {
		t.Fatalf("the top of the ranking must render:\n%s", section)
	}
	if strings.Contains(section, "- `Fn02`") {
		t.Fatalf("Fn02 renders past the derived count:\n%s", section)
	}
	if want := "and 8 more callers; `callers_of` lists every call site"; !strings.Contains(section, want) {
		t.Fatalf("missing the true remainder %q:\n%s", want, section)
	}
	if strings.Contains(section, "not stored here") {
		t.Fatalf("prioritized remainder was mixed with the call-graph storage cap:\n%s", section)
	}
}

// The call-graph fallback branch sizes the same way: 30 stored callers at 14
// bytes need 420 of a 100-byte allowance, so 100/14 = 7 render and the
// remainder counts the full walk (30) minus what is shown.
func TestBudgetedCallers_CallGraphFallbackRendersDerivedN(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	if err := os.WriteFile(target, []byte("package p\n\nfunc Target() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const edges = 30
	calls := make([]core.Fact, 0, edges)
	for i := 0; i < edges; i++ {
		calls = append(calls, core.Fact{Predicate: "code_calls", Args: []any{fmt.Sprintf("caller_%02d", i), "p.Target"}})
	}
	h := NewHolographicProvider(&stubQuerier{facts: map[string][]core.Fact{
		"code_defines": {{
			Predicate: "code_defines",
			Args:      []any{target, "p.Target", "/function", int64(3), int64(3)},
		}},
		"code_calls": calls,
	}}, dir)
	section, total, avg, _ := budgetedSection(t, h, target, 10, 1000)
	if total != edges {
		t.Fatalf("measured pool = %d callers, want %d", total, edges)
	}
	if avg != len("- `caller_00`\n") {
		t.Fatalf("measured mean = %d bytes, want one rendered line", avg)
	}
	if got := countCallerLines(t, section, "- `caller_"); got != 7 {
		t.Fatalf("rendered %d caller lines, want the derived 7:\n%s", got, section)
	}
	if want := "and 23 more callers; `callers_of` lists every call site"; !strings.Contains(section, want) {
		t.Fatalf("missing the true remainder %q:\n%s", want, section)
	}
}

// A decider failure withholds the block behind its honest remainder line
// instead of guessing a count: no caller renders, and the line still states
// the true rest and names callers_of.
func TestBudgetedCallers_DeciderFailureWithholdsBehindRemainder(t *testing.T) {
	h, target := impactFixture(t, 10)
	section := h.PromptSectionWithCallerBudget(context.Background(), target, 1000,
		func(context.Context, string, int, int, int) (int, error) {
			return 0, fmt.Errorf("engine unavailable")
		})
	if got := countCallerLines(t, section, "- `Fn"); got != 0 {
		t.Fatalf("a failed decision rendered %d caller lines:\n%s", got, section)
	}
	if want := "and 10 more callers; `callers_of` lists every call site"; !strings.Contains(section, want) {
		t.Fatalf("missing the withholding remainder %q:\n%s", want, section)
	}
}
