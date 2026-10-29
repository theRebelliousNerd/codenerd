package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	coresys "codenerd/internal/system"
)

func TestGenerateAgentPromptsTemplate(t *testing.T) {
	// Create a temporary workspace
	tmpDir := t.TempDir()

	// Test data
	agentName := "TestAgent"
	role := "Expert in testing and quality assurance"
	topics := "Go testing, TDD, mocking frameworks"

	// Write the definition the way the wizard does once research has run.
	_, err := coresys.WriteAgentDefinition(context.Background(), tmpDir, agentName, role, topics, "", "## Key Concepts\n- table-driven tests\n\n## Pitfalls\n- t.Parallel with shared state")
	if err != nil {
		t.Fatalf("WriteAgentDefinition failed: %v", err)
	}

	// Verify the file was created
	promptsPath := filepath.Join(tmpDir, ".nerd", "agents", agentName, "prompts.yaml")
	if _, err := os.Stat(promptsPath); os.IsNotExist(err) {
		t.Fatalf("prompts.yaml was not created at %s", promptsPath)
	}

	// Read the file
	content, err := os.ReadFile(promptsPath)
	if err != nil {
		t.Fatalf("failed to read prompts.yaml: %v", err)
	}

	contentStr := string(content)

	// The identity is the template's; the researched knowledge becomes the
	// domain body, one atom per heading. With no methodology written, no
	// generic methodology atom rides along, and no atom carries a priority
	// (Jev's importance replaced it).
	for _, expected := range []string{
		"# Prompt atoms for " + agentName,
		agentName + "/identity",
		agentName + "/domain",
		agentName + "/domain/pitfalls",
		"category: \"identity\"",
		"category: \"domain\"",
		role,
		"You are " + agentName,
		"is_mandatory: true",
		"## Key Concepts\n    - table-driven tests",
		"## Pitfalls\n    - t.Parallel with shared state",
		"depends_on: [\"" + agentName + "/identity\"]",
	} {
		if !strings.Contains(contentStr, expected) {
			t.Errorf("prompts.yaml missing expected element: %q\n%s", expected, contentStr)
		}
	}
	for _, absent := range []string{"[Add ", "priority:", agentName + "/methodology", "Understand the full context"} {
		if strings.Contains(contentStr, absent) {
			t.Errorf("prompts.yaml carries %q:\n%s", absent, contentStr)
		}
	}
	if atomCount := strings.Count(contentStr, "- id:"); atomCount != 3 {
		t.Errorf("expected identity and two domain atoms, got %d", atomCount)
	}
}

func TestGenerateAgentPromptsTemplate_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Don't pre-create the directory structure
	agentName := "NewAgent"
	role := "Test role"
	topics := "Test topics"

	_, err := coresys.WriteAgentDefinition(context.Background(), tmpDir, agentName, role, topics, "", "")
	if err != nil {
		t.Fatalf("WriteAgentDefinition failed: %v", err)
	}

	// Verify the directory was created
	agentDir := filepath.Join(tmpDir, ".nerd", "agents", agentName)
	if _, err := os.Stat(agentDir); os.IsNotExist(err) {
		t.Fatalf("agent directory was not created at %s", agentDir)
	}
}

func TestGenerateAgentPromptsTemplate_ValidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	agentName := "ValidatorAgent"
	role := "Expert in validation"
	topics := "YAML, JSON schema, data validation"

	_, err := coresys.WriteAgentDefinition(context.Background(), tmpDir, agentName, role, topics, "", "")
	if err != nil {
		t.Fatalf("WriteAgentDefinition failed: %v", err)
	}

	// Read the file
	promptsPath := filepath.Join(tmpDir, ".nerd", "agents", agentName, "prompts.yaml")
	content, err := os.ReadFile(promptsPath)
	if err != nil {
		t.Fatalf("failed to read prompts.yaml: %v", err)
	}

	// Basic YAML structure validation
	contentStr := string(content)

	// Should start with a comment
	if !strings.HasPrefix(contentStr, "# Prompt atoms") {
		t.Error("prompts.yaml should start with a comment")
	}

	// Should have proper YAML list syntax
	if !strings.Contains(contentStr, "- id:") {
		t.Error("prompts.yaml missing YAML list item syntax")
	}

	// Should have proper indentation for content blocks
	if !strings.Contains(contentStr, "  content: |") {
		t.Error("prompts.yaml missing proper content block syntax")
	}
}
