package init

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A stack agent carries the catalog key that created it: the technology the
// session kernel's expert_needed matches a task's files against. A Go project
// with a Bubble Tea dependency yields GoExpert for "go" and BubbleTeaExpert
// for "bubbletea"; before orient_agent_domain the agent knew its topics
// (free text) and nothing a rule could join on.
func TestStackAgentsCarryTheirCatalogDomain(t *testing.T) {
	agents, err := agentsFromProfile(context.Background(), ProjectProfile{
		Language:     "go",
		Dependencies: []DependencyInfo{{Name: "bubbletea", Type: "direct"}},
	})
	if err != nil {
		t.Fatalf("agentsFromProfile: %v", err)
	}
	domains := map[string][]string{}
	for _, agent := range agents {
		domains[agent.Name] = agent.Domains
	}
	if !slices.Equal(domains["GoExpert"], []string{"go"}) {
		t.Errorf("GoExpert domains = %v, want [go]", domains["GoExpert"])
	}
	if !slices.Equal(domains["BubbleTeaExpert"], []string{"bubbletea"}) {
		t.Errorf("BubbleTeaExpert domains = %v, want [bubbletea]", domains["BubbleTeaExpert"])
	}
}

// A recommendation that already names a catalog agent gains the catalog's
// domains instead of dropping them.
func TestMergeDerivedAgentsKeepsTheCatalogDomain(t *testing.T) {
	merged := mergeDerivedAgents(
		[]RecommendedAgent{{Name: "GoExpert", Description: "kept"}},
		[]RecommendedAgent{{Name: "GoExpert", Domains: []string{"go"}}},
		"go",
	)
	if len(merged) != 1 || merged[0].Description != "kept" || !slices.Equal(merged[0].Domains, []string{"go"}) {
		t.Errorf("merged = %+v, want the existing GoExpert with domains [go]", merged)
	}
}

// The registry init writes carries the domains, in the field boot reads.
func TestAgentRegistryWritesDomains(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.json")
	i := &Initializer{}
	if err := i.saveAgentRegistry(path, []CreatedAgent{{Name: "GoExpert", Type: "persistent", Status: "ready", Domains: []string{"go"}}}); err != nil {
		t.Fatalf("saveAgentRegistry: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var reg struct {
		Agents []struct {
			Domains []string `json:"domains"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(reg.Agents) != 1 || !slices.Equal(reg.Agents[0].Domains, []string{"go"}) {
		t.Errorf("agents.json = %s, want domains [go]", data)
	}
}
