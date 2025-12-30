package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/tactile"
)

func TestPendingActionPipelineProducesRoutingResult(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workdir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workdir, ".nerd", "logs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workdir, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	kernel, err := core.NewRealKernelWithWorkspace(workdir)
	if err != nil {
		t.Fatalf("NewRealKernelWithWorkspace() error = %v", err)
	}

	executor := tactile.NewDirectExecutor()
	vsCfg := core.DefaultVirtualStoreConfig()
	vsCfg.WorkingDir = workdir
	virtualStore := core.NewVirtualStoreWithConfig(executor, vsCfg)
	virtualStore.SetKernel(kernel)
	virtualStore.DisableBootGuard()

	constitution := NewConstitutionGateShard()
	constitution.Kernel = kernel

	router := NewTactileRouterShard()
	router.Kernel = kernel
	router.VirtualStore = virtualStore

	actionID := "action-test"
	if err := kernel.Assert(core.Fact{
		Predicate: "pending_action",
		Args:      []interface{}{actionID, "/read_file", "hello.txt", map[string]interface{}{}, time.Now().Unix()},
	}); err != nil {
		t.Fatalf("assert pending_action: %v", err)
	}

	if err := constitution.processPendingActions(ctx); err != nil {
		t.Fatalf("processPendingActions: %v", err)
	}

	perm, err := kernel.Query("permission_check_result")
	if err != nil {
		t.Fatalf("Query(permission_check_result) error = %v", err)
	}
	foundPermit := false
	for _, f := range perm {
		if len(f.Args) < 2 {
			continue
		}
		if id := f.Args[0]; id != actionID {
			continue
		}
		foundPermit = true
		status := fmt.Sprintf("%v", f.Args[1])
		if status != "/permit" {
			t.Fatalf("permission_check_result status = %v, want /permit", f.Args[1])
		}
		break
	}
	if !foundPermit {
		t.Fatalf("permission_check_result not found for %s (got %d total)", actionID, len(perm))
	}

	if err := router.processPermittedActions(ctx); err != nil {
		t.Fatalf("processPermittedActions: %v", err)
	}

	results, err := kernel.Query("routing_result")
	if err != nil {
		t.Fatalf("Query(routing_result) error = %v", err)
	}

	found := false
	for _, f := range results {
		if len(f.Args) < 2 {
			continue
		}
		if id := f.Args[0]; id != actionID {
			continue
		}
		found = true
		status := fmt.Sprintf("%v", f.Args[1])
		if status != "/success" {
			reason := "unknown"
			if len(f.Args) >= 3 {
				reason = fmt.Sprintf("%v", f.Args[2])
			}
			t.Fatalf("routing_result status = %v, want /success. Reason: %s", f.Args[1], reason)
		}
		if len(f.Args) < 3 {
			t.Fatalf("routing_result missing output details for %s", actionID)
		}
		break
	}
	if !found {
		t.Fatalf("routing_result not found for %s (got %d total)", actionID, len(results))
	}
}
