package config

// ToolGenerationConfig configures the Ouroboros tool generation settings.
type ToolGenerationConfig struct {
	TargetOS   string `yaml:"target_os" json:"target_os"`     // e.g., "windows", "linux", "darwin"
	TargetArch string `yaml:"target_arch" json:"target_arch"` // e.g., "amd64", "arm64"
}

// DefaultToolGenerationConfig returns default tool generation targets.
func DefaultToolGenerationConfig() ToolGenerationConfig {
	return ToolGenerationConfig{
		TargetOS:   "windows",
		TargetArch: "amd64",
	}
}
