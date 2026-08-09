package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	browsersecurity "codenerd/internal/browser/security"
	"codenerd/internal/mangle"
)

func TestFlightRecorderRedactsFactsAndBoundsReads(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig()
	cfg.WorkspaceRoot = root
	cfg.MaxEvidenceFiles = 4
	manager := NewSessionManagerWithSink(cfg, nil)
	if !manager.EvidenceEnabled() {
		t.Fatal("expected workspace flight recorder")
	}
	now := time.Now()
	if err := manager.addFacts([]mangle.Fact{
		{Predicate: "net_request", Args: []any{"session-a", "req-1", "GET", "https://example.test/api?token=secret-token", "fetch", now.UnixMilli()}, Timestamp: now},
		{Predicate: "input_event", Args: []any{"session-a", "password-field", "secret-password", now.UnixMilli()}, Timestamp: now},
	}); err != nil {
		t.Fatalf("addFacts: %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, err := manager.RecordEvidence("session-a", "tool", map[string]any{"index": i}); err != nil {
			t.Fatalf("RecordEvidence: %v", err)
		}
	}

	result, err := manager.ReadEvidence("session-a", FlightReadOptions{MaxItems: 2})
	if err != nil {
		t.Fatalf("ReadEvidence: %v", err)
	}
	if len(result.Events) != 2 || !result.Truncated {
		t.Fatalf("unexpected bounded read: %+v", result)
	}
	path := manager.recorder.sessionPath("session-a")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(content)
	if strings.Contains(text, "secret-token") || strings.Contains(text, "secret-password") || !strings.Contains(text, "[REDACTED]") {
		t.Fatalf("flight trace redaction failed: %s", text)
	}
	private, err := browsersecurity.IsPrivatePath(path, false)
	if err != nil || !private {
		t.Fatalf("trace private policy = %v, %v", private, err)
	}
}

func TestFlightRecorderRotatesAndPrunesInsideTraceRoot(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig()
	cfg.WorkspaceRoot = root
	cfg.MaxEvidenceFiles = 2
	cfg.MaxEvidenceFileBytes = 256
	manager := NewSessionManagerWithSink(cfg, nil)
	for i := 0; i < 8; i++ {
		if _, err := manager.RecordEvidence("session-a", "large", map[string]any{"index": i, "payload": strings.Repeat("x", 180)}); err != nil {
			t.Fatalf("RecordEvidence %d: %v", i, err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(defaultEvidenceDir)))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	traceCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "flight_") && strings.HasSuffix(entry.Name(), ".jsonl") {
			traceCount++
		}
	}
	if traceCount == 0 || traceCount > 2 {
		t.Fatalf("trace count = %d, want 1..2", traceCount)
	}
}

func TestFlightRecorderExportIsConfinedAndOwnerOnly(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig()
	cfg.WorkspaceRoot = root
	manager := NewSessionManagerWithSink(cfg, nil)
	if _, err := manager.RecordEvidence("session-a", "reason", map[string]any{"status": "ok"}); err != nil {
		t.Fatalf("RecordEvidence: %v", err)
	}
	if _, _, err := manager.ExportEvidence("session-a", filepath.Join("..", "..", "..", "escape.jsonl"), FlightReadOptions{}); err == nil {
		t.Fatal("expected traversal export refusal")
	}
	path, result, err := manager.ExportEvidence("session-a", "", FlightReadOptions{})
	if err != nil {
		t.Fatalf("ExportEvidence: %v", err)
	}
	if len(result.Events) != 1 || !strings.HasPrefix(filepath.Clean(path), filepath.Join(root, ".nerd", "browser", "traces")) {
		t.Fatalf("unexpected export: path=%s result=%+v", path, result)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var event FlightEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(content))), &event); err != nil || event.Type != "reason" {
		t.Fatalf("export event = %+v, %v", event, err)
	}
	private, err := browsersecurity.IsPrivatePath(path, false)
	if err != nil || !private {
		t.Fatalf("export private policy = %v, %v", private, err)
	}
	if _, _, err := manager.ExportEvidence("session-a", path, FlightReadOptions{}); err == nil {
		t.Fatal("evidence export overwrote an existing path")
	}
}

func TestSafeFlightSessionDoesNotCollapseUnsafeNames(t *testing.T) {
	if safeFlightSession("a/b") == safeFlightSession("ab") {
		t.Fatal("unsafe session names collided")
	}
}

func TestFlightRecorderCustomDirectoryAndConfigurationHardCaps(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig()
	cfg.WorkspaceRoot = root
	cfg.EvidenceDir = filepath.Join("artifacts", "browser-evidence")
	cfg.WritableRoots = []string{filepath.Join(root, "artifacts")}
	cfg.MaxEvidenceFiles = 1000
	cfg.MaxEvidenceFileBytes = 1 << 30
	manager := NewSessionManagerWithSink(cfg, nil)
	if manager.recorder == nil {
		t.Fatal("expected recorder for custom confined evidence directory")
	}
	if manager.recorder.maxFiles != 256 || manager.recorder.maxFileSize != 64<<20 {
		t.Fatalf("recorder hard caps = files:%d bytes:%d", manager.recorder.maxFiles, manager.recorder.maxFileSize)
	}
	if _, err := manager.RecordEvidence("session-a", "reason", map[string]any{"status": "ok"}); err != nil {
		t.Fatal(err)
	}
	path, _, err := manager.ExportEvidence("session-a", "", FlightReadOptions{})
	if err != nil {
		t.Fatalf("custom evidence export: %v", err)
	}
	wantRoot := filepath.Join(root, "artifacts", "browser-evidence", "exports")
	if !strings.HasPrefix(filepath.Clean(path), wantRoot) {
		t.Fatalf("custom export path = %q, want below %q", path, wantRoot)
	}
}
