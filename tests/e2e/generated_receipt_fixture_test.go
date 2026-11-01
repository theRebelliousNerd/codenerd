//go:build integration

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/evidence"
	"codenerd/internal/types"
)

// The interface compiles against unchanged main, so the same control observes
// a lost caller-visible receipt rather than a baseline compilation failure.
type generatedReceiptResult interface {
	GeneratedToolReceipts() []types.GeneratedToolReceipt
}

func TestGeneratedPipelineReceiptAndAcceptanceBoundary(t *testing.T) {
	for _, mode := range []string{"effect", "missing_effect", "partial_failure", "canceled_after_start", "not_approved"} {
		t.Run(mode, func(t *testing.T) {
			f := newPipelineFixture(t, false)
			fault := ""
			if mode == "missing_effect" {
				fault = "disconnect-effect"
			} else if mode == "partial_failure" {
				fault = "partial-fail"
			}
			gateURL := ""
			var gate *pipelineGate
			if mode == "canceled_after_start" {
				gate = newPipelineGate(t)
				gateURL = gate.server.URL
			}
			plan := f.plan("receipt-boundary", "actual receipt-boundary effect\n", fault, gateURL)
			if mode != "not_approved" {
				f.approve(plan)
			}
			before, err := evidence.Snapshot(context.Background(), f.root)
			if err != nil {
				t.Fatal(err)
			}
			var outcome pipelineOutcome
			if gate != nil {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := f.start(ctx, f.cortex.SessionExecutor, plan)
				if admitted := awaitPipeline(t, gate.admitted, "owned cancellation admission"); admitted != plan.Call.ID {
					t.Fatal("wrong generated process reached the cancellation barrier")
				}
				cancel()
				outcome = awaitPipeline(t, done, "owned canceled process and result")
				awaitPipeline(t, gate.drained, "owned canceled child drain")
			} else {
				outcome = f.run(plan)
			}
			after, err := evidence.Snapshot(context.Background(), f.root)
			if err != nil {
				t.Fatal(err)
			}
			assertPipelineFailed(t, outcome)
			if outcome.result == nil || outcome.result.WriteToolEffects != 0 || len(outcome.result.WrittenPaths) != 0 || len(outcome.result.ToolReceipts) != 0 || outcome.result.TurnOutcome == "/done" {
				t.Fatalf("arbitrary generated execution was promoted to a recognized/accepted write: %+v", outcome.result)
			}
			if mode == "effect" {
				requirePipelineTransportOutcomeWithWriteRefusal(t, f, plan, outcome)
				if before == after {
					t.Fatal("whole-workspace snapshot missed the real fixture effect")
				}
				f.assertTotals(1, 1, 1)
			} else if mode == "not_approved" {
				f.assertNoEffect(plan)
				f.assertTotals(0, 0, 0)
				if before != after {
					t.Fatal("pre-dispatch refusal changed the fixture workspace")
				}
			} else {
				f.assertFailedLearning(plan)
				if mode == "missing_effect" || mode == "canceled_after_start" {
					f.assertNoEffect(plan)
					f.assertTotals(1, 0, 1)
				} else {
					f.assertTotals(1, 1, 1)
				}
				if before == after {
					t.Fatal("snapshot did not observe the started process's journal/effect")
				}
			}
			provider, ok := any(outcome.result).(generatedReceiptResult)
			if !ok {
				t.Fatal("session result discarded its typed generated-process receipt")
			}
			receipts := provider.GeneratedToolReceipts()
			if mode == "not_approved" {
				if len(receipts) != 0 {
					t.Fatalf("pre-bridge refusal invented a receipt: %+v", receipts)
				}
			} else {
				if len(receipts) != 1 {
					t.Fatalf("actual bridge receipt count=%d, want one", len(receipts))
				}
				receipt := receipts[0]
				request := f.request(plan)
				if receipt.Request != request || !receipt.ProcessStarted || !receipt.Feedback.Durable || receipt.Feedback.ExecutionID != request.Key() || receipt.Feedback.Fingerprint != request.Fingerprint() {
					t.Fatalf("caller lost exact process/feedback identity: %+v", receipt)
				}
				if mode == "effect" && (!receipt.ValidationCompleted || !receipt.ValidationPassed || receipt.Err() != nil) {
					t.Fatalf("primitive receipt lost validation while task acceptance still refuses: %+v", receipt)
				}
				if mode == "missing_effect" && (!receipt.ValidationCompleted || receipt.ValidationPassed || receipt.ValidationError == nil) {
					t.Fatalf("snapshot delta or prose replaced missing-target validation: %+v", receipt)
				}
				if mode == "partial_failure" && (receipt.BackendError == nil || !strings.Contains(receipt.Output, "partial fixture output")) {
					t.Fatalf("partial execution evidence was lost: %+v", receipt)
				}
				if mode == "canceled_after_start" && (receipt.BackendError == nil || !strings.Contains(receipt.BackendError.Error(), context.Canceled.Error())) {
					t.Fatalf("actual cancellation evidence was lost: %+v", receipt)
				}
				receipts[0].Request.Target = "caller-local alteration"
				if provider.GeneratedToolReceipts()[0].Request != request {
					t.Fatal("caller mutation changed the retained receipt")
				}
			}
			f.assertReleased(plan)
			f.assertBaseline()
		})
	}
}

func TestGeneratedPipelineReceiptCallerIsolation(t *testing.T) {
	f := newPipelineFixtureWithCompanion(t, false, productionPipelineCompanion)
	gate := newPipelineGate(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	plans := []pipelinePlan{f.plan("receipt-a", "owned A\n", "", gate.server.URL), f.planForTool(productionPipelineCompanion, "receipt-b", "owned B\n", "", gate.server.URL)}
	var completions []<-chan pipelineOutcome
	for _, plan := range plans {
		f.approve(plan)
		executor := f.cortex.SessionExecutor.CloneForTask()
		executor.SetSessionID("receipt-owner-" + plan.Call.ID)
		f.configure(executor)
		completions = append(completions, f.start(ctx, executor, plan))
	}
	admitted := map[string]bool{}
	for range plans {
		admitted[awaitPipeline(t, gate.admitted, "independent receipt owner admission")] = true
	}
	if len(admitted) != 2 || !admitted[plans[0].Call.ID] || !admitted[plans[1].Call.ID] {
		t.Fatal("receipt ownership control did not reach actual concurrent processes")
	}
	gate.open()
	for index, plan := range plans {
		outcome := awaitPipeline(t, completions[index], "independent receipt owner completion")
		requirePipelineTransportOutcomeWithWriteRefusal(t, f, plan, outcome)
		provider, ok := any(outcome.result).(generatedReceiptResult)
		if !ok {
			t.Fatal("concurrent caller lost its typed generated receipt")
		}
		receipts := provider.GeneratedToolReceipts()
		if len(receipts) != 1 || receipts[0].Request != f.request(plan) {
			t.Fatalf("another caller's receipt leaked into this result: %+v", receipts)
		}
		f.assertReleased(plan)
		awaitPipeline(t, gate.drained, "independent receipt owner drain")
	}
	f.assertTotals(2, 2, 2)
	f.assertBaseline()
}
