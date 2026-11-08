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

func TestParseTrialCommand_StabilizeEdges(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    TrialCommand
		wantErr bool
	}{
		{name: "hours only keeps exposures default", args: []string{"a/b", "6"}, want: TrialCommand{Action: TrialApprove, AtomID: "a/b", TTL: 6 * time.Hour}},
		{name: "non-numeric hours rejected", args: []string{"a/b", "x"}, wantErr: true},
		{name: "negative hours rejected", args: []string{"a/b", "-3"}, wantErr: true},
		{name: "zero max-exposures rejected", args: []string{"a/b", "2", "0"}, wantErr: true},
		{name: "negative max-exposures rejected", args: []string{"a/b", "2", "-1"}, wantErr: true},
		{name: "non-numeric max-exposures rejected", args: []string{"a/b", "2", "y"}, wantErr: true},
		{name: "empty hours rejected", args: []string{"a/b", ""}, wantErr: true},
		{name: "empty list rejected", args: []string{}, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseTrialCommand(c.args)
			if (err != nil) != c.wantErr {
				t.Fatalf("ParseTrialCommand(%v) err=%v wantErr=%v", c.args, err, c.wantErr)
			}
			if !c.wantErr && got != c.want {
				t.Errorf("ParseTrialCommand(%v) = %+v, want %+v", c.args, got, c.want)
			}
		})
	}
}

func TestTrialEnvelopeState_StabilizeEdges(t *testing.T) {
	now := time.Unix(1700000000, 0)
	t.Run("expiry boundary is expired", func(t *testing.T) {
		e := TrialEnvelope{Version: "abc", ExpiresAt: now, MaxExposures: 5}
		if got := e.State(now); got != "expired" {
			t.Errorf("State(at expiry) = %q, want expired", got)
		}
		if e.Live(now) {
			t.Error("Live(at expiry) = true, want false")
		}
	})
	t.Run("expired outranks exhausted", func(t *testing.T) {
		e := TrialEnvelope{Version: "abc", ExpiresAt: now.Add(-time.Hour), MaxExposures: 2, Exposures: 5}
		if got := e.State(now); got != "expired" {
			t.Errorf("State(expired+exhausted) = %q, want expired", got)
		}
		if e.Live(now) {
			t.Error("Live(expired+exhausted) = true, want false")
		}
	})
	t.Run("zero budget is exhausted while live", func(t *testing.T) {
		e := TrialEnvelope{Version: "abc", ExpiresAt: now.Add(time.Hour), MaxExposures: 0}
		if got := e.State(now); got != "exhausted" {
			t.Errorf("State(zero budget) = %q, want exhausted", got)
		}
	})
}

func TestTrialEnvelopeShortVersion_StabilizeEdges(t *testing.T) {
	cases := []struct {
		name    string
		version string
		want    string
	}{
		{name: "empty stays empty", version: "", want: ""},
		{name: "short stays whole", version: "abc", want: "abc"},
		{name: "exact twelve stays whole", version: "0123456789ab", want: "0123456789ab"},
		{name: "thirteen truncates", version: "0123456789abc", want: "0123456789ab"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := TrialEnvelope{Version: c.version}
			if got := e.ShortVersion(); got != c.want {
				t.Errorf("ShortVersion(%q) = %q, want %q", c.version, got, c.want)
			}
		})
	}
}
