package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
)

// seedGoFile writes a minimal parseable Go file and returns its directory.
func seedGoFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := "package seed\n\n// SeedFunc is indexed.\nfunc SeedFunc() int { return 42 }\n"
	if err := os.WriteFile(filepath.Join(dir, "seed.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// runExecuteUntilFacts runs Execute in a goroutine until file_topology facts
// appear (or a deadline hits), then cancels and joins. It returns the facts
// observed. Execute blocks in its event loop by design, so the test must
// drive it asynchronously.
func runExecuteUntilFacts(t *testing.T, shard *WorldModelIngestorShard, kernel *core.RealKernel, task string) []core.Fact {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := shard.Execute(ctx, task)
		done <- err
	}()

	deadline := time.Now().Add(30 * time.Second)
	for {
		facts, err := kernel.Query("file_topology")
		if err != nil {
			cancel()
			<-done
			t.Fatalf("Query(file_topology): %v", err)
		}
		if len(facts) > 0 {
			cancel()
			<-done
			return facts
		}
		if time.Now().After(deadline) {
			cancel()
			execErr := <-done
			t.Fatalf("no file_topology facts within 30s (execute err: %v)", execErr)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// TestExecuteLifecycleLabelKeepsConfiguredRoot is the completion proof for
// on-demand world-model activation: a system-spawned ingestor (whose task is
// a lifecycle label, not a path) must scan its configured root and assert
// world facts — not silently scan a nonexistent directory.
func TestExecuteLifecycleLabelKeepsConfiguredRoot(t *testing.T) {
	dir := seedGoFile(t)
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}

	cfg := DefaultWorldModelConfig()
	cfg.RootPath = dir
	shard := NewWorldModelIngestorShardWithConfig(cfg)
	shard.SetParentKernel(kernel)

	facts := runExecuteUntilFacts(t, shard, kernel, "on_demand_activation")

	found := false
	for _, f := range facts {
		for _, arg := range f.Args {
			if s, ok := arg.(string); ok && strings.Contains(s, "seed.go") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("file_topology facts do not cover the configured root %s: %v", dir, facts)
	}
}

// TestExecuteDirectoryTaskOverridesRoot pins the ad-hoc affordance the guard
// preserves: a task naming a real directory still redirects the scan there.
func TestExecuteDirectoryTaskOverridesRoot(t *testing.T) {
	scanDir := seedGoFile(t)
	otherDir := t.TempDir() // configured root: empty, must contribute nothing
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}

	cfg := DefaultWorldModelConfig()
	cfg.RootPath = otherDir
	shard := NewWorldModelIngestorShardWithConfig(cfg)
	shard.SetParentKernel(kernel)

	facts := runExecuteUntilFacts(t, shard, kernel, scanDir)

	found := false
	for _, f := range facts {
		for _, arg := range f.Args {
			if s, ok := arg.(string); ok && strings.Contains(s, "seed.go") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("directory task %s was not scanned: %v", scanDir, facts)
	}
}
