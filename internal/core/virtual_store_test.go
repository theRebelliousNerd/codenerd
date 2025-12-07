package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/store"
)

type stubKernel struct {
	permitted []Fact
	asserted  []Fact
}

func (s *stubKernel) LoadFacts([]Fact) error                       { return nil }
func (s *stubKernel) Query(predicate string) ([]Fact, error) {
	if predicate == "permitted" {
		return s.permitted, nil
	}
	return nil, nil
}
func (s *stubKernel) QueryAll() (map[string][]Fact, error)         { return nil, nil }
func (s *stubKernel) Assert(f Fact) error                          { s.asserted = append(s.asserted, f); return nil }
func (s *stubKernel) Retract(string) error                         { return nil }
func (s *stubKernel) RetractFact(Fact) error                       { return nil }

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
		Args:      []interface{}{"/exec_cmd", "echo hi"},
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
	sm := NewShardManager()
	sm.results["id"] = ShardResult{ShardID: "id"}

	if _, ok := sm.GetResult("id"); !ok {
		t.Fatalf("expected result to be returned")
	}

	if _, ok := sm.GetResult("id"); ok {
		t.Fatalf("expected result to be cleaned up after retrieval")
	}
}
