package init

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/store"
	"codenerd/internal/types"
)

func TestAgentsFromProfile_ReproducesTheOldSwitch(t *testing.T) {
	cases := []struct {
		name    string
		profile ProjectProfile
		want    []string
	}{
		{name: "go", profile: ProjectProfile{Language: "go"}, want: []string{"GoExpert"}},
		{name: "go+gin", profile: ProjectProfile{Language: "go", Framework: "gin"}, want: []string{"GoExpert", "WebAPIExpert"}},
		{name: "go+rod", profile: ProjectProfile{Language: "go", Dependencies: []DependencyInfo{{Name: "rod"}}}, want: []string{"GoExpert", "RodExpert"}},
		{name: "javascript", profile: ProjectProfile{Language: "javascript"}, want: []string{"TSExpert"}},
		{name: "kotlin", profile: ProjectProfile{Language: "kotlin"}, want: []string{"AndroidExpert"}},
		{name: "empty", profile: ProjectProfile{}, want: []string{"SecurityAuditor", "TestArchitect"}},
		{name: "haskell", profile: ProjectProfile{Language: "haskell"}, want: []string{"SecurityAuditor", "TestArchitect"}},
		{name: "unknown", profile: ProjectProfile{Language: "unknown"}, want: []string{"SecurityAuditor", "TestArchitect"}},
		{name: "chromedp", profile: ProjectProfile{Dependencies: []DependencyInfo{{Name: "chromedp"}}}, want: []string{"BrowserAutomationExpert"}},
		{name: "go+chromedp", profile: ProjectProfile{Language: "go", Dependencies: []DependencyInfo{{Name: "chromedp"}}}, want: []string{"BrowserAutomationExpert", "GoExpert"}},
		{name: "module path is not rod", profile: ProjectProfile{Dependencies: []DependencyInfo{{Name: "github.com/go-rod/rod"}}}, want: []string{"SecurityAuditor", "TestArchitect"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agents, err := agentsFromProfile(context.Background(), tc.profile)
			if err != nil {
				t.Fatal(err)
			}
			got := agentNames(agents)
			slices.Sort(got)
			want := append([]string{}, tc.want...)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("agents = %v, want %v", got, want)
			}
		})
	}
}

func TestAgentsFromProfile_SamePrioritySortsByName(t *testing.T) {
	agents, err := agentsFromProfile(context.Background(), ProjectProfile{
		Language:     "go",
		Dependencies: []DependencyInfo{{Name: "rod"}, {Name: "mangle"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := agentNames(agents)
	want := []string{"GoExpert", "MangleExpert", "RodExpert"}
	if !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestAgentsFromProfile_FrameworkWhyUsesTheCatalogKey(t *testing.T) {
	agents, err := agentsFromProfile(context.Background(), ProjectProfile{Language: "go", Framework: "Gin"})
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range agents {
		if agent.Name != "WebAPIExpert" {
			continue
		}
		if agent.Reason != "gin framework detected - API expertise beneficial" {
			t.Fatalf("reason = %q", agent.Reason)
		}
		return
	}
	t.Fatal("WebAPIExpert missing")
}

func TestAgentsFromProfile_IgnoresModuleLanguages(t *testing.T) {
	agents, err := agentsFromProfile(context.Background(), ProjectProfile{
		Language: "go",
		Modules:  []ModuleProfile{{Path: "services/api", Language: "rust"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := agentNames(agents); !slices.Equal(got, []string{"GoExpert"}) {
		t.Fatalf("agents = %v", got)
	}
}

func TestProfileFacts_DependencyNamesAreExact(t *testing.T) {
	facts := profileFacts(ProjectProfile{Dependencies: []DependencyInfo{
		{Name: "rod"},
		{Name: "rod"},
		{Name: ""},
		{Name: " Rod "},
		{Name: "Rod"},
	}})
	var deps []string
	for _, fact := range facts {
		if fact.Predicate == "profile_signal" && types.ExtractString(fact.Args[0]) == "/dependency" {
			deps = append(deps, types.ExtractString(fact.Args[1]))
		}
	}
	slices.Sort(deps)
	if !slices.Equal(deps, []string{"Rod", "rod"}) {
		t.Fatalf("deps = %v", deps)
	}
}

func TestKnowledgeChunkRunes_AbsentOrNonPositiveStoresParagraphs(t *testing.T) {
	if knowledgeChunkRunes(nil) != 0 {
		t.Fatal("absent limit")
	}
	for _, n := range []any{int64(0), int64(-3), float64(0)} {
		rows := []types.Fact{{Predicate: "config_param", Args: []any{"/orient_knowledge_chunk_runes", n}}}
		if knowledgeChunkRunes(rows) != 0 {
			t.Fatalf("%v produced a limit", n)
		}
	}
	rows := []types.Fact{{Predicate: "config_param", Args: []any{"/orient_knowledge_chunk_runes", int64(40)}}}
	if knowledgeChunkRunes(rows) != 40 {
		t.Fatal("positive limit")
	}
	body := "abcdef"
	if got := strings.Join(ecosystemBodyParts(body, 0), "\n\n"); got != body {
		t.Fatalf("paragraph round trip %q", got)
	}
	pages := ecosystemBodyParts("ab\n\nHi.", 4)
	if strings.Join(pages, "") != "ab\n\nHi." {
		t.Fatalf("pages = %#v", pages)
	}
	parts := ecosystemBodyParts("para\n\nHi.\n", 0)
	if strings.Join(parts, "\n\n") != "para\n\nHi.\n" {
		t.Fatalf("parts = %#v", parts)
	}
}

func TestCurateAgents_DerivedReasonStaysRecommended(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	offered := []RecommendedAgent{
		{Name: "SecurityAuditor", Reason: "Security analysis is critical for all projects"},
		{Name: "TestArchitect", Reason: "Test quality directly impacts code reliability"},
		{Name: "GoExpert", Reason: "Go project detected - expert knowledge improves code quality"},
		{Name: "RedisExpert", Priority: 40},
		{Name: "reviewer", Reason: "imported subagent"},
	}
	accept := &Initializer{config: InitConfig{
		Workspace:     workspace,
		Interactive:   true,
		InteractiveIO: scriptedInteractiveConfig(t, "y\n"),
	}}
	kept := agentNames(accept.curateAgents(context.Background(), offered, ProjectProfile{Language: "go"}, &InitResult{}))
	slices.Sort(kept)
	if !slices.Contains(kept, "reviewer") || slices.Contains(kept, "RedisExpert") {
		t.Fatalf("accepted = %v", kept)
	}

	declineDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(declineDir, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	decline := &Initializer{config: InitConfig{
		Workspace:     declineDir,
		Interactive:   true,
		InteractiveIO: scriptedInteractiveConfig(t, "n\n"),
	}}
	left := agentNames(decline.curateAgents(context.Background(), offered, ProjectProfile{Language: "go"}, &InitResult{}))
	slices.Sort(left)
	if slices.Contains(left, "reviewer") || slices.Contains(left, "GoExpert") {
		t.Fatalf("declined = %v", left)
	}
	if !slices.Contains(left, "SecurityAuditor") || !slices.Contains(left, "TestArchitect") {
		t.Fatalf("declined = %v", left)
	}
}

func TestIntegrateEcosystem_MaterializesAgentsKnowledgeAndReport(t *testing.T) {
	ws := t.TempDir()
	skillBody := "This skill is the longer copy of lint guidance.\n\nHi.\n"
	loserBody := "LOSER_BODY_DO_NOT_STORE\n"
	reviewerBody := "REVIEWER_BODY_DISTINCTIVE\n\nsecond paragraph\n"
	writeInitFile(t, ws, ".claude/agents/reviewer.md", "---\nname: reviewer\ndescription: Reviews changes\n---\n"+reviewerBody)
	writeInitFile(t, ws, ".claude/skills/lint/SKILL.md", "---\nname: lint\ntags: [reviewer]\n---\n"+skillBody)
	writeInitFile(t, ws, ".codex/skills/lint/SKILL.md", "---\nname: lint\ntags: [reviewer]\n---\n"+loserBody)
	writeInitFile(t, ws, ".agents/rules/packaging.md", "---\nname: packaging-notes\ndescription: how releases are cut\n---\nRULE_BODY_PACKAGING\n")
	writeInitFile(t, ws, ".jules/day.md", "JOURNAL_NOT_ATTACHED\n")

	rec := &topicRecorder{}
	db, err := store.NewLocalStore(filepath.Join(ws, ".nerd", "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ini := &Initializer{
		config:      InitConfig{Workspace: ws, SkipResearch: false},
		embedEngine: fixedInitEngine{},
		fetchTopic:  rec.fetch,
		localDB:     db,
	}
	profile := ProjectProfile{Language: "haskell"}
	if err := ini.runOrientation(context.Background(), filepath.Join(ws, ".nerd"), &InitResult{}); err != nil {
		t.Fatal(err)
	}
	defer ini.orientation.Close()
	recommended := ini.determineRequiredAgents(profile)
	result := &InitResult{}
	recommended = ini.integrateEcosystem(context.Background(), result, profile, recommended)

	names := map[string]bool{}
	var reviewer RecommendedAgent
	for _, agent := range recommended {
		names[agent.Name] = true
		if agent.Name == "reviewer" {
			reviewer = agent
		}
	}
	if !names["reviewer"] || !names["SecurityAuditor"] || !names["TestArchitect"] {
		t.Fatalf("agents = %v warnings = %v", agentNames(recommended), result.Warnings)
	}
	if reviewer.Reason != "imported subagent" {
		t.Fatalf("reviewer why = %q", reviewer.Reason)
	}

	report, err := os.ReadFile(filepath.Join(ws, ".nerd", "orientation", "agents.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(report)
	for _, needle := range []string{"imported subagent", "/larger_body", ".jules/day.md", "Sources not attached", ".codex/skills/lint/SKILL.md", "packaging_notes"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("agents.md missing %q\n%s", needle, text)
		}
	}

	atom, err := db.GetPromptAtom("project/ecosystem/agents/rule/packaging_notes")
	if err != nil || atom == nil {
		t.Fatalf("prompt atom: %v", err)
	}
	if !strings.Contains(atom.Content, "RULE_BODY_PACKAGING") || !strings.Contains(atom.Content, ".agents/rules/packaging.md") {
		t.Fatalf("atom content = %q", atom.Content)
	}

	// createType3Agents reports KB sizes, not an error. A failed agent is a warning.
	if _, sizes := ini.createType3Agents(context.Background(), filepath.Join(ws, ".nerd"), recommended, result); sizes["reviewer"] == 0 {
		t.Fatalf("reviewer kb missing: %v warnings %v", sizes, result.Warnings)
	}
	kb, err := store.NewLocalStore(filepath.Join(ws, ".nerd", "shards", "reviewer_knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kb.Close()
	atoms, err := kb.GetAllKnowledgeAtoms()
	if err != nil {
		t.Fatal(err)
	}
	if got := reconstructParagraphs(atoms, ".claude/agents/reviewer.md"); got != reviewerBody {
		t.Fatalf("reviewer body = %q", got)
	}
	if got := reconstructParagraphs(atoms, ".claude/skills/lint/SKILL.md"); got != skillBody {
		t.Fatalf("skill body = %q", got)
	}
	for _, atom := range atoms {
		if strings.Contains(atom.Content, "LOSER_BODY_DO_NOT_STORE") || strings.Contains(atom.Content, "JOURNAL_NOT_ATTACHED") {
			t.Fatalf("stored %s: %q", atom.Concept, atom.Content)
		}
	}
	prompts, err := os.ReadFile(filepath.Join(ws, ".nerd", "agents", "reviewer", "prompts.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePromptsYAML(prompts, "reviewer"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(prompts), "REVIEWER_BODY_DISTINCTIVE") {
		t.Fatalf("prompts.yaml lost the imported body:\n%s", prompts)
	}
	fetched := rec.take()
	if !slices.Contains(fetched, "OWASP top 10") {
		t.Fatalf("security topics not fetched: %v", fetched)
	}
	for _, topic := range fetched {
		if topic == "reviewer" || topic == "Reviews changes" {
			t.Fatalf("covered reviewer topic was fetched: %v", fetched)
		}
	}
}

func reconstructParagraphs(atoms []store.KnowledgeAtom, path string) string {
	type piece struct {
		i       int
		content string
	}
	var pieces []piece
	for _, atom := range atoms {
		var i, n, nbytes int
		var tool, gotPath string
		if _, err := fmt.Sscanf(atom.Concept, "ecosystem paragraph %d/%d tool=%s path=%s bytes=%d", &i, &n, &tool, &gotPath, &nbytes); err != nil {
			continue
		}
		if gotPath != path {
			continue
		}
		pieces = append(pieces, piece{i, atom.Content})
	}
	slices.SortFunc(pieces, func(a, b piece) int { return a.i - b.i })
	var b strings.Builder
	for idx, piece := range pieces {
		if idx > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(piece.content)
	}
	return b.String()
}

func writeInitFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
