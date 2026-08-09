package research

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/browser"
	"codenerd/internal/tools"
)

// BrowserEvidenceTool exposes bounded, redacted flight-recorder reads and
// confined exports without turning trace files into an unbounded tool result.
func BrowserEvidenceTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_evidence",
		Description: `Read or export bounded, owner-only browser flight-recorder evidence. Operations: status, read, export. Evidence is session-scoped, redacted before persistence, rotated by configured size/file ceilings, and reports truncation and scan scope. Export paths remain confined to configured browser writable roots.`,
		Category:    tools.CategoryResearch,
		Priority:    74,
		Execute:     executeBrowserEvidence,
		Schema: tools.ToolSchema{
			Required: []string{"operation", "session_id"},
			Properties: map[string]tools.Property{
				"operation":  {Type: "string", Enum: []any{"status", "read", "export"}},
				"session_id": {Type: "string"},
				"since_ms":   {Type: "integer", Description: "Inclusive event timestamp lower bound"},
				"types":      {Type: "array", Description: "Optional event-type filter", Items: &tools.PropertyItems{Type: "string"}},
				"max_items":  {Type: "integer", Default: 20, Description: "Read hard-capped at 100; export at 1000"},
				"path":       {Type: "string", Description: "Optional export path under configured writable roots"},
			},
		},
	}
}

func executeBrowserEvidence(_ context.Context, args map[string]any) (string, error) {
	manager := getBrowserManager()
	sessionID := strings.TrimSpace(stringArg(args, "session_id"))
	if sessionID == "" {
		return "", fmt.Errorf("browser evidence: session_id is required")
	}
	operation := strings.ToLower(strings.TrimSpace(stringArg(args, "operation")))
	if operation == "status" {
		return marshalProgressiveResult(map[string]any{
			"success": true, "operation": operation, "session_id": sessionID,
			"enabled": manager.EvidenceEnabled(),
		})
	}
	opts := browser.FlightReadOptions{
		SinceMS: int64Arg(args, "since_ms", 0), Types: stringSliceArg(args["types"]),
		MaxItems: intArg(args, "max_items", 20),
	}
	switch operation {
	case "read":
		result, err := manager.ReadEvidence(sessionID, opts)
		if err != nil {
			return "", fmt.Errorf("browser evidence: %w", err)
		}
		return marshalProgressiveResult(map[string]any{
			"success": true, "operation": operation, "session_id": sessionID,
			"events": result.Events, "count": len(result.Events), "files_read": result.FilesRead,
			"scanned_bytes": result.ScannedBytes, "truncated": result.Truncated,
			"evidence_handles": []string{fmt.Sprintf("browser:%s:evidence:%d", sessionID, opts.SinceMS)},
		})
	case "export":
		path, result, err := manager.ExportEvidence(sessionID, stringArg(args, "path"), opts)
		if err != nil {
			return "", fmt.Errorf("browser evidence: %w", err)
		}
		return marshalProgressiveResult(map[string]any{
			"success": true, "operation": operation, "session_id": sessionID,
			"path": path, "count": len(result.Events), "files_read": result.FilesRead,
			"scanned_bytes": result.ScannedBytes, "truncated": result.Truncated,
			"evidence_handles": []string{fmt.Sprintf("browser:%s:evidence:export", sessionID)},
		})
	default:
		return "", fmt.Errorf("browser evidence: unsupported operation %q", operation)
	}
}

func recordBrowserToolEvidence(sessionID, eventType string, data map[string]any) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	_, _ = getBrowserManager().RecordEvidence(sessionID, eventType, data)
}
