package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// passEnv are the ambient variables every workspace-declared command keeps (a
// gate, a generator). Such a command runs project
// code (its tests), and the process running it may hold credentials, so the
// environment is an allowlist like the one Go builds get; these are what the
// toolchains the detector knows need to find themselves on each platform.
// nerd.md commands.env and .nerd/config.json execution.allowed_env_vars add
// to it.
var passEnv = []string{
	"PATH", "HOME", "USER", "LOGNAME", "SHELL", "LANG", "LC_ALL", "LC_CTYPE", "TERM",
	"TEMP", "TMP", "TMPDIR",
	"USERPROFILE", "APPDATA", "LOCALAPPDATA", "SystemRoot", "SystemDrive", "WINDIR", "COMSPEC", "PATHEXT",
	"ProgramData", "ProgramFiles", "ProgramFiles(x86)",
	"GOPATH", "GOROOT", "GOCACHE", "GOMODCACHE", "GOFLAGS",
	"VIRTUAL_ENV", "CONDA_PREFIX", "PYTHONPATH",
	"NODE_PATH", "NVM_DIR", "PNPM_HOME",
	"CARGO_HOME", "RUSTUP_HOME",
}

// WorkspaceCommand is the process for a command the workspace declares (a
// nerd.md gate or generator) in root, unstarted. `go` goes through
// GoInvocation, like every go the harness spawns, so a declared command and
// the session's own build agree on the environment and flags. Everything else
// gets the allowlisted environment (AllowedEnv). A workspace-relative program
// (./scripts/check.sh) is resolved against root, not against this process's
// directory. env (nerd.md commands.env) is overlaid last.
func WorkspaceCommand(ctx context.Context, root string, argv []string, env map[string]string) *exec.Cmd {
	var cmd *exec.Cmd
	if argv[0] == "go" {
		goEnv, args := GoInvocation(root, root, argv[1:])
		cmd = exec.CommandContext(ctx, "go", args...)
		cmd.Env = goEnv
	} else {
		prog := argv[0]
		if strings.ContainsAny(prog, `/\`) && !filepath.IsAbs(prog) {
			prog = filepath.Join(root, prog)
		}
		cmd = exec.CommandContext(ctx, prog, argv[1:]...)
		cmd.Env = AllowedEnv(root)
	}
	cmd.Dir = root
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		cmd.Env = setEnvVar(cmd.Env, k, env[k])
	}
	return cmd
}

// AllowedEnv is the allowlisted ambient environment for a workspace-declared
// command: passEnv plus the workspace's execution.allowed_env_vars.
func AllowedEnv(root string) []string {
	var env []string
	keys := append([]string(nil), passEnv...)
	if cfg := WorkspaceUserConfig(root); cfg != nil {
		keys = append(keys, cfg.GetExecution().AllowedEnvVars...)
	}
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			env = setEnvVar(env, k, v)
		}
	}
	return env
}

func setEnvVar(env []string, k, v string) []string {
	for i, kv := range env {
		if name, _, ok := strings.Cut(kv, "="); ok && strings.EqualFold(name, k) {
			env[i] = k + "=" + v
			return env
		}
	}
	return append(env, k+"="+v)
}
