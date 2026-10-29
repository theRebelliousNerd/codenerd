package core

import (
	"sort"
	"testing"

	"codenerd/internal/types"
)

// expertPolicyKernel boots the production corpus and asserts a two-agent
// roster: a Mangle expert (language domain) and a Bubble Tea expert (an import
// domain), plus the import root the Bubble Tea domain is matched through.
func expertPolicyKernel(t *testing.T) *RealKernel {
	t.Helper()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	facts := []types.Fact{
		{Predicate: "shard_agent", Args: []any{types.MangleAtom("/mangleexpert")}},
		{Predicate: "shard_agent", Args: []any{types.MangleAtom("/bubbleteaexpert")}},
		{Predicate: "shard_agent_domain", Args: []any{types.MangleAtom("/mangleexpert"), types.MangleAtom("/mangle"), types.MangleAtom("/registry")}},
		{Predicate: "shard_agent_domain", Args: []any{types.MangleAtom("/bubbleteaexpert"), types.MangleAtom("/bubbletea"), types.MangleAtom("/registry")}},
		{Predicate: "technology_import_root", Args: []any{"github.com/charmbracelet/bubbletea", types.MangleAtom("/bubbletea")}},
		// A file in the world model that imports Bubble Tea, in a package dir.
		{Predicate: "dependency_link", Args: []any{"cmd/nerd/chat/view.go", "pkg:github.com/charmbracelet/bubbletea", "github.com/charmbracelet/bubbletea"}},
		{Predicate: "file_dir", Args: []any{"cmd/nerd/chat/view.go", "cmd/nerd/chat"}},
	}
	if err := k.AssertBatch(facts); err != nil {
		t.Fatalf("assert roster: %v", err)
	}
	return k
}

func derivedFor(t *testing.T, k *RealKernel, predicate, task string, agentArg int) []string {
	t.Helper()
	rows, err := k.Query(predicate)
	if err != nil {
		t.Fatalf("query %s: %v", predicate, err)
	}
	var out []string
	for _, f := range rows {
		taskArg := 0
		if predicate == "specialist_match" || predicate == "specialist_should_execute" || predicate == "specialist_should_advise" {
			taskArg = 1
		}
		if types.ExtractString(f.Args[taskArg]) != task {
			continue
		}
		out = append(out, types.ExtractString(f.Args[agentArg]))
	}
	sort.Strings(out)
	return out
}

// A task aimed at a policy file needs the Mangle expert, whatever the verb
// that reads or changes code; a greeting needs none; a Bubble Tea import in a
// targeted package directory needs the Bubble Tea expert, strongly enough to
// execute, while a language match only pushes knowledge.
func TestShardExpertPolicyDerivesExpertNeeded(t *testing.T) {
	k := expertPolicyKernel(t)
	task := []types.Fact{
		{Predicate: "expert_task", Args: []any{"t-mg", types.MangleAtom("/fix")}},
		{Predicate: "expert_task_target", Args: []any{"t-mg", "internal/core/defaults/policy/shards.mg"}},
		{Predicate: "file_extension", Args: []any{"internal/core/defaults/policy/shards.mg", ".mg"}},
		{Predicate: "expert_task", Args: []any{"t-explain", types.MangleAtom("/explain")}},
		{Predicate: "expert_task_target", Args: []any{"t-explain", "internal/core/defaults/policy/shards.mg"}},
		{Predicate: "expert_task", Args: []any{"t-greet", types.MangleAtom("/greet")}},
		{Predicate: "expert_task_target", Args: []any{"t-greet", "internal/core/defaults/policy/shards.mg"}},
		{Predicate: "expert_task", Args: []any{"t-tui", types.MangleAtom("/refactor")}},
		{Predicate: "expert_task_target", Args: []any{"t-tui", "cmd/nerd/chat"}},
		{Predicate: "expert_task", Args: []any{"t-md", types.MangleAtom("/fix")}},
		{Predicate: "expert_task_target", Args: []any{"t-md", "README.md"}},
		{Predicate: "file_extension", Args: []any{"README.md", ".md"}},
	}
	if err := k.AssertBatch(task); err != nil {
		t.Fatalf("assert tasks: %v", err)
	}

	for _, tc := range []struct {
		task string
		want []string
	}{
		{"t-mg", []string{"/mangleexpert"}},
		{"t-explain", []string{"/mangleexpert"}},
		{"t-greet", nil},
		{"t-tui", []string{"/bubbleteaexpert"}},
		{"t-md", nil},
	} {
		got := derivedFor(t, k, "expert_needed", tc.task, 1)
		if len(got) != len(tc.want) || (len(got) > 0 && got[0] != tc.want[0]) {
			t.Errorf("expert_needed for %s = %v, want %v", tc.task, got, tc.want)
		}
	}

	if got := derivedFor(t, k, "expert_file", "t-tui", 2); len(got) != 1 || got[0] != "cmd/nerd/chat/view.go" {
		t.Errorf("expert_file for the directory task = %v, want the importing file", got)
	}
	// policy/shards.mg classifies /bubbleteaexpert as an executor, so a
	// framework-import match hands it the task; the language-only match on
	// the Mangle file does not.
	if got := derivedFor(t, k, "specialist_should_execute", "t-tui", 0); len(got) != 1 || got[0] != "/bubbleteaexpert" {
		t.Errorf("specialist_should_execute for the import match = %v, want [/bubbleteaexpert]", got)
	}
	if got := derivedFor(t, k, "specialist_should_execute", "t-mg", 0); len(got) != 0 {
		t.Errorf("specialist_should_execute for a language-only match = %v, want none", got)
	}
	rows, err := k.Query("specialist_match")
	if err != nil {
		t.Fatalf("query specialist_match: %v", err)
	}
	confidence := map[string]int64{}
	for _, f := range rows {
		n, _ := f.Args[2].(int64)
		confidence[types.ExtractString(f.Args[1])+" "+types.ExtractString(f.Args[0])] = n
	}
	if confidence["t-tui /bubbleteaexpert"] != 90 {
		t.Errorf("an import match scores %d, want 90 (executes)", confidence["t-tui /bubbleteaexpert"])
	}
	if confidence["t-mg /mangleexpert"] != 40 {
		t.Errorf("a language match scores %d, want 40 (knowledge only)", confidence["t-mg /mangleexpert"])
	}
	if got := derivedFor(t, k, "specialist_should_advise", "t-mg", 0); len(got) != 0 {
		t.Errorf("specialist_should_advise for a language-only match = %v, want none", got)
	}
}

// An agent that is not on the roster (not ready) is never needed, even when
// its domain matches.
func TestShardExpertPolicyNeedsARosterAgent(t *testing.T) {
	k := expertPolicyKernel(t)
	facts := []types.Fact{
		{Predicate: "shard_agent_domain", Args: []any{types.MangleAtom("/goneexpert"), types.MangleAtom("/mangle"), types.MangleAtom("/knowledge_base")}},
		{Predicate: "expert_task", Args: []any{"t", types.MangleAtom("/fix")}},
		{Predicate: "expert_task_target", Args: []any{"t", "a.mg"}},
		{Predicate: "file_extension", Args: []any{"a.mg", ".mg"}},
	}
	if err := k.AssertBatch(facts); err != nil {
		t.Fatalf("assert: %v", err)
	}
	if got := derivedFor(t, k, "expert_needed", "t", 1); len(got) != 1 || got[0] != "/mangleexpert" {
		t.Errorf("expert_needed = %v, want only the roster agent", got)
	}
}
