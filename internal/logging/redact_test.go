package logging

import (
	"strings"
	"testing"
)

func TestRedactSecrets_WhenTextCarriesCredentials_ShouldRemoveValues(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		absent []string
		keep   []string
	}{
		{
			name:   "openai style key",
			in:     "calling with sk-abcdefghij0123456789ABCDEF now",
			absent: []string{"sk-abcdefghij0123456789ABCDEF"},
			keep:   []string{"calling with", "now"},
		},
		{
			name:   "github token",
			in:     "remote url uses ghp_0123456789abcdefghijABCDEF",
			absent: []string{"ghp_0123456789abcdefghijABCDEF"},
		},
		{
			name:   "aws access key id",
			in:     "AKIAIOSFODNN7EXAMPLE is the key",
			absent: []string{"AKIAIOSFODNN7EXAMPLE"},
		},
		{
			name:   "slack token",
			in:     "xoxb-1234567890-abcdefghijkl",
			absent: []string{"xoxb-1234567890-abcdefghijkl"},
		},
		{
			name:   "authorization header",
			in:     "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig",
			absent: []string{"eyJhbGciOiJIUzI1NiJ9.payload.sig"},
			keep:   []string{"Authorization"},
		},
		{
			name:   "json api key field",
			in:     `{"anthropic_api_key": "s3cr3t-value-here", "model": "opus"}`,
			absent: []string{"s3cr3t-value-here"},
			keep:   []string{"anthropic_api_key", "model", "opus"},
		},
		{
			name:   "env style password",
			in:     "DB_PASSWORD=hunter2hunter2",
			absent: []string{"hunter2hunter2"},
			keep:   []string{"DB_PASSWORD"},
		},
		{
			name:   "url credentials",
			in:     "cloning https://user:tokentoken@github.com/acme/repo.git",
			absent: []string{"tokentoken"},
			keep:   []string{"https://user", "github.com/acme/repo.git"},
		},
		{
			name:   "pem private key",
			in:     "-----BEGIN RSA PRIVATE KEY-----\nMIIEow...\n-----END RSA PRIVATE KEY-----",
			absent: []string{"MIIEow"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactSecrets(tc.in)
			for _, secret := range tc.absent {
				if strings.Contains(got, secret) {
					t.Errorf("secret survived redaction: %q in %q", secret, got)
				}
			}
			for _, keep := range tc.keep {
				if !strings.Contains(got, keep) {
					t.Errorf("context lost: expected %q in %q", keep, got)
				}
			}
			if !strings.Contains(got, RedactionPlaceholder) {
				t.Errorf("expected placeholder in %q", got)
			}
		})
	}
}

func TestRedactSecrets_WhenTextIsBenign_ShouldPassThrough(t *testing.T) {
	in := "compiled 42 files in 1.3s; kernel derived next_action(/edit, \"main.go\")"
	if got := RedactSecrets(in); got != in {
		t.Errorf("benign text was altered:\n got: %q\nwant: %q", got, in)
	}
}

func TestRedactForLog_WhenPayloadExceedsLimit_ShouldTruncate(t *testing.T) {
	long := strings.Repeat("a", 200)
	got := RedactForLog(long, 50)
	if len(got) > 70 {
		t.Errorf("expected truncation, got %d chars", len(got))
	}
	if !strings.HasSuffix(got, "...[truncated]") {
		t.Errorf("expected truncation marker, got %q", got)
	}
}
