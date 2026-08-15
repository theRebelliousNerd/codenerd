package mcp

import (
	"strings"
	"testing"
)

func TestRedactSecrets_WhenPayloadCarriesCredentials_ShouldRemoveValues(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		leaked  string
		keepsub string
	}{
		{
			name:    "json api key",
			in:      `{"api_key": "abcd1234efgh5678", "path": "main.go"}`,
			leaked:  "abcd1234efgh5678",
			keepsub: "main.go",
		},
		{
			name:   "authorization header",
			in:     "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig",
			leaked: "eyJhbGciOiJIUzI1NiJ9.payload.sig",
		},
		{
			name:   "bare bearer token",
			in:     "retrying with bearer sk_live_0123456789abcdef",
			leaked: "sk_live_0123456789abcdef",
		},
		{
			name:   "openai style key",
			in:     "using sk-abcdefghijklmnopqrstuvwx for the call",
			leaked: "sk-abcdefghijklmnopqrstuvwx",
		},
		{
			name:   "github token",
			in:     "clone failed for ghp_abcdefghijklmnopqrstuvwxyz0123456789",
			leaked: "ghp_abcdefghijklmnopqrstuvwxyz0123456789",
		},
		{
			name:   "aws access key",
			in:     "AKIAIOSFODNN7EXAMPLE denied",
			leaked: "AKIAIOSFODNN7EXAMPLE",
		},
		{
			name:   "password in env dump",
			in:     "DB_PASSWORD=hunter2hunter2 other=fine",
			leaked: "hunter2hunter2",
		},
		{
			name:   "url credentials",
			in:     "connecting to postgres://admin:s3cretpw@db.internal:5432/app",
			leaked: "s3cretpw",
		},
		{
			name:   "private key block",
			in:     "-----BEGIN RSA PRIVATE KEY-----\nMIIEow...\n-----END RSA PRIVATE KEY-----",
			leaked: "MIIEow",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactSecrets(tc.in)
			if strings.Contains(got, tc.leaked) {
				t.Errorf("secret survived redaction:\n in: %s\nout: %s", tc.in, got)
			}
			if !strings.Contains(got, redactionPlaceholder) {
				t.Errorf("expected a %s marker, got %q", redactionPlaceholder, got)
			}
			if tc.keepsub != "" && !strings.Contains(got, tc.keepsub) {
				t.Errorf("redaction destroyed non-secret context %q: %s", tc.keepsub, got)
			}
		})
	}
}

func TestRedactSecrets_WhenPayloadIsBenign_ShouldPassThrough(t *testing.T) {
	in := `{"path":"internal/mcp/store.go","line":42,"status":"ok"}`
	if got := RedactSecrets(in); got != in {
		t.Errorf("benign payload was altered:\n in: %s\nout: %s", in, got)
	}
}

func TestRedactForLog_WhenPayloadIsHuge_ShouldTruncate(t *testing.T) {
	payload := strings.Repeat("a", maxLoggedPayload*3)
	got := redactForLog(payload, maxLoggedPayload)
	if len(got) > maxLoggedPayload+len("...[truncated]") {
		t.Errorf("log payload not truncated: %d bytes", len(got))
	}
	if !strings.HasSuffix(got, "[truncated]") {
		t.Errorf("expected a truncation marker, got %q", got[len(got)-20:])
	}
}
