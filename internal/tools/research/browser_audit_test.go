package research

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/browser"
	"codenerd/internal/tools"
	"codenerd/internal/types"

	"codeberg.org/TauCeti/mangle-go/analysis"
)

type auditTestKernel struct {
	mu    sync.RWMutex
	facts []types.Fact
}

func (k *auditTestKernel) LoadFacts(facts []types.Fact) error { return k.AssertBatch(facts) }
func (k *auditTestKernel) Query(query string) ([]types.Fact, error) {
	predicate := strings.TrimSpace(strings.TrimSuffix(query, "."))
	if idx := strings.IndexByte(predicate, '('); idx >= 0 {
		predicate = strings.TrimSpace(predicate[:idx])
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	result := make([]types.Fact, 0)
	for _, fact := range k.facts {
		if fact.Predicate == predicate {
			result = append(result, types.Fact{Predicate: fact.Predicate, Args: append([]any(nil), fact.Args...)})
		}
	}
	return result, nil
}
func (k *auditTestKernel) QueryAll() (map[string][]types.Fact, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	result := make(map[string][]types.Fact)
	for _, fact := range k.facts {
		result[fact.Predicate] = append(result[fact.Predicate], fact)
	}
	return result, nil
}
func (k *auditTestKernel) Assert(fact types.Fact) error {
	k.mu.Lock()
	k.facts = append(k.facts, types.Fact{Predicate: fact.Predicate, Args: append([]any(nil), fact.Args...)})
	k.mu.Unlock()
	return nil
}
func (k *auditTestKernel) AssertBatch(facts []types.Fact) error {
	for _, f := range facts {
		if err := k.Assert(f); err != nil {
			return err
		}
	}
	return nil
}
func (k *auditTestKernel) Retract(string) error                      { return nil }
func (k *auditTestKernel) RetractFact(types.Fact) error              { return nil }
func (k *auditTestKernel) UpdateSystemFacts() error                  { return nil }
func (k *auditTestKernel) GetProgramInfo() *analysis.ProgramInfo     { return nil }
func (k *auditTestKernel) Reset()                                    {}
func (k *auditTestKernel) AppendPolicy(string)                       {}
func (k *auditTestKernel) RetractExactFactsBatch([]types.Fact) error { return nil }
func (k *auditTestKernel) RemoveFactsByPredicateSet(map[string]struct{}) error {
	return nil
}

// Transaction returns a buffering transaction that applies to the same backing
// store on Commit, satisfying types.KernelTransactor so types.NewKernelTx does
// not panic on this test double. Buffered asserts become visible only after
// Commit; retracts delegate to the same no-op/filtering helpers as the
// non-transactional methods.
func (k *auditTestKernel) Transaction() types.KernelTransaction {
	return &auditTestTx{k: k}
}

type auditTestOp struct {
	kind      string
	fact      types.Fact
	predicate string
	set       map[string]struct{}
}

type auditTestTx struct {
	k   *auditTestKernel
	ops []auditTestOp
}

func (tx *auditTestTx) Assert(fact types.Fact) {
	tx.ops = append(tx.ops, auditTestOp{kind: "assert", fact: fact})
}

func (tx *auditTestTx) Retract(predicate string) {
	tx.ops = append(tx.ops, auditTestOp{kind: "retract", predicate: predicate})
}

func (tx *auditTestTx) RetractFact(fact types.Fact) {
	tx.ops = append(tx.ops, auditTestOp{kind: "retract_fact", fact: fact})
}

func (tx *auditTestTx) RetractExactFact(fact types.Fact) {
	tx.ops = append(tx.ops, auditTestOp{kind: "retract_exact", fact: fact})
}

func (tx *auditTestTx) RetractPredicateSet(predicates map[string]struct{}) {
	tx.ops = append(tx.ops, auditTestOp{kind: "retract_set", set: predicates})
}

func (tx *auditTestTx) Commit() error {
	for _, op := range tx.ops {
		var err error
		switch op.kind {
		case "assert":
			err = tx.k.Assert(op.fact)
		case "retract":
			err = tx.k.Retract(op.predicate)
		case "retract_fact":
			err = tx.k.RetractFact(op.fact)
		case "retract_exact":
			err = tx.k.RetractExactFactsBatch([]types.Fact{op.fact})
		case "retract_set":
			err = tx.k.RemoveFactsByPredicateSet(op.set)
		}
		if err != nil {
			return err
		}
	}
	tx.ops = nil
	return nil
}

func writeAuditFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		t.Fatalf("mkdir %q: %v", full, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write %q: %v", full, err)
	}
}

func TestBrowserAuditTool_ShouldDeclareOnlyDiscoverOperation(t *testing.T) {
	tool := BrowserAuditTool()
	if tool.Name != "browser_audit" {
		t.Fatalf("expected browser_audit, got %q", tool.Name)
	}
	prop, ok := tool.Schema.Properties["operation"]
	if !ok {
		t.Fatal("operation property missing")
	}
	if len(prop.Enum) != 1 {
		t.Fatalf("enum must have exactly one value, got %v", prop.Enum)
	}
	if fmt.Sprint(prop.Enum[0]) != "discover" {
		t.Fatalf("enum must be [discover], got %v", prop.Enum)
	}
	if tool.Category != tools.CategoryResearch {
		t.Fatalf("expected CategoryResearch, got %v", tool.Category)
	}
	if tool.Priority != 70 {
		t.Fatalf("expected priority 70, got %d", tool.Priority)
	}
}

func TestBrowserAuditTool_WhenRepoRootEscapesWorkspace_ShouldRefuse(t *testing.T) {
	ws := t.TempDir()
	outside := t.TempDir()
	cfg := browser.Config{WorkspaceRoot: ws}
	mgr := browser.NewSessionManagerWithSink(cfg, nil)
	kernel := &auditTestKernel{}
	SetBrowserRuntime(mgr, kernel)
	defer ClearBrowserManager(mgr)
	tool := BrowserAuditTool()
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation": "discover", "session_id": "sess-a", "repo_root": outside,
	})
	if err == nil {
		t.Fatalf("expected refusal when repo_root escapes workspace")
	}
	lower := strings.ToLower(err.Error())
	if !strings.Contains(lower, "escapes") && !strings.Contains(lower, "outside") {
		t.Fatalf("expected escapes error, got %v", err)
	}
}

func TestBrowserAuditTool_WhenWorkspaceRootUnset_ShouldRefuse(t *testing.T) {
	cfg := browser.Config{WorkspaceRoot: ""}
	mgr := browser.NewSessionManagerWithSink(cfg, nil)
	kernel := &auditTestKernel{}
	SetBrowserRuntime(mgr, kernel)
	defer ClearBrowserManager(mgr)
	tool := BrowserAuditTool()
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation": "discover", "session_id": "sess-a",
	})
	if err == nil {
		t.Fatal("expected refusal when workspace root unset")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "workspace root") {
		t.Fatalf("expected workspace root error, got %v", err)
	}
}

func TestBrowserAuditTool_ShouldNotEchoAbsolutePathInOutput(t *testing.T) {
	ws := t.TempDir()
	writeAuditFile(t, ws, "app.txt", "hello orders world")
	cfg := browser.Config{WorkspaceRoot: ws}
	mgr := browser.NewSessionManagerWithSink(cfg, nil)
	now := time.Now().UnixMilli()
	kernel := &auditTestKernel{facts: []types.Fact{
		{Predicate: "navigation_event", Args: []any{"sess-a", "/orders/page", now}},
	}}
	SetBrowserRuntime(mgr, kernel)
	defer ClearBrowserManager(mgr)
	tool := BrowserAuditTool()
	outStr, err := tool.Execute(context.Background(), map[string]any{
		"operation": "discover", "session_id": "sess-a", "view": "full",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	checkNoAbsolutePath(t, ws, outStr)
}

func checkNoAbsolutePath(t *testing.T, ws, outStr string) {
	t.Helper()
	if strings.Contains(outStr, ws) {
		t.Fatalf("output must not echo absolute workspace path %q: %s", ws, outStr)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(outStr), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	val, ok := out["repo_root_confined"]
	if !ok {
		t.Fatal("repo_root_confined missing")
	}
	b, ok := val.(bool)
	if !ok || !b {
		t.Fatalf("repo_root_confined must be boolean true, got %T %v", val, val)
	}
}

func TestBrowserAuditTool_ShouldReportCountsByFindingKind(t *testing.T) {
	ws := t.TempDir()
	writeAuditFile(t, ws, "src/orders.go", "package src\nfunc handleOrders() {}\n")
	cfg := browser.Config{WorkspaceRoot: ws}
	mgr := browser.NewSessionManagerWithSink(cfg, nil)
	now := time.Now().UnixMilli()
	kernel := &auditTestKernel{facts: []types.Fact{
		{Predicate: "navigation_event", Args: []any{"sess-a", "/orders/checkout", now}},
	}}
	SetBrowserRuntime(mgr, kernel)
	defer ClearBrowserManager(mgr)
	tool := BrowserAuditTool()
	outStr, err := tool.Execute(context.Background(), map[string]any{
		"operation": "discover", "session_id": "sess-a", "view": "compact",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	verifyAuditCounts(t, outStr)
}

func verifyAuditCounts(t *testing.T, outStr string) {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(outStr), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	countsRaw, ok := out["counts"]
	if !ok {
		t.Fatal("counts missing")
	}
	counts, ok := countsRaw.(map[string]any)
	if !ok {
		t.Fatalf("counts wrong type %T", countsRaw)
	}
	if v, ok := counts["inference"]; !ok || int(v.(float64)) != 1 {
		t.Fatalf("expected inference=1, got %v in %v", v, counts)
	}
	if v, ok := counts["observation"]; !ok || int(v.(float64)) != 1 {
		t.Fatalf("expected observation=1, got %v in %v", v, counts)
	}
	for _, kind := range []string{"observation", "inference", "skipped", "approval_required", "execution_failure", "contract_mismatch"} {
		if _, ok := counts[kind]; !ok {
			t.Fatalf("counts missing kind %q", kind)
		}
	}
}

func TestBrowserAuditTool_WhenLimitAboveCeiling_ShouldClamp(t *testing.T) {
	ws := t.TempDir()
	var b strings.Builder
	for i := 0; i < 150; i++ {
		b.WriteString("clampme line\n")
	}
	writeAuditFile(t, ws, "clamp.txt", b.String())
	cfg := browser.Config{WorkspaceRoot: ws}
	mgr := browser.NewSessionManagerWithSink(cfg, nil)
	now := time.Now().UnixMilli()
	kernel := &auditTestKernel{facts: []types.Fact{
		{Predicate: "navigation_event", Args: []any{"sess-a", "/clampme", now}},
	}}
	SetBrowserRuntime(mgr, kernel)
	defer ClearBrowserManager(mgr)
	tool := BrowserAuditTool()
	outStr, err := tool.Execute(context.Background(), map[string]any{
		"operation": "discover", "session_id": "sess-a", "view": "full", "max_matches": 10000, "max_files": 10000, "max_depth": 100, "max_file_bytes": 1000000,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	verifyClampedOutput(t, outStr)
}

func verifyClampedOutput(t *testing.T, outStr string) {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(outStr), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if mc, ok := out["match_count"].(float64); !ok || int(mc) > 100 {
		t.Fatalf("expected match_count clamped to <=100, got %v", out["match_count"])
	}
	notesRaw, ok := out["notes"]
	if !ok {
		t.Fatal("expected notes with clamp, got none")
	}
	notesSlice, ok := notesRaw.([]any)
	if !ok {
		t.Fatalf("notes wrong type %T", notesRaw)
	}
	found := false
	for _, n := range notesSlice {
		if s, ok := n.(string); ok && strings.Contains(strings.ToLower(s), "clamped") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected clamped note, got %v", notesSlice)
	}
}

func TestBrowserAuditTool_ShouldRegisterInResearchTools(t *testing.T) {
	reg := tools.NewRegistry()
	if err := RegisterAll(reg); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	if !reg.Has("browser_audit") {
		t.Fatal("browser_audit not registered")
	}
	tool := reg.Get("browser_audit")
	if tool == nil || tool.Name != "browser_audit" {
		t.Fatalf("expected browser_audit tool, got %+v", tool)
	}
}
