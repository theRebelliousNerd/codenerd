package main

import (
	"testing"
	"time"

	"codenerd/internal/campaign"
)

func resumeSelectFixture(id string, status campaign.CampaignStatus, age time.Duration, blockReason string) resumeCandidate {
	return resumeCandidate{
		Campaign: &campaign.Campaign{
			ID:          id,
			Title:       id,
			Status:      status,
			BlockReason: blockReason,
			UpdatedAt:   time.Now().Add(age),
		},
		Path: "/tmp/" + id + ".json",
	}
}

func assertSelectID(t *testing.T, cands []resumeCandidate, retry bool, want string) {
	t.Helper()
	got := selectResumeCampaign(cands, retry)
	if want == "" && got != nil {
		t.Fatalf("want nil, got %s", got.Campaign.ID)
	}
	if want != "" && got == nil {
		t.Fatalf("want %s, got nil", want)
	}
	if want != "" && got.Campaign.ID != want {
		t.Errorf("got %s, want %s", got.Campaign.ID, want)
	}
}

func TestSelectResumePreference(t *testing.T) {
	cands := []resumeCandidate{
		resumeSelectFixture("failed", campaign.StatusFailed, -3*time.Hour, "/all_tasks_blocked"),
		resumeSelectFixture("active", campaign.StatusActive, -2*time.Hour, ""),
		resumeSelectFixture("paused", campaign.StatusPaused, -1*time.Hour, ""),
	}
	assertSelectID(t, cands, false, "paused")
}

func TestSelectResumeActiveBeatsBlocked(t *testing.T) {
	cands := []resumeCandidate{
		resumeSelectFixture("blocked", campaign.StatusFailed, -1*time.Hour, "/all_tasks_blocked"),
		resumeSelectFixture("active", campaign.StatusActive, -2*time.Hour, ""),
	}
	assertSelectID(t, cands, false, "active")
}

func TestSelectResumeNewestWins(t *testing.T) {
	oldPaused := []resumeCandidate{
		resumeSelectFixture("p-old", campaign.StatusPaused, -5*time.Hour, ""),
		resumeSelectFixture("p-new", campaign.StatusPaused, -1*time.Hour, ""),
	}
	assertSelectID(t, oldPaused, false, "p-new")
	oldBlocked := []resumeCandidate{
		resumeSelectFixture("b-old", campaign.StatusFailed, -5*time.Hour, "/all_tasks_blocked"),
		resumeSelectFixture("b-new", campaign.StatusFailed, -1*time.Hour, "/other"),
	}
	assertSelectID(t, oldBlocked, false, "b-new")
}

func TestSelectResumeRetryFailed(t *testing.T) {
	plain := []resumeCandidate{
		resumeSelectFixture("plain", campaign.StatusFailed, -1*time.Hour, ""),
	}
	assertSelectID(t, plain, false, "")
	assertSelectID(t, plain, true, "plain")
}

func TestSelectResumeEmpty(t *testing.T) {
	assertSelectID(t, nil, false, "")
	assertSelectID(t, nil, true, "")
	done := []resumeCandidate{
		resumeSelectFixture("done", campaign.StatusCompleted, -1*time.Hour, ""),
	}
	assertSelectID(t, done, true, "")
}

func TestRunCampaignResumeCallsPrepareResume(t *testing.T) {
	if !resumeCallSet(t)["PrepareResume"] {
		t.Errorf("runCampaignResume must call PrepareResume")
	}
}
