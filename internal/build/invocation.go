package build

import (
	"path/filepath"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/logging"
)

// GoInvocation prepares a `go` subcommand that codeNERD runs on a workspace's
// behalf -- the typed run_build / run_tests tools and the impacted-test runner:
// the environment it runs under, and its argv with the workspace's configured
// build.go_flags injected.
//
// Those runners used to spawn `go` with the process environment untouched.
// Two things followed. A project whose build needs CGO headers failed there
// with a compile error the model read as a test failure, while the session's
// own verification gate -- which does use this package -- passed the same
// tree; the model and the gate disagreed about one build. And the test binary,
// which is project code, inherited every variable the process held, API keys
// included, where GetBuildEnv deliberately passes an allowlist.
//
// workspaceRoot is where .nerd/config.json lives. dir is the command's working
// directory and may be a nested module; the header-detection root is found by
// walking up from it (DetectionRootFor) but never past the workspace, because
// a sqlite_headers above the workspace root is not the workspace's to adopt.
//
// args excludes the "go" binary itself: ["test", "-count=1", "./..."]. A test
// subcommand gets GetBuildEnvForTest, every other one GetBuildEnv.
func GoInvocation(workspaceRoot, dir string, args []string) (env []string, argv []string) {
	userCfg := WorkspaceUserConfig(workspaceRoot)
	detection := detectionRootWithin(workspaceRoot, dir)
	if len(args) > 0 && args[0] == "test" {
		env = GetBuildEnvForTest(userCfg, detection)
	} else {
		env = GetBuildEnv(userCfg, detection)
	}
	return env, AppendGoFlags(userCfg, detection, args)
}

// WorkspaceUserConfig loads <workspaceRoot>/.nerd/config.json for a caller
// that spawns go on the workspace's behalf but was handed no UserConfig. A
// missing file is an empty config. An unreadable or invalid one is reported and
// treated as absent: the command still runs, under the unconfigured build env,
// and the log says why its configured flags did not apply.
func WorkspaceUserConfig(workspaceRoot string) *config.UserConfig {
	if strings.TrimSpace(workspaceRoot) == "" {
		return nil
	}
	path := filepath.Join(workspaceRoot, ".nerd", "config.json")
	cfg, err := config.LoadUserConfig(path)
	if err != nil {
		logging.Get(logging.CategoryBuild).Warn(
			"build env: %s could not be loaded, so its build settings do not apply to this go command: %v", path, err)
		return nil
	}
	return cfg
}

// detectionRootWithin is DetectionRootFor(dir) bounded by workspaceRoot. With
// no workspace root there is no bound, and the walk's own repository boundary
// is the only one.
func detectionRootWithin(workspaceRoot, dir string) string {
	if strings.TrimSpace(dir) == "" {
		dir = workspaceRoot
	}
	detection := DetectionRootFor(dir)
	if strings.TrimSpace(workspaceRoot) == "" {
		return detection
	}
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return detection
	}
	rel, err := filepath.Rel(root, detection)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return root
	}
	return detection
}
