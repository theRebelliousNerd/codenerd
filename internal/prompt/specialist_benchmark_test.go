package prompt

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func BenchmarkInjectAvailableSpecialists(b *testing.B) {
	// Setup a temporary workspace with agents.json
	tmpDir := b.TempDir()
	nerdDir := filepath.Join(tmpDir, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		b.Fatal(err)
	}

	agentsJSON := `{
		"agents": [
			{"name": "researcher", "type": "research", "status": "ready", "description": "Researches stuff"},
			{"name": "coder", "type": "coding", "status": "ready", "description": "Writes code"}
		]
	}`
	if err := os.WriteFile(filepath.Join(nerdDir, "agents.json"), []byte(agentsJSON), 0644); err != nil {
		b.Fatal(err)
	}

	cc := NewCompilationContext()

	b.ResetTimer()
	for b.Loop() {
		// We pass the tmpDir as workspace
		_ = InjectAvailableSpecialists(cc, tmpDir)
	}
}

func TestInjectAvailableSpecialists_CacheInvalidation(t *testing.T) {
	// Setup a temporary workspace
	tmpDir := t.TempDir()
	nerdDir := filepath.Join(tmpDir, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		t.Fatal(err)
	}
	agentsPath := filepath.Join(nerdDir, "agents.json")

	// 1. Write initial file
	agentsJSON1 := `{
		"agents": [
			{"name": "agent1", "type": "type1", "status": "ready", "description": "Desc1"}
		]
	}`
	if err := os.WriteFile(agentsPath, []byte(agentsJSON1), 0644); err != nil {
		t.Fatal(err)
	}

	cc := NewCompilationContext()

	// 2. First call - should load agent1
	if err := InjectAvailableSpecialists(cc, tmpDir); err != nil {
		t.Fatal(err)
	}
	// Verify content
	if len(cc.AvailableSpecialists) == 0 {
		t.Error("AvailableSpecialists is empty")
	}
	if !strings.Contains(cc.AvailableSpecialists, "agent1") {
		t.Fatalf("initial specialists omitted agent1: %q", cc.AvailableSpecialists)
	}

	// 3. Update file
	agentsJSON2 := `{
		"agents": [
			{"name": "agent2", "type": "type2", "status": "ready", "description": "Desc2"}
		]
	}`
	// Ensure mtime changes (filesystems have resolution limits)
	// We might need to wait or touch the time explicitly if the test runs too fast.
	// But os.WriteFile usually updates mtime.
	if err := os.WriteFile(agentsPath, []byte(agentsJSON2), 0644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(agentsPath, future, future); err != nil {
		t.Fatal(err)
	}

	// Expire the cheap stat-avoidance TTL so the next call observes the new
	// file timestamp without adding a multi-second sleep to the test suite.
	specialistCacheMu.Lock()
	entry := specialistCache[agentsPath]
	entry.lastChecked = time.Now().Add(-specialistCacheTTL)
	specialistCache[agentsPath] = entry
	specialistCacheMu.Unlock()

	// 4. Second call - should load agent2 (cache invalidation)
	if err := InjectAvailableSpecialists(cc, tmpDir); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(cc.AvailableSpecialists, "agent2") || strings.Contains(cc.AvailableSpecialists, "agent1") {
		t.Fatalf("specialist cache remained stale after registry update: %q", cc.AvailableSpecialists)
	}
}

func TestFormatSpecialists_IsDeterministic(t *testing.T) {
	registry := agentRegistry{}
	registry.Agents = append(registry.Agents,
		struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Status      string `json:"status"`
			Description string `json:"description"`
			Topics      string `json:"topics"`
		}{Name: "zeta", Type: "test", Status: "ready", Description: "Z"},
		struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Status      string `json:"status"`
			Description string `json:"description"`
			Topics      string `json:"topics"`
		}{Name: "alpha", Type: "test", Status: "ready", Description: "A"},
	)

	lines := strings.Split(formatSpecialists(registry), "\n")
	if !sort.StringsAreSorted(lines) {
		t.Fatalf("specialist output is not sorted: %v", lines)
	}
}
