package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/processutil"
	"codenerd/internal/tools"
)

// verificationTimeoutSeconds reports how long a typed verification call may
// run. The default is 300s for builds and 600s for tests; timeout_seconds
// overrides it when valid. An invalid value is reported so the tool itself
// still fails; Timeout hooks fall back to the default instead.
func verificationTimeoutSeconds(args map[string]any, tests bool) (int, error) {
	seconds := 300
	if tests {
		seconds = 600
	}
	if value, exists := args["timeout_seconds"]; exists && value != nil {
		n, ok := coerceInt(value)
		if !ok || n <= 0 || n > 86400 {
			return 0, fmt.Errorf("invalid timeout_seconds")
		}
		seconds = n
	}
	return seconds, nil
}

// executeTypedVerification maps a typed request to a known project runner.
// Model strings are never interpreted by a shell or treated as executable flags.
func executeTypedVerification(ctx context.Context, args map[string]any, tests bool) (string, error) {
	if _, exists := args["command"]; exists {
		return "", fmt.Errorf("custom commands are not accepted; use working_dir, packages and test pattern")
	}
	dir, _ := args["working_dir"].(string)
	dir, err := resolveWorkingDir(ctx, dir)
	if err != nil {
		return "", err
	}
	detected, ok := tools.BuildCommandForDir(dir)
	if tests {
		detected, ok = tools.TestCommandForDir(dir)
	}
	if !ok {
		return "", fmt.Errorf("no supported project verification runner in %s", dir)
	}
	argv := strings.Fields(detected) // repository-owned mapping, never model input
	pattern, _ := args["pattern"].(string)
	if len(pattern) > 4096 || strings.ContainsRune(pattern, 0) {
		return "", fmt.Errorf("invalid test pattern")
	}
	if len(argv) == 0 {
		return "", fmt.Errorf("empty verification runner")
	}
	scope := "" // set when the package list was derived, so the result says what it covers
	count := 1
	if value, exists := args["count"]; exists && value != nil {
		n, ok := coerceInt(value)
		if !ok || n < 1 || n > 1000 {
			return "", fmt.Errorf("invalid count")
		}
		count = n
	}
	// goEnv is the environment a go runner executes under; nil (inherit the
	// process environment) for the other runners, whose toolchains the build
	// env's allowlist does not describe.
	var goEnv []string
	if argv[0] == "go" {
		isBuild := !tests && len(argv) > 1 && argv[1] == "build"
		argv = argv[:2]
		if tests {
			argv = append(argv, fmt.Sprintf("-count=%d", count))
			if race, _ := args["race"].(bool); race {
				argv = append(argv, "-race")
			}
			if pattern != "" {
				argv = append(argv, "-run", pattern)
			}
		}
		var packages []string
		switch v := args["packages"].(type) {
		case nil:
		case []string:
			packages = v
		case []any:
			for _, value := range v {
				s, ok := value.(string)
				if !ok {
					return "", fmt.Errorf("packages must contain strings")
				}
				packages = append(packages, s)
			}
		default:
			return "", fmt.Errorf("packages must be an array")
		}
		if len(packages) == 0 {
			packages = []string{"./..."}
			if written, scoped := writtenTestScope(ctx, dir, tests); scoped {
				if len(written) == 0 {
					return nothingWrittenToTest(dir), nil
				}
				packages, scope = written, "the Go packages written this turn; pass packages [\"./...\"] for the whole module"
			}
		}
		for _, pkg := range packages {
			if pkg != "." && !strings.HasPrefix(pkg, "./") {
				return "", fmt.Errorf("package must be workspace-relative: %q", pkg)
			}
			if strings.ContainsAny(pkg, "\\\x00\r\n") {
				return "", fmt.Errorf("invalid package: %q", pkg)
			}
			base := strings.TrimSuffix(pkg, "/...")
			clean := filepath.Clean(base)
			if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return "", fmt.Errorf("package escapes workspace")
			}
			if _, err := tools.ResolveWorkspacePath(tools.WithWorkspaceRoot(ctx, dir), "", base); err != nil {
				return "", err
			}
		}
		// The same environment and configured go_flags the session's own
		// verification gate compiles under, so the model and the gate cannot
		// disagree about one build -- and the process's API keys stay out of
		// a test binary that is project code.
		root, _ := tools.WorkspaceRoot(ctx)
		var goArgs []string
		goEnv, goArgs = build.GoInvocation(root, dir, argv[1:])
		argv = append([]string{argv[0]}, goArgs...)
		if isBuild {
			// A build checks that the code compiles and changes nothing on
			// disk. Output goes into a per-invocation temp directory that is
			// removed on return, so no binary is ever left behind in the
			// workspace — not even for a single main package, where plain
			// `go build` would otherwise drop (or, on Windows, rotate aside
			// and replace) an executable next to the sources. A directory,
			// not a file, because `go build -o` with a file target refuses
			// more than one package at once, while a directory target
			// accepts any package list: zero, one, or many main packages.
			seconds, err := verificationTimeoutSeconds(args, tests)
			if err != nil {
				return "", err
			}
			tmpDir, err := os.MkdirTemp("", "nerd-build-*")
			if err != nil {
				return "", fmt.Errorf("create temp build output: %w", err)
			}
			defer os.RemoveAll(tmpDir)
			run := func(buildArgv []string) (string, int, error) {
				runCtx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
				defer cancel()
				cmd := newCommand(runCtx, buildArgv[0], buildArgv[1:]...)
				cmd.Dir = dir
				cmd.Env = goEnv
				out, runErr := processutil.CombinedOutput(cmd)
				code := 0
				if cmd.ProcessState != nil {
					code = cmd.ProcessState.ExitCode()
				} else if runErr != nil {
					code = -1
				}
				if runCtx.Err() != nil {
					runErr = runCtx.Err()
				}
				return string(out), code, runErr
			}
			reported := append(append(append([]string{}, argv...), "-o", "<discarded-temp-dir>"), packages...)
			out, code, runErr := run(append(append(append([]string{}, argv...), "-o", tmpDir), packages...))
			if runErr != nil && strings.Contains(out, "no main packages to build") {
				// Nothing in the package list is linkable, so `go build -o`
				// refuses outright. A plain `go build` of such packages
				// compiles and discards everything — there is no binary to
				// emit — so it likewise changes nothing on disk.
				plain := append(append([]string{}, argv...), packages...)
				reported = plain
				out, code, runErr = run(plain)
			}
			data, _ := json.Marshal(struct {
				Argv      []string `json:"argv"`
				Directory string   `json:"directory"`
				ExitCode  int      `json:"exit_code"`
				Output    string   `json:"output"`
			}{reported, dir, code, out})
			return string(data), runErr
		}
		argv = append(argv, packages...)
	} else {
		if _, exists := args["packages"]; exists {
			return "", fmt.Errorf("packages currently requires a Go project")
		}
		if pattern != "" {
			switch argv[0] {
			case "pytest":
				argv = append(argv, "-k", pattern)
			case "cargo":
				argv = append(argv, "--", pattern)
			case "npm":
				argv = append(argv, "--", "--testNamePattern", pattern)
			default:
				return "", fmt.Errorf("test filtering unsupported for %s", argv[0])
			}
		}
	}
	seconds, err := verificationTimeoutSeconds(args, tests)
	if err != nil {
		return "", err
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()
	cmd := newCommand(runCtx, argv[0], argv[1:]...)
	cmd.Dir = dir
	if goEnv != nil {
		cmd.Env = goEnv
	}
	out, runErr := processutil.CombinedOutput(cmd)
	code := 0
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
		if tests {
			tools.RecordTestRun(ctx, tools.TestRun{Argv: argv, ExitCode: code})
		}
	} else if runErr != nil {
		code = -1
	}

	// The output is returned whole. A build or test log is exactly the kind
	// of result the working context archives and pages; cutting it here would
	// hand the model the head of a log whose failures are at the tail.
	data, _ := json.Marshal(struct {
		Argv      []string `json:"argv"`
		Directory string   `json:"directory"`
		Scope     string   `json:"scope,omitempty"`
		ExitCode  int      `json:"exit_code"`
		Output    string   `json:"output"`
	}{argv, dir, scope, code, string(out)})
	if runCtx.Err() != nil {
		runErr = runCtx.Err()
	}
	return string(data), runErr
}

// writtenTestScope returns the packages a test call with no packages should
// run. scoped is false when there is nothing to scope by -- a build, no session
// write set, or a working_dir below the workspace root, where "./..." is
// already the caller's own narrowing and the write set's workspace-relative
// packages would not resolve.
func writtenTestScope(ctx context.Context, dir string, tests bool) (packages []string, scoped bool) {
	if !tests {
		return nil, false
	}
	scope, ok := tools.TestScopeFrom(ctx)
	if !ok {
		return nil, false
	}
	root, err := tools.WorkspaceRoot(ctx)
	if err != nil || !sameDir(root, dir) {
		return nil, false
	}
	for _, pkg := range scope() {
		// A package written and then deleted this turn has nothing to test.
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(pkg))); err == nil && info.IsDir() {
			packages = append(packages, pkg)
		}
	}
	return packages, true
}

func sameDir(a, b string) bool {
	if ca, err := tools.CanonicalWorkspaceRoot(a); err == nil {
		a = ca
	}
	if cb, err := tools.CanonicalWorkspaceRoot(b); err == nil {
		b = cb
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// nothingWrittenToTest is the result of a test call with no packages in a turn
// that has written no Go package. Nothing ran, and the result says so in a
// field a reader cannot take for a pass: there is no exit_code.
func nothingWrittenToTest(dir string) string {
	data, _ := json.Marshal(struct {
		Ran       bool   `json:"ran"`
		Directory string `json:"directory"`
		Reason    string `json:"reason"`
	}{false, dir, "no tests ran: this turn has written no Go package, and with no packages given the tests of the written packages are what runs. " +
		"To test a package pass packages, e.g. [\"./internal/session\"]; for the whole module pass [\"./...\"] with a timeout_seconds the suite fits in."})
	return string(data)
}
