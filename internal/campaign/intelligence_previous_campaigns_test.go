package campaign

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Previous campaigns come from the workspace's campaign records: the ones that
// ran to an end, completed or failed, newest first. They used to come from a
// campaign_completed kernel fact nothing produced, so planning never saw one.
func TestGatherPreviousCampaigns_ReadsTheWorkspacesEndedCampaigns(t *testing.T) {
	ws := t.TempDir()
	dir := filepath.Join(ws, ".nerd", "campaigns")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for _, c := range []Campaign{
		{ID: "/campaign_old", Goal: "old goal", Status: StatusCompleted, TotalTasks: 4, CompletedTasks: 4, UpdatedAt: now.Add(-2 * time.Hour)},
		{ID: "/campaign_new", Goal: "new goal", Status: StatusFailed, TotalTasks: 4, CompletedTasks: 1, UpdatedAt: now.Add(-time.Hour)},
		{ID: "/campaign_live", Goal: "still running", Status: StatusActive, TotalTasks: 2, UpdatedAt: now},
	} {
		data, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, c.ID[1:]+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.json"), []byte(`{"not":"a campaign"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	g := NewIntelligenceGatherer(ws, nil, nil, nil, nil, nil, nil, nil, nil)
	report := &IntelligenceReport{}
	g.gatherPreviousCampaigns(context.Background(), report, "goal", func(msg string) { t.Errorf("gather error: %s", msg) })

	got := report.PreviousCampaigns
	if len(got) != 2 || got[0].CampaignID != "/campaign_new" || got[1].CampaignID != "/campaign_old" {
		t.Fatalf("previous campaigns = %+v, want the failed then the completed one, newest first", got)
	}
	if got[0].SuccessRate != 0.25 || got[1].SuccessRate != 1 || got[0].TaskCount != 4 || got[0].Goal != "new goal" {
		t.Fatalf("previous campaign fields = %+v", got)
	}
}
