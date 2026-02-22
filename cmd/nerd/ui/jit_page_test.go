package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"codenerd/internal/prompt"
	"strings"
	"testing"
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

	// Update content
	model.UpdateContent(result)

	// Verify FilterValue includes content
	items := model.list.Items()
	found := false
	for _, item := range items {
		ai, ok := item.(atomItem)
		if !ok {
			continue
		}
		if ai.atom.ID == "atom1" {
			filterVal := ai.FilterValue()
			if strings.Contains(filterVal, "unique_keyword") {
				found = true
			}
		}
	}

	if !found {
		t.Errorf("Expected FilterValue to contain content 'unique_keyword', but it did not.")
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

	// Test 'y' key
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
	_, cmd = model.Update(msg)
	if cmd == nil {
		t.Errorf("Expected a tea.Cmd after pressing 'y'")
	}

	// Test 'p' key
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}
	_, cmd = model.Update(msg)
	if cmd == nil {
		t.Errorf("Expected a tea.Cmd after pressing 'p'")
	}
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
