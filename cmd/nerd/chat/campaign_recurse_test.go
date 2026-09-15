package chat

import (
	"strings"
	"testing"

	"codenerd/internal/campaign"
)

func TestParseRecurseArgs(t *testing.T) {
	cfg, err := parseRecurseArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxWaves != 1 || len(cfg.Angles) != 0 || len(cfg.Subsystems) != 0 {
		t.Fatalf("defaults wrong: %+v", cfg)
	}

	cfg, err = parseRecurseArgs([]string{"--waves", "3", "--angles", "harden,secure", "--subsystem", "session", "--stall-waves=5"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxWaves != 3 || cfg.StallWaveLimit != 5 {
		t.Fatalf("scalars wrong: %+v", cfg)
	}
	if len(cfg.Angles) != 2 || cfg.Angles[0] != campaign.RecurseAngleHarden {
		t.Fatalf("angles wrong: %+v", cfg.Angles)
	}
	if len(cfg.Subsystems) != 1 || cfg.Subsystems[0] != "session" {
		t.Fatalf("subsystems wrong: %+v", cfg.Subsystems)
	}

	// Bare tokens name subsystems.
	cfg, err = parseRecurseArgs([]string{"session", "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Subsystems) != 2 {
		t.Fatalf("bare subsystems wrong: %+v", cfg.Subsystems)
	}

	for _, args := range [][]string{
		{"--waves", "x"},
		{"--angles", "vibes"},
		{"--nope"},
		{"sauron"},
		{"--subsystem", "sauron"},
	} {
		if _, err := parseRecurseArgs(args); err == nil {
			t.Errorf("%v must fail", args)
		}
	}
}

// recurseTestFinished builds a finished wave-zero campaign: everything
// completed, so the loop judges progress unless the budget is spent.
func recurseTestFinished() *campaign.Campaign {
	return &campaign.Campaign{
		ID:          "/campaign_test",
		RecurseID:   "/recurse_test",
		RecurseWave: 0,
		Status:      campaign.StatusCompleted,
		Phases: []campaign.Phase{{
			ID:   "p",
			Name: "recurse:mangle:Mangle",
			Tasks: []campaign.Task{
				{ID: "t1", Status: campaign.TaskCompleted},
				{ID: "t2", Status: campaign.TaskCompleted},
			},
		}},
	}
}

func recurseTestModel(t *testing.T, loop *campaign.RecurseLoop) Model {
	t.Helper()
	m := NewTestModel()
	m.workspace = t.TempDir()
	m.recurse = &recurseState{loop: loop, cfg: campaign.RecurseConfig{MaxWaves: 1}}
	m.activeCampaign = recurseTestFinished()
	m.campaignProgressChan = make(chan campaign.Progress, 10)
	m.campaignEventChan = make(chan campaign.OrchestratorEvent, 10)
	return m
}

func TestUpdate_RecurseBoundaryFinishesOnBounds(t *testing.T) {
	m := recurseTestModel(t, &campaign.RecurseLoop{MaxWaves: 1})
	finished := recurseTestFinished()

	updated, cmd := m.Update(campaignCompletedMsg(finished))
	result := updated.(Model)
	if result.recurse != nil {
		t.Fatal("spent budget must clear the sweep")
	}
	if result.activeCampaign != nil {
		t.Fatal("spent budget must stand the UI down")
	}
	if cmd != nil {
		t.Fatal("a finished sweep must not arm listeners")
	}
	last := result.history[len(result.history)-1]
	if !strings.Contains(last.Content, "Sweep complete") || !strings.Contains(last.Content, "2 tasks completed") {
		t.Fatalf("missing finish summary: %q", last.Content)
	}
}

func TestUpdate_RecurseBoundaryStallsHonestly(t *testing.T) {
	m := recurseTestModel(t, &campaign.RecurseLoop{StallWaveLimit: 1})
	finished := recurseTestFinished()
	for i := range finished.Phases[0].Tasks {
		finished.Phases[0].Tasks[i].Status = campaign.TaskFailed
	}

	updated, _ := m.Update(campaignCompletedMsg(finished))
	result := updated.(Model)
	if result.recurse != nil {
		t.Fatal("a stalled sweep must clear")
	}
	last := result.history[len(result.history)-1]
	if !strings.Contains(last.Content, "stall fuse") {
		t.Fatalf("missing stall verdict: %q", last.Content)
	}
}

func TestUpdate_RecurseChainFailureStopsHonestly(t *testing.T) {
	// MaxWaves 5 with progress: the loop says proceed, but the test model
	// has no kernel/client, so building the next wave's orchestrator fails.
	// The sweep must stop with the reason visible, not hang or spin.
	m := recurseTestModel(t, &campaign.RecurseLoop{MaxWaves: 5})

	updated, _ := m.Update(campaignCompletedMsg(recurseTestFinished()))
	result := updated.(Model)
	if result.recurse != nil {
		t.Fatal("unchainable sweep must clear")
	}
	last := result.history[len(result.history)-1]
	if !strings.Contains(last.Content, "Could not start the next wave") {
		t.Fatalf("missing chain-failure note: %q", last.Content)
	}
}

func TestUpdate_RecurseErrorWaveCountsAndFinishes(t *testing.T) {
	m := recurseTestModel(t, &campaign.RecurseLoop{MaxWaves: 1})

	updated, _ := m.Update(campaignErrorMsg{err: errRecurseTest})
	result := updated.(Model)
	if result.recurse != nil {
		t.Fatal("error wave on a spent budget must finish the sweep")
	}
	joined := ""
	for _, msg := range result.history {
		joined += msg.Content + "\n"
	}
	if !strings.Contains(joined, "hit an error and the sweep continues") {
		t.Fatalf("wave error must stay visible: %q", joined)
	}
	if !strings.Contains(joined, "Sweep complete") {
		t.Fatalf("sweep must still finish: %q", joined)
	}
}

func TestUpdate_RecurseRiskBlockClearsSweep(t *testing.T) {
	m := recurseTestModel(t, &campaign.RecurseLoop{MaxWaves: 5})
	blocked := campaign.OrchestratorEvent{
		Type:    campaign.EventRiskGateBlocked,
		Message: "risk gate blocked campaign start (/advisory): test",
	}

	updated, _ := m.Update(campaignEventMsg(blocked))
	result := updated.(Model)
	if result.recurse != nil {
		t.Fatal("a risk refusal must stop the sweep: the same targets would refuse again")
	}
}

func TestCampaign_PauseHoldsRecurse(t *testing.T) {
	m := recurseTestModel(t, &campaign.RecurseLoop{MaxWaves: 5})

	updated, _ := m.handleCampaignCommand("/campaign pause", []string{"/campaign", "pause"})
	result := updated.(Model)
	if result.recurse == nil || !result.recurse.held {
		t.Fatal("pause must hold the sweep")
	}
	last := result.history[len(result.history)-1]
	if !strings.Contains(last.Content, "holds between waves") {
		t.Fatalf("pause must say the sweep holds: %q", last.Content)
	}
}

func TestCampaign_ResumeHeldRecurseChains(t *testing.T) {
	m := recurseTestModel(t, &campaign.RecurseLoop{MaxWaves: 5})
	m.recurse.held = true
	m.recurse.lastWave = recurseTestFinished()
	m.activeCampaign = nil // boundary cleanup cleared the run

	updated, _ := m.handleCampaignCommand("/campaign resume", []string{"/campaign", "resume"})
	result := updated.(Model)
	// No kernel in the test model, so the next wave cannot start — but the
	// resume must have ATTEMPTED the chain (and cleared the hold), not the
	// ordinary "no paused campaign" path.
	if result.recurse != nil {
		t.Fatal("failed chain must clear the sweep")
	}
	joined := ""
	for _, msg := range result.history {
		joined += msg.Content + "\n"
	}
	if !strings.Contains(joined, "Could not start the next wave") {
		t.Fatalf("resume must attempt the chain: %q", joined)
	}
}

type recurseTestError struct{}

func (recurseTestError) Error() string { return "boom" }

var errRecurseTest = recurseTestError{}
