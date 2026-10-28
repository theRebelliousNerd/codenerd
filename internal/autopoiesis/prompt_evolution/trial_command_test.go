package prompt_evolution

import (
	"testing"
	"time"
)

func TestParseTrialCommand(t *testing.T) {
	cases := []struct {
		args    []string
		want    TrialCommand
		wantErr bool
	}{
		{args: []string{"list"}, want: TrialCommand{Action: TrialList}},
		{args: []string{"revoke", "a/b"}, want: TrialCommand{Action: TrialRevoke, AtomID: "a/b"}},
		{args: []string{"a/b"}, want: TrialCommand{Action: TrialApprove, AtomID: "a/b"}},
		{args: []string{"a/b", "6", "4"}, want: TrialCommand{Action: TrialApprove, AtomID: "a/b", TTL: 6 * time.Hour, MaxExposures: 4}},
		{args: nil, wantErr: true},
		{args: []string{"revoke"}, wantErr: true},
		{args: []string{"a/b", "0"}, wantErr: true},
		{args: []string{"a/b", "2", "x"}, wantErr: true},
	}
	for _, c := range cases {
		got, err := ParseTrialCommand(c.args)
		if (err != nil) != c.wantErr || (!c.wantErr && got != c.want) {
			t.Errorf("ParseTrialCommand(%v) = %+v, %v", c.args, got, err)
		}
	}
}

func TestTrialEnvelopeState(t *testing.T) {
	now := time.Unix(1700000000, 0)
	e := TrialEnvelope{Version: "0123456789abcdef", ExpiresAt: now.Add(time.Hour), MaxExposures: 2}
	if e.State(now) != "live" || !e.Live(now) || e.ShortVersion() != "0123456789ab" {
		t.Errorf("fresh envelope: %s %v %s", e.State(now), e.Live(now), e.ShortVersion())
	}
	e.Exposures = 2
	if e.State(now) != "exhausted" || e.Live(now) {
		t.Error("exhausted envelope still live")
	}
	e.Exposures = 0
	if e.State(now.Add(2*time.Hour)) != "expired" || e.Live(now.Add(2*time.Hour)) {
		t.Error("expired envelope still live")
	}
}
