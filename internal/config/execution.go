package config

import "sync/atomic"

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

	// MaxReadFileBytes is the process ceiling for a VirtualStore file read.
	// 0 reads the file whole: the observation codec projects what the model
	// sees and retains the rest under the read handle. A positive value
	// refuses the read. It must not return a prefix — the old 100KB cut made
	// both the file_content fact and the edit precondition a prefix of the file.
	MaxReadFileBytes int64 `yaml:"max_read_file_bytes" json:"max_read_file_bytes,omitempty"`

	// MaxSearchFileBytes is how large a file code search will load.
	// The walk reads each visited file whole, so an unchecked bundle or
	// binary becomes a multi-hundred-MB read on every search. 0 means this
	// default. Files over the bound are not matches that were dropped: the
	// result says how many were skipped.
	MaxSearchFileBytes int64 `yaml:"max_search_file_bytes" json:"max_search_file_bytes,omitempty"`

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
		// Google's downloads: the OAuth client file is named client_secret_<id>.json
		// and a service-account key is saved under a name the user picks, which
		// in practice carries "service-account"/"service_account". A workspace's
		// .credentials/ or .secrets/ directory holds keys under any name.
		"client_secret*.json", "*service-account*.json", "*service_account*.json",
		".credentials/*", ".secrets/*",
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
		SecretPaths:        DefaultSecretPaths(),
		MaxReadFileBytes:   0,
		MaxSearchFileBytes: 1 << 20,
	}
}

// Process-wide file ceilings installed by LoadUserConfig, the same way LLM
// timeouts are installed: VirtualStore reads them without opening the config
// file itself. Tests and a store with an explicit VirtualStoreConfig field
// override these.
var (
	resolvedMaxReadFileBytes   atomic.Int64
	resolvedMaxSearchFileBytes atomic.Int64
)

func init() {
	SetExecutionFileLimits(*DefaultExecutionConfig())
}

// SetExecutionFileLimits installs the file-read and code-search ceilings.
// A non-positive search bound takes the default; a negative read bound is
// treated as 0 (read the file whole). LoadUserConfig is the production caller.
func SetExecutionFileLimits(c ExecutionConfig) {
	read := c.MaxReadFileBytes
	if read < 0 {
		read = 0
	}
	search := c.MaxSearchFileBytes
	if search <= 0 {
		search = DefaultExecutionConfig().MaxSearchFileBytes
	}
	resolvedMaxReadFileBytes.Store(read)
	resolvedMaxSearchFileBytes.Store(search)
}

// ResolvedMaxReadFileBytes is the installed read ceiling. 0 means no ceiling.
func ResolvedMaxReadFileBytes() int64 { return resolvedMaxReadFileBytes.Load() }

// ResolvedMaxSearchFileBytes is the installed code-search file ceiling.
func ResolvedMaxSearchFileBytes() int64 { return resolvedMaxSearchFileBytes.Load() }
