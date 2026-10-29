package context

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/store"
)

// newKernelBackedCompressor builds a compressor over a real RealKernel, which
// loads the embedded defaults/policy/*.mg — including context_compilation.mg.
// These tests exercise the kernel rules for real rather than stubbing them.
func newKernelBackedCompressor(t *testing.T) *Compressor {
	t.Helper()
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	localStore, err := store.NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	comp := NewCompressor(kernel, localStore, &MockLLMClient{})
	comp.config = DefaultTestContextConfig()
	comp.budget = NewTokenBudget(comp.config)
	comp.activation = NewActivationEngine(comp.config)
	return comp
}

func fact(pred string, args ...any) core.Fact {
	return core.Fact{Predicate: pred, Args: args}
}

// =============================================================================
// C3 — should_mask_observation
// =============================================================================

func TestAssertTurnAgeCategories_WhenCompressing_ShouldReachKernel(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	comp.turnNumber = 40

	comp.assertTurnAgeCategories([]CompressedTurn{
		{TurnNumber: 1},  // age 39 -> /ancient
		{TurnNumber: 38}, // age 2  -> /recent
	})

	cats, err := comp.kernel.Query("turn_age_category")
	if err != nil {
		t.Fatalf("query turn_age_category: %v", err)
	}
	if len(cats) != 2 {
		// The trailing "." this used to append made every assertion fail to
		// parse, and the error was discarded, so masking never fired.
		t.Fatalf("expected 2 turn_age_category facts in kernel, got %d: %v", len(cats), cats)
	}
}

func TestMaskedObservationTurns_WhenKernelMarksOldTurns_ShouldReturnThoseTurns(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	comp.turnNumber = 40
	comp.assertTurnAgeCategories([]CompressedTurn{
		{TurnNumber: 1},  // /ancient -> masked
		{TurnNumber: 30}, // age 10 -> /old -> masked
		{TurnNumber: 39}, // age 1 -> /recent -> not masked
	})

	masked := comp.maskedObservationTurns()
	if !masked[turnMaskID(1)] || !masked[turnMaskID(30)] {
		t.Errorf("expected ancient/old turns masked, got %v", masked)
	}
	if masked[turnMaskID(39)] {
		t.Errorf("recent turn must not be masked, got %v", masked)
	}
}

func TestCompress_WhenKernelMasksObservations_ShouldDropResultsAndKeepReasoning(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	comp.turnNumber = 60 // every synthesized turn is /ancient

	for i := 1; i <= 5; i++ {
		intent := fact("user_intent", "i", "/cat", "/fix", "auth.go", "none")
		comp.recentTurns = append(comp.recentTurns, CompressedTurn{
			TurnNumber:     i,
			Role:           "user",
			Timestamp:      time.Now(),
			OriginalTokens: 400,
			IntentAtom:     &intent,
			FocusAtoms:     []core.Fact{fact("focus_resolution", "f", "auth.go", "sym", "0.9")},
			ActionAtoms:    []core.Fact{fact("action_taken", "edit_file", "auth.go")},
			ResultAtoms:    []core.Fact{fact("diagnostic", "auth.go", "OBSERVATION_SURFACE_TEXT")},
		})
	}

	if err := comp.compress(context.Background()); err != nil {
		t.Fatalf("compress: %v", err)
	}
	if len(comp.rollingSummary.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(comp.rollingSummary.Segments))
	}

	seg := comp.rollingSummary.Segments[0]
	// window is 2, so turns 1..3 compress.
	if seg.MaskedTurns != 3 {
		t.Errorf("expected 3 masked turns, got %d", seg.MaskedTurns)
	}
	if comp.rollingSummary.TotalMaskedTurns != 3 {
		t.Errorf("rolling TotalMaskedTurns = %d, want 3", comp.rollingSummary.TotalMaskedTurns)
	}
	if strings.Contains(seg.Summary, "OBSERVATION_SURFACE_TEXT") {
		t.Errorf("masked turns must not carry observation atoms:\n%s", seg.Summary)
	}
	for _, want := range []string{"user_intent", "focus_resolution", "action_taken"} {
		if !strings.Contains(seg.Summary, want) {
			t.Errorf("reasoning atom %q dropped from masked summary:\n%s", want, seg.Summary)
		}
	}
}

func TestGenerateObservationMaskedSummary_WhenNothingMasked_ShouldMatchSimpleSummary(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	intent := fact("user_intent", "i", "/cat", "/fix", "auth.go", "none")
	turns := []CompressedTurn{{
		TurnNumber:  7,
		IntentAtom:  &intent,
		ResultAtoms: []core.Fact{fact("diagnostic", "auth.go", "err")},
	}}

	got, masked := comp.generateObservationMaskedSummary(turns, nil)
	if masked != 0 {
		t.Errorf("masked = %d, want 0", masked)
	}
	if want := comp.generateSimpleSummary(turns); got != want {
		t.Errorf("empty mask set must degrade to the simple summary:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestMaskedObservationTurns_WhenNoKernel_ShouldReturnNil(t *testing.T) {
	comp := &Compressor{}
	if got := comp.maskedObservationTurns(); got != nil {
		t.Errorf("expected nil mask set without a kernel, got %v", got)
	}
}

// =============================================================================
// C1/C4 — should_include_context with the real context_compilation.mg loaded
// =============================================================================

func TestBuildContext_WhenKernelDerivesInclusion_ShouldUseKernelSelection(t *testing.T) {
	comp := newKernelBackedCompressor(t)

	// These EDB facts drive context_compilation.mg's C1 rules:
	//   user_intent -> context_relevant(Target, /p100)
	//   modified    -> context_relevant(File, /p85)
	if err := comp.kernel.AssertString(`user_intent("i1", "/code", "/fix", "auth.go", "none")`); err != nil {
		t.Fatalf("assert user_intent: %v", err)
	}
	if err := comp.kernel.AssertString(`modified("auth.go")`); err != nil {
		t.Fatalf("assert modified: %v", err)
	}

	inc, err := comp.kernel.Query("should_include_context")
	if err != nil || len(inc) == 0 {
		t.Fatalf("context_compilation.mg derived no should_include_context (err=%v, rows=%d)", err, len(inc))
	}

	ctxData, err := comp.BuildContext(context.Background())
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	stats := comp.GetSelectionStats()
	if stats.LastMode != SelectionKernel {
		t.Fatalf("expected kernel selection, got mode=%s reason=%s", stats.LastMode, stats.LastReason)
	}
	if stats.LastSelectedFacts == 0 {
		t.Fatal("kernel selection produced an empty context block")
	}
	if !strings.Contains(ctxData.ContextAtoms, "auth.go") {
		t.Errorf("kernel-selected entity missing from ACTIVE CONTEXT:\n%s", ctxData.ContextAtoms)
	}
}

func TestBuildContext_WhenKernelDerivesNothing_ShouldFallBackToGoActivation(t *testing.T) {
	comp := newKernelBackedCompressor(t)

	if _, err := comp.BuildContext(context.Background()); err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	stats := comp.GetSelectionStats()
	if stats.LastMode != SelectionGoFallback {
		t.Fatalf("expected Go fallback with no kernel facts, got %s", stats.LastMode)
	}
	if stats.LastReason != reasonNoKernelFacts {
		t.Errorf("reason = %q, want %q", stats.LastReason, reasonNoKernelFacts)
	}
}

func TestGetSelectionStats_WhenBothPathsRun_ShouldTrackInclusionRate(t *testing.T) {
	comp := newKernelBackedCompressor(t)

	// Build 1: no kernel opinion -> Go fallback.
	if _, err := comp.BuildContext(context.Background()); err != nil {
		t.Fatalf("BuildContext 1: %v", err)
	}
	// Build 2: kernel has an opinion -> kernel selection.
	if err := comp.kernel.AssertString(`modified("auth.go")`); err != nil {
		t.Fatalf("assert modified: %v", err)
	}
	if _, err := comp.BuildContext(context.Background()); err != nil {
		t.Fatalf("BuildContext 2: %v", err)
	}

	stats := comp.GetSelectionStats()
	if stats.KernelSelections != 1 || stats.GoFallbacks != 1 {
		t.Fatalf("kernel=%d fallback=%d, want 1/1 (%s)", stats.KernelSelections, stats.GoFallbacks, stats.LastReason)
	}
	if rate := stats.KernelInclusionRate(); rate != 0.5 {
		t.Errorf("KernelInclusionRate = %v, want 0.5", rate)
	}

	metrics := comp.GetMetrics()
	if metrics["kernel_selections"].(int) != 1 || metrics["go_fallbacks"].(int) != 1 {
		t.Errorf("GetMetrics must expose the dual-path split, got %v", metrics)
	}
}

func TestBuildKernelDerivedContext_WhenEntityNamesNoFact_ShouldReturnNil(t *testing.T) {
	comp := newKernelBackedCompressor(t)

	kernelFacts := []core.Fact{fact("should_include_context", "ghost_entity_that_exists_nowhere", "/p100")}
	if got := comp.buildKernelDerivedContext(kernelFacts, []core.Fact{fact("modified", "auth.go")}); got != nil {
		t.Errorf("unresolvable entity must yield nil so BuildContext falls back, got %v", got)
	}
}

func TestBuildKernelDerivedContext_WhenEntityIsFactArgument_ShouldResolveToFacts(t *testing.T) {
	comp := newKernelBackedCompressor(t)

	all := []core.Fact{
		fact("modified", "auth.go"),
		fact("file_topology", "auth.go", "go"),
		fact("modified", "unrelated.go"),
	}
	kernelFacts := []core.Fact{fact("should_include_context", "auth.go", "/p100")}

	got := comp.buildKernelDerivedContext(kernelFacts, all)
	if len(got) != 2 {
		t.Fatalf("expected both auth.go facts resolved, got %d: %v", len(got), got)
	}
	for _, sf := range got {
		if sf.Score != 100 {
			t.Errorf("kernel priority /p100 must map to 100, got %v", sf.Score)
		}
		if strings.Contains(sf.Fact.String(), "unrelated.go") {
			t.Errorf("resolution leaked an unrelated fact: %s", sf.Fact.String())
		}
	}
}

// =============================================================================
// LoadState / RefreshBudget pairing
// =============================================================================

func TestLoadState_WhenCalledWithoutRefreshBudget_ShouldStillUpdateBudget(t *testing.T) {
	src := newKernelBackedCompressor(t)
	big := strings.Repeat("word ", 400)
	if _, err := src.ProcessTurn(context.Background(), Turn{Number: 1, Role: "user", UserInput: big, Timestamp: time.Now()}); err != nil {
		t.Fatalf("ProcessTurn: %v", err)
	}
	state := src.GetState()

	dst := newKernelBackedCompressor(t)
	if used, _ := dst.GetBudgetUsage(); used != 0 {
		t.Fatalf("fresh compressor should report 0 used, got %d", used)
	}

	// Deliberately NOT calling RefreshBudget: rehydration must not depend on
	// the caller remembering to pair the two calls.
	if err := dst.LoadState(state); err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	used, _ := dst.GetBudgetUsage()
	if used == 0 {
		t.Error("LoadState must leave the budget describing the restored state")
	}
}

func TestRefreshBudget_WhenCalledAfterLoadState_ShouldBeIdempotent(t *testing.T) {
	src := newKernelBackedCompressor(t)
	if _, err := src.ProcessTurn(context.Background(), Turn{Number: 1, Role: "user", UserInput: strings.Repeat("w ", 200)}); err != nil {
		t.Fatalf("ProcessTurn: %v", err)
	}

	dst := newKernelBackedCompressor(t)
	if err := dst.LoadState(src.GetState()); err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	before, _ := dst.GetBudgetUsage()
	dst.RefreshBudget()
	after, _ := dst.GetBudgetUsage()

	if before != after {
		t.Errorf("RefreshBudget after LoadState changed usage %d -> %d; the pair must be idempotent", before, after)
	}
}
