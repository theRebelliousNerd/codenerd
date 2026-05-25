package ui

import (
	"codenerd/internal/prompt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestJITPageFilterByContent(t *testing.T) {
	// Create model
	model := NewJITPageModel()

	// Create atoms
	atom1 := &prompt.PromptAtom{
		ID:          "atom1",
		Category:    prompt.CategoryIdentity,
		Content:     "This is unique_keyword content.",
		IsMandatory: true,
		TokenCount:  10,
		Priority:    5,
	}
	atom2 := &prompt.PromptAtom{
		ID:          "atom2",
		Category:    prompt.CategoryIdentity,
		Content:     "Just normal stuff.",
		IsMandatory: true,
		TokenCount:  10,
		Priority:    5,
	}

	// Create compilation result
	result := &prompt.CompilationResult{
		IncludedAtoms: []*prompt.PromptAtom{atom1, atom2},
	}

	// Update content (initializes unfiltered list)
	model.UpdateContent(result)

	if len(model.list.Items()) != 2 {
		t.Errorf("Expected 2 items initially, got %d", len(model.list.Items()))
	}

	// Simulate typing into the filter input
	model.filterInput.SetValue("unique_keyword")
	model.applyFilter()

	items := model.list.Items()
	if len(items) != 1 {
		t.Fatalf("Expected 1 item after filtering, got %d", len(items))
	}

	ai, ok := items[0].(atomItem)
	if !ok || ai.atom.ID != "atom1" {
		t.Errorf("Expected filtered item to be atom1, got %v", items[0])
	}
}

func TestJITPageClipboardKeys(t *testing.T) {
	// Mock clipboard for test
	oldClipboard := clipboardWriteAll
	clipboardWriteAll = func(string) error { return nil }
	defer func() { clipboardWriteAll = oldClipboard }()

	// Create model
	model := NewJITPageModel()

	// Create atoms
	atom := &prompt.PromptAtom{
		ID:          "atom1",
		Category:    prompt.CategoryIdentity,
		Content:     "This is atom content.",
		IsMandatory: true,
		TokenCount:  10,
		Priority:    5,
	}

	// Create compilation result
	result := &prompt.CompilationResult{
		Prompt:        "This is the full prompt.",
		IncludedAtoms: []*prompt.PromptAtom{atom},
	}

	// Update content
	model.UpdateContent(result)

	// Trigger selection update by calling Update with nil
	model, _ = model.Update(nil)

	if model.selected == nil {
		t.Fatal("Expected model.selected to be set after Update(nil)")
	}

	// Test 'c' key
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}
	_, cmd := model.Update(msg)
	if cmd == nil {
		t.Errorf("Expected a tea.Cmd after pressing 'c'")
	}

	msgResult := cmd()
	model, _ = model.Update(msgResult)

	// Test 'y' key
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
	_, cmd = model.Update(msg)
	if cmd == nil {
		t.Errorf("Expected a tea.Cmd after pressing 'y'")
	}

	msgResult = cmd()
	model, _ = model.Update(msgResult)

	// Test 'p' key
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}
	_, cmd = model.Update(msg)
	if cmd == nil {
		t.Errorf("Expected a tea.Cmd after pressing 'p'")
	}

	msgResult = cmd()
	model, _ = model.Update(msgResult)
}

func TestJITPageFocusSwitching(t *testing.T) {
	model := NewJITPageModel()

	// Initial state: Focus on List (focusViewport = false)
	if model.focusViewport {
		t.Errorf("Expected initial focus to be on List (focusViewport=false), got true")
	}

	// Send Tab
	msg := tea.KeyMsg{Type: tea.KeyTab}
	model, _ = model.Update(msg)

	// Expect Focus on Viewport
	if !model.focusViewport {
		t.Errorf("Expected focus to switch to Viewport after Tab, got false")
	}

	// Send Tab again
	model, _ = model.Update(msg)

	// Expect Focus on List
	if model.focusViewport {
		t.Errorf("Expected focus to switch back to List after Tab, got true")
	}
}
