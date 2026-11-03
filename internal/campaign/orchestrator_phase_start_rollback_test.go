package campaign

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/northstar"
	"codenerd/internal/types"
)

// rollbackStubKernel is a minimal core.Kernel stub for startNextPhase.
// It only serves phase_eligible queries and records campaign_phase writes.
type rollbackStubKernel struct {
	core.Kernel
	eligible       []string
	asserted       []core.Fact
	retractedFacts []core.Fact
	assertErr      error
	assertStartErr error
	retractErr     error
	queryErr       error
	phaseFact      *core.Fact
}

func (s *rollbackStubKernel) Query(predicate string) ([]core.Fact, error) {
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	if predicate != "phase_eligible" {
		return nil, nil
	}
	var out []core.Fact
	for _, id := range s.eligible {
		out = append(out, core.Fact{Predicate: "phase_eligible", Args: []any{id}})
	}
	return out, nil
}

func (s *rollbackStubKernel) Assert(f core.Fact) error {
	if s.assertErr != nil && f.Predicate == "campaign_phase" {
		return s.assertErr
	}
	s.asserted = append(s.asserted, f)
	if f.Predicate == "campaign_phase" {
		s.phaseFact = &f
	}
	return nil
}

func (s *rollbackStubKernel) RetractFact(f core.Fact) error {
	s.retractedFacts = append(s.retractedFacts, f)
	if f.Predicate == "campaign_phase" {
		s.phaseFact = nil
	}
	return nil
}

func rollbackTestCampaign(phaseID string, status PhaseStatus) *Campaign {
	return &Campaign{
		ID: "camp-rollback",
		Phases: []Phase{{
			ID:         phaseID,
			CampaignID: "camp-rollback",
			Name:       "Build",
			Order:      0,
			Status:     status,
		}},
	}
}

type rollbackBlockingClient struct {
	calls int
}

func (c *rollbackBlockingClient) CompleteWithSystem(_ context.Context, _, _ string) (string, error) {
	c.calls++
	return "SCORE: 0.0\nRESULT: blocked\nEXPLANATION: refused", nil
}

func TestStartNextPhase_ObserverRefusalRestoresPhase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		ctx             context.Context
		eligible        []string
		initialStatus   PhaseStatus
		assertErr       error
		queryErr        error
		wantErr         bool
		wantStatus      PhaseStatus
		wantAssertCount int
	}{
		{
			name:            "happy starts pending phase",
			ctx:             context.Background(),
			eligible:        []string{"phase-1"},
			initialStatus:   PhasePending,
			wantErr:         false,
			wantStatus:      PhaseInProgress,
			wantAssertCount: 1,
		},
		{
			name:            "edge empty eligible returns error",
			ctx:             context.Background(),
			eligible:        nil,
			initialStatus:   PhasePending,
			wantErr:         true,
			wantStatus:      PhasePending,
			wantAssertCount: 0,
		},
		{
			name:            "nil context returns error without mutation",
			ctx:             nil,
			eligible:        []string{"phase-1"},
			initialStatus:   PhasePending,
			wantErr:         true,
			wantStatus:      PhasePending,
			wantAssertCount: 0,
		},
		{
			name:            "empty phase id eligible is skipped",
			ctx:             context.Background(),
			eligible:        []string{""},
			initialStatus:   PhasePending,
			wantErr:         true,
			wantStatus:      PhasePending,
			wantAssertCount: 0,
		},
		{
			name:            "audited kernel assert failure restores pending",
			ctx:             context.Background(),
			eligible:        []string{"phase-1"},
			initialStatus:   PhasePending,
			assertErr:       errors.New("injected campaign_phase assert failure"),
			wantErr:         true,
			wantStatus:      PhasePending,
			wantAssertCount: 0,
		},
		{
			name:            "query error leaves phase pending",
			ctx:             context.Background(),
			eligible:        []string{"phase-1"},
			initialStatus:   PhasePending,
			queryErr:        errors.New("injected query failure"),
			wantErr:         true,
			wantStatus:      PhasePending,
			wantAssertCount: 0,
		},
	}

	t.Run("observer refusal restores pending phase", func(t *testing.T) {
		t.Parallel()
		workspace := t.TempDir()
		store, err := northstar.NewStore(filepath.Join(workspace, ".nerd"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := store.Close(); err != nil {
				t.Errorf("close northstar store: %v", err)
			}
		})
		guardian := northstar.NewGuardian(store, northstar.DefaultGuardianConfig())
		if err := guardian.Initialize(); err != nil {
			t.Fatal(err)
		}
		if err := guardian.UpdateVision(&northstar.Vision{Mission: "Protect the project"}); err != nil {
			t.Fatal(err)
		}
		client := &rollbackBlockingClient{}
		guardian.SetLLMClient(client)
		campaign := rollbackTestCampaign("phase-1", PhasePending)
		initialFact := campaign.Phases[0].ToFacts()[0]
		stub := &rollbackStubKernel{eligible: []string{"phase-1"}, phaseFact: &initialFact}
		o := &Orchestrator{
			kernel:            stub,
			campaign:          campaign,
			workspace:         workspace,
			northstarObserver: northstar.NewCampaignObserver(guardian),
			eventChan:         make(chan OrchestratorEvent, 1),
		}
		o.SetMetricsSink(NewInMemoryMetrics())
		err = o.startNextPhase(context.Background())
		check := o.northstarObserver.GetPhaseCheck("phase-1")
		if client.calls != 1 || check == nil || check.Result != northstar.AlignmentBlocked {
			t.Fatalf("observer did not block phase: calls=%d check=%+v error=%v", client.calls, check, err)
		}
		if err == nil {
			t.Fatal("startNextPhase() succeeded after observer refusal")
		}
		if got := campaign.Phases[0].Status; got != PhasePending {
			t.Errorf("phase status = %v, want %v", got, PhasePending)
		}
		wantFact := core.Fact{
			Predicate: "campaign_phase",
			Args:      []any{"phase-1", "camp-rollback", "Build", 0, string(PhasePending), ""},
		}
		if !reflect.DeepEqual(stub.phaseFact, &wantFact) {
			t.Errorf("campaign_phase fact = %+v, want %+v", stub.phaseFact, &wantFact)
		}
		if _, ok := o.phaseStarts.Load("phase-1"); ok {
			t.Error("phase start time retained after observer refusal")
		}
		select {
		case event := <-o.eventChan:
			t.Errorf("unexpected phase-start event after refusal: %+v", event)
		default:
		}
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stub := &rollbackStubKernel{
				eligible:  tt.eligible,
				assertErr: tt.assertErr,
				queryErr:  tt.queryErr,
			}
			o := &Orchestrator{
				kernel:   stub,
				campaign: rollbackTestCampaign("phase-1", tt.initialStatus),
			}
			orig := o.campaign.Phases[0].Status
			t.Cleanup(func() {
				_ = orig
			})

			err := o.startNextPhase(tt.ctx)
			if (err != nil) != tt.wantErr {
				t.Fatalf("startNextPhase() error = %v, wantErr %v", err, tt.wantErr)
			}
			got := o.campaign.Phases[0].Status
			if got != tt.wantStatus {
				t.Errorf("phase status = %v, want %v (err=%v)", got, tt.wantStatus, err)
			}
			if len(stub.asserted) != tt.wantAssertCount {
				t.Errorf("asserted campaign_phase count = %d, want %d", len(stub.asserted), tt.wantAssertCount)
			}
		})
	}
}

type phaseStartFailingKernel struct {
	*rollbackStubKernel
	retractFailure error
	startFailure   error
	restoreFailure error
}

func (k *phaseStartFailingKernel) RetractFact(f core.Fact) error {
	if k.retractFailure != nil {
		return k.retractFailure
	}
	return k.rollbackStubKernel.RetractFact(f)
}

func (k *phaseStartFailingKernel) Assert(f core.Fact) error {
	if f.Predicate == "campaign_phase" && len(f.Args) > 4 && types.ExtractString(f.Args[4]) == "/in_progress" {
		if k.startFailure != nil {
			return k.startFailure
		}
	} else if f.Predicate == "campaign_phase" && k.restoreFailure != nil {
		return k.restoreFailure
	}
	return k.rollbackStubKernel.Assert(f)
}

func TestStartNextPhase_KernelFailureRestoresPreviousPhase(t *testing.T) {
	t.Parallel()
	retractFailure := errors.New("retract failed")
	startFailure := errors.New("start assertion failed")
	restoreFailure := errors.New("restore assertion failed")
	for _, tc := range []struct {
		name       string
		retractErr error
		startErr   error
		restoreErr error
		wantErr    error
	}{
		{name: "retract fails and prior fact is restored", retractErr: retractFailure, wantErr: retractFailure},
		{name: "start assertion fails and prior fact is restored", startErr: startFailure, wantErr: startFailure},
		{name: "retract and restoration failures are both returned", retractErr: retractFailure, restoreErr: restoreFailure, wantErr: retractFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := rollbackTestCampaign("phase-1", PhasePending)
			original := c.Phases[0].ToFacts()[0]
			stub := &rollbackStubKernel{eligible: []string{"phase-1"}, phaseFact: &original}
			kernel := &phaseStartFailingKernel{rollbackStubKernel: stub, retractFailure: tc.retractErr, startFailure: tc.startErr, restoreFailure: tc.restoreErr}
			o := &Orchestrator{kernel: kernel, campaign: c, eventChan: make(chan OrchestratorEvent, 1)}
			err := o.startNextPhase(context.Background())
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want cause %v", err, tc.wantErr)
			}
			if tc.restoreErr != nil && !errors.Is(err, tc.restoreErr) {
				t.Errorf("error = %v, missing restoration failure %v", err, tc.restoreErr)
			}
			if got := c.Phases[0].Status; got != PhasePending {
				t.Errorf("phase status = %v, want pending", got)
			}
			if tc.restoreErr == nil {
				if len(stub.asserted) != 1 || stub.phaseFact == nil || len(stub.phaseFact.Args) != 6 || types.ExtractString(stub.phaseFact.Args[4]) != string(PhasePending) {
					t.Errorf("restored fact = %+v, assertions = %+v; want previous pending fact", stub.phaseFact, stub.asserted)
				}
			}
			if _, ok := o.phaseStarts.Load("phase-1"); ok {
				t.Error("failed admission started phase clock")
			}
			select {
			case event := <-o.eventChan:
				t.Errorf("failed admission emitted event: %+v", event)
			default:
			}
		})
	}
}
