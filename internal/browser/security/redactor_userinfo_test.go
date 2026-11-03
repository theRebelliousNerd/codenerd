package security

import (
	"strings"
	"testing"
)

// URLs that carry userinfo must not survive redaction with their credentials.
// RedactURL returns its input unchanged when the query holds no sensitive key
// and never looks at the URL's userinfo, so https://user:s3cret@host/ leaks.
func TestRedactURL_RedactsUserinfoPassword(t *testing.T) {
	const secret = "s3cretD3ploy"

	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "userinfo without query",
			raw:  "https://deploy:" + secret + "@example.com/path",
		},
		{
			name: "userinfo with a non-sensitive query",
			raw:  "https://deploy:" + secret + "@example.com/path?page=2",
		},
		{
			name: "userinfo with a port and a non-sensitive query",
			raw:  "https://deploy:" + secret + "@example.com:8443/a?b=c",
		},
		{
			name: "userinfo alongside a sensitive query key",
			raw:  "https://deploy:" + secret + "@example.com/path?token=abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRedactor(nil)
			if got := r.RedactURL(tt.raw); strings.Contains(got, secret) {
				t.Errorf("RedactURL(%q) = %q; want no userinfo password", tt.raw, got)
			}
			if got := r.RedactURL(tt.raw); !strings.Contains(got, "example.com") {
				t.Errorf("RedactURL(%q) = %q; want the host preserved", tt.raw, got)
			}
			if got := r.SanitizeString("fetching " + tt.raw + " done"); strings.Contains(got, secret) {
				t.Errorf("SanitizeString(%q) = %q; want no userinfo password", tt.raw, got)
			}
		})
	}
}

// The non-userinfo paths keep working: a plain URL is untouched and a
// sensitive query key is still redacted.
func TestRedactURL_UserinfoControls(t *testing.T) {
	r := NewRedactor(nil)

	if got := r.RedactURL("https://example.com/path?page=2"); got != "https://example.com/path?page=2" {
		t.Errorf("RedactURL of a clean URL = %q; want it unchanged", got)
	}
	if got := r.RedactURL("https://example.com/path?password=hunter2"); strings.Contains(got, "hunter2") {
		t.Errorf("RedactURL of a sensitive query = %q; want the value redacted", got)
	}
}

// TestRedactURL_RemovesUserinfoAndPreservesCleanURLs pins both sides of the
// userinfo decision, including exact preservation of an untouched URL.
func TestRedactURL_RemovesUserinfoAndPreservesCleanURLs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "password and username removed without query",
			raw:  "https://account:sample-password@example.com/path",
			want: "https://example.com/path",
		},
		{
			name: "username removed and ordinary query preserved",
			raw:  "https://account@example.com/path?b=2&a=%2f",
			want: "https://example.com/path?b=2&a=%2f",
		},
		{
			name: "clean URL with empty fragment remains byte-for-byte unchanged",
			raw:  "https://example.com/path#",
			want: "https://example.com/path#",
		},
	}
	redactor := NewRedactor(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactor.RedactURL(tt.raw); got != tt.want {
				t.Errorf("RedactURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
