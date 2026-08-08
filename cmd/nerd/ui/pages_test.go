package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/campaign"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
	"codenerd/internal/usage"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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

	// Test empty data
	model.UpdateContent(nil, nil)
	view = model.View()
	if !strings.Contains(view, "No items") {
		t.Fatalf("expected empty state message for learnings tab")
	}

	// Test rapid tab switching
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab}) // Back to patterns
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab}) // Back to learnings
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab}) // Back to patterns

	if model.activeTabIndex != 0 {
		t.Fatalf("expected active tab to be 0 after 3 switches from 1, got %d", model.activeTabIndex)
	}
	if !strings.Contains(model.View(), "No items") {
		t.Fatalf("expected empty state message for patterns tab")
	}
}

func TestCampaignPageModelViewAndUpdate(t *testing.T) {
	model := NewCampaignPageModel()
	if !strings.Contains(model.View(), "No Active Campaign") {
		t.Fatalf("expected empty campaign view, got:\n%s", model.View())
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

	// Test large dataset performance
	largeAtoms := make([]*prompt.PromptAtom, 1000)
	for i := 0; i < 1000; i++ {
		largeAtoms[i] = &prompt.PromptAtom{
			ID:          fmt.Sprintf("atom-%d", i),
			Category:    prompt.CategoryContext,
			Priority:    1,
			TokenCount:  10,
			IsMandatory: false,
			Content:     fmt.Sprintf("content %d", i),
		}
	}
	largeResult := &prompt.CompilationResult{
		IncludedAtoms: largeAtoms,
		TotalTokens:   10000,
		BudgetUsed:    0.8,
	}

	model.UpdateContent(largeResult)
	view := model.View()
	if !strings.Contains(view, "1000 items") {
		t.Fatalf("expected view to indicate 1000 items")
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

func TestAutopoiesisPageModelResize(t *testing.T) {
	model := NewAutopoiesisPageModel()

	// Initial State
	model.SetSize(80, 20)

	pt := model.tabs[0].(*PatternsTab)

	if pt.list.Width() != 76 { // 80 - 4
		t.Errorf("expected list width 76, got %d", pt.list.Width())
	}
	if pt.list.Height() != 10 { // 20 - 10
		t.Errorf("expected list height 10, got %d", pt.list.Height())
	}

	// Resize
	model.SetSize(50, 30)
	if pt.list.Width() != 46 { // 50 - 4
		t.Errorf("expected list width 46, got %d", pt.list.Width())
	}
	if pt.list.Height() != 20 { // 30 - 10
		t.Errorf("expected list height 20, got %d", pt.list.Height())
	}
}

func TestAutopoiesisPageModelJSONRendering(t *testing.T) {
	model := NewAutopoiesisPageModel()
	model.SetSize(80, 20)

	// Case 1: Valid JSON
	jsonExample := `{"key": "value", "number": 123}`
	patterns := []*autopoiesis.DetectedPattern{
		{
			PatternID:  "json-pattern",
			IssueType:  autopoiesis.IssueIncomplete,
			Confidence: 0.9,
			Examples:   []string{jsonExample},
		},
	}
	model.UpdateContent(patterns, nil)

	view := model.View()
	// glamour/chroma wraps each JSON token in ANSI color codes, which would
	// otherwise split the literal substring. Strip control codes first so the
	// assertion checks the actual rendered text content.
	plainView := ansi.Strip(view)
	if !strings.Contains(plainView, "\"key\": \"value\"") {
		t.Errorf("expected formatted JSON content in view, got: %s", plainView)
	}

	// Case 2: Invalid JSON (Plain text)
	plainExample := "Simple error message"
	patterns[0].Examples = []string{plainExample}
	model.UpdateContent(patterns, nil)

	view = model.View()
	if !strings.Contains(view, plainExample) {
		t.Errorf("expected plain text in view")
	}
}

func TestCampaignPageModelSummaryToggle(t *testing.T) {
	model := NewCampaignPageModel()
	model.SetSize(80, 50)

	camp := &campaign.Campaign{
		Title:              "Test Campaign Summary",
		Goal:               "Test goal text",
		Status:             campaign.StatusActive,
		ContextUtilization: 0.75,
		Confidence:         0.9,
		CompletedPhases:    1,
		TotalPhases:        3,
		CompletedTasks:     5,
		TotalTasks:         10,
		Phases: []campaign.Phase{
			{
				Name:   "Phase 1",
				Status: campaign.PhaseCompleted,
			},
			{
				Name:   "Phase 2",
				Status: campaign.PhaseInProgress,
				Tasks: []campaign.Task{
					{
						Description: "Task 1",
						Type:        campaign.TaskTypeTestWrite,
						Status:      campaign.TaskInProgress,
					},
				},
			},
			{
				Name:   "Phase 3",
				Status: campaign.PhasePending,
			},
		},
	}
	prog := &campaign.Progress{
		OverallProgress: 0.5,
		CurrentPhase:    "Phase 2",
		CompletedPhases: 1,
		TotalPhases:     3,
		CurrentTask:     "Task 1",
	}

	model.UpdateContent(prog, camp)

	// Verify default view
	view := model.View()
	if !strings.Contains(view, "Phase 2") {
		t.Fatalf("expected phase name in default view")
	}

	// Toggle view
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	view = newModel.View()

	// Assert summary dashboard metrics are present
	if !strings.Contains(view, "Campaign Summary") {
		t.Fatalf("expected summary title in view")
	}
	if !strings.Contains(view, "Test goal text") {
		t.Fatalf("expected goal text in view")
	}
	if !strings.Contains(view, "Phase 2") { // loose check
		t.Fatalf("expected active phase details in view")
	}
	if !strings.Contains(view, "Task 1") {
		t.Fatalf("expected active task in view")
	}
	if !strings.Contains(view, "Phases:") {
		t.Fatalf("expected phases stats in view, got: %v", view)
	}
	if !strings.Contains(view, "Tasks:") { // loose check
		t.Fatalf("expected tasks stats in view")
	}
	if !strings.Contains(view, "Budget:") { // loose check
		t.Fatalf("expected budget stat in view")
	}
	if !strings.Contains(view, "Confidence: 90.0%") {
		t.Fatalf("expected confidence stat in view")
	}
}
