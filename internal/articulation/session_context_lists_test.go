package articulation

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// The old blackboard caps (20, and 15 for dependencies) hid lines the heading
// told the model to act on. Past those caps, every diagnostic, failing test,
// and blocked action has to be in the prompt AssembleSystemPrompt returns,
// and the "... and N more" lines have to be gone. The same bar covers the
// other session lists: none of them has a tool that lists the remainder.
func TestAssembleSystemPrompt_SessionListsRenderWhole(t *testing.T) {
	const n = 24 // above the old caps of 20 and 15
	numbered := func(prefix string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("%s-%02d", prefix, i)
		}
		return out
	}

	diags := numbered("lc16-diag")
	failing := numbered("lc16-TestFail")
	findings := numbered("lc16-finding")
	hits := numbered("lc16-reflection")
	impacted := numbered("lc16-impacted.go")
	deps := numbered("lc16-dep")
	commits := numbered("lc16-commit")
	modified := numbered("lc16-modified.go")
	actions := numbered("lc16-action")
	atoms := numbered("lc16-atom")
	hints := numbered("lc16-hint")
	blocked := numbered("lc16-blocked")
	warnings := numbered("lc16-warning")

	outputs := make([]types.ShardSummary, n)
	tools := make([]types.ToolInfo, n)
	for i := 0; i < n; i++ {
		outputs[i] = types.ShardSummary{
			ShardType: "reviewer",
			Task:      fmt.Sprintf("lc16-shardtask-%02d", i),
			Summary:   fmt.Sprintf("lc16-shardsum-%02d", i),
			Success:   i%2 == 0,
		}
		tools[i] = types.ToolInfo{
			Name:        fmt.Sprintf("lc16-tool-%02d", i),
			Description: fmt.Sprintf("lc16-tooldesc-%02d", i),
			BinaryPath:  fmt.Sprintf("lc16-bin-%02d", i),
		}
	}

	pa, err := NewPromptAssembler(newMockKernel())
	if err != nil {
		t.Fatalf("NewPromptAssembler: %v", err)
	}
	result, err := pa.AssembleSystemPrompt(context.Background(), &PromptContext{
		ShardID:   "coder-lists",
		ShardType: "coder",
		SessionCtx: &types.SessionContext{
			CurrentDiagnostics: diags,
			TestState:          "/failing",
			FailingTests:       failing,
			RecentFindings:     findings,
			ReflectionHits:     hits,
			ImpactedFiles:      impacted,
			DependencyContext:  deps,
			GitBranch:          "lc16-branch",
			GitModifiedFiles:   modified,
			GitRecentCommits:   commits,
			PriorShardOutputs:  outputs,
			RecentActions:      actions,
			KnowledgeAtoms:     atoms,
			SpecialistHints:    hints,
			AvailableTools:     tools,
			BlockedActions:     blocked,
			SafetyWarnings:     warnings,
		},
	})
	if err != nil {
		t.Fatalf("AssembleSystemPrompt: %v", err)
	}

	var want []string
	want = append(want, diags...)
	want = append(want, failing...)
	want = append(want, findings...)
	want = append(want, hits...)
	want = append(want, impacted...)
	want = append(want, deps...)
	want = append(want, commits...)
	want = append(want, modified...)
	want = append(want, actions...)
	want = append(want, atoms...)
	want = append(want, hints...)
	want = append(want, blocked...)
	want = append(want, warnings...)
	for i := 0; i < n; i++ {
		want = append(want,
			fmt.Sprintf("lc16-shardtask-%02d", i),
			fmt.Sprintf("lc16-shardsum-%02d", i),
			fmt.Sprintf("lc16-tool-%02d", i),
			fmt.Sprintf("lc16-tooldesc-%02d", i),
			fmt.Sprintf("lc16-bin-%02d", i),
		)
	}
	for _, item := range want {
		if !strings.Contains(result, item) {
			t.Errorf("assembled prompt lost %q", item)
		}
	}

	for _, banned := range []string{
		"more build/lint errors",
		"more failing tests",
		"more findings",
		"more reflection hits",
		"more impacted files",
		"more commits",
		"more prior shard results",
		"more recent actions",
		"more knowledge atoms",
		"more hints",
		"more tools",
		"more blocked actions",
		"more safety warnings",
		"... and",
	} {
		if strings.Contains(result, banned) {
			t.Errorf("assembled prompt still hides a list behind %q", banned)
		}
	}
	if types.IsClamped(result) {
		t.Error("a session past the old count caps but under the block ceiling was truncated")
	}
}

// Modified paths are part of the git section even when no branch and no
// commit was recorded. The old gate required one of those, so the names
// never rendered.
func TestAssembleSystemPrompt_ModifiedFilesRenderWithoutBranch(t *testing.T) {
	pa, err := NewPromptAssembler(newMockKernel())
	if err != nil {
		t.Fatalf("NewPromptAssembler: %v", err)
	}
	result, err := pa.AssembleSystemPrompt(context.Background(), &PromptContext{
		ShardID:   "coder-git",
		ShardType: "coder",
		SessionCtx: &types.SessionContext{
			GitModifiedFiles: []string{"internal/articulation/prompt_assembler.go", "internal/types/types.go"},
		},
	})
	if err != nil {
		t.Fatalf("AssembleSystemPrompt: %v", err)
	}
	for _, want := range []string{
		"GIT CONTEXT",
		"Modified files: 2",
		"internal/articulation/prompt_assembler.go",
		"internal/types/types.go",
	} {
		if !strings.Contains(result, want) {
			t.Errorf("assembled prompt lost %q\n%s", want, result)
		}
	}
}

// With the lists uncapped, the block ceiling (32 KiB, head two thirds and tail
// one third, middle dropped) fires on an ordinary long tool list. Safety
// constraints used to sit near the end and survived only while they fit in
// the ~10.9 KiB tail. Here they are ~18 KiB -- more than that tail, less than
// the head -- behind ~90 KiB of tools: every BLOCKED and WARNING line must
// still be in the prompt, and ahead of the tools.
func TestAssembleSystemPrompt_SafetyConstraintsSurviveTheBlockCeiling(t *testing.T) {
	pad := strings.Repeat("x", 400)
	tools := make([]types.ToolInfo, 200)
	for i := range tools {
		tools[i] = types.ToolInfo{
			Name:        fmt.Sprintf("ceil-tool-%03d", i),
			Description: fmt.Sprintf("ceil-tooldesc-%03d %s", i, pad),
		}
	}
	blocked := make([]string, 30)
	for i := range blocked {
		blocked[i] = fmt.Sprintf("ceil-blocked-%02d %s", i, pad)
	}
	warnings := make([]string, 10)
	for i := range warnings {
		warnings[i] = fmt.Sprintf("ceil-warning-%02d %s", i, pad)
	}

	pa, err := NewPromptAssembler(newMockKernel())
	if err != nil {
		t.Fatalf("NewPromptAssembler: %v", err)
	}
	result, err := pa.AssembleSystemPrompt(context.Background(), &PromptContext{
		ShardID:   "coder-ceiling",
		ShardType: "coder",
		SessionCtx: &types.SessionContext{
			AvailableTools: tools,
			BlockedActions: blocked,
			SafetyWarnings: warnings,
		},
	})
	if err != nil {
		t.Fatalf("AssembleSystemPrompt: %v", err)
	}
	if !strings.Contains(result, "[codenerd: truncated") {
		t.Fatal("the fixture must exceed the block ceiling, or it does not test the clamp")
	}
	for i := range blocked {
		if id := fmt.Sprintf("ceil-blocked-%02d", i); !strings.Contains(result, id) {
			t.Errorf("blocked action %s was dropped by the block ceiling", id)
		}
	}
	for i := range warnings {
		if id := fmt.Sprintf("ceil-warning-%02d", i); !strings.Contains(result, id) {
			t.Errorf("safety warning %s was dropped by the block ceiling", id)
		}
	}
	if s, tl := strings.Index(result, "SAFETY CONSTRAINTS:"), strings.Index(result, "ceil-tool-000"); s < 0 || tl < 0 || s > tl {
		t.Errorf("safety constraints must precede the tool list (safety at %d, first tool at %d)", s, tl)
	}
}
