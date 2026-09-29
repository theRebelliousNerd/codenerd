package config

import (
	"fmt"
	"sync/atomic"
	"time"
)

// ResearchConfig is the `research` section of .nerd/config.json: the
// per-request bounds the research tools read for one network round trip or
// one evidence window (web_fetch, context7_fetch, web_search,
// browser_extract, browser_reason).
//
// Why its own section instead of llm_timeouts: those bound LLM calls (one
// HTTP call to a model vendor, a slot wait, an articulation call). These
// bound researcher HTTP fetches and browser evidence paging — different
// vendors, different latency profiles, different owners. Every value here
// bounds one request or one paging window, never a run: a windowed result
// names the remainder and the offset that reaches it.
type ResearchConfig struct {
	// WebFetchTimeout bounds one web_fetch HTTP round trip.
	WebFetchTimeout string `json:"web_fetch_timeout,omitempty"`
	// Context7Timeout bounds one context7_fetch HTTP round trip.
	Context7Timeout string `json:"context7_timeout,omitempty"`
	// WebSearchTimeout bounds one web_search HTTP round trip.
	WebSearchTimeout string `json:"web_search_timeout,omitempty"`
	// BrowserExtractTimeout bounds one browser_extract DOM read.
	BrowserExtractTimeout string `json:"browser_extract_timeout,omitempty"`

	// BrowserExtractMaxChars is the default combined text/HTML window in
	// runes, excluding the paging notice. Every rune stays reachable via
	// offset, so the window is paging rather than a cut.
	BrowserExtractMaxChars int `json:"browser_extract_max_chars,omitempty"`
	// BrowserExtractMaxCharsCap is the hard cap on that window: an explicit
	// max_chars above it is clamped, never refused.
	BrowserExtractMaxCharsCap int `json:"browser_extract_max_chars_cap,omitempty"`

	// BrowserReasonItems sizes one paging window over browser facts for the
	// full reason view. An explicit max_items is the model's own choice and
	// is honored as given.
	BrowserReasonItems int `json:"browser_reason_items,omitempty"`
	// BrowserReasonCompactItems is the same window for the compact reason
	// view when max_items is absent: compact stays the cheap rung of the
	// summary/compact/full ladder.
	BrowserReasonCompactItems int `json:"browser_reason_compact_items,omitempty"`
}

// DefaultResearchConfig is the research section with every field written
// down. The timeouts are the bounds the tools carried as literals before
// 2026-09-29 (60s/30s/30s/10s); the windows are the paging sizes those same
// call sites used (8000/32000, 20/10).
func DefaultResearchConfig() ResearchConfig {
	return ResearchConfig{
		WebFetchTimeout:           "60s",
		Context7Timeout:           "30s",
		WebSearchTimeout:          "30s",
		BrowserExtractTimeout:     "10s",
		BrowserExtractMaxChars:    8000,
		BrowserExtractMaxCharsCap: 32000,
		BrowserReasonItems:        20,
		BrowserReasonCompactItems: 10,
	}
}

// GetResearchConfig returns the research section with every absent field
// defaulted. A nil receiver is the defaults.
func (c *UserConfig) GetResearchConfig() ResearchConfig {
	if c == nil || c.Research == nil {
		return DefaultResearchConfig()
	}
	return c.Research.WithDefaults()
}

// WithDefaults fills every absent field from DefaultResearchConfig.
func (c ResearchConfig) WithDefaults() ResearchConfig {
	d := DefaultResearchConfig()
	if c.WebFetchTimeout == "" {
		c.WebFetchTimeout = d.WebFetchTimeout
	}
	if c.Context7Timeout == "" {
		c.Context7Timeout = d.Context7Timeout
	}
	if c.WebSearchTimeout == "" {
		c.WebSearchTimeout = d.WebSearchTimeout
	}
	if c.BrowserExtractTimeout == "" {
		c.BrowserExtractTimeout = d.BrowserExtractTimeout
	}
	if c.BrowserExtractMaxChars == 0 {
		c.BrowserExtractMaxChars = d.BrowserExtractMaxChars
	}
	if c.BrowserExtractMaxCharsCap == 0 {
		c.BrowserExtractMaxCharsCap = d.BrowserExtractMaxCharsCap
	}
	if c.BrowserReasonItems == 0 {
		c.BrowserReasonItems = d.BrowserReasonItems
	}
	if c.BrowserReasonCompactItems == 0 {
		c.BrowserReasonCompactItems = d.BrowserReasonCompactItems
	}
	return c
}

// ResearchPolicy is a ResearchConfig resolved for the tools: defaults
// filled, durations parsed. Resolve is the only way to make one, so a
// policy in hand has passed Check.
type ResearchPolicy struct {
	WebFetchTimeout       time.Duration
	Context7Timeout       time.Duration
	WebSearchTimeout      time.Duration
	BrowserExtractTimeout time.Duration

	BrowserExtractMaxChars    int
	BrowserExtractMaxCharsCap int
	BrowserReasonItems        int
	BrowserReasonCompactItems int
}

// Resolve defaults, checks and parses the section. Every problem Check
// finds is an error here.
func (c ResearchConfig) Resolve() (ResearchPolicy, error) {
	c = c.WithDefaults()
	if problems := c.Check("research"); len(problems) > 0 {
		msg := fmt.Sprintf("the research config has %d error(s):", len(problems))
		for _, p := range problems {
			msg += "\n  " + p.String()
		}
		return ResearchPolicy{}, fmt.Errorf("%s", msg)
	}
	d := func(s string) time.Duration {
		v, _ := time.ParseDuration(s) // Check parsed every one of these
		return v
	}
	return ResearchPolicy{
		WebFetchTimeout:           d(c.WebFetchTimeout),
		Context7Timeout:           d(c.Context7Timeout),
		WebSearchTimeout:          d(c.WebSearchTimeout),
		BrowserExtractTimeout:     d(c.BrowserExtractTimeout),
		BrowserExtractMaxChars:    c.BrowserExtractMaxChars,
		BrowserExtractMaxCharsCap: c.BrowserExtractMaxCharsCap,
		BrowserReasonItems:        c.BrowserReasonItems,
		BrowserReasonCompactItems: c.BrowserReasonCompactItems,
	}, nil
}

// Check reports the contradictions in a research section, addressed under
// prefix ("research"). Absent fields are defaulted first, so only what the
// file says can be wrong.
func (c ResearchConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	add := func(field, msg, fix string) {
		out = append(out, Problem{Severity: SeverityError, Path: prefix + "." + field, Message: msg, Fix: fix})
	}
	for _, f := range []struct {
		name string
		v    string
	}{
		{"web_fetch_timeout", c.WebFetchTimeout},
		{"context7_timeout", c.Context7Timeout},
		{"web_search_timeout", c.WebSearchTimeout},
		{"browser_extract_timeout", c.BrowserExtractTimeout},
	} {
		d, err := time.ParseDuration(f.v)
		switch {
		case err != nil:
			add(f.name, fmt.Sprintf("%q is not a duration: %v", f.v, err), `a Go duration such as "30s", "10s" or "1500ms"`)
		case d <= 0:
			add(f.name, fmt.Sprintf("%q is not positive", f.v), "a positive duration, or remove the key for the default")
		}
	}
	for _, f := range []struct {
		name string
		v    int
	}{
		{"browser_extract_max_chars", c.BrowserExtractMaxChars},
		{"browser_extract_max_chars_cap", c.BrowserExtractMaxCharsCap},
		{"browser_reason_items", c.BrowserReasonItems},
		{"browser_reason_compact_items", c.BrowserReasonCompactItems},
	} {
		if f.v < 1 {
			add(f.name, fmt.Sprintf("%d is below 1", f.v), "a count of at least 1, or remove the key for the default")
		}
	}
	if c.BrowserExtractMaxChars >= 1 && c.BrowserExtractMaxCharsCap >= 1 &&
		c.BrowserExtractMaxCharsCap < c.BrowserExtractMaxChars {
		add("browser_extract_max_chars_cap",
			fmt.Sprintf("%d is below browser_extract_max_chars (%d): the default window would not fit its own cap",
				c.BrowserExtractMaxCharsCap, c.BrowserExtractMaxChars),
			"a cap at least browser_extract_max_chars")
	}
	return out
}

// activeResearchPolicy is the process-wide resolved research policy,
// installed by LoadUserConfig the same way LLM timeouts are: the research
// tool call sites read it without opening the config file themselves.
var activeResearchPolicy atomic.Pointer[ResearchPolicy]

func init() {
	SetResearchPolicy(mustResolveResearchDefaults())
}

// mustResolveResearchDefaults resolves the defaults for the init install.
// A failure here is a programming error in the defaults above, not a user
// error, and the process must not boot on silently-wrong bounds.
func mustResolveResearchDefaults() ResearchPolicy {
	p, err := DefaultResearchConfig().Resolve()
	if err != nil {
		panic("config: DefaultResearchConfig does not resolve: " + err.Error())
	}
	return p
}

// SetResearchPolicy installs the process-wide research policy.
// LoadUserConfig is the production caller; tests install and restore.
func SetResearchPolicy(p ResearchPolicy) {
	activeResearchPolicy.Store(&p)
}

// ResolvedResearchPolicy is the installed research policy. Without a load
// it is the defaults.
func ResolvedResearchPolicy() ResearchPolicy {
	if p := activeResearchPolicy.Load(); p != nil {
		return *p
	}
	return mustResolveResearchDefaults()
}
