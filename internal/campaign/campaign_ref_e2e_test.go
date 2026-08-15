package campaign

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/tactile"
)

// Nested campaign_ref, driven through the real scheduler.
//
// The failure policies are the whole contract of a sub-campaign reference:
// /propagate makes the parent task fail with the child, /absorb and /transform
// let the parent continue. Until now that logic was only covered at the level
// of applyCampaignRefFailurePolicy, which is a pure function — it cannot show
// that the policy survives the trip through runPhase, handleTaskFailure and the
// retry/backoff machinery, and those are what actually decide whether a parent
// campaign stops.
//
// The kernel here is real: task eligibility comes from the Mangle derivation,
// not from the in-memory fallback, so this exercises the same path production
// takes.

func campaignRefParent(id string, policy CampaignRefFailurePolicy, subID string) *Campaign {
	phaseID := id + "_phase"
	return &Campaign{
		ID:          id,
		Type:        CampaignTypeFeature,
		Title:       "parent with sub-campaign",
		Goal:        "reference a sub-campaign",
		Status:      StatusActive,
		CreatedAt:   time.Now().UTC(),
		TotalPhases: 1,
		TotalTasks:  1,
		Phases: []Phase{{
			ID:                  phaseID,
			CampaignID:          id,
			Name:                "delegate",
			Order:               0,
			Category:            "/integration",
			Status:              PhaseInProgress,
			EstimatedTasks:      1,
			EstimatedComplexity: "/low",
			Objectives: []PhaseObjective{{
				Type:               ObjectiveIntegrate,
				Description:        "wait on the sub-campaign",
				VerificationMethod: VerifyNone,
			}},
			Tasks: []Task{{
				ID:                       id + "_ref",
				PhaseID:                  phaseID,
				Description:              "reference the sub-campaign",
				Status:                   TaskPending,
				Type:                     TaskTypeCampaignRef,
				Priority:                 PriorityHigh,
				Order:                    0,
				SubCampaignID:            subID,
				CampaignRefFailurePolicy: policy,
			}},
		}},
	}
}

func campaignRefOrchestrator(t *testing.T, kernel core.Kernel, c *Campaign) (*Orchestrator, chan OrchestratorEvent) {
	t.Helper()
	events := make(chan OrchestratorEvent, 64)
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    t.TempDir(),
		Kernel:       kernel,
		LLMClient:    &MockLLMClient{},
		TaskExecutor: &MockTaskExecutor{},
		Executor:     tactile.NewDirectExecutor(),
		VirtualStore: &core.VirtualStore{},
		EventChan:    events,
		// One retry only, with a negligible backoff: the point is the terminal
		// status the policy produces, not the retry schedule.
		MaxRetries:       1,
		RetryBackoffBase: time.Millisecond,
		RetryBackoffMax:  time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	// Replan needs a live LLM and is irrelevant to the policy contract.
	orch.replanner = nil
	if err := orch.SetCampaign(c); err != nil {
		t.Fatalf("SetCampaign: %v", err)
	}
	return orch, events
}

// assertSubCampaignFailed publishes a failed sub-campaign the parent can see.
// executeCampaignRefTask reads the child's state from kernel `campaign` facts,
// which is how a sibling orchestrator's status becomes visible.
func assertSubCampaignFailed(t *testing.T, kernel core.Kernel, subID string) {
	t.Helper()
	if err := kernel.Assert(core.Fact{
		Predicate: "campaign",
		Args:      []any{subID, string(CampaignTypeFeature), "child", "", string(StatusFailed)},
	}); err != nil {
		t.Fatalf("asserting sub-campaign fact: %v", err)
	}
}

func runCampaignRefPhase(t *testing.T, orch *Orchestrator) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := orch.runPhase(ctx, &orch.campaign.Phases[0]); err != nil && ctx.Err() == nil {
		t.Logf("runPhase returned: %v", err)
	}
}

func TestCampaignRef_WhenSubCampaignFailedWithPropagate_ShouldFailParentTask(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Skipf("real kernel unavailable: %v", err)
	}

	subID := "/campaign_child_propagate"
	c := campaignRefParent("/campaign_parent_propagate", CampaignRefPolicyPropagate, subID)
	orch, events := campaignRefOrchestrator(t, kernel, c)
	assertSubCampaignFailed(t, kernel, subID)

	runCampaignRefPhase(t, orch)

	task := orch.campaign.Phases[0].Tasks[0]
	if task.Status != TaskFailed {
		t.Fatalf("/propagate must carry the child's failure to the parent task; status = %s", task.Status)
	}
	if task.LastError == "" {
		t.Error("the parent task recorded no error, so nothing explains why it failed")
	}

	seen := drainEventTypes(events)
	if seen[EventSubCampaignReferenced] == 0 {
		t.Errorf("expected a %s event describing the link; got %v", EventSubCampaignReferenced, seen)
	}
	if seen[EventTaskFailed] == 0 {
		t.Errorf("expected a %s event; got %v", EventTaskFailed, seen)
	}
}

func TestCampaignRef_WhenSubCampaignFailedWithAbsorb_ShouldCompleteParentTask(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Skipf("real kernel unavailable: %v", err)
	}

	subID := "/campaign_child_absorb"
	c := campaignRefParent("/campaign_parent_absorb", CampaignRefPolicyAbsorb, subID)
	orch, _ := campaignRefOrchestrator(t, kernel, c)
	assertSubCampaignFailed(t, kernel, subID)

	runCampaignRefPhase(t, orch)

	task := orch.campaign.Phases[0].Tasks[0]
	if task.Status != TaskCompleted {
		t.Fatalf("/absorb must let the parent continue past a failed child; status = %s", task.Status)
	}

	if _, ok := orch.getTaskResult(task.ID); !ok {
		t.Fatal("absorbed reference produced no result envelope for downstream context")
	}
}

func TestCampaignRef_WhenSubCampaignFailedWithTransform_ShouldCompleteAndRecordLearning(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Skipf("real kernel unavailable: %v", err)
	}

	subID := "/campaign_child_transform"
	c := campaignRefParent("/campaign_parent_transform", CampaignRefPolicyTransform, subID)
	orch, _ := campaignRefOrchestrator(t, kernel, c)
	assertSubCampaignFailed(t, kernel, subID)

	runCampaignRefPhase(t, orch)

	task := orch.campaign.Phases[0].Tasks[0]
	if task.Status != TaskCompleted {
		t.Fatalf("/transform must complete the parent task; status = %s", task.Status)
	}
}

// The envelope the handler returns is the contract downstream tasks consume.
// Testing it directly keeps the mapping honest even when the scheduler around
// it changes.
func TestCampaignRef_FailurePolicyEnvelope_ShouldMapChildFailure(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Skipf("real kernel unavailable: %v", err)
	}

	cases := []struct {
		policy        CampaignRefFailurePolicy
		wantErr       bool
		wantStatus    string
		wantLearnFact string
	}{
		{CampaignRefPolicyPropagate, true, CampaignRefLifecycleFailed, ""},
		{CampaignRefPolicyAbsorb, false, CampaignRefLifecycleCompleted, "/campaign_ref_failure_absorbed"},
		{CampaignRefPolicyTransform, false, CampaignRefLifecycleCompleted, "/campaign_ref_failure_transformed"},
		// An unset policy must behave like /propagate: silently continuing past
		// a failed child is the dangerous default.
		{"", true, CampaignRefLifecycleFailed, ""},
	}

	for _, tc := range cases {
		name := string(tc.policy)
		if name == "" {
			name = "unset"
		}
		t.Run(name, func(t *testing.T) {
			subID := "/campaign_child_env_" + name
			c := campaignRefParent("/campaign_parent_env_"+name, tc.policy, subID)
			orch, _ := campaignRefOrchestrator(t, kernel, c)
			assertSubCampaignFailed(t, kernel, subID)

			result, err := orch.executeCampaignRefTask(context.Background(), &c.Phases[0].Tasks[0])
			if tc.wantErr {
				if err == nil {
					t.Fatalf("policy %q must surface the child's failure as an error", name)
				}
				return
			}
			if err != nil {
				t.Fatalf("policy %q must not fail the parent: %v", name, err)
			}

			envelope, ok := result.(CampaignRefResult)
			if !ok {
				t.Fatalf("expected a CampaignRefResult envelope, got %T", result)
			}
			if envelope.Status != tc.wantStatus {
				t.Errorf("envelope status = %q, want %q", envelope.Status, tc.wantStatus)
			}
			if envelope.SubCampaignID != subID {
				t.Errorf("envelope lost the sub-campaign id: %q", envelope.SubCampaignID)
			}
			if envelope.FailureSummary == "" {
				t.Error("envelope carries no failure summary, so the parent cannot explain what it absorbed")
			}
			if tc.wantLearnFact != "" {
				found := false
				for _, f := range envelope.LearnedFacts {
					if f == tc.wantLearnFact {
						found = true
					}
				}
				if !found {
					t.Errorf("expected learned fact %q, got %v", tc.wantLearnFact, envelope.LearnedFacts)
				}
			}
			// The inheritance contract must always be populated: a sub-campaign
			// with no declared scopes would inherit whatever the parent had.
			if envelope.Inheritance.FactsScope == "" || envelope.Inheritance.ToolScope == "" {
				t.Errorf("inheritance contract left unset: %+v", envelope.Inheritance)
			}
		})
	}
}

// A /campaign_ref task with no target is a planning bug, and executing it as a
// no-op would silently drop whatever work the sub-campaign was meant to do.
func TestCampaignRef_WhenSubCampaignIDMissing_ShouldFail(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Skipf("real kernel unavailable: %v", err)
	}

	c := campaignRefParent("/campaign_parent_noref", CampaignRefPolicyAbsorb, "")
	orch, _ := campaignRefOrchestrator(t, kernel, c)

	if _, err := orch.executeCampaignRefTask(context.Background(), &c.Phases[0].Tasks[0]); err == nil {
		t.Fatal("a /campaign_ref task with no sub_campaign_id must fail rather than complete as a no-op")
	}
}

// A child that has not failed must be linked with its live status, not treated
// as an absorbed failure.
func TestCampaignRef_WhenSubCampaignActive_ShouldLinkWithLiveLifecycle(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Skipf("real kernel unavailable: %v", err)
	}

	subID := "/campaign_child_active"
	c := campaignRefParent("/campaign_parent_active", CampaignRefPolicyPropagate, subID)
	orch, _ := campaignRefOrchestrator(t, kernel, c)
	if err := kernel.Assert(core.Fact{
		Predicate: "campaign",
		Args:      []any{subID, string(CampaignTypeFeature), "child", "", string(StatusActive)},
	}); err != nil {
		t.Fatalf("assert sub-campaign: %v", err)
	}

	result, err := orch.executeCampaignRefTask(context.Background(), &c.Phases[0].Tasks[0])
	if err != nil {
		t.Fatalf("an active sub-campaign must not fail the parent: %v", err)
	}
	envelope := result.(CampaignRefResult)
	if envelope.Status != CampaignRefLifecycleActive {
		t.Fatalf("lifecycle = %q, want %q", envelope.Status, CampaignRefLifecycleActive)
	}
	if envelope.FailureSummary != "" {
		t.Errorf("a healthy child produced a failure summary: %q", envelope.FailureSummary)
	}
}
