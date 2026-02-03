package prompt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewJITPromptCompiler_BoundaryValues covers Vector A1 from QA analysis.
// See .quality_assurance/2026-02-01_jit_compiler_boundary_analysis.md
func TestNewJITPromptCompiler_BoundaryValues(t *testing.T) {
	t.Run("Vector A1: Nil options safety", func(t *testing.T) {
		// Verify that passing a nil CompilerOption does not panic
		// Scenario: Programmatic construction of options slice might produce nils
		assert.NotPanics(t, func() {
			var nilOpt CompilerOption = nil
			compiler, err := NewJITPromptCompiler(nilOpt)

			// Should succeed (ignore nil)
			require.NoError(t, err)
			assert.NotNil(t, compiler)
		}, "NewJITPromptCompiler(nil) should not panic")

		// Verify with mixed valid and nil options
		assert.NotPanics(t, func() {
			var nilOpt CompilerOption = nil
			compiler, err := NewJITPromptCompiler(
				WithConfig(DefaultCompilerConfig()),
				nilOpt,
				WithDefaultTokenBudget(100),
			)
			require.NoError(t, err)
			assert.NotNil(t, compiler)
			assert.Equal(t, 100, compiler.config.DefaultTokenBudget)
		})
	})

	t.Run("Vector A1: WithKernel(nil)", func(t *testing.T) {
		// Verify behavior when explicit WithKernel(nil) is passed
		assert.NotPanics(t, func() {
			compiler, err := NewJITPromptCompiler(WithKernel(nil))
			require.NoError(t, err)
			assert.NotNil(t, compiler)
			assert.Nil(t, compiler.kernel)
		})
	})
}
