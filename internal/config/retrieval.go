package config

import "fmt"

// RetrievalConfig is the `retrieval` section of .nerd/config.json: the
// thresholds the kernel decides an issue-driven retrieval pass's hand-off
// with (retrieval_brief_file in internal/core/defaults/policy/retrieval.mg).
//
// Whether a turn retrieves at all is not a knob: it is issue_retrieval_wanted,
// derived from the turn's intent verb. What the model is then handed as
// starting points is: a file the issue named always, a file the search found
// only when its relevance clears BriefMinRelevance.
type RetrievalConfig struct {
	// BriefMinRelevance is the relevance, as a percent, at or above which a
	// file the sparse search found (tiers 2-4) is handed to the model as a
	// starting point. Relevance is types.PercentFromRatio of the builder's
	// score, so a file matching several weighted keywords saturates at 100;
	// import neighbours score 50 and the definition-scan fallback 30.
	BriefMinRelevance int `json:"brief_min_relevance,omitempty"`
}

// DefaultRetrievalConfig is the retrieval section with every field written
// down. 60 hands over keyword matches on a primary or secondary term and
// drops import neighbours and the definition-scan fallback, which are
// evidence for the context compressor, not places to start reading.
func DefaultRetrievalConfig() RetrievalConfig {
	return RetrievalConfig{BriefMinRelevance: 60}
}

// GetRetrievalConfig returns the retrieval section with every absent field
// defaulted. A nil receiver is the defaults.
func (c *UserConfig) GetRetrievalConfig() RetrievalConfig {
	if c == nil || c.Retrieval == nil {
		return DefaultRetrievalConfig()
	}
	return c.Retrieval.WithDefaults()
}

// WithDefaults fills every absent field from DefaultRetrievalConfig.
func (c RetrievalConfig) WithDefaults() RetrievalConfig {
	if c.BriefMinRelevance == 0 {
		c.BriefMinRelevance = DefaultRetrievalConfig().BriefMinRelevance
	}
	return c
}

// Check reports the contradictions in a retrieval section, addressed under
// prefix ("retrieval").
func (c RetrievalConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	if v := c.BriefMinRelevance; v < 1 {
		out = append(out, Problem{
			Severity: SeverityError,
			Path:     prefix + ".brief_min_relevance",
			Message:  fmt.Sprintf("%d is not a positive percent", v),
			Fix:      "a percent of 1 or more, or remove the key for the default",
		})
	}
	return out
}

// Params are the retrieval knobs the kernel's rules read, as config_param
// rows, read by retrieval_brief_file (policy/retrieval.mg). The key is not
// declared required: without it the rule hands over only the files the issue
// named, which fails closed.
func (c RetrievalConfig) Params() []Param {
	c = c.WithDefaults()
	return []Param{
		{Key: "/retrieval_brief_min_relevance", Value: int64(c.BriefMinRelevance)},
	}
}
