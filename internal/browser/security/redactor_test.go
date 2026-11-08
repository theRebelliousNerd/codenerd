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

func TestRedactorNilReceiverPassthrough(t *testing.T) {
	var r *Redactor
	if r.IsSensitiveKey("password") {
		t.Fatal("nil Redactor IsSensitiveKey should be false")
	}
	if got := r.SanitizeString("password=hunter2"); got != "password=hunter2" {
		t.Fatalf("nil Redactor SanitizeString = %q, want passthrough", got)
	}
	if got := r.RedactURL("https://example.test/?api_key=secret"); got != "https://example.test/?api_key=secret" {
		t.Fatalf("nil Redactor RedactURL = %q, want passthrough", got)
	}
	if got := r.RedactHeader("X-Api-Key", "secret"); got != "secret" {
		t.Fatalf("nil Redactor RedactHeader = %q, want passthrough", got)
	}
	if got := r.Sanitize("visible"); got != "visible" {
		t.Fatalf("nil Redactor Sanitize = %#v, want passthrough", got)
	}
}

func TestRedactorIsSensitiveKey_NormalizationAndSubstring(t *testing.T) {
	r := NewRedactor(nil)
	cases := []struct {
		key  string
		want bool
	}{
		{"PASSWORD", true},
		{"api_key", true},
		{"  Api Key  ", true},
		{"X-Api-Key-Field", true},
		{"my-token-value", true},
		{"my-cvv-field", false},
		{"safe", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := r.IsSensitiveKey(tc.key); got != tc.want {
			t.Errorf("IsSensitiveKey(%q) = %v, want %v", tc.key, got, tc.want)
		}
	}
}

func TestRedactHeader_Variants(t *testing.T) {
	r := NewRedactor(nil)
	if got := r.RedactHeader("X-Custom", "hello"); got != "hello" {
		t.Fatalf("RedactHeader non-sensitive = %q, want %q", got, "hello")
	}
	if got := r.RedactHeader("X-Trace", "password=hunter2"); strings.Contains(got, "hunter2") {
		t.Fatalf("RedactHeader non-sensitive should sanitize value, got %q", got)
	}
	if got := r.RedactHeader("X-Api-Key", "secret-value"); got != Redacted {
		t.Fatalf("RedactHeader generic sensitive = %q, want %q", got, Redacted)
	}
	if got := r.RedactHeader("Proxy-Authorization", "Bearer abc123"); got != "Bearer "+Redacted {
		t.Fatalf("RedactHeader proxy-authorization = %q", got)
	}
	if got := r.RedactHeader("Authorization", "top-secret"); got != Redacted {
		t.Fatalf("RedactHeader bare authorization = %q, want %q", got, Redacted)
	}
	if got := r.RedactHeader("authorization", "Basic abc123"); got != "Basic "+Redacted {
		t.Fatalf("RedactHeader lower-case authorization = %q", got)
	}
}

func TestRedactInputValue_FallbackSanitizes(t *testing.T) {
	r := NewRedactor(nil)
	if got := r.RedactInputValue("cc-number", "4111111111111111"); got != Redacted {
		t.Fatalf("RedactInputValue descriptor cc-number = %q, want %q", got, Redacted)
	}
	if got := r.RedactInputValue("username", "password=hunter2"); strings.Contains(got, "hunter2") || !strings.Contains(got, Redacted) {
		t.Fatalf("RedactInputValue non-sensitive should sanitize value, got %q", got)
	}
	var nilRedactor *Redactor
	if got := nilRedactor.RedactInputValue("type=password", "x"); got != Redacted {
		t.Fatalf("nil Redactor RedactInputValue = %q, want %q", got, Redacted)
	}
}

func TestSanitize_StructSlicePointerNil(t *testing.T) {
	r := NewRedactor(nil)
	type sample struct {
		Password string `json:"password"`
		Visible  string `json:"visible"`
		Count    int    `json:"count"`
		hidden   string
	}
	got := r.Sanitize(sample{Password: "hunter2", Visible: "hello", Count: 7, hidden: "shh"}).(map[string]any)
	if got["password"] != Redacted {
		t.Fatalf("struct password = %#v, want redacted", got["password"])
	}
	if got["visible"] != "hello" || got["count"] != 7 {
		t.Fatalf("struct safe fields = %#v", got)
	}
	if _, ok := got["hidden"]; ok {
		t.Fatalf("unexported field should be skipped, got %#v", got)
	}
	slice := r.Sanitize([]string{"hello", "password=hunter2"}).([]any)
	if slice[0] != "hello" || strings.Contains(slice[1].(string), "hunter2") {
		t.Fatalf("slice sanitization = %#v", slice)
	}
	if out := r.Sanitize(nil); out != nil {
		t.Fatalf("Sanitize(nil) = %#v, want nil", out)
	}
	s := "password=hunter2"
	if out := r.Sanitize(&s); strings.Contains(out.(string), "hunter2") {
		t.Fatalf("pointer sanitization = %#v", out)
	}
	if out := r.Sanitize(42); out != 42 {
		t.Fatalf("scalar passthrough = %#v", out)
	}
}

func TestSanitizeString_BasicAndAssignmentColon(t *testing.T) {
	r := NewRedactor(nil)
	if got := r.SanitizeString("Basic abc123"); strings.Contains(got, "abc123") || !strings.Contains(got, Redacted) {
		t.Fatalf("SanitizeString basic = %q", got)
	}
	if got := r.SanitizeString("api_key: secret-value"); strings.Contains(got, "secret-value") || !strings.Contains(got, Redacted) {
		t.Fatalf("SanitizeString colon assignment = %q", got)
	}
	if got := r.SanitizeString("hello world"); got != "hello world" {
		t.Fatalf("SanitizeString safe = %q, want unchanged", got)
	}
	if got := r.SanitizeString("not a url [[["); got != "not a url [[[" {
		t.Fatalf("RedactURL invalid input should passthrough, got %q", got)
	}
}
