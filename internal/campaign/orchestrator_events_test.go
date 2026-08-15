package campaign

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The event type set must stay closed.
//
// OrchestratorEvent.Type is a plain string switched on by name in three
// consumers. A literal written at an emit site and nowhere else produces an
// event every consumer drops through its default branch — the campaign still
// runs, the operator just never sees that step happen. This reads the emit
// sites and fails if any of them names a type the constant list does not.
func TestOrchestratorEventTypes_AreClosedSet(t *testing.T) {
	pkgDir := filepath.Join(repoRoot(t), "internal", "campaign")

	known := make(map[string]bool, len(orchestratorEventTypes))
	for _, v := range orchestratorEventTypes {
		known[v] = true
	}

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	var offenders []string
	emitSites := 0

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, filepath.Join(pkgDir, name), nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name != "emitEvent" && sel.Sel.Name != "emitRiskAudit" {
				return true
			}
			emitSites++

			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				// An identifier: it resolves to one of the constants below, or
				// the compiler would have rejected it.
				return true
			}
			value, uerr := strconv.Unquote(lit.Value)
			if uerr != nil {
				return true
			}
			if !known[value] {
				offenders = append(offenders, name+": "+value)
			}
			return true
		})
	}

	if emitSites < 20 {
		t.Fatalf("found only %d emit sites; the AST scan is broken, not the event set", emitSites)
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("event types emitted as bare literals outside the closed set:\n  %s\n"+
			"Add the constant to orchestrator_events.go and use it, so every UI that "+
			"switches on event type is forced to consider the new event.",
			strings.Join(offenders, "\n  "))
	}
}

func TestOrchestratorEventTypes_ListIsSortedAndUnique(t *testing.T) {
	// IsKnownOrchestratorEventType binary-searches the slice.
	for i := 1; i < len(orchestratorEventTypes); i++ {
		if orchestratorEventTypes[i-1] >= orchestratorEventTypes[i] {
			t.Fatalf("orchestratorEventTypes is not strictly sorted at %d: %q then %q; "+
				"IsKnownOrchestratorEventType would miss entries",
				i, orchestratorEventTypes[i-1], orchestratorEventTypes[i])
		}
	}
	for _, v := range orchestratorEventTypes {
		if !IsKnownOrchestratorEventType(v) {
			t.Errorf("IsKnownOrchestratorEventType(%q) = false for a listed type", v)
		}
	}
	if IsKnownOrchestratorEventType("definitely_not_an_event") {
		t.Error("IsKnownOrchestratorEventType accepted an unlisted type")
	}
}

func TestOrchestratorEventTypes_ReturnsCopy(t *testing.T) {
	first := OrchestratorEventTypes()
	if len(first) == 0 {
		t.Fatal("no event types returned")
	}
	first[0] = "mutated"
	if OrchestratorEventTypes()[0] == "mutated" {
		t.Fatal("OrchestratorEventTypes exposed its backing array; a caller could corrupt the closed set")
	}
}

// Metrics hooks must observe real work without the engine depending on any
// particular backend.
func TestMetricsSink_ShouldObserveTaskAndCheckpointDurations(t *testing.T) {
	orch, _ := newCheckpointRegressionOrchestrator(t, "PASS: verified")
	metrics := NewInMemoryMetrics()
	orch.SetMetricsSink(metrics)

	orch.markPhaseStart(orch.campaign.Phases[0].ID)
	orch.observeTaskDuration(orch.campaign.Phases[0].ID, &orch.campaign.Phases[0].Tasks[0], "completed", 250*time.Millisecond)

	if err := orch.runPhase(t.Context(), &orch.campaign.Phases[0]); err != nil {
		t.Fatalf("runPhase: %v", err)
	}

	snapshot := metrics.Snapshot()
	tasks, _ := snapshot["tasks"].(map[string]any)
	if len(tasks) == 0 {
		t.Fatal("no task durations observed")
	}
	entry, ok := tasks[string(TaskTypeFileCreate)+"|completed"].(map[string]any)
	if !ok {
		t.Fatalf("expected an entry for file_create|completed, got %v", tasks)
	}
	if entry["count"].(int) != 1 || entry["max_ms"].(int64) != 250 {
		t.Errorf("task observation lost detail: %v", entry)
	}

	checkpoints, _ := snapshot["checkpoints"].(map[string]any)
	if len(checkpoints) == 0 {
		t.Fatal("no checkpoint outcomes observed; a passing verification recorded nothing")
	}
	if _, ok := checkpoints[string(VerifyShardValidate)]; !ok {
		t.Errorf("expected an entry for %s, got %v", VerifyShardValidate, checkpoints)
	}

	phases, _ := snapshot["phases_ms"].(map[string]int64)
	if _, ok := phases[orch.campaign.Phases[0].ID]; !ok {
		t.Errorf("phase duration was not observed on completion: %v", phases)
	}
}

// A nil sink must cost nothing and never panic — that is the whole reason the
// hooks are optional rather than a hard dependency.
func TestMetricsSink_WhenUnset_ShouldBeInert(t *testing.T) {
	orch, _ := newCheckpointRegressionOrchestrator(t, "PASS: verified")

	orch.markPhaseStart("/phase_ckpt_0")
	orch.observePhaseDuration("/phase_ckpt_0")
	orch.observeCheckpoint("/phase_ckpt_0", string(VerifyNone), true, time.Second)
	orch.observeTaskDuration("/phase_ckpt_0", &orch.campaign.Phases[0].Tasks[0], "completed", time.Second)
	orch.observeRiskPreflight("/campaign_ckpt", &RiskGateEvaluation{Allowed: true})

	if _, loaded := orch.phaseStarts.Load("/phase_ckpt_0"); loaded {
		t.Error("phase start times were recorded with no sink attached")
	}
}
