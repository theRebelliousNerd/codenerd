package main

import (
	"testing"
)

// The direct-action commands used to register a local --verbose that shadowed
// the root's persistent -v/--verbose, so `nerd fix -v` failed to parse and
// `--verbose` set a different variable. One flag now.
func TestFlags_VerboseParsesOnDirectActions(t *testing.T) {
	for _, args := range [][]string{{"-v", "x"}, {"--verbose", "x"}} {
		verbose = false
		if err := fixCmd.ParseFlags(args); err != nil {
			t.Fatalf("fix %v: %v", args, err)
		}
		if !verbose {
			t.Fatalf("fix %v did not set the root verbose flag", args)
		}
	}
	verbose = false
}

// --dry-run and --trace-api were registered and never read; they are gone.
func TestFlags_DeadDebugFlagsRemoved(t *testing.T) {
	for _, flag := range []string{"--dry-run", "--trace-api"} {
		if err := fixCmd.ParseFlags([]string{flag}); err == nil {
			t.Errorf("fix %s parsed, want unknown-flag error", flag)
		}
	}
}

// --disable-system-shard was local to `run` but read by every command that
// boots a Cortex; it is persistent on the root now.
func TestFlags_DisableSystemShardIsPersistent(t *testing.T) {
	for name, cmd := range map[string]interface {
		ParseFlags([]string) error
	}{"chat": chatCmd, "query": queryCmd, "run": runCmd, "root": rootCmd} {
		if err := cmd.ParseFlags([]string{"--disable-system-shard", "legislator"}); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	disableSystemShards = nil
}

// A debug aid never fails the command: flag off or no kernel returns "".
func TestFlags_DumpKernelSnapshotIsInert(t *testing.T) {
	dumpKernel = false
	if got := DumpKernelSnapshot(nil, t.TempDir()); got != "" {
		t.Fatalf("flag off: got %q, want empty", got)
	}
	dumpKernel = true
	defer func() { dumpKernel = false }()
	if got := DumpKernelSnapshot(nil, t.TempDir()); got != "" {
		t.Fatalf("nil kernel: got %q, want empty", got)
	}
}
