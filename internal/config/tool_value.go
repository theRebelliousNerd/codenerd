package config

import "fmt"

// ToolValueConfig is the `tool_value` section of .nerd/config.json: the
// thresholds the tool-value policy (policy/tool_value.mg) classifies every
// tool with, as /earning, /marginal, /dead or /unknown. A tool earns its place
// by doing what nothing else can, by consuming far more than it returns, or by
// costing far fewer tokens than the work it does; these say how much "far more"
// is and how much evidence is enough to say so. Each field reaches the policy
// as config_param(/tool_value_<key>, N) (Params); a zero is absent
// (WithDefaults).
//
// Nothing here retires a tool. A class of /dead derives a proposal, and a
// proposal waits for the person (or the standing yolo instruction) to approve.
type ToolValueConfig struct {
	// MinSessions is how many sessions a tool must have been offered, without
	// one call, to be /dead as never called (/tool_value_min_sessions).
	MinSessions int `json:"min_sessions,omitempty"`
	// MinCalls is how many calls with a reported scan a tool needs before its
	// condensation and leverage are evidence (/tool_value_min_calls).
	MinCalls int `json:"min_calls,omitempty"`
	// MinCondensationPercent is consumed bytes as a percent of returned bytes
	// at or above which a tool condenses enough to earn: 1000 is ten times
	// what it returns (/tool_value_min_condensation_percent).
	MinCondensationPercent int `json:"min_condensation_percent,omitempty"`
	// MinLeveragePercent is the tokens of material consumed as a percent of the
	// tokens the tool cost (prompt share, calls, results, verification) at or
	// above which it earns: 1000 is ten tokens of work per token spent
	// (/tool_value_min_leverage_percent).
	MinLeveragePercent int `json:"min_leverage_percent,omitempty"`
	// DeadLeveragePercent is the leverage below which an overlapping tool is
	// /dead for costing more than the work it does: 100 is break-even
	// (/tool_value_dead_leverage_percent).
	DeadLeveragePercent int `json:"dead_leverage_percent,omitempty"`
}

// DefaultToolValueConfig is the section with every field written down. The
// session and call floors are the smallest samples that are more than an
// accident of one long task; the ratios are the user's example (about 1,000
// tokens of round trip for work worth about 100,000) cut to a tenth, so a tool
// has to be an order of magnitude ahead, not a hundred.
func DefaultToolValueConfig() ToolValueConfig {
	return ToolValueConfig{
		MinSessions:            10,
		MinCalls:               20,
		MinCondensationPercent: 1000,
		MinLeveragePercent:     1000,
		DeadLeveragePercent:    100,
	}
}

// GetToolValueConfig returns the section with every absent field defaulted. A
// nil receiver is the defaults.
func (c *UserConfig) GetToolValueConfig() ToolValueConfig {
	if c == nil || c.ToolValue == nil {
		return DefaultToolValueConfig()
	}
	return c.ToolValue.WithDefaults()
}

// WithDefaults fills every absent field from DefaultToolValueConfig.
func (c ToolValueConfig) WithDefaults() ToolValueConfig {
	d := DefaultToolValueConfig()
	if c.MinSessions == 0 {
		c.MinSessions = d.MinSessions
	}
	if c.MinCalls == 0 {
		c.MinCalls = d.MinCalls
	}
	if c.MinCondensationPercent == 0 {
		c.MinCondensationPercent = d.MinCondensationPercent
	}
	if c.MinLeveragePercent == 0 {
		c.MinLeveragePercent = d.MinLeveragePercent
	}
	if c.DeadLeveragePercent == 0 {
		c.DeadLeveragePercent = d.DeadLeveragePercent
	}
	return c
}

// Check reports the contradictions in a tool_value section, addressed under
// prefix ("tool_value"). Absent fields are defaulted first.
func (c ToolValueConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	for _, f := range []struct {
		name string
		v    int
	}{
		{"min_sessions", c.MinSessions},
		{"min_calls", c.MinCalls},
		{"min_condensation_percent", c.MinCondensationPercent},
		{"min_leverage_percent", c.MinLeveragePercent},
		{"dead_leverage_percent", c.DeadLeveragePercent},
	} {
		if f.v < 1 {
			out = append(out, Problem{
				Severity: SeverityError,
				Path:     prefix + "." + f.name,
				Message:  fmt.Sprintf("%d is below 1", f.v),
				Fix:      "a count or percent of at least 1, or remove the key for the default",
			})
		}
	}
	// An overlapping tool cannot be dead below a bar that is above the one an
	// earner must clear: that would class one tool /dead and /earning at once.
	if c.DeadLeveragePercent >= 1 && c.MinLeveragePercent >= 1 && c.DeadLeveragePercent >= c.MinLeveragePercent {
		out = append(out, Problem{
			Severity: SeverityError,
			Path:     prefix + ".dead_leverage_percent",
			Message: fmt.Sprintf("%d is not below min_leverage_percent (%d)",
				c.DeadLeveragePercent, c.MinLeveragePercent),
			Fix: "a dead bar under the earning bar, or remove the key for the default",
		})
	}
	return out
}

// ToolValueBytesPerTokenKey is the unit conversion the policy divides consumed
// bytes by to price them in tokens. It is BytesPerToken, the estimator's own
// unit, asserted as a param so the rule and the estimator cannot disagree.
const ToolValueBytesPerTokenKey = "/tool_value_bytes_per_token"

// Params renders the section as the config_param rows the policy reads.
func (c ToolValueConfig) Params() []Param {
	c = c.WithDefaults()
	return []Param{
		{Key: "/tool_value_min_sessions", Value: int64(c.MinSessions)},
		{Key: "/tool_value_min_calls", Value: int64(c.MinCalls)},
		{Key: "/tool_value_min_condensation_percent", Value: int64(c.MinCondensationPercent)},
		{Key: "/tool_value_min_leverage_percent", Value: int64(c.MinLeveragePercent)},
		{Key: "/tool_value_dead_leverage_percent", Value: int64(c.DeadLeveragePercent)},
		{Key: ToolValueBytesPerTokenKey, Value: BytesPerToken},
	}
}
