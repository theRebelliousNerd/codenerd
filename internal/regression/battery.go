// Package regression provides a lightweight, optional regression battery harness.
// Batteries are YAML-defined task suites that can be run as part of Nemesis
// gauntlets or manually to continuously evaluate agent behavior.
package regression

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Battery is a collection of regression tasks.
type Battery struct {
	Version int    `yaml:"version"`
	Tasks   []Task `yaml:"tasks"`
}

// Task is a single regression task.
// Currently supported: type=shell.
type Task struct {
	ID         string `yaml:"id"`
	Type       string `yaml:"type"` // "shell"
	Command    string `yaml:"command"`
	TimeoutSec int    `yaml:"timeout_sec,omitempty"`
}

// Result captures execution outcome for a task.
type Result struct {
	TaskID     string
	Success    bool
	Output     string
	Error      string
	DurationMs int64
}

// LoadBattery reads a YAML battery file from disk.
func LoadBattery(path string) (*Battery, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var b Battery
	if err := yaml.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("failed to parse battery YAML: %w", err)
	}
	return &b, nil
}

// RunBattery executes all tasks in order using the local shell.
// workdir is used as the subprocess working directory when non-empty.
func RunBattery(ctx context.Context, b *Battery, workdir string) ([]Result, error) {
	if b == nil || len(b.Tasks) == 0 {
		return nil, nil
	}

	results := make([]Result, 0, len(b.Tasks))

	for _, task := range b.Tasks {
		start := time.Now()
		t := strings.ToLower(strings.TrimSpace(task.Type))
		if t == "" {
			t = "shell"
		}

		res := Result{TaskID: task.ID}
		switch t {
		case "shell":
			timeout := time.Duration(task.TimeoutSec) * time.Second
			if timeout <= 0 {
				timeout = 5 * time.Minute
			}
			tctx, cancel := context.WithTimeout(ctx, timeout)
			out, err := runShell(tctx, task.Command, workdir)
			cancel()
			res.Output = out
			if err != nil {
				res.Success = false
				res.Error = err.Error()
			} else {
				res.Success = true
			}
		default:
			res.Success = false
			res.Error = fmt.Sprintf("unsupported task type: %s", task.Type)
		}

		res.DurationMs = time.Since(start).Milliseconds()
		results = append(results, res)

		// Fail-fast on first hard failure to keep gauntlet latency bounded.
		if !res.Success {
			break
		}
	}

	return results, nil
}

func runShell(ctx context.Context, command string, workdir string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("empty command")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", command)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-lc", command)
	}

	if workdir != "" {
		cmd.Dir = workdir
	}

	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(out), ctx.Err()
	}
	if err != nil {
		return string(out), fmt.Errorf("command failed (%s): %w", command, err)
	}
	return string(out), nil
}

// DefaultBatteryPath returns the canonical battery path for a workspace.
func DefaultBatteryPath(workspace string) string {
	return filepath.Join(workspace, ".nerd", "regression", "battery.yaml")
}

