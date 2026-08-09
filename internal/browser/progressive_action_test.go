package browser

import (
	"context"
	"strings"
	"testing"
)

func TestExecuteActions_StopOnErrorAndBoundOperations(t *testing.T) {
	manager := NewSessionManager(DefaultConfig(), nil)
	execution, err := manager.ExecuteActions(context.Background(), "", []ActionOperation{
		{Type: "not-a-real-operation"},
		{Type: "sleep", DurationMS: 0},
	}, true)
	if err != nil {
		t.Fatalf("ExecuteActions returned transport error: %v", err)
	}
	if execution.Success || len(execution.Results) != 1 || execution.Counts["failed"] != 1 {
		t.Fatalf("stop-on-error contract failed: %+v", execution)
	}

	tooMany := make([]ActionOperation, maxActionOperations+1)
	for i := range tooMany {
		tooMany[i] = ActionOperation{Type: "sleep"}
	}
	if _, err := manager.ExecuteActions(context.Background(), "", tooMany, true); err == nil || !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("expected operation bound error, got %v", err)
	}
}

func TestNormalizeObserveOptions_DefaultsAndBounds(t *testing.T) {
	mode, view, maxItems, filter, err := normalizeObserveOptions(ObserveOptions{MaxItems: maxObservationItems + 50})
	if err != nil {
		t.Fatalf("normalizeObserveOptions failed: %v", err)
	}
	if mode != "composite" || view != "compact" || maxItems != maxObservationItems || filter != "all" {
		t.Fatalf("unexpected normalized options: mode=%q view=%q max=%d filter=%q", mode, view, maxItems, filter)
	}
	if _, _, _, _, err := normalizeObserveOptions(ObserveOptions{Mode: "javascript"}); err == nil {
		t.Fatal("arbitrary observation mode should fail closed")
	}
}
