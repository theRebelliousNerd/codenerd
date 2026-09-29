package config

import (
	"strings"
	"testing"
)

// The defaults are the classifier's previous window, moved out of Go
// constants: five turns, and 5 × (2000 content + 800 reasoning summary).
func TestClassificationConfig_Defaults(t *testing.T) {
	if problems := DefaultClassificationConfig().Check("classification"); len(problems) != 0 {
		t.Fatalf("the defaults do not check clean: %v", problems)
	}
	got := DefaultClassificationConfig().Resolve()
	if got.TurnWindow != 5 || got.CharBudget != 14000 {
		t.Fatalf("defaults = %+v, want window 5 and budget 14000", got)
	}
	var empty *UserConfig
	if got := empty.GetClassificationConfig().Resolve(); got.TurnWindow != 5 || got.CharBudget != 14000 {
		t.Fatalf("nil config resolved %+v, want the defaults", got)
	}
}

// 0 turns is a value (send none). A 0 char budget is the absent key.
func TestClassificationConfig_ZeroWindowIsNoneAndZeroBudgetIsTheDefault(t *testing.T) {
	path := writeCampaignConfig(t, `{"classification":{"history_turn_window":0,"history_char_budget":0}}`)
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got := cfg.GetClassificationConfig().Resolve()
	if got.TurnWindow != 0 {
		t.Errorf("history_turn_window = %d, want the file's 0", got.TurnWindow)
	}
	if got.CharBudget != DefaultClassificationConfig().HistoryCharBudget {
		t.Errorf("history_char_budget = %d, want the default (0 is the absent key)", got.CharBudget)
	}
}

func TestClassificationConfig_CheckNamesEachContradiction(t *testing.T) {
	window := -3
	problems := (ClassificationConfig{HistoryTurnWindow: &window, HistoryCharBudget: -1}).Check("classification")
	joined := ""
	for _, p := range problems {
		if p.Severity != SeverityError {
			t.Errorf("%s is %s, want an error", p.Path, p.Severity)
		}
		joined += p.Path + " "
	}
	for _, want := range []string{"classification.history_turn_window", "classification.history_char_budget"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Check did not name %s; it found %v", want, problems)
		}
	}
}

func TestClassificationConfig_AnUnknownKeyIsRefused(t *testing.T) {
	path := writeCampaignConfig(t, `{"classification":{"history_turns":5}}`)
	if _, err := LoadUserConfig(path); err == nil {
		t.Fatal("classification.history_turns (not a field) loaded; the strict decoder must refuse it")
	}
}

// A set section reaches the installed policy through a real load. A refused
// file must not replace it. A missing file installs the defaults.
func TestLoadUserConfig_InstallsClassificationHistory(t *testing.T) {
	t.Cleanup(func() { SetClassificationHistory(DefaultClassificationConfig().Resolve()) })

	custom := ClassificationHistory{TurnWindow: 2, CharBudget: 500}
	SetClassificationHistory(custom)

	bad := writeCampaignConfig(t, `{"classification":{"history_char_budget":-5}}`)
	_, err := LoadUserConfig(bad)
	if err == nil || !strings.Contains(err.Error(), "classification.history_char_budget") {
		t.Fatalf("LoadUserConfig error = %v, want classification.history_char_budget named", err)
	}
	if got := ResolvedClassificationHistory(); got != custom {
		t.Fatalf("a refused file installed %+v, want the previous policy %+v", got, custom)
	}

	path := writeCampaignConfig(t, `{"classification":{"history_turn_window":2,"history_char_budget":500}}`)
	if _, err := LoadUserConfig(path); err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	if got := ResolvedClassificationHistory(); got != custom {
		t.Fatalf("installed = %+v, want %+v", got, custom)
	}

	// The loader's not-exist branch installs the defaults, so a missing file
	// does not leave a previous run's policy in place.
	if _, err := LoadUserConfig(path + ".absent"); err != nil {
		t.Fatalf("LoadUserConfig (missing): %v", err)
	}
	if got := ResolvedClassificationHistory(); got != DefaultClassificationConfig().Resolve() {
		t.Fatalf("missing file installed %+v, want defaults", got)
	}
}
