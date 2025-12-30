package core

import (
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
	asserted  []Fact
}

func (s *stubKernel) LoadFacts([]Fact) error { return nil }
func (s *stubKernel) Query(predicate string) ([]Fact, error) {
	if predicate == "permitted" {
		return s.permitted, nil
	}
	return nil, nil
}
func (s *stubKernel) QueryAll() (map[string][]Fact, error) { return nil, nil }
func (s *stubKernel) Assert(f Fact) error                  { s.asserted = append(s.asserted, f); return nil }
func (s *stubKernel) Retract(string) error                 { return nil }
func (s *stubKernel) RetractFact(Fact) error               { return nil }
func (s *stubKernel) UpdateSystemFacts() error             { return nil }
func (s *stubKernel) Reset()                                                   {}
func (s *stubKernel) AppendPolicy(string)                                      {}
func (s *stubKernel) RemoveFactsByPredicateSet(map[string]struct{}) error      { return nil }
func (s *stubKernel) RetractExactFactsBatch([]Fact) error                      { return nil }

func TestRouteActionBlockedWhenNotPermitted(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	k := &stubKernel{
		permitted: []Fact{
			{Predicate: "permitted", Args: []interface{}{"/read_file"}},
		},
	}
	vs.SetKernel(k)

	// exec_cmd should be blocked because kernel has no permitted(/exec_cmd)
	_, err := vs.RouteAction(context.Background(), Fact{
		Predicate: "next_action",
		Args:      []interface{}{"act_1", "/exec_cmd", "echo hi"},
	})
	if err == nil {
		t.Fatalf("expected exec_cmd to be blocked by kernel permission gate")
	}
}

func TestExecCmdDisallowedBinary(t *testing.T) {
	cfg := DefaultVirtualStoreConfig()
	cfg.AllowedBinaries = []string{"allowed"}
	vs := NewVirtualStoreWithConfig(nil, cfg)

	res, err := vs.handleExecCmd(context.Background(), ActionRequest{
		Type:   ActionExecCmd,
		Target: "echo hi",
		Payload: map[string]interface{}{
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
		Payload: map[string]interface{}{
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
		Payload: map[string]interface{}{},
	}
	got := commandFromActionRequest(req, "go test ./...")
	if got != "go test ./internal/core/... -count=1" {
		t.Fatalf("unexpected command: %q", got)
	}
}

func TestTimeoutSecondsFromActionRequest_DefaultAndOverrides(t *testing.T) {
	req := ActionRequest{Payload: map[string]interface{}{}}
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

func TestHydrateLearningsPreservesArgs(t *testing.T) {
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

	if err := db.StoreFact("pref_pred", []interface{}{"a", "b"}, "preference", 10); err != nil {
		t.Fatalf("failed to store fact: %v", err)
	}

	k := &stubKernel{}
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.SetLocalDB(db)
	vs.SetKernel(k)

	if _, err := vs.HydrateLearnings(context.Background()); err != nil {
		t.Fatalf("hydrate learnings failed: %v", err)
	}

	if len(k.asserted) == 0 {
		t.Fatalf("expected assertions into kernel")
	}

	found := false
	for _, f := range k.asserted {
		if f.Predicate == "learned_preference" {
			if len(f.Args) != 2 {
				t.Fatalf("expected learned_preference to have 2 args, got %d", len(f.Args))
			}
			if _, ok := f.Args[1].([]interface{}); !ok {
				t.Fatalf("expected second arg to be []interface{}, got %T", f.Args[1])
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("learned_preference assertion not found")
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

func (s *stubShard) GetID() string                    { return s.id }
func (s *stubShard) GetState() types.ShardState       { return s.state }
func (s *stubShard) GetConfig() types.ShardConfig     { return s.config }
func (s *stubShard) Stop() error                      { return nil }
func (s *stubShard) SetParentKernel(k types.Kernel)   {}
func (s *stubShard) SetLLMClient(client types.LLMClient) {}
func (s *stubShard) SetSessionContext(ctx *types.SessionContext) {}

// TestPermissionCacheOptimization verifies O(1) permission lookups via the cache.
func TestPermissionCacheOptimization(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())

	// Set up a kernel with multiple permitted actions
	k := &stubKernel{
		permitted: []Fact{
			{Predicate: "permitted", Args: []interface{}{"/read_file"}},
			{Predicate: "permitted", Args: []interface{}{"/write_file"}},
			{Predicate: "permitted", Args: []interface{}{"/review"}},
			{Predicate: "permitted", Args: []interface{}{"/run_tests"}},
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

	// Test O(1) lookups - both with and without leading slash
	testCases := []struct {
		action   string
		expected bool
	}{
		{"/read_file", true},
		{"read_file", true},
		{"/write_file", true},
		{"write_file", true},
		{"/review", true},
		{"review", true},
		{"/exec_cmd", false},
		{"exec_cmd", false},
		{"/delete_all", false},
	}

	for _, tc := range testCases {
		result := vs.CheckKernelPermitted(tc.action, "test_target", map[string]interface{}{})
		if result != tc.expected {
			t.Errorf("CheckKernelPermitted(%q) = %v, expected %v", tc.action, result, tc.expected)
		}
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

	out, err := vs.RouteAction(context.Background(), Fact{
		Predicate: "next_action",
		Args:      []interface{}{"act_test", "/read_file", filename},
	})
	if err != nil {
		t.Fatalf("RouteAction(read_file) error: %v", err)
	}
	if out != content {
		t.Fatalf("unexpected output content; got len=%d want len=%d", len(out), len(content))
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
