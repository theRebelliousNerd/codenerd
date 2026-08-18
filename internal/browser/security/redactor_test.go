package security

import (
	"strings"
	"testing"
)

func TestRedactorSanitizesBrowserEvidence(t *testing.T) {
	r := NewRedactor([]string{"workspace-secret"})
	tests := []struct {
		name  string
		value string
		deny  string
	}{
		{name: "authorization", value: "Bearer abc.def-123", deny: "abc.def-123"},
		{name: "query", value: "https://example.test/?api_key=secret-value&safe=yes", deny: "secret-value"},
		{name: "embedded query", value: "Open https://example.test/callback?token=secret-value after login.", deny: "secret-value"},
		{name: "assignment", value: "password=hunter2", deny: "hunter2"},
		{name: "jwt", value: "eyJabcdefghijk.abcdefghijk.abcdefghijk", deny: "eyJabcdefghijk"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := r.SanitizeString(test.value)
			if strings.Contains(got, test.deny) || !strings.Contains(got, Redacted) {
				t.Fatalf("SanitizeString(%q) = %q", test.value, got)
			}
		})
	}
	if got := r.RedactHeader("Authorization", "Bearer top-secret"); got != "Bearer "+Redacted {
		t.Fatalf("RedactHeader() = %q", got)
	}
	if got := r.RedactInputValue("type=password name=login", "top-secret"); got != Redacted {
		t.Fatalf("RedactInputValue() = %q", got)
	}
}

func TestRedactorRecursivelySanitizesSensitiveInput(t *testing.T) {
	r := NewRedactor(nil)
	got := r.Sanitize(map[string]any{
		"authorization": "Bearer secret",
		"input":         map[string]any{"type": "password", "value": "hunter2"},
		"safe":          "visible",
	}).(map[string]any)
	if got["authorization"] != Redacted {
		t.Fatalf("authorization = %#v", got["authorization"])
	}
	input := got["input"].(map[string]any)
	if input["value"] != Redacted || got["safe"] != "visible" {
		t.Fatalf("unexpected recursive sanitization: %#v", got)
	}
}

func TestNewRedactor(t *testing.T) {
	extra := []string{"MY_CUSTOM_SECRET", "  spaces key  "}
	r := NewRedactor(extra)

	// Verify default keys
	defaults := []string{"password", "authorization", "api-key", "token", "cvv"}
	for _, d := range defaults {
		if !r.IsSensitiveKey(d) {
			t.Errorf("Expected default key %q to be sensitive", d)
		}
	}

	// Verify extra keys (and normalization via IsSensitiveKey)
	if !r.IsSensitiveKey("my-custom-secret") {
		t.Errorf("Expected extra key 'my-custom-secret' to be sensitive")
	}
	if !r.IsSensitiveKey("spaces-key") {
		t.Errorf("Expected extra key 'spaces-key' to be sensitive")
	}
}
