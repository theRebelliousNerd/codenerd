package tools

import (
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"

	"codenerd/internal/config"
)

// A secret path is a file whose contents must never reach a model. Everything
// a tool returns is sent to the model's provider, and a workspace holds its own
// keys: .env, .nerd/config.json. Until 2026-09-22 nothing stood between a model
// and those files -- read_file was a safe_action with no condition on its
// target, and grep walked hidden files -- while the not-found hint listed .env
// among its suggestions for any missing name in the workspace root.
//
// This is the one matcher. The session executor asks it before the
// constitution is queried (it asserts touches_secret_path, which the policy
// turns into dangerous_content, so permitted cannot derive), and the search
// tools ask it to leave secret files out of a walk that has no single target
// for the policy to judge. The patterns are execution.secret_paths in
// config.json.

// secretPatterns holds the patterns in force, lower-cased. Nil until boot sets
// them, and IsSecretPath then uses the documented defaults: a guard that goes
// quiet because a caller never reached the setter is worse than one that
// guards with the default list.
var secretPatterns atomic.Pointer[[]string]

// SetSecretPathPatterns installs the configured patterns. Called once at boot
// with ExecutionConfig.ResolvedSecretPaths(). An empty, non-nil list is the
// user's explicit "no secret paths" and is honoured.
func SetSecretPathPatterns(patterns []string) {
	lowered := make([]string, 0, len(patterns))
	for _, p := range patterns {
		if p = strings.TrimSpace(p); p != "" {
			lowered = append(lowered, strings.ToLower(filepath.ToSlash(p)))
		}
	}
	secretPatterns.Store(&lowered)
}

func activeSecretPatterns() []string {
	if p := secretPatterns.Load(); p != nil {
		return *p
	}
	defaults := config.DefaultSecretPaths()
	for i := range defaults {
		defaults[i] = strings.ToLower(defaults[i])
	}
	return defaults
}

// IsSecretPath reports whether p names a secret file. p may be absolute,
// relative, or slash- or backslash-separated. A pattern matches the file's
// base name ("*.pem", ".env") or any trailing run of its path segments
// (".nerd/config.json" matches /any/where/.nerd/config.json), always
// case-insensitively: on Windows .ENV is the same file as .env.
func IsSecretPath(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" {
		return false
	}
	clean := strings.ToLower(filepath.ToSlash(filepath.Clean(p)))
	segments := strings.Split(strings.TrimPrefix(clean, "/"), "/")
	for _, pattern := range activeSecretPatterns() {
		depth := strings.Count(pattern, "/") + 1
		if depth > len(segments) {
			continue
		}
		tail := strings.Join(segments[len(segments)-depth:], "/")
		if ok, err := path.Match(pattern, tail); err == nil && ok {
			return true
		}
	}
	return false
}

// SecretPathInCommand reports the first whitespace- or quote-delimited token
// of a shell command line that names a secret file, if any. It is a direct-
// reference check, not a sandbox: a command that assembles the name at run
// time is not caught here, and the shell's binary allowlist remains the gate
// for that.
func SecretPathInCommand(command string) (string, bool) {
	fields := strings.FieldsFunc(command, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', '"', '\'', '`', '=', '<', '>', '|', ';', '&', '(', ')', ',':
			return true
		}
		return false
	})
	for _, f := range fields {
		if IsSecretPath(f) {
			return f, true
		}
	}
	return "", false
}
