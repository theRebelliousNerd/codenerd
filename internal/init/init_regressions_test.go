package init

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/northstar"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// The config.json init writes for a fresh workspace names no provider, model
// or engine and no provider-tuned window: those are the user's choices.
func TestCreateDefaultConfig_NamesNoProviderOrModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := (&Initializer{}).createDefaultConfig(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadUserConfig(path)
	if err != nil {
		t.Fatalf("the written config does not load: %v", err)
	}
	if cfg.HasExplicitLLMSelection() || cfg.Provider != "" || cfg.Model != "" || cfg.Engine != "" {
		t.Fatalf("init chose an LLM: provider=%q model=%q engine=%q", cfg.Provider, cfg.Model, cfg.Engine)
	}
	if cfg.Gemini != nil || cfg.ContextWindow != nil {
		t.Fatal("init wrote provider-specific settings or a provider-tuned context window")
	}
	if cfg.Embedding != nil && (cfg.Embedding.Provider != "" || cfg.Embedding.OllamaEndpoint != "") {
		t.Fatalf("init chose an embedding provider: %+v", cfg.Embedding)
	}
}

// A session's shard manager is told about created agents and nothing else:
// init used to start a second copy of every auto-start system shard on it
// and switch its client.
func TestInitialize_LeavesACallersShardManagerAlone(t *testing.T) {
	root := orientationFixture(t)
	sm := coreshards.NewShardManager()
	spawned := 0
	sm.RegisterShard("probe_system", func(id string, cfg types.ShardConfig) types.ShardAgent {
		spawned++
		return coreshards.NewBaseShardAgent(id, cfg)
	})
	sm.DefineProfile("probe_system", types.ShardConfig{Name: "probe_system", Type: types.ShardTypeSystem, StartupMode: types.StartupAuto})

	cfg := DefaultInitConfig(root)
	cfg.Output = nil
	cfg.Interactive = false
	cfg.SkipResearch = true
	cfg.ShardManager = sm
	cfg.EmbeddingEngine = fixedInitEngine{}
	ini, err := NewInitializer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer ini.Close()
	result, err := ini.Initialize(context.Background())
	if err != nil || !result.Success {
		t.Fatalf("Initialize: %v failures=%v", err, result.Failures)
	}
	if spawned != 0 || len(sm.GetActiveShards()) != 0 {
		t.Fatalf("init started system shards on the caller's manager (%d spawned)", spawned)
	}
	for _, agent := range result.CreatedAgents {
		profile, ok := sm.GetProfile(agent.Name)
		if !ok || profile.Type != types.ShardTypeUser || profile.KnowledgePath != agent.KnowledgePath {
			t.Fatalf("agent %s registered as %+v, want boot's user-agent profile", agent.Name, profile)
		}
	}
}

// Re-init never replaces the session a user can resume.
func TestSeedSessionState_KeepsAnExistingSession(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, ".nerd", "session.json")
	ini := &Initializer{config: InitConfig{Workspace: ws}}
	created, err := ini.seedSessionState(path)
	if err != nil || !created {
		t.Fatalf("first seed: created=%v err=%v", created, err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	created, err = ini.seedSessionState(path)
	if err != nil || created {
		t.Fatalf("second seed: created=%v err=%v", created, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatal("re-init replaced the existing session")
	}
}

func strategicStore(t *testing.T) *store.LocalStore {
	t.Helper()
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetEmbeddingEngine(fixedInitEngine{})
	return db
}

func conceptCounts(t *testing.T, db *store.LocalStore, prefix string) map[string]int {
	t.Helper()
	atoms, err := db.GetKnowledgeAtomsByPrefix(prefix)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, atom := range atoms {
		counts[atom.Concept+" = "+atom.Content]++
	}
	return counts
}

// Strategic knowledge is the vision of record, replaced on every write: a
// re-run adds no duplicate, a new vision retires the old one, and "unknown"
// or an empty derivation writes no atom.
func TestPersistStrategicKnowledge_ReplacesAndStoresNoFiller(t *testing.T) {
	db := strategicStore(t)
	ini := &Initializer{}
	ctx := context.Background()
	vision := &northstar.Vision{Mission: "Wire what you write", VisionStmt: "Every change is reached", Constraints: []string{"never skip tests"}}

	for range 2 {
		if _, err := ini.PersistStrategicKnowledge(ctx, strategicKnowledgeFromVision(vision), db); err != nil {
			t.Fatal(err)
		}
	}
	for concept, n := range conceptCounts(t, db, "strategic/") {
		if n != 1 {
			t.Fatalf("%s stored %d times after two identical writes", concept, n)
		}
	}

	vision.VisionStmt = "Every change is reached and measured"
	if _, err := ini.PersistStrategicKnowledge(ctx, strategicKnowledgeFromVision(vision), db); err != nil {
		t.Fatal(err)
	}
	counts := conceptCounts(t, db, "strategic/vision")
	if counts["strategic/vision = Every change is reached"] != 0 || counts["strategic/vision = Every change is reached and measured"] != 1 {
		t.Fatalf("vision atoms after a change: %v", counts)
	}

	if n, err := ini.PersistStrategicKnowledge(ctx, &StrategicKnowledge{ArchitectureStyle: "unknown"}, db); err != nil || n != 0 {
		t.Fatalf("filler wrote %d atoms (err %v)", n, err)
	}
	if strategicKnowledgeFromVision(nil) != nil {
		t.Fatal("no vision must yield no knowledge, not a limitation that says so")
	}
	if counts := conceptCounts(t, db, "strategic/architecture_style"); len(counts) != 0 {
		t.Fatalf("\"unknown\" was stored as knowledge: %v", counts)
	}
}

func TestSyncStrategicSources_RetiresRemovedAndKeepsUnchanged(t *testing.T) {
	db := strategicStore(t)
	ctx := context.Background()
	if err := syncStrategicSources(ctx, db, map[string]string{"strategic/source/a.md": "A", "strategic/source/b.md": "B"}); err != nil {
		t.Fatal(err)
	}
	before, err := db.GetKnowledgeAtomsByPrefix("strategic/source/a.md")
	if err != nil || len(before) != 1 {
		t.Fatalf("a.md atoms = %v err %v", before, err)
	}
	if err := syncStrategicSources(ctx, db, map[string]string{"strategic/source/a.md": "A", "strategic/source/c.md": "C2"}); err != nil {
		t.Fatal(err)
	}
	counts := conceptCounts(t, db, "strategic/source/")
	want := map[string]int{"strategic/source/a.md = A": 1, "strategic/source/c.md = C2": 1}
	if len(counts) != len(want) || counts["strategic/source/a.md = A"] != 1 || counts["strategic/source/c.md = C2"] != 1 {
		t.Fatalf("sources = %v, want %v", counts, want)
	}
	after, err := db.GetKnowledgeAtomsByPrefix("strategic/source/a.md")
	if err != nil || len(after) != 1 || after[0].ID != before[0].ID {
		t.Fatal("an unchanged source was rewritten")
	}
}

// The document refresh behind /refresh-docs: OrientationDocuments retains
// the engine extraction needs, extraction runs on the main client and
// synthesis on the planner, an unchanged document is not extracted again,
// --force extracts everything, and a reply that is not atoms stores nothing.
func TestDocumentRefresh_ExtractsIncrementallyOnTheRightClients(t *testing.T) {
	root := orientationFixture(t)
	setup := DefaultInitConfig(root)
	setup.Output, setup.Interactive, setup.SkipResearch, setup.SkipAgentCreate = nil, false, true, true
	setup.EmbeddingEngine = fixedInitEngine{}
	setup.LLMClient = &orientationClient{}
	first, err := NewInitializer(setup)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := first.Initialize(context.Background()); err != nil || !result.Success {
		t.Fatalf("Initialize: %v %v", err, result.Failures)
	}
	_ = first.Close()

	extractReply := `[{"concept":"architecture/evidence","content":"Decisions come from repository evidence.","confidence":0.9}]`
	main := &identifiedClient{provider: "meta", model: "worker-model", reply: func(system, _ string) (string, error) {
		if strings.Contains(system, "Extract key knowledge atoms") {
			return extractReply, nil
		}
		return "", nil
	}}
	planner := &identifiedClient{provider: "anthropic", model: "planner-model", reply: func(_, user string) (string, error) {
		return `{"project_vision":"Evidence-led repository decisions","core_philosophy":"Measure first"}`, nil
	}}
	db := strategicStore(t)
	refresh := func(force bool) *DocIngestionState {
		t.Helper()
		ini, err := NewInitializer(InitConfig{Workspace: root, LLMClient: main, PlannerClient: planner, EmbeddingEngine: fixedInitEngine{}})
		if err != nil {
			t.Fatal(err)
		}
		defer ini.Close()
		docs, _, err := ini.OrientationDocuments(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		state, err := ini.ProcessDocumentsWithTracking(context.Background(), docs, db, nil, force)
		if err != nil {
			t.Fatalf("ProcessDocumentsWithTracking: %v", err)
		}
		// As runDocRefresh does: synthesis runs when this refresh stored any.
		if state.SynthesisReady && state.TotalStored > 0 {
			if _, err := ini.SynthesizeFromStoredAtoms(context.Background(), db, state); err != nil {
				t.Fatalf("SynthesizeFromStoredAtoms: %v", err)
			}
		}
		return state
	}

	state := refresh(false)
	if state.TotalStored == 0 || state.AtomsStored != state.TotalStored {
		t.Fatalf("first refresh state = %+v", state)
	}
	extractions := len(main.requests())
	if extractions == 0 || len(planner.requests()) != 1 {
		t.Fatalf("main=%d planner=%d requests; want extraction on main, one synthesis on the planner", extractions, len(planner.requests()))
	}

	state = refresh(false)
	if state.TotalStored != 0 || len(main.requests()) != extractions || len(planner.requests()) != 1 {
		t.Fatalf("an unchanged refresh extracted or synthesized again: stored=%d main=%d planner=%d",
			state.TotalStored, len(main.requests()), len(planner.requests()))
	}

	extractReply = "I could not find structured knowledge in this chunk."
	state = refresh(true)
	if len(main.requests()) == extractions {
		t.Fatal("--force did not extract again")
	}
	if atoms, _ := db.GetKnowledgeAtomsByPrefix("doc/"); len(atoms) != 0 {
		t.Fatalf("a reply that is not atoms left %d atoms (the old ones retired, nothing invented)", len(atoms))
	}
}

// A non-interactive init honors an agent the user declined interactively.
func TestCurateAgents_NonInteractiveHonorsRecordedRejections(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SaveAgentPreferences(ws, &AgentSelectionPreferences{RejectedAgents: []string{"RodExpert"}}); err != nil {
		t.Fatal(err)
	}
	ini := &Initializer{config: InitConfig{Workspace: ws, Interactive: false}}
	result := &InitResult{}
	kept := ini.curateAgents(context.Background(), []RecommendedAgent{
		{Name: "GoExpert"}, {Name: "RodExpert"}, {Name: "rodexpert", Type: typeUAgentType},
	}, ProjectProfile{}, result)
	if names := agentNames(kept); !slices.Equal(names, []string{"GoExpert", "rodexpert"}) {
		t.Fatalf("kept = %v; a declined agent was recreated or a user-defined one dropped", names)
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "declined in an earlier agent selection") {
		t.Fatalf("warnings = %v", result.Warnings)
	}
}

// agents.json keeps agents an earlier run or the user registered, drops the
// declined, and grants no tool boot would refuse.
func TestRunPhase11Registry_KeepsOtherAgentsAndRestrictsGrants(t *testing.T) {
	ws := t.TempDir()
	nerdDir := filepath.Join(ws, ".nerd")
	if err := os.MkdirAll(filepath.Join(nerdDir, "shards"), 0o755); err != nil {
		t.Fatal(err)
	}
	userKB := filepath.Join(nerdDir, "shards", "customexpert_knowledge.db")
	if err := os.WriteFile(userKB, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ini := &Initializer{config: InitConfig{Workspace: ws}}
	if err := ini.saveAgentRegistry(filepath.Join(nerdDir, "agents.json"), []CreatedAgent{
		{Name: "CustomExpert", KnowledgePath: userKB},
		{Name: "GoneExpert", KnowledgePath: filepath.Join(nerdDir, "shards", "gone_knowledge.db")},
		{Name: "RodExpert"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := SaveAgentPreferences(ws, &AgentSelectionPreferences{RejectedAgents: []string{"RodExpert"}}); err != nil {
		t.Fatal(err)
	}
	ini.registeredTools = []ToolDefinition{{Name: "go_test"}, {Name: "go_build"}}
	result := &InitResult{CreatedAgents: []CreatedAgent{{
		Name: "TSExpert", Tools: []string{"npm_test", "go_test"}, ToolPreferences: map[string]string{"test": "npm_test", "build": "go_build"},
	}}}
	ini.runPhase11Registry(newPhaseRunner(ini), result, nerdDir)

	data, err := os.ReadFile(filepath.Join(nerdDir, "agents.json"))
	if err != nil {
		t.Fatal(err)
	}
	var registry AgentRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatal(err)
	}
	byName := map[string]CreatedAgent{}
	for _, agent := range registry.Agents {
		byName[agent.Name] = agent
	}
	if _, ok := byName["CustomExpert"]; !ok {
		t.Fatalf("the user's agent was dropped: %s", data)
	}
	if _, ok := byName["GoneExpert"]; ok {
		t.Fatal("an agent whose knowledge base is gone was kept")
	}
	if _, ok := byName["RodExpert"]; ok {
		t.Fatal("a declined agent was kept")
	}
	ts := byName["TSExpert"]
	if !slices.Equal(ts.Tools, []string{"go_test"}) || ts.ToolPreferences["test"] != "" || ts.ToolPreferences["build"] != "go_build" {
		t.Fatalf("TSExpert grants = %v %v; an unregistered tool reached agents.json", ts.Tools, ts.ToolPreferences)
	}
}

// An upgrade whose knowledge base exists but whose prompts.yaml does not
// (an earlier run could not write it) writes prompts.yaml now.
func TestCreateAgent_WritesMissingPromptsOnUpgrade(t *testing.T) {
	ws := t.TempDir()
	shards := filepath.Join(ws, ".nerd", "shards")
	if err := os.MkdirAll(shards, 0o755); err != nil {
		t.Fatal(err)
	}
	ini := &Initializer{config: InitConfig{Workspace: ws, SkipResearch: true}, embedEngine: fixedInitEngine{}}
	agent := RecommendedAgent{Name: "GoExpert", Description: "Go", Topics: []string{"go testing"}}
	if res := ini.createAgent(context.Background(), shards, agent, nil); res.Error != nil {
		t.Fatal(res.Error)
	}
	promptsPath := filepath.Join(ws, ".nerd", "agents", "goexpert", "prompts.yaml")
	if err := os.Remove(promptsPath); err != nil {
		t.Fatal(err)
	}
	res := ini.createAgent(context.Background(), shards, agent, nil)
	if res.Error != nil || !res.UpgradeMode {
		t.Fatalf("upgrade: %+v", res)
	}
	if _, err := os.Stat(promptsPath); err != nil {
		t.Fatalf("prompts.yaml was not written on upgrade: %v", err)
	}
}

// With no Output configured, init writes nothing to the process's standard
// output: in the chat TUI that output went straight into the alt screen.
func TestInitialize_NilOutputWritesNothingToStdout(t *testing.T) {
	root := orientationFixture(t)
	cfg := InitConfig{Workspace: root, SkipResearch: true, SkipAgentCreate: true, EmbeddingEngine: fixedInitEngine{}}
	ini, err := NewInitializer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer ini.Close()
	out := captureStdout(t, func() {
		if _, err := ini.Initialize(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	if strings.TrimSpace(out) != "" {
		t.Fatalf("init wrote to stdout with no Output configured:\n%.400s", out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			b.Write(buf[:n])
			if err != nil {
				break
			}
		}
		done <- b.String()
	}()
	defer func() { os.Stdout = saved }()
	fn()
	_ = w.Close()
	os.Stdout = saved
	select {
	case got := <-done:
		return got
	case <-time.After(10 * time.Second):
		t.Fatal("stdout capture did not finish")
		return ""
	}
}
