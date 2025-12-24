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
