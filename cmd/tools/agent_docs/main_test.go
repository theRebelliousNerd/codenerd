package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCheck(t *testing.T) {
	errBoom := errors.New("boom")
	tests := []struct {
		name     string
		stale    []string
		staleErr error
		wantCode int
		wantOut  string
		wantErr  string
		wantRoot string
	}{
		{
			name:     "ReportsLoadErrorAsExit2",
			staleErr: errBoom,
			wantCode: 2,
			wantOut:  "",
			wantErr:  "boom\n",
		},
		{
			name:     "ListsSingleStaleTwinAsExit1",
			stale:    []string{"nerd.md"},
			wantCode: 1,
			wantOut:  "nerd.md\n",
			wantErr:  "1 stale twin(s); run go run ./cmd/tools/agent_docs\n",
		},
		{
			name:     "ListsMultipleStaleTwinsAsExit1",
			stale:    []string{"a/nerd.md", "b/CLAUDE.md"},
			wantCode: 1,
			wantOut:  "a/nerd.md\nb/CLAUDE.md\n",
			wantErr:  "2 stale twin(s); run go run ./cmd/tools/agent_docs\n",
		},
		{
			name:     "CleanTreeExitsZeroQuietly",
			stale:    nil,
			wantCode: 0,
			wantOut:  "",
			wantErr:  "",
		},
		{
			name:     "EmptySliceIsClean",
			stale:    []string{},
			wantCode: 0,
			wantOut:  "",
			wantErr:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			gotRoot := ""
			code := runCheck("testroot", &stdout, &stderr, func(root string) ([]string, error) {
				gotRoot = root
				return tc.stale, tc.staleErr
			})
			if code != tc.wantCode {
				t.Fatalf("runCheck exit = %d, want %d", code, tc.wantCode)
			}
			if gotRoot != "testroot" {
				t.Fatalf("runCheck root = %q, want %q", gotRoot, "testroot")
			}
			if stdout.String() != tc.wantOut {
				t.Fatalf("runCheck stdout = %q, want %q", stdout.String(), tc.wantOut)
			}
			if stderr.String() != tc.wantErr {
				t.Fatalf("runCheck stderr = %q, want %q", stderr.String(), tc.wantErr)
			}
		})
	}
}

func TestRunWrite(t *testing.T) {
	errBoom := errors.New("write boom")
	tests := []struct {
		name       string
		written    []string
		writeErr   error
		wantCode   int
		wantOut    string
		wantErrSub string
	}{
		{
			name:     "WritesTwinsAndReportsCount",
			written:  []string{"CLAUDE.md", "nerd.md"},
			wantCode: 0,
			wantOut:  "wrote CLAUDE.md\nwrote nerd.md\n2 twin(s) written\n",
		},
		{
			name:     "NoTwinsReportsZero",
			written:  nil,
			wantCode: 0,
			wantOut:  "0 twin(s) written\n",
		},
		{
			name:       "PartialWriteStillListsWrittenBeforeError",
			written:    []string{"CLAUDE.md"},
			writeErr:   errBoom,
			wantCode:   1,
			wantOut:    "wrote CLAUDE.md\n",
			wantErrSub: "write boom",
		},
		{
			name:       "ErrorWithoutWritesReportsOnlyError",
			writeErr:   errBoom,
			wantCode:   1,
			wantOut:    "",
			wantErrSub: "write boom",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runWrite("testroot", &stdout, &stderr, func(root string) ([]string, error) {
				if root != "testroot" {
					t.Fatalf("runWrite root = %q, want %q", root, "testroot")
				}
				return tc.written, tc.writeErr
			})
			if code != tc.wantCode {
				t.Fatalf("runWrite exit = %d, want %d", code, tc.wantCode)
			}
			if stdout.String() != tc.wantOut {
				t.Fatalf("runWrite stdout = %q, want %q", stdout.String(), tc.wantOut)
			}
			if tc.writeErr != nil {
				if !strings.Contains(stderr.String(), tc.wantErrSub) {
					t.Fatalf("runWrite stderr = %q, want substring %q", stderr.String(), tc.wantErrSub)
				}
			} else if stderr.String() != "" {
				t.Fatalf("runWrite stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRun_DispatchesOnCheckFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run("root", true, &stdout, &stderr,
		func(root string) ([]string, error) {
			if root != "root" {
				t.Fatalf("stale root = %q", root)
			}
			return nil, nil
		},
		func(root string) ([]string, error) {
			t.Fatal("writeFn must not run in check mode")
			return nil, nil
		},
	)
	if code != 0 {
		t.Fatalf("run check exit = %d, want 0", code)
	}

	stdout.Reset()
	stderr.Reset()
	code = run("root", false, &stdout, &stderr,
		func(root string) ([]string, error) {
			t.Fatal("staleFn must not run in write mode")
			return nil, nil
		},
		func(root string) ([]string, error) {
			if root != "root" {
				t.Fatalf("write root = %q", root)
			}
			return []string{"nerd.md"}, nil
		},
	)
	if code != 0 {
		t.Fatalf("run write exit = %d, want 0", code)
	}
	if stdout.String() != "wrote nerd.md\n1 twin(s) written\n" {
		t.Fatalf("run write stdout = %q", stdout.String())
	}
}

func invokeMain(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	oldArgs := os.Args
	oldCmd := flag.CommandLine
	oldExit := osExit
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	t.Cleanup(func() {
		os.Args = oldArgs
		flag.CommandLine = oldCmd
		osExit = oldExit
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	})
	flag.CommandLine = flag.NewFlagSet("agent_docs", flag.ContinueOnError)
	os.Args = args
	outFile, err := os.CreateTemp("", "agentdocs-stdout")
	if err != nil {
		t.Fatal(err)
	}
	errFile, err := os.CreateTemp("", "agentdocs-stderr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Remove(outFile.Name())
		os.Remove(errFile.Name())
	})
	os.Stdout = outFile
	os.Stderr = errFile
	exitCode := -1
	osExit = func(code int) { exitCode = code }
	main()
	outFile.Close()
	errFile.Close()
	outBytes, _ := os.ReadFile(outFile.Name())
	errBytes, _ := os.ReadFile(errFile.Name())
	return exitCode, string(outBytes), string(errBytes)
}

func writeAgentsFile(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "agents.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMain_CheckCleanExitsZero(t *testing.T) {
	dir := t.TempDir()
	writeAgentsFile(t, dir, "guidance\n")
	if code, _, _ := invokeMain(t, []string{"agent_docs", "-root", dir}); code != 0 {
		t.Fatalf("write exit = %d, want 0", code)
	}
	code, stdout, stderr := invokeMain(t, []string{"agent_docs", "-root", dir, "-check"})
	if code != 0 {
		t.Fatalf("check clean exit = %d, want 0 (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("check clean stdout = %q, want empty", stdout)
	}
	if stderr != "" {
		t.Fatalf("check clean stderr = %q, want empty", stderr)
	}
}

func TestMain_CheckStaleExitsOne(t *testing.T) {
	dir := t.TempDir()
	writeAgentsFile(t, dir, "guidance\n")
	code, stdout, stderr := invokeMain(t, []string{"agent_docs", "-root", dir, "-check"})
	if code != 1 {
		t.Fatalf("check stale exit = %d, want 1 (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "nerd.md") {
		t.Fatalf("check stale stdout = %q, want nerd.md", stdout)
	}
	if !strings.Contains(stderr, "stale twin(s)") {
		t.Fatalf("check stale stderr = %q, want stale twin hint", stderr)
	}
}

func TestMain_CheckErrorExitsTwo(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	code, _, stderr := invokeMain(t, []string{"agent_docs", "-root", missing, "-check"})
	if code != 2 {
		t.Fatalf("check error exit = %d, want 2 (stderr=%q)", code, stderr)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Fatal("check error stderr is empty, want error")
	}
}

func TestMain_WriteCreatesTwinsExitsZero(t *testing.T) {
	dir := t.TempDir()
	writeAgentsFile(t, dir, "guidance\n")
	code, stdout, _ := invokeMain(t, []string{"agent_docs", "-root", dir})
	if code != 0 {
		t.Fatalf("write exit = %d, want 0 (stdout=%q)", code, stdout)
	}
	if !strings.Contains(stdout, "twin(s) written") {
		t.Fatalf("write stdout = %q, want twin count", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Fatalf("CLAUDE.md not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "nerd.md")); err != nil {
		t.Fatalf("nerd.md not written: %v", err)
	}
}
