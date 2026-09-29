package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/logging"
)

func TestWorkspaceFromArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "absent", args: []string{"features"}, want: ""},
		{name: "long", args: []string{"--workspace", `C:\ws`, "features"}, want: `C:\ws`},
		{name: "equals", args: []string{`--workspace=C:\ws`, "features"}, want: `C:\ws`},
		{name: "short", args: []string{"features", "-w", `C:\ws`}, want: `C:\ws`},
		{name: "short equals", args: []string{`-w=C:\ws`}, want: `C:\ws`},
		{name: "short joined", args: []string{`-wC:\ws`, "features"}, want: `C:\ws`},
		{name: "short joined relative", args: []string{"-wrel/ws"}, want: `rel/ws`},
		{name: "end of flags", args: []string{"--", "--workspace", `C:\ws`}, want: ""},
		{name: "missing value", args: []string{"--workspace"}, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := workspaceFromArgs(tc.args); got != tc.want {
				t.Fatalf("workspaceFromArgs(%q) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

// execRoot runs the real root command the way a user would invoke it.
// Output is buffered so a command that reaches RunE does not dump its
// table into the test log.
func execRoot(t *testing.T, args ...string) error {
	t.Helper()
	oldWorkspace := workspace
	oldSilence := rootCmd.SilenceErrors
	oldUsage := rootCmd.SilenceUsage
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SilenceErrors = true
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		workspace = oldWorkspace
		rootCmd.SilenceErrors = oldSilence
		rootCmd.SilenceUsage = oldUsage
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})
	return rootCmd.Execute()
}

// A workspace with no config still boots. Declared before the removed-key
// test: that one fails Initialize, and a failed Initialize stays failed for
// the rest of the process.
func TestRootCommand_WorkspaceWithoutConfigStillRuns(t *testing.T) {
	ws := t.TempDir()
	if err := bindEarlyFileLogging([]string{"--workspace", ws, "features"}); err != nil {
		t.Fatalf("missing config stopped early logging init: %v", err)
	}
	if err := execRoot(t, "--workspace", ws, "features"); err != nil {
		t.Fatalf("features in a workspace with no config: %v", err)
	}
}

// A config still carrying logging.json_format must refuse the boot on the CLI
// path, not warn past it. Finding 6 (commit 39625fbb): initializeInternal
// returns the removed-key rejection, but PersistentPreRunE printed it and
// returned nil, the command continuing with file logging torn down
// (logger.go closes the sinks, then sets initialized false). main's early
// init had the same hole, and it bound the process cwd because --workspace
// is not parsed yet.
//
// This drives both sites. bindEarlyFileLogging is what main calls.
// execRoot is the parsed-flag path, in-process, with --workspace pointing
// at a temp workspace whose config this test writes (no API keys).
//
// A failed Initialize bricks later Initialize calls process-wide (the
// sync.Once is consumed and initialized stays false, so every later call
// returns the stale error). This file sorts after cmd_audit_test.go, the
// other logging.Initialize user in this package. Nothing after it touches
// logging.
func TestRootCommand_RemovedLoggingKeyFailsTheBoot(t *testing.T) {
	bad := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bad, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := `{"logging":{"json_format":true}}`
	if err := os.WriteFile(filepath.Join(bad, ".nerd", "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Equals form: main reads argv itself, before cobra splits flags.
	early := bindEarlyFileLogging([]string{"--workspace=" + bad, "features"})
	if early == nil {
		t.Fatal("early logging init succeeded, want a fatal boot error")
	}
	if !strings.Contains(early.Error(), "json_format") {
		t.Fatalf("early boot error does not name the removed key: %v", early)
	}
	if !logging.IsRemovedKeyError(early) {
		t.Errorf("early boot error is not the removed-key rejection: %T %v", early, early)
	}

	// `features` is the cheapest leaf: its RunE only prints the resolved
	// registry. The PersistentPreRunE rejection fails the command before
	// RunE runs. Separate-argument form, which cobra parses into the flag.
	err := execRoot(t, "--workspace", bad, "features")
	if err == nil {
		t.Fatal("root command in a workspace with logging.json_format succeeded, want a fatal boot error")
	}
	if !strings.Contains(err.Error(), "json_format") {
		t.Fatalf("boot error does not name the removed key: %v", err)
	}
	if !logging.IsRemovedKeyError(err) {
		t.Errorf("boot error is not the removed-key rejection: %T %v", err, err)
	}
}
