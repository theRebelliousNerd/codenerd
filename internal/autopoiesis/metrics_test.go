package autopoiesis

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExportMetrics_WhenRunsRecorded_ShouldDeriveRatesAndLatency(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)
	mock := replaceOuroborosWithMock(orch)

	mock.GetStatsFunc = func() OuroborosStats {
		return OuroborosStats{
			ToolsGenerated:      3,
			ToolsCompiled:       3,
			ToolsRejected:       1,
			SafetyViolations:    1,
			ExecutionCount:      12,
			Panics:              1,
			ThunderdomeRuns:     4,
			ThunderdomeKills:    1,
			ThunderdomeSurvived: 3,
			GenerationRuns:      8,
			TotalGenerationTime: 8 * time.Second,
			LongestGeneration:   5 * time.Second,
		}
	}
	mock.ListRuntimeToolsFunc = func() []*RuntimeTool {
		return []*RuntimeTool{runtimeToolFixture("a"), runtimeToolFixture("b"), runtimeToolFixture("c")}
	}

	m := orch.ExportMetrics()

	if m.MeanGenerationLatency != time.Second {
		t.Errorf("mean latency = %v, want 1s", m.MeanGenerationLatency)
	}
	if m.MaxGenerationLatency != 5*time.Second {
		t.Errorf("max latency = %v, want 5s", m.MaxGenerationLatency)
	}
	// Rejections are measured against verdicts (3 generated + 1 rejected), not
	// against loop runs, because retries inflate the run count.
	if got, want := m.RejectRate, 0.25; got != want {
		t.Errorf("reject rate = %v, want %v", got, want)
	}
	if got, want := m.SafetyViolationRate, 0.25; got != want {
		t.Errorf("safety violation rate = %v, want %v", got, want)
	}
	if got, want := m.ThunderdomeKillRate, 0.25; got != want {
		t.Errorf("thunderdome kill rate = %v, want %v", got, want)
	}
	if got, want := m.ThunderdomeEntryRate, 0.5; got != want {
		t.Errorf("thunderdome entry rate = %v, want %v", got, want)
	}
	if got, want := m.PanicRate, 0.125; got != want {
		t.Errorf("panic rate = %v, want %v", got, want)
	}
	if m.RegisteredTools != 3 {
		t.Errorf("registered tools = %d, want 3", m.RegisteredTools)
	}

	line := m.String()
	for _, key := range []string{"reject_rate", "mean_generation_ms", "thunderdome_kill_rate"} {
		if !strings.Contains(line, key) {
			t.Errorf("metrics line missing %q: %s", key, line)
		}
	}
}

func TestExportMetrics_WhenNothingHappened_ShouldNotDivideByZero(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)
	replaceOuroborosWithMock(orch)

	m := orch.ExportMetrics()
	if m.RejectRate != 0 || m.PanicRate != 0 || m.ThunderdomeKillRate != 0 || m.MeanGenerationLatency != 0 {
		t.Errorf("zero-state metrics are non-zero: %s", m.String())
	}

	var nilOrch *Orchestrator
	if got := nilOrch.ExportMetrics(); got.GenerationRuns != 0 {
		t.Error("ExportMetrics on a nil Orchestrator should be inert")
	}
}

// Latency has to be recorded for rejected runs too, or the metric improves as
// generation gets worse.
func TestOuroborosStats_WhenRunIsRejected_ShouldStillRecordLatency(t *testing.T) {
	llm := &scriptedLLM{
		unsafeCode:  multistageUnsafeTool,
		safeCode:    multistageUnsafeTool,
		attacksJSON: multistageAttacks,
	}
	cfg := DefaultOuroborosConfig(t.TempDir())
	cfg.EnableThunderdome = false
	cfg.WorkspaceRoot = ""

	loop := NewOuroborosLoop(llm, cfg)
	execCfg := DefaultExecuteConfig()
	execCfg.Retry.RetryDelay = time.Millisecond

	result := loop.ExecuteWithConfig(context.Background(), &ToolNeed{
		Name:       "rejected_tool",
		Purpose:    "will never pass safety",
		InputType:  "string",
		OutputType: "string",
		Confidence: 0.9,
	}, execCfg)

	if result.Success {
		t.Fatal("fixture was supposed to be rejected")
	}
	stats := loop.GetStats()
	if stats.GenerationRuns != 1 {
		t.Errorf("GenerationRuns = %d, want 1", stats.GenerationRuns)
	}
	if stats.TotalGenerationTime <= 0 {
		t.Error("rejected run contributed no latency; the metric would improve as generation degrades")
	}
	if result.Duration <= 0 {
		t.Error("LoopResult.Duration is unset for a failed run")
	}
}
