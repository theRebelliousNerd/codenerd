package config

// BuildConfig configures the environment for `go build` / `go test` subprocesses.
//
// This is the single definition of the shape. internal/build used to declare an
// identical struct and copy field-by-field between the two; they drifted (only
// this one ever had yaml tags) and every new field had to be added twice.
// internal/build now aliases this type, so the persisted shape and the in-memory
// shape cannot diverge.
type BuildConfig struct {
	// EnvVars are additional environment variables for builds.
	// Key examples: CGO_CFLAGS, CGO_LDFLAGS, CGO_ENABLED, CC, CXX
	EnvVars map[string]string `yaml:"env_vars" json:"env_vars,omitempty"`

	// GoFlags are additional flags for go build/test commands.
	// Consumed by build.AppendGoFlags, which injects them after the subcommand.
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
