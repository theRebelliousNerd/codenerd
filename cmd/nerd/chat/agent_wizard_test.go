package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateAgentPromptsTemplate(t *testing.T) {
	// Create a temporary workspace
	tmpDir := t.TempDir()

	// Test data
	agentName := "TestAgent"
	role := "Expert in testing and quality assurance"
	topics := "Go testing, TDD, mocking frameworks"

	// Generate the template
	err := generateAgentPromptsTemplate(tmpDir, agentName, role, topics)
	if err != nil {
		t.Fatalf("generateAgentPromptsTemplate failed: %v", err)
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

	// Verify key elements are present
	expectedElements := []string{
		"# Prompt atoms for " + agentName,
		agentName + "/identity",
		agentName + "/methodology",
		agentName + "/domain_knowledge",
		"category: \"identity\"",
		"category: \"methodology\"",
		"category: \"domain_knowledge\"",
		"shard_types: [\"/" + agentName + "\"]",
		role,
		topics,
		"You are " + agentName,
		"priority: 100",
		"priority: 80",
		"priority: 70",
		"is_mandatory: true",
		"is_mandatory: false",
	}

	for _, expected := range expectedElements {
		if !strings.Contains(contentStr, expected) {
			t.Errorf("prompts.yaml missing expected element: %q", expected)
		}
	}

	// Verify structure - should have 3 atoms
	atomCount := strings.Count(contentStr, "- id:")
	if atomCount != 3 {
		t.Errorf("expected 3 atoms, got %d", atomCount)
	}

	// Verify dependency structure
	if !strings.Contains(contentStr, "depends_on: [\""+agentName+"/identity\"]") {
		t.Error("methodology atom missing dependency on identity")
	}

	if !strings.Contains(contentStr, "depends_on: [\""+agentName+"/identity\", \""+agentName+"/methodology\"]") {
		t.Error("domain_knowledge atom missing dependencies")
	}
}

func TestGenerateAgentPromptsTemplate_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Don't pre-create the directory structure
	agentName := "NewAgent"
	role := "Test role"
	topics := "Test topics"

	err := generateAgentPromptsTemplate(tmpDir, agentName, role, topics)
	if err != nil {
		t.Fatalf("generateAgentPromptsTemplate failed: %v", err)
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

	err := generateAgentPromptsTemplate(tmpDir, agentName, role, topics)
	if err != nil {
		t.Fatalf("generateAgentPromptsTemplate failed: %v", err)
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
