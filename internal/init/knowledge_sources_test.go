package init

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/knowledge"
	"codenerd/internal/store"
)

// A technology domain the workspace pins becomes one subject per pinned
// package, carrying its ecosystem and the version its manifest pins (what
// go doc reads and what the refresh policy compares); a domain it pins
// nothing for and every research topic are plain subjects.
func TestKnowledgeSubjects_PinnedModulesAndTopics(t *testing.T) {
	root := t.TempDir()
	writeInitFile(t, root, "go.mod", "module example.com/app\n\ngo 1.24\n\nrequire github.com/spf13/cobra v1.10.2\n")
	writeInitFile(t, root, "web/package.json", `{"dependencies": {"react": "^19.1.0"}}`)
	pins, err := ManifestPins(root)
	if err != nil {
		t.Fatalf("ManifestPins: %v", err)
	}
	if len(pins) != 2 || pins[0].Module != "github.com/spf13/cobra" || pins[0].Version != "v1.10.2" ||
		pins[1].Module != "react" || pins[1].Version != "^19.1.0" || pins[1].Manifest != "web/package.json" {
		t.Fatalf("pins = %+v", pins)
	}
	subjects := KnowledgeSubjects(root, "cobraexpert", []string{"cobra", "sql", "react", "tokio"}, []string{"cobra command trees", "Cobra Command Trees"}, pins)

	byKey := map[string]knowledge.Subject{}
	for _, s := range subjects {
		byKey[s.Key()] = s
		if s.Agent != "cobraexpert" {
			t.Fatalf("subject %+v lacks its agent", s)
		}
	}
	if len(byKey) != 5 || len(subjects) != 5 {
		t.Fatalf("subjects = %+v, want five with the duplicate topic folded", subjects)
	}
	if s := byKey["github.com/spf13/cobra"]; s.Ecosystem != "go" || s.Version != "v1.10.2" || s.Technology != "cobra" {
		t.Fatalf("cobra subject = %+v", s)
	}
	if s := byKey["database/sql"]; s.Ecosystem != "go" || s.Version != "" {
		t.Fatalf("standard-library subject = %+v", s)
	}
	if s := byKey["react"]; s.Ecosystem != "npm" || s.Version != "^19.1.0" {
		t.Fatalf("react subject = %+v", s)
	}
	if s := byKey["tokio"]; s.Module != "" || s.Topic != "tokio" {
		t.Fatalf("an unpinned domain = %+v, want a plain subject", s)
	}
	if _, ok := byKey["cobra command trees"]; !ok {
		t.Fatalf("the topic is missing: %+v", subjects)
	}
	if got := KnowledgeSubjects("", "x", []string{"cobra"}, nil, nil); len(got) != 1 || got[0].Module != "" {
		t.Fatalf("without a workspace a domain pins nothing: %+v", got)
	}
	writeInitFile(t, root, "broken/go.mod", "module example.com/broken\n\nrequire (\n")
	if _, err := ManifestPins(root); err == nil || !strings.Contains(err.Error(), "broken/go.mod") {
		t.Fatalf("a manifest that does not parse is not named: %v", err)
	}
}

// A full init acquires every agent's knowledge through the typed sources:
// each KB records every source's outcome, and no KB holds a placeholder.
func TestInitialize_AcquiresKnowledgeWithTypedOutcomes(t *testing.T) {
	root := codingAgentFixture(t)
	main, planner := newRoutedClients(true)
	runRoutedInit(t, root, main, planner)

	paths, err := filepath.Glob(filepath.Join(root, ".nerd", "shards", "*_knowledge.db"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no agent KBs: %v", err)
	}
	sources := map[string]bool{}
	for _, path := range paths {
		kb, err := store.NewLocalStore(path)
		if err != nil {
			t.Fatal(err)
		}
		outcomes, err := knowledge.Outcomes(context.Background(), kb.GetDB())
		atoms, aerr := kb.GetAllKnowledgeAtoms()
		_ = kb.Close()
		if err != nil || aerr != nil {
			t.Fatalf("%s: outcomes err %v, atoms err %v", path, err, aerr)
		}
		for _, o := range outcomes {
			sources[o.Source] = true
		}
		for _, a := range atoms {
			if knowledge.IsLegacyPlaceholder(a.Concept, a.Content) {
				t.Fatalf("%s holds a placeholder: %s", filepath.Base(path), a.Concept)
			}
		}
	}
	for _, want := range []string{"web", "context7", "godoc", "repo/world"} {
		if !sources[want] {
			t.Fatalf("no KB records an outcome from %s; recorded %v", want, sources)
		}
	}

	// The registry keeps each agent's topics: the subjects the background
	// refresh keeps current.
	data, err := os.ReadFile(filepath.Join(root, ".nerd", "agents.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reg AgentRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	withTopics := 0
	for _, agent := range reg.Agents {
		if len(agent.Topics) > 0 {
			withTopics++
		}
	}
	if withTopics == 0 {
		t.Fatalf("no agent in agents.json carries its topics: %s", data)
	}
}
