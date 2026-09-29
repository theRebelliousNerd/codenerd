package config

import (
	"strings"
	"testing"
)

// The defaults are the spans the policy carried as literals until 2026-09-23,
// and they check clean. The ledger ceiling is the working-memory share of the
// default context window, not the flat 65536-byte figure that compacted a
// 16-read working set on 2026-09-29 (session 20260929_052520).
func TestWorkingConfig_TheDefaultsCheckClean(t *testing.T) {
	def := DefaultWorkingConfig()
	if problems := def.Check("working"); len(problems) != 0 {
		t.Fatalf("the defaults do not check clean: %v", problems)
	}
	window := DefaultContextWindowConfig()
	want := window.MaxTokens * window.WorkingReservePercent / 100 * 4
	if def.LedgerCeilingBytes != want || want != 400000 {
		t.Fatalf("ledger_ceiling_bytes = %d, want %d (context_window.max_tokens %d * working_reserve_percent %d / 100 tokens, times 4 bytes)", def.LedgerCeilingBytes, want, window.MaxTokens, window.WorkingReservePercent)
	}
	if got := LedgerCeilingBytesFromContext(window); got != want {
		t.Fatalf("LedgerCeilingBytesFromContext = %d, want %d", got, want)
	}
	var nilCfg *UserConfig
	if got := nilCfg.GetWorkingConfig().LedgerCeilingBytes; got != want {
		t.Fatalf("a nil config's ceiling = %d, want %d", got, want)
	}
}

// An absent ledger_ceiling_bytes follows this file's context window. An
// explicit value wins, including over a window that would derive something else.
func TestWorkingConfig_AnAbsentCeilingFollowsTheContextWindow(t *testing.T) {
	load := func(body string) WorkingConfig {
		t.Helper()
		cfg, err := LoadUserConfig(writeCampaignConfig(t, body))
		if err != nil {
			t.Fatalf("load %s: %v", body, err)
		}
		return cfg.GetWorkingConfig()
	}

	// 100000 tokens * 25% = 25000 tokens, times 4 bytes = 100000 bytes.
	custom := load(`{"context_window": {"max_tokens": 100000, "working_reserve_percent": 25}}`)
	if custom.LedgerCeilingBytes != 100000 {
		t.Fatalf("no working section, custom window: ceiling = %d, want 100000", custom.LedgerCeilingBytes)
	}

	absent := load(`{}`)
	if absent.LedgerCeilingBytes != 400000 {
		t.Fatalf("no working section and no context_window: ceiling = %d, want the default window's 400000", absent.LedgerCeilingBytes)
	}

	explicit := load(`{"context_window": {"max_tokens": 100000, "working_reserve_percent": 25}, "working": {"ledger_ceiling_bytes": 32768}}`)
	if explicit.LedgerCeilingBytes != 32768 {
		t.Fatalf("an explicit ledger_ceiling_bytes must win over the window: got %d", explicit.LedgerCeilingBytes)
	}

	keepOnly := load(`{"context_window": {"max_tokens": 100000, "working_reserve_percent": 25}, "working": {"ledger_keep_rounds": 4}}`)
	if keepOnly.LedgerCeilingBytes != 100000 || keepOnly.LedgerKeepRounds != 4 {
		t.Fatalf("a working section that omits the ceiling derives it and keeps the key it set: %+v", keepOnly)
	}
}

// A key the user writes reaches the policy; every key they leave out is the
// default.
func TestWorkingConfig_AKeyInTheFileReachesThePolicy(t *testing.T) {
	path := writeCampaignConfig(t, `{"working": {"nudge_rounds": 5, "ledger_ceiling_bytes": 32768}}`)
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	spans := cfg.GetWorkingConfig()
	if spans.NudgeRounds != 5 {
		t.Errorf("nudge_rounds = %d, want the file's 5", spans.NudgeRounds)
	}
	if spans.LedgerCeilingBytes != 32768 {
		t.Errorf("ledger_ceiling_bytes = %d, want the file's 32768", spans.LedgerCeilingBytes)
	}
	def := DefaultWorkingConfig()
	if spans.CommitRounds != def.CommitRounds || spans.RepeatThreshold != def.RepeatThreshold || spans.LedgerKeepRounds != def.LedgerKeepRounds {
		t.Errorf("absent keys did not take the defaults: %+v", spans)
	}
	params := map[string]int64{}
	for _, p := range spans.Params() {
		params[p.Key] = p.Value
	}
	if params["/working_nudge_rounds"] != 5 || params["/working_ledger_ceiling"] != 32768 {
		t.Errorf("params = %v, want the file's values under the policy's keys", params)
	}
}

func TestWorkingConfig_AnUnknownKeyIsRefused(t *testing.T) {
	path := writeCampaignConfig(t, `{"working": {"nudge_round": 5}}`)
	if _, err := LoadUserConfig(path); err == nil {
		t.Fatal("working.nudge_round (a typo) loaded; the strict decoder must refuse it")
	}
}

// A repeat threshold below 2 is not a repeat detector (every turn would stop
// at its first round), and a stall span below the commit span stops a change
// task before its reading closes. The loader refuses both.
func TestWorkingConfig_CheckNamesEachContradiction(t *testing.T) {
	bad := WorkingConfig{RepeatThreshold: 1, CommitRounds: 20, StallRounds: 10, LedgerCeilingBytes: 100}
	var paths []string
	for _, p := range bad.Check("working") {
		if p.Severity != SeverityError {
			t.Errorf("%s is %s, want an error", p.Path, p.Severity)
		}
		paths = append(paths, p.Path)
	}
	joined := strings.Join(paths, " ")
	for _, want := range []string{"working.repeat_threshold", "working.stall_rounds", "working.ledger_ceiling_bytes"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Check did not name %s; it found %v", want, paths)
		}
	}
	path := writeCampaignConfig(t, `{"working": {"repeat_threshold": 1}}`)
	if _, err := LoadUserConfig(path); err == nil {
		t.Fatal("a working section with repeat_threshold 1 loaded")
	}
}
