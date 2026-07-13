package prompt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompilationContext_Hash(t *testing.T) {
	t.Run("nil context returns nil string", func(t *testing.T) {
		var cc *CompilationContext
		assert.Equal(t, "nil", cc.Hash())
	})

	t.Run("same context produces same hash", func(t *testing.T) {
		cc1 := &CompilationContext{
			OperationalMode: "/active",
			Language:        "/go",
			TokenBudget:     1000,
		}
		cc2 := &CompilationContext{
			OperationalMode: "/active",
			Language:        "/go",
			TokenBudget:     1000,
		}
		assert.Equal(t, cc1.Hash(), cc2.Hash())
	})

	t.Run("different contexts produce different hashes", func(t *testing.T) {
		cc1 := &CompilationContext{
			OperationalMode: "/active",
		}
		cc2 := &CompilationContext{
			OperationalMode: "/passive",
		}
		assert.NotEqual(t, cc1.Hash(), cc2.Hash())
	})

	t.Run("retry and rendered capability fields change identity", func(t *testing.T) {
		base := NewCompilationContext()
		base.AvailableTools = []string{"read_file"}

		retry := base.Clone()
		retry.PreviousAttemptNoToolCall = true
		assert.NotEqual(t, base.Hash(), retry.Hash())

		withWrite := base.Clone()
		withWrite.AvailableTools = []string{"read_file", "write_file"}
		assert.NotEqual(t, base.Hash(), withWrite.Hash())

		withSpecialist := base.Clone()
		withSpecialist.AvailableSpecialists = "- security-auditor"
		assert.NotEqual(t, base.Hash(), withSpecialist.Hash())
	})

	t.Run("set-like fields are canonical", func(t *testing.T) {
		left := NewCompilationContext()
		left.Frameworks = []string{"/gin", "/bubbletea", "/gin"}
		left.AvailableTools = []string{"write_file", "read_file", "read_file"}

		right := NewCompilationContext()
		right.Frameworks = []string{"/bubbletea", "/gin"}
		right.AvailableTools = []string{"read_file", "write_file"}

		assert.Equal(t, left.Hash(), right.Hash())
		assert.Equal(t, []string{"/gin", "/bubbletea", "/gin"}, left.Frameworks)
		assert.Equal(t, []string{"write_file", "read_file", "read_file"}, left.AvailableTools)
	})

	t.Run("budget and search fields change identity", func(t *testing.T) {
		base := NewCompilationContext()

		reserved := base.Clone()
		reserved.ReservedTokens++
		assert.NotEqual(t, base.Hash(), reserved.Hash())

		topK := base.Clone()
		topK.SemanticTopK++
		assert.NotEqual(t, base.Hash(), topK.Hash())

		newFiles := base.Clone()
		newFiles.HasNewFiles = true
		assert.NotEqual(t, base.Hash(), newFiles.Hash())

		highChurn := base.Clone()
		highChurn.IsHighChurn = true
		assert.NotEqual(t, base.Hash(), highChurn.Hash())
	})
}
