package shell

import (
	"testing"
	"time"
)

func TestToolTimeoutHooks(t *testing.T) {
	runTests := RunTestsTool()
	runBuild := RunBuildTool()
	runCommand := RunCommandTool()
	bash := BashTool()

	table := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"run_tests no args", runTests.Timeout(map[string]any{}), 600 * time.Second},
		{"run_tests timeout 1200", runTests.Timeout(map[string]any{"timeout_seconds": 1200}), 1200 * time.Second},
		{"run_build no args", runBuild.Timeout(map[string]any{}), 300 * time.Second},
		{"run_command go test", runCommand.Timeout(map[string]any{"command": "go test ./..."}), 600 * time.Second},
		{"run_command echo", runCommand.Timeout(map[string]any{"command": "echo hi"}), 60 * time.Second},
		{"run_command echo override", runCommand.Timeout(map[string]any{"command": "echo hi", "timeout_seconds": 900}), 900 * time.Second},
		{"bash go build", bash.Timeout(map[string]any{"script": "go build ./...\necho done"}), 600 * time.Second},
	}
	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %v, want %v", tc.got, tc.want)
			}
		})
	}
}
