package config

import "fmt"

// RoutingConfig is the `routing` section of .nerd/config.json: the thresholds
// the kernel's routing arbitration (policy/routing_arbitration.mg) decides an
// interactive turn's lane with. Steve's rule, as for every section:
// "everything is configurable, not hard coded.. all from config.json".
//
// Before this section the delegation gate was the literal 50 in
// should_delegate, and chat carried a Go copy of it (confidence >= 0.5) that
// replaced the kernel's "no" with its own answer (sweep finding F12).
type RoutingConfig struct {
	// DelegationMinConfidence is the perception confidence, as a percent, at
	// or above which a turn whose verb maps to a shard is handed to that
	// shard. A mutation below it deliberates first (the /deliberate lane).
	DelegationMinConfidence int `json:"delegation_min_confidence,omitempty"`
	// Deliberation bounds include the initial divergent round. A stalled
	// thread stops without claiming success or asking for missing data.
	DeliberationRoundCap     int `json:"deliberation_round_cap,omitempty"`
	DeliberationNoProgress   int `json:"deliberation_no_progress,omitempty"`
	DeliberationToolCalls    int `json:"deliberation_tool_calls,omitempty"`
	DeliberationCandidateCap int `json:"deliberation_candidate_cap,omitempty"`
	DeliberationContextBytes int `json:"deliberation_context_bytes,omitempty"`
}

// DefaultRoutingConfig is the routing section with every field written down.
func DefaultRoutingConfig() RoutingConfig {
	return RoutingConfig{DelegationMinConfidence: 50, DeliberationRoundCap: 4,
		DeliberationNoProgress: 2, DeliberationToolCalls: 6,
		DeliberationCandidateCap: 4, DeliberationContextBytes: 64 << 10}
}

// GetRoutingConfig returns the routing section with every absent field
// defaulted. A nil receiver is the defaults.
func (c *UserConfig) GetRoutingConfig() RoutingConfig {
	if c == nil || c.Routing == nil {
		return DefaultRoutingConfig()
	}
	return c.Routing.WithDefaults()
}

// WithDefaults fills every absent field from DefaultRoutingConfig.
func (c RoutingConfig) WithDefaults() RoutingConfig {
	if c.DelegationMinConfidence == 0 {
		c.DelegationMinConfidence = DefaultRoutingConfig().DelegationMinConfidence
	}
	d := DefaultRoutingConfig()
	if c.DeliberationRoundCap == 0 {
		c.DeliberationRoundCap = d.DeliberationRoundCap
	}
	if c.DeliberationNoProgress == 0 {
		c.DeliberationNoProgress = d.DeliberationNoProgress
	}
	if c.DeliberationToolCalls == 0 {
		c.DeliberationToolCalls = d.DeliberationToolCalls
	}
	if c.DeliberationCandidateCap == 0 {
		c.DeliberationCandidateCap = d.DeliberationCandidateCap
	}
	if c.DeliberationContextBytes == 0 {
		c.DeliberationContextBytes = d.DeliberationContextBytes
	}
	return c
}

// Check reports the contradictions in a routing section, addressed under
// prefix ("routing").
func (c RoutingConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	if v := c.DelegationMinConfidence; v < 1 || v > 100 {
		out = append(out, Problem{
			Severity: SeverityError,
			Path:     prefix + ".delegation_min_confidence",
			Message:  fmt.Sprintf("%d is not a percent between 1 and 100", v),
			Fix:      "a percent from 1 to 100, or remove the key for the default",
		})
	}
	for _, knob := range []struct {
		name  string
		value int
	}{
		{"deliberation_round_cap", c.DeliberationRoundCap},
		{"deliberation_no_progress", c.DeliberationNoProgress},
		{"deliberation_tool_calls", c.DeliberationToolCalls},
		{"deliberation_candidate_cap", c.DeliberationCandidateCap},
		{"deliberation_context_bytes", c.DeliberationContextBytes},
	} {
		if knob.value < 1 {
			out = append(out, Problem{Severity: SeverityError,
				Path: prefix + "." + knob.name, Message: "must be positive", Fix: "use a positive integer or omit for the default"})
		}
	}
	if c.DeliberationCandidateCap == 1 {
		out = append(out, Problem{Severity: SeverityError, Path: prefix + ".deliberation_candidate_cap",
			Message: "divergence requires at least two candidates", Fix: "use 2 or more, or omit for the default"})
	}
	return out
}

// Params are the routing knobs the kernel's rules read, as config_param
// rows. The key is declared config_param_required(/routing, Key) next to
// should_delegate (policy/delegation.mg).
func (c RoutingConfig) Params() []Param {
	c = c.WithDefaults()
	return []Param{
		{Key: "/routing_delegation_min_confidence", Value: int64(c.DelegationMinConfidence)},
		{Key: "/deliberation_round_cap", Value: int64(c.DeliberationRoundCap)},
		{Key: "/deliberation_no_progress", Value: int64(c.DeliberationNoProgress)},
		{Key: "/deliberation_tool_calls", Value: int64(c.DeliberationToolCalls)},
		{Key: "/deliberation_candidate_cap", Value: int64(c.DeliberationCandidateCap)},
		{Key: "/deliberation_context_bytes", Value: int64(c.DeliberationContextBytes)},
	}
}
