package config

import (
	"fmt"
	"time"
)

// ApprovalConfig is the `approval` section of .nerd/config.json: the bounds of
// the request-bound human-approval lifecycle (internal/approval).
//
// It sets no policy. Which actions a person may approve is the constitution's
// human_approvable/1; this section only says how long a request waits, how
// much of an operation its preview shows, and how long decided requests stay
// in the journal.
type ApprovalConfig struct {
	// RequestTTL is how long a request waits for a decision, and then for the
	// approved call to claim it. Past it the request expires and the call is
	// refused. Default "15m".
	RequestTTL string `json:"request_ttl,omitempty"`
	// PreviewMaxBytes bounds the arguments text and each side of the diff a
	// preview shows. Anything over it is withheld and marked incomplete,
	// never cut silently. Default 262144.
	PreviewMaxBytes int `json:"preview_max_bytes,omitempty"`
	// JournalRetention is how long a decided request is kept in
	// .nerd/approvals/journal.jsonl after it was created. Default "168h".
	JournalRetention string `json:"journal_retention,omitempty"`
}

// DefaultApprovalConfig is the approval section with every field written down.
func DefaultApprovalConfig() ApprovalConfig {
	return ApprovalConfig{RequestTTL: "15m", PreviewMaxBytes: 256 * 1024, JournalRetention: "168h"}
}

// GetApprovalConfig returns the approval section with every absent field
// defaulted. A nil receiver is the defaults.
func (c *UserConfig) GetApprovalConfig() ApprovalConfig {
	if c == nil || c.Approval == nil {
		return DefaultApprovalConfig()
	}
	return c.Approval.WithDefaults()
}

// WithDefaults fills every absent field from DefaultApprovalConfig.
func (c ApprovalConfig) WithDefaults() ApprovalConfig {
	d := DefaultApprovalConfig()
	if c.RequestTTL == "" {
		c.RequestTTL = d.RequestTTL
	}
	if c.JournalRetention == "" {
		c.JournalRetention = d.JournalRetention
	}
	if c.PreviewMaxBytes == 0 {
		c.PreviewMaxBytes = d.PreviewMaxBytes
	}
	return c
}

// minApprovalPreviewBytes is one screen of a diff: a smaller preview would
// withhold nearly every real change and show the person only the marker.
const minApprovalPreviewBytes = 1024

// Check reports the contradictions in an approval section, addressed under
// prefix ("approval").
func (c ApprovalConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	add := func(field, msg, fix string) {
		out = append(out, Problem{Severity: SeverityError, Path: prefix + "." + field, Message: msg, Fix: fix})
	}
	durations := map[string]time.Duration{}
	for _, f := range []struct{ name, v string }{
		{"request_ttl", c.RequestTTL},
		{"journal_retention", c.JournalRetention},
	} {
		d, err := time.ParseDuration(f.v)
		switch {
		case err != nil:
			add(f.name, fmt.Sprintf("%q is not a duration: %v", f.v, err), `a Go duration such as "15m" or "168h"`)
		case d <= 0:
			add(f.name, fmt.Sprintf("%q is not positive", f.v), "a positive duration, or remove the key for the default")
		default:
			durations[f.name] = d
		}
	}
	if ttl, ok := durations["request_ttl"]; ok {
		if keep, ok := durations["journal_retention"]; ok && keep < ttl {
			add("journal_retention", fmt.Sprintf("%s is shorter than request_ttl (%s): a request could leave the journal while still open", c.JournalRetention, c.RequestTTL),
				"a retention at least request_ttl")
		}
	}
	if c.PreviewMaxBytes < minApprovalPreviewBytes {
		add("preview_max_bytes", fmt.Sprintf("%d is below %d", c.PreviewMaxBytes, minApprovalPreviewBytes),
			fmt.Sprintf("a byte count of at least %d, or remove the key for the default", minApprovalPreviewBytes))
	}
	return out
}

// ApprovalPolicy is an ApprovalConfig resolved: defaults filled, durations
// parsed. Resolve is the only way to make one, so a policy in hand has passed
// Check.
type ApprovalPolicy struct {
	RequestTTL       time.Duration
	PreviewMaxBytes  int
	JournalRetention time.Duration
}

// Resolve defaults, checks and parses the section.
func (c ApprovalConfig) Resolve() (ApprovalPolicy, error) {
	c = c.WithDefaults()
	if problems := c.Check("approval"); len(problems) > 0 {
		msg := fmt.Sprintf("the approval config has %d error(s):", len(problems))
		for _, p := range problems {
			msg += "\n  " + p.String()
		}
		return ApprovalPolicy{}, fmt.Errorf("%s", msg)
	}
	ttl, _ := time.ParseDuration(c.RequestTTL)
	keep, _ := time.ParseDuration(c.JournalRetention)
	return ApprovalPolicy{RequestTTL: ttl, PreviewMaxBytes: c.PreviewMaxBytes, JournalRetention: keep}, nil
}
