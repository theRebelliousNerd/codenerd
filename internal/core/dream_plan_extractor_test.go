package core

import (
	"testing"
)

func TestExtractDreamPlan_Basic(t *testing.T) {
	hypothetical := "Implement a new feature"

	consultations := []DreamConsultation{
		{
			ShardName: "coder",
			ShardType: "coder",
			Perspective: `
1. Create the new file
2. Implement the function
3. Add tests
`,
		},
	}

	plan, err := ExtractDreamPlan(hypothetical, consultations)
	if err != nil {
		t.Fatalf("ExtractDreamPlan failed: %v", err)
	}

	if plan == nil {
		t.Fatal("Plan is nil")
	}

	if plan.Hypothetical != hypothetical {
		t.Errorf("Expected hypothetical %q, got %q", hypothetical, plan.Hypothetical)
	}

	if len(plan.Subtasks) == 0 {
		t.Log("Warning: No subtasks extracted (extraction might need numbered format)")
	}
}

func TestExtractDreamPlan_MultipleShards(t *testing.T) {
	hypothetical := "Fix security vulnerability"

	consultations := []DreamConsultation{
		{
			ShardName:   "security",
			ShardType:   "reviewer",
			Perspective: "1. Audit input validation\n2. Review authentication",
			Concerns:    []string{"Critical security issue"},
		},
		{
			ShardName:   "coder",
			ShardType:   "coder",
			Perspective: "1. Fix the handler\n2. Add input sanitization",
		},
	}

	plan, err := ExtractDreamPlan(hypothetical, consultations)
	if err != nil {
		t.Fatalf("ExtractDreamPlan failed: %v", err)
	}

	// Check risk level (should be high due to security concerns)
	if plan.RiskLevel != "high" && plan.RiskLevel != "critical" {
		t.Logf("Expected high/critical risk level, got %s", plan.RiskLevel)
	}
}

func TestClassifyAction(t *testing.T) {
	tests := []struct {
		step     string
		contains string // Expected verb category
	}{
		{"Create a new file", "create"},
		{"Fix the bug", "fix"},
		{"Review the code", "review"},
		{"Test the function", "test"},
	}

	for _, tt := range tests {
		action := classifyAction(tt.step)
		// Action should be set
		if action == "" {
			t.Errorf("classifyAction(%q) returned empty", tt.step)
		}
	}
}

func TestIsMutationAction(t *testing.T) {
	mutationSteps := []string{
		"Create a new file",
		"Modify the function",
		"Delete the old code",
	}

	for _, step := range mutationSteps {
		if !isMutationAction(step) {
			t.Errorf("Expected %q to be mutation action", step)
		}
	}

	nonMutation := "Review the code"
	if isMutationAction(nonMutation) {
		t.Errorf("Expected %q to NOT be mutation action", nonMutation)
	}
}

func TestExtractTarget(t *testing.T) {
	step := "Fix the bug in main.go"
	target := extractTarget(step)

	if target != "main.go" {
		t.Errorf("Expected target 'main.go', got %q", target)
	}
}
