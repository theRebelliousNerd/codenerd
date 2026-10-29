package system

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/experts"
	"codenerd/internal/orient"
	"codenerd/internal/types"
)

// healFixtureWorkspace lays down a workspace shaped like a .nerd that predates
// agents.json domains: four ready agents with no domains field, each with a
// prompts.yaml (boot syncs it into the agent's KB), and the orientation
// snapshot init leaves behind, whose profile signals say Go, Bubble Tea and
// Mangle.
func healFixtureWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	names := []string{"GoExpert", "MangleExpert", "BubbleTeaExpert", "ContextEngineeringExpert"}
	var agents []map[string]any
	for _, name := range names {
		dir := filepath.Join(ws, ".nerd", "agents", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		yml := "- id: \"" + name + "/methodology\"\n  category: \"methodology\"\n  subcategory: \"" + name + "\"\n  is_mandatory: false\n  description: \"Methodology for " + name + "\"\n  content: |\n    " + name + " methodology.\n"
		if err := os.WriteFile(filepath.Join(dir, "prompts.yaml"), []byte(yml), 0o644); err != nil {
			t.Fatalf("write prompts.yaml: %v", err)
		}
		agents = append(agents, map[string]any{"name": name, "type": "persistent", "status": "ready", "kb_size": 3})
	}
	writeAgentsJSON(t, ws, agents)

	var facts []types.Fact
	for _, s := range [][2]string{{"/language", "go"}, {"/dependency", "bubbletea"}, {"/dependency", "mangle"}, {"/tests", "present"}} {
		facts = append(facts, types.Fact{Predicate: "profile_signal", Args: []any{types.MangleAtom(s[0]), s[1]}})
	}
	body, err := json.Marshal(orient.Snapshot{Config: config.DefaultOrientConfig(), Facts: facts})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	orientDir := filepath.Join(ws, ".nerd", "orientation")
	if err := os.MkdirAll(orientDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orientDir, "snapshot.json"), body, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	return ws
}

func recordedDomains(t *testing.T, ws string) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(ws, ".nerd", "agents.json"))
	if err != nil {
		t.Fatalf("read agents.json: %v", err)
	}
	var reg struct {
		Agents []struct {
			Name    string    `json:"name"`
			Domains *[]string `json:"domains"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatalf("decode agents.json: %v", err)
	}
	out := map[string][]string{}
	for _, a := range reg.Agents {
		if a.Domains != nil {
			out[a.Name] = *a.Domains
		}
	}
	return out
}

// An existing workspace heals on boot without init: each ready agent with no
// domains gets the init catalog's key for it from the saved orientation
// (orient_agent_domain), asserted as /catalog, and boot writes the result back
// to agents.json -- "none" included, as an empty list -- so the derivation
// runs once. After that boot a fix on a policy file needs the Mangle expert,
// which no agent in this registry could be needed for before.
func TestBootHealsAgentDomainsAnExistingWorkspaceNeverRecorded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := healFixtureWorkspace(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	if !cortex.SessionExecutor.HasExpertConsultant() || !cortex.SessionExecutor.CloneForTask().HasExpertConsultant() {
		t.Error("boot wired no consultation spawner: expert_spawn would never run on the executor path")
	}
	domains, err := cortex.Kernel.Query("shard_agent_domain")
	if err != nil {
		t.Fatalf("query shard_agent_domain: %v", err)
	}
	for _, want := range [][2]string{{"/goexpert", "/go"}, {"/mangleexpert", "/mangle"}, {"/bubbleteaexpert", "/bubbletea"}} {
		if !factWith(domains, want[0], want[1], "/catalog") {
			t.Errorf("catalog domain %v missing after boot: %v", want, domains)
		}
	}
	sel, err := experts.Select(cortex.Kernel, experts.Task{Verb: "/fix", Targets: []string{"internal/core/defaults/policy/shards.mg"}, Workspace: ws})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if got := sel.Agents(); len(got) != 1 || got[0] != "mangleexpert" {
		t.Errorf("a fix on a policy file needs %v after the heal, want [mangleexpert]", got)
	}
	recorded := recordedDomains(t, ws)
	requireWorkspaceReleased(t, cortex, ws)
	for name, want := range map[string][]string{
		"GoExpert":                 {"go"},
		"MangleExpert":             {"mangle"},
		"BubbleTeaExpert":          {"bubbletea"},
		"ContextEngineeringExpert": {},
	} {
		got, ok := recorded[name]
		if !ok || !slices.Equal(got, want) {
			t.Errorf("agents.json domains for %s = %v (recorded %v), want %v", name, got, ok, want)
		}
	}
}

func newRosterKernel(t *testing.T) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	return k
}

// The second boot reads what the first recorded (/registry) and writes
// nothing.
func TestRecordedDomainsAreReadBackNotDerivedAgain(t *testing.T) {
	ws := healFixtureWorkspace(t)
	k := newRosterKernel(t)
	if err := refreshShardRoster(context.Background(), k, ws, nil); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(ws, ".nerd", "agents.json"))
	if err != nil {
		t.Fatal(err)
	}
	k2 := newRosterKernel(t)
	if err := refreshShardRoster(context.Background(), k2, ws, nil); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(ws, ".nerd", "agents.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("the second boot rewrote agents.json:\n%s\n--\n%s", before, after)
	}
	rows, err := k2.Query("shard_agent_domain")
	if err != nil {
		t.Fatal(err)
	}
	if !factWith(rows, "/goexpert", "/go", "/registry") || factWith(rows, "/goexpert", "/go", "/catalog") {
		t.Errorf("the second boot should read /registry and derive no /catalog rows: %v", rows)
	}
}
