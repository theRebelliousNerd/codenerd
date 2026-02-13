package prompt

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebuggingAtoms(t *testing.T) {
	// Initialize loader
	loader := NewAtomLoader(nil)

	// Define test cases for each new atom
	tests := []struct {
		filename     string
		expectedID   string
		expectedLang []string
	}{
		{
			filename:     "atoms/methodology/debugging_go.yaml",
			expectedID:   "methodology/debugging/go",
			expectedLang: []string{"/go", "/golang"},
		},
		{
			filename:     "atoms/methodology/debugging_python.yaml",
			expectedID:   "methodology/debugging/python",
			expectedLang: []string{"/python"},
		},
		{
			filename:     "atoms/methodology/debugging_typescript.yaml",
			expectedID:   "methodology/debugging/typescript",
			expectedLang: []string{"/typescript", "/javascript", "/node"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.expectedID, func(t *testing.T) {
			path := filepath.Join(".", tt.filename)
			atoms, err := loader.ParseYAML(path)
			require.NoError(t, err, "Failed to parse %s", path)
			require.Len(t, atoms, 1, "Expected exactly 1 atom in %s", path)

			atom := atoms[0]
			assert.Equal(t, tt.expectedID, atom.ID)
			assert.Equal(t, CategoryMethodology, atom.Category)
			assert.Equal(t, "debugging", atom.Subcategory)

			// Verify languages
			assert.ElementsMatch(t, tt.expectedLang, atom.Languages)

			// Verify dependency
			assert.Contains(t, atom.DependsOn, "methodology/debugging/core")

			// Verify validation passes
			err = atom.Validate()
			assert.NoError(t, err, "Atom validation failed")
		})
	}
}
