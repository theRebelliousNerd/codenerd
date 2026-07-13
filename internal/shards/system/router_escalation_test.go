package system

import (
	"context"
	"fmt"
	"testing"
	"time"

	"codenerd/internal/core"
)

func TestRouterMissingRouteEmitsFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	router := NewTactileRouterShard()
	router.Kernel = kernel

	actionID := "action-missing-route"
	payload := map[string]any{"intent_id": "/current_intent"}
	if err := kernel.Assert(core.Fact{
		Predicate: "permitted_action",
		Args:      []any{actionID, "/nonexistent_action", "", payload, time.Now().Unix()},
	}); err != nil {
		t.Fatalf("assert permitted_action: %v", err)
	}

	if err := router.processPermittedActions(ctx); err != nil {
		t.Fatalf("processPermittedActions: %v", err)
	}

	results, err := kernel.Query("routing_result")
	if err != nil {
		t.Fatalf("Query(routing_result) error = %v", err)
	}
	found := false
	for _, f := range results {
		if len(f.Args) < 3 {
			continue
		}
		if fmt.Sprintf("%v", f.Args[0]) != actionID {
			continue
		}
		status := fmt.Sprintf("%v", f.Args[1])
		reason := fmt.Sprintf("%v", f.Args[2])
		if status != "/failure" || reason != "no_handler" {
			t.Fatalf("routing_result = (%s, %s), want (/failure, no_handler)", status, reason)
		}
		found = true
		break
	}
	if !found {
		t.Fatalf("routing_result not found for %s", actionID)
	}

	reasons, err := kernel.Query("no_action_reason")
	if err != nil {
		t.Fatalf("Query(no_action_reason) error = %v", err)
	}
	reasonFound := false
	for _, f := range reasons {
		if len(f.Args) < 2 {
			continue
		}
		intentID := fmt.Sprintf("%v", f.Args[0])
		reason := fmt.Sprintf("%v", f.Args[1])
		if intentID == "/current_intent" && reason == "/no_route" {
			reasonFound = true
			break
		}
	}
	if !reasonFound {
		t.Fatalf("no_action_reason not asserted for /current_intent /no_route")
	}

	remaining, err := kernel.Query("permitted_action")
	if err != nil {
		t.Fatalf("Query(permitted_action) error = %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("unmapped permitted action was not consumed: %v", remaining)
	}

	// A later polling/event pass must not amplify the terminal failure.
	if err := router.processPermittedActions(ctx); err != nil {
		t.Fatalf("second processPermittedActions: %v", err)
	}
	resultsAfterRetry, err := kernel.Query("routing_result")
	if err != nil {
		t.Fatalf("second Query(routing_result) error = %v", err)
	}
	if len(resultsAfterRetry) != len(results) {
		t.Fatalf("routing failures amplified from %d to %d", len(results), len(resultsAfterRetry))
	}
}

func TestRouterAllowedUnmappedActionIsRecordedAndConsumed(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}
	config := DefaultRouterConfig()
	config.AllowUnmappedActions = true
	router := NewTactileRouterShardWithConfig(config)
	router.Kernel = kernel

	fact := core.Fact{
		Predicate: "permitted_action",
		Args:      []any{"action-learn-route", "/novel_action", "target", map[string]any{}, time.Now().Unix()},
	}
	if err := kernel.Assert(fact); err != nil {
		t.Fatalf("assert permitted_action: %v", err)
	}
	if err := router.processPermittedActions(context.Background()); err != nil {
		t.Fatalf("processPermittedActions: %v", err)
	}

	remaining, err := kernel.Query("permitted_action")
	if err != nil {
		t.Fatalf("Query(permitted_action) error = %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("allowed-unmapped action was not consumed: %v", remaining)
	}
	results, err := kernel.Query("routing_result")
	if err != nil {
		t.Fatalf("Query(routing_result) error = %v", err)
	}
	if len(results) != 1 || fmt.Sprint(results[0].Args[1]) != "/failure" || fmt.Sprint(results[0].Args[2]) != "no_handler" {
		t.Fatalf("routing_result = %v, want one /failure no_handler", results)
	}
}
