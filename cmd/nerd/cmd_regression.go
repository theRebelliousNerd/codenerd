package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/regression"

	"github.com/spf13/cobra"
)

var (
	regressionFile     string
	regressionContinue bool
	regressionJSON     bool
	regressionLogin    bool
	regressionNoSave   bool
)

// exampleBattery seeds a new workspace with a battery that actually exercises
// the project rather than a placeholder that always passes.
const exampleBattery = `# codeNERD regression battery
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

var regressionCmd = &cobra.Command{
	Use:   "regression",
	Short: "Run and inspect regression batteries",
	Long: `Regression batteries are YAML-defined shell suites kept under
.nerd/regression/. They exist to answer one question repeatedly and identically:
does this workspace still behave.

Tasks run in a non-login shell so a battery result never depends on the
operator's dotfiles.`,
}

var regressionRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute the workspace regression battery",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := workspaceRootOrCwd()

		path := regressionFile
		if path == "" {
			path = regression.DefaultBatteryPath(root)
		}

		battery, err := regression.LoadBattery(path)
		if err != nil {
			// Point at `init` rather than just failing: a missing battery is
			// the common first-run case, not a broken workspace.
			if os.IsNotExist(err) {
				return fmt.Errorf("no battery at %s — run `nerd regression init` to create one", path)
			}
			return err
		}

		summary, err := regression.RunBatteryWithOptions(context.Background(), battery, regression.RunOptions{
			Workdir:           root,
			ContinueOnFailure: regressionContinue,
			LoginShell:        regressionLogin,
		})
		if err != nil {
			return err
		}

		if !regressionNoSave {
			if runPath, saveErr := regression.SaveRun(root, summary); saveErr != nil {
				// A failure to persist must not change the verdict of the run.
				fmt.Fprintf(os.Stderr, "warning: could not persist run record: %v\n", saveErr)
			} else if !regressionJSON {
				defer fmt.Printf("\nRun record: %s\n", runPath)
			}
		}

		if regressionJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(summary); err != nil {
				return err
			}
		} else {
			fmt.Print(regression.FormatSummary(summary))
		}

		if !summary.OK() {
			// A non-zero exit lets CI gate on this without parsing output.
			return fmt.Errorf("regression battery failed: %d failed, %d skipped",
				summary.Failed, summary.Skipped)
		}
		return nil
	},
}

var regressionInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a starter battery to .nerd/regression/battery.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := workspaceRootOrCwd()
		path := regression.DefaultBatteryPath(root)

		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("battery already exists at %s (delete it first to regenerate)", path)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("create regression dir: %w", err)
		}
		if err := os.WriteFile(path, []byte(exampleBattery), 0644); err != nil {
			return fmt.Errorf("write battery: %w", err)
		}

		fmt.Printf("Wrote starter battery to %s\n", path)
		fmt.Println("Edit it, then run: nerd regression run")
		return nil
	},
}

var regressionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List persisted regression run records, newest first",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := workspaceRootOrCwd()

		runs, err := regression.ListRuns(root)
		if err != nil {
			return err
		}
		if len(runs) == 0 {
			fmt.Printf("No regression runs recorded under %s\n", regression.RunsDir(root))
			return nil
		}

		for _, path := range runs {
			raw, err := os.ReadFile(path)
			if err != nil {
				fmt.Printf("%s  (unreadable: %v)\n", filepath.Base(path), err)
				continue
			}
			var summary regression.Summary
			if err := json.Unmarshal(raw, &summary); err != nil {
				fmt.Printf("%s  (unparseable: %v)\n", filepath.Base(path), err)
				continue
			}
			verdict := "PASS"
			if !summary.OK() {
				verdict = "FAIL"
			}
			fmt.Printf("%s  %s  %d passed, %d failed, %d skipped  (%dms)\n",
				filepath.Base(path), verdict, summary.Passed, summary.Failed, summary.Skipped, summary.DurationMs)
		}
		return nil
	},
}

// workspaceRootOrCwd resolves the --workspace flag, falling back to the
// current directory.
func workspaceRootOrCwd() string {
	if workspace != "" {
		return workspace
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

func init() {
	regressionRunCmd.Flags().StringVarP(&regressionFile, "file", "f", "",
		"battery file to run (default .nerd/regression/battery.yaml)")
	regressionRunCmd.Flags().BoolVar(&regressionContinue, "continue", false,
		"run every task even after one fails")
	regressionRunCmd.Flags().BoolVar(&regressionJSON, "json", false,
		"emit the run summary as JSON")
	regressionRunCmd.Flags().BoolVar(&regressionLogin, "login-shell", false,
		"use a profile-loading login shell (non-deterministic; off by default)")
	regressionRunCmd.Flags().BoolVar(&regressionNoSave, "no-save", false,
		"do not persist a run record under .nerd/regression/runs/")

	regressionCmd.AddCommand(regressionRunCmd, regressionInitCmd, regressionListCmd)
}
