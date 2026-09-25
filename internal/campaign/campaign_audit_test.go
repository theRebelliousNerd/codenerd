package campaign

import (
	"testing"

	"codenerd/internal/logging"
	"codenerd/internal/types/typestest"
)

// A campaign's lifecycle reaches the audit trail through the one function
// every status change passes through. The trail recorded no campaign event at
// all before, so a forensic read of a run could not say when a campaign began
// or how it ended. Not parallel: it binds the process's logging.
func TestUpdateCampaignStatus_ShouldRecordTheLifecycleInTheAuditTrail(t *testing.T) {
	ws := t.TempDir()
	logging.ApplyConfig(logging.Config{DebugMode: true, Level: "debug"})
	t.Cleanup(func() {
		logging.CloseAll()
		logging.ClearInjectedConfig()
	})
	if err := logging.Initialize(ws); err != nil {
		t.Fatalf("logging.Initialize: %v", err)
	}

	o := &Orchestrator{
		kernel:   typestest.NewMockKernel(),
		campaign: &Campaign{ID: "/campaign_audit_probe", Title: "probe", Type: CampaignTypeCustom},
	}
	o.updateCampaignStatus(StatusPlanning) // not a lifecycle end: not recorded
	o.updateCampaignStatus(StatusActive)
	o.updateCampaignStatus(StatusFailed)
	logging.CloseAudit()

	path, err := logging.LatestAuditLogPath()
	if err != nil {
		t.Fatalf("LatestAuditLogPath: %v", err)
	}
	events, err := logging.ReadRecentAuditEvents(path, []logging.AuditEventType{
		logging.AuditCampaignStart, logging.AuditCampaignComplete, logging.AuditCampaignAbort, logging.AuditCampaignPhase,
	}, 10)
	if err != nil {
		t.Fatalf("ReadRecentAuditEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d campaign events, want start and abort: %+v", len(events), events)
	}
	var sawStart, sawAbort bool
	for _, e := range events {
		if e.SessionID != "/campaign_audit_probe" {
			t.Errorf("event not keyed by the campaign: %+v", e)
		}
		switch e.EventType {
		case logging.AuditCampaignStart:
			sawStart = e.Success
		case logging.AuditCampaignAbort:
			sawAbort = !e.Success
		}
	}
	if !sawStart || !sawAbort {
		t.Fatalf("want a successful start and an unsuccessful abort: %+v", events)
	}
}
