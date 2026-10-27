package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/transparency"
	"codenerd/internal/types"
)

func TestRouteActionResult_DispatchedHandlerErrorHasCorrelatedFailureObservations(t *testing.T) {
	workDir := t.TempDir()
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}

	manager := transparency.NewTransparencyManager(&config.TransparencyConfig{
		Enabled:            true,
		SafetyExplanations: true,
		OperationSummaries: true,
	})
	previousManager := transparency.SetProcessManager(manager)
	t.Cleanup(func() { transparency.SetProcessManager(previousManager) })

	glassBox := transparency.NewGlassBoxEventBus()
	glassBox.Enable()
	glassBoxEvents := glassBox.Subscribe()
	t.Cleanup(func() {
		glassBox.Unsubscribe(glassBoxEvents)
	})

	toolEvents := transparency.NewToolEventBus()
	t.Cleanup(toolEvents.Close)

	cfg := DefaultVirtualStoreConfig()
	cfg.WorkingDir = workDir
	vs := NewVirtualStoreWithConfig(nil, cfg)
	vs.SetKernel(kernel)
	vs.SetGlassBoxBus(glassBox)
	vs.SetToolEventBus(toolEvents)
	vs.DisableBootGuard()

	const actionID = "write-error-observation"
	const target = "must-not-exist.txt"
	payload := map[string]any{}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	pending := Fact{
		Predicate: "pending_action",
		Args:      []any{actionID, MangleAtom("/write_file"), target, string(payloadJSON), time.Now().Unix()},
	}
	if err := kernel.Assert(pending); err != nil {
		t.Fatalf("assert pending action: %v", err)
	}
	if !vs.CheckKernelPermitted("write_file", target, payload) {
		t.Fatal("real kernel did not permit the exact pending write request")
	}

	result, routeErr := vs.RouteActionResult(context.Background(), Fact{
		Predicate: "next_action",
		Args:      []any{actionID, MangleAtom("/write_file"), target, payload},
	})
	if routeErr == nil || !strings.Contains(routeErr.Error(), "write_file requires 'content'") {
		t.Fatalf("RouteActionResult error = %v, want original dispatched handler error", routeErr)
	}
	if result.Success {
		t.Fatal("dispatched handler error returned a successful ActionResult")
	}
	if result.Error != routeErr.Error() {
		t.Fatalf("ActionResult.Error = %q, want original error %q", result.Error, routeErr)
	}
	if _, err := os.Stat(filepath.Join(workDir, target)); !os.IsNotExist(err) {
		t.Fatalf("malformed write unexpectedly created the target: stat error = %v", err)
	}

	results, err := kernel.Query("execution_result")
	if err != nil {
		t.Fatalf("query execution_result: %v", err)
	}
	var correlated []Fact
	for _, fact := range results {
		if len(fact.Args) == 6 && types.ExtractString(fact.Args[0]) == actionID {
			correlated = append(correlated, fact)
		}
	}
	if len(correlated) != 1 {
		t.Fatalf("correlated execution_result count = %d, want exactly one; facts: %+v", len(correlated), correlated)
	}
	if success, ok := types.ExtractBool(correlated[0].Args[3]); !ok || success {
		t.Fatalf("execution_result success = %#v, want false", correlated[0].Args[3])
	}
	if got := types.ExtractString(correlated[0].Args[1]); got != "write_file" {
		t.Fatalf("execution_result action = %q, want write_file", got)
	}
	if got := types.ExtractString(correlated[0].Args[2]); got != target {
		t.Fatalf("execution_result target = %q, want %q", got, target)
	}
	if timestamp, ok := types.ExtractInt64(correlated[0].Args[5]); !ok || timestamp <= 0 {
		t.Fatalf("execution_result completion timestamp = %#v, want positive number", correlated[0].Args[5])
	}

	errorFacts, err := kernel.Query("execution_error")
	if err != nil {
		t.Fatalf("query execution_error: %v", err)
	}
	foundError := false
	for _, fact := range errorFacts {
		if len(fact.Args) == 2 && types.ExtractString(fact.Args[0]) == actionID && types.ExtractString(fact.Args[1]) == routeErr.Error() {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Fatalf("execution_error did not preserve dispatched handler error %q: %+v", routeErr, errorFacts)
	}

	select {
	case event := <-toolEvents.Subscribe():
		if event.ToolName != "write_file" || event.Success || event.Result != routeErr.Error() || event.Duration <= 0 {
			t.Fatalf("ToolEvent lost failed action outcome: %+v", event)
		}
	default:
		t.Fatal("dispatched handler error did not emit a ToolEvent")
	}

	var routingEvents []transparency.GlassBoxEvent
	for {
		select {
		case event := <-glassBoxEvents:
			if event.Category == transparency.CategoryRouting {
				routingEvents = append(routingEvents, event)
			}
		default:
			goto eventsDrained
		}
	}
eventsDrained:
	if len(routingEvents) != 1 || !strings.Contains(routingEvents[0].Summary, "write_file") || !strings.HasSuffix(routingEvents[0].Summary, "❌") || routingEvents[0].Duration <= 0 {
		t.Fatalf("routing failure event = %+v, want one failed write_file event", routingEvents)
	}

	operations := manager.RecentOperations(0)
	if len(operations) != 1 {
		t.Fatalf("operation summaries = %d, want one failed dispatched action: %+v", len(operations), operations)
	}
	if operations[0].Operation != "write_file" || operations[0].Outcome != "Failed" || operations[0].Details != routeErr.Error() || operations[0].Source != actionID || operations[0].Duration == "" {
		t.Fatalf("operation summary lost failure identity or duration context: %+v", operations[0])
	}

	if err := kernel.RetractFact(pending); err != nil {
		t.Fatalf("retract pending action: %v", err)
	}
	const deniedID = "unpermitted-action"
	_, deniedErr := vs.RouteActionResult(context.Background(), Fact{
		Predicate: "next_action",
		Args:      []any{deniedID, MangleAtom("/read_file"), "unpermitted.txt"},
	})
	if deniedErr == nil || !strings.Contains(deniedErr.Error(), "not permitted") {
		t.Fatalf("unpermitted action error = %v, want default-deny refusal", deniedErr)
	}
	results, err = kernel.Query("execution_result")
	if err != nil {
		t.Fatalf("query execution_result after denial: %v", err)
	}
	for _, fact := range results {
		if len(fact.Args) > 0 && types.ExtractString(fact.Args[0]) == deniedID {
			t.Fatalf("pre-dispatch denial fabricated execution_result: %+v", fact)
		}
	}
	if stats := toolEvents.Stats(); stats.Emitted != 1 {
		t.Fatalf("pre-dispatch denial changed ToolEvent count: %+v", stats)
	}
}
