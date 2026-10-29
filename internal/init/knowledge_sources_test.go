package init

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"codenerd/internal/knowledge"
	"codenerd/internal/store"
)

// A technology domain the workspace pins becomes one subject per pinned
// package (what go doc and the repository source read); a domain it does not
// pin and every research topic are plain subjects.
func TestKnowledgeSubjects_PinnedModulesAndTopics(t *testing.T) {
	root := t.TempDir()
	writeInitFile(t, root, "go.mod", "module example.com/app\n\ngo 1.24\n\nrequire github.com/spf13/cobra v1.10.2\n")
	subjects := KnowledgeSubjects(root, "cobraexpert", []string{"cobra", "sql", "react"}, []string{"cobra command trees", "Cobra Command Trees"})

	var keys []string
	for _, s := range subjects {
		keys = append(keys, s.Key())
		if s.Agent != "cobraexpert" {
			t.Fatalf("subject %+v lacks its agent", s)
		}
	}
	for _, want := range []string{"cobra command trees", "github.com/spf13/cobra", "database/sql", "react"} {
		if !slices.Contains(keys, want) {
			t.Fatalf("subjects %v lack %q", keys, want)
		}
	}
	if len(keys) != 4 {
		t.Fatalf("subjects = %v, want the duplicate topic folded", keys)
	}
	if got := KnowledgeSubjects("", "x", []string{"cobra"}, nil); len(got) != 1 || got[0].Module != "" {
		t.Fatalf("without a workspace a domain pins nothing: %+v", got)
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
