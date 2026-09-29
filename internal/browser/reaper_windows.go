//go:build windows

package browser

// Process-tree inspection is adapted from BrowserNERD's Apache-2.0 reaper.
// Native handles replace shell termination and retain the inspected identity.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"codenerd/internal/config"
	"golang.org/x/sys/windows"
)

type windowsProcessQuerier struct {
	policy config.BrowserReaperConfig
}

func newProcessQuerier(policy config.BrowserReaperConfig) ProcessQuerier {
	return &windowsProcessQuerier{policy: policy}
}

func (q *windowsProcessQuerier) QueryChromeProcesses(ctx context.Context) ([]ProcessInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, q.policy.Timeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		`$ErrorActionPreference='Stop'; [Console]::OutputEncoding=[System.Text.UTF8Encoding]::new($false); @(Get-CimInstance Win32_Process -Filter "Name = 'chrome.exe' or Name = 'chromium.exe' or Name = 'chrome-headless-shell.exe'" | Select-Object ProcessId,ParentProcessId,CommandLine) | ConvertTo-Json -Compress`)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("inspect Chrome process command lines: %w", err)
	}
	return decodeChromeProcesses(out)
}

func decodeChromeProcesses(out []byte) ([]ProcessInfo, error) {
	out = bytes.TrimSpace(bytes.TrimPrefix(out, []byte{0xef, 0xbb, 0xbf}))
	if len(out) == 0 || bytes.Equal(out, []byte("null")) {
		return nil, nil
	}
	var processes []ProcessInfo
	if out[0] == '[' {
		if err := json.Unmarshal(out, &processes); err != nil {
			return nil, fmt.Errorf("decode Chrome process array: %w", err)
		}
	} else {
		var process ProcessInfo
		if err := json.Unmarshal(out, &process); err != nil {
			return nil, fmt.Errorf("decode Chrome process: %w", err)
		}
		processes = append(processes, process)
	}
	for _, process := range processes {
		if process.PID <= 0 || process.PPID < 0 {
			return nil, fmt.Errorf("invalid Chrome process identity")
		}
	}
	return processes, nil
}

func (q *windowsProcessQuerier) IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		// Only a nonexistent PID proves death; access/query failures prove nothing.
		return !errors.Is(err, windows.ERROR_INVALID_PARAMETER)
	}
	defer windows.CloseHandle(handle)
	status, err := windows.WaitForSingleObject(handle, 0)
	return err != nil || status != windows.WAIT_OBJECT_0
}

func windowsProcessTable() ([]ProcessInfo, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("snapshot Windows process tree: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	err = windows.Process32First(snapshot, &entry)
	var processes []ProcessInfo
	for err == nil {
		processes = append(processes, ProcessInfo{PID: int(entry.ProcessID), PPID: int(entry.ParentProcessID)})
		err = windows.Process32Next(snapshot, &entry)
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, fmt.Errorf("enumerate Windows process tree: %w", err)
	}
	return processes, nil
}

func (q *windowsProcessQuerier) KillProcessTree(pid int) error {
	if pid <= 1 || pid == os.Getpid() {
		return fmt.Errorf("safety refusal: invalid process tree root %d", pid)
	}
	processes, err := windowsProcessTable()
	if err != nil {
		return err
	}
	pids := descendantPIDs(pid, processes)
	// Open every handle before termination so later PID reuse cannot redirect
	// an individual kill. Stop new descendants by terminating the root first.
	type heldProcess struct {
		pid    int
		handle windows.Handle
	}
	var held []heldProcess
	defer func() {
		for _, process := range held {
			_ = windows.CloseHandle(process.handle)
		}
	}()
	for _, child := range pids {
		if child <= 1 || child == os.Getpid() {
			return fmt.Errorf("safety refusal: protected PID in Chrome tree")
		}
		handle, openErr := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(child))
		if errors.Is(openErr, windows.ERROR_INVALID_PARAMETER) {
			continue
		}
		if openErr != nil {
			return fmt.Errorf("open Chrome process %d: %w", child, openErr)
		}
		held = append(held, heldProcess{pid: child, handle: handle})
		if child == pid {
			var buffer [32768]uint16
			size := uint32(len(buffer))
			if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
				return fmt.Errorf("verify Chrome tree root: %w", err)
			}
			name := strings.ToLower(filepath.Base(windows.UTF16ToString(buffer[:size])))
			if name != "chrome.exe" && name != "chromium.exe" && name != "chrome-headless-shell.exe" {
				return fmt.Errorf("safety refusal: tree root is no longer Chrome")
			}
		}
	}
	var result error
	for _, process := range held {
		status, waitErr := windows.WaitForSingleObject(process.handle, 0)
		if waitErr == nil && status == windows.WAIT_OBJECT_0 {
			continue
		}
		if err := windows.TerminateProcess(process.handle, 1); err != nil {
			result = errors.Join(result, fmt.Errorf("terminate Chrome process %d: %w", process.pid, err))
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), q.policy.Timeout())
	defer cancel()
	for _, process := range held {
		for {
			status, err := windows.WaitForSingleObject(process.handle, 0)
			if err != nil {
				result = errors.Join(result, fmt.Errorf("wait for Chrome process %d: %w", process.pid, err))
				break
			}
			if status == windows.WAIT_OBJECT_0 {
				break
			}
			if err := waitProcessPoll(ctx, q.policy.PollInterval()); err != nil {
				return errors.Join(result, fmt.Errorf("Chrome tree did not exit: %w", err))
			}
		}
	}
	return result
}
