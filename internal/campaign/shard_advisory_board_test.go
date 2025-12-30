package campaign

import (
	"strings"
	"testing"
	"time"
)

func TestNewShardAdvisoryBoard(t *testing.T) {
	// Test with nil consultation provider
	board := NewShardAdvisoryBoard(nil)
	if board == nil {
		t.Fatal("NewShardAdvisoryBoard returned nil")
	}

	// Check default config was applied
	if board.config.MinApprovalRatio != 0.5 {
		t.Errorf("expected MinApprovalRatio 0.5, got %f", board.config.MinApprovalRatio)
	}
	if board.config.MinConfidence != 0.5 {
		t.Errorf("expected MinConfidence 0.5, got %f", board.config.MinConfidence)
	}
	if !board.config.RequireCriticalApproval {
		t.Error("expected RequireCriticalApproval true")
	}
	if len(board.config.EnabledAdvisors) != 4 {
		t.Errorf("expected 4 enabled advisors, got %d", len(board.config.EnabledAdvisors))
	}
}

func TestDefaultAdvisoryConfig(t *testing.T) {
	cfg := DefaultAdvisoryConfig()

	if cfg.ConsultTimeout != 2*time.Minute {
		t.Errorf("expected ConsultTimeout 2m, got %v", cfg.ConsultTimeout)
	}
	if cfg.MinApprovalRatio != 0.5 {
		t.Errorf("expected MinApprovalRatio 0.5, got %f", cfg.MinApprovalRatio)
	}
	if cfg.MinConfidence != 0.5 {
		t.Errorf("expected MinConfidence 0.5, got %f", cfg.MinConfidence)
	}
	if cfg.RequireUnanimous {
		t.Error("expected RequireUnanimous false")
	}
	if !cfg.RequireCriticalApproval {
		t.Error("expected RequireCriticalApproval true")
	}

	expectedAdvisors := []string{"coder", "tester", "reviewer", "researcher"}
	if len(cfg.EnabledAdvisors) != len(expectedAdvisors) {
		t.Errorf("expected %d advisors, got %d", len(expectedAdvisors), len(cfg.EnabledAdvisors))
	}
}

func TestShardAdvisoryBoard_WithConfig(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)

	customConfig := AdvisoryConfig{
		ConsultTimeout:     5 * time.Minute,
		MinApprovalRatio:   0.75,
		RequireUnanimous:   true,
		EnabledAdvisors:    []string{"coder"},
	}

	result := board.WithConfig(customConfig)

	if result != board {
		t.Error("WithConfig should return same board for chaining")
	}
	if board.config.ConsultTimeout != 5*time.Minute {
		t.Errorf("expected ConsultTimeout 5m, got %v", board.config.ConsultTimeout)
	}
	if board.config.MinApprovalRatio != 0.75 {
		t.Errorf("expected MinApprovalRatio 0.75, got %f", board.config.MinApprovalRatio)
	}
	if !board.config.RequireUnanimous {
		t.Error("expected RequireUnanimous true")
	}
}

func TestSynthesizeVotes_NoResponses(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)

	synthesis := board.SynthesizeVotes([]AdvisoryResponse{})

	if !synthesis.Approved {
		t.Error("empty responses should auto-approve")
	}
	if synthesis.Summary != "No advisors configured; plan auto-approved." {
		t.Errorf("unexpected summary: %s", synthesis.Summary)
	}
}

func TestSynthesizeVotes_AllApprove(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)

	responses := []AdvisoryResponse{
		{AdvisorName: "coder", Vote: VoteApprove, Confidence: 0.9},
		{AdvisorName: "tester", Vote: VoteApprove, Confidence: 0.85},
		{AdvisorName: "reviewer", Vote: VoteApprove, Confidence: 0.8},
	}

	synthesis := board.SynthesizeVotes(responses)

	if !synthesis.Approved {
		t.Error("all approvals should result in approved synthesis")
	}
	if synthesis.ApprovalRatio != 1.0 {
		t.Errorf("expected approval ratio 1.0, got %f", synthesis.ApprovalRatio)
	}
	if len(synthesis.BlockingConcerns) != 0 {
		t.Errorf("expected no blocking concerns, got %d", len(synthesis.BlockingConcerns))
	}
}

func TestSynthesizeVotes_CriticalReject(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)

	responses := []AdvisoryResponse{
		{AdvisorName: "coder", Vote: VoteReject, Confidence: 0.9, Reasoning: "Code quality issues"},
		{AdvisorName: "tester", Vote: VoteApprove, Confidence: 0.85},
		{AdvisorName: "reviewer", Vote: VoteApprove, Confidence: 0.8},
	}

	synthesis := board.SynthesizeVotes(responses)

	// Coder is critical - rejection should block
	if synthesis.Approved {
		t.Error("critical advisor rejection should block approval")
	}
	if len(synthesis.BlockingConcerns) != 1 {
		t.Errorf("expected 1 blocking concern, got %d", len(synthesis.BlockingConcerns))
	}
	if synthesis.BlockingConcerns[0].Advisor != "coder" {
		t.Errorf("expected blocking concern from coder, got %s", synthesis.BlockingConcerns[0].Advisor)
	}
}

func TestSynthesizeVotes_NonCriticalReject(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)

	responses := []AdvisoryResponse{
		{AdvisorName: "coder", Vote: VoteApprove, Confidence: 0.9},
		{AdvisorName: "tester", Vote: VoteApprove, Confidence: 0.85},
		{AdvisorName: "researcher", Vote: VoteReject, Confidence: 0.8, Reasoning: "Not enough research"},
	}

	synthesis := board.SynthesizeVotes(responses)

	// Researcher is not critical - majority still approves
	if !synthesis.Approved {
		t.Error("non-critical rejection with majority approval should pass")
	}
	// Researcher rejection shouldn't be blocking
	for _, concern := range synthesis.BlockingConcerns {
		if concern.Advisor == "researcher" {
			t.Error("researcher rejection should not be blocking")
		}
	}
}

func TestSynthesizeVotes_LowConfidenceIgnored(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)

	responses := []AdvisoryResponse{
		{AdvisorName: "coder", Vote: VoteApprove, Confidence: 0.9},
		{AdvisorName: "tester", Vote: VoteReject, Confidence: 0.3}, // Low confidence - should be ignored
		{AdvisorName: "reviewer", Vote: VoteApprove, Confidence: 0.8},
	}

	synthesis := board.SynthesizeVotes(responses)

	// Tester's low-confidence rejection should be ignored
	if !synthesis.Approved {
		t.Error("low confidence votes should be ignored")
	}
	// Only 2 valid votes (coder and reviewer)
	if synthesis.ApprovalRatio != 1.0 {
		t.Errorf("expected approval ratio 1.0 (2/2 valid votes), got %f", synthesis.ApprovalRatio)
	}
}

func TestSynthesizeVotes_Abstain(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)

	responses := []AdvisoryResponse{
		{AdvisorName: "coder", Vote: VoteApprove, Confidence: 0.9},
		{AdvisorName: "tester", Vote: VoteAbstain, Confidence: 0.8},
		{AdvisorName: "reviewer", Vote: VoteApprove, Confidence: 0.8},
	}

	synthesis := board.SynthesizeVotes(responses)

	if !synthesis.Approved {
		t.Error("abstentions should not count against approval")
	}
	// Only 2 counted votes (coder and reviewer)
	if synthesis.ApprovalRatio != 1.0 {
		t.Errorf("expected approval ratio 1.0, got %f", synthesis.ApprovalRatio)
	}
}

func TestSynthesizeVotes_ApproveWithNotes(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)

	responses := []AdvisoryResponse{
		{AdvisorName: "coder", Vote: VoteApproveWithNotes, Confidence: 0.9, Suggestions: []string{"Add more tests"}},
		{AdvisorName: "tester", Vote: VoteApprove, Confidence: 0.85},
	}

	synthesis := board.SynthesizeVotes(responses)

	if !synthesis.Approved {
		t.Error("approve with notes should count as approval")
	}
	if len(synthesis.AllSuggestions) != 1 {
		t.Errorf("expected 1 suggestion, got %d", len(synthesis.AllSuggestions))
	}
}

func TestSynthesizeVotes_RequestChanges(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)

	responses := []AdvisoryResponse{
		{AdvisorName: "coder", Vote: VoteRequestChanges, Confidence: 0.9, Reasoning: "Need refactoring"},
		{AdvisorName: "tester", Vote: VoteApprove, Confidence: 0.85},
	}

	synthesis := board.SynthesizeVotes(responses)

	// Coder is critical, request_changes should block
	if synthesis.Approved {
		t.Error("request_changes from critical advisor should block")
	}
	if len(synthesis.BlockingConcerns) != 1 {
		t.Errorf("expected 1 blocking concern, got %d", len(synthesis.BlockingConcerns))
	}
	if synthesis.BlockingConcerns[0].Severity != "requires_changes" {
		t.Errorf("expected severity 'requires_changes', got %s", synthesis.BlockingConcerns[0].Severity)
	}
}

func TestSynthesizeVotes_UnanimousRequired(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)
	board.config.RequireUnanimous = true
	board.config.RequireCriticalApproval = false // Only test unanimous

	responses := []AdvisoryResponse{
		{AdvisorName: "coder", Vote: VoteApprove, Confidence: 0.9},
		{AdvisorName: "tester", Vote: VoteApprove, Confidence: 0.85},
		{AdvisorName: "researcher", Vote: VoteReject, Confidence: 0.8},
	}

	synthesis := board.SynthesizeVotes(responses)

	if synthesis.Approved {
		t.Error("unanimous requirement not met - should not approve")
	}
}

func TestIncorporateFeedback_NoFeedback(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)

	synthesis := AdvisorySynthesis{
		AllConcerns:    []string{},
		AllSuggestions: []string{},
	}

	result := board.IncorporateFeedback("Original plan", synthesis)

	if result != "Original plan" {
		t.Error("no feedback should return original plan unchanged")
	}
}

func TestIncorporateFeedback_WithFeedback(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)

	synthesis := AdvisorySynthesis{
		AllConcerns:    []string{"Security concern", "Performance issue"},
		AllSuggestions: []string{"Add caching", "Use connection pool"},
		AllCaveats:     []string{"Monitor memory usage"},
	}

	result := board.IncorporateFeedback("Original plan", synthesis)

	if result == "Original plan" {
		t.Error("feedback should modify the plan")
	}
	if !strings.Contains(result, "Original plan") {
		t.Error("result should contain original plan")
	}
	if !strings.Contains(result, "Advisory Feedback Integration") {
		t.Error("result should contain feedback header")
	}
	if !strings.Contains(result, "Security concern") {
		t.Error("result should contain concerns")
	}
	if !strings.Contains(result, "Add caching") {
		t.Error("result should contain suggestions")
	}
	if !strings.Contains(result, "Monitor memory usage") {
		t.Error("result should contain caveats")
	}
}

func TestAdvisorySynthesis_FormatForContext(t *testing.T) {
	synthesis := &AdvisorySynthesis{
		Approved:       true,
		ApprovalRatio:  0.75,
		OverallConfidence: 0.85,
		Summary:        "3 approve, 1 reject",
		Recommendation: "PROCEED WITH CAUTION",
		Responses: []AdvisoryResponse{
			{AdvisorName: "coder", Vote: VoteApprove, Confidence: 0.9, Reasoning: "LGTM"},
		},
		AllConcerns:    []string{"Minor issue"},
		AllSuggestions: []string{"Add tests"},
	}

	formatted := synthesis.FormatForContext()

	if formatted == "" {
		t.Fatal("FormatForContext should not return empty string")
	}
	if !strings.Contains(formatted, "ADVISORY BOARD REVIEW") {
		t.Error("should contain header")
	}
	if !strings.Contains(formatted, "APPROVED") {
		t.Error("should indicate approval status")
	}
	if !strings.Contains(formatted, "PROCEED WITH CAUTION") {
		t.Error("should contain recommendation")
	}
	if !strings.Contains(formatted, "coder") {
		t.Error("should contain advisor name")
	}
}

func TestAdvisorySynthesis_FormatVoteSummary(t *testing.T) {
	synthesis := &AdvisorySynthesis{
		Responses: []AdvisoryResponse{
			{Vote: VoteApprove},
			{Vote: VoteApprove},
			{Vote: VoteReject},
			{Vote: VoteAbstain},
		},
	}

	summary := synthesis.FormatVoteSummary()

	if summary == "" {
		t.Fatal("FormatVoteSummary should not return empty string")
	}
	// Should contain counts
	if !strings.Contains(summary, "approve=2") {
		t.Error("should contain approve count")
	}
	if !strings.Contains(summary, "reject=1") {
		t.Error("should contain reject count")
	}
	if !strings.Contains(summary, "abstain=1") {
		t.Error("should contain abstain count")
	}
}

func TestVoteType_Constants(t *testing.T) {
	// Verify vote type constants
	if VoteApprove != "approve" {
		t.Errorf("expected VoteApprove 'approve', got %s", VoteApprove)
	}
	if VoteApproveWithNotes != "approve_with_notes" {
		t.Errorf("expected VoteApproveWithNotes 'approve_with_notes', got %s", VoteApproveWithNotes)
	}
	if VoteRequestChanges != "request_changes" {
		t.Errorf("expected VoteRequestChanges 'request_changes', got %s", VoteRequestChanges)
	}
	if VoteReject != "reject" {
		t.Errorf("expected VoteReject 'reject', got %s", VoteReject)
	}
	if VoteAbstain != "abstain" {
		t.Errorf("expected VoteAbstain 'abstain', got %s", VoteAbstain)
	}
}

func TestIsCriticalAdvisor(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)

	criticalTests := []struct {
		name     string
		expected bool
	}{
		{"coder", true},
		{"tester", true},
		{"reviewer", false},
		{"researcher", false},
		{"Coder", true},   // Case insensitive
		{"TESTER", true},  // Case insensitive
		{"unknown", false},
	}

	for _, tc := range criticalTests {
		result := board.isCriticalAdvisor(tc.name)
		if result != tc.expected {
			t.Errorf("isCriticalAdvisor(%s): expected %v, got %v", tc.name, tc.expected, result)
		}
	}
}
