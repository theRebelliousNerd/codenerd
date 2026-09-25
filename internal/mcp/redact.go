package mcp

import "codenerd/internal/logging"

// Secret redaction for anything an MCP server sends us that ends up in a log
// line. MCP servers are configured integrations, not trusted code paths: their
// stderr, notifications, and tool payloads routinely echo the credentials they
// were handed. Logs outlive sessions and get pasted into bug reports, so the
// redaction happens at the log boundary rather than relying on servers to
// behave.
//
// The redactor is internal/logging's. This package used to carry a byte-for-byte
// copy of its pattern table, justified by a comment in logging that had the
// import graph backwards: mcp already imports logging, so mcp can share its
// redactor; only the reverse is forbidden. Two tables were two places for a new
// credential shape to be added to one and not the other.

// redactForLog redacts and truncates a server payload for a log line. Server
// payloads are unbounded; a megabyte of JSON in a log message is its own denial
// of service.
func redactForLog(payload string, maxLen int) string {
	return logging.RedactForLog(payload, maxLen)
}

// maxLoggedPayload bounds a single logged server payload.
const maxLoggedPayload = 512
