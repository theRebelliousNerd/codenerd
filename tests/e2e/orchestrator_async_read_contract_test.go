//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
)

func TestAsyncResearchLegacyRefusal(t *testing.T) {
	// The legacy helper does not replace a production handler already owned
	// by another fixture. Its write path and executor workspace also differ.
	owned := tools.NewRegistry()
	write := toolscore.WriteFileTool()
	write.Effect = tools.EffectWrite
	if err := owned.Register(write); err != nil {
		t.Fatal(err)
	}
	owned.SetWorkspaceRoot(t.TempDir())
	owned.SetAllowlist(&tools.Allowlist{Enforced: true, Names: []string{"write_file"}})
	restore := tools.SwapGlobal(owned)
	t.Cleanup(func() {
		if tools.Global() == owned {
			restore()
		} else {
			t.Error("legacy diagnostic lost global ownership")
		}
	})
	executor, _ := setupRaceEnvironment(t, time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result, err := executor.ProcessWithIntent(ctx, "diagnose legacy research carrier", &perception.Intent{Verb: "/research"})
	if result == nil || errors.Join(err, result.Error) == nil || result.TurnOutcome == "/done" || len(result.ToolErrors) == 0 || result.WriteToolEffects != 0 {
		t.Fatalf("legacy write fixture did not reproduce actual refusal: result=%+v err=%v", result, err)
	}
	t.Logf("legacy actual tool errors=%v; outcome=%s; successful=%d writes=%d", result.ToolErrors, result.TurnOutcome, result.SuccessfulToolCalls, result.WriteToolEffects)
}

func TestAsyncResearchRegistryAndCacheControls(t *testing.T) {
	for _, mode := range []string{"owned_cache", "denied_allowlist", "missing_file", "canceled_before_spawn"} {
		t.Run(mode, func(t *testing.T) {
			f := newAsyncReadFixture(t, time.Millisecond)
			// A competing owner can perform the same legitimate read. If the
			// child loses its explicit binding, correct content alone cannot
			// disguise that it ran through the wrong registry.
			global := tools.NewRegistry()
			global.SetWorkspaceRoot(f.root)
			global.SetAllowlist(&tools.Allowlist{Enforced: true, Names: []string{"read_file"}})
			var globalReads atomic.Int32
			read := toolscore.ReadFileTool()
			read.Effect = tools.EffectRead
			execute := read.Execute
			read.Execute = func(ctx context.Context, args map[string]any) (string, error) {
				globalReads.Add(1)
				return execute(ctx, args)
			}
			if err := global.Register(read); err != nil {
				t.Fatal(err)
			}
			restore := tools.SwapGlobal(global)
			t.Cleanup(func() {
				if globalReads.Load() != 0 {
					t.Errorf("isolated child used the competing global owner %d times", globalReads.Load())
				}
				if tools.Global() == global {
					restore()
				} else {
					t.Error("competing registry owner changed")
				}
			})
			if mode == "denied_allowlist" {
				f.registry.SetAllowlist(&tools.Allowlist{Enforced: true, Names: []string{"list_files"}})
			}
			if mode == "missing_file" {
				f.model.path = f.path + ".missing"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if mode == "canceled_before_spawn" {
				cancel()
			}
			id, err := f.jit.ExecuteAsync(ctx, session.TaskRequest{IntentVerb: "/research", Task: "read the observation", Target: "observation.txt"})
			if mode == "canceled_before_spawn" {
				if !errors.Is(err, context.Canceled) || id != "" || f.reads.Load() != 0 || globalReads.Load() != 0 {
					t.Fatalf("pre-canceled task dispatched: id=%q err=%v own=%d global=%d", id, err, f.reads.Load(), globalReads.Load())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			answer, waitErr := f.jit.WaitForResult(ctx, id)
			if mode == "owned_cache" {
				if waitErr != nil || !strings.Contains(answer, f.marker) || f.reads.Load() != 1 {
					t.Fatalf("no real read: answer=%q err=%v reads=%d", answer, waitErr, f.reads.Load())
				}
				f.model.mu.Lock()
				calls := f.model.calls
				f.model.mu.Unlock()
				// A deliberate change AFTER success proves GetResult is a cache,
				// not another read. The result remains a historical observation.
				if err := os.WriteFile(f.path, []byte("later observation\n"), 0600); err != nil {
					t.Fatal(err)
				}
				var wg sync.WaitGroup
				for range 16 {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for range 4 {
							got, done, err := f.jit.GetResult(id)
							if err != nil || !done || got != answer {
								t.Errorf("cached result changed: %q %v %v", got, done, err)
							}
						}
					}()
				}
				wg.Wait()
				f.model.mu.Lock()
				afterCalls := f.model.calls
				f.model.mu.Unlock()
				if afterCalls != calls || f.reads.Load() != 1 {
					t.Error("cache replayed model/tool work")
				}
				if body, err := os.ReadFile(f.path); err != nil || string(body) != "later observation\n" {
					t.Errorf("cache changed source: %q %v", body, err)
				}
			} else {
				wantReads := int32(0)
				if mode == "missing_file" {
					wantReads = 1
				}
				if waitErr == nil || f.reads.Load() != wantReads {
					t.Fatalf("refused/failed read reported success: err=%v reads=%d want=%d", waitErr, f.reads.Load(), wantReads)
				}
				_, done, cachedErr := f.jit.GetResult(id)
				if !done || cachedErr == nil || cachedErr.Error() != waitErr.Error() {
					t.Fatalf("execution error was not cached: done=%v got=%v want=%v", done, cachedErr, waitErr)
				}
				if body, err := os.ReadFile(f.path); err != nil || string(body) != f.body {
					t.Errorf("failed read changed source: %q %v", body, err)
				}
			}
			if globalReads.Load() != 0 {
				t.Errorf("isolated child used another registry owner %d times", globalReads.Load())
			}
			if active := f.spawner.ListActive(); len(active) != 0 {
				t.Errorf("terminal task left active worker: %v", active)
			}
		})
	}
}

func TestAsyncResearchIndependentWorkspaces(t *testing.T) {
	first, second := newAsyncReadFixture(t, time.Millisecond), newAsyncReadFixture(t, time.Millisecond)
	global := tools.Global()
	globalRoot, globalRead := global.WorkspaceRoot(), global.Get("read_file")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for _, pair := range [][2]*asyncReadFixture{{first, second}, {second, first}} {
		wg.Add(1)
		go func(f, other *asyncReadFixture) {
			defer wg.Done()
			id, err := f.jit.ExecuteAsync(ctx, session.TaskRequest{IntentVerb: "/research", Task: "read owned observation", Target: "observation.txt"})
			if err != nil {
				t.Error(err)
				return
			}
			answer, err := f.jit.WaitForResult(ctx, id)
			if err != nil || !strings.Contains(answer, f.marker) || strings.Contains(answer, other.marker) || f.reads.Load() != 1 {
				t.Errorf("crossed read owners: answer=%q err=%v reads=%d", answer, err, f.reads.Load())
			}
		}(pair[0], pair[1])
	}
	wg.Wait()
	for _, f := range []*asyncReadFixture{first, second} {
		if body, err := os.ReadFile(f.path); err != nil || string(body) != f.body {
			t.Errorf("read changed owned source: %q %v", body, err)
		}
	}
	if tools.Global() != global || global.WorkspaceRoot() != globalRoot || global.Get("read_file") != globalRead {
		t.Error("independent sessions changed the global registry")
	}
}
