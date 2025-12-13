package config

// LoggingConfig configures logging.
type LoggingConfig struct {
	Level      string          `yaml:"level" json:"level,omitempty"`           // debug, info, warn, error
	Format     string          `yaml:"format" json:"format,omitempty"`         // json, text
	File       string          `yaml:"file" json:"file,omitempty"`             // legacy single file
	DebugMode  bool            `yaml:"debug_mode" json:"debug_mode,omitempty"` // Master toggle - false = no logging (production)
	Categories map[string]bool `yaml:"categories" json:"categories,omitempty"` // Per-category toggles
}

// IsCategoryEnabled returns whether logging is enabled for a category.
// Returns false if debug_mode is false (production mode).
// Returns true if debug_mode is true and category is enabled (or not specified).
func (c *LoggingConfig) IsCategoryEnabled(category string) bool {
	if !c.DebugMode {
		return false
	}
	if c.Categories == nil {
		return true // All enabled by default in debug mode
	}
	enabled, exists := c.Categories[category]
	if !exists {
		return true // Enable by default if not specified
	}
	return enabled
}
