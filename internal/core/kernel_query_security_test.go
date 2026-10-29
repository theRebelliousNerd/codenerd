package core

import (
	"strings"
	"testing"
)

func TestGitCmdSecurityValidation(t *testing.T) {
	tests := []struct {
		name          string
		workspaceRoot string
		args          []string
		wantErr       bool
		errMsg        string
	}{
		{
			name:          "Valid subcommand - status",
			workspaceRoot: ".",
			args:          []string{"status", "--porcelain"},
			wantErr:       false,
		},
		{
			name:          "Valid subcommand - rev-parse",
			workspaceRoot: ".",
			args:          []string{"rev-parse", "--abbrev-ref", "HEAD"},
			wantErr:       false,
		},
		{
			name:          "Valid subcommand - log",
			workspaceRoot: ".",
			args:          []string{"log", "-n", "5"},
			wantErr:       false,
		},
		{
			name:          "Empty workspace root",
			workspaceRoot: "",
			args:          []string{"status"},
			wantErr:       true,
			errMsg:        "workspace root is empty",
		},
		{
			name:          "No arguments provided",
			workspaceRoot: ".",
			args:          []string{},
			wantErr:       true,
			errMsg:        "no git subcommand provided",
		},
		{
			name:          "Unauthorized subcommand",
			workspaceRoot: ".",
			args:          []string{"clone", "https://evil.com/repo"},
			wantErr:       true,
			errMsg:        "unauthorized git subcommand: clone",
		},
		{
			name:          "Dangerous argument --exec-path",
			workspaceRoot: ".",
			args:          []string{"status", "--exec-path=/tmp"},
			wantErr:       true,
			errMsg:        "unauthorized git argument: --exec-path=/tmp",
		},
		{
			name:          "Dangerous argument -c",
			workspaceRoot: ".",
			args:          []string{"status", "-ccore.pager=malicious"},
			wantErr:       true,
			errMsg:        "unauthorized git argument: -ccore.pager=malicious",
		},
		{
			name:          "Dangerous argument --upload-pack",
			workspaceRoot: ".",
			args:          []string{"status", "--upload-pack=malicious"},
			wantErr:       true,
			errMsg:        "unauthorized git argument: --upload-pack=malicious",
		},
		{
			name:          "Dangerous argument --receive-pack",
			workspaceRoot: ".",
			args:          []string{"status", "--receive-pack=malicious"},
			wantErr:       true,
			errMsg:        "unauthorized git argument: --receive-pack=malicious",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We only want to test the validation logic, not the actual execution.
			// However, since gitCmd executes exec.Command, for valid commands
			// it will execute them or fail if git is not present/workspace is not a repo.
			// So we check if an error occurred. If we expect a security validation error,
			// we verify the error message matches.
			_, err := gitCmd(tt.workspaceRoot, tt.args...)

			if tt.wantErr {
				if err == nil {
					t.Errorf("gitCmd() expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("gitCmd() expected error containing %q, got %v", tt.errMsg, err)
				}
			} else {
				// For valid commands, they might fail because we're running them in a temp dir
				// that might not be a git repo during tests, but they shouldn't fail with our
				// security validation errors.
				if err != nil {
					if strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "empty") {
						t.Errorf("gitCmd() failed with validation error for valid input: %v", err)
					}
				}
			}
		})
	}
}
