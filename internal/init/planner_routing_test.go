package init

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/store"
	"codenerd/internal/types"
)

// groundedClient is a main client with grounded web search, the way the
// Meta worker is configured.
type groundedClient struct {
	identifiedClient
	mu      sync.Mutex
	queries []string
}

func (c *groundedClient) SupportsGroundedWebSearch() bool { return true }

func (c *groundedClient) GroundedWebSearch(_ context.Context, query string) (*types.GroundedWebSearchResult, error) {
	c.mu.Lock()
	c.queries = append(c.queries, query)
	c.mu.Unlock()
	body := "Finding one about " + query + ": agents that skip wiring leave code no entry point reaches.\n\n" +
		"Finding two about " + query + ": regression suites that drive the shipped entry points catch it."
	return &types.GroundedWebSearchResult{
		Text:      body,
		Citations: []types.GroundedCitation{{URL: "https://example.test/agents/" + sanitizeToolName(query)}},
	}, nil
}

func (c *groundedClient) searched() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.queries...)
}

var docLine = regexp.MustCompile(`(?m)^@@DOC ([^\n]+)`)

// scriptedPlanner answers every planner-phase request init makes, and fails
// a request nothing scripted, so an unexpected call is visible.
func scriptedPlanner(system, user string) (string, error) {
	switch {
	case strings.HasPrefix(user, "@@DOC "):
		var documents []map[string]any
		for _, match := range docLine.FindAllStringSubmatch(user, -1) {
			role, evidence := "instructions", "Review repository changes."
			if match[1] == "README.md" {
				role, evidence = "vision", "Build a coding agent that wires what it writes and does not waste tokens."
			}
			documents = append(documents, map[string]any{"path": match[1], "roles": []map[string]any{{"role": role, "confidence_pct": 95, "evidence": evidence}}, "themes": []string{"coding agents"}})
		}
		body, err := json.Marshal(map[string]any{"documents": documents})
		return string(body), err
	case strings.HasPrefix(user, "@@BRIEF"):
		return `{"Mission":"Build a coding agent that wires what it writes.","Problem":"Coding agents forget to wire code and waste tokens.","Vision":"An agent whose every change is reached from a shipped entry point.","FieldSources":{"Mission":["README.md"],"Problem":["README.md"],"Vision":["README.md"]}}`, nil
	case strings.Contains(system, "Propose the domain experts"):
		return `{"experts":[
			{"name":"AgentWiringExpert","mission":"Keep agent-written code reachable from shipped entry points","why":"the north star says every change must be reached","cites":["north_star"],
			 "topics":["entry-point wiring"],"failure_modes":["code built but never called"],
			 "research":[{"question":"how do coding agents forget to wire new code","fast_moving":false},{"question":"which coding agent techniques shipped this quarter","fast_moving":true}]},
			{"name":"TokenEconomyExpert","mission":"Find where the agent wastes tokens","why":"the README names token waste","cites":["README.md","docs/missing.md"],
			 "topics":["prompt caching"],"research":[{"question":"where do coding agents waste tokens","fast_moving":false}]},
			{"name":"UncitedExpert","mission":"has no evidence","why":"because","cites":["nowhere.md"],"topics":["x"]},
			{"name":"lower-case","mission":"bad name","why":"x","cites":["north_star"],"topics":["x"]}
		],"core":[{"name":"SecurityAuditor","why":"the agent runs tools on the user's machine","cites":["north_star"]}]}`, nil
	case strings.Contains(system, "Choose which of this repository's own commands"):
		return `{"tools":[{"command":"make test","name":"test","category":"test","description":"Runs the fixture's tests."}]}`, nil
	case strings.Contains(system, "Write the methodology atoms"):
		return "## Methodology\n\n- Trace every artifact to its reader before calling work done.", nil
	case strings.Contains(system, "Write the domain atoms"):
		return "## Domain\n\n- Agents fail by building code that nothing calls.", nil
	}
	return "", fmt.Errorf("unscripted planner request: %.120q / %.120q", system, user)
}

// phaseOf names the init phase one recorded request belongs to.
func phaseOf(request string) string {
	switch {
	case strings.Contains(request, "@@DOC "):
		return "document_roles"
	case strings.Contains(request, "@@BRIEF"):
		return "north_star"
	case strings.Contains(request, "Propose the domain experts"):
		return "domain_experts"
	case strings.Contains(request, "Choose which of this repository's own commands"):
		return "project_tools"
	case strings.Contains(request, "Write the methodology atoms"), strings.Contains(request, "Write the domain atoms"):
		return "specialist_prompts"
	}
	return "other"
}

func phasesOf(requests []string) []string {
	seen := map[string]bool{}
	var phases []string
	for _, request := range requests {
		phase := phaseOf(request)
		if !seen[phase] {
			seen[phase] = true
			phases = append(phases, phase)
		}
	}
	slices.Sort(phases)
	return phases
}

// codingAgentFixture is a repository whose north star is about building a
// coding agent, with a Makefile and an imported subagent.
func codingAgentFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeInitFile(t, root, "README.md", "# Agent\nBuild a coding agent that wires what it writes and does not waste tokens.\n")
	writeInitFile(t, root, ".claude/agents/reviewer.md", "---\nname: reviewer\ndescription: Reviews changes\n---\nReview repository changes.\n")
	for _, name := range []string{"planning", "delivery", "review"} {
		writeInitFile(t, root, "docs/"+name+".md", "# "+name+"\n[Project](../README.md)\nReview repository changes.\n")
	}
	writeInitFile(t, root, "go.mod", "module example.com/agent\n\ngo 1.24\n")
	writeInitFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	writeInitFile(t, root, "main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}\n")
	writeInitFile(t, root, "Makefile", "test:\n\tgo test ./...\n\nbuild:\n\tgo build ./...\n")
	orientationFixtureGit(t, root, "init")
	orientationFixtureGit(t, root, "add", ".")
	orientationFixtureGit(t, root, "commit", "-m", "origin")
	return root
}

type routedInit struct {
	init    *Initializer
	result  *InitResult
	fetched *countingFetcher
}

// runRoutedInit drives a full Initialize through NewInitializer with the
// given clients, Context7 replaced by a counting fetcher.
func runRoutedInit(t *testing.T, root string, main, planner *groundedClient) routedInit {
	t.Helper()
	cfg := DefaultInitConfig(root)
	cfg.Output = nil
	cfg.Interactive = false
	cfg.LLMClient = main
	if planner != nil {
		cfg.PlannerClient = planner
	}
	cfg.EmbeddingEngine = fixedInitEngine{}
	ini, err := NewInitializer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ini.Close() })
	fetched := &countingFetcher{}
	ini.fetchTopic = fetched.fetch
	result, err := ini.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize: %v (failures %v)", err, result.Failures)
	}
	if !result.Success {
		t.Fatalf("init failures=%v warnings=%v", result.Failures, result.Warnings)
	}
	return routedInit{init: ini, result: result, fetched: fetched}
}

func newRoutedClients(withPlanner bool) (*groundedClient, *groundedClient) {
	main := &groundedClient{identifiedClient: identifiedClient{provider: "meta", model: "worker-model", reply: scriptedPlanner}}
	if !withPlanner {
		return main, nil
	}
	planner := &groundedClient{identifiedClient: identifiedClient{provider: "anthropic", model: "planner-model", reply: scriptedPlanner}}
	return main, planner
}

func createdNames(result *InitResult) []string {
	var names []string
	for _, agent := range result.CreatedAgents {
		names = append(names, agent.Name)
	}
	slices.Sort(names)
	return names
}

// With a planner configured, orientation (document roles and the north
// star), the domain-expert proposal, specialist prompts and project tool
// selection go to the planner; grounded domain research and nothing else
// goes to the main client; stack topics are fetched as documentation, not
// asked of a model.
func TestInitialize_PlannerPhasesHitThePlanner(t *testing.T) {
	root := codingAgentFixture(t)
	main, planner := newRoutedClients(true)
	run := runRoutedInit(t, root, main, planner)

	wantPlanner := []string{"document_roles", "domain_experts", "north_star", "project_tools", "specialist_prompts"}
	if got := phasesOf(planner.requests()); !slices.Equal(got, wantPlanner) {
		t.Fatalf("planner phases = %v, want %v", got, wantPlanner)
	}
	if got := main.requests(); len(got) != 0 {
		t.Fatalf("main client was sent planner work: %v", phasesOf(got))
	}
	if len(main.searched()) == 0 {
		t.Fatal("domain research did not reach the main client's grounded search")
	}
	if len(planner.searched()) != 0 {
		t.Fatal("domain research went to the planner")
	}
	if len(run.fetched.fetches) == 0 {
		t.Fatal("stack topics were not researched through the documentation fetch")
	}

	// The planner's experts reached the roster through the .mg rules, with
	// knowledge bases built from grounded, sourced, dated research.
	names := createdNames(run.result)
	for _, want := range []string{"AgentWiringExpert", "GoExpert", "SecurityAuditor", "TestArchitect", "TokenEconomyExpert", "reviewer"} {
		if !slices.Contains(names, want) {
			t.Fatalf("created agents = %v, want %s", names, want)
		}
	}
	for _, unwanted := range []string{"UncitedExpert", "lower-case"} {
		if slices.Contains(names, unwanted) {
			t.Fatalf("invalid proposal %s became an agent: %v", unwanted, names)
		}
	}
	joined := strings.Join(run.result.Warnings, "\n")
	if !strings.Contains(joined, "UncitedExpert discarded") {
		t.Fatalf("the discarded expert is not reported:\n%s", joined)
	}
	kb, err := store.NewLocalStore(filepath.Join(root, ".nerd", "shards", "agentwiringexpert_knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	atoms, err := kb.GetKnowledgeAtomsByPrefix("domain/")
	_ = kb.Close()
	if err != nil || len(atoms) == 0 {
		t.Fatalf("domain knowledge atoms = %d, err %v", len(atoms), err)
	}
	for _, atom := range atoms {
		if !strings.Contains(atom.Content, "Sources: https://example.test/") || !domainRetrievedLine.MatchString(atom.Content) {
			t.Fatalf("atom %s lacks its sources or retrieval date:\n%s", atom.Concept, atom.Content)
		}
	}

	// Metrics name each client and the phases it served.
	rows := map[string]InitLLMClientMetrics{}
	for _, row := range run.result.LLMMetrics.Clients {
		rows[row.Slot] = row
	}
	if !slices.Contains(rows["planner"].Phases, "north_star") || !slices.Contains(rows["main"].Phases, "domain_research") {
		t.Fatalf("client rows = %+v", run.result.LLMMetrics.Clients)
	}

	// The registry that boot reads carries the domain experts.
	data, err := os.ReadFile(filepath.Join(root, ".nerd", "agents.json"))
	if err != nil || !strings.Contains(string(data), "AgentWiringExpert") {
		t.Fatalf("agents.json does not register the domain expert: %v", err)
	}
}

// Without a planner every phase runs on the main client.
func TestInitialize_WithoutPlannerEverythingHitsMain(t *testing.T) {
	root := codingAgentFixture(t)
	main, _ := newRoutedClients(false)
	run := runRoutedInit(t, root, main, nil)

	want := []string{"document_roles", "domain_experts", "north_star", "project_tools", "specialist_prompts"}
	if got := phasesOf(main.requests()); !slices.Equal(got, want) {
		t.Fatalf("main phases = %v, want %v", got, want)
	}
	for _, row := range run.result.LLMMetrics.Clients {
		if row.Slot != "main" {
			t.Fatalf("a %s row exists with no planner configured: %+v", row.Slot, run.result.LLMMetrics.Clients)
		}
	}
}

// A second init over an unchanged repository redoes nothing the model was
// asked the first time, and keeps every artifact.
func TestInitialize_ReinitOnlyRedoesWhatChanged(t *testing.T) {
	root := codingAgentFixture(t)
	main, planner := newRoutedClients(true)
	first := runRoutedInit(t, root, main, planner)
	firstPlanner := len(planner.requests())
	firstSearches := len(main.searched())

	second := runRoutedInit(t, root, main, planner)
	if extra := planner.requests()[firstPlanner:]; len(extra) != 0 {
		t.Fatalf("an unchanged re-init asked the planner again: %v", phasesOf(extra))
	}
	if extra := main.searched()[firstSearches:]; len(extra) != 0 {
		t.Fatalf("an unchanged re-init researched fresh questions again: %v", extra)
	}
	if !slices.Equal(createdNames(first.result), createdNames(second.result)) {
		t.Fatalf("roster changed: %v then %v", createdNames(first.result), createdNames(second.result))
	}
	notes := strings.Join(second.result.Warnings, "\n")
	if !strings.Contains(notes, "kept their role claims") || !strings.Contains(notes, "stored north star was kept") {
		t.Fatalf("the re-init does not say what it reused:\n%s", notes)
	}

	// A fast-moving answer past the staleness bound is researched again.
	kbPath := filepath.Join(root, ".nerd", "shards", "agentwiringexpert_knowledge.db")
	kb, err := store.NewLocalStore(kbPath)
	if err != nil {
		t.Fatal(err)
	}
	fast := domainQuestion{Question: "which coding agent techniques shipped this quarter", FastMoving: true}
	prefix := domainConceptPrefix(fast)
	if _, err := kb.DeleteKnowledgeAtomsByPrefix(prefix); err != nil {
		t.Fatal(err)
	}
	old := time.Now().AddDate(0, 0, -90).Format(domainRetrievedLayout)
	if err := kb.StoreKnowledgeAtom(prefix+"section_0", "An old answer about agent techniques that a newer release has replaced.\n\nSources: https://example.test/old\nRetrieved: "+old, 0.85); err != nil {
		t.Fatal(err)
	}
	_ = kb.Close()
	before := len(main.searched())
	runRoutedInit(t, root, main, planner)
	refreshed := main.searched()[before:]
	if len(refreshed) != 1 || refreshed[0] != fast.Question {
		t.Fatalf("researched %v after the fast answer went stale, want only %q", refreshed, fast.Question)
	}
	kb, err = store.NewLocalStore(kbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()
	atoms, err := kb.GetKnowledgeAtomsByPrefix(prefix)
	if err != nil {
		t.Fatal(err)
	}
	for _, atom := range atoms {
		if strings.Contains(atom.Content, "https://example.test/old") {
			t.Fatal("the stale answer was kept beside its replacement")
		}
	}
}

// Without a north star or documents the roster is the stack catalog, and
// no domain experts are invented.
func TestInitialize_StackCatalogWithoutNorthStar(t *testing.T) {
	root := t.TempDir()
	writeInitFile(t, root, "go.mod", "module example.com/plain\n\ngo 1.24\n")
	writeInitFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	orientationFixtureGit(t, root, "init")
	orientationFixtureGit(t, root, "add", ".")
	orientationFixtureGit(t, root, "commit", "-m", "origin")

	cfg := DefaultInitConfig(root)
	cfg.Output = nil
	cfg.Interactive = false
	cfg.SkipResearch = true
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
	if names := createdNames(result); !slices.Equal(names, []string{"GoExpert"}) {
		t.Fatalf("created agents = %v, want the stack expert alone", names)
	}
}
