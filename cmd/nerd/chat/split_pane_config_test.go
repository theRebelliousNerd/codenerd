package chat

import (
	"testing"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/config"
)

// The chat builds its split pane at ui.split_pane_ratio. Until 2026-09-25 it
// called NewSplitPaneView, which hard-codes the default, and the config
// section that names the ratio had no field on UserConfig.
func TestNewSplitPane_UsesTheConfiguredRatio(t *testing.T) {
	cfg := config.DefaultUserConfig()
	cfg.UI = &config.UIConfig{SplitPaneRatio: 0.5}
	if got := newSplitPane(ui.DefaultStyles(), cfg).SplitRatio; got != 0.5 {
		t.Fatalf("split ratio %g, want the configured 0.5", got)
	}
	if got := newSplitPane(ui.DefaultStyles(), nil).SplitRatio; got != config.DefaultUIConfig().SplitPaneRatio {
		t.Fatalf("split ratio %g with no config, want the default", got)
	}
}
