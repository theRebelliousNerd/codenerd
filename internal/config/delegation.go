package config

import "fmt"

// DelegationConfig is the `delegation` section of .nerd/config.json: the
// thresholds the kernel decides a chat delegation's attempts with
// (policy/delegation.mg, delegation_move). The attempt cap is the persona's
// own, shard_profiles.<persona>.max_retries.
//
// Until 2026-09-23 an LLM judge's word decided whether a delegated mutation
// succeeded, the turn's kernel verdict was dropped on the way, and an LLM
// "shard selection advisor" picked who retried (sweep finding F5). The
// kernel's verdict decides now, and the judge can only withhold.
type DelegationConfig struct {
	// JudgeRejectConfidence is the confidence, as a percent, at or above
	// which the LLM judge's "fail" withholds an attempt the kernel verdict
	// calls done. 0 lets any failing judgment withhold. A pointer because 0
	// is meaningful.
	JudgeRejectConfidence *int `json:"judge_reject_confidence,omitempty"`
}

// DefaultDelegationConfig is the delegation section with every field written
// down.
func DefaultDelegationConfig() DelegationConfig {
	reject := 0
	return DelegationConfig{JudgeRejectConfidence: &reject}
}

// GetDelegationConfig returns the delegation section with every absent field
// defaulted. A nil receiver is the defaults.
func (c *UserConfig) GetDelegationConfig() DelegationConfig {
	if c == nil || c.Delegation == nil {
		return DefaultDelegationConfig()
	}
	return c.Delegation.WithDefaults()
}

// WithDefaults fills every absent field from DefaultDelegationConfig.
func (c DelegationConfig) WithDefaults() DelegationConfig {
	if c.JudgeRejectConfidence == nil {
		c.JudgeRejectConfidence = DefaultDelegationConfig().JudgeRejectConfidence
	}
	return c
}

// Check reports the contradictions in a delegation section, addressed under
// prefix ("delegation").
func (c DelegationConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	if v := *c.JudgeRejectConfidence; v < 0 || v > 100 {
		out = append(out, Problem{
			Severity: SeverityError,
			Path:     prefix + ".judge_reject_confidence",
			Message:  fmt.Sprintf("%d is outside 0-100", v),
			Fix:      "an integer percent, or remove the key for the default",
		})
	}
	return out
}

// Params are the delegation knobs the kernel's rules read, as config_param
// rows. The key is declared config_param_required(/delegation, Key) next to
// judge_rejects (policy/delegation.mg).
func (c DelegationConfig) Params() []Param {
	c = c.WithDefaults()
	return []Param{
		{Key: "/delegation_judge_reject_confidence", Value: int64(*c.JudgeRejectConfidence)},
	}
}
