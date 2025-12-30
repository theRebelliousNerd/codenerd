package ui

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/campaign"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
	"codenerd/internal/usage"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAutopoiesisPageModelUpdateAndTab(t *testing.T) {
	model := NewAutopoiesisPageModel()
	model.SetSize(80, 20)

	patterns := []*autopoiesis.DetectedPattern{
		{
			PatternID:  "pattern-1",
			IssueType:  autopoiesis.IssueIncomplete,
			Confidence: 0.75,
			Examples:   []string{"example trace"},
		},
	}
	learnings := []*autopoiesis.ToolLearning{
		{
			ToolName:        "tool-1",
			TotalExecutions: 10,
			SuccessRate:     0.6,
		},
	}

	model.UpdateContent(patterns, learnings)
	view := model.View()
	if !strings.Contains(view, "pattern-1") {
		t.Fatalf("expected pattern to be rendered")
	}

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	view = model.View()
	if !strings.Contains(view, "tool-1") {
		t.Fatalf("expected tool learning to be rendered after tab switch")
	}
}

func TestCampaignPageModelViewAndUpdate(t *testing.T) {
	model := NewCampaignPageModel()
	if !strings.Contains(model.View(), "No active campaign") {
		t.Fatalf("expected empty campaign view")
	}

	camp := &campaign.Campaign{
		Title:              "Test Campaign",
		Status:             campaign.StatusActive,
		ContextUtilization: 0.5,
		Learnings:          []campaign.Learning{{Type: "/success_pattern"}},
		RevisionNumber:     2,
		Phases: []campaign.Phase{
			{
				Name:   "Phase 1",
				Status: campaign.PhaseInProgress,
				Tasks: []campaign.Task{
					{
						Description: "Task 1",
						Type:        campaign.TaskTypeTestWrite,
						Status:      campaign.TaskInProgress,
					},
				},
			},
		},
	}
	prog := &campaign.Progress{OverallProgress: 0.25}

	model.UpdateContent(prog, camp)
	view := model.View()
	if !strings.Contains(view, "Test Campaign") {
		t.Fatalf("expected campaign title in view")
	}
	if !strings.Contains(view, "Phase 1") {
		t.Fatalf("expected phase name in view")
	}
	if !strings.Contains(view, "Task 1") {
		t.Fatalf("expected task description in view")
	}
}

func TestJITPageModelUpdateAndRender(t *testing.T) {
	model := NewJITPageModel()
	atoms := []*prompt.PromptAtom{
		{
			ID:          "atom-high",
			Category:    prompt.CategoryIdentity,
			Priority:    10,
			TokenCount:  20,
			IsMandatory: true,
			Content:     "high content",
		},
		{
			ID:          "atom-low",
			Category:    prompt.CategoryProtocol,
			Priority:    1,
			TokenCount:  5,
			IsMandatory: false,
			Content:     "low content",
		},
	}
	result := &prompt.CompilationResult{
		IncludedAtoms: atoms,
		TotalTokens:   25,
		BudgetUsed:    0.5,
	}

	model.UpdateContent(result)
	if model.lastResult == nil {
		t.Fatalf("expected compilation result to be stored")
	}
	if !strings.Contains(model.list.Title, "JIT Inspector (2 atoms, 25 tokens, 50% budget)") {
		t.Fatalf("expected list title to include stats")
	}

	content := model.renderAtomContent(atoms[0])
	if !strings.Contains(content, "Category: identity") {
		t.Fatalf("expected category in atom content")
	}
	if !strings.Contains(content, "MANDATORY") {
		t.Fatalf("expected mandatory label in atom content")
	}
}

func TestShardPageModelUpdateContent(t *testing.T) {
	model := NewShardPageModel()
	model.SetSize(80, 20)

	cfg := types.ShardConfig{
		Name: "tester",
		Type: types.ShardTypeEphemeral,
	}
	agent := coreshards.NewBaseShardAgent("shard-1", cfg)
	agent.SetState(types.ShardStateRunning)

	bp := &coreshards.BackpressureStatus{
		QueueDepth:     2,
		AvailableSlots: 1,
	}
	model.UpdateContent([]types.ShardAgent{agent}, bp)
	view := model.View()
	if !strings.Contains(view, "shard-1") {
		t.Fatalf("expected shard id in view")
	}
	if !strings.Contains(view, "Queue: 2 pending") {
		t.Fatalf("expected backpressure stats in view")
	}
}

func TestUsagePageModelContent(t *testing.T) {
	model := NewUsagePageModel(nil, DefaultStyles())
	model.SetSize(80, 20)
	model.UpdateContent()
	if !strings.Contains(model.View(), "Usage tracking not available") {
		t.Fatalf("expected empty usage message")
	}

	tracker, err := usage.NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}
	tracker.Track(context.Background(), "model-a", "provider-a", 10, 5, "complete")

	model = NewUsagePageModel(tracker, DefaultStyles())
	model.SetSize(80, 20)
	model.UpdateContent()
	view := model.View()
	if !strings.Contains(view, "Total Input") {
		t.Fatalf("expected usage totals in view")
	}
	if !strings.Contains(view, "provider-a") {
		t.Fatalf("expected provider name in view")
	}
}
