package core

import (
	"context"
	"strings"
	"testing"
)

func TestExecutionValidator_New(t *testing.T) {
	v := NewExecutionValidator()
	if v == nil {
		t.Fatal("NewExecutionValidator returned nil")
	}
	if len(v.failurePatterns) == 0 {
		t.Error("NewExecutionValidator has no failure patterns")
	}
}

func TestExecutionValidator_CanValidate(t *testing.T) {
	v := NewExecutionValidator()

	testCases := []struct {
		action ActionType
		want   bool
	}{
		{ActionRunCommand, true},
		{ActionBash, true},
		{ActionExecCmd, true},
		{ActionRunBuild, true},
		{ActionRunTests, true},
		{ActionGitOperation, true},
		{ActionWriteFile, false},
		{ActionReadFile, false},
	}

	for _, tc := range testCases {
		got := v.CanValidate(tc.action)
		if got != tc.want {
			t.Errorf("CanValidate(%v) = %v, want %v", tc.action, got, tc.want)
		}
	}
}

func TestExecutionValidator_Validate_GeneralPatterns(t *testing.T) {
	v := NewExecutionValidator()
	ctx := context.Background()
	req := ActionRequest{Type: ActionRunCommand, Target: "echo hello"}

	testCases := []struct {
		name      string
		output    string
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "Clean output",
			output:    "Hello world\nEverything is fine",
			shouldErr: false,
		},
		{
			name:      "Panic",
			output:    "Doing work...\npanic: runtime error: index out of range",
			shouldErr: true,
			errMsg:    "failure pattern detected",
		},
		{
			name:      "Fatal",
			output:    "Starting...\nFATAL: database connection lost",
			shouldErr: true,
			errMsg:    "failure pattern detected",
		},
		{
			name:      "Error",
			output:    "Compiling...\nError: syntax error on line 5",
			shouldErr: true,
			errMsg:    "failure pattern detected",
		},
		{
			name:      "Permission denied",
			output:    "cp: cannot open file: Permission denied",
			shouldErr: true,
			errMsg:    "failure pattern detected",
		},
		{
			name:      "Command not found",
			output:    "bash: foo: command not found",
			shouldErr: true,
			errMsg:    "failure pattern detected",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ActionResult{Success: true, Output: tc.output}
			vr := v.Validate(ctx, req, result)

			if tc.shouldErr {
				if vr.Verified {
					t.Errorf("Expected failure for output: %q", tc.output)
				}
				if vr.Error == "" {
					t.Error("Expected error message, got empty string")
				}
				if tc.errMsg != "" && !strings.Contains(vr.Error, tc.errMsg) {
					t.Errorf("Expected error to contain %q, got %q", tc.errMsg, vr.Error)
				}
			} else {
				if !vr.Verified {
					t.Errorf("Expected success, got error: %s", vr.Error)
				}
			}
		})
	}
}

func TestExecutionValidator_Validate_AlreadyFailed(t *testing.T) {
	v := NewExecutionValidator()
	ctx := context.Background()
	req := ActionRequest{Type: ActionRunCommand, Target: "ls"}
	result := ActionResult{Success: false, Error: "os error"}

	vr := v.Validate(ctx, req, result)
	if vr.Verified {
		t.Error("Expected validation failure when result.Success is false")
	}
	if vr.Error != "command reported failure: os error" {
		t.Errorf("Unexpected error message: %s", vr.Error)
	}
}

func TestExecutionValidator_CommandSpecific(t *testing.T) {
	v := NewExecutionValidator()
	ctx := context.Background()

	testCases := []struct {
		name      string
		cmd       string
		output    string
		shouldErr bool
		errorContains string
	}{
		// Go Build
		{
			name:      "Go build success",
			cmd:       "go build .",
			output:    "",
			shouldErr: false,
		},
		{
			name:      "Go build cannot find package",
			cmd:       "go build .",
			output:    "main.go:3:8: cannot find package",
			shouldErr: true,
			errorContains: "failure pattern detected", // Matches "cannot find" general pattern
		},
		{
			name:      "Go vet undefined",
			cmd:       "go vet .",
			output:    "main.go:5:2: undefined: Foo",
			shouldErr: true,
			errorContains: "Go compilation error",
		},

		// Go Test
		{
			name:      "Go test pass",
			cmd:       "go test ./...",
			output:    "ok\tgithub.com/foo/bar\t0.123s",
			shouldErr: false,
		},
		{
			name:      "Go test fail",
			cmd:       "go test ./...",
			output:    "--- FAIL: TestFoo (0.00s)\nFAIL\nFAIL\tgithub.com/foo/bar\t0.002s",
			shouldErr: true,
			errorContains: "Go test failure",
		},

		// NPM
		{
			name:      "NPM error",
			cmd:       "npm install",
			output:    "npm ERR! code E404",
			shouldErr: true,
			errorContains: "npm/yarn error",
		},

		// Python
		{
			name:      "Python traceback",
			cmd:       "python script.py",
			output:    "Traceback (most recent call last):\n  File \"script.py\", line 1, in <module>\nNameError: name 'x' is not defined",
			shouldErr: true,
			errorContains: "failure pattern detected", // Matches "traceback" general pattern
		},

		// Git
		{
			name:      "Git conflict",
			cmd:       "git merge feature",
			output:    "CONFLICT (content): Merge conflict in file.txt",
			shouldErr: true,
			errorContains: "Git error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := ActionRequest{Type: ActionRunCommand, Target: tc.cmd}
			result := ActionResult{Success: true, Output: tc.output}

			vr := v.Validate(ctx, req, result)

			if tc.shouldErr {
				if vr.Verified {
					t.Errorf("Expected failure for %s", tc.name)
				}
				if vr.Error == "" {
					t.Error("Expected error message")
				}
				if tc.errorContains != "" && !strings.Contains(vr.Error, tc.errorContains) {
					t.Errorf("Expected error to contain %q, got %q", tc.errorContains, vr.Error)
				}
			} else {
				if !vr.Verified {
					t.Errorf("Expected success for %s, got: %s", tc.name, vr.Error)
				}
			}
		})
	}
}

func TestBuildValidator(t *testing.T) {
	v := NewBuildValidator()
	if v.Name() != "build_validator" {
		t.Errorf("Unexpected name: %s", v.Name())
	}
	if v.Priority() != 8 {
		t.Errorf("Unexpected priority: %d", v.Priority())
	}
	if !v.CanValidate(ActionRunBuild) {
		t.Error("Should validate ActionRunBuild")
	}

	ctx := context.Background()
	req := ActionRequest{Type: ActionRunBuild, Target: "make"}

	// Test build-specific pattern
	result := ActionResult{Success: true, Output: "gcc: error: compilation failed"}
	vr := v.Validate(ctx, req, result)

	if vr.Verified {
		t.Error("Expected build failure detection")
	}
}

func TestTestValidator(t *testing.T) {
	v := NewTestValidator()
	if v.Name() != "test_validator" {
		t.Errorf("Unexpected name: %s", v.Name())
	}
	if !v.CanValidate(ActionRunTests) {
		t.Error("Should validate ActionRunTests")
	}

	ctx := context.Background()
	req := ActionRequest{Type: ActionRunTests, Target: "go test"}

	// Test test-specific pattern
	result := ActionResult{Success: true, Output: "FAIL: test_foo"}
	vr := v.Validate(ctx, req, result)

	if vr.Verified {
		t.Error("Expected test failure detection")
	}
}
