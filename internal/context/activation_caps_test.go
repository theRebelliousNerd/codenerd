package context

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// The per-component score caps are a safety property, not a tuning detail: an
// uncapped component lets a single adversarial or pathological fact outscore
// everything else and monopolise the atom reserve. The context backlog carries
// them as a standing constraint ("preserve issue weight clamp + score caps when
// editing activation_scoring.go"), which only survives if a test enforces it.
//
// Each case below feeds a context designed to blow past the cap several times
// over, then asserts the documented ceiling.

func TestComputeCampaignScore_WhenEveryBoostMatches_ShouldCapAt60(t *testing.T) {
	engine := NewActivationEngine(DefaultConfig())
	engine.campaignContext = &CampaignActivationContext{
		CampaignID:      "camp",
		CurrentPhase:    "camp",
		CurrentTask:     "camp",
		PhaseGoals:      []string{"camp", "camp", "camp"},
		RelevantFiles:   []string{"camp", "camp", "camp"},
		RelevantSymbols: []string{"camp", "camp", "camp"},
	}

	// Every boost condition matches this fact simultaneously.
	f := core.Fact{Predicate: "campaign_task", Args: []any{"camp camp camp"}}
	if score := engine.computeCampaignScore(f); score > 60.0 {
		t.Errorf("campaign score %f exceeds the 60.0 cap", score)
	}
}

func TestComputeIssueScore_WhenEveryBoostMatches_ShouldCapAt100(t *testing.T) {
	engine := NewActivationEngine(DefaultConfig())
	keywords := make(map[string]float64, 20)
	for i := range 20 {
		keywords[fmt.Sprintf("kw%d", i)] = 1.0
	}
	engine.issueContext = &IssueActivationContext{
		IssueID:        "iss",
		Keywords:       keywords,
		MentionedFiles: []string{"iss"},
		TieredFiles:    map[string]int{"iss": 1},
		ExpectedTests:  []string{"iss"},
	}

	var sb strings.Builder
	for i := range 20 {
		fmt.Fprintf(&sb, "kw%d ", i)
	}
	f := core.Fact{Predicate: "issue_text", Args: []any{sb.String() + " iss"}}
	if score := engine.computeIssueScore(f); score > 100.0 {
		t.Errorf("issue score %f exceeds the 100.0 cap", score)
	}
}

func TestComputeBackReferenceScore_WhenEveryBoostMatches_ShouldCapAt70(t *testing.T) {
	engine := NewActivationEngine(DefaultConfig())
	engine.backReferenceContext = &BackReferenceActivationContext{
		ReferencedTurnIDs: []int{3},
		ReferenceStrength: 1.0,
		ReferencedTopics:  []string{"ref"},
		ReferencedFiles:   []string{"ref"},
		ReferencedSymbols: []string{"ref"},
		ReferencedErrors:  []string{"ref"},
	}

	f := core.Fact{Predicate: "turn_context", Args: []any{3, "ref ref ref ref"}}
	if score := engine.computeBackReferenceScore(f); score > 70.0 {
		t.Errorf("back-reference score %f exceeds the 70.0 cap", score)
	}
}

func TestComputeDependencyScore_WhenFanInIsHuge_ShouldCapAt40(t *testing.T) {
	engine := NewActivationEngine(DefaultConfig())
	f := core.Fact{Predicate: "symbol_graph", Args: []any{"hot"}}
	key := factKey(f)
	for i := range 200 {
		engine.reverseDependencies[key] = append(engine.reverseDependencies[key], fmt.Sprintf("dep%d", i))
		engine.dependencies[key] = append(engine.dependencies[key], fmt.Sprintf("user_intent(%d)", i))
	}

	if score := engine.computeDependencyScore(f); score > 40.0 {
		t.Errorf("dependency score %f exceeds the 40.0 cap", score)
	}
}

func TestComputeFeedbackScore_WhenNoStore_ShouldStayWithinBounds(t *testing.T) {
	engine := NewActivationEngine(DefaultConfig())
	f := core.Fact{Predicate: "diagnostic", Args: []any{"x"}}
	score := engine.computeFeedbackScore(f)
	if score < -20.0 || score > 20.0 {
		t.Errorf("feedback score %f outside the documented [-20,20] band", score)
	}
}
