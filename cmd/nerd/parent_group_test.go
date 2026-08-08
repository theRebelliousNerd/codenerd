package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// A typo'd subcommand reporting success is the same false-success family as the
// rest of this session's work: the caller cannot tell "did what you asked" from
// "did nothing at all". Scripts, campaigns and shards all consume these exit
// codes.
//
// Observed before the fix: `nerd memory stats` (the real subcommand is `status`)
// printed the group's help text and exited 0.
func TestParentGroupsRejectUnknownSubcommands(t *testing.T) {
	groups := []*cobra.Command{
		memoryCmd, mcpCmd, campaignCmd, authCmd,
		autopoiesisCmd, northstarCmd, embeddingCmd, browserCmd, domCmd,
	}
	for _, g := range groups {
		t.Run(g.Name(), func(t *testing.T) {
			// The presence of a Run/RunE is what gets cobra past its
			// `if !c.Runnable() { return flag.ErrHelp }` short-circuit, which is
			// what makes an unknown subcommand an error instead of a silent
			// help print. Without it the group is unrunnable and exits 0.
			if !g.Runnable() {
				t.Errorf("%q has no Run/RunE, so an unknown subcommand prints help and exits 0", g.Name())
			}
			if len(g.Commands()) == 0 {
				t.Errorf("%q is treated as a group but has no subcommands", g.Name())
			}
		})
	}
}

// Bare invocation must still print help and succeed — that is correct for a
// group, and the fix must not turn it into an error.
func TestParentGroupBareInvocationPrintsHelp(t *testing.T) {
	var sb strings.Builder
	memoryCmd.SetOut(&sb)
	defer memoryCmd.SetOut(nil)

	if err := parentGroupRunE(memoryCmd, nil); err != nil {
		t.Fatalf("bare group invocation returned an error: %v", err)
	}
	if !strings.Contains(sb.String(), "memory") {
		t.Errorf("help output does not mention the command; got %q", sb.String())
	}
}
