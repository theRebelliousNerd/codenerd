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

// The holographic shares reach the policy under their config_param keys.
// Callers default to 2, the same number the caller block decided with before
// the dimension was generalized. A share past 100 over-books the render. The
// outline signature floor is a rune count: below 1 is refused, and it has no
// percent cap. The old singular holographic_caller_share_percent key is not
// a config key anymore.
func TestWorkingConfig_CallerShareReachesThePolicy(t *testing.T) {
	def := DefaultWorkingConfig()
	if def.HolographicCallersSharePercent != 2 || def.HolographicSignaturesSharePercent != 2 ||
		def.HolographicTypesSharePercent != 1 || def.HolographicImportersSharePercent != 1 ||
		def.HolographicOutlineSharePercent != 2 || def.HolographicOutlineSignatureFloor != 40 {
		t.Fatalf("holographic defaults = callers %d, signatures %d, types %d, importers %d, outline %d, floor %d; want 2, 2, 1, 1, 2, 40",
			def.HolographicCallersSharePercent, def.HolographicSignaturesSharePercent, def.HolographicTypesSharePercent,
			def.HolographicImportersSharePercent, def.HolographicOutlineSharePercent, def.HolographicOutlineSignatureFloor)
	}
	path := writeCampaignConfig(t, `{"working": {"holographic_callers_share_percent": 5}}`)
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	spans := cfg.GetWorkingConfig()
	if spans.HolographicCallersSharePercent != 5 {
		t.Fatalf("holographic_callers_share_percent = %d, want the file's 5", spans.HolographicCallersSharePercent)
	}
	found := false
	for _, p := range spans.Params() {
		if p.Key == "/working_holographic_callers_share_percent" && p.Value == 5 {
			found = true
		}
	}
	if !found {
		t.Fatalf("no /working_holographic_callers_share_percent=5 row in %+v", spans.Params())
	}
	// 0 is absent (omitempty), like every sibling span, so it takes the
	// default; only a share past 100 is a contradiction the loader can see.
	// An explicit 0 on signatures must not drag the callers value with it.
	absent, err := LoadUserConfig(writeCampaignConfig(t, `{"working": {"holographic_callers_share_percent": 0, "holographic_signatures_share_percent": 0}}`))
	if err != nil {
		t.Fatalf("a zero share refused to load: %v", err)
	}
	got := absent.GetWorkingConfig()
	if got.HolographicCallersSharePercent != 2 || got.HolographicSignaturesSharePercent != 2 {
		t.Fatalf("zero shares took callers %d, signatures %d; want the defaults 2 and 2", got.HolographicCallersSharePercent, got.HolographicSignaturesSharePercent)
	}
	mixed, err := LoadUserConfig(writeCampaignConfig(t, `{"working": {"holographic_callers_share_percent": 5, "holographic_signatures_share_percent": 0}}`))
	if err != nil {
		t.Fatalf("a zero signature share refused to load: %v", err)
	}
	got = mixed.GetWorkingConfig()
	if got.HolographicCallersSharePercent != 5 || got.HolographicSignaturesSharePercent != 2 {
		t.Fatalf("mixed shares took callers %d, signatures %d; want 5 and the default 2", got.HolographicCallersSharePercent, got.HolographicSignaturesSharePercent)
	}
	if _, err := LoadUserConfig(writeCampaignConfig(t, `{"working": {"holographic_callers_share_percent": 101}}`)); err == nil {
		t.Fatal("a share of 101 loaded; the checker must refuse it")
	}
	if _, err := LoadUserConfig(writeCampaignConfig(t, `{"working": {"holographic_outline_signature_floor": -1}}`)); err == nil {
		t.Fatal("an outline signature floor of -1 loaded; the checker must refuse it")
	}
	wide, err := LoadUserConfig(writeCampaignConfig(t, `{"working": {"holographic_outline_signature_floor": 1000}}`))
	if err != nil {
		t.Fatalf("a floor of 1000 refused to load: %v", err)
	}
	if got := wide.GetWorkingConfig().HolographicOutlineSignatureFloor; got != 1000 {
		t.Fatalf("outline signature floor = %d, want 1000 (it is not a percent)", got)
	}
	if _, err := LoadUserConfig(writeCampaignConfig(t, `{"working": {"holographic_caller_share_percent": 5}}`)); err == nil {
		t.Fatal("the singular holographic_caller_share_percent key loaded; it is not a config key")
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
