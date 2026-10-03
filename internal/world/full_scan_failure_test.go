package world

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func resetFullScanManifest(testingT *testing.T, fixture incrementalFailureFixture, state string) []byte {
	testingT.Helper()
	if state == "absent" {
		if err := os.Remove(fixture.manifestPath); err != nil {
			testingT.Fatal(err)
		}
		return nil
	}
	contents := map[string][]byte{
		"empty":   []byte(" \n{}\n "),
		"invalid": []byte("{invalid-manifest\n"),
		"null":    []byte("null\n"),
	}[state]
	if contents == nil {
		testingT.Fatalf("unknown manifest state %q", state)
	}
	if err := os.WriteFile(fixture.manifestPath, contents, 0o644); err != nil {
		testingT.Fatal(err)
	}
	return contents
}

func assertFullScanGenerationUnchanged(testingT *testing.T, fixture incrementalFailureFixture, databaseBefore, manifestBefore []byte) {
	testingT.Helper()
	if !bytes.Equal(databaseBefore, incrementalWorldGeneration(testingT, fixture.database)) {
		testingT.Fatal("failed fallback changed persisted world rows")
	}
	if manifestBefore != nil {
		assertIncrementalGenerationUnchanged(testingT, fixture, databaseBefore, manifestBefore)
		return
	}
	if _, err := os.Stat(fixture.manifestPath); !errors.Is(err, fs.ErrNotExist) {
		testingT.Fatalf("failed fallback published an absent manifest: %v", err)
	}
}

func awaitFullScanSignal(testingT *testing.T, signal <-chan struct{}, name string) {
	testingT.Helper()
	select {
	case <-signal:
	case <-time.After(30 * time.Second):
		testingT.Fatalf("timed out awaiting %s", name)
	}
}

type fallbackScanOutcome struct {
	result *IncrementalResult
	err    error
}

func TestIncrementalScan_FallbackFailurePreservesGeneration(testingT *testing.T) {
	for _, state := range []string{"empty", "absent", "invalid", "null"} {
		for _, mode := range []string{"walk_callback", "walk_return", "disappearance", "during_walk", "at_walk_return", "during_join"} {
			testingT.Run(state+"/"+mode, func(testingT *testing.T) {
				fixture := newIncrementalFailureFixture(testingT)
				manifestBefore := resetFullScanManifest(testingT, fixture, state)
				databaseBefore := incrementalWorldGeneration(testingT, fixture.database)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				failure := errors.New("second census failure")
				expected := failure
				if mode == "disappearance" {
					expected = fs.ErrNotExist
				} else if mode == "during_walk" || mode == "at_walk_return" || mode == "during_join" {
					expected = context.Canceled
				}
				workerReady := make(chan struct{})
				workerExited := make(chan struct{})
				censusDone := make(chan struct{})
				releaseWorker := make(chan struct{})
				var releaseOnce sync.Once
				release := func() { releaseOnce.Do(func() { close(releaseWorker) }) }
				finished := make(chan fallbackScanOutcome, 1)
				scanDone := make(chan struct{})
				testingT.Cleanup(func() {
					cancel()
					release()
					awaitFullScanSignal(testingT, scanDone, "scan cleanup")
					select {
					case <-workerReady:
						awaitFullScanSignal(testingT, workerExited, "worker cleanup")
					default:
					}
				})
				firstCensusDone := false
				secondCensus := false
				sawDisappearance := false
				movedDirectory := filepath.Join(testingT.TempDir(), "moved")
				options := IncrementalOptions{}
				options.walkDir = func(root string, visit fs.WalkDirFunc) error {
					walkErr := filepath.WalkDir(root, visit)
					firstCensusDone = walkErr == nil
					return walkErr
				}
				options.fullScan.runFile = func(path string, process func()) {
					process()
					if path == filepath.Join(fixture.root, "alpha.txt") {
						defer close(workerExited)
						close(workerReady)
						<-releaseWorker
					}
				}
				options.fullScan.walk = func(root string, visit filepath.WalkFunc) error {
					defer close(censusDone)
					secondCensus = true
					walkErr := filepath.Walk(root, func(path string, info os.FileInfo, pathErr error) error {
						sawDisappearance = sawDisappearance || errors.Is(pathErr, fs.ErrNotExist)
						if mode == "walk_callback" && path == filepath.Join(root, "nested", "keep.txt") {
							return visit(path, info, fmt.Errorf("walk metadata: %w", failure))
						}
						visitErr := visit(path, info, pathErr)
						if visitErr == nil && path == filepath.Join(root, "alpha.txt") {
							select {
							case <-workerReady:
							case <-ctx.Done():
								return ctx.Err()
							}
							if mode == "during_walk" {
								cancel()
							} else if mode == "disappearance" {
								return os.Rename(filepath.Join(root, "nested"), movedDirectory)
							}
						}
						return visitErr
					})
					if mode == "walk_return" && walkErr == nil {
						return fmt.Errorf("walk completion: %w", failure)
					}
					if mode == "at_walk_return" {
						cancel()
					}
					return walkErr
				}
				go func() {
					defer close(scanDone)
					result, err := fixture.scanner.ScanWorkspaceIncremental(ctx, fixture.root, fixture.database, options)
					finished <- fallbackScanOutcome{result: result, err: err}
				}()
				awaitFullScanSignal(testingT, censusDone, "second census")
				if mode == "during_join" {
					cancel()
				}
				select {
				case outcome := <-finished:
					testingT.Fatalf("scan returned before its admitted worker joined: %+v", outcome)
				case <-time.After(100 * time.Millisecond):
				}
				assertFullScanGenerationUnchanged(testingT, fixture, databaseBefore, manifestBefore)
				release()
				awaitFullScanSignal(testingT, scanDone, "joined scan")
				outcome := <-finished
				select {
				case <-workerExited:
				default:
					testingT.Fatal("admitted worker remains active after return")
				}
				if !firstCensusDone || !secondCensus || (mode == "disappearance" && !sawDisappearance) {
					testingT.Fatalf("missing census witness: first=%v second=%v disappearance=%v", firstCensusDone, secondCensus, sawDisappearance)
				}
				if !errors.Is(outcome.err, expected) || outcome.result != nil {
					testingT.Fatalf("expected original failure and nil result, got %+v", outcome)
				}
				assertFullScanGenerationUnchanged(testingT, fixture, databaseBefore, manifestBefore)
			})
		}
	}
}

func TestIncrementalScan_FallbackSuccessPublishesGeneration(testingT *testing.T) {
	for _, state := range []string{"empty", "absent", "invalid", "null"} {
		testingT.Run(state, func(testingT *testing.T) {
			fixture := newIncrementalFailureFixture(testingT)
			resetFullScanManifest(testingT, fixture, state)
			result, err := fixture.scanner.ScanWorkspaceIncremental(context.Background(), fixture.root, fixture.database, IncrementalOptions{})
			if err != nil || result == nil || !result.Full || result.FileCount != 2 || len(result.NewFacts) == 0 {
				testingT.Fatalf("first-run fallback did not publish a full result: %+v, %v", result, err)
			}
			paths, err := fixture.database.ListWorldFilePaths()
			if err != nil {
				testingT.Fatal(err)
			}
			for _, path := range paths {
				if isNonCanonicalWorldPath(path) {
					testingT.Fatalf("successful fallback retained legacy path %q", path)
				}
			}
			manifest, err := os.ReadFile(fixture.manifestPath)
			if err != nil {
				testingT.Fatal(err)
			}
			var entries map[string]CacheEntry
			if err := json.Unmarshal(manifest, &entries); err != nil || len(entries) != 2 {
				testingT.Fatalf("successful fallback did not publish two cache entries: %v, %v", entries, err)
			}
			for _, relative := range []string{"alpha.txt", "nested/keep.txt"} {
				entry, exists := entries[filepath.Join(fixture.root, filepath.FromSlash(relative))]
				facts, _, err := fixture.database.LoadWorldFactsForFile(relative, "fast")
				if err != nil {
					testingT.Fatalf("load persisted facts for %s: %v", relative, err)
				}
				var storedHash string
				topologyCount := 0
				for _, fact := range facts {
					if fact.Predicate != "file_topology" {
						continue
					}
					topologyCount++
					if len(fact.Args) != 5 {
						testingT.Fatalf("malformed persisted topology for %s: %+v", relative, fact)
					}
					storedPath, pathOK := fact.Args[0].(string)
					contentHash, hashOK := fact.Args[1].(string)
					if !pathOK || storedPath != relative || !hashOK || contentHash == "" {
						testingT.Fatalf("invalid persisted path/hash for %s: %+v", relative, fact)
					}
					storedHash = contentHash
				}
				if !exists || entry.Hash == "" || topologyCount != 1 || storedHash != entry.Hash {
					testingT.Fatalf("cache and persisted facts disagree for %s: cache=%+v topologyCount=%d storedHash=%q", relative, entry, topologyCount, storedHash)
				}
			}
		})
	}
}

func TestIncrementalScan_FallbackWorkerFailurePreservesGeneration(testingT *testing.T) {
	for _, state := range []string{"empty", "absent", "invalid", "null"} {
		testingT.Run(state, func(testingT *testing.T) {
			fixture := newIncrementalFailureFixture(testingT)
			manifestBefore := resetFullScanManifest(testingT, fixture, state)
			databaseBefore := incrementalWorldGeneration(testingT, fixture.database)
			var removeErr error
			injected := false
			options := IncrementalOptions{}
			options.fullScan.runFile = func(path string, process func()) {
				if path == filepath.Join(fixture.root, "alpha.txt") {
					injected = true
					removeErr = os.Remove(path)
				}
				process()
			}
			result, err := fixture.scanner.ScanWorkspaceIncremental(context.Background(), fixture.root, fixture.database, options)
			if !injected || removeErr != nil || !errors.Is(err, fs.ErrNotExist) || result != nil {
				testingT.Fatalf("missing admitted hash failure: injected=%v removal=%v result=%+v err=%v", injected, removeErr, result, err)
			}
			assertFullScanGenerationUnchanged(testingT, fixture, databaseBefore, manifestBefore)
		})
	}
}

func TestIncrementalScan_FallbackMembershipFailurePreservesGeneration(testingT *testing.T) {
	for _, state := range []string{"empty", "absent", "invalid", "null"} {
		testingT.Run(state, func(testingT *testing.T) {
			fixture := newIncrementalFailureFixture(testingT)
			manifestBefore := resetFullScanManifest(testingT, fixture, state)
			databaseBefore := incrementalWorldGeneration(testingT, fixture.database)
			for _, relative := range []string{".git/objects", ".git/refs"} {
				if err := os.MkdirAll(filepath.Join(fixture.root, filepath.FromSlash(relative)), 0o755); err != nil {
					testingT.Fatal(err)
				}
			}
			for relative, content := range map[string]string{
				".git/HEAD":   "ref: refs/heads/main\n",
				".git/config": "[core]\n\trepositoryformatversion = 0\n\tbare = false\n",
			} {
				if err := os.WriteFile(filepath.Join(fixture.root, filepath.FromSlash(relative)), []byte(content), 0o644); err != nil {
					testingT.Fatal(err)
				}
			}
			scanner := NewScannerWithConfig(ScannerConfig{MaxConcurrency: 2, IgnorePatterns: []string{"ignored", "membership-failure-witness"}})
			firstCensusDone := false
			fullWalkCalled := false
			options := IncrementalOptions{}
			options.walkDir = func(root string, visit fs.WalkDirFunc) error {
				if err := filepath.WalkDir(root, visit); err != nil {
					return err
				}
				firstCensusDone = true
				return os.WriteFile(filepath.Join(root, ".git", "index"), []byte("invalid second-census index\n"), 0o644)
			}
			options.fullScan.walk = func(root string, visit filepath.WalkFunc) error {
				fullWalkCalled = true
				return filepath.Walk(root, visit)
			}
			result, err := scanner.ScanWorkspaceIncremental(context.Background(), fixture.root, fixture.database, options)
			if !firstCensusDone || fullWalkCalled || err == nil || !strings.Contains(err.Error(), "git ls-files") || result != nil {
				testingT.Fatalf("missing second membership failure: first=%v walk=%v result=%+v err=%v", firstCensusDone, fullWalkCalled, result, err)
			}
			assertFullScanGenerationUnchanged(testingT, fixture, databaseBefore, manifestBefore)
		})
	}
}

func TestFullScan_PublicFailurePreservesManifest(testingT *testing.T) {
	for _, entry := range []string{"directory", "workspace"} {
		for _, state := range []string{"empty", "absent", "invalid", "null"} {
			for _, mode := range []string{"canceled", "read_error"} {
				testingT.Run(entry+"/"+state+"/"+mode, func(testingT *testing.T) {
					fixture := newIncrementalFailureFixture(testingT)
					manifestBefore := resetFullScanManifest(testingT, fixture, state)
					databaseBefore := incrementalWorldGeneration(testingT, fixture.database)
					if err := os.Remove(filepath.Join(fixture.root, "nested", "keep.txt")); err != nil {
						testingT.Fatal(err)
					}
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					scanner := NewScannerWithConfig(ScannerConfig{MaxConcurrency: 1, IgnorePatterns: []string{"ignored"}})
					var removeErr error
					scanner.parserPool.New = func() any {
						if mode == "canceled" {
							cancel()
						} else {
							removeErr = os.Remove(filepath.Join(fixture.root, "alpha.txt"))
						}
						return NewTreeSitterParser()
					}
					var scanErr error
					if entry == "directory" {
						result, err := scanner.ScanDirectory(ctx, fixture.root)
						scanErr = err
						if result != nil {
							testingT.Fatalf("failed directory scan returned partial facts: %+v", result)
						}
					} else {
						facts, err := scanner.ScanWorkspaceCtx(ctx, fixture.root)
						scanErr = err
						if facts != nil {
							testingT.Fatalf("failed workspace scan returned partial facts: %+v", facts)
						}
					}
					expected := error(context.Canceled)
					if mode == "read_error" {
						expected = fs.ErrNotExist
					}
					if removeErr != nil || !errors.Is(scanErr, expected) {
						testingT.Fatalf("missing public failure witness: %v, %v", removeErr, scanErr)
					}
					assertFullScanGenerationUnchanged(testingT, fixture, databaseBefore, manifestBefore)
				})
			}
		}
	}
}
