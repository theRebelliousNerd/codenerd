package autopoiesis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"codenerd/internal/types"
)

func TestGeneratedFeedbackDurableReopenAndDuplicate(t *testing.T) {
	dir := t.TempDir()
	store := NewLearningStore(dir)
	feedback := &ExecutionFeedback{ToolName: "probe", ExecutionID: "scope/exec-call", RequestFingerprint: "fingerprint", Input: `{"path":"effect.txt"}`, Output: "literal", Success: true}
	ack, err := store.RecordLearningDurable(context.Background(), "probe", feedback, nil)
	if err != nil || !ack.Durable {
		t.Fatalf("ack=%+v err=%v", ack, err)
	}
	duplicate, err := store.RecordLearningDurable(context.Background(), "probe", feedback, nil)
	if err != nil || !duplicate.Replayed || store.GetLearning("probe").TotalExecutions != 1 {
		t.Fatalf("duplicate=%+v err=%v", duplicate, err)
	}
	reopened := NewLearningStore(dir)
	recalled, ok := reopened.ExecutionFeedback(feedback.ExecutionID)
	if !ok || recalled.Output != "literal" || recalled.Input != feedback.Input || reopened.GetLearning("probe").TotalExecutions != 1 {
		t.Fatalf("recalled=%+v ok=%v", recalled, ok)
	}
	feedback.RequestFingerprint = "changed"
	if _, err := reopened.RecordLearningDurable(context.Background(), "probe", feedback, nil); err == nil {
		t.Fatal("conflicting durable identity admitted")
	}
}

func TestGeneratedFeedbackPublicationFailureAndRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "learnings")
	store := NewLearningStore(path)
	if err := os.WriteFile(path, []byte("publication blocker"), 0600); err != nil {
		t.Fatal(err)
	}
	feedback := &ExecutionFeedback{ToolName: "probe", ExecutionID: "call", RequestFingerprint: "fp", Success: false, Output: "partial"}
	ack, err := store.RecordLearningDurable(context.Background(), "probe", feedback, nil)
	if err == nil || ack.Durable || store.GetLearning("probe") != nil {
		t.Fatalf("silent save or leaked mutation: ack=%+v err=%v", ack, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	ack, err = store.RecordLearningDurable(context.Background(), "probe", feedback, nil)
	if err != nil || !ack.Durable || NewLearningStore(path).GetLearning("probe").TotalExecutions != 1 {
		t.Fatalf("retry ack=%+v err=%v", ack, err)
	}
}

func TestGeneratedFeedbackConcurrentPublicationRetainsEveryRecord(t *testing.T) {
	path := t.TempDir()
	store := NewLearningStore(path)
	start := make(chan struct{})
	failures := make(chan error, 16)
	var workers sync.WaitGroup
	for index := 0; index < 16; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			<-start
			id := fmt.Sprintf("exec-%d", index)
			_, err := store.RecordLearningDurable(context.Background(), "probe", &ExecutionFeedback{ToolName: "probe", ExecutionID: id, RequestFingerprint: id, Success: true}, nil)
			failures <- err
		}(index)
	}
	close(start)
	workers.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	reopened := NewLearningStore(path)
	if learning := reopened.GetLearning("probe"); learning == nil || learning.TotalExecutions != 16 {
		t.Fatalf("lost concurrent learning: %+v", learning)
	}
	for index := 0; index < 16; index++ {
		if _, ok := reopened.ExecutionAcknowledgment(fmt.Sprintf("exec-%d", index)); !ok {
			t.Fatalf("lost record %d", index)
		}
	}
}

func TestGeneratedFeedbackRecorderFailureIsNotAcknowledged(t *testing.T) {
	store := NewLearningStore(t.TempDir())
	orchestrator := &Orchestrator{learnings: store}
	receipt := types.GeneratedToolReceipt{Request: types.GeneratedToolRequest{ScopeID: "scope", CallID: "call", AuthorizationID: "exec-call", Tool: types.GeneratedToolIdentity{Name: "probe"}, CanonicalArgs: "{}"},
		ProcessStarted: true, StartedAt: time.Now(), ValidationCompleted: true, ValidationPassed: true}
	ack, err := orchestrator.RecordGeneratedExecution(context.Background(), receipt)
	if err == nil || ack.Durable || store.GetLearning("probe") != nil {
		t.Fatalf("missing evaluator was acknowledged: %+v %v", ack, err)
	}
}

func TestGeneratedFeedbackCorruptionRequiresRecovery(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tool_learnings.json"), []byte(`{"version":2,"feedback":`), 0600); err != nil {
		t.Fatal(err)
	}
	store := NewLearningStore(dir)
	if store.DurabilityError() == nil {
		t.Fatal("corrupt history became an acknowledged empty store")
	}
	ack, err := store.RecordLearningDurable(context.Background(), "probe", &ExecutionFeedback{ToolName: "probe", ExecutionID: "call", RequestFingerprint: "fingerprint"}, nil)
	if err == nil || ack.Durable {
		t.Fatalf("corrupt-store acknowledgement=%+v err=%v", ack, err)
	}
}
