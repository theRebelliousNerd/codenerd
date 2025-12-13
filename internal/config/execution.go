package config

// ExecutionConfig configures the tactile interface.
type ExecutionConfig struct {
	// Allowed binaries (Constitutional Logic)
	AllowedBinaries []string `yaml:"allowed_binaries" json:"allowed_binaries,omitempty"`

	// Default timeout for commands
	DefaultTimeout string `yaml:"default_timeout" json:"default_timeout,omitempty"`

	// Working directory
	WorkingDirectory string `yaml:"working_directory" json:"working_directory,omitempty"`

	// Environment variables to pass
	AllowedEnvVars []string `yaml:"allowed_env_vars" json:"allowed_env_vars,omitempty"`
}
