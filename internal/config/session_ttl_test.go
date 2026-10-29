package config

import (
	"testing"
	"time"
)

func TestSessionRetention(t *testing.T) {
	cases := []struct {
		raw  string
		want time.Duration
		bad  bool
	}{
		{"", 14 * 24 * time.Hour, false},
		{"  ", 14 * 24 * time.Hour, false},
		{"0", 0, false},
		{"off", 0, false},
		{"OFF", 0, false},
		{"14d", 14 * 24 * time.Hour, false},
		{"30d", 30 * 24 * time.Hour, false},
		{"0.5d", 12 * time.Hour, false},
		{"36h", 36 * time.Hour, false},
		{"90m", 90 * time.Minute, false},
		{"-1d", 0, true},
		{"-5h", 0, true},
		{"soon", 0, true},
		{"d", 0, true},
		{"14", 0, true},
	}
	for _, c := range cases {
		got, err := (&LoggingConfig{SessionTTL: c.raw}).SessionRetention()
		if c.bad {
			if err == nil {
				t.Errorf("SessionRetention(%q) = %v, want an error", c.raw, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("SessionRetention(%q) = %v, %v; want %v", c.raw, got, err, c.want)
		}
	}
}

func TestSessionRetentionOfNoLoggingBlockIsTheDefault(t *testing.T) {
	var lc *LoggingConfig
	if got, err := lc.SessionRetention(); err != nil || got != DefaultSessionTTL {
		t.Fatalf("got %v, %v", got, err)
	}
	if DefaultSessionTTL != 14*24*time.Hour {
		t.Fatalf("the default is 14 days, got %v", DefaultSessionTTL)
	}
}

func TestCheckFlagsABadSessionTTL(t *testing.T) {
	cfg := &UserConfig{Logging: &LoggingConfig{SessionTTL: "forever"}}
	var found bool
	for _, p := range cfg.Check(nil) {
		if p.Path == "logging.session_ttl" && p.Severity == SeverityError {
			found = true
		}
	}
	if !found {
		t.Fatal("Check must report a logging.session_ttl that does not parse")
	}
	ok := &UserConfig{Logging: &LoggingConfig{SessionTTL: "off"}}
	for _, p := range ok.Check(nil) {
		if p.Path == "logging.session_ttl" {
			t.Fatalf("a valid value was flagged: %+v", p)
		}
	}
}
