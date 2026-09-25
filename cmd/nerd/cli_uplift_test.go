package main

import (
	"sort"
	"testing"

	"github.com/spf13/cobra"
)

// The CLI is the production entry path, so its wiring is pinned behaviorally:
// every defined command must be reachable from rootCmd, documented, and
// uniquely addressed. Dropping a registration (or adding a command file
// without wiring it) fails here instead of shipping a silent dead command.

func walkCommandPaths(t *testing.T) map[string]*cobra.Command {
	t.Helper()
	paths := map[string]*cobra.Command{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		p := c.CommandPath()
		if _, dup := paths[p]; dup {
			t.Errorf("duplicate command path %q", p)
		}
		paths[p] = c
		if c.Use == "" {
			t.Errorf("command %q has empty Use", p)
		}
		if c.Short == "" {
			t.Errorf("command %q has empty Short", p)
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
	return paths
}

func TestCLI_TopLevelCommands_MatchExpectedSet(t *testing.T) {
	want := []string{
		"agents", "analyze", "audit", "auth", "autopoiesis", "browser",
		"campaign", "chat", "check-mangle", "commit", "config", "context-stats",
		"create", "define-agent", "dom", "dream", "embedding", "explain",
		"features", "fix", "glassbox", "init", "jit", "knowledge", "logic",
		"logs", "mangle-lsp", "mcp", "memory", "meter", "northstar",
		"perception", "push", "query", "refactor", "reflection",
		"regression", "retrieve", "review", "run", "scan", "security",
		"sessions", "shadow", "snapshot", "spawn", "status", "swebench",
		"test", "test-context", "tool", "transparency", "usage", "whatif",
		"why", "world",
	}
	paths := walkCommandPaths(t)
	var got []string
	for p := range paths {
		var name string
		if n := len("nerd "); len(p) > n {
			name = p[n:]
		} else {
			continue // root itself
		}
		hasSpace := false
		for _, r := range name {
			if r == ' ' {
				hasSpace = true
				break
			}
		}
		if !hasSpace {
			got = append(got, name)
		}
	}
	assertStringSet(t, "top-level commands", want, got)
}

func TestCLI_SubcommandGroups_MatchExpectedSets(t *testing.T) {
	groups := map[string][]string{
		"nerd audit":            {"facts", "playbook"},
		"nerd auth":             {"claude", "codex", "grok", "status"},
		"nerd autopoiesis":      {"learning", "status", "tools"},
		"nerd browser":          {"click", "fork", "honeypot", "launch", "list", "screenshot", "session", "snapshot", "type"},
		"nerd campaign":         {"assault", "journal", "list", "pause", "recurse", "report", "resume", "start", "status"},
		"nerd campaign journal": {"replay", "verify"},
		"nerd config":           {"check", "full"},
		"nerd dom":              {"apply", "demo", "edit", "get", "inspect", "replace"},
		"nerd embedding":        {"reembed", "set", "stats"},
		"nerd knowledge":        {"list", "search"},
		"nerd mcp":              {"list", "metrics", "select", "status", "tools"},
		"nerd memory":           {"status"},
		"nerd meter":            {"atoms", "epochs"},
		"nerd northstar":        {"drift", "export", "facts", "history", "load", "query", "show", "state", "stats", "summary", "sync"},
		"nerd regression":       {"init", "list", "run"},
		"nerd sessions":         {"list", "load"},
		"nerd snapshot":         {"export", "import", "list", "verify"},
		"nerd swebench":         {"evaluate", "setup"},
		"nerd world":            {"predicates", "runbook"},
	}
	paths := walkCommandPaths(t)
	for parent, want := range groups {
		cmd, ok := paths[parent]
		if !ok {
			t.Errorf("parent command %q is not reachable", parent)
			continue
		}
		var got []string
		for _, sub := range cmd.Commands() {
			got = append(got, sub.Name())
		}
		assertStringSet(t, "subcommands of "+parent, want, got)
	}
}

func TestCLI_TotalCommandCount(t *testing.T) {
	paths := walkCommandPaths(t)
	// Root + 56 top-level + 74 subcommands. Any add/remove must update this
	// pin deliberately, with the group sets above saying where it landed.
	if len(paths) != 131 {
		t.Errorf("reachable command paths = %d, want 131", len(paths))
	}
}

func assertStringSet(t *testing.T, what string, want, got []string) {
	t.Helper()
	sort.Strings(want)
	sort.Strings(got)
	if len(want) != len(got) {
		t.Errorf("%s: got %v, want %v", what, got, want)
		return
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("%s: got %v, want %v", what, got, want)
			return
		}
	}
}
