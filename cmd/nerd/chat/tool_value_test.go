package chat

import (
	"strings"
	"testing"

	"codenerd/internal/approval"
	"codenerd/internal/autopoiesis"
	"codenerd/internal/core"
	"time"
)

func TestToolValue_WithoutAutopoiesisSaysNoLedgerIsKept(t *testing.T) {
	if got := (Model{}).renderToolValue(); !strings.Contains(got, "no tool ledger") {
		t.Errorf("got %q", got)
	}
}

func TestToolValue_RendersTheKernelsClassification(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	orch := autopoiesis.NewOrchestrator(nil, autopoiesis.DefaultConfig(t.TempDir()))
	orch.SetKernel(kernel)
	got := (Model{autopoiesis: orch}).renderToolValue()
	if !strings.Contains(got, "## Tool value") || !strings.Contains(got, "unknown") {
		t.Errorf("report:\n%s", got)
	}
}

// Nothing is withdrawn without a derived proposal: an unknown name, or a
// built-in, is refused before anybody is asked.
func TestRetireTool_RefusesWithoutAProposal(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	orch := autopoiesis.NewOrchestrator(nil, autopoiesis.DefaultConfig(t.TempDir()))
	orch.SetKernel(kernel)
	broker, err := approval.NewBroker(approval.Options{TTL: time.Minute, PreviewMaxBytes: 4096, JournalRetention: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = broker.Close() })
	broker.SetAutonomous(true)
	m := Model{autopoiesis: orch, approvals: broker}
	for _, name := range []string{"no_such_tool", "bash"} {
		msg := m.retireTool(name)()
		if _, failed := msg.(errorMsg); !failed {
			t.Errorf("retire %q returned %T, want a refusal", name, msg)
		}
	}
}
