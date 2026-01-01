package config

import "encoding/json"

// ReflectionConfig configures System 2 reflection recall.
type ReflectionConfig struct {
	// Enabled controls whether reflection recall runs (default: true)
	Enabled bool `yaml:"enabled" json:"enabled"`

	// TopK is the max hits to consider per category (default: 5)
	TopK int `yaml:"top_k" json:"top_k"`

	// MinScore is the minimum weighted similarity score (0.0-1.0) (default: 0.70)
	MinScore float64 `yaml:"min_score" json:"min_score"`

	// RecencyHalfLifeDays controls decay weighting by age (default: 14 days)
	RecencyHalfLifeDays int `yaml:"recency_half_life_days" json:"recency_half_life_days"`

	// BacklogWatermark controls when the embedder prioritizes failures (default: 300)
	BacklogWatermark int `yaml:"backlog_watermark" json:"backlog_watermark"`

	enabledSet bool
}

// UnmarshalJSON tracks which boolean fields were explicitly set so defaults apply.
func (c *ReflectionConfig) UnmarshalJSON(data []byte) error {
	type alias ReflectionConfig
	aux := struct {
		Enabled *bool `json:"enabled"`
		*alias
	}{
		alias: (*alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Enabled != nil {
		c.Enabled = *aux.Enabled
		c.enabledSet = true
	}
	return nil
}

// DefaultReflectionConfig returns sensible defaults for reflection recall.
func DefaultReflectionConfig() ReflectionConfig {
	return ReflectionConfig{
		Enabled:              true,
		TopK:                 5,
		MinScore:             0.70,
		RecencyHalfLifeDays:  14,
		BacklogWatermark:     300,
	}
}
