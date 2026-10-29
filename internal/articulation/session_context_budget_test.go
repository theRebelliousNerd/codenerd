package articulation

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// The blackboard ceiling is a share of the prompt budget the assembler holds.
// A bigger budget keeps more of the same block; a small one clamps it, with
// the marker, and the safety lines — written first — are still all there.
func TestAssembleSystemPrompt_ConfiguredBudgetChangesTheCeiling(t *testing.T) {
	const (
		smallBudget = 4000
		bigBudget   = 80000
	)
	share := config.DefaultArticulationConfig().SessionContextSharePercent
	smallCeiling := smallBudget * share * config.BytesPerToken / 100
	bigCeiling := bigBudget * share * config.BytesPerToken / 100
	if smallCeiling >= bigCeiling {
		t.Fatalf("ceilings did not grow with the budget: small %d, big %d", smallCeiling, bigCeiling)
	}

	history := strings.Repeat("H", 1499) + "Z" // 1500 chars: the old gate omitted this
	if len(history) != 1500 {
		t.Fatalf("history len = %d, want 1500", len(history))
	}
	// Between the two ceilings, so only the small budget clamps it.
	body := strings.Repeat("b", smallCeiling+2048)
	blocked := []string{"budget-blocked-a", "budget-blocked-b"}
	warnings := []string{"budget-warning-a"}

	assemble := func(t *testing.T, budget int) string {
		t.Helper()
		pa, err := NewPromptAssembler(newMockKernel())
		if err != nil {
			t.Fatalf("NewPromptAssembler: %v", err)
		}
		pa.SetJITBudgets(budget, 1, 1, 1)
		if got, want := pa.sessionContextCharCeiling(), budget*share*config.BytesPerToken/100; got != want {
			t.Fatalf("ceiling = %d, want %d from budget %d", got, want, budget)
		}
		result, err := pa.AssembleSystemPrompt(context.Background(), &PromptContext{
			ShardID:   "coder-budget",
			ShardType: "coder",
			SessionCtx: &types.SessionContext{
				BlockedActions:    blocked,
				SafetyWarnings:    warnings,
				RecentActions:     []string{body},
				CompressedHistory: history,
			},
		})
		if err != nil {
			t.Fatalf("AssembleSystemPrompt: %v", err)
		}
		return result
	}

	small := assemble(t, smallBudget)
	if !strings.Contains(small, "[codenerd: truncated") || !strings.Contains(small, "compressed history") {
		t.Fatal("a small budget must clamp the block, and the marker must name the compressed history")
	}
	for _, id := range append(append([]string{}, blocked...), warnings...) {
		if !strings.Contains(small, id) {
			t.Errorf("small budget dropped safety line %q", id)
		}
	}
	if s, act := strings.Index(small, "SAFETY CONSTRAINTS:"), strings.Index(small, "RECENT SESSION ACTIONS:"); s < 0 || act < 0 || s > act {
		t.Errorf("safety must stay ahead of the clamped body (safety at %d, actions at %d)", s, act)
	}

	big := assemble(t, bigBudget)
	if types.IsClamped(big) {
		t.Fatal("the same block fit a larger budget and was still clamped")
	}
	if !strings.Contains(big, history) || !strings.Contains(big, body) {
		t.Fatal("a larger budget did not keep the history and the body whole")
	}
	if !strings.Contains(big, "SESSION HISTORY (compressed)") {
		t.Fatal("compressed history was omitted")
	}
}

// len >= 1500 used to drop CompressedHistory with no marker. It is rendered
// whole when the ceiling holds it, and a ceiling that cannot hold it clamps
// the block and says the history was in what was cut.
func TestAssembleSystemPrompt_CompressedHistoryIsNeverOmitted(t *testing.T) {
	pa, err := NewPromptAssembler(newMockKernel())
	if err != nil {
		t.Fatalf("NewPromptAssembler: %v", err)
	}
	history := strings.Repeat("H", 1499) + "Z"
	result, err := pa.AssembleSystemPrompt(context.Background(), &PromptContext{
		ShardID:   "coder-history",
		ShardType: "coder",
		SessionCtx: &types.SessionContext{
			CompressedHistory: history,
		},
	})
	if err != nil {
		t.Fatalf("AssembleSystemPrompt: %v", err)
	}
	if !strings.Contains(result, history) || !strings.Contains(result, "SESSION HISTORY (compressed)") {
		t.Fatal("a 1500-character history was omitted")
	}
	if types.IsClamped(result) {
		t.Fatal("a 1500-character history was clamped on the default budget")
	}

	pa.SetJITBudgets(4000, 1, 1, 1)
	prefix := "HISTORY-HEAD-MARK-"
	suffix := "-HISTORY-TAIL-MARK"
	long := prefix + strings.Repeat("h", pa.sessionContextCharCeiling()+4096) + suffix
	clamped, err := pa.AssembleSystemPrompt(context.Background(), &PromptContext{
		ShardID:   "coder-history-long",
		ShardType: "coder",
		SessionCtx: &types.SessionContext{
			BlockedActions:    []string{"history-blocked-a"},
			CompressedHistory: long,
		},
	})
	if err != nil {
		t.Fatalf("AssembleSystemPrompt: %v", err)
	}
	if strings.Contains(clamped, long) {
		t.Fatal("a history past the ceiling was kept whole; the clamp did not fire")
	}
	if !strings.Contains(clamped, "[codenerd: truncated") || !strings.Contains(clamped, "compressed history") {
		t.Fatal("the clamp marker did not say the block included compressed history")
	}
	if !strings.Contains(clamped, suffix) {
		t.Fatal("the ceiling dropped the tail of the history without keeping its end")
	}
	if !strings.Contains(clamped, "history-blocked-a") {
		t.Fatal("safety line was dropped when history was clamped")
	}
}

// Kernel-injected rows follow jit.kernel_context_rows and
// jit.kernel_context_row_chars, the same keys the JIT path reads.
func TestAssembleSystemPrompt_KernelRowsFollowJITConfig(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		rows := config.DefaultJITConfig().KernelContextRows
		mk := newMockKernel()
		for i := 0; i < rows+1; i++ {
			mk.addFact("injectable_context", "coder-rows", fmt.Sprintf("kc-row-%03d", i))
		}
		pa, err := NewPromptAssembler(mk)
		if err != nil {
			t.Fatalf("NewPromptAssembler: %v", err)
		}
		result, err := pa.AssembleSystemPrompt(context.Background(), &PromptContext{
			ShardID:   "coder-rows",
			ShardType: "coder",
		})
		if err != nil {
			t.Fatalf("AssembleSystemPrompt: %v", err)
		}
		if !strings.Contains(result, "kc-row-000") {
			t.Fatal("the first kernel row was dropped")
		}
		lastShown := fmt.Sprintf("kc-row-%03d", rows-1)
		omitted := fmt.Sprintf("kc-row-%03d", rows)
		if !strings.Contains(result, lastShown) {
			t.Errorf("row %s was omitted under the default cap %d", lastShown, rows)
		}
		if strings.Contains(result, omitted) {
			t.Errorf("row %s was shown past jit.kernel_context_rows %d", omitted, rows)
		}
		notice := fmt.Sprintf("[codenerd: truncated 1 of %d injectable_context rows]", rows+1)
		if !strings.Contains(result, notice) {
			t.Errorf("missing truncation notice %q", notice)
		}
	})

	t.Run("configured rows", func(t *testing.T) {
		limits := config.DefaultJITConfig()
		limits.KernelContextRows = 3
		mk := newMockKernel()
		for i := 0; i < 7; i++ {
			mk.addFact("injectable_context", "coder-rows", fmt.Sprintf("kc-cfg-%02d", i))
		}
		pa, err := NewPromptAssembler(mk)
		if err != nil {
			t.Fatalf("NewPromptAssembler: %v", err)
		}
		pa.SetKernelContextLimits(limits)
		result, err := pa.AssembleSystemPrompt(context.Background(), &PromptContext{
			ShardID:   "coder-rows",
			ShardType: "coder",
		})
		if err != nil {
			t.Fatalf("AssembleSystemPrompt: %v", err)
		}
		for _, id := range []string{"kc-cfg-00", "kc-cfg-01", "kc-cfg-02"} {
			if !strings.Contains(result, id) {
				t.Errorf("configured cap of 3 dropped %s", id)
			}
		}
		if strings.Contains(result, "kc-cfg-03") {
			t.Error("kc-cfg-03 was shown past jit.kernel_context_rows 3")
		}
		if !strings.Contains(result, "[codenerd: truncated 4 of 7 injectable_context rows]") {
			t.Error("missing the configured-cap truncation notice")
		}
	})

	t.Run("configured row chars", func(t *testing.T) {
		limits := config.DefaultJITConfig()
		limits.KernelContextRowChars = 20
		mk := newMockKernel()
		mk.addFact("injectable_context", "coder-rows", strings.Repeat("R", 200))
		pa, err := NewPromptAssembler(mk)
		if err != nil {
			t.Fatalf("NewPromptAssembler: %v", err)
		}
		pa.SetKernelContextLimits(limits)
		result, err := pa.AssembleSystemPrompt(context.Background(), &PromptContext{
			ShardID:   "coder-rows",
			ShardType: "coder",
		})
		if err != nil {
			t.Fatalf("AssembleSystemPrompt: %v", err)
		}
		if strings.Contains(result, strings.Repeat("R", 21)) {
			t.Fatal("a row longer than jit.kernel_context_row_chars was kept whole")
		}
		if !strings.Contains(result, strings.Repeat("R", 20)) || !strings.Contains(result, "injectable_context row") {
			t.Fatal("the row clamp did not keep the head and name the cut")
		}
	})

	// Boot copies jit.kernel_context_rows onto the compiler the assembler
	// already holds. The legacy block reads that config, not a second literal.
	t.Run("attached compiler", func(t *testing.T) {
		jit, err := prompt.NewJITPromptCompiler()
		if err != nil {
			t.Fatalf("NewJITPromptCompiler: %v", err)
		}
		t.Cleanup(func() { _ = jit.Close() })
		cfg := jit.GetConfig()
		cfg.KernelContextRows = 4
		jit.SetConfig(cfg)

		mk := newMockKernel()
		for i := 0; i < 6; i++ {
			mk.addFact("injectable_context", "coder-rows", fmt.Sprintf("kc-jit-%02d", i))
		}
		pa, err := NewPromptAssembler(mk)
		if err != nil {
			t.Fatalf("NewPromptAssembler: %v", err)
		}
		pa.SetJITCompiler(jit)
		pa.EnableJIT(false)
		result, err := pa.AssembleSystemPrompt(context.Background(), &PromptContext{
			ShardID:   "coder-rows",
			ShardType: "coder",
		})
		if err != nil {
			t.Fatalf("AssembleSystemPrompt: %v", err)
		}
		if !strings.Contains(result, "kc-jit-03") {
			t.Error("the compiler's kernel_context_rows of 4 dropped kc-jit-03")
		}
		if strings.Contains(result, "kc-jit-04") {
			t.Error("kc-jit-04 was shown past the compiler's kernel_context_rows of 4")
		}
	})
}

func TestSessionContextCharCeiling_FollowsBudgetAndShare(t *testing.T) {
	pa := &PromptAssembler{}
	defBudget := config.DefaultJITConfig().TokenBudget
	defShare := config.DefaultArticulationConfig().SessionContextSharePercent
	want := defBudget * defShare * config.BytesPerToken / 100
	if got := pa.sessionContextCharCeiling(); got != want {
		t.Fatalf("unset ceiling = %d, want %d", got, want)
	}

	pa.SetJITBudgets(4000, 1, 1, 1)
	pa.SetSessionContextSharePercent(10)
	// 4000 tokens * 10% * 4 bytes = 1600 characters.
	if got := pa.sessionContextCharCeiling(); got != 1600 {
		t.Fatalf("share ceiling = %d, want 1600", got)
	}
}
