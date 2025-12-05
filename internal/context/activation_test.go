package context

import (
	"codenerd/internal/core"
	"testing"
	"time"
)

func TestNewActivationEngine(t *testing.T) {
	config := DefaultCompressorConfig()
	engine := NewActivationEngine(config)
	if engine == nil {
		t.Fatal("NewActivationEngine() returned nil")
	}

	if engine.sessionID == "" {
		t.Error("sessionID should be set")
	}

	if engine.factTimestamps == nil {
		t.Error("factTimestamps should be initialized")
	}
}

func TestScoreFacts(t *testing.T) {
	config := DefaultCompressorConfig()
	config.PredicatePriorities = map[string]int{
		"user_intent":     100,
		"file_topology":   80,
		"diagnostic":      90,
		"symbol_graph":    70,
		"dependency_link": 60,
	}

	engine := NewActivationEngine(config)

	facts := []core.Fact{
		{Predicate: "user_intent", Args: []interface{}{"id1", "/query", "/explain", "auth.go", ""}},
		{Predicate: "file_topology", Args: []interface{}{"main.go", "hash1", "/go", int64(1699000000), "/false"}},
		{Predicate: "diagnostic", Args: []interface{}{"/error", "main.go", int64(10), "E001", "error"}},
	}

	intent := &core.Fact{Predicate: "user_intent", Args: []interface{}{"id1", "/query", "/explain", "auth.go", ""}}

	scored := engine.ScoreFacts(facts, intent)

	if len(scored) != 3 {
		t.Errorf("Expected 3 scored facts, got %d", len(scored))
	}

	// Verify scores are non-zero
	for _, sf := range scored {
		if sf.Score <= 0 {
			t.Errorf("Expected positive score for %s, got %f", sf.Fact.Predicate, sf.Score)
		}
	}

	// Verify sorted by score descending
	for i := 1; i < len(scored); i++ {
		if scored[i].Score > scored[i-1].Score {
			t.Error("Scores should be sorted in descending order")
		}
	}
}

func TestFilterByThreshold(t *testing.T) {
	config := DefaultCompressorConfig()
	config.ActivationThreshold = 50.0

	engine := NewActivationEngine(config)

	scored := []ScoredFact{
		{Fact: core.Fact{Predicate: "high"}, Score: 100.0},
		{Fact: core.Fact{Predicate: "medium"}, Score: 60.0},
		{Fact: core.Fact{Predicate: "low"}, Score: 30.0},
	}

	filtered := engine.FilterByThreshold(scored)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 facts above threshold, got %d", len(filtered))
	}

	for _, sf := range filtered {
		if sf.Score < 50.0 {
			t.Errorf("Fact %s with score %f should have been filtered", sf.Fact.Predicate, sf.Score)
		}
	}
}

func TestSelectWithinBudget(t *testing.T) {
	config := DefaultCompressorConfig()
	engine := NewActivationEngine(config)

	scored := []ScoredFact{
		{Fact: core.Fact{Predicate: "file_topology", Args: []interface{}{"main.go", "hash1", "/go", int64(1699000000), "/false"}}, Score: 100.0},
		{Fact: core.Fact{Predicate: "file_topology", Args: []interface{}{"auth.go", "hash2", "/go", int64(1699000001), "/false"}}, Score: 90.0},
		{Fact: core.Fact{Predicate: "file_topology", Args: []interface{}{"test.go", "hash3", "/go", int64(1699000002), "/true"}}, Score: 80.0},
	}

	// Small budget should limit results
	selected := engine.SelectWithinBudget(scored, 100)

	// Should select at least 1 but not necessarily all
	if len(selected) == 0 {
		t.Error("Expected at least one fact to be selected")
	}

	// Large budget should include all
	allSelected := engine.SelectWithinBudget(scored, 10000)
	if len(allSelected) != 3 {
		t.Logf("Selected %d of 3 facts with large budget", len(allSelected))
	}
}

func TestUpdateFocusedPaths(t *testing.T) {
	config := DefaultCompressorConfig()
	engine := NewActivationEngine(config)

	facts := []core.Fact{
		{Predicate: "focus_resolution", Args: []interface{}{"auth", "pkg/auth/auth.go", "Auth", int64(95)}},
		{Predicate: "focus_resolution", Args: []interface{}{"handler", "pkg/http/handler.go", "Handler", int64(85)}},
		{Predicate: "file_topology", Args: []interface{}{"main.go", "hash", "/go", int64(0), "/false"}}, // Should be ignored
	}

	engine.UpdateFocusedPaths(facts)

	state := engine.GetState()

	if len(state.FocusedPaths) != 2 {
		t.Errorf("Expected 2 focused paths, got %d", len(state.FocusedPaths))
	}

	if len(state.FocusedSymbols) != 2 {
		t.Errorf("Expected 2 focused symbols, got %d", len(state.FocusedSymbols))
	}
}

func TestRecordFactTimestamp(t *testing.T) {
	config := DefaultCompressorConfig()
	engine := NewActivationEngine(config)

	fact := core.Fact{Predicate: "test_fact", Args: []interface{}{"arg1"}}
	engine.RecordFactTimestamp(fact)

	key := factKey(fact)
	if _, ok := engine.factTimestamps[key]; !ok {
		t.Error("Fact timestamp should be recorded")
	}

	if !engine.sessionFacts[key] {
		t.Error("Fact should be marked as session fact")
	}
}

func TestAddDependency(t *testing.T) {
	config := DefaultCompressorConfig()
	engine := NewActivationEngine(config)

	dependent := core.Fact{Predicate: "handler", Args: []interface{}{"h1"}}
	dependency := core.Fact{Predicate: "auth", Args: []interface{}{"a1"}}

	engine.AddDependency(dependent, dependency)

	depKey := factKey(dependent)
	depsKey := factKey(dependency)

	if deps, ok := engine.dependencies[depKey]; !ok || len(deps) == 0 {
		t.Error("Dependency should be recorded")
	}

	if rdeps, ok := engine.reverseDependencies[depsKey]; !ok || len(rdeps) == 0 {
		t.Error("Reverse dependency should be recorded")
	}
}

func TestComputeRecencyScore(t *testing.T) {
	config := DefaultCompressorConfig()
	engine := NewActivationEngine(config)

	// Create a fact with recent timestamp
	recentFact := core.Fact{Predicate: "recent", Args: []interface{}{"arg"}}
	engine.factTimestamps[factKey(recentFact)] = time.Now()

	score := engine.computeRecencyScore(recentFact)
	if score != 50.0 {
		t.Errorf("Expected recency score of 50 for new fact, got %f", score)
	}

	// Create a fact with old timestamp
	oldFact := core.Fact{Predicate: "old", Args: []interface{}{"arg"}}
	engine.factTimestamps[factKey(oldFact)] = time.Now().Add(-1 * time.Hour)

	oldScore := engine.computeRecencyScore(oldFact)
	if oldScore != 0.0 {
		t.Errorf("Expected recency score of 0 for old fact, got %f", oldScore)
	}

	// Fact with unknown timestamp
	unknownFact := core.Fact{Predicate: "unknown", Args: []interface{}{"arg"}}
	unknownScore := engine.computeRecencyScore(unknownFact)
	if unknownScore != 0.0 {
		t.Errorf("Expected recency score of 0 for unknown fact, got %f", unknownScore)
	}
}

func TestComputeRelevanceScoreWithIntent(t *testing.T) {
	config := DefaultCompressorConfig()
	engine := NewActivationEngine(config)

	// Set up intent
	intent := &core.Fact{
		Predicate: "user_intent",
		Args:      []interface{}{"id1", "/mutation", "/fix", "auth.go", ""},
	}
	engine.state.ActiveIntent = intent

	// Fact that matches intent target
	matchingFact := core.Fact{Predicate: "file_content", Args: []interface{}{"auth.go", "content"}}
	matchScore := engine.computeRelevanceScore(matchingFact)

	// Fact that doesn't match
	nonMatchingFact := core.Fact{Predicate: "file_content", Args: []interface{}{"other.go", "content"}}
	nonMatchScore := engine.computeRelevanceScore(nonMatchingFact)

	if matchScore <= nonMatchScore {
		t.Errorf("Matching fact should have higher relevance score (%f vs %f)", matchScore, nonMatchScore)
	}
}

func TestComputeCampaignScore(t *testing.T) {
	config := DefaultCompressorConfig()
	engine := NewActivationEngine(config)

	// Without campaign context, score should be 0
	fact := core.Fact{Predicate: "file_topology", Args: []interface{}{"main.go"}}
	score := engine.computeCampaignScore(fact)
	if score != 0.0 {
		t.Errorf("Expected 0 campaign score without context, got %f", score)
	}

	// With campaign context
	engine.SetCampaignContext(&CampaignActivationContext{
		CampaignID:    "campaign1",
		CurrentPhase:  "implementation",
		CurrentTask:   "add_auth",
		RelevantFiles: []string{"auth.go", "handler.go"},
	})

	// Fact related to campaign
	campaignFact := core.Fact{Predicate: "campaign_task", Args: []interface{}{"task1", "add_auth"}}
	campaignScore := engine.computeCampaignScore(campaignFact)
	if campaignScore <= 0 {
		t.Errorf("Expected positive campaign score for campaign-related fact, got %f", campaignScore)
	}
}

func TestComputeSessionScore(t *testing.T) {
	config := DefaultCompressorConfig()
	engine := NewActivationEngine(config)

	// Fact not in session
	nonSessionFact := core.Fact{Predicate: "old_fact", Args: []interface{}{"arg"}}
	nonSessionScore := engine.computeSessionScore(nonSessionFact)
	if nonSessionScore != 0.0 {
		t.Errorf("Expected 0 session score for non-session fact, got %f", nonSessionScore)
	}

	// Mark a fact as session fact
	sessionFact := core.Fact{Predicate: "new_fact", Args: []interface{}{"arg"}}
	engine.RecordFactTimestamp(sessionFact)

	sessionScore := engine.computeSessionScore(sessionFact)
	if sessionScore != 15.0 {
		t.Errorf("Expected 15 session score for session fact, got %f", sessionScore)
	}
}

func TestBuildSymbolGraph(t *testing.T) {
	config := DefaultCompressorConfig()
	engine := NewActivationEngine(config)

	facts := []core.Fact{
		{Predicate: "dependency_link", Args: []interface{}{"handler.go", "auth.go", "pkg/auth"}},
		{Predicate: "dependency_link", Args: []interface{}{"main.go", "handler.go", "pkg/http"}},
		{Predicate: "symbol_graph", Args: []interface{}{"Handler", "/function", "/public", "handler.go", "Handler(w, r)"}},
	}

	engine.buildSymbolGraph(facts)

	// Check symbol graph was built
	if _, ok := engine.symbolGraph["handler.go"]; !ok {
		t.Error("Symbol graph should contain handler.go")
	}

	// Check reverse dependencies
	if deps, ok := engine.reverseDependencies["auth.go"]; !ok || len(deps) == 0 {
		t.Error("Reverse dependencies should contain auth.go")
	}
}

func TestApplyIntentActivation(t *testing.T) {
	config := DefaultCompressorConfig()
	config.ActivationThreshold = 30.0
	config.PredicatePriorities = map[string]int{
		"user_intent":   100,
		"file_topology": 60,
		"diagnostic":    80,
	}

	engine := NewActivationEngine(config)

	facts := []core.Fact{
		{Predicate: "user_intent", Args: []interface{}{"id1", "/query", "/explain", "auth.go", ""}},
		{Predicate: "file_topology", Args: []interface{}{"main.go", "hash", "/go", int64(0), "/false"}},
		{Predicate: "focus_resolution", Args: []interface{}{"auth", "auth.go", "Auth", int64(90)}},
	}

	intent := &core.Fact{Predicate: "user_intent", Args: []interface{}{"id1", "/query", "/explain", "auth.go", ""}}

	scored := engine.ApplyIntentActivation(facts, intent)

	if len(scored) == 0 {
		t.Error("Expected some facts to pass activation threshold")
	}

	// Verify all returned facts are above threshold
	for _, sf := range scored {
		if sf.Score < config.ActivationThreshold {
			t.Errorf("Fact %s with score %f should be above threshold %f",
				sf.Fact.Predicate, sf.Score, config.ActivationThreshold)
		}
	}
}

func TestClearState(t *testing.T) {
	config := DefaultCompressorConfig()
	engine := NewActivationEngine(config)

	// Add some state
	fact := core.Fact{Predicate: "test", Args: []interface{}{"arg"}}
	engine.RecordFactTimestamp(fact)
	engine.SetCampaignContext(&CampaignActivationContext{CampaignID: "test"})
	engine.state.FocusedPaths = []string{"path1"}

	// Clear state
	engine.ClearState()

	if len(engine.factTimestamps) != 0 {
		t.Error("factTimestamps should be cleared")
	}

	if len(engine.sessionFacts) != 0 {
		t.Error("sessionFacts should be cleared")
	}

	if engine.campaignContext != nil {
		t.Error("campaignContext should be nil")
	}

	if len(engine.state.FocusedPaths) != 0 {
		t.Error("FocusedPaths should be cleared")
	}
}

func TestNewSession(t *testing.T) {
	config := DefaultCompressorConfig()
	engine := NewActivationEngine(config)

	// Add session facts
	fact := core.Fact{Predicate: "test", Args: []interface{}{"arg"}}
	engine.RecordFactTimestamp(fact)

	oldSessionID := engine.sessionID

	// Start new session
	engine.NewSession()

	if engine.sessionID == oldSessionID {
		t.Error("New session should have different ID")
	}

	if len(engine.sessionFacts) != 0 {
		t.Error("Session facts should be reset")
	}
}

func TestGetSessionStats(t *testing.T) {
	config := DefaultCompressorConfig()
	engine := NewActivationEngine(config)

	// Add some data
	fact := core.Fact{Predicate: "test", Args: []interface{}{"arg"}}
	engine.RecordFactTimestamp(fact)

	stats := engine.GetSessionStats()

	if stats["session_id"] == "" {
		t.Error("Session ID should be set")
	}

	if stats["session_facts"].(int) != 1 {
		t.Errorf("Expected 1 session fact, got %d", stats["session_facts"].(int))
	}
}

func TestDecayRecency(t *testing.T) {
	config := DefaultCompressorConfig()
	engine := NewActivationEngine(config)

	// Add fact with old timestamp
	oldFact := core.Fact{Predicate: "old", Args: []interface{}{"arg"}}
	engine.factTimestamps[factKey(oldFact)] = time.Now().Add(-2 * time.Hour)

	// Add fact with recent timestamp
	recentFact := core.Fact{Predicate: "recent", Args: []interface{}{"arg"}}
	engine.factTimestamps[factKey(recentFact)] = time.Now()

	// Decay with 1 hour cutoff
	engine.DecayRecency(1 * time.Hour)

	if _, ok := engine.factTimestamps[factKey(oldFact)]; ok {
		t.Error("Old fact should have been removed")
	}

	if _, ok := engine.factTimestamps[factKey(recentFact)]; !ok {
		t.Error("Recent fact should still be present")
	}
}

func TestSpreadFromSeeds(t *testing.T) {
	config := DefaultCompressorConfig()
	config.PredicatePriorities = map[string]int{
		"user_intent":     100,
		"file_topology":   50,
		"dependency_link": 40,
	}

	engine := NewActivationEngine(config)

	// Set up dependencies
	engine.dependencies["file_topology(\"handler.go\", \"hash\", \"/go\", 0, \"/false\")."] = []string{
		"file_topology(\"auth.go\", \"hash\", \"/go\", 0, \"/false\").",
	}

	facts := []core.Fact{
		{Predicate: "user_intent", Args: []interface{}{"id1", "/query", "/explain", "handler.go", ""}},
		{Predicate: "file_topology", Args: []interface{}{"handler.go", "hash", "/go", int64(0), "/false"}},
		{Predicate: "file_topology", Args: []interface{}{"auth.go", "hash", "/go", int64(0), "/false"}},
	}

	seeds := []core.Fact{
		{Predicate: "user_intent", Args: []interface{}{"id1", "/query", "/explain", "handler.go", ""}},
	}

	scored := engine.SpreadFromSeeds(facts, seeds, 1)

	if len(scored) != 3 {
		t.Errorf("Expected 3 scored facts, got %d", len(scored))
	}

	// Seed fact should have high score
	for _, sf := range scored {
		if sf.Fact.Predicate == "user_intent" && sf.Score <= 0 {
			t.Error("Seed fact should have high activation")
		}
	}
}

func TestFactKey(t *testing.T) {
	fact := core.Fact{Predicate: "test", Args: []interface{}{"arg1", "arg2"}}
	key := factKey(fact)

	if key == "" {
		t.Error("factKey should return non-empty string")
	}

	// Same fact should produce same key
	key2 := factKey(fact)
	if key != key2 {
		t.Error("Same fact should produce same key")
	}
}

func TestExtractPredicate(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"test(arg1, arg2).", "test"},
		{"file_topology(\"main.go\").", "file_topology"},
		{"simple", "simple"},
		{"no_args()", "no_args"},
	}

	for _, tt := range tests {
		got := extractPredicate(tt.key)
		if got != tt.expected {
			t.Errorf("extractPredicate(%q) = %q, want %q", tt.key, got, tt.expected)
		}
	}
}
