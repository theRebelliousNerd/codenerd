package config

import (
	"strings"
	"testing"
)

// The defaults are the spans the policy carried as literals until 2026-09-23,
// and they check clean.
func TestWorkingConfig_TheDefaultsCheckClean(t *testing.T) {
	if problems := DefaultWorkingConfig().Check("working"); len(problems) != 0 {
		t.Fatalf("the defaults do not check clean: %v", problems)
	}
}

// A key the user writes reaches the policy; every key they leave out is the
// default. transcript_slack 0 is a value (the every-round slide), not
// "absent".
func TestWorkingConfig_AKeyInTheFileReachesThePolicy(t *testing.T) {
	path := writeCampaignConfig(t, `{"working": {"nudge_rounds": 5, "transcript_slack": 0}}`)
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	spans := cfg.GetWorkingConfig()
	if spans.NudgeRounds != 5 {
		t.Errorf("nudge_rounds = %d, want the file's 5", spans.NudgeRounds)
	}
	if spans.TranscriptSlack == nil || *spans.TranscriptSlack != 0 {
		t.Errorf("transcript_slack = %v, want the file's 0", spans.TranscriptSlack)
	}
	def := DefaultWorkingConfig()
	if spans.CommitRounds != def.CommitRounds || spans.RepeatThreshold != def.RepeatThreshold || spans.SectionCeilingBytes != def.SectionCeilingBytes {
		t.Errorf("absent keys did not take the defaults: %+v", spans)
	}
	params := map[string]int64{}
	for _, p := range spans.Params() {
		params[p.Key] = p.Value
	}
	if params["/working_nudge_rounds"] != 5 || params["/working_transcript_slack"] != 0 {
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
	bad := WorkingConfig{RepeatThreshold: 1, CommitRounds: 20, StallRounds: 10, SectionCeilingBytes: 100}
	var paths []string
	for _, p := range bad.Check("working") {
		if p.Severity != SeverityError {
			t.Errorf("%s is %s, want an error", p.Path, p.Severity)
		}
		paths = append(paths, p.Path)
	}
	joined := strings.Join(paths, " ")
	for _, want := range []string{"working.repeat_threshold", "working.stall_rounds", "working.section_ceiling_bytes"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Check did not name %s; it found %v", want, paths)
		}
	}
	path := writeCampaignConfig(t, `{"working": {"repeat_threshold": 1}}`)
	if _, err := LoadUserConfig(path); err == nil {
		t.Fatal("a working section with repeat_threshold 1 loaded")
	}
}
