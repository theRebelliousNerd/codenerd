package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// writeConfigCheckFile seeds one file under a temp workspace's .nerd dir.
func writeConfigCheckFile(t *testing.T, ws, rel, content string) {
	t.Helper()
	p := filepath.Join(ws, ".nerd", rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeConfigCheckAgents builds a minimal agents.json (the registry shape
// loadAgentRegistry reads: version + agents with name/tools) at test time.
func writeConfigCheckAgents(t *testing.T, ws string, agents map[string][]string) {
	t.Helper()
	type entry struct {
		Name  string   `json:"name"`
		Tools []string `json:"tools,omitempty"`
	}
	doc := struct {
		Version string  `json:"version"`
		Agents  []entry `json:"agents"`
	}{Version: "1.5.0"}
	for name, tools := range agents {
		doc.Agents = append(doc.Agents, entry{Name: name, Tools: tools})
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	writeConfigCheckFile(t, ws, "agents.json", string(raw))
}

// runConfigCheckCaptured runs runConfigCheck against ws and returns its stdout.
func runConfigCheckCaptured(t *testing.T, ws string) (string, error) {
	t.Helper()
	withWorkspace(t, ws)
	saved := configNoImplicit
	configNoImplicit = false
	t.Cleanup(func() { configNoImplicit = saved })
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := runConfigCheck(cmd, nil)
	return buf.String(), err
}

// An agents.json entry naming a tool the host never registered must show up
// in `nerd config check` naming both the agent and the tool; an entry whose
// tool boot accepts (read_file is in the /general envelope via /core, so it
// passes with no tool_registered fact) must stay silent.
func TestConfigCheck_ReportsUnregisteredAgentTool(t *testing.T) {
	ws := t.TempDir()
	writeConfigCheckFile(t, ws, "config.json", `{}`)
	writeConfigCheckAgents(t, ws, map[string][]string{
		"BadExpert":  {"zz_no_such_tool"},
		"GoodExpert": {"read_file"},
	})

	out, err := runConfigCheckCaptured(t, ws)
	if err != nil {
		t.Fatalf("warnings must not fail the command: %v\n%s", err, out)
	}
	for _, want := range []string{"BadExpert", "zz_no_such_tool", "[warning]"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not mention %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "GoodExpert") {
		t.Errorf("registered tool reported as a problem:\n%s", out)
	}
}

// A tool from .nerd/tools/available_tools.json is registered at boot by
// HydrateStaticTools, so an agent declaring it is accepted there and must be
// accepted here too. Without the same hydration this test fails: the tool is
// in no persona envelope, so a bare kernel refuses it.
func TestConfigCheck_StaticToolIsNotReported(t *testing.T) {
	ws := t.TempDir()
	writeConfigCheckFile(t, ws, "config.json", `{}`)
	toolsDoc, err := json.Marshal([]map[string]string{
		{"name": "zz_static_tool", "command": "true", "shard_affinity": "CoderShard"},
	})
	if err != nil {
		t.Fatal(err)
	}
	writeConfigCheckFile(t, ws, filepath.Join("tools", "available_tools.json"), string(toolsDoc))
	writeConfigCheckAgents(t, ws, map[string][]string{
		"StaticExpert": {"zz_static_tool"},
	})

	out, err := runConfigCheckCaptured(t, ws)
	if err != nil {
		t.Fatalf("warnings must not fail the command: %v\n%s", err, out)
	}
	if strings.Contains(out, "StaticExpert") || strings.Contains(out, "zz_static_tool") {
		t.Errorf("static tool from available_tools.json reported as a problem:\n%s", out)
	}
}

// No agents.json (or an unreadable one) is not a problem: boot treats the
// registry as best-effort and so does the check.
func TestConfigCheck_MissingAgentsFileIsSilent(t *testing.T) {
	ws := t.TempDir()
	writeConfigCheckFile(t, ws, "config.json", `{}`)

	out, err := runConfigCheckCaptured(t, ws)
	if err != nil {
		t.Fatalf("a workspace without agents.json must still check: %v\n%s", err, out)
	}
	if strings.Contains(out, "agents.") {
		t.Errorf("no agents.json, yet agent problems were reported:\n%s", out)
	}
}

func TestConfigCheck_MalformedAgentsFileIsSilent(t *testing.T) {
	ws := t.TempDir()
	writeConfigCheckFile(t, ws, "config.json", `{}`)
	writeConfigCheckFile(t, ws, "agents.json", `{"agents": [`)

	out, err := runConfigCheckCaptured(t, ws)
	if err != nil {
		t.Fatalf("a malformed agents.json must not fail the command: %v\n%s", err, out)
	}
	if strings.Contains(out, "agents.") {
		t.Errorf("malformed agents.json produced agent problems:\n%s", out)
	}
}
