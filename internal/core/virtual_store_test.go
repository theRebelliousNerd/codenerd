package core

import (
	"codeberg.org/TauCeti/mangle-go/analysis"

	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

type stubKernel struct {
	permitted []Fact
	safe      []Fact
	asserted  []Fact
}

func (s *stubKernel) LoadFacts([]Fact) error { return nil }
func (s *stubKernel) Query(predicate string) ([]Fact, error) {
	if predicate == "permitted" {
		return s.permitted, nil
	}
	if predicate == "safe_action" {
		return s.safe, nil
	}
	return nil, nil
}
func (s *stubKernel) QueryAll() (map[string][]Fact, error) { return nil, nil }
func (s *stubKernel) Assert(f Fact) error                  { s.asserted = append(s.asserted, f); return nil }
func (s *stubKernel) AssertBatch(facts []Fact) error {
	s.asserted = append(s.asserted, facts...)
	return nil
}
func (s *stubKernel) Retract(string) error                                { return nil }
func (s *stubKernel) RetractFact(Fact) error                              { return nil }
func (s *stubKernel) UpdateSystemFacts() error                            { return nil }
func (s *stubKernel) Reset()                                              {}
func (s *stubKernel) AppendPolicy(string)                                 {}
func (s *stubKernel) RemoveFactsByPredicateSet(map[string]struct{}) error { return nil }
func (s *stubKernel) RetractExactFactsBatch([]Fact) error                 { return nil }

func TestRouteActionBlockedWhenNotPermitted(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.DisableBootGuard()
	k := &stubKernel{
		permitted: []Fact{
			{Predicate: "permitted", Args: []any{"/read_file", "allowed.go", "{}"}},
		},
	}
	vs.SetKernel(k)

	t.Run("denied target", func(t *testing.T) {
		_, err := vs.RouteAction(context.Background(), Fact{
			Predicate: "next_action",
			Args:      []any{"act_1", "/read_file", "denied.go"},
		})
		if err == nil {
			t.Fatalf("expected denied.go to be blocked by kernel permission gate")
		}
		if !strings.Contains(err.Error(), "not permitted") {
			t.Fatalf("expected kernel-permission refusal, got: %v", err)
		}
		for _, f := range k.asserted {
			if f.Predicate == "security_violation" {
				return
			}
		}
		t.Fatalf("expected a security_violation fact to be asserted on denial, got %v", k.asserted)
	})

	t.Run("allowed target", func(t *testing.T) {
		_, err := vs.RouteAction(context.Background(), Fact{
			Predicate: "next_action",
			Args:      []any{"act_1", "/read_file", "allowed.go"},
		})
		if err != nil && strings.Contains(err.Error(), "not permitted") {
			t.Fatalf("allowed.go must not fail with a permission refusal, got: %v", err)
		}
	})
}

func TestExecCmdDisallowedBinary(t *testing.T) {
	cfg := DefaultVirtualStoreConfig()
	cfg.AllowedBinaries = []string{"allowed"}
	vs := NewVirtualStoreWithConfig(nil, cfg)

	res, err := vs.handleExecCmd(context.Background(), ActionRequest{
		Type:   ActionExecCmd,
		Target: "echo hi",
		Payload: map[string]any{
			"binary": "forbidden",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error == "" {
		t.Fatalf("expected disallowed binary to fail, got success=%v error=%q", res.Success, res.Error)
	}
}

func TestCommandFromActionRequest_PayloadOverridesTarget(t *testing.T) {
	req := ActionRequest{
		Target: "go test ./... -count=1",
		Payload: map[string]any{
			"command": "go test ./internal/core/... -count=1",
		},
	}
	got := commandFromActionRequest(req, "go test ./...")
	if got != "go test ./internal/core/... -count=1" {
		t.Fatalf("unexpected command: %q", got)
	}
}

func TestCommandFromActionRequest_TargetFallback(t *testing.T) {
	req := ActionRequest{
		Target:  "go test ./internal/core/... -count=1",
		Payload: map[string]any{},
	}
	got := commandFromActionRequest(req, "go test ./...")
	if got != "go test ./internal/core/... -count=1" {
		t.Fatalf("unexpected command: %q", got)
	}
}

func TestTimeoutSecondsFromActionRequest_DefaultAndOverrides(t *testing.T) {
	req := ActionRequest{Payload: map[string]any{}}
	if got := timeoutSecondsFromActionRequest(req, 300); got != 300 {
		t.Fatalf("expected default timeout 300, got %d", got)
	}

	req.Payload["timeout_seconds"] = 600
	if got := timeoutSecondsFromActionRequest(req, 300); got != 600 {
		t.Fatalf("expected payload timeout 600, got %d", got)
	}

	req.Payload["timeout_seconds"] = 900.0
	if got := timeoutSecondsFromActionRequest(req, 300); got != 900 {
		t.Fatalf("expected payload float timeout 900, got %d", got)
	}

	req.Payload["timeout_seconds"] = json.Number("1200")
	if got := timeoutSecondsFromActionRequest(req, 300); got != 1200 {
		t.Fatalf("expected payload json.Number timeout 1200, got %d", got)
	}

	req.Payload["timeout_seconds"] = "1500"
	if got := timeoutSecondsFromActionRequest(req, 300); got != 1500 {
		t.Fatalf("expected payload string timeout 1500, got %d", got)
	}

	req.Timeout = 42
	if got := timeoutSecondsFromActionRequest(req, 300); got != 42 {
		t.Fatalf("expected request timeout 42, got %d", got)
	}
}

// The Args slot of a learned_* fact is declared /string (schemas_memory.mg) and
// the rules that read it bind it as one value. This pins the shape hydration
// asserts: a one-argument row carries its value bare, a longer row is JSON.
func TestHydrateLearnings_ArgsSlotIsOneString(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "knowledge.db")
	db, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create local store: %v", err)
	}
	defer func() {
		_ = db.Close()
		_ = os.RemoveAll(dir)
	}()

	if err := db.StoreFact("codedom_alias", []any{"codedom means internal/tools/codedom"}, "preference", 10); err != nil {
		t.Fatalf("failed to store fact: %v", err)
	}
	if err := db.StoreFact("pair_pred", []any{"a", "b"}, "preference", 10); err != nil {
		t.Fatalf("failed to store fact: %v", err)
	}

	k := &stubKernel{}
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.SetLocalDB(db)
	vs.SetKernel(k)

	if _, err := vs.HydrateLearnings(context.Background()); err != nil {
		t.Fatalf("hydrate learnings failed: %v", err)
	}

	want := map[string]string{
		"codedom_alias": "codedom means internal/tools/codedom",
		"pair_pred":     `["a","b"]`,
	}
	for _, f := range k.asserted {
		if f.Predicate != "learned_preference" || len(f.Args) != 2 {
			continue
		}
		key, _ := f.Args[0].(string)
		expected, ok := want[key]
		if !ok {
			continue
		}
		got, isString := f.Args[1].(string)
		if !isString {
			t.Errorf("learned_preference(%q, _): Args slot is %T, want string -- the Decl is /string", key, f.Args[1])
		} else if got != expected {
			t.Errorf("learned_preference(%q, _): Args = %q, want %q", key, got, expected)
		}
		delete(want, key)
	}
	for key := range want {
		t.Errorf("learned_preference(%q, _) was never asserted", key)
	}
}

// The store held preferences, facts and constraints that the real kernel
// rejected on every boot once Decl type checks went live -- "hydrate learnings
// incomplete after 249 facts: assert preference codedom_alias: type error
// asserting learned_preference: arg 1 declared /string, got []interface {}",
// seen in a live chat session 2026-09-17. Nothing the user had taught the
// system reached the kernel. This drives the real kernel with its real Decls.
func TestHydrateLearnings_RealKernelAcceptsEveryLearnedShape(t *testing.T) {
	dir := t.TempDir()
	db, err := store.NewLocalStore(filepath.Join(dir, "knowledge.db"))
	if err != nil {
		t.Fatalf("failed to create local store: %v", err)
	}
	defer func() { _ = db.Close() }()

	sentence := "When user says 'codedom', they mean the internal tool at internal/tools/codedom"
	if err := db.StoreFact("codedom_alias", []any{sentence}, "preference", 10); err != nil {
		t.Fatalf("store preference: %v", err)
	}
	if err := db.StoreFact("project_language", []any{"go", "1.25"}, "user_fact", 10); err != nil {
		t.Fatalf("store user fact: %v", err)
	}
	if err := db.StoreFact("never_force_push", []any{}, "constraint", 10); err != nil {
		t.Fatalf("store constraint: %v", err)
	}

	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.SetLocalDB(db)
	vs.SetKernel(kernel)

	count, err := vs.HydrateLearnings(context.Background())
	if err != nil {
		t.Fatalf("the real kernel rejected hydration: %v", err)
	}
	if count < 3 {
		t.Fatalf("hydrated %d facts, want at least the 3 stored", count)
	}

	prefs, err := kernel.Query("learned_preference")
	if err != nil {
		t.Fatalf("query learned_preference: %v", err)
	}
	found := false
	for _, f := range prefs {
		if len(f.Args) == 2 && f.Args[0] == "codedom_alias" {
			found = true
			if got, _ := f.Args[1].(string); got != sentence {
				t.Errorf("learned_preference(codedom_alias, Args): Args = %#v, want the stored sentence", f.Args[1])
			}
		}
	}
	if !found {
		t.Errorf("learned_preference(codedom_alias, _) is not in the kernel after hydration; got %d learned_preference facts", len(prefs))
	}
}

func TestShardManagerGetResultCleansUp(t *testing.T) {
	sm := coreshards.NewShardManager()
	sm.RegisterShard("stub", func(id string, config types.ShardConfig) types.ShardAgent {
		return &stubShard{id: id, config: config}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	shardID, err := sm.SpawnAsync(ctx, "stub", "task")
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}

	found := false
	for !found {
		if _, ok := sm.GetResult(shardID); ok {
			found = true
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for shard result")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if _, ok := sm.GetResult(shardID); ok {
		t.Fatalf("expected result to be cleaned up after retrieval")
	}
}

type stubShard struct {
	id     string
	config types.ShardConfig
	state  types.ShardState
}

func (s *stubShard) Execute(ctx context.Context, task string) (string, error) {
	s.state = types.ShardStateCompleted
	return "ok", nil
}

func (s *stubShard) GetID() string                               { return s.id }
func (s *stubShard) GetState() types.ShardState                  { return s.state }
func (s *stubShard) GetConfig() types.ShardConfig                { return s.config }
func (s *stubShard) Stop() error                                 { return nil }
func (s *stubShard) SetParentKernel(k types.Kernel)              {}
func (s *stubShard) SetLLMClient(client types.LLMClient)         {}
func (s *stubShard) SetSessionContext(ctx *types.SessionContext) {}

// TestPermissionCacheIsClassificationOnly verifies that safe_action/1 never
// substitutes for a request-specific permitted/3 proof.
func TestPermissionCacheIsClassificationOnly(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())

	// Set up a kernel with multiple safe actions
	k := &stubKernel{
		safe: []Fact{
			{Predicate: "safe_action", Args: []any{"/read_file"}},
			{Predicate: "safe_action", Args: []any{"/write_file"}},
			{Predicate: "safe_action", Args: []any{"/review"}},
			{Predicate: "safe_action", Args: []any{"/run_tests"}},
		},
	}
	vs.SetKernel(k)

	// Test that the cache was populated
	vs.mu.RLock()
	cache := vs.permittedCache
	vs.mu.RUnlock()

	if cache == nil {
		t.Fatalf("Expected permission cache to be populated")
	}

	// Cache membership alone must deny.
	testCases := []struct {
		action   string
		expected bool
	}{
		{"/read_file", false},
		{"read_file", false},
		{"/write_file", false},
		{"write_file", false},
		{"/review", false},
		{"review", false},
		{"/exec_cmd", false},
		{"exec_cmd", false},
		{"/delete_all", false},
	}

	for _, tc := range testCases {
		result := vs.CheckKernelPermitted(tc.action, "test_target", map[string]any{})
		if result != tc.expected {
			t.Errorf("CheckKernelPermitted(%q) = %v, expected %v", tc.action, result, tc.expected)
		}
	}

	// The same action is allowed only after an exact permitted/3 result exists.
	k.permitted = []Fact{{
		Predicate: "permitted",
		Args:      []any{MangleAtom("/read_file"), "test_target", "{}"},
	}}
	if !vs.CheckKernelPermitted("read_file", "test_target", map[string]any{}) {
		t.Fatal("expected exact permitted/3 fact to authorize request")
	}
	if vs.CheckKernelPermitted("read_file", "other_target", map[string]any{}) {
		t.Fatal("permitted/3 fact for another target must not authorize request")
	}
	if vs.CheckKernelPermitted("read_file", "test_target", map[string]any{"mode": "other"}) {
		t.Fatal("permitted/3 fact for another payload must not authorize request")
	}

	t.Logf("Permission cache size: %d entries", len(cache))
}

func TestRouteActionReadFile_PersistsContentFacts(t *testing.T) {
	workspace := t.TempDir()
	filename := "sample.go"
	absPath := filepath.Join(workspace, filename)
	content := "// Package main\npackage main\n\nfunc main() {}\n"
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}

	cfg := DefaultVirtualStoreConfig()
	cfg.WorkingDir = workspace
	vs := NewVirtualStoreWithConfig(nil, cfg)
	vs.SetKernel(kernel)
	vs.DisableBootGuard()
	pending := Fact{
		Predicate: "pending_action",
		Args:      []any{"act_test", MangleAtom("/read_file"), filename, "{}", time.Now().Unix()},
	}
	if err := kernel.Assert(pending); err != nil {
		t.Fatalf("assert pending_action: %v", err)
	}
	defer func() { _ = kernel.RetractFact(pending) }()

	out, err := vs.RouteAction(context.Background(), Fact{
		Predicate: "next_action",
		Args:      []any{"act_test", "/read_file", filename},
	})
	if err != nil {
		t.Fatalf("RouteAction(read_file) error: %v", err)
	}
	// read_file returns the read projection; file_content below is what keeps
	// the raw bytes. Both matter and they are not the same thing: shrinking the
	// fact to the projection would change what has_file_content means in
	// coder_workflow.mg, and that is checked immediately after.
	for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		if !strings.Contains(out, line) {
			t.Fatalf("read projection dropped %q from a four-line file:\n%s", line, out)
		}
	}

	fileFacts, err := kernel.Query("file_content")
	if err != nil {
		t.Fatalf("Query(file_content) error: %v", err)
	}
	foundContent := false
	for _, f := range fileFacts {
		if len(f.Args) < 2 {
			continue
		}
		p, _ := f.Args[0].(string)
		c, _ := f.Args[1].(string)
		if p == absPath {
			if !strings.HasPrefix(c, "// Package") {
				t.Fatalf("file_content content not preserved; got prefix=%q", c[:min(len(c), 16)])
			}
			foundContent = true
			break
		}
	}
	if !foundContent {
		t.Fatalf("expected file_content fact for %s", absPath)
	}

	execFacts, err := kernel.Query("execution_result")
	if err != nil {
		t.Fatalf("Query(execution_result) error: %v", err)
	}
	foundExec := false
	for _, f := range execFacts {
		if len(f.Args) < 6 {
			continue
		}
		actionID := f.Args[0]
		actionType := f.Args[1]
		target := f.Args[2]
		success := f.Args[3]

		if actionID == "act_test" && actionType == "read_file" && target == filename {
			if success != "/true" {
				t.Fatalf("execution_result success=%v, want /true", success)
			}
			foundExec = true
			break
		}
	}
	if !foundExec {
		t.Fatalf("expected execution_result for act_test read_file %s", filename)
	}
}

// min is declared in validator_paranoid.go, removing redeclaration

// -----------------------------------------------------------------------------
// Boundary Value Analysis: Identified Gaps (Vector A: Null/Undefined/Empty)
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Boundary Value Analysis: Identified Gaps (Vector B: Type Coercion)
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Boundary Value Analysis: Identified Gaps (Vector C: User Extremes)
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Boundary Value Analysis: Identified Gaps (Vector D: State Conflicts & Concurrency)
// -----------------------------------------------------------------------------

// =============================================================================
// PRE-CHAOS HARDENING TESTS (Phase 4)
// =============================================================================

// createTestVirtualStore creates a minimal VirtualStore for constitution/env tests.
func createTestVirtualStore(t *testing.T) *VirtualStore {
	t.Helper()
	cfg := DefaultVirtualStoreConfig()
	vs := NewVirtualStoreWithConfig(nil, cfg)
	return vs
}

func TestConstitution_PathTraversal_EditFile(t *testing.T) {
	vs := createTestVirtualStore(t)
	req := ActionRequest{
		Type:   ActionEditFile,
		Target: "../../etc/passwd",
	}
	err := vs.checkConstitution(req)
	if err == nil {
		t.Error("path traversal via ActionEditFile should be blocked")
	}
}

func TestConstitution_PathTraversal_CleanPath(t *testing.T) {
	vs := createTestVirtualStore(t)
	// filepath.Clean normalizes this
	req := ActionRequest{
		Type:   ActionReadFile,
		Target: "foo/bar/../../../etc/passwd",
	}
	err := vs.checkConstitution(req)
	if err == nil {
		t.Error("normalized path traversal should be blocked")
	}
}

func TestConstitution_SystemPath_CaseInsensitive(t *testing.T) {
	vs := createTestVirtualStore(t)
	tests := []struct {
		name   string
		target string
	}{
		{"lowercase", "c:/windows/system32/config"},
		{"uppercase", "C:/WINDOWS/System32/Config"},
		{"mixed", "C:/WiNdOwS/system32"},
		{"backslash", "C:\\Windows\\System32"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ActionRequest{
				Type:   ActionWriteFile,
				Target: tt.target,
			}
			err := vs.checkConstitution(req)
			if err == nil {
				t.Errorf("system path %q should be blocked", tt.target)
			}
		})
	}
}

func TestFilterCallerEnv_AllowedOnly(t *testing.T) {
	vs := createTestVirtualStore(t)
	// Set a known allowlist
	vs.allowedEnvVars = []string{"HOME", "PATH"}

	env := []string{
		"HOME=/home/user",
		"PATH=/usr/bin",
		"LD_PRELOAD=/evil.so",
		"MALICIOUS=true",
	}
	filtered := vs.filterCallerEnv(env)

	for _, e := range filtered {
		key := strings.SplitN(e, "=", 2)[0]
		if strings.ToUpper(key) != "HOME" && strings.ToUpper(key) != "PATH" {
			t.Errorf("non-allowlisted env var %q should be filtered", key)
		}
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 filtered env vars, got %d", len(filtered))
	}
}

func TestFilterCallerEnv_CaseInsensitive(t *testing.T) {
	vs := createTestVirtualStore(t)
	vs.allowedEnvVars = []string{"PATH"}

	env := []string{"path=/usr/bin", "Path=/usr/local/bin"}
	filtered := vs.filterCallerEnv(env)
	if len(filtered) != 2 {
		t.Errorf("case-insensitive match should allow both, got %d", len(filtered))
	}
}

func TestFilterCallerEnv_Empty(t *testing.T) {
	vs := createTestVirtualStore(t)
	filtered := vs.filterCallerEnv(nil)
	if filtered != nil {
		t.Error("nil input should return nil")
	}
	filtered = vs.filterCallerEnv([]string{})
	if filtered != nil {
		t.Error("empty input should return nil")
	}
}

func (s *stubKernel) GetProgramInfo() *analysis.ProgramInfo { return nil }
