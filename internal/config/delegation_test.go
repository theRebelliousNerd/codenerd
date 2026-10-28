package config

import (
	"encoding/json"
	"strings"
	"testing"
)

// The delegation section's judge bounds are defaulted, settable in
// .nerd/config.json, validated by the config check, and -- for the one the
// kernel decides with -- handed to the kernel as a config_param.
func TestDelegationConfig_JudgeBounds(t *testing.T) {
	d := DefaultDelegationConfig()
	if *d.JudgeEvidenceRounds != 1 || *d.JudgeEvidenceBytes != 24000 || *d.JudgeReplyBytes != 16384 || *d.JudgeRejectConfidence != 0 {
		t.Fatalf("defaults = %+v", d)
	}

	var set DelegationConfig
	if err := json.Unmarshal([]byte(`{"judge_evidence_rounds":2,"judge_evidence_bytes":8000,"judge_reply_bytes":4096}`), &set); err != nil {
		t.Fatal(err)
	}
	set = set.WithDefaults()
	if *set.JudgeEvidenceRounds != 2 || *set.JudgeEvidenceBytes != 8000 || *set.JudgeReplyBytes != 4096 || *set.JudgeRejectConfidence != 0 {
		t.Fatalf("set = %+v, want the file's values and the rest defaulted", set)
	}

	params := map[string]int64{}
	for _, p := range set.Params() {
		params[p.Key] = p.Value
	}
	if params["/delegation_judge_evidence_rounds"] != 2 || len(params) != 2 {
		t.Fatalf("params = %v, want the evidence rounds beside the reject confidence", params)
	}
	if problems := set.Check("delegation"); len(problems) != 0 {
		t.Fatalf("valid section reported %v", problems)
	}

	bad, tiny, huge := 9, 10, 1<<30
	problems := DelegationConfig{JudgeEvidenceRounds: &bad, JudgeEvidenceBytes: &tiny, JudgeReplyBytes: &huge}.Check("delegation")
	var paths []string
	for _, p := range problems {
		paths = append(paths, p.Path)
	}
	for _, want := range []string{"delegation.judge_evidence_rounds", "delegation.judge_evidence_bytes", "delegation.judge_reply_bytes"} {
		if !strings.Contains(strings.Join(paths, " "), want) {
			t.Errorf("Check reported %v, want %s", paths, want)
		}
	}
}
