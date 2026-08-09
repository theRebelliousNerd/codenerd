package research

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"codenerd/internal/browser"
)

func TestBrowserEvidenceToolReadsAndExportsBoundedTrace(t *testing.T) {
	cfg := browser.DefaultConfig()
	cfg.WorkspaceRoot = t.TempDir()
	manager := browser.NewSessionManagerWithSink(cfg, nil)
	SetBrowserManager(manager)
	defer ClearBrowserManager(manager)

	for i := 0; i < 3; i++ {
		if _, err := manager.RecordEvidence("session-a", "reason", map[string]any{"index": i, "token": "secret-value"}); err != nil {
			t.Fatalf("RecordEvidence: %v", err)
		}
	}
	raw, err := BrowserEvidenceTool().Execute(context.Background(), map[string]any{
		"operation": "read", "session_id": "session-a", "types": []any{"reason"}, "max_items": 2,
	})
	if err != nil {
		t.Fatalf("browser_evidence read: %v", err)
	}
	if strings.Contains(raw, "secret-value") || !strings.Contains(raw, "[REDACTED]") {
		t.Fatalf("read result leaked evidence: %s", raw)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("decode read: %v", err)
	}
	if decoded["count"] != float64(2) || decoded["truncated"] != true {
		t.Fatalf("unexpected bounded read: %+v", decoded)
	}

	exported, err := BrowserEvidenceTool().Execute(context.Background(), map[string]any{
		"operation": "export", "session_id": "session-a", "max_items": 2,
	})
	if err != nil {
		t.Fatalf("browser_evidence export: %v", err)
	}
	if err := json.Unmarshal([]byte(exported), &decoded); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	path, _ := decoded["path"].(string)
	content, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(content), "secret-value") {
		t.Fatalf("exported evidence = %q, %v", content, err)
	}
}
