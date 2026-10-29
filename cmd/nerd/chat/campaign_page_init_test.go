package chat

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/campaign"
)

// The chat built its campaign page as a zero value, so the page rendered with
// no progress bar (the "Overall Progress" line was blank) and none of its
// styles. Until 2026-09-25 InitChat constructed every other page and not this
// one.
func TestInitChat_CampaignPageShowsProgress(t *testing.T) {
	m := InitChat(Config{})
	page := m.campaignPage
	page.SetSize(80, 20)

	camp := &campaign.Campaign{ID: "/c", Title: "a campaign"}
	for i := range 4 {
		camp.Phases = append(camp.Phases, campaign.Phase{ID: fmt.Sprintf("/p%d", i), Name: fmt.Sprintf("phase %d", i)})
	}
	page.UpdateContent(&campaign.Progress{TotalPhases: len(camp.Phases)}, camp)

	if view := page.View(); !strings.Contains(view, "0%") {
		t.Fatalf("the chat's campaign page renders no progress bar:\n%s", view)
	}
}
