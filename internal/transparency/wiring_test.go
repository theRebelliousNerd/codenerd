package transparency

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// --- Structured errors at subsystem boundaries -------------------------

func TestClassifyError_WhenBoundaryError_ShouldUseDeclaredCategory(t *testing.T) {
	t.Parallel()

	// "permission denied" from the OS is a filesystem fault, but the substring
	// heuristics see "permission"/"denied" first and report it as a
	// constitutional refusal — which sends the operator to /shadow and
	// /query permitted for a chmod problem. A declared category ends the guess.
	raw := fmt.Errorf("open /srv/data/report.txt: permission denied")
	if got := ClassifyError(raw).Category; got != ErrorCategorySafety {
		t.Fatalf("precondition changed: heuristic now returns %s for the ambiguous message", got)
	}

	typed := NewBoundaryError(ErrorCategoryFilesystem, "/read_file", "/srv/data/report.txt", raw)
	classified := ClassifyError(typed)
	if classified.Category != ErrorCategoryFilesystem {
		t.Fatalf("expected declared filesystem category, got %s", classified.Category)
	}
	if len(classified.Remediation) == 0 {
		t.Error("expected remediation steps for a declared category")
	}
	if !errors.Is(classified, raw) {
		t.Error("expected the original error to remain unwrappable")
	}
}

func TestNewSafetyError_ShouldPreserveMessageAndContext(t *testing.T) {
	t.Parallel()
	inner := errors.New("action /delete_file not permitted by kernel policy")
	err := NewSafetyError("/delete_file", "main.go", "permitted", inner)

	if err.Error() != inner.Error() {
		t.Fatalf("wrapping must not change the message: %q", err.Error())
	}
	if err.Op != "/delete_file" || err.Target != "main.go" || err.Rule != "permitted" {
		t.Fatalf("boundary context lost: %+v", err)
	}
	if ClassifyError(err).Category != ErrorCategorySafety {
		t.Error("expected safety classification")
	}

	var boundary *BoundaryError
	if !errors.As(fmt.Errorf("wrapped: %w", err), &boundary) {
		t.Error("expected BoundaryError to survive further wrapping")
	}
}

// --- Operation summaries ----------------------------------------------

func TestRecordOperation_WhenFlagEnabled_ShouldFormatWithFormatOperationSummary(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(&config.TransparencyConfig{
		Enabled:            true,
		OperationSummaries: true,
	})

	tm.RecordOperation(types.OperationRecord{
		Operation:     "coder shard",
		Outcome:       "Success",
		Duration:      2 * time.Second,
		Details:       "wrote 3 files",
		Source:        "coder-1",
		FilesAffected: []string{"a.go", "b.go"},
		NextSteps:     []string{"run tests"},
	})

	formatted := tm.FormatLastOperation()
	for _, want := range []string{"coder shard Complete", "2s", "Success", "a.go", "run tests"} {
		if !strings.Contains(formatted, want) {
			t.Errorf("expected %q in formatted summary:\n%s", want, formatted)
		}
	}
	if len(tm.RecentOperations(0)) != 1 {
		t.Errorf("expected 1 recorded operation, got %d", len(tm.RecentOperations(0)))
	}
}

func TestRecordOperation_WhenFlagDisabled_ShouldRecordNothing(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(&config.TransparencyConfig{
		Enabled:            true,
		OperationSummaries: false,
	})

	tm.RecordOperation(types.OperationRecord{Operation: "coder shard", Outcome: "Success"})

	if got := len(tm.RecentOperations(0)); got != 0 {
		t.Fatalf("expected no operations recorded, got %d", got)
	}
	if tm.FormatLastOperation() != "" {
		t.Error("expected empty formatting with no recorded operations")
	}
	if strings.Contains(tm.GetStatus(), "Recent Operations") {
		t.Error("status must not advertise a section the flag disabled")
	}
}

func TestRecordOperation_WhenRingOverflows_ShouldKeepMostRecent(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(&config.TransparencyConfig{Enabled: true, OperationSummaries: true})

	for i := 0; i < maxOperationHistory+5; i++ {
		tm.RecordOperation(types.OperationRecord{Operation: fmt.Sprintf("op-%d", i), Outcome: "Success"})
	}

	recent := tm.RecentOperations(0)
	if len(recent) != maxOperationHistory {
		t.Fatalf("expected the ring bounded at %d, got %d", maxOperationHistory, len(recent))
	}
	if recent[len(recent)-1].Operation != fmt.Sprintf("op-%d", maxOperationHistory+4) {
		t.Errorf("expected newest operation last, got %s", recent[len(recent)-1].Operation)
	}
}

// --- Status honesty ---------------------------------------------------

func TestGetStatus_WhenStreamReasoningSet_ShouldLabelItExperimental(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(&config.TransparencyConfig{
		Enabled:            true,
		ShardPhases:        true,
		StreamReasoning:    true,
		SafetyExplanations: true,
		JITExplain:         true,
		OperationSummaries: true,
		VerboseErrors:      true,
	})

	status := tm.GetStatus()
	line := ""
	for _, l := range strings.Split(status, "\n") {
		if strings.Contains(l, "Stream Reasoning") {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatal("expected a Stream Reasoning row in the status table")
	}
	if !strings.Contains(line, "experimental") {
		t.Errorf("an unwired flag must not read as a working feature: %q", line)
	}

	// The wired flags must NOT be labelled experimental.
	for _, feature := range []string{"Shard Phases", "JIT Explain", "Operation Summaries", "Safety Explanations"} {
		for _, l := range strings.Split(status, "\n") {
			if strings.Contains(l, feature) && strings.Contains(l, "experimental") {
				t.Errorf("%s is wired and must not be marked experimental: %q", feature, l)
			}
		}
	}
}

func TestTransparencyManager_NilReceiver_ShouldBeSafe(t *testing.T) {
	t.Parallel()
	var tm *TransparencyManager

	// Producers hold this as a possibly-unset interface value.
	if tm.IsEnabled() {
		t.Error("nil manager should report disabled")
	}
	tm.StartShard("s", "coder", "task")
	tm.UpdateShardPhase("s", PhaseExecuting, "msg")
	tm.EndShard("s", false)
	tm.RecordOperation(types.OperationRecord{Operation: "x"})
	if tm.ReportSafetyViolation("a", "b", "c") != nil {
		t.Error("nil manager should report no violation")
	}
	if len(tm.RecentOperations(5)) != 0 {
		t.Error("nil manager should have no operations")
	}
}

// --- Shard lifecycle feed ---------------------------------------------

func TestTransparencyManager_ShardLifecycle_ShouldPopulateActiveOperations(t *testing.T) {
	t.Parallel()
	tm := NewTransparencyManager(&config.TransparencyConfig{Enabled: true, ShardPhases: true})

	tm.StartShard("coder-1", "coder", "add a test")
	tm.UpdateShardPhase("coder-1", PhaseExecuting, "running tools")

	status := tm.GetStatus()
	if !strings.Contains(status, "Active Operations") || !strings.Contains(status, "[coder] Executing") {
		t.Fatalf("expected the running shard in Active Operations:\n%s", status)
	}

	tm.EndShard("coder-1", false)
	if len(tm.ShardObserver().GetActiveExecutions()) != 0 {
		t.Error("a finished shard must leave the active list")
	}
	if exec := tm.ShardObserver().GetExecution("coder-1"); exec == nil || exec.Phase != PhaseComplete {
		t.Errorf("expected terminal phase recorded, got %+v", exec)
	}
}
