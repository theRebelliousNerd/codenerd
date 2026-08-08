package campaign

import (
	"testing"

	"codenerd/internal/northstar"
)

// The defect this guards: OrchestratorConfig.NorthstarObserver was not consumed
// by NewOrchestrator. wireIntelligenceComponents read
// `o.configuredNorthstarObserver = o.northstarObserver` -- a field nothing ever
// populated -- so it stayed nil forever. risk_scoring.go refuses to start any
// campaign whose targets touch a protected root when it is nil, so every
// campaign against internal/core (and every other protected surface) was
// permanently blocked with "northstar observer not configured".
//
// The setter SetNorthstarObserver existed and had zero callers repo-wide. That
// is the shape of bug this test exists to catch: a collaborator that is
// settable, required, and never set.
func TestWireIntelligenceComponents_ConsumesConfiguredNorthstarObserver(t *testing.T) {
	observer := northstar.NewCampaignObserver(nil)

	o := &Orchestrator{}
	wireIntelligenceComponents(o, OrchestratorConfig{NorthstarObserver: observer})

	if o.configuredNorthstarObserver == nil {
		t.Fatal("configuredNorthstarObserver is nil after wiring; the protected-surface risk gate will refuse every campaign and report only 'northstar observer not configured'")
	}
	if o.configuredNorthstarObserver != observer {
		t.Error("configuredNorthstarObserver is not the observer supplied in config")
	}
}

// Absent config must leave it nil so the gate stays CLOSED. Defaulting to a
// non-nil inert observer would satisfy the gate while guarding nothing, which
// is worse than refusing to start.
func TestWireIntelligenceComponents_NilObserverKeepsGateClosed(t *testing.T) {
	o := &Orchestrator{}
	wireIntelligenceComponents(o, OrchestratorConfig{})

	if o.configuredNorthstarObserver != nil {
		t.Error("a config with no observer produced a non-nil observer; the risk gate would pass while nothing is guarding")
	}
}

// SetNorthstarObserver is the other way in. It must reach the same field the
// risk gate reads, or wiring through it silently does nothing.
func TestSetNorthstarObserver_ReachesTheFieldTheRiskGateReads(t *testing.T) {
	observer := northstar.NewCampaignObserver(nil)

	o := &Orchestrator{}
	o.SetNorthstarObserver(observer)

	if o.configuredNorthstarObserver != observer {
		t.Error("SetNorthstarObserver did not set configuredNorthstarObserver, which is the field risk_scoring.go checks")
	}
}
