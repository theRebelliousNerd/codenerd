package system

import (
	"os"
	"testing"

	"codenerd/internal/config"
)

// The boot opens one broker per workspace journal, the Cortex owns it, and
// closing the Cortex releases it.
func TestInitApprovalBroker_OpensTheWorkspaceJournal(t *testing.T) {
	ws := t.TempDir()
	bctx := &bootContext{workspace: ws, appCfg: config.DefaultUserConfig()}
	if err := initApprovalBroker(bctx); err != nil {
		t.Fatal(err)
	}
	if bctx.approvals == nil || bctx.approvalsRelease == nil {
		t.Fatal("no broker was opened")
	}
	if _, err := os.Stat(approvalJournalPath(ws)); err != nil {
		t.Fatalf("journal not created: %v", err)
	}
	if bctx.approvals.HostAttached() {
		t.Fatal("a fresh boot has a host attached; only the interactive UI attaches one")
	}
	c := &Cortex{Approvals: bctx.approvals, approvalsRelease: bctx.approvalsRelease}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := bctx.approvals.AttachHost("tui:after-close", "op"); err == nil {
		t.Fatal("the broker outlived the Cortex that owned it")
	}
}

func TestInitApprovalBroker_RefusesAnInvalidSection(t *testing.T) {
	cfg := config.DefaultUserConfig()
	cfg.Approval = &config.ApprovalConfig{RequestTTL: "never"}
	if err := initApprovalBroker(&bootContext{workspace: t.TempDir(), appCfg: cfg}); err == nil {
		t.Fatal("an invalid approval section booted")
	}
}
