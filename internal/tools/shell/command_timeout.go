package shell

import (
	"path/filepath"
	"strings"
)

// Default run_command timeouts, in seconds, when the caller passes no
// timeout_seconds.
//
// The flat 60-second default was the single most common reason codeNERD
// could not verify its own work: `go build ./...` and `go test ./<pkg>/`
// in this repository routinely take one to several minutes on a cold cache
// or a loaded machine, and four dogfood runs on 2026-09-04 reported "go test
// timed out with no output" while the code under test was fine. A model that
// forgets timeout_seconds should still get a verification result, so
// toolchain build/test invocations default to ten minutes. Everything else
// keeps the short default: an unknown command that hangs should fail fast.
const (
	defaultShortTimeoutSeconds     = 60
	defaultToolchainTimeoutSeconds = 600
)

// toolchainSubcommands lists, per toolchain binary, the subcommands that
// compile or run tests and therefore deserve the long default.
var toolchainSubcommands = map[string]map[string]bool{
	"go":    {"build": true, "test": true, "vet": true, "install": true, "generate": true, "run": true, "mod": true},
	"cargo": {"build": true, "test": true, "check": true, "clippy": true, "run": true},
	"npm":   {"test": true, "run": true, "ci": true, "install": true},
	"pnpm":  {"test": true, "run": true, "install": true},
	"yarn":  {"test": true, "run": true, "install": true},
	"mvn":   {"test": true, "package": true, "verify": true, "install": true, "compile": true},
	"dotnet": {"build": true, "test": true, "restore": true},
}

// standaloneToolchainCommands are build/test drivers whose first token alone
// identifies a long-running invocation.
var standaloneToolchainCommands = map[string]bool{
	"make": true, "pytest": true, "gradle": true, "gradlew": true, "tsc": true,
}

// defaultCommandTimeout returns the default timeout for command in seconds.
// The first token is matched by base name with any .exe suffix removed, so
// "C:\Go\bin\go.exe test ./..." and "go test ./..." are treated alike.
func defaultCommandTimeout(command string) int {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return defaultShortTimeoutSeconds
	}
	bin := strings.ToLower(strings.TrimSuffix(filepath.Base(filepath.ToSlash(fields[0])), ".exe"))
	if standaloneToolchainCommands[bin] {
		return defaultToolchainTimeoutSeconds
	}
	subs, ok := toolchainSubcommands[bin]
	if !ok || len(fields) < 2 {
		return defaultShortTimeoutSeconds
	}
	if subs[strings.ToLower(fields[1])] {
		return defaultToolchainTimeoutSeconds
	}
	return defaultShortTimeoutSeconds
}
