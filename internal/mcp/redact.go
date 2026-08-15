package mcp

import (
	"regexp"
	"strings"
)

// Secret redaction for anything an MCP server sends us that ends up in a log
// line. MCP servers are configured integrations, not trusted code paths: their
// stderr, notifications, and tool payloads routinely echo the credentials they
// were handed. Logs outlive sessions and get pasted into bug reports, so the
// redaction happens at the log boundary rather than relying on servers to
// behave.
//
// The patterns are deliberately shape-based. Matching on key names alone misses
// bearer tokens embedded in free text; matching on entropy alone flags hashes
// and IDs. Both are applied.
var secretPatterns = []*regexp.Regexp{
	// key: value / "key": "value" / key=value for credential-ish key names.
	// The surrounding [A-Za-z0-9_.-]* is what catches prefixed and suffixed
	// forms like DB_PASSWORD or githubToken: a plain \b never fires there
	// because "_" is itself a word character.
	regexp.MustCompile(`(?i)([A-Za-z0-9_.\-]*(?:api[_-]?key|apikey|access[_-]?token|refresh[_-]?token|auth[_-]?token|secret[_-]?key|client[_-]?secret|passwd|password|secret|token|credential)[A-Za-z0-9_.\-]*)(\s*[:=]\s*|"\s*:\s*")("?)([^\s",;}]{4,})`),
	// Authorization: Bearer <token> (and Basic, Token, ...).
	regexp.MustCompile(`(?i)\b(authorization|proxy-authorization)\b(\s*:\s*)([^\s",;}]+\s+)?([^\s",;}]{8,})`),
	// Bare bearer tokens in free text.
	regexp.MustCompile(`(?i)\bbearer\s+([A-Za-z0-9._\-+/=]{12,})`),
	// Well-known provider key shapes.
	regexp.MustCompile(`\bsk-[A-Za-z0-9._\-]{16,}`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{16,}`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}`),
	// PEM private key blocks.
	regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
	// Credentials embedded in a URL.
	regexp.MustCompile(`\b([a-zA-Z][a-zA-Z0-9+.\-]*://)([^/\s:@]+):([^/\s@]+)@`),
}

// redactionPlaceholder is what replaces a matched secret. Keeping the key name
// visible is deliberate: "which credential leaked" is the useful half, and the
// value is the dangerous half.
const redactionPlaceholder = "[REDACTED]"

// RedactSecrets removes credential-shaped substrings from text destined for a
// log. It is intentionally lossy and never returns the original value for a
// matched region.
func RedactSecrets(text string) string {
	if text == "" {
		return text
	}

	out := text
	for i, pattern := range secretPatterns {
		switch i {
		case 0:
			out = pattern.ReplaceAllString(out, "${1}${2}${3}"+redactionPlaceholder)
		case 1:
			out = pattern.ReplaceAllString(out, "${1}${2}${3}"+redactionPlaceholder)
		case len(secretPatterns) - 1:
			// URL credentials: keep the scheme and user, drop the password.
			out = pattern.ReplaceAllString(out, "${1}${2}:"+redactionPlaceholder+"@")
		default:
			out = pattern.ReplaceAllString(out, redactionPlaceholder)
		}
	}
	return out
}

// redactForLog redacts and truncates a payload for a log line. Server payloads
// are unbounded; a megabyte of JSON in a log message is its own denial of
// service.
func redactForLog(payload string, maxLen int) string {
	redacted := RedactSecrets(payload)
	if maxLen > 0 && len(redacted) > maxLen {
		return redacted[:maxLen] + "...[truncated]"
	}
	return strings.TrimSpace(redacted)
}

// maxLoggedPayload bounds a single logged server payload.
const maxLoggedPayload = 512
