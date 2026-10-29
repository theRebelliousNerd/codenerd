package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSameTurnCreatedSource_AllowsDeleteFact(t *testing.T) {
	e, _ := newBuildableTreeWorkspace(t)
	e.turnCreatedSources = []string{"scratch_same_turn.go"}
	fact, ok := e.recoverableDeleteFact(context.Background(), "/delete_file", "scratch_same_turn.go")
	if !ok {
		t.Fatalf("same-turn created source was not recoverable: want ok=true, got false")
	}
	if string(fact.Predicate) != deleteRecoverablePredicate {
		t.Fatalf("want predicate %q, got %q", deleteRecoverablePredicate, fact.Predicate)
	}
}

func TestSameTurnCreatedTest_AllowsDeleteFact(t *testing.T) {
	e, _ := newBuildableTreeWorkspace(t)
	e.turnCreatedTests = [][2]string{{"repair_run_test.go", "repair.go"}}
	_, ok := e.recoverableDeleteFact(context.Background(), "/delete_file", "repair_run_test.go")
	if !ok {
		t.Fatalf("same-turn created test was not recoverable: want ok=true, got false")
	}
}

func TestPreExistingFile_StillRefused(t *testing.T) {
	e, workspace := newBuildableTreeWorkspace(t)
	pre := filepath.Join(workspace, "pre_existing.go")
	if err := os.WriteFile(pre, []byte("package p\n"), 0o644); err != nil {
		t.Fatalf("write pre-existing file: %v", err)
	}
	_, ok := e.recoverableDeleteFact(context.Background(), "/delete_file", "pre_existing.go")
	if ok {
		t.Fatalf("pre-existing file was recoverable: want ok=false, got true")
	}
}

func TestRecoverableDeleteFact_GitTrackedClean_AllowsDelete(t *testing.T) {
	e, _ := newBuildableTreeWorkspace(t)
	orig := gitRecoverableRunner
	t.Cleanup(func() { gitRecoverableRunner = orig })
	gitRecoverableRunner = func(ctx context.Context, dir string, args []string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ls-files" {
			return []byte("pre_existing.go\n"), nil
		}
		if len(args) > 0 && args[0] == "status" {
			return []byte(""), nil
		}
		return nil, nil
	}
	_, ok := e.recoverableDeleteFact(context.Background(), "/delete_file", "pre_existing.go")
	if !ok {
		t.Fatalf("git-tracked clean file was not recoverable: want ok=true, got false")
	}
}

func TestSameTurnCreatedFile_EmptyTarget_IsNotSameTurn(t *testing.T) {
	e, _ := newBuildableTreeWorkspace(t)
	if e.sameTurnCreatedFile("") {
		t.Fatalf("empty target was treated as same-turn created: want false, got true")
	}
	if e.sameTurnCreatedFile("   ") {
		t.Fatalf("blank target was treated as same-turn created: want false, got true")
	}
}

func TestRecoverableDeleteFact_EmptyTarget_StillRefused(t *testing.T) {
	e, _ := newBuildableTreeWorkspace(t)
	_, ok := e.recoverableDeleteFact(context.Background(), "/delete_file", "")
	if ok {
		t.Fatalf("empty delete target was recoverable: want ok=false, got true")
	}
}
