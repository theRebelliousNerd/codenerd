package config

import "time"

// BrowserReaperConfig bounds individual process inspection and cleanup requests.
type BrowserReaperConfig struct {
	TimeoutMs      int `json:"timeout_ms,omitempty"`
	PollIntervalMs int `json:"poll_interval_ms,omitempty"`
}

func DefaultBrowserReaperConfig() BrowserReaperConfig {
	return BrowserReaperConfig{TimeoutMs: 2000, PollIntervalMs: 50}
}

func (c BrowserReaperConfig) WithDefaults() BrowserReaperConfig {
	d := DefaultBrowserReaperConfig()
	if c.TimeoutMs == 0 {
		c.TimeoutMs = d.TimeoutMs
	}
	if c.PollIntervalMs == 0 {
		c.PollIntervalMs = d.PollIntervalMs
	}
	return c
}

func (c BrowserReaperConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	for _, field := range []struct {
		name  string
		value int
	}{{"timeout_ms", c.TimeoutMs}, {"poll_interval_ms", c.PollIntervalMs}} {
		if field.value <= 0 || int64(field.value) > int64((1<<63-1)/time.Millisecond) {
			out = append(out, Problem{Severity: SeverityError, Path: prefix + "." + field.name,
				Message: "must be a positive millisecond duration that fits time.Duration", Fix: "set a positive duration or omit the key"})
		}
	}
	if c.TimeoutMs > 0 && c.PollIntervalMs > c.TimeoutMs {
		out = append(out, Problem{Severity: SeverityError, Path: prefix + ".poll_interval_ms",
			Message: "exceeds timeout_ms", Fix: "use an interval no greater than timeout_ms"})
	}
	return out
}

func (c BrowserReaperConfig) Timeout() time.Duration {
	return time.Duration(c.WithDefaults().TimeoutMs) * time.Millisecond
}

func (c BrowserReaperConfig) PollInterval() time.Duration {
	return time.Duration(c.WithDefaults().PollIntervalMs) * time.Millisecond
}
