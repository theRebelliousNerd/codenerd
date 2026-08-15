package logging

import (
	"regexp"
	"strings"
)

// Secret redaction for anything this package writes to disk.
//
// The LLM I/O trace is the acute case: trace_llm_io dumps the entire prompt
// package and the raw response, and prompts are assembled from the same
// environment that holds provider keys — a tool result echoing `env`, a config
// blob pasted into context, an Authorization header quoted back in an error.
// Those files then outlive the session and get attached to bug reports.
// Redaction therefore happens at the log boundary, not by trusting every
// producer upstream to sanitise first.
//
// The patterns are deliberately shape-based, and this is a deliberate
// duplicate of internal/mcp's redactor rather than a shared import: internal/mcp
// depends on internal/logging, so the dependency can only run one way, and
// logging sits below everything and must stay import-free. If one side gains a
// pattern the other should follow — the shapes, not the code, are the contract.
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

// RedactionPlaceholder is what replaces a matched secret. Keeping the key name
// visible is deliberate: "which credential leaked" is the useful half, and the
// value is the dangerous half.
const RedactionPlaceholder = "[REDACTED]"

// RedactSecrets removes credential-shaped substrings from text destined for a
// log file. It is intentionally lossy and never returns the original value for
// a matched region.
func RedactSecrets(text string) string {
	if text == "" {
		return text
	}

	out := text
	for i, pattern := range secretPatterns {
		switch i {
		case 0, 1:
			// Keep the key name and separator; drop the value.
			out = pattern.ReplaceAllString(out, "${1}${2}${3}"+RedactionPlaceholder)
		case len(secretPatterns) - 1:
			// URL credentials: keep the scheme and user, drop the password.
			out = pattern.ReplaceAllString(out, "${1}${2}:"+RedactionPlaceholder+"@")
		default:
			out = pattern.ReplaceAllString(out, RedactionPlaceholder)
		}
	}
	return out
}

// RedactForLog redacts and then bounds a payload for a single log line. An
// unbounded payload in a log message is its own denial of service; maxLen <= 0
// disables truncation.
func RedactForLog(payload string, maxLen int) string {
	redacted := RedactSecrets(payload)
	if maxLen > 0 && len(redacted) > maxLen {
		return redacted[:maxLen] + "...[truncated]"
	}
	return strings.TrimSpace(redacted)
}

// redactTrace applies redaction to LLM trace text unless the operator has
// explicitly opted into a raw dump. Answering OPEN-QUESTIONS Q4: redaction is
// the default because the failure mode it prevents (a key in a file that gets
// pasted into an issue) is unrecoverable, while the failure mode it causes
// (a masked value in a prompt during JIT debugging) is recoverable by setting
// trace_llm_io_raw for one run.
func redactTrace(text string) string {
	if rawLLMTraceEnabled() {
		return text
	}
	return RedactSecrets(text)
}
