package reviewer

import (
	"strings"
	"testing"
	"time"
)

func TestFormatResult_HybridOutput(t *testing.T) {
	// Setup
	shard := &ReviewerShard{}

	result := &ReviewResult{
		Files: []string{"main.go"},
		Findings: []ReviewFinding{
			{
				File:     "main.go",
				Line:     10,
				Severity: "error",
				Category: "security",
				Message:  "Hardcoded secret",
			},
		},
		Severity:       ReviewSeverityError,
		Summary:        "Found security issues",
		Duration:       100 * time.Millisecond,
		AnalysisReport: "# Agent Summary\nThis code has issues.\n\n## Holographic Analysis\nData layer code.\n\n## Campaign Status\nAligned.",
		SpecialistRecommendations: []SpecialistRecommendation{
			{
				ShardName:  "security-shard",
				Confidence: 0.9,
				Reason:     "Detected credentials",
			},
		},
	}

	// Execute
	output := shard.formatResult(result)

	// Verify Structure
	expectedParts := []string{
		"# Review Result: Found security issues",
		"❌ **Status**: ISSUES",
		"# Agent Summary",
		"## Holographic Analysis",
		"## Campaign Status",
		"## Detailed Findings",
		"| ❌ error | security | `main.go:10` | Hardcoded secret |",
		"## Specialist Recommendations",
		"- **security-shard** (90%)",
		"<!-- JSON_FINDINGS_START -->",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("Output missing expected part:\nExpected to find: %q\nActual output:\n%s", part, output)
		}
	}
}
