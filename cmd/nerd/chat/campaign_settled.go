package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"codenerd/internal/campaign"
	"codenerd/internal/logging"
)

// handleCampaignSettled applies a run's terminal receipt. It acts only when
// the receipt's run is still the current one: a run already retired -- by a
// completed progress snapshot, a hard risk block, or a replacement campaign --
// is left alone, so the receipt is idempotent and never touches another run.
// The outcome is the one Run settled on: completed renders the completion
// transcript; failed, paused or an error says so; nothing is reported as a
// success because a channel closed or Run returned.
func (m Model) handleCampaignSettled(msg campaignSettledMsg) Model {
	if msg.orch == nil || msg.orch != m.campaignOrch {
		logging.Kernel("[campaign] settled receipt for a run no longer current (status %s); ignored", msg.status)
		return m
	}
	active := m.activeCampaign
	if active == nil {
		active = msg.campaign
	}
	m = m.retireCampaign()
	switch {
	case msg.status == campaign.StatusCompleted && msg.err == nil:
		return m.pushAssistantMsg(m.renderCampaignCompleted(active))
	case msg.err != nil && !errors.Is(msg.err, context.Canceled):
		if report, ok := campaign.FormatRiskBlock(msg.err); ok {
			return m.pushAssistantMsg("## Campaign Blocked by Risk Gate\n\n```\n" + report + "```")
		}
		return m.pushAssistantMsg(fmt.Sprintf("## Campaign Ended (%s)\n\n%v", statusWord(msg.status), msg.err))
	default:
		return m.pushAssistantMsg(fmt.Sprintf("## Campaign Ended (%s)\n\nThe run settled without completing; `/campaign resume` continues a paused campaign.", statusWord(msg.status)))
	}
}

// retireCampaign stands the campaign UI down: the run's handle, its channels
// and its pane.
func (m Model) retireCampaign() Model {
	m.isLoading = false
	m.activeCampaign = nil
	m.campaignOrch = nil
	m.campaignProgress = nil
	m.campaignProgressChan = nil
	m.campaignEventChan = nil
	m.showCampaignPanel = false
	return m.applyLayout()
}

func statusWord(s campaign.CampaignStatus) string {
	if w := strings.TrimPrefix(string(s), "/"); w != "" {
		return w
	}
	return "unknown"
}
