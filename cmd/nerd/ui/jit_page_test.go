package ui

import (
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
