package regression

import (
	"fmt"
	"os"
	"path/filepath"
)

// TemplateBattery is the starter suite written into a fresh workspace.
//
// It runs the project rather than a placeholder that always passes: a seeded
// battery whose only task is `echo ok` teaches the operator that batteries are
// decorative. Every task carries an explicit expectation so a tool that exits 0
// while printing a failure still fails the suite.
const TemplateBattery = `# codeNERD regression battery
#
# Each task runs in a non-login shell (no profile/rc) so results do not depend
# on the operator's dotfiles. Run with: nerd regression run
version: 1
tasks:
  - id: build
    type: shell
    command: go build ./...
    timeout_sec: 600
    expect_exit: 0

  - id: vet
    type: shell
    command: go vet ./...
    timeout_sec: 600
    expect_exit: 0

  - id: unit-tests
    type: shell
    command: go test ./internal/... 2>&1
    timeout_sec: 1800
    expect_exit: 0
    expect_not_contains:
      - "panic:"
      - "DATA RACE"
`

// Seed writes the starter battery under .nerd/regression/ when the workspace
// has none, and reports the path plus whether it created the file.
//
// An existing battery is never overwritten and never treated as an error: the
// operator's suite is the thing this package exists to protect, and Seed is
// meant to be safe to call from every `nerd init`, including `--force`, which
// reinitializes a workspace that already has one.
func Seed(workspace string) (path string, created bool, err error) {
	path = DefaultBatteryPath(workspace)

	if _, statErr := os.Stat(path); statErr == nil {
		return path, false, nil
	} else if !os.IsNotExist(statErr) {
		return path, false, fmt.Errorf("stat battery: %w", statErr)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return path, false, fmt.Errorf("create regression dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(TemplateBattery), 0644); err != nil {
		return path, false, fmt.Errorf("write battery: %w", err)
	}
	return path, true, nil
}
