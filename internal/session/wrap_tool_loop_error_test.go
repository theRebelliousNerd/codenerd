package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wrapToolLoopError separates LLM failures from post-edit verification
// failures. A plain error keeps the "LLM generation failed" prefix; a
// verification failure marked with ErrVerificationFailed passes through
// unchanged.
func TestWrapToolLoopError(t *testing.T) {
	cases := []struct {
		name                string
		err                 error
		wantLLMPrefix       bool
		wantVerificationErr bool
	}{
		{
			name:                "plain error keeps LLM prefix",
			err:                 errors.New("boom"),
			wantLLMPrefix:       true,
			wantVerificationErr: false,
		},
		{
			name:                "verification failure passes through",
			err:                 fmt.Errorf("%w: edits broke the build", ErrVerificationFailed),
			wantLLMPrefix:       false,
			wantVerificationErr: true,
		},
		{
			name: "wrapped verification failure with detail passes through",
			err: fmt.Errorf(
				"%w: edits broke the build and the repair round did not fix it. Compiler output:\n%s",
				ErrVerificationFailed, "main.go:3: undefined: neverWritten"),
			wantLLMPrefix:       false,
			wantVerificationErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapToolLoopError(tc.err)
			if got == nil {
				t.Fatalf("wrapToolLoopError(%v) = nil; want non-nil", tc.err)
			}
			hasPrefix := strings.HasPrefix(got.Error(), "LLM generation failed: ")
			if hasPrefix != tc.wantLLMPrefix {
				t.Errorf("wrapToolLoopError(%v) LLM prefix = %v; want %v (message %q)",
					tc.err, hasPrefix, tc.wantLLMPrefix, got.Error())
			}
			if gotVerification := errors.Is(got, ErrVerificationFailed); gotVerification != tc.wantVerificationErr {
				t.Errorf("errors.Is(wrapToolLoopError(%v), ErrVerificationFailed) = %v; want %v",
					tc.err, gotVerification, tc.wantVerificationErr)
			}
			if tc.wantVerificationErr && strings.Contains(got.Error(), "LLM generation failed") {
				t.Errorf("verification error must not contain LLM prefix, got %q", got.Error())
			}
		})
	}
}

func TestWrapToolLoopError_NilStaysNil(t *testing.T) {
	if got := wrapToolLoopError(nil); got != nil {
		t.Errorf("wrapToolLoopError(nil) = %v; want nil", got)
	}
}

// The build gate must mark its failures so the executor does not report them
// as LLM failures. Drives verifyAndRepairBuild against a throwaway broken
// package with a nil repair channel: no provider calls, one `go build`.
func TestVerifyAndRepairBuild_FailureCarriesSentinel(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a throwaway package")
	}
	ws := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("go.mod", "module verifyprobe\n\ngo 1.21\n")
	write("main.go", "package main\n\nfunc main() { neverWritten() }\n")

	cfg := DefaultExecutorConfig()
	cfg.WorkspaceRoot = ws
	e := &Executor{config: cfg}
	result := &ExecutionResult{SuccessfulWriteTools: 1, WrittenPaths: []string{"main.go"}}

	_, _, err := e.verifyAndRepairBuild(context.Background(), nil, "", nil, nil, nil, nil, result)
	if err == nil {
		t.Fatal("verifyAndRepairBuild returned nil for a broken package; want a verification failure")
	}
	if !errors.Is(err, ErrVerificationFailed) {
		t.Errorf("errors.Is(err, ErrVerificationFailed) = false; want true (message %q)", err.Error())
	}
	if !strings.Contains(err.Error(), "neverWritten") {
		t.Errorf("verification error drops the compiler detail the repair round needs: %q", err.Error())
	}
	if got := wrapToolLoopError(err); strings.Contains(got.Error(), "LLM generation failed") {
		t.Errorf("verification failure must not gain an LLM prefix, got %q", got.Error())
	}
}
