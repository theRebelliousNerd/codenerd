package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/tools"
)

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
	count := 1
	if value, exists := args["count"]; exists && value != nil {
		n, ok := coerceInt(value)
		if !ok || n < 1 || n > 1000 {
			return "", fmt.Errorf("invalid count")
		}
		count = n
	}
	if argv[0] == "go" {
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
			argv = append(argv, pkg)
		}
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
	seconds := 300
	if tests {
		seconds = 600
	}
	if value, exists := args["timeout_seconds"]; exists && value != nil {
		n, ok := coerceInt(value)
		if !ok || n <= 0 || n > 86400 {
			return "", fmt.Errorf("invalid timeout_seconds")
		}
		seconds = n
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()
	cmd := newCommand(runCtx, argv[0], argv[1:]...)
	cmd.Dir = dir
	out, runErr := cmd.CombinedOutput()
	code := 0
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	} else if runErr != nil {
		code = -1
	}
	// The output is returned whole. A build or test log is exactly the kind
	// of result the working context archives and pages; cutting it here would
	// hand the model the head of a log whose failures are at the tail.
	data, _ := json.Marshal(struct {
		Argv      []string `json:"argv"`
		Directory string   `json:"directory"`
		ExitCode  int      `json:"exit_code"`
		Output    string   `json:"output"`
	}{argv, dir, code, string(out)})
	if runCtx.Err() != nil {
		runErr = runCtx.Err()
	}
	return string(data), runErr
}
