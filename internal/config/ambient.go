package config

import (
	"fmt"
	"time"
)

// AmbientConfig bounds how the chat reads .nerd/ambient.json, the editor
// bridge's report of the user's active file, cursor, selection and
// diagnostics (types.AmbientContext).
type AmbientConfig struct {
	// MaxAgeSeconds is how old the file may be and still describe the
	// editor (default: 900). An older file means the bridge stopped writing
	// it, and the chat leaves the ambient block out.
	MaxAgeSeconds int `json:"max_age_seconds,omitempty"`
}

// DefaultAmbientConfig returns the ambient section's defaults.
func DefaultAmbientConfig() AmbientConfig {
	return AmbientConfig{MaxAgeSeconds: 900}
}

// GetAmbientConfig returns the ambient section with defaults applied.
func (c *UserConfig) GetAmbientConfig() AmbientConfig {
	if c == nil || c.Ambient == nil {
		return DefaultAmbientConfig()
	}
	return c.Ambient.WithDefaults()
}

// WithDefaults fills every absent field from DefaultAmbientConfig.
func (c AmbientConfig) WithDefaults() AmbientConfig {
	if c.MaxAgeSeconds == 0 {
		c.MaxAgeSeconds = DefaultAmbientConfig().MaxAgeSeconds
	}
	return c
}

// MaxAge is MaxAgeSeconds as a duration.
func (c AmbientConfig) MaxAge() time.Duration {
	return time.Duration(c.MaxAgeSeconds) * time.Second
}

// Check reports the contradictions in an ambient section, addressed under
// prefix ("ambient").
func (c AmbientConfig) Check(prefix string) []Problem {
	if c.MaxAgeSeconds < 0 {
		return []Problem{{
			Severity: SeverityError,
			Path:     prefix + ".max_age_seconds",
			Message:  fmt.Sprintf("%d is negative", c.MaxAgeSeconds),
			Fix:      "an age in seconds, or remove the key for the default",
		}}
	}
	return nil
}
