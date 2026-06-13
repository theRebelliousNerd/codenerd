package core

import (
	"errors"
	"testing"
)

func TestDreamLearningCollector_New(t *testing.T) {
	collector := NewDreamLearningCollector()
	if collector == nil {
		t.Fatal("NewDreamLearningCollector returned nil")
	}
}

func TestDreamLearningCollector_ExtractLearnings(t *testing.T) {
	collector := NewDreamLearningCollector()

	// 1. Test extracting nothing due to error in consultation
	consultationsErr := []DreamConsultation{
		{
			ShardName: "coder",
			Error:     errors.New("shard crashed"),
		},
	}
	learnings := collector.ExtractLearnings("Implement feature", consultationsErr)
	if len(learnings) != 0 {
		t.Errorf("Expected 0 learnings, got %d", len(learnings))
	}

	// 2. Test extracting procedural step, tool need, and risk pattern
	// Use path/file details and project-specific terms to ensure high novelty
	consultations := []DreamConsultation{
		{
			ShardName:   "specialist-knowledge",
			ShardType:   "knowledge_specialist",
			Perspective: "approach:\n1. We need a tool called 'file_scanner'.\n2. implement procedural step in path/helper.go using mangle.\n3. Never do X because it might break deployment.",
			Concerns:    []string{"Stability risk"},
		},
	}

	learnings = collector.ExtractLearnings("Build tools", consultations)
	if len(learnings) == 0 {
		t.Fatal("Expected to extract learnings, got 0")
	}

	// Verify details
	var hasProcedural, hasToolNeed, hasRisk bool
	for _, l := range learnings {
		if l.ID == "" {
			t.Error("Learning missing ID")
		}
		switch l.Type {
		case LearningTypeProcedural:
			hasProcedural = true
		case LearningTypeToolNeed:
			hasToolNeed = true
			if l.Metadata["tool_name"] != "file_scanner" {
				t.Errorf("Expected tool name 'file_scanner', got %q", l.Metadata["tool_name"])
			}
		case LearningTypeRiskPattern:
			hasRisk = true
			if l.Metadata["risk_type"] != "stability" {
				t.Errorf("Expected risk type 'stability', got %q", l.Metadata["risk_type"])
			}
		}
	}

	if !hasProcedural {
		t.Error("Expected to extract procedural learning")
	}
	if !hasToolNeed {
		t.Error("Expected to extract tool need learning")
	}
	if !hasRisk {
		t.Error("Expected to extract risk pattern learning")
	}
}

func TestDreamLearningCollector_IsUsefulFilters(t *testing.T) {
	collector := NewDreamLearningCollector()

	// 1. Low novelty filter
	lLowNovelty := &DreamLearning{
		Type:    LearningTypeProcedural,
		Content: "generic tip",
		Novelty: 0.1,
	}
	if collector.isUseful(lLowNovelty) {
		t.Error("Expected low novelty learning to be filtered out")
	}

	// 2. Already known filter
	lKnown := &DreamLearning{
		Type:    LearningTypeProcedural,
		Content: "unique tip for kernel",
		Novelty: 0.8,
	}
	// Add to known patterns
	patternKey := lKnown.Type.String() + ":" + normalizeForDedup(lKnown.Content)
	collector.knownPatterns[patternKey] = true
	if collector.isUseful(lKnown) {
		t.Error("Expected known pattern to be filtered out")
	}

	// 3. Generic advice filter (contains generic phrase, novelty < 0.6)
	lGenericLowNovelty := &DreamLearning{
		Type:    LearningTypeProcedural,
		Content: "consult documentation for files",
		Novelty: 0.5,
	}
	if collector.isUseful(lGenericLowNovelty) {
		t.Error("Expected generic low novelty to be filtered out")
	}

	// 4. Generic advice but high novelty (should NOT be filtered)
	lGenericHighNovelty := &DreamLearning{
		Type:    LearningTypeProcedural,
		Content: "consult documentation for files",
		Novelty: 0.8,
	}
	if !collector.isUseful(lGenericHighNovelty) {
		t.Error("Expected generic high novelty learning to be useful")
	}
}

func TestDreamLearningCollector_ScoreNovelty(t *testing.T) {
	collector := NewDreamLearningCollector()

	// 1. File path / extension boosts novelty
	n1 := collector.scoreNovelty("simple string")
	n2 := collector.scoreNovelty("path/to/helper.go")
	if n2 <= n1 {
		t.Errorf("Expected path/to/helper.go to have higher novelty than simple string, got %f <= %f", n2, n1)
	}

	// 2. Project terms boost novelty
	n3 := collector.scoreNovelty("mangle kernel transducer")
	if n3 <= n1 {
		t.Errorf("Expected project terms to boost novelty, got %f <= %f", n3, n1)
	}

	// 3. Numbers boost novelty
	n4 := collector.scoreNovelty("step 42 is finished")
	if n4 <= n1 {
		t.Errorf("Expected numbers to boost novelty, got %f <= %f", n4, n1)
	}

	// 4. Generic terms reduce novelty
	n5 := collector.scoreNovelty("typically standard common generally usually")
	if n5 >= n1 {
		t.Errorf("Expected generic terms to reduce novelty, got %f >= %f", n5, n1)
	}

	// Clamping check:
	// Very generic string (reduces score below 0.0)
	nClampMin := collector.scoreNovelty("typically typically typically typically standard common generally usually usually usually")
	if nClampMin != 0.0 {
		t.Errorf("Expected novelty to clamp to 0.0, got %f", nClampMin)
	}

	// Very specific string (boosts score above 1.0)
	nClampMax := collector.scoreNovelty("mangle kernel transducer shard campaign nerd 42 /helper.go /helper.go /helper.go /helper.go")
	if nClampMax != 1.0 {
		t.Errorf("Expected novelty to clamp to 1.0, got %f", nClampMax)
	}
}

func TestDreamLearningCollector_ConfirmLearnings(t *testing.T) {
	collector := NewDreamLearningCollector()

	// Populate staged
	l := &DreamLearning{
		ID:         "L1",
		Type:       LearningTypeProcedural,
		Content:    "Unique procedure",
		Confidence: 0.5,
	}
	collector.staged[l.ID] = l

	// Test soft confirmation
	confirmed := collector.ConfirmLearnings("Looks okay")
	if len(confirmed) != 1 {
		t.Fatalf("Expected 1 confirmed learning, got %d", len(confirmed))
	}
	if confirmed[0].Confidence != 0.8 { // 0.5 + 0.3
		t.Errorf("Expected 0.8 confidence, got %f", confirmed[0].Confidence)
	}

	// Reset and test strong confirmation
	collector.staged[l.ID] = l
	l.Confirmed = false
	l.Confidence = 0.6
	confirmedStrong := collector.ConfirmLearnings("Yes, learn this always!")
	if len(confirmedStrong) != 1 {
		t.Fatalf("Expected 1 confirmed learning, got %d", len(confirmedStrong))
	}
	// 0.6 + 0.5 = 1.1 -> clamped to 1.0
	if confirmedStrong[0].Confidence != 1.0 {
		t.Errorf("Expected 1.0 confidence, got %f", confirmedStrong[0].Confidence)
	}
	if !confirmedStrong[0].Confirmed {
		t.Error("Expected confirmed to be true")
	}
	if confirmedStrong[0].ConfirmedAt == nil {
		t.Error("Expected ConfirmedAt to be set")
	}

	// Staged should be cleared
	if len(collector.staged) != 0 {
		t.Errorf("Expected staged to be cleared, got %d items", len(collector.staged))
	}
}

func TestDreamLearningCollector_LearnCorrection(t *testing.T) {
	collector := NewDreamLearningCollector()
	collector.lastDreamID = "dream_1"

	// Mock a staged item so function can find a Hypothetical
	collector.staged["staged_1"] = &DreamLearning{
		Hypothetical: "How to fix engine",
	}

	correction := "No, we use PostgreSQL not MySQL"
	learning := collector.LearnCorrection(correction, LearningTypePreference)

	if learning == nil {
		t.Fatal("LearnCorrection returned nil")
	}
	if learning.Content != correction {
		t.Errorf("Expected content %q, got %q", correction, learning.Content)
	}
	if learning.Hypothetical != "How to fix engine" {
		t.Errorf("Expected hypothetical 'How to fix engine', got %q", learning.Hypothetical)
	}
	if learning.Confidence != 0.9 {
		t.Errorf("Expected 0.9 confidence, got %f", learning.Confidence)
	}
	if learning.Novelty != 1.0 {
		t.Errorf("Expected 1.0 novelty, got %f", learning.Novelty)
	}
	if !learning.Confirmed {
		t.Error("Expected confirmed to be true")
	}
	if learning.ConfirmedAt == nil {
		t.Error("Expected ConfirmedAt to be set")
	}

	// Verify deduplication key is added
	patternKey := LearningTypePreference.String() + ":" + normalizeForDedup(correction)
	if !collector.knownPatterns[patternKey] {
		t.Error("Expected pattern key to be added to knownPatterns")
	}
}

func TestDreamLearningCollector_MarkPersisted(t *testing.T) {
	collector := NewDreamLearningCollector()
	l := &DreamLearning{
		ID:        "L1",
		Persisted: false,
	}
	collector.confirmed = append(collector.confirmed, l)

	collector.MarkPersisted("L2", "DbStore") // non-matching, no-op
	if l.Persisted {
		t.Error("Expected Persisted to remain false")
	}

	collector.MarkPersisted("L1", "DbStore") // matching
	if !l.Persisted {
		t.Error("Expected Persisted to be true")
	}
	if l.PersistedTo != "DbStore" {
		t.Errorf("Expected persisted to 'DbStore', got %q", l.PersistedTo)
	}
}

func TestDreamLearningCollector_GetStats(t *testing.T) {
	collector := NewDreamLearningCollector()

	// Empty stats
	stats := collector.GetStats()
	if stats["staged_count"] != 0 || stats["confirmed_count"] != 0 {
		t.Errorf("Expected 0 staged/confirmed, got %v", stats)
	}

	// Populated stats
	collector.staged["L1"] = &DreamLearning{ID: "L1"}
	collector.confirmed = append(collector.confirmed, &DreamLearning{
		ID:         "L2",
		Type:       LearningTypeProcedural,
		Confidence: 0.8,
		Persisted:  false,
	})
	collector.confirmed = append(collector.confirmed, &DreamLearning{
		ID:         "L3",
		Type:       LearningTypeProcedural,
		Confidence: 0.6,
		Persisted:  true,
	})

	stats = collector.GetStats()
	if stats["staged_count"] != 1 {
		t.Errorf("Expected 1 staged, got %v", stats["staged_count"])
	}
	if stats["confirmed_count"] != 2 {
		t.Errorf("Expected 2 confirmed, got %v", stats["confirmed_count"])
	}
	if stats["avg_confidence"] != 0.7 {
		t.Errorf("Expected average confidence 0.7, got %v", stats["avg_confidence"])
	}
	if stats["pending_persistence"] != 1 {
		t.Errorf("Expected 1 pending persistence, got %v", stats["pending_persistence"])
	}
}

func TestClassifyRiskType(t *testing.T) {
	tests := []struct {
		content  string
		expected string
	}{
		{"critical security credential secret leak", "security"},
		{"loss of data and corrupt data integrity", "data_integrity"},
		{"performance is slow and memory leak", "performance"},
		{"crash or fail or break the system stability", "stability"},
		{"production release deployment issue", "deployment"},
		{"something else totally general", "general"},
	}

	for _, tt := range tests {
		got := classifyRiskType(tt.content)
		if got != tt.expected {
			t.Errorf("classifyRiskType(%q) = %q, want %q", tt.content, got, tt.expected)
		}
	}
}

func TestDreamLearningCollector_GetPendingAndConfirmed(t *testing.T) {
	collector := NewDreamLearningCollector()

	collector.staged["L1"] = &DreamLearning{ID: "L1"}
	collector.confirmed = append(collector.confirmed, &DreamLearning{ID: "L2"})

	pending := collector.GetPendingLearnings()
	if len(pending) != 1 || pending[0].ID != "L1" {
		t.Errorf("Expected pending to contain 'L1', got %v", pending)
	}

	confirmed := collector.GetConfirmedLearnings()
	if len(confirmed) != 1 || confirmed[0].ID != "L2" {
		t.Errorf("Expected confirmed to contain 'L2', got %v", confirmed)
	}
}

func TestDreamLearningCollector_ClearStaged(t *testing.T) {
	collector := NewDreamLearningCollector()
	collector.staged["L1"] = &DreamLearning{ID: "L1"}

	collector.ClearStaged()
	if len(collector.staged) != 0 {
		t.Errorf("Expected staged to be empty after ClearStaged, got %d", len(collector.staged))
	}
}
