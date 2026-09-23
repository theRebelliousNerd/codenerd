package verification

import (
	"slices"
	"testing"
)

func TestParseVerificationResponse_StripsCodeFences(t *testing.T) {
	response := "```json\n" +
		"{\"success\":true,\"confidence\":0.9,\"reason\":\"ok\",\"quality_violations\":[],\"evidence\":[\"e\"],\"suggestions\":[\"s\"]}\n" +
		"```"

	parsed, err := parseVerificationResponse(response)
	if err != nil {
		t.Fatalf("parseVerificationResponse error: %v", err)
	}
	if !parsed.Success || parsed.Confidence != 0.9 || parsed.Reason != "ok" {
		t.Fatalf("parsed unexpected: %#v", parsed)
	}
	if len(parsed.Evidence) != 1 || parsed.Evidence[0] != "e" {
		t.Fatalf("parsed evidence unexpected: %#v", parsed.Evidence)
	}
	if len(parsed.Suggestions) != 1 || parsed.Suggestions[0] != "s" {
		t.Fatalf("parsed suggestions unexpected: %#v", parsed.Suggestions)
	}
}

func containsViolation(vs []QualityViolation, want QualityViolation) bool {
	return slices.Contains(vs, want)
}
