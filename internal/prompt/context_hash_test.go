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
}
