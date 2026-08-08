package python

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BaseImage != "python:3.10-slim" {
		t.Fatalf("unexpected base image: %s", cfg.BaseImage)
	}
	if cfg.PythonVersion != "3.10" {
		t.Fatalf("unexpected python version: %s", cfg.PythonVersion)
	}
	if cfg.MemoryLimit <= 0 || cfg.CPULimit <= 0 {
		t.Fatalf("expected positive resource limits")
	}
	if cfg.TestTimeout != 5*time.Minute {
		t.Fatalf("unexpected test timeout: %v", cfg.TestTimeout)
	}
	if cfg.WorkspaceDir != "/workspace" {
		t.Fatalf("unexpected workspace dir: %s", cfg.WorkspaceDir)
	}
}

func TestProjectInfoRepoName(t *testing.T) {
	info := ProjectInfo{Name: "explicit", GitURL: "https://github.com/org/repo.git"}
	if info.RepoName() != "explicit" {
		t.Fatalf("expected explicit name to win")
	}

	info = ProjectInfo{GitURL: "https://github.com/org/repo.git"}
	if info.RepoName() != "repo" {
		t.Fatalf("expected repo name from git url")
	}

	info = ProjectInfo{GitURL: ""}
	if info.RepoName() != "" {
		t.Fatalf("expected empty repo name for empty git url")
	}
}

func TestNewEnvironmentPathsAndState(t *testing.T) {
	project := &ProjectInfo{GitURL: "https://github.com/org/repo.git"}
	cfg := DefaultConfig()
	env := NewEnvironment(project, cfg, nil)

	if env.State() != StateInitializing {
		t.Fatalf("expected initial state initializing")
	}
	if env.RepoPath() != "/workspace/repo" {
		t.Fatalf("unexpected repo path: %s", env.RepoPath())
	}
	if env.VenvPath() != "/workspace/venv" {
		t.Fatalf("unexpected venv path: %s", env.VenvPath())
	}
	if env.ContainerID() != "" {
		t.Fatalf("expected empty container id without container")
	}
}

func TestEnvironmentSetError(t *testing.T) {
	project := &ProjectInfo{Name: "proj"}
	cfg := DefaultConfig()
	env := NewEnvironment(project, cfg, nil)

	err := errors.New("boom")
	env.setError(err)

	if env.State() != StateError {
		t.Fatalf("expected state error after setError")
	}
	if env.GetError() == nil || env.GetError().Error() != "boom" {
		t.Fatalf("expected stored error")
	}
}

func TestExtractPytestError(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "assertion error line returned",
			input: "collecting tests\nAssertionError: expected 1 but got 2\nmore output",
			want:  "AssertionError: expected 1 but got 2",
		},
		{
			name:  "error colon line returned",
			input: "running pytest\nValueError: Error: something went wrong\nstack trace",
			want:  "ValueError: Error: something went wrong",
		},
		{
			name:  "failed line returned",
			input: "collected 1 item\nFAILED test_foo.py::test_bar - assert False\nshort summary",
			want:  "FAILED test_foo.py::test_bar - assert False",
		},
		{
			name:  "first matching line wins",
			input: "FAILED test_a.py::test_one\nAssertionError: second error\nError: third",
			want:  "FAILED test_a.py::test_one",
		},
		{
			name:  "trims whitespace from matched line",
			input: "  AssertionError: spaced  \nnext line",
			want:  "AssertionError: spaced",
		},
		{
			name:  "fallback returns last non-empty line",
			input: "collected 2 items\npassed in 0.03s\nsome summary line",
			want:  "some summary line",
		},
		{
			name:  "fallback trims and skips trailing empty lines",
			input: "line one\nline two\n\n   \n",
			want:  "line two",
		},
		{
			name:  "fallback trims whitespace from last line",
			input: "hello\n  last line with spaces   ",
			want:  "last line with spaces",
		},
		{
			name:  "unknown error on empty string",
			input: "",
			want:  "unknown error",
		},
		{
			name:  "unknown error on whitespace only",
			input: "   \n\n  \n",
			want:  "unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPytestError(tt.input)
			if got != tt.want {
				t.Errorf("extractPytestError() = %q, want %q", got, tt.want)
			}
		})
	}
}

