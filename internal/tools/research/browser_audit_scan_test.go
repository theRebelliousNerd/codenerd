package research

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/browser"
	"codenerd/internal/types"
)

// LC5b: past the kernel scan bound the audit keeps what it read and says
// what it did not, instead of seeing nothing silently. A kernel error fails
// the audit instead of certifying empty evidence.

// makeAuditScanFacts generates maxBrowserKernelScan+1 facts for one
// predicate and session at test time, so the tests pin the bound without
// carrying a 2001-row fixture.
func makeAuditScanFacts(predicate, session string, arg func(i int) []any) []types.Fact {
	facts := make([]types.Fact, 0, maxBrowserKernelScan+1)
	for i := 0; i <= maxBrowserKernelScan; i++ {
		facts = append(facts, types.Fact{Predicate: predicate, Args: arg(i)})
	}
	return facts
}

func checkAuditScanNote(t *testing.T, note, predicate string) {
	t.Helper()
	if !strings.Contains(note, predicate) {
		t.Fatalf("note must name the %s scan, got %q", predicate, note)
	}
	if !strings.Contains(note, "not read") {
		t.Fatalf("note must say the remainder was not read, got %q", note)
	}
}

func TestCollectAuditRequestURLs_WhenScanLimit_KeepsRowsAndNotes(t *testing.T) {
	kernel := &auditTestKernel{facts: makeAuditScanFacts("net_request", "sess-scan", func(i int) []any {
		return []any{"sess-scan", i, "GET", fmt.Sprintf("https://api.example.com/orders/%d", i), int64(i)}
	})}
	urls, note, err := collectAuditRequestURLs(context.Background(), kernel, "sess-scan")
	if err != nil {
		t.Fatalf("a bounded scan is degraded, not failed: %v", err)
	}
	if len(urls) != maxBrowserKernelScan {
		t.Fatalf("kept %d rows, want the %d read before the scan stopped", len(urls), maxBrowserKernelScan)
	}
	if urls[0] != "https://api.example.com/orders/0" {
		t.Fatalf("first kept row = %q, want the scan's first URL", urls[0])
	}
	checkAuditScanNote(t, note, "net_request")
}

func TestCollectAuditFormFields_WhenScanLimit_KeepsRowsAndNotes(t *testing.T) {
	kernel := &auditTestKernel{facts: makeAuditScanFacts("input_event", "sess-scan", func(i int) []any {
		return []any{"sess-scan", fmt.Sprintf("field-%d", i), "change", int64(i)}
	})}
	fields, note, err := collectAuditFormFields(context.Background(), kernel, "sess-scan")
	if err != nil {
		t.Fatalf("a bounded scan is degraded, not failed: %v", err)
	}
	if len(fields) != maxBrowserKernelScan {
		t.Fatalf("kept %d rows, want the %d read before the scan stopped", len(fields), maxBrowserKernelScan)
	}
	checkAuditScanNote(t, note, "input_event")
}

func TestCollectAuditRoutes_WhenScanLimit_KeepsRoutesAndNotes(t *testing.T) {
	kernel := &auditTestKernel{facts: makeAuditScanFacts("navigation_event", "sess-scan", func(i int) []any {
		return []any{"sess-scan", fmt.Sprintf("/orders/page-%d", i), int64(i)}
	})}
	routes, notes, err := collectAuditRoutes(context.Background(), kernel, "sess-scan")
	if err != nil {
		t.Fatalf("a bounded scan is degraded, not failed: %v", err)
	}
	if len(routes) != maxBrowserKernelScan {
		t.Fatalf("kept %d routes, want the %d read before the scan stopped", len(routes), maxBrowserKernelScan)
	}
	joined := strings.Join(notes, "\n")
	checkAuditScanNote(t, joined, "navigation_event")
}

// auditScanFailingKernel answers every query with err. It embeds the Kernel
// interface so only Query needs an implementation; the scan path never calls
// the other methods.
type auditScanFailingKernel struct {
	types.Kernel
	err error
}

func (k *auditScanFailingKernel) Query(string) ([]types.Fact, error) {
	return nil, k.err
}

func TestCollectAuditRequestURLs_WhenKernelFails_ReturnsTheError(t *testing.T) {
	boom := errors.New("kernel exploded")
	kernel := &auditScanFailingKernel{err: boom}
	if _, _, err := collectAuditRequestURLs(context.Background(), kernel, "sess-scan"); !errors.Is(err, boom) {
		t.Fatalf("collector error = %v, want the kernel failure, not nil rows", err)
	}
	if _, _, err := collectAuditFormFields(context.Background(), kernel, "sess-scan"); !errors.Is(err, boom) {
		t.Fatalf("collector error = %v, want the kernel failure, not nil rows", err)
	}
	if _, _, err := collectAuditRoutes(context.Background(), kernel, "sess-scan"); !errors.Is(err, boom) {
		t.Fatalf("collector error = %v, want the kernel failure, not nil rows", err)
	}
}

func TestBrowserAudit_WhenScanLimit_NotesPartialResults(t *testing.T) {
	ws := t.TempDir()
	writeAuditFile(t, ws, "src/orders.go", "package src\nfunc handleOrders() {}\n")
	mgr := browser.NewSessionManagerWithSink(browser.Config{WorkspaceRoot: ws}, nil)
	kernel := &auditTestKernel{facts: makeAuditScanFacts("net_request", "sess-scan", func(i int) []any {
		return []any{"sess-scan", i, "GET", fmt.Sprintf("https://api.example.com/orders/%d", i), int64(i)}
	})}
	SetBrowserRuntime(mgr, kernel)
	defer ClearBrowserManager(mgr)

	outStr, err := BrowserAuditTool().Execute(context.Background(), map[string]any{
		"operation": "discover", "session_id": "sess-scan", "view": "summary",
	})
	if err != nil {
		t.Fatalf("a bounded scan degrades the audit, it does not fail it: %v", err)
	}
	var out struct {
		Success bool     `json:"success"`
		Notes   []string `json:"notes"`
	}
	if err := json.Unmarshal([]byte(outStr), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.Success {
		t.Fatalf("success = false: %s", outStr)
	}
	checkAuditScanNote(t, strings.Join(out.Notes, "\n"), "net_request")
}

func TestBrowserAudit_WhenKernelFails_FailsTheAudit(t *testing.T) {
	ws := t.TempDir()
	mgr := browser.NewSessionManagerWithSink(browser.Config{WorkspaceRoot: ws}, nil)
	kernel := &auditScanFailingKernel{err: errors.New("kernel exploded")}
	SetBrowserRuntime(mgr, kernel)
	defer ClearBrowserManager(mgr)

	outStr, err := BrowserAuditTool().Execute(context.Background(), map[string]any{
		"operation": "discover", "session_id": "sess-scan", "view": "summary",
	})
	if err == nil {
		t.Fatalf("a kernel failure must fail the audit, got success: %s", outStr)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "browser audit") {
		t.Fatalf("error must identify the audit, got %v", err)
	}
}
