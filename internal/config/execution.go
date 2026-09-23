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

	// SecretPaths are the path patterns whose contents never reach a model:
	// no tool may read, write or search them, and a shell command that names
	// one is refused. A pattern is matched (path.Match, case-insensitive)
	// against a file's base name and against its workspace-relative path.
	// Absent means the defaults below; an explicit [] means none -- the
	// user's decision to make, never the code's.
	//
	// Whatever a tool returns is sent to the model's provider. A workspace
	// holds its own keys (.env, .nerd/config.json), and until 2026-09-22 a
	// read_file or a directory-wide grep of them went through: read_file was
	// a safe_action with no condition on its target.
	SecretPaths []string `yaml:"secret_paths" json:"secret_paths,omitempty"`
}

// DefaultSecretPaths is what secret_paths means when config.json does not say.
func DefaultSecretPaths() []string {
	return []string{
		".env", ".env.*", "*.env",
		".nerd/config.json",
		"*.pem", "*.key", "*.p12", "*.pfx", "*.jks", "*.keystore", "*.kdbx",
		"id_rsa*", "id_dsa*", "id_ecdsa*", "id_ed25519*",
		".netrc", ".npmrc", ".pypirc", ".git-credentials",
		// Credential files by their real shapes (AWS's bare file, gcloud's
		// JSON), not "credentials*": that would refuse credentials.go.
		"credentials", "credentials.json", "*_credentials.json",
	}
}

// ResolvedSecretPaths is the list in force: the configured one when the key
// is present (even empty), the defaults when it is absent.
func (c *ExecutionConfig) ResolvedSecretPaths() []string {
	if c == nil || c.SecretPaths == nil {
		return DefaultSecretPaths()
	}
	return c.SecretPaths
}

// DefaultExecutionConfig returns an ExecutionConfig with sensible defaults.
func DefaultExecutionConfig() *ExecutionConfig {
	return &ExecutionConfig{
		AllowedBinaries: []string{
			"go", "git", "grep", "ls", "mkdir", "cp", "mv",
			"npm", "npx", "node", "python", "python3", "pip",
			"cargo", "rustc", "make", "cmake",
		},
		DefaultTimeout:   "30s",
		WorkingDirectory: ".",
		AllowedEnvVars: []string{
			"PATH", "HOME", "GOPATH", "GOROOT",
			"TEMP", "TMP", "GOCACHE", "LOCALAPPDATA",
		},
		SecretPaths: DefaultSecretPaths(),
	}
}
