package gates

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codenerd/internal/build"
)

// Result is one gate run.
type Result struct {
	Gate Gate
	// Node is the node the gate ran for; "" for a workspace-scoped run.
	Node string
	// Argv is the command as run, placeholders expanded.
	Argv []string
	// ExitCode is the process's exit code, or -1 when it did not run to an
	// exit (it could not start, or the context stopped it).
	ExitCode int
	// Output is combined stdout and stderr, bounded by OutputLimit.
	Output string
	// Passed is the gate's verdict: it ran and its exit code is a pass.
	Passed bool
	// Err is why the gate produced no verdict: the program would not start, or
	// the run was cancelled. A Result with Err set is unverified, not failed.
	Err      error
	Duration time.Duration
}

// Unverified reports whether the run produced no verdict.
func (r Result) Unverified() bool { return r.Err != nil }

// OutputLimit bounds the output a Result keeps. Past it the head and the tail
// are kept -- a Go test run names the package at the top and the verdict at
// the bottom -- and the middle is marked as dropped.
var OutputLimit = 1 << 20

// passEnv are the ambient variables every gate keeps. A gate runs project
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

// Run runs g in root for node ("" or "." for the root node) and returns its
// verdict. It never returns a pass it did not observe: a program that will not
// start, or a run the context stopped, comes back Unverified.
func Run(ctx context.Context, root string, g Gate, node string) Result {
	res := Result{Gate: g, ExitCode: -1}
	if g.Scope == ScopeNode {
		res.Node = node
		res.Argv = g.ForNode(node)
	} else {
		res.Argv = append([]string(nil), g.Argv...)
	}
	if len(res.Argv) == 0 {
		res.Err = errors.New("empty command")
		return res
	}

	cmd := command(ctx, root, res.Argv)
	cmd.Dir = root
	for _, k := range sortedEnvKeys(g.Env) {
		cmd.Env = setEnv(cmd.Env, k, g.Env[k])
	}
	out := &boundedBuffer{limit: OutputLimit}
	cmd.Stdout = out
	cmd.Stderr = out

	start := time.Now()
	err := cmd.Run()
	res.Duration = time.Since(start)
	res.Output = out.String()

	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		res.Err = fmt.Errorf("stopped: %w", ctx.Err())
	case err == nil:
		res.ExitCode = 0
	case errors.As(err, &exitErr) && exitErr.Exited():
		res.ExitCode = exitErr.ExitCode()
	default:
		res.Err = err
	}
	res.Passed = res.Err == nil && g.Passed(res.ExitCode)
	return res
}

// command builds the process for argv. `go` goes through internal/build, like
// every go the harness spawns, so a gate and the session's own build agree on
// the environment and flags. Everything else gets the allowlisted
// environment. A workspace-relative program (./scripts/check.sh) is resolved
// against root, not against this process's directory.
func command(ctx context.Context, root string, argv []string) *exec.Cmd {
	if argv[0] == "go" {
		env, args := build.GoInvocation(root, root, argv[1:])
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Env = env
		return cmd
	}
	prog := argv[0]
	if strings.ContainsAny(prog, `/\`) && !filepath.IsAbs(prog) {
		prog = filepath.Join(root, prog)
	}
	cmd := exec.CommandContext(ctx, prog, argv[1:]...)
	cmd.Env = allowedEnv(root)
	return cmd
}

func allowedEnv(root string) []string {
	var env []string
	keys := append([]string(nil), passEnv...)
	if cfg := build.WorkspaceUserConfig(root); cfg != nil {
		keys = append(keys, cfg.GetExecution().AllowedEnvVars...)
	}
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			env = setEnv(env, k, v)
		}
	}
	return env
}

// setEnv sets k in env, replacing an existing entry. Windows variable names
// are case-insensitive, so the comparison is too; elsewhere PATH and Path are
// different variables but no toolchain here depends on the difference.
func setEnv(env []string, k, v string) []string {
	for i, kv := range env {
		if name, _, ok := strings.Cut(kv, "="); ok && strings.EqualFold(name, k) {
			env[i] = k + "=" + v
			return env
		}
	}
	return append(env, k+"="+v)
}

func sortedEnvKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// boundedBuffer keeps the first and last limit/2 bytes written to it.
type boundedBuffer struct {
	limit   int
	head    bytes.Buffer
	tail    []byte
	dropped int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	half := b.limit / 2
	if room := half - b.head.Len(); room > 0 {
		take := min(room, len(p))
		b.head.Write(p[:take])
		p = p[take:]
	}
	if len(p) == 0 {
		return n, nil
	}
	b.tail = append(b.tail, p...)
	if over := len(b.tail) - (b.limit - half); over > 0 {
		b.dropped += over
		b.tail = append(b.tail[:0], b.tail[over:]...)
	}
	return n, nil
}

func (b *boundedBuffer) String() string {
	if b.dropped == 0 {
		return b.head.String() + string(b.tail)
	}
	return fmt.Sprintf("%s\n... [%d bytes dropped] ...\n%s", b.head.String(), b.dropped, b.tail)
}
