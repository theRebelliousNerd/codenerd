package system

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/tools"
)

// The config's usage section reaches the tracker the boot meters with. Price
// overrides (usage.RegisterPrice) and the event log (usage.WithEventLog) had
// no caller, so a model the built-in table lacks was always "unpriced" and the
// event ring could not be switched on.
func TestInitCoreComponents_ShouldMeterWithTheConfigsUsageSection(t *testing.T) {
	prevEnv, hadEnv := os.LookupEnv("CODENERD_WORKSPACE_ROOT")
	prevRoot := tools.Global().WorkspaceRoot()
	t.Cleanup(func() {
		if hadEnv {
			_ = os.Setenv("CODENERD_WORKSPACE_ROOT", prevEnv)
		} else {
			_ = os.Unsetenv("CODENERD_WORKSPACE_ROOT")
		}
		tools.SetGlobalWorkspaceRoot(prevRoot)
	})

	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"usage":{"prices":{"usage-boot-probe":{"input_per_mtok":2,"output_per_mtok":4}},"event_log":true}}`
	if err := os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	bctx := &bootContext{cfg: BootConfig{Workspace: ws}}
	if err := initCoreComponents(bctx); err != nil {
		t.Fatalf("initCoreComponents: %v", err)
	}
	if bctx.tracker == nil {
		t.Fatal("boot acquired no usage tracker")
	}
	t.Cleanup(func() { _ = bctx.tracker.Close() })

	bctx.tracker.Track(context.Background(), "usage-boot-probe-7b", "acme", 1_000_000, 500_000, "chat")

	stats := bctx.tracker.Stats()
	if stats.UnpricedTokens != 0 {
		t.Errorf("the configured model was metered as unpriced (%d tokens)", stats.UnpricedTokens)
	}
	if got, want := stats.TotalProject.Cost, 4.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("cost = %g, want %g from usage.prices (1M in at $2, 0.5M out at $4)", got, want)
	}
	events := bctx.tracker.Events()
	if len(events) != 1 || events[0].Model != "usage-boot-probe-7b" {
		t.Fatalf("usage.event_log is on but the ring holds %+v", events)
	}
}
