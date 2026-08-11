package main

import (
	"strings"
	"testing"
	"time"
)

// The defect these guard (F-DREAM-1, observed live): `nerd dream` consulted 22
// agents, every one failed with "context deadline exceeded", and the command
// printed "✅ Dream state consultation complete" and exited 0. Nothing in the
// output or the exit code distinguished a total failure from a clean run, so
// no script and no unattended operator could tell.
//
// Three faults produced it, all fixed together: a hardcoded 5-minute ceiling
// that ignored --timeout, a strictly sequential fan-out that shared that one
// budget across every agent, and this summary, which never consulted its own
// results.
//
// The all-failed branch is now hard to reach live — the concurrent fan-out
// completes several agents even on a 32-second budget — which is precisely how
// the original bug survived unnoticed.

func TestDreamSummary_AllFailedIsAnError(t *testing.T) {
	msg, err := dreamSummary(0, 16, 5*time.Minute, true)

	if err == nil {
		t.Fatal("16 of 16 agents failed and the command would still exit 0")
	}
	if !strings.Contains(msg, "FAILED") {
		t.Errorf("summary does not report failure: %q", msg)
	}
	if strings.Contains(msg, "✅") {
		t.Errorf("summary claims success after total failure: %q", msg)
	}
}

// When the budget is what killed the run, say so and name the flag — a bare
// "context deadline exceeded" per agent gives an unattended operator nothing
// to act on.
func TestDreamSummary_NamesTheBudgetWhenTheDeadlineExpired(t *testing.T) {
	msg, _ := dreamSummary(0, 16, 75*time.Second, true)

	if !strings.Contains(msg, "1m15s") {
		t.Errorf("summary does not report the budget that expired: %q", msg)
	}
	if !strings.Contains(msg, "--timeout") {
		t.Errorf("summary does not name the flag that fixes it: %q", msg)
	}
}

// A total failure with no expired deadline is a different problem, and
// blaming the budget would send the reader in the wrong direction.
func TestDreamSummary_DoesNotBlameTheBudgetWhenItDidNotExpire(t *testing.T) {
	msg, err := dreamSummary(0, 3, 25*time.Minute, false)

	if err == nil {
		t.Fatal("total failure must still be an error")
	}
	if strings.Contains(msg, "--timeout") {
		t.Errorf("summary blames the timeout for a non-timeout failure: %q", msg)
	}
}

// Partial success is not failure — some agents produced usable perspectives —
// but it must not read as a clean run either. Verified live at a 75s budget:
// 7 succeeded, 9 failed.
func TestDreamSummary_PartialIsReportedAndNotAnError(t *testing.T) {
	msg, err := dreamSummary(7, 9, 75*time.Second, true)

	if err != nil {
		t.Errorf("partial success became a hard error: %v", err)
	}
	if !strings.Contains(msg, "7") || !strings.Contains(msg, "9") {
		t.Errorf("summary hides the split: %q", msg)
	}
	if strings.Contains(msg, "✅") {
		t.Errorf("partial run claims a clean result: %q", msg)
	}
}

func TestDreamSummary_FullSuccess(t *testing.T) {
	msg, err := dreamSummary(16, 0, 25*time.Minute, false)

	if err != nil {
		t.Errorf("a clean run returned an error: %v", err)
	}
	if !strings.Contains(msg, "✅") || !strings.Contains(msg, "16") {
		t.Errorf("clean run summary is wrong: %q", msg)
	}
}

// Zero agents consulted is not success. Reporting it as one would recreate the
// original defect in a different shape.
func TestDreamSummary_NoAgentsIsNotSuccess(t *testing.T) {
	msg, err := dreamSummary(0, 0, 25*time.Minute, false)

	if err != nil {
		t.Errorf("no agents available should not be a hard error: %v", err)
	}
	if strings.Contains(msg, "✅") {
		t.Errorf("consulting nothing reported as success: %q", msg)
	}
}

// Dream relevance ranking: the most topically matching agent should be first.
//
// Scenario about bubbletea terminal UI should rank the bubbletea expert above
// go and rod experts, proving the overlap scorer works. This is the pure,
// filesystem-free function required by the task.
func TestDreamRanking_PutsTopicalFirst(t *testing.T) {
	scenario := "implement bubbletea terminal UI with lipgloss styling and bubbles components"
	metas := []dreamAgentMeta{
		{Name: "goexpert", Role: "Expert in Go idioms, concurrency patterns, and standard library", Topics: []string{"go concurrency", "go error handling", "go interfaces", "go testing"}},
		{Name: "bubbleteaexpert", Role: "Expert in Bubbletea TUI framework, Elm architecture, and terminal rendering", Topics: []string{"bubbletea", "elm architecture", "terminal UI", "lipgloss styling", "bubbles components"}},
		{Name: "rodexpert", Role: "Expert in Rod browser automation, selectors, and CDP protocol", Topics: []string{"rod browser automation", "CDP protocol", "web scraping", "headless chrome", "page selectors"}},
	}
	ranked := rankDreamAgents(scenario, metas)
	if len(ranked) != 3 {
		t.Fatalf("ranked %d agents, want 3", len(ranked))
	}
	if ranked[0].Meta.Name != "bubbleteaexpert" {
		t.Errorf("top ranked agent is %q (score %d), want bubbleteaexpert; full order: %v", ranked[0].Meta.Name, ranked[0].Score, ranked)
	}
	if ranked[0].Score == 0 {
		t.Errorf("top agent scored 0, want >0 for topical match")
	}
	if ranked[1].Score > ranked[0].Score {
		t.Errorf("ranking not descending: %v", ranked)
	}
}

// --max-agents caps how many are consulted even when more are relevant.
func TestDreamMaxAgents_CapsSelection(t *testing.T) {
	scenario := "bubbletea terminal lipgloss bubbles"
	metas := []dreamAgentMeta{
		{Name: "a", Role: "bubbletea expert", Topics: []string{"bubbletea"}},
		{Name: "b", Role: "bubbletea terminal", Topics: []string{"terminal"}},
		{Name: "c", Role: "lipgloss styling", Topics: []string{"lipgloss"}},
		{Name: "d", Role: "bubbles components", Topics: []string{"bubbles"}},
		{Name: "e", Role: "elm architecture", Topics: []string{"elm"}},
	}
	selected := selectDreamAgents(scenario, metas, 2, false)
	if len(selected) != 2 {
		t.Fatalf("max-agents 2 selected %d agents, want 2", len(selected))
	}
	// Should be the two highest scoring.
	ranked := rankDreamAgents(scenario, metas)
	want := ranked[0].Meta.Name
	if selected[0].Meta.Name != want {
		t.Errorf("capped selection first is %q, want top ranked %q", selected[0].Meta.Name, want)
	}
}

// All-zero scores still select up to the cap rather than none.
func TestDreamAllZeroScores_SelectsUpToCap(t *testing.T) {
	scenario := "quantum entanglement photon coherence"
	metas := []dreamAgentMeta{
		{Name: "goexpert", Role: "Expert in Go idioms, concurrency patterns", Topics: []string{"go concurrency", "go error handling"}},
		{Name: "bubbleteaexpert", Role: "Expert in Bubbletea TUI framework", Topics: []string{"bubbletea", "lipgloss styling"}},
		{Name: "rodexpert", Role: "Expert in Rod browser automation", Topics: []string{"rod browser automation", "CDP protocol"}},
	}
	for _, m := range metas {
		if s := dreamRelevanceScore(scenario, m); s != 0 {
			t.Fatalf("expected zero score for %q, got %d", m.Name, s)
		}
	}
	selected := selectDreamAgents(scenario, metas, 2, false)
	if len(selected) != 2 {
		t.Fatalf("all-zero scores selected %d agents, want 2 (cap)", len(selected))
	}
	if selected[0].Score != 0 || selected[1].Score != 0 {
		t.Errorf("expected zero scores in capped fallback, got %v", selected)
	}
}

// --all bypasses ranking entirely and consults every consultable agent.
func TestDreamAllFlag_BypassesRanking(t *testing.T) {
	scenario := "implement bubbletea terminal UI with lipgloss styling and bubbles components"
	metas := []dreamAgentMeta{
		{Name: "goexpert", Role: "Expert in Go idioms, concurrency patterns", Topics: []string{"go concurrency"}},
		{Name: "rodexpert", Role: "Expert in Rod browser automation", Topics: []string{"rod browser automation"}},
		{Name: "bubbleteaexpert", Role: "Expert in Bubbletea TUI framework, Elm architecture, and terminal rendering", Topics: []string{"bubbletea", "elm architecture", "terminal UI", "lipgloss styling", "bubbles components"}},
	}
	// With max-agents 1 and all=false only the top agent would be selected.
	capped := selectDreamAgents(scenario, metas, 1, false)
	if len(capped) != 1 {
		t.Fatalf("capped selection len %d, want 1", len(capped))
	}
	if capped[0].Meta.Name != "bubbleteaexpert" {
		t.Errorf("capped top is %q, want bubbleteaexpert", capped[0].Meta.Name)
	}
	// With --all, every agent is consulted regardless of cap and ranking is bypassed (original order).
	all := selectDreamAgents(scenario, metas, 1, true)
	if len(all) != len(metas) {
		t.Fatalf("--all selected %d agents, want %d (all)", len(all), len(metas))
	}
	if all[0].Meta.Name != metas[0].Name || all[1].Meta.Name != metas[1].Name || all[2].Meta.Name != metas[2].Name {
		t.Errorf("--all did not preserve original order (ranking bypass): got %v, want %v", []string{all[0].Meta.Name, all[1].Meta.Name, all[2].Meta.Name}, []string{metas[0].Name, metas[1].Name, metas[2].Name})
	}
}
