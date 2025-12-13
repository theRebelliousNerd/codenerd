package verification

import (
	"strings"
	"testing"
)

func TestIsReviewTask(t *testing.T) {
	cases := []struct {
		task string
		want bool
	}{
		{task: "review internal/core/kernel.go", want: true},
		{task: "security_scan internal", want: true},
		{task: "please audit this patch", want: true},
		{task: "implement feature X", want: false},
		{task: "run unit tests", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.task, func(t *testing.T) {
			if got := isReviewTask(tc.task); got != tc.want {
				t.Fatalf("isReviewTask(%q) = %v, want %v", tc.task, got, tc.want)
			}
		})
	}
}

func TestBasicQualityCheck(t *testing.T) {
	v := &TaskVerifier{}

	t.Run("clean", func(t *testing.T) {
		res := v.basicQualityCheck("all good")
		if !res.Success || len(res.QualityViolations) != 0 {
			t.Fatalf("clean result = %#v", res)
		}
	})

	t.Run("detects_common_violations", func(t *testing.T) {
		res := v.basicQualityCheck("TODO: implement\nfunc MockThing() {}\npanic(\"not implemented\")\nplaceholder stub")
		if res.Success {
			t.Fatalf("Success=true, want false: %#v", res)
		}

		if !containsViolation(res.QualityViolations, PlaceholderCode) {
			t.Fatalf("missing PlaceholderCode: %#v", res.QualityViolations)
		}
		if !containsViolation(res.QualityViolations, MockCode) {
			t.Fatalf("missing MockCode: %#v", res.QualityViolations)
		}
		if !containsViolation(res.QualityViolations, IncompleteImpl) {
			t.Fatalf("missing IncompleteImpl: %#v", res.QualityViolations)
		}
	})
}

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

func TestTruncateHelpers(t *testing.T) {
	t.Run("truncateContext", func(t *testing.T) {
		got := truncateContext("0123456789abcdef", 10)
		if !strings.HasPrefix(got, "0123456789") || !strings.Contains(got, "[truncated]") {
			t.Fatalf("truncateContext unexpected: %q", got)
		}
		if got2 := truncateContext("short", 10); got2 != "short" {
			t.Fatalf("truncateContext short = %q, want %q", got2, "short")
		}
	})

	t.Run("truncateForVerification", func(t *testing.T) {
		long := strings.Repeat("a", 9000)
		got := truncateForVerification(long)
		if len(got) <= 8000 {
			t.Fatalf("truncateForVerification len=%d, want > 8000", len(got))
		}
		if !strings.HasSuffix(got, "[truncated]") {
			t.Fatalf("truncateForVerification missing suffix: %q", got[len(got)-32:])
		}
	})
}

func containsViolation(vs []QualityViolation, want QualityViolation) bool {
	for _, v := range vs {
		if v == want {
			return true
		}
	}
	return false
}
