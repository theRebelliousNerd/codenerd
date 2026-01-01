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
	payload := map[string]interface{}{"intent_id": "/current_intent"}
	if err := kernel.Assert(core.Fact{
		Predicate: "permitted_action",
		Args:      []interface{}{actionID, "/nonexistent_action", "", payload, time.Now().Unix()},
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
}
