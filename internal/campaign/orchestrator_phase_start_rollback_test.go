package campaign

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/core"
)

// rollbackStubKernel is a minimal core.Kernel stub for startNextPhase.
// It only serves phase_eligible queries and records campaign_phase writes.
type rollbackStubKernel struct {
	core.Kernel
	eligible       []string
	asserted       []core.Fact
	retractedFacts []core.Fact
	assertErr      error
	queryErr       error
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
	return nil
}

func (s *rollbackStubKernel) RetractFact(f core.Fact) error {
	s.retractedFacts = append(s.retractedFacts, f)
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
