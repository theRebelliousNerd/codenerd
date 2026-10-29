package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The `ui` and `retrieval` sections reach a loaded config and default when
// absent. UIConfig existed with no UserConfig field, so no config.json could
// set it.
func TestSections_UIAndRetrievalLoadFromConfigJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"ui":{"split_pane_ratio":0.5},"retrieval":{"brief_min_relevance":80}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	if got := cfg.GetUIConfig().SplitPaneRatio; got != 0.5 {
		t.Errorf("ui.split_pane_ratio = %g, want 0.5", got)
	}
	if got := cfg.GetRetrievalConfig().BriefMinRelevance; got != 80 {
		t.Errorf("retrieval.brief_min_relevance = %d, want 80", got)
	}
	params := cfg.GetRetrievalConfig().Params()
	if len(params) != 1 || params[0].Key != "/retrieval_brief_min_relevance" || params[0].Value != 80 {
		t.Errorf("retrieval params = %+v", params)
	}

	var empty *UserConfig
	if got := empty.GetUIConfig().SplitPaneRatio; got != DefaultUIConfig().SplitPaneRatio {
		t.Errorf("absent ui section: ratio %g, want the default", got)
	}
	if got := empty.GetRetrievalConfig().BriefMinRelevance; got != DefaultRetrievalConfig().BriefMinRelevance {
		t.Errorf("absent retrieval section: floor %d, want the default", got)
	}
}

func TestSections_UIAndRetrievalCheckTheirRanges(t *testing.T) {
	if p := (UIConfig{SplitPaneRatio: 1.5}).Check("ui"); len(p) != 1 || !strings.Contains(p[0].Path, "split_pane_ratio") {
		t.Errorf("a ratio of 1.5 was not refused: %+v", p)
	}
	if p := (RetrievalConfig{BriefMinRelevance: -1}).Check("retrieval"); len(p) != 1 {
		t.Errorf("a negative floor was not refused: %+v", p)
	}
	if p := DefaultUserConfig().Check(nil); hasError(p) {
		t.Errorf("the default config fails its own check: %+v", p)
	}
}

func hasError(problems []Problem) bool {
	for _, p := range problems {
		if p.Severity == SeverityError {
			return true
		}
	}
	return false
}
