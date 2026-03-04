package campaign

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWithTaskExecutionSnapshot_RollsBackOnError(t *testing.T) {
	orch := newSnapshotTestOrchestrator()

	beforeCampaign, err := cloneCampaignForTest(orch.campaign)
	if err != nil {
		t.Fatalf("cloneCampaignForTest() error = %v", err)
	}
	beforeResults := cloneStringMapForTest(orch.taskResults)
	beforeOrder := append([]string(nil), orch.taskResultOrder...)

	expectedErr := errors.New("forced mutation failure")
	_, err = orch.withTaskExecutionSnapshot(&Task{ID: "/task_txn", Type: TaskTypeAssaultDiscover}, func() (any, error) {
		orch.campaign.Phases[0].Tasks = append(orch.campaign.Phases[0].Tasks, Task{
			ID:          "/task_added",
			PhaseID:     "/phase_0",
			Description: "Mutated task",
			Status:      TaskPending,
			Type:        TaskTypeFileModify,
			Priority:    PriorityNormal,
			Order:       1,
		})
		orch.campaign.TotalTasks++
		orch.campaign.UpdatedAt = orch.campaign.UpdatedAt.Add(5 * time.Minute)
		orch.taskResults["/task_added"] = "mutated result"
		orch.taskResultOrder = append(orch.taskResultOrder, "/task_added")
		return nil, expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if !reflect.DeepEqual(orch.campaign, beforeCampaign) {
		t.Fatalf("campaign state not rolled back\nwant: %#v\ngot: %#v", beforeCampaign, orch.campaign)
	}
	if !reflect.DeepEqual(orch.taskResults, beforeResults) {
		t.Fatalf("taskResults not rolled back\nwant: %#v\ngot: %#v", beforeResults, orch.taskResults)
	}
	if !reflect.DeepEqual(orch.taskResultOrder, beforeOrder) {
		t.Fatalf("taskResultOrder not rolled back\nwant: %#v\ngot: %#v", beforeOrder, orch.taskResultOrder)
	}
}

func TestWithTaskExecutionSnapshot_PersistsOnSuccess(t *testing.T) {
	orch := newSnapshotTestOrchestrator()

	res, err := orch.withTaskExecutionSnapshot(&Task{ID: "/task_txn", Type: TaskTypeAssaultDiscover}, func() (any, error) {
		orch.campaign.Phases[0].Tasks = append(orch.campaign.Phases[0].Tasks, Task{
			ID:          "/task_added",
			PhaseID:     "/phase_0",
			Description: "Mutated task",
			Status:      TaskPending,
			Type:        TaskTypeFileModify,
			Priority:    PriorityNormal,
			Order:       1,
		})
		orch.campaign.TotalTasks++
		orch.taskResults["/task_added"] = "mutated result"
		orch.taskResultOrder = append(orch.taskResultOrder, "/task_added")
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("withTaskExecutionSnapshot() error = %v", err)
	}
	if res != "ok" {
		t.Fatalf("expected result 'ok', got %v", res)
	}
	if len(orch.campaign.Phases[0].Tasks) != 2 {
		t.Fatalf("expected 2 tasks after successful mutation, got %d", len(orch.campaign.Phases[0].Tasks))
	}
	if _, ok := orch.taskResults["/task_added"]; !ok {
		t.Fatalf("expected /task_added in taskResults after successful mutation")
	}
}

func TestWithTaskExecutionSnapshot_ScopedToMutatingTypes(t *testing.T) {
	orch := newSnapshotTestOrchestrator()

	_, err := orch.withTaskExecutionSnapshot(&Task{ID: "/task_non_mutating", Type: TaskTypeResearch}, func() (any, error) {
		orch.campaign.Phases[0].Tasks = append(orch.campaign.Phases[0].Tasks, Task{
			ID:          "/task_added",
			PhaseID:     "/phase_0",
			Description: "Should stay mutated (no snapshot for this type)",
			Status:      TaskPending,
			Type:        TaskTypeFileModify,
			Priority:    PriorityNormal,
			Order:       1,
		})
		return nil, errors.New("regular execution failure")
	})
	if err == nil {
		t.Fatalf("expected error for non-mutating task execution")
	}

	// Snapshot wrapper is intentionally scoped, so non-mutating task types do not rollback.
	if len(orch.campaign.Phases[0].Tasks) != 2 {
		t.Fatalf("expected mutation to persist for non-mutating task type, got %d tasks", len(orch.campaign.Phases[0].Tasks))
	}
}

func TestWithTaskExecutionSnapshot_RollsBackOnPanic(t *testing.T) {
	orch := newSnapshotTestOrchestrator()

	beforeCampaign, err := cloneCampaignForTest(orch.campaign)
	if err != nil {
		t.Fatalf("cloneCampaignForTest() error = %v", err)
	}

	_, err = orch.withTaskExecutionSnapshot(&Task{ID: "/task_txn", Type: TaskTypeAssaultTriage}, func() (any, error) {
		orch.campaign.TotalTasks++
		panic("simulated panic")
	})

	if err == nil {
		t.Fatalf("expected panic to be converted into an error")
	}
	if !strings.Contains(err.Error(), "panic during") {
		t.Fatalf("expected panic classification in error, got %v", err)
	}
	if !reflect.DeepEqual(orch.campaign, beforeCampaign) {
		t.Fatalf("campaign state not rolled back after panic")
	}
}

func TestWithTaskExecutionSnapshot_RiskGateBlocksMutatingTaskWithForceBlockMode(t *testing.T) {
	orch := newSnapshotTestOrchestrator()
	orch.config.EnableRiskAutoWiring = true
	orch.config.RiskGateMode = RiskGateModeForceBlock
	orch.riskDecision = &CampaignRiskDecision{
		Score:         90,
		Threshold:     70,
		Gated:         true,
		OverrideLevel: "score_threshold",
		SnapshotID:    "snapshot-1",
	}

	ran := false
	_, err := orch.withTaskExecutionSnapshot(&Task{ID: "/task_seed", Type: TaskTypeFileModify}, func() (any, error) {
		ran = true
		return "ok", nil
	})
	if err == nil {
		t.Fatalf("expected risk gate block error")
	}
	if !strings.Contains(err.Error(), "risk gate blocked mutating task") {
		t.Fatalf("unexpected error: %v", err)
	}
	if ran {
		t.Fatalf("risk-gated callback should not execute")
	}
}

func TestExecuteTaskWithRollback_MicroCheckpointFailureRollsBackState(t *testing.T) {
	orch := newSnapshotTestOrchestrator()
	orch.workspace = t.TempDir()
	orch.taskExecutor = &MockTaskExecutor{
		ExecuteFunc: func(ctx context.Context, intent string, task string) (string, error) {
			return "ok", nil
		},
	}

	task := &orch.campaign.Phases[0].Tasks[0]
	task.Type = TaskTypeFileModify
	task.Shard = "coder"
	task.WriteSet = []string{"internal/missing_file.go"}

	_, err := orch.executeTaskWithRollback(context.Background(), task)
	if err == nil {
		t.Fatalf("expected micro-checkpoint failure")
	}
	if !strings.Contains(err.Error(), "micro-checkpoint") {
		t.Fatalf("expected micro-checkpoint error, got %v", err)
	}
	if got := orch.campaign.Phases[0].Tasks[0].Status; got != TaskPending {
		t.Fatalf("expected task status rollback to pending, got %s", got)
	}
}

func TestRecoverJournalSequence_TruncatesInvalidTail(t *testing.T) {
	orch := &Orchestrator{
		nerdDir: t.TempDir(),
	}
	campaignID := "campaign_recover"
	path := orch.journalPath(campaignID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	line1 := mustJournalLine(t, campaignJournalEvent{
		Seq:              1,
		TimestampUnix:    1,
		EventType:        "snapshot_write_requested",
		CampaignID:       campaignID,
		SnapshotChecksum: "a1",
	})
	line2 := mustJournalLine(t, campaignJournalEvent{
		Seq:              2,
		TimestampUnix:    2,
		EventType:        "snapshot_write_committed",
		CampaignID:       campaignID,
		SnapshotChecksum: "a1",
	})
	// Non-monotonic sequence should be treated as invalid tail and truncated.
	line5 := mustJournalLine(t, campaignJournalEvent{
		Seq:              5,
		TimestampUnix:    5,
		EventType:        "snapshot_write_requested",
		CampaignID:       campaignID,
		SnapshotChecksum: "b2",
	})

	payload := strings.Join([]string{line1, line2, line5}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(payload), 0644); err != nil {
		t.Fatalf("write journal seed failed: %v", err)
	}

	orch.recoverJournalSequence(campaignID)
	if got := orch.journalSeq.Load(); got != 2 {
		t.Fatalf("expected recovered sequence 2, got %d", got)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read repaired journal failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected truncated journal length 2, got %d", len(lines))
	}
}

func TestWriteCampaignSnapshotAtomic_ReplacesExistingAndCleansTemp(t *testing.T) {
	orch := &Orchestrator{}
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.json")

	if err := orch.writeCampaignSnapshotAtomic(path, []byte("v1")); err != nil {
		t.Fatalf("first atomic write failed: %v", err)
	}
	if err := orch.writeCampaignSnapshotAtomic(path, []byte("v2")); err != nil {
		t.Fatalf("second atomic write failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot failed: %v", err)
	}
	if string(data) != "v2" {
		t.Fatalf("expected replaced snapshot contents 'v2', got %q", string(data))
	}

	tempMatches, err := filepath.Glob(filepath.Join(dir, "campaign.json.tmp-*"))
	if err != nil {
		t.Fatalf("glob temp files failed: %v", err)
	}
	if len(tempMatches) != 0 {
		t.Fatalf("expected no temp files after atomic write, found %v", tempMatches)
	}
}

func TestRiskDecision_ConfigurableAndDeterministic(t *testing.T) {
	orch := newSnapshotTestOrchestrator()
	orch.config.EnableRiskAutoWiring = true
	orch.config.RiskGateMode = RiskGateModeAuto
	orch.config.RiskGateThreshold = 65
	orch.config.GlobalRiskGate = true
	orch.riskGateState = riskGateResolved{Advisory: true, Edge: true, Northstar: false}
	orch.campaign.Phases[0].Tasks[0].WriteSet = []string{"internal/campaign/txn.go"}

	d1 := orch.computeCampaignRiskDecision()
	d2 := orch.computeCampaignRiskDecision()
	if d1 == nil || d2 == nil {
		t.Fatalf("expected risk decisions, got d1=%v d2=%v", d1, d2)
	}
	if d1.Score != d2.Score || d1.Gated != d2.Gated || d1.SnapshotID != d2.SnapshotID || d1.OverrideLevel != d2.OverrideLevel {
		t.Fatalf("expected deterministic decision, got d1=%+v d2=%+v", d1, d2)
	}

	orch.config.RiskGateMode = RiskGateModeForceAllow
	forcedAllow := orch.computeCampaignRiskDecision()
	if forcedAllow == nil || forcedAllow.Gated {
		t.Fatalf("expected force-allow to disable gating, got %+v", forcedAllow)
	}

	orch.config.RiskGateMode = RiskGateModeForceBlock
	forcedBlock := orch.computeCampaignRiskDecision()
	if forcedBlock == nil || !forcedBlock.Gated {
		t.Fatalf("expected force-block to enable gating, got %+v", forcedBlock)
	}

	orch.config.RiskGateMode = RiskGateModeAuto
	orch.config.EnableRiskAutoWiring = false
	orch.riskDecision = &CampaignRiskDecision{Gated: true}
	if got := orch.shouldGateTask("/task_seed"); got {
		t.Fatalf("expected task gate false when auto wiring disabled in auto mode")
	}

	orch.config.EnableRiskAutoWiring = true
	orch.config.TaskRiskOverrides = map[string]bool{"/task_seed": false}
	orch.riskDecision = &CampaignRiskDecision{Gated: true}
	if got := orch.shouldGateTask("/task_seed"); got {
		t.Fatalf("expected task override to disable gating")
	}
}

func newSnapshotTestOrchestrator() *Orchestrator {
	now := time.Now()
	return &Orchestrator{
		kernel: &MockKernel{},
		campaign: &Campaign{
			ID:        "/campaign_txn_test",
			Type:      CampaignTypeCustom,
			Title:     "transaction test",
			Goal:      "verify rollback",
			Status:    StatusActive,
			CreatedAt: now,
			UpdatedAt: now,
			Phases: []Phase{
				{
					ID:         "/phase_0",
					CampaignID: "/campaign_txn_test",
					Name:       "phase 0",
					Order:      0,
					Status:     PhaseInProgress,
					Tasks: []Task{
						{
							ID:          "/task_seed",
							PhaseID:     "/phase_0",
							Description: "seed",
							Status:      TaskPending,
							Type:        TaskTypeFileModify,
							Priority:    PriorityNormal,
							Order:       0,
						},
					},
				},
			},
			CompletedPhases: 0,
			TotalPhases:     1,
			CompletedTasks:  0,
			TotalTasks:      1,
		},
		taskResults:     map[string]string{"/task_seed": "seed-result"},
		taskResultOrder: []string{"/task_seed"},
		config: OrchestratorConfig{
			TaskResultCacheLimit: 100,
		},
	}
}

func mustJournalLine(t *testing.T, ev campaignJournalEvent) string {
	t.Helper()
	ev.Checksum = checksumJournalEvent(ev)
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal journal event failed: %v", err)
	}
	return string(raw)
}

func cloneCampaignForTest(src *Campaign) (*Campaign, error) {
	if src == nil {
		return nil, nil
	}

	raw, err := json.Marshal(src)
	if err != nil {
		return nil, err
	}

	var cloned Campaign
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

func cloneStringMapForTest(src map[string]string) map[string]string {
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
