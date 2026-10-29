package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/campaign"
)

func resumeSelectFixture(id string, status campaign.CampaignStatus, age time.Duration, blockReason string) *campaign.Campaign {
	return &campaign.Campaign{
		ID:          id,
		Title:       id,
		Status:      status,
		BlockReason: blockReason,
		UpdatedAt:   time.Now().Add(age),
	}
}

func assertSelectID(t *testing.T, cands []*campaign.Campaign, retry bool, filter, want string) {
	t.Helper()
	got := selectResumeCampaign(cands, retry, filter)
	if want == "" && got != nil {
		t.Fatalf("want nil, got %s", got.ID)
	}
	if want != "" && got == nil {
		t.Fatalf("want %s, got nil", want)
	}
	if want != "" && got.ID != want {
		t.Errorf("got %s, want %s", got.ID, want)
	}
}

func TestSelectResumePreference(t *testing.T) {
	cands := []*campaign.Campaign{
		resumeSelectFixture("failed", campaign.StatusFailed, -3*time.Hour, "/all_tasks_blocked"),
		resumeSelectFixture("active", campaign.StatusActive, -2*time.Hour, ""),
		resumeSelectFixture("paused", campaign.StatusPaused, -1*time.Hour, ""),
	}
	assertSelectID(t, cands, false, "", "paused")
}

func TestSelectResumeActiveBeatsBlocked(t *testing.T) {
	cands := []*campaign.Campaign{
		resumeSelectFixture("blocked", campaign.StatusFailed, -1*time.Hour, "/all_tasks_blocked"),
		resumeSelectFixture("active", campaign.StatusActive, -2*time.Hour, ""),
	}
	assertSelectID(t, cands, false, "", "active")
}

func TestSelectResumeNewestWins(t *testing.T) {
	oldPaused := []*campaign.Campaign{
		resumeSelectFixture("p-old", campaign.StatusPaused, -5*time.Hour, ""),
		resumeSelectFixture("p-new", campaign.StatusPaused, -1*time.Hour, ""),
	}
	assertSelectID(t, oldPaused, false, "", "p-new")
	oldBlocked := []*campaign.Campaign{
		resumeSelectFixture("b-old", campaign.StatusFailed, -5*time.Hour, "/all_tasks_blocked"),
		resumeSelectFixture("b-new", campaign.StatusFailed, -1*time.Hour, "/other"),
	}
	assertSelectID(t, oldBlocked, false, "", "b-new")
}

func TestSelectResumeRetryFailed(t *testing.T) {
	plain := []*campaign.Campaign{
		resumeSelectFixture("plain", campaign.StatusFailed, -1*time.Hour, ""),
	}
	assertSelectID(t, plain, false, "", "")
	assertSelectID(t, plain, true, "", "plain")
}

func TestSelectResumeIDFilterBeatsPreference(t *testing.T) {
	cands := []*campaign.Campaign{
		resumeSelectFixture("campaign_aaa", campaign.StatusPaused, -1*time.Hour, ""),
		resumeSelectFixture("campaign_bbb", campaign.StatusFailed, -3*time.Hour, "/all_tasks_blocked"),
	}
	assertSelectID(t, cands, false, "bbb", "campaign_bbb")
	assertSelectID(t, cands, false, "BBB", "campaign_bbb")
	assertSelectID(t, cands, false, "zzz", "")
}

func TestSelectResumeEmpty(t *testing.T) {
	assertSelectID(t, nil, false, "", "")
	assertSelectID(t, nil, true, "", "")
	done := []*campaign.Campaign{
		resumeSelectFixture("done", campaign.StatusCompleted, -1*time.Hour, ""),
	}
	assertSelectID(t, done, true, "", "")
}

func TestRunCampaignResumeCallsPrepareResume(t *testing.T) {
	if !resumeCallSet(t)["PrepareResume"] {
		t.Errorf("runCampaignResume must call PrepareResume")
	}
}

// The CLI is not a second snapshot writer: it admits through the one door and
// never sets or writes a campaign itself. Its early status write once left a
// snapshot the journal never recorded whenever Cortex boot then failed.
func TestRunCampaignResumeAdmitsThroughTheSingleDoor(t *testing.T) {
	got := resumeCallSet(t)
	if !got["ResumeCampaign"] {
		t.Error("runCampaignResume must admit through ResumeCampaign")
	}
	for _, banned := range []string{"SetCampaign", "WriteFile", "Rename", "writeCampaignJSON"} {
		if got[banned] {
			t.Errorf("runCampaignResume must not call %s: only the owning orchestrator publishes", banned)
		}
	}
}

// A death inside the snapshot-replacement fallback leaves a journal and the
// moved-aside/temp snapshot but no <id>.json. Discovery must still name the
// campaign so admission can classify and restore it.
func TestLoadResumeCandidates_FindsCampaignWhoseSnapshotIsMidReplacement(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, c *campaign.Campaign) {
		t.Helper()
		data, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("campaign_whole.json", resumeSelectFixture("/campaign_whole", campaign.StatusPaused, -time.Hour, ""))
	write("campaign_torn.json.prev-1", resumeSelectFixture("/campaign_torn", campaign.StatusActive, 0, ""))
	if err := os.WriteFile(filepath.Join(dir, "campaign_torn.journal.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A leftover with no journal proves nothing and is not offered.
	write("campaign_stray.json.tmp-1", resumeSelectFixture("/campaign_stray", campaign.StatusActive, 0, ""))

	got, err := loadResumeCandidates(dir)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, c := range got {
		ids[c.ID] = true
	}
	if len(got) != 2 || !ids["/campaign_whole"] || !ids["/campaign_torn"] {
		t.Fatalf("candidates = %v, want whole and torn only", ids)
	}
}
