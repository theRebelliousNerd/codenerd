package campaign

import (
	"context"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/tactile"
)

// The assault executor is a registered direct bypass: it runs campaign
// commands outside VirtualStore. Its commands used to leave nothing in the
// kernel; through auditedExecutor each one lands as execution facts.
func TestAssaultExecutorAuditsIntoTheCampaignKernel(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	o := &Orchestrator{kernel: kernel, workspace: t.TempDir()}

	exec := o.auditedExecutor(newAssaultExecutor(o.workspace, 1<<20, 0))
	result, err := exec.Execute(context.Background(), tactile.Command{
		Binary:    "go",
		Arguments: []string{"version"},
		RequestID: "assault-audit-probe",
	})
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("go version through the assault executor: result=%+v err=%v", result, err)
	}

	for _, predicate := range []string{"execution_started", "execution_completed"} {
		facts, err := kernel.Query(predicate)
		if err != nil {
			t.Fatalf("query %s: %v", predicate, err)
		}
		if len(facts) == 0 {
			t.Fatalf("assault command left no %s fact in the campaign kernel", predicate)
		}
	}
}
