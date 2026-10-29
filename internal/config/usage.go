package config

import (
	"fmt"
	"sort"
	"strings"
)

// UsageConfig configures token metering (internal/usage).
type UsageConfig struct {
	// Prices adds or overrides list prices, keyed by model-name prefix, in USD
	// per million tokens: negotiated rates, or models the built-in table does
	// not know (their tokens are otherwise reported as unpriced). The longest
	// matching prefix wins, as in the built-in table.
	Prices map[string]UsagePrice `json:"prices,omitempty"`

	// EventLog keeps a bounded ring of the most recent metered calls in
	// .nerd/usage.json, listed by `nerd usage --events`. Off by default: the
	// ring roughly doubles the file, and the aggregates are the record.
	EventLog bool `json:"event_log"`
}

// UsagePrice is one price entry: USD per million input and output tokens.
type UsagePrice struct {
	InputPerMTok  float64 `json:"input_per_mtok"`
	OutputPerMTok float64 `json:"output_per_mtok"`
}

// GetUsageConfig returns the usage section, or its zero value (built-in
// prices, no event log) when the file has none.
func (c *UserConfig) GetUsageConfig() UsageConfig {
	if c == nil || c.Usage == nil {
		return UsageConfig{}
	}
	return *c.Usage
}

// Check reports price entries that cannot be applied.
func (c UsageConfig) Check(prefix string) []Problem {
	keys := make([]string, 0, len(c.Prices))
	for k := range c.Prices {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out []Problem
	for _, k := range keys {
		p := c.Prices[k]
		if strings.TrimSpace(k) == "" {
			out = append(out, Problem{
				Severity: SeverityError,
				Path:     prefix + ".prices",
				Message:  "an empty model prefix would price every model",
				Fix:      "key each price by a model-name prefix such as \"gpt-4o\"",
			})
			continue
		}
		if p.InputPerMTok < 0 || p.OutputPerMTok < 0 {
			out = append(out, Problem{
				Severity: SeverityError,
				Path:     fmt.Sprintf("%s.prices.%s", prefix, k),
				Message:  fmt.Sprintf("negative price (input %g, output %g)", p.InputPerMTok, p.OutputPerMTok),
				Fix:      "USD per million tokens, 0 or more",
			})
		}
	}
	return out
}
