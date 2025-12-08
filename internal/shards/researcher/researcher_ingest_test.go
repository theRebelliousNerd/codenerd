package researcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIngestDocumentation_Robust verifies that the researcher handles
// complex, messy documentation folder structures correctly.
func TestIngestDocumentation_Robust(t *testing.T) {
	// 1. Setup a temporary test workspace
	tmpDir, err := os.MkdirTemp("", "nerd_research_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 2. Create a messy structure with "Goofy" names but relevant content
	structure := map[string]string{
		"Docs/goals.md": `# Project Goals
Distrubtion is key.`,

		"Docs/weird_stuff/but_actually_specs/frontend_spec.md": `# Frontend Spec
Use htmx.`,

		"Docs/legacy/OLD_roadmap.md": `# Roadmap 2024
Old truth.`,

		// New: Expanded target dir
		"Research/competitors.md": `# Competitor Analysis
They are weak.`,

		// New: Heuristic match (random folder, but clear header)
		"RandomFolder/my_life_story.md": `# My Story
It began in 1990...`, // Should SKIP - No signal keywords

		"RandomFolder/secret_prophecy.md": `# Vision: The Future
We will build the ultimate life coach.`, // Should MATCH heuristic "Vision"

		// New: Heuristic match (Audit)
		"Audits/security_audit_2024.md": `# Security Audit
All clean.`, // MATCHES target dir "Audits"

		"RandomFolder/architecture.md": `# System Architecture
Microservices.`, // MATCHES filename
	}

	for path, content := range structure {
		fullPath := filepath.Join(tmpDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// 3. Initialize Researcher
	r := NewResearcherShard() // Uses default config
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 4. Run IngestDocumentation
	atoms, err := r.IngestDocumentation(ctx, tmpDir)
	require.NoError(t, err)

	// 5. Verify Results "Hard" Checks

	// We check for specific files we expect to find
	foundFiles := make(map[string]bool)
	for _, atom := range atoms {
		foundFiles[filepath.Base(atom.SourceURL)] = true
		assert.Equal(t, 1.0, atom.Confidence, "Documentation should be treated as high confidence truth")
		assert.Equal(t, "project_truth", atom.Concept, "Concept should be project_truth")
	}

	// Core Docs
	assert.True(t, foundFiles["goals.md"], "Docs/goals.md (Standard)")
	assert.True(t, foundFiles["frontend_spec.md"], "Docs/.../frontend_spec.md (Nested)")
	assert.True(t, foundFiles["OLD_roadmap.md"], "Docs/.../OLD_roadmap.md (Legacy)")

	// New Feature Checks
	assert.True(t, foundFiles["competitors.md"], "Research/competitors.md (New Target Dir)")
	assert.True(t, foundFiles["security_audit_2024.md"], "Audits/security_audit_2024.md (New Target Dir)")
	assert.True(t, foundFiles["secret_prophecy.md"], "RandomFolder/secret_prophecy.md (Heuristic Header Match)")
	assert.True(t, foundFiles["architecture.md"], "RandomFolder/architecture.md (Filename Match)")

	// Negative Checks
	assert.False(t, foundFiles["my_life_story.md"], "RandomFolder/my_life_story.md (Noise should be skipped)")

	assert.GreaterOrEqual(t, len(atoms), 7, "Should find at least 7 valid documentation files")
}
