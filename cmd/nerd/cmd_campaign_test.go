package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/campaign"
	"codenerd/internal/consultation"
	"codenerd/internal/session"
)

// panelExecutor is a TaskExecutor whose consultations are the
// one-rejection-plus-one-timeout fixture: the coder rejects, the tester never
// answers, the rest approve. It records the intent verbs it was given.
type panelExecutor struct {
	session.TaskExecutor
	mu    sync.Mutex
	verbs []string
}

func (p *panelExecutor) Execute(ctx context.Context, req session.TaskRequest) (string, error) {
	p.mu.Lock()
	p.verbs = append(p.verbs, req.IntentVerb)
	p.mu.Unlock()
	switch req.IntentVerb {
	case "/consult/coder":
		return "VOTE: REJECT\nCONFIDENCE: 90\nREASONING:\nthe write sets collide", nil
	case "/consult/tester":
		<-ctx.Done()
		return "", ctx.Err()
	}
	return "VOTE: APPROVE\nCONFIDENCE: 80\nREASONING:\nfine", nil
}

// The CLI door builds its provider exactly like this (buildCampaignOrchestratorConfig):
// a failed tester must neither erase the coder's rejection nor let the panel
// approve, and every specialist is addressed as /consult/<name>.
func TestCampaignCLIDoor_RejectionSurvivesTimeout(t *testing.T) {
	exec := &panelExecutor{}
	var provider campaign.ConsultationProvider = consultation.New(session.TaskConsultationSpawner{Executor: exec})
	board := campaign.NewShardAdvisoryBoard(provider)
	cfg := campaign.DefaultAdvisoryConfig()
	cfg.ConsultTimeout = 50 * time.Millisecond
	board.WithConfig(cfg)

	responses, err := board.ConsultAdvisors(context.Background(), campaign.AdvisoryRequest{CampaignID: "/c", Goal: "g"})
	if err != nil {
		t.Fatal(err)
	}
	s := board.SynthesizeVotes(responses)
	if s.Approved || !s.Incomplete {
		t.Fatalf("approved=%v incomplete=%v, want an incomplete, unapproved panel", s.Approved, s.Incomplete)
	}
	if len(s.BlockingConcerns) != 1 || s.BlockingConcerns[0].Advisor != "coder" {
		t.Fatalf("blocking concerns = %+v, want the coder's rejection", s.BlockingConcerns)
	}
	if len(s.Missing) != 1 || s.Missing[0].Advisor != "tester" {
		t.Fatalf("missing = %+v, want tester", s.Missing)
	}
	exec.mu.Lock()
	defer exec.mu.Unlock()
	for _, v := range exec.verbs {
		if !strings.HasPrefix(v, "/consult/") {
			t.Fatalf("intent verb %q is not /consult/<name>", v)
		}
	}
}
