package store

import "testing"

func TestToolStoreStore_PreservesReferenceMetadataOnUpsert(t *testing.T) {
	ts, err := NewToolStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create tool store: %v", err)
	}
	defer ts.Close()

	initial := ToolExecution{
		CallID:           "call-1",
		SessionID:        "session-1",
		ToolName:         "tool-a",
		Action:           "initial",
		Input:            `{"step":1}`,
		Result:           "first result",
		Success:          true,
		DurationMs:       10,
		ResultSize:       len("first result"),
		SessionRuntimeMs: 100,
		UsefulnessScore:  0.4,
	}
	if err := ts.Store(initial); err != nil {
		t.Fatalf("initial Store failed: %v", err)
	}

	if err := ts.IncrementReference(initial.CallID); err != nil {
		t.Fatalf("IncrementReference failed: %v", err)
	}

	updated := initial
	updated.Action = "updated"
	updated.Result = "updated result"
	updated.ResultSize = len(updated.Result)
	updated.UsefulnessScore = 0.9
	if err := ts.Store(updated); err != nil {
		t.Fatalf("update Store failed: %v", err)
	}

	got, err := ts.GetByCallID(initial.CallID)
	if err != nil {
		t.Fatalf("GetByCallID failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected stored execution")
	}
	if got.ReferenceCount != 1 {
		t.Fatalf("expected reference count to be preserved, got %d", got.ReferenceCount)
	}
	if got.LastReferenced == nil {
		t.Fatal("expected last referenced timestamp to be preserved")
	}
	if got.Result != updated.Result {
		t.Fatalf("expected result to be updated, got %q", got.Result)
	}
	if got.Action != updated.Action {
		t.Fatalf("expected action to be updated, got %q", got.Action)
	}
	if got.UsefulnessScore != updated.UsefulnessScore {
		t.Fatalf("expected usefulness score to be updated, got %f", got.UsefulnessScore)
	}
}
