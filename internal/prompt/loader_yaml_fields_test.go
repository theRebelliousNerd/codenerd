package prompt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomLoader_ParseYAML_LoadsDescriptionAndVariants(t *testing.T) {
	dir := t.TempDir()

	contentPath := filepath.Join(dir, "body.md")
	require.NoError(t, os.WriteFile(contentPath, []byte("hello from content file"), 0644))

	yamlPath := filepath.Join(dir, "atom.yaml")
	yamlContent := `- id: "test/atom"
  category: "knowledge"
  subcategory: "test"
  description: "short description"
  content_concise: "concise content"
  content_min: "min content"
  priority: 50
  is_mandatory: false
  content_file: "body.md"
`
	require.NoError(t, os.WriteFile(yamlPath, []byte(yamlContent), 0644))

	loader := NewAtomLoader(nil)
	atoms, err := loader.ParseYAML(yamlPath)
	require.NoError(t, err)
	require.Len(t, atoms, 1)

	atom := atoms[0]
	require.Equal(t, "test/atom", atom.ID)
	require.Equal(t, CategoryKnowledge, atom.Category)
	require.Equal(t, "test", atom.Subcategory)
	require.Equal(t, "hello from content file", atom.Content)
	require.Equal(t, "short description", atom.Description)
	require.Equal(t, "concise content", atom.ContentConcise)
	require.Equal(t, "min content", atom.ContentMin)
}

