package config

// BuildConfig configures build environment for go build/test commands.
// This ensures all components (preflight, thunderdome, ouroboros, attack_runner)
// use consistent environment variables like CGO_CFLAGS.
type BuildConfig struct {
	// EnvVars are additional environment variables for builds.
	// Key examples: CGO_CFLAGS, CGO_LDFLAGS, CGO_ENABLED, CC, CXX
	EnvVars map[string]string `yaml:"env_vars" json:"env_vars,omitempty"`

	// GoFlags are additional flags for go build/test commands.
	GoFlags []string `yaml:"go_flags" json:"go_flags,omitempty"`

	// CGOPackages lists packages that require CGO (for documentation/detection).
	CGOPackages []string `yaml:"cgo_packages" json:"cgo_packages,omitempty"`
}

// DefaultBuildConfig returns sensible defaults.
func DefaultBuildConfig() BuildConfig {
	return BuildConfig{
		EnvVars:     make(map[string]string),
		GoFlags:     []string{},
		CGOPackages: []string{},
	}
}
