//go:build !windows

package browser

// Process inspection is adapted from BrowserNERD's Apache-2.0 reaper.
// Descendants are enumerated explicitly; unrelated process groups are never signaled.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"codenerd/internal/config"
)

type unixProcessQuerier struct {
	policy config.BrowserReaperConfig
}

func newProcessQuerier(policy config.BrowserReaperConfig) ProcessQuerier {
	return &unixProcessQuerier{policy: policy}
}

func queryUnixProcesses(ctx context.Context) ([]ProcessInfo, error) {
	cmd := exec.CommandContext(ctx, "ps", "-axww", "-o", "pid=,ppid=,args=")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("inspect Unix process table: %w", err)
	}
	return parsePsOutput(string(out))
}

func parsePsOutput(out string) ([]ProcessInfo, error) {
	var processes []ProcessInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, fmt.Errorf("incomplete Unix process row")
		}
		pid, pidErr := strconv.Atoi(fields[0])
		ppid, parentErr := strconv.Atoi(fields[1])
		if pidErr != nil || parentErr != nil || pid <= 0 || ppid < 0 {
			return nil, fmt.Errorf("invalid Unix process identity")
		}
		command := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
		command = strings.TrimSpace(strings.TrimPrefix(command, fields[1]))
		processes = append(processes, ProcessInfo{PID: pid, PPID: ppid, CommandLine: command})
	}
	return processes, nil
}

func (q *unixProcessQuerier) QueryChromeProcesses(ctx context.Context) ([]ProcessInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, q.policy.Timeout())
	defer cancel()
	processes, err := queryUnixProcesses(ctx)
	if err != nil {
		return nil, err
	}
	var chrome []ProcessInfo
	for _, process := range processes {
		command := strings.TrimSpace(process.CommandLine)
		if command == "" {
			continue
		}
		first := strings.Fields(command)[0]
		name := strings.ToLower(filepath.Base(first))
		macApp := strings.HasPrefix(command, "/") && (strings.Contains(command, "/Google Chrome.app/Contents/") || strings.Contains(command, "/Chromium.app/Contents/"))
		if name != "chrome" && name != "chromium" && name != "chromium-browser" && name != "chrome-headless-shell" &&
			!macApp {
			continue
		}
		if macApp {
			// Verify the executable separately: the flattened args column may
			// contain an application path merely as another program's argument.
			out, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(process.PID), "-o", "comm=").Output()
			if err != nil {
				if !q.IsProcessAlive(process.PID) {
					continue
				}
				return nil, fmt.Errorf("verify Chrome executable: %w", err)
			}
			executable := strings.TrimSpace(string(out))
			if !strings.Contains(executable, "/Google Chrome.app/Contents/") && !strings.Contains(executable, "/Chromium.app/Contents/") {
				continue
			}
		}
		// Linux's cmdline preserves argument boundaries. ps flattens paths with
		// spaces, which must not become permission to kill a different profile.
		if raw, readErr := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", process.PID)); readErr == nil {
			var args []string
			for _, arg := range strings.Split(string(raw), "\x00") {
				if arg != "" {
					args = append(args, strconv.Quote(arg))
				}
			}
			process.CommandLine = strings.Join(args, " ")
		}
		chrome = append(chrome, process)
	}
	return chrome, nil
}

func (q *unixProcessQuerier) IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
		// The command name can contain spaces and parentheses. The state
		// follows the final closing parenthesis, not the fourth Fields token.
		if end := strings.LastIndexByte(string(data), ')'); end >= 0 {
			fields := strings.Fields(string(data[end+1:]))
			if len(fields) > 0 && (fields[0] == "Z" || fields[0] == "X") {
				return false
			}
		}
	}
	err := syscall.Kill(pid, 0)
	return err == nil || !errors.Is(err, syscall.ESRCH)
}

func (q *unixProcessQuerier) KillProcessTree(pid int) error {
	if pid <= 1 || pid == os.Getpid() {
		return fmt.Errorf("safety refusal: invalid process tree root %d", pid)
	}
	ctx, cancel := context.WithTimeout(context.Background(), q.policy.Timeout())
	defer cancel()
	processes, err := queryUnixProcesses(ctx)
	if err != nil {
		return err
	}
	var result error
	pids := make(map[int]bool)
	descendants := descendantPIDs(pid, processes)
	for _, child := range descendants {
		if child <= 1 || child == os.Getpid() {
			return fmt.Errorf("safety refusal: protected PID in Chrome tree")
		}
	}
	for _, child := range descendants {
		if err := syscall.Kill(child, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			result = errors.Join(result, fmt.Errorf("terminate Chrome process %d: %w", child, err))
		}
		pids[child] = true
	}
	return errors.Join(result, waitProcessExit(ctx, q, pids, q.policy.PollInterval()))
}
