package prompt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContentForMode locks the variant selection the assembler relies on: the
// budget manager's Fit only chooses concise/min when that variant is non-empty,
// so contentForMode must return the variant for concise/min and fall back to the
// standard Content otherwise.
func TestContentForMode(t *testing.T) {
	atom := &PromptAtom{
		Content:        "FULL_STANDARD_TEXT",
		ContentConcise: "CONCISE_TEXT",
		ContentMin:     "MIN_TEXT",
	}
	assert.Equal(t, "FULL_STANDARD_TEXT", contentForMode(atom, "standard"))
	assert.Equal(t, "FULL_STANDARD_TEXT", contentForMode(atom, ""))
	assert.Equal(t, "CONCISE_TEXT", contentForMode(atom, "concise"))
	assert.Equal(t, "MIN_TEXT", contentForMode(atom, "min"))
	assert.Equal(t, "FULL_STANDARD_TEXT", contentForMode(atom, "unknown"))

	// Empty variants fall back to the standard Content.
	empty := &PromptAtom{Content: "ONLY_STANDARD"}
	assert.Equal(t, "ONLY_STANDARD", contentForMode(empty, "concise"))
	assert.Equal(t, "ONLY_STANDARD", contentForMode(empty, "min"))
}

// TestTokenCountForMode mirrors TestContentForMode for the accounting side so
// emitted text and charged/reported tokens stay in agreement.
func TestTokenCountForMode(t *testing.T) {
	atom := &PromptAtom{
		Content:        strings.Repeat("word ", 100),
		ContentConcise: "short concise",
		ContentMin:     "min",
		TokenCount:     999,
	}
	assert.Equal(t, EstimateTokens(atom.ContentConcise), tokenCountForMode(atom, "concise"))
	assert.Equal(t, EstimateTokens(atom.ContentMin), tokenCountForMode(atom, "min"))
	assert.Equal(t, 999, tokenCountForMode(atom, "standard"))
	assert.Equal(t, 999, tokenCountForMode(atom, ""))

	// Empty variant → standard TokenCount (matches contentForMode's fallback).
	empty := &PromptAtom{Content: "x", TokenCount: 42}
	assert.Equal(t, 42, tokenCountForMode(empty, "concise"))
	assert.Equal(t, 42, tokenCountForMode(empty, "min"))

	// Negative counts clamp to 0.
	neg := &PromptAtom{TokenCount: -5}
	assert.Equal(t, 0, tokenCountForMode(neg, "standard"))
}

// TestAssembler_HonorsRenderMode is the regression: Fit records the render mode
// it degraded an atom to (and charges the smaller token count); the assembler
// must emit that variant, not the full standard Content — otherwise the prompt
// silently exceeds the budget accounting and the manifest misreports.
func TestAssembler_HonorsRenderMode(t *testing.T) {
	assembler := NewFinalAssembler()

	mk := func(mode string) *OrderedAtom {
		return &OrderedAtom{
			Atom: &PromptAtom{
				ID:             "a",
				Category:       CategoryIdentity,
				Content:        "STANDARD_FULL_SIZE_TEXT",
				ContentConcise: "CONCISE_VARIANT",
				ContentMin:     "MIN_VARIANT",
			},
			Order:      0,
			RenderMode: mode,
		}
	}

	t.Run("concise emits the concise variant", func(t *testing.T) {
		out, err := assembler.Assemble([]*OrderedAtom{mk("concise")}, NewCompilationContext())
		require.NoError(t, err)
		assert.Contains(t, out, "CONCISE_VARIANT")
		assert.NotContains(t, out, "STANDARD_FULL_SIZE_TEXT")
	})

	t.Run("min emits the min variant", func(t *testing.T) {
		out, err := assembler.Assemble([]*OrderedAtom{mk("min")}, NewCompilationContext())
		require.NoError(t, err)
		assert.Contains(t, out, "MIN_VARIANT")
		assert.NotContains(t, out, "STANDARD_FULL_SIZE_TEXT")
	})

	t.Run("standard emits the full content", func(t *testing.T) {
		out, err := assembler.Assemble([]*OrderedAtom{mk("standard")}, NewCompilationContext())
		require.NoError(t, err)
		assert.Contains(t, out, "STANDARD_FULL_SIZE_TEXT")
	})

	t.Run("concise with an empty variant falls back to standard", func(t *testing.T) {
		oa := &OrderedAtom{
			Atom:       &PromptAtom{ID: "a", Category: CategoryIdentity, Content: "ONLY_STANDARD_HERE"},
			Order:      0,
			RenderMode: "concise",
		}
		out, err := assembler.Assemble([]*OrderedAtom{oa}, NewCompilationContext())
		require.NoError(t, err)
		assert.Contains(t, out, "ONLY_STANDARD_HERE")
	})
}
