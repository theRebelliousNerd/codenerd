package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/experts"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/types"
)

// A concept agent (mangle-logic-architect, SecurityAuditor, TestArchitect) was
// created for what it is for, not for a technology the init catalog keyed, so
// its agents.json entry has `domains: []`. Before concept domains the kernel
// never needed it: expert_needed joins a task's technologies to
// shard_agent_domain, and the agent had none. Its name says what it holds, and
// the kernel derives it: a word of the name that is a language, a technology
// or a concept term is a /agent_name domain, and a concept's verb names it.
func TestConceptAgentsAreNeededThroughDomainsTheKernelDerives(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	var agents []map[string]any
	for _, name := range []string{"mangle-logic-architect", "SecurityAuditor", "TestArchitect", "arch_propose_auditor"} {
		dir := filepath.Join(ws, ".nerd", "agents", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		prompts := "- id: \"" + name + "/identity\"\n  category: \"identity\"\n  subcategory: \"" + name + "\"\n  is_mandatory: true\n  description: \"Identity\"\n  content: |\n    You are " + name + ".\n"
		if err := os.WriteFile(filepath.Join(dir, "prompts.yaml"), []byte(prompts), 0o644); err != nil {
			t.Fatalf("write prompts.yaml: %v", err)
		}
		agents = append(agents, map[string]any{"name": name, "type": "persistent", "status": "ready", "domains": []string{}})
	}
	// A technology agent with recorded domains: the control for the decision.
	agents = append(agents, map[string]any{"name": "GoExpert", "type": "persistent", "status": "ready", "domains": []string{"go"}})
	dir := filepath.Join(ws, ".nerd", "agents", "GoExpert")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	goPrompts := "- id: \"goexpert/identity\"\n  category: \"identity\"\n  subcategory: \"goexpert\"\n  is_mandatory: true\n  description: \"Identity\"\n  content: |\n    You are GoExpert.\n"
	if err := os.WriteFile(filepath.Join(dir, "prompts.yaml"), []byte(goPrompts), 0o644); err != nil {
		t.Fatalf("write prompts.yaml: %v", err)
	}
	writeAgentsJSON(t, ws, agents)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)

	domains, err := cortex.Kernel.Query("shard_agent_domain")
	if err != nil {
		t.Fatalf("query shard_agent_domain: %v", err)
	}
	for _, want := range [][]string{
		{"/mangle-logic-architect", "/mangle", "/agent_name"},
		{"/securityauditor", "/security", "/agent_name"},
		{"/testarchitect", "/testing", "/agent_name"},
		{"/goexpert", "/go", "/registry"},
	} {
		if !factWith(domains, want...) {
			t.Errorf("shard_agent_domain lacks %v; rows: %v", want, domains)
		}
	}
	for _, f := range domains {
		if len(f.Args) > 0 && strings.Contains(strings.ToLower(types.ExtractString(f.Args[0])), "arch_propose_auditor") {
			t.Errorf("an agent whose name states no domain was given one: %v", f)
		}
	}

	needs := func(verb string, targets ...string) []string {
		t.Helper()
		sel, err := experts.Select(cortex.Kernel, experts.Task{Verb: verb, Targets: targets, Workspace: ws})
		if err != nil {
			t.Fatalf("Select(%s %v): %v", verb, targets, err)
		}
		return sel.Agents()
	}
	has := func(list []string, name string) bool {
		for _, n := range list {
			if n == name {
				return true
			}
		}
		return false
	}

	// Mangle work needs the Mangle architect, and only it.
	got := needs("/fix", "internal/x/policy.mg")
	if !has(got, "mangle-logic-architect") {
		t.Errorf("a fix on a .mg file needs mangle-logic-architect, got %v", got)
	}
	if has(got, "goexpert") || has(got, "securityauditor") || has(got, "testarchitect") {
		t.Errorf("a fix on a .mg file needs neither the Go, security nor test expert, got %v", got)
	}
	// Near miss: Go work does not.
	if got := needs("/fix", "internal/x/main.go"); has(got, "mangle-logic-architect") || !has(got, "goexpert") {
		t.Errorf("a fix on a .go file needs goexpert and not mangle-logic-architect, got %v", got)
	}
	// A concept is needed by the verb that names it, on any file.
	if got := needs("/security", "internal/x/main.go"); !has(got, "securityauditor") {
		t.Errorf("a security task needs securityauditor, got %v", got)
	}
	if got := needs("/test", "internal/x/main.go"); !has(got, "testarchitect") {
		t.Errorf("a test task needs testarchitect, got %v", got)
	}
	if got := needs("/fix", "internal/x/main.go"); has(got, "securityauditor") || has(got, "testarchitect") {
		t.Errorf("a fix names no concept, but got %v", got)
	}
	// A concept expert's evidence is a verb match: knowledge is pushed into the
	// turn and nothing is spawned for it.
	sel, err := experts.Select(cortex.Kernel, experts.Task{Verb: "/security", Targets: []string{"internal/x/main.go"}, Workspace: ws})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	for _, n := range sel.Needs {
		if n.Agent == "securityauditor" && n.Spawn != "" {
			t.Errorf("a verb match spawned %s as %q; only import evidence spawns", n.Agent, n.Spawn)
		}
	}

	// The knowledge refresh reads the same domains, so a concept agent has
	// subjects to know, and a technology agent's stay what they were.
	r := &knowledgeRefresher{workspace: ws, kernel: cortex.Kernel}
	techs, err := r.domains()
	if err != nil {
		t.Fatalf("refresher domains: %v", err)
	}
	subjects := nerdinit.KnowledgeSubjects(ws, "securityauditor", techs["securityauditor"], nil, nil)
	if len(subjects) == 0 || subjects[0].Technology != "security" {
		t.Errorf("securityauditor has no security subjects: %+v", subjects)
	}
	if !has(techs["mangle-logic-architect"], "mangle") {
		t.Errorf("the refresh does not see mangle-logic-architect's /mangle domain: %v", techs["mangle-logic-architect"])
	}

	// Nothing was written back: the derivation is the kernel's every boot.
	data, err := os.ReadFile(filepath.Join(ws, ".nerd", "agents.json"))
	if err != nil {
		t.Fatalf("read agents.json: %v", err)
	}
	if strings.Contains(string(data), `"mangle"`) || strings.Contains(string(data), `"security"`) {
		t.Errorf("a derived concept domain was written into agents.json: %s", data)
	}
}
