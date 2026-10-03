package world

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/store"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

type incrementalFailureFixture struct {
	root         string
	scanner      *Scanner
	database     *store.LocalStore
	manifestPath string
	legacyPath   string
}

func newIncrementalFailureFixture(testingT *testing.T) incrementalFailureFixture {
	testingT.Helper()
	root, err := tools.CanonicalWorkspaceRoot(testingT.TempDir())
	if err != nil {
		testingT.Fatal(err)
	}
	testingT.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	for relative, content := range map[string]string{
		".nerd/config.json":  "{}\n",
		"alpha.txt":          "alpha generation\n",
		"nested/keep.txt":    "keep generation\n",
		"ignored/hidden.txt": "excluded generation\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			testingT.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			testingT.Fatal(err)
		}
	}
	database, err := store.NewLocalStore(filepath.Join(testingT.TempDir(), "world.db"))
	if err != nil {
		testingT.Fatalf("open required SQLite fixture: %v", err)
	}
	testingT.Cleanup(func() {
		if err := database.Close(); err != nil {
			testingT.Errorf("close world database: %v", err)
		}
	})
	scanner := NewScannerWithConfig(ScannerConfig{
		MaxConcurrency: 2,
		IgnorePatterns: []string{"ignored"},
	})
	initial, err := scanner.ScanWorkspaceIncremental(context.Background(), root, database, IncrementalOptions{})
	if err != nil {
		testingT.Fatalf("seed through production scanner: %v", err)
	}
	if initial == nil || !initial.Full || initial.FileCount != 2 {
		testingT.Fatalf("expected a full census of two admitted files, got %+v", initial)
	}
	legacyPath := filepath.Join(root, "legacy.txt")
	if err := database.UpsertWorldFile(store.WorldFileMeta{
		Path: legacyPath, Lang: "unknown", Hash: "legacy-hash", Fingerprint: "legacy-generation",
	}); err != nil {
		testingT.Fatal(err)
	}
	if err := database.ReplaceWorldFactsForFile(legacyPath, "fast", "legacy-generation", []store.WorldFactInput{
		{Predicate: "file_topology", Args: []any{legacyPath, "legacy-hash", "/unknown", int64(42), "/false"}},
	}); err != nil {
		testingT.Fatal(err)
	}
	if err := database.ReplaceWorldFactsForFile("nested/keep.txt", "deep", "deep-generation", []store.WorldFactInput{
		{Predicate: "code_defines", Args: []any{"nested/keep.txt", "keep-symbol", types.MangleAtom("/function"), int64(1), int64(1)}},
	}); err != nil {
		testingT.Fatal(err)
	}
	fixture := incrementalFailureFixture{
		root: root, scanner: scanner, database: database, legacyPath: legacyPath,
		manifestPath: filepath.Join(root, ".nerd", "cache", "manifest.json"),
	}
	manifest, err := os.ReadFile(fixture.manifestPath)
	if err != nil {
		testingT.Fatalf("read seeded manifest: %v", err)
	}
	var entries map[string]CacheEntry
	if err := json.Unmarshal(manifest, &entries); err != nil {
		testingT.Fatal(err)
	}
	if len(entries) != 2 {
		testingT.Fatalf("expected two cached files, got %v", entries)
	}
	for _, relative := range []string{"alpha.txt", "nested/keep.txt"} {
		facts, _, err := database.LoadWorldFactsForFile(relative, "fast")
		if err != nil || len(facts) == 0 {
			testingT.Fatalf("seeded file %s has no persisted facts: %v", relative, err)
		}
	}
	return fixture
}

func incrementalWorldGeneration(testingT *testing.T, database *store.LocalStore) []byte {
	testingT.Helper()
	var tables [][][]any
	for _, query := range []string{
		"SELECT * FROM world_files ORDER BY path",
		"SELECT * FROM world_facts ORDER BY path, depth, predicate, args",
	} {
		rows, err := database.GetDB().Query(query)
		if err != nil {
			testingT.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			_ = rows.Close()
			testingT.Fatal(err)
		}
		var records [][]any
		for rows.Next() {
			values := make([]any, len(columns))
			destinations := make([]any, len(columns))
			for index := range values {
				destinations[index] = &values[index]
			}
			if err := rows.Scan(destinations...); err != nil {
				_ = rows.Close()
				testingT.Fatal(err)
			}
			for index, value := range values {
				if raw, ok := value.([]byte); ok {
					values[index] = string(raw)
				}
			}
			records = append(records, values)
		}
		rowsErr := rows.Err()
		closeErr := rows.Close()
		if rowsErr != nil || closeErr != nil {
			testingT.Fatalf("read persisted generation: %v, close: %v", rowsErr, closeErr)
		}
		tables = append(tables, records)
	}
	encoded, err := json.Marshal(tables)
	if err != nil {
		testingT.Fatal(err)
	}
	return encoded
}

func assertIncrementalGenerationUnchanged(testingT *testing.T, fixture incrementalFailureFixture, databaseBefore, manifestBefore []byte) {
	testingT.Helper()
	databaseAfter := incrementalWorldGeneration(testingT, fixture.database)
	if !bytes.Equal(databaseBefore, databaseAfter) {
		testingT.Fatalf("failed census changed persisted generation\nbefore: %s\nafter: %s", databaseBefore, databaseAfter)
	}
	manifestAfter, err := os.ReadFile(fixture.manifestPath)
	if err != nil {
		testingT.Fatal(err)
	}
	if !bytes.Equal(manifestBefore, manifestAfter) {
		testingT.Fatalf("failed census changed cache bytes\nbefore: %s\nafter: %s", manifestBefore, manifestAfter)
	}
}

type incrementalInfoFailure struct {
	fs.DirEntry
	failure error
}

func (entry incrementalInfoFailure) Info() (fs.FileInfo, error) {
	return nil, entry.failure
}

func TestIncrementalScan_FailedEnumerationPreservesGeneration(testingT *testing.T) {
	for _, mode := range []string{"walk_callback", "walk_return", "admitted_info"} {
		testingT.Run(mode, func(testingT *testing.T) {
			fixture := newIncrementalFailureFixture(testingT)
			databaseBefore := incrementalWorldGeneration(testingT, fixture.database)
			manifestBefore, err := os.ReadFile(fixture.manifestPath)
			if err != nil {
				testingT.Fatal(err)
			}
			failure := errors.New("injected incomplete census")
			admitted := 0
			injected := false
			options := IncrementalOptions{SkipWhenUnchanged: true}
			options.walkDir = func(root string, visit fs.WalkDirFunc) error {
				walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, pathErr error) error {
					if path == filepath.Join(root, "nested", "keep.txt") {
						injected = true
						switch mode {
						case "walk_callback":
							return visit(path, entry, failure)
						case "walk_return":
							return fs.SkipAll
						case "admitted_info":
							return visit(path, incrementalInfoFailure{DirEntry: entry, failure: failure}, pathErr)
						}
					}
					visitErr := visit(path, entry, pathErr)
					if visitErr == nil && path == filepath.Join(root, "alpha.txt") {
						admitted++
					}
					return visitErr
				})
				if mode == "walk_return" && walkErr == nil {
					return failure
				}
				return walkErr
			}
			result, scanErr := fixture.scanner.ScanWorkspaceIncremental(context.Background(), fixture.root, fixture.database, options)
			if !injected || admitted != 1 {
				testingT.Fatalf("failure must follow an admitted file: injected=%v admitted=%d", injected, admitted)
			}
			if !errors.Is(scanErr, failure) || result != nil {
				testingT.Fatalf("expected the actual census error and no result, got %+v, %v", result, scanErr)
			}
			assertIncrementalGenerationUnchanged(testingT, fixture, databaseBefore, manifestBefore)
		})
	}
}

func TestIncrementalScan_CanceledEnumerationPreservesGeneration(testingT *testing.T) {
	for _, mode := range []string{"before_scan", "during_walk", "at_walk_return"} {
		testingT.Run(mode, func(testingT *testing.T) {
			fixture := newIncrementalFailureFixture(testingT)
			databaseBefore := incrementalWorldGeneration(testingT, fixture.database)
			manifestBefore, err := os.ReadFile(fixture.manifestPath)
			if err != nil {
				testingT.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "before_scan" {
				cancel()
			}
			walkCalled := false
			admitted := 0
			walkReturned := false
			options := IncrementalOptions{SkipWhenUnchanged: true}
			options.walkDir = func(root string, visit fs.WalkDirFunc) error {
				walkCalled = true
				walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, pathErr error) error {
					visitErr := visit(path, entry, pathErr)
					if visitErr == nil && path == filepath.Join(root, "alpha.txt") {
						admitted++
						if mode == "during_walk" {
							cancel()
						}
					}
					return visitErr
				})
				if mode == "at_walk_return" {
					cancel()
				}
				walkReturned = true
				return walkErr
			}
			result, scanErr := fixture.scanner.ScanWorkspaceIncremental(ctx, fixture.root, fixture.database, options)
			if !errors.Is(scanErr, context.Canceled) || result != nil {
				testingT.Fatalf("expected cancellation and no result, got %+v, %v", result, scanErr)
			}
			if mode == "before_scan" {
				if walkCalled {
					testingT.Fatal("pre-canceled scan admitted enumeration")
				}
			} else if !walkReturned || admitted != 1 {
				testingT.Fatalf("scanner returned before its partial census finished: returned=%v admitted=%d", walkReturned, admitted)
			}
			assertIncrementalGenerationUnchanged(testingT, fixture, databaseBefore, manifestBefore)
		})
	}
}

func TestIncrementalScan_ActualWalkErrorPreservesGeneration(testingT *testing.T) {
	fixture := newIncrementalFailureFixture(testingT)
	databaseBefore := incrementalWorldGeneration(testingT, fixture.database)
	manifestBefore, err := os.ReadFile(fixture.manifestPath)
	if err != nil {
		testingT.Fatal(err)
	}
	movedDirectory := filepath.Join(testingT.TempDir(), "moved")
	admitted := 0
	sawWalkError := false
	options := IncrementalOptions{}
	options.walkDir = func(root string, visit fs.WalkDirFunc) error {
		return filepath.WalkDir(root, func(path string, entry fs.DirEntry, pathErr error) error {
			sawWalkError = sawWalkError || errors.Is(pathErr, fs.ErrNotExist)
			visitErr := visit(path, entry, pathErr)
			if visitErr == nil && path == filepath.Join(root, "alpha.txt") {
				admitted++
				return os.Rename(filepath.Join(root, "nested"), movedDirectory)
			}
			return visitErr
		})
	}
	result, scanErr := fixture.scanner.ScanWorkspaceIncremental(context.Background(), fixture.root, fixture.database, options)
	if admitted != 1 || !sawWalkError {
		testingT.Fatalf("expected a real walk error after an admitted file: admitted=%d walkError=%v", admitted, sawWalkError)
	}
	if !errors.Is(scanErr, fs.ErrNotExist) || result != nil {
		testingT.Fatalf("expected the actual filesystem error and no result, got %+v, %v", result, scanErr)
	}
	assertIncrementalGenerationUnchanged(testingT, fixture, databaseBefore, manifestBefore)
}

func TestIncrementalScan_MembershipErrorPreservesGeneration(testingT *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		testingT.Fatalf("Git is required for the membership-failure witness: %v", err)
	}
	fixture := newIncrementalFailureFixture(testingT)
	databaseBefore := incrementalWorldGeneration(testingT, fixture.database)
	manifestBefore, err := os.ReadFile(fixture.manifestPath)
	if err != nil {
		testingT.Fatal(err)
	}
	for _, relative := range []string{".git/objects", ".git/refs"} {
		if err := os.MkdirAll(filepath.Join(fixture.root, filepath.FromSlash(relative)), 0o755); err != nil {
			testingT.Fatal(err)
		}
	}
	for relative, content := range map[string]string{
		".git/HEAD":   "ref: refs/heads/main\n",
		".git/config": "[core]\n\trepositoryformatversion = 0\n\tbare = false\n",
		".git/index":  "invalid index generation\n",
	} {
		if err := os.WriteFile(filepath.Join(fixture.root, filepath.FromSlash(relative)), []byte(content), 0o644); err != nil {
			testingT.Fatal(err)
		}
	}
	scanner := NewScannerWithConfig(ScannerConfig{
		MaxConcurrency: 2,
		IgnorePatterns: []string{"ignored", "never-present"},
	})
	result, scanErr := scanner.ScanWorkspaceIncremental(context.Background(), fixture.root, fixture.database, IncrementalOptions{})
	if scanErr == nil || !strings.Contains(scanErr.Error(), "git ls-files") || result != nil {
		testingT.Fatalf("expected the real membership snapshot error and no result, got %+v, %v", result, scanErr)
	}
	assertIncrementalGenerationUnchanged(testingT, fixture, databaseBefore, manifestBefore)
}

func TestIncrementalScan_SuccessfulCensusDeletesAndPrunes(testingT *testing.T) {
	fixture := newIncrementalFailureFixture(testingT)
	deletedPath := filepath.Join(fixture.root, "alpha.txt")
	deletedFacts, _, err := fixture.database.LoadWorldFactsForFile("alpha.txt", "fast")
	if err != nil || len(deletedFacts) == 0 {
		testingT.Fatalf("deletion control has no facts: %v", err)
	}
	deepBefore, fingerprintBefore, err := fixture.database.LoadWorldFactsForFile("nested/keep.txt", "deep")
	if err != nil {
		testingT.Fatal(err)
	}
	if len(deepBefore) != 1 || fingerprintBefore != "deep-generation" {
		testingT.Fatalf("deep preservation control is not seeded: %v, %q", deepBefore, fingerprintBefore)
	}
	if err := os.Remove(deletedPath); err != nil {
		testingT.Fatal(err)
	}
	visitedHidden := false
	options := IncrementalOptions{}
	options.walkDir = func(root string, visit fs.WalkDirFunc) error {
		return filepath.WalkDir(root, func(path string, entry fs.DirEntry, pathErr error) error {
			if path == filepath.Join(root, "ignored", "hidden.txt") {
				visitedHidden = true
			}
			return visit(path, entry, pathErr)
		})
	}
	result, err := fixture.scanner.ScanWorkspaceIncremental(context.Background(), fixture.root, fixture.database, options)
	if err != nil {
		testingT.Fatal(err)
	}
	if result == nil || result.Full || !reflect.DeepEqual(result.DeletedFiles, []string{deletedPath}) {
		testingT.Fatalf("successful census did not identify the genuine deletion: %+v", result)
	}
	if visitedHidden {
		testingT.Fatal("membership pruning descended into an excluded directory")
	}
	for _, original := range deletedFacts {
		matched := false
		for _, retracted := range result.RetractFacts {
			if retracted.Predicate == original.Predicate && reflect.DeepEqual(retracted.Args, original.Args) {
				matched = true
				break
			}
		}
		if !matched {
			testingT.Fatalf("missing exact retraction for %+v", original)
		}
	}
	paths, err := fixture.database.ListWorldFilePaths()
	if err != nil {
		testingT.Fatal(err)
	}
	kept := false
	for _, path := range paths {
		if path == "alpha.txt" || path == fixture.legacyPath {
			testingT.Fatalf("successful census retained deleted or noncanonical row %q", path)
		}
		kept = kept || path == "nested/keep.txt"
	}
	if !kept {
		testingT.Fatal("successful census removed the surviving canonical file")
	}
	deletedAfter, _, err := fixture.database.LoadWorldFactsForFile("alpha.txt", "fast")
	if err != nil || len(deletedAfter) != 0 {
		testingT.Fatalf("deleted facts remain: %v, %v", deletedAfter, err)
	}
	deepAfter, fingerprintAfter, err := fixture.database.LoadWorldFactsForFile("nested/keep.txt", "deep")
	if err != nil || !reflect.DeepEqual(deepBefore, deepAfter) || fingerprintBefore != fingerprintAfter {
		testingT.Fatalf("unrelated deep generation changed: %v, %v", deepAfter, err)
	}
	manifest, err := os.ReadFile(fixture.manifestPath)
	if err != nil {
		testingT.Fatal(err)
	}
	var entries map[string]CacheEntry
	if err := json.Unmarshal(manifest, &entries); err != nil {
		testingT.Fatal(err)
	}
	if _, exists := entries[deletedPath]; exists {
		testingT.Fatal("successful census retained the deleted cache entry")
	}
	if _, exists := entries[filepath.Join(fixture.root, "nested", "keep.txt")]; !exists {
		testingT.Fatal("successful census removed the surviving cache entry")
	}
}

type incrementalWorkerOutcome struct {
	result *IncrementalResult
	err    error
}

func TestIncrementalScan_DeltaWorkerFailurePreservesGeneration(testingT *testing.T) {
	for _, manifestState := range []string{"present", "removed_after_load"} {
		for _, mode := range []string{"hash_error", "read_error", "cancel_before_work", "cancel_after_work", "cancel_during_read", "cancel_admission"} {
			testingT.Run(manifestState+"/"+mode, func(testingT *testing.T) {
				fixture := newIncrementalFailureFixture(testingT)
				manifestBefore, err := os.ReadFile(fixture.manifestPath)
				if err != nil {
					testingT.Fatal(err)
				}
				if manifestState == "removed_after_load" {
					manifestBefore = nil
				}
				databaseBefore := incrementalWorldGeneration(testingT, fixture.database)
				target := filepath.Join(fixture.root, "alpha.txt")
				if err := os.WriteFile(target, []byte("changed alpha content for a real delta worker\n"), 0o644); err != nil {
					testingT.Fatal(err)
				}
				if err := os.Remove(filepath.Join(fixture.root, "nested", "keep.txt")); err != nil {
					testingT.Fatal(err)
				}
				if mode == "cancel_admission" {
					if err := os.WriteFile(filepath.Join(fixture.root, "zeta.txt"), []byte("second worker\n"), 0o644); err != nil {
						testingT.Fatal(err)
					}
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				workerReady := make(chan struct{})
				workerExited := make(chan struct{})
				releaseWorker := make(chan struct{})
				var releaseOnce sync.Once
				release := func() { releaseOnce.Do(func() { close(releaseWorker) }) }
				scanDone := make(chan struct{})
				finished := make(chan incrementalWorkerOutcome, 1)
				testingT.Cleanup(func() {
					cancel()
					release()
					awaitFullScanSignal(testingT, scanDone, "delta scan cleanup")
					select {
					case <-workerReady:
						awaitFullScanSignal(testingT, workerExited, "delta worker cleanup")
					default:
					}
				})
				scanner := NewScannerWithConfig(ScannerConfig{MaxConcurrency: 1, IgnorePatterns: []string{"ignored"}})
				if mode == "read_error" || mode == "cancel_during_read" {
					scanner.parserPool.New = func() any {
						close(workerReady)
						<-releaseWorker
						return NewTreeSitterParser()
					}
				}
				firstCensusDone := false
				manifestRemoved := false
				admitted := 0
				options := IncrementalOptions{}
				options.walkDir = func(root string, visit fs.WalkDirFunc) error {
					if err := filepath.WalkDir(root, visit); err != nil {
						return err
					}
					firstCensusDone = true
					if manifestState == "removed_after_load" {
						if err := os.Remove(fixture.manifestPath); err != nil {
							return err
						}
						manifestRemoved = true
					}
					return nil
				}
				options.runDeltaFile = func(path string, process func()) {
					admitted++
					if path != target {
						process()
						return
					}
					defer close(workerExited)
					if mode == "read_error" || mode == "cancel_during_read" {
						process()
						return
					}
					if mode == "cancel_after_work" {
						process()
					}
					close(workerReady)
					<-releaseWorker
					if mode != "cancel_after_work" {
						process()
					}
				}
				go func() {
					defer close(scanDone)
					result, err := scanner.ScanWorkspaceIncremental(ctx, fixture.root, fixture.database, options)
					finished <- incrementalWorkerOutcome{result: result, err: err}
				}()
				awaitFullScanSignal(testingT, workerReady, "admitted delta worker")
				expected := error(context.Canceled)
				if mode == "hash_error" || mode == "read_error" {
					expected = fs.ErrNotExist
					if err := os.Remove(target); err != nil {
						testingT.Fatal(err)
					}
				} else {
					cancel()
				}
				select {
				case outcome := <-finished:
					testingT.Fatalf("delta scan returned with an admitted worker still active: %+v", outcome)
				case <-time.After(100 * time.Millisecond):
				}
				assertFullScanGenerationUnchanged(testingT, fixture, databaseBefore, manifestBefore)
				release()
				awaitFullScanSignal(testingT, scanDone, "joined delta scan")
				outcome := <-finished
				select {
				case <-workerExited:
				default:
					testingT.Fatal("delta worker did not join before return")
				}
				if !firstCensusDone || admitted != 1 || (manifestState == "removed_after_load" && !manifestRemoved) {
					testingT.Fatalf("missing nonempty-cache delta witness: census=%v admitted=%d manifestRemoved=%v", firstCensusDone, admitted, manifestRemoved)
				}
				if !errors.Is(outcome.err, expected) || outcome.result != nil {
					testingT.Fatalf("expected original delta failure and nil result, got %+v", outcome)
				}
				assertFullScanGenerationUnchanged(testingT, fixture, databaseBefore, manifestBefore)
			})
		}
	}
}

func TestIncrementalScan_DeltaContentFailurePreservesGeneration(testingT *testing.T) {
	fixture := newIncrementalFailureFixture(testingT)
	manifestBefore, err := os.ReadFile(fixture.manifestPath)
	if err != nil {
		testingT.Fatal(err)
	}
	databaseBefore := incrementalWorldGeneration(testingT, fixture.database)
	target := filepath.Join(fixture.root, "alpha.txt")
	if err := os.WriteFile(target, []byte("changed content before census\n"), 0o644); err != nil {
		testingT.Fatal(err)
	}
	firstCensusDone := false
	workerCalled := false
	options := IncrementalOptions{}
	options.walkDir = func(root string, visit fs.WalkDirFunc) error {
		if err := filepath.WalkDir(root, visit); err != nil {
			return err
		}
		firstCensusDone = true
		return os.Remove(target)
	}
	options.runDeltaFile = func(path string, process func()) {
		workerCalled = true
		process()
	}
	result, err := fixture.scanner.ScanWorkspaceIncremental(context.Background(), fixture.root, fixture.database, options)
	if !firstCensusDone || workerCalled || !errors.Is(err, fs.ErrNotExist) || result != nil {
		testingT.Fatalf("missing content-classification failure: census=%v worker=%v result=%+v err=%v", firstCensusDone, workerCalled, result, err)
	}
	assertIncrementalGenerationUnchanged(testingT, fixture, databaseBefore, manifestBefore)
}

func TestIncrementalScan_DeltaSuccessPublishesGeneration(testingT *testing.T) {
	fixture := newIncrementalFailureFixture(testingT)
	changedPath := filepath.Join(fixture.root, "alpha.txt")
	newPath := filepath.Join(fixture.root, "new.txt")
	deletedPath := filepath.Join(fixture.root, "nested", "keep.txt")
	var expectedRetractions []store.WorldFactInput
	for _, relative := range []string{"alpha.txt", "nested/keep.txt"} {
		facts, _, err := fixture.database.LoadWorldFactsForFile(relative, "fast")
		if err != nil || len(facts) == 0 {
			testingT.Fatalf("missing old facts for %s: %v", relative, err)
		}
		expectedRetractions = append(expectedRetractions, facts...)
	}
	if err := os.WriteFile(changedPath, []byte("successful changed delta content\n"), 0o644); err != nil {
		testingT.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("successful new delta file\n"), 0o644); err != nil {
		testingT.Fatal(err)
	}
	if err := os.Remove(deletedPath); err != nil {
		testingT.Fatal(err)
	}
	result, err := fixture.scanner.ScanWorkspaceIncremental(context.Background(), fixture.root, fixture.database, IncrementalOptions{})
	if err != nil || result == nil || result.Full || !reflect.DeepEqual(result.ChangedFiles, []string{changedPath}) || !reflect.DeepEqual(result.NewFiles, []string{newPath}) || !reflect.DeepEqual(result.DeletedFiles, []string{deletedPath}) {
		testingT.Fatalf("successful delta did not refresh, add and delete the intended files: %+v, %v", result, err)
	}
	for _, original := range expectedRetractions {
		matched := false
		for _, retracted := range result.RetractFacts {
			matched = matched || (retracted.Predicate == original.Predicate && reflect.DeepEqual(retracted.Args, original.Args))
		}
		if !matched {
			testingT.Fatalf("missing exact delta retraction for %+v", original)
		}
	}
	paths, err := fixture.database.ListWorldFilePaths()
	if err != nil {
		testingT.Fatal(err)
	}
	for _, path := range paths {
		if path == "nested/keep.txt" || isNonCanonicalWorldPath(path) {
			testingT.Fatalf("successful delta retained deleted or legacy path %q", path)
		}
	}
	manifest, err := os.ReadFile(fixture.manifestPath)
	if err != nil {
		testingT.Fatal(err)
	}
	var entries map[string]CacheEntry
	if err := json.Unmarshal(manifest, &entries); err != nil || len(entries) != 2 {
		testingT.Fatalf("expected two published delta cache entries: %v, %v", entries, err)
	}
	if _, exists := entries[deletedPath]; exists {
		testingT.Fatal("successful delta retained the deleted manifest entry")
	}
	for _, relative := range []string{"alpha.txt", "new.txt"} {
		entry, exists := entries[filepath.Join(fixture.root, relative)]
		facts, _, err := fixture.database.LoadWorldFactsForFile(relative, "fast")
		if !exists || entry.Hash == "" || err != nil || len(facts) == 0 {
			testingT.Fatalf("successful delta did not publish %s: %+v, %v, %v", relative, entry, facts, err)
		}
		matched := false
		for _, fact := range facts {
			if fact.Predicate == "file_topology" && len(fact.Args) == 5 {
				matched = matched || (fact.Args[0] == relative && fact.Args[1] == entry.Hash)
			}
		}
		if !matched {
			testingT.Fatalf("persisted delta hash does not match cache for %s", relative)
		}
	}
}

func TestIncrementalScan_DeltaSiblingFailurePreservesGeneration(testingT *testing.T) {
	fixture := newIncrementalFailureFixture(testingT)
	sources := map[string]string{
		"a.go": "package p\n\nfunc T(value int) int { return value }\n",
		"b.go": "package p\n\nfunc Use(value int) int { return T(value) }\n",
	}
	for relative, content := range sources {
		if err := os.WriteFile(filepath.Join(fixture.root, relative), []byte(content), 0o644); err != nil {
			testingT.Fatal(err)
		}
	}
	if _, err := fixture.scanner.ScanWorkspaceIncremental(context.Background(), fixture.root, fixture.database, IncrementalOptions{}); err != nil {
		testingT.Fatal(err)
	}
	if _, err := EnsureDeepFactsInRoot(context.Background(), fixture.root, []string{filepath.Join(fixture.root, "a.go"), filepath.Join(fixture.root, "b.go")}, fixture.database, 1); err != nil {
		testingT.Fatal(err)
	}
	if !loadDeepCalls(testingT, fixture.database, fixture.root, "b.go")[[2]string{"fn:p.Use", "fn:p.T"}] {
		testingT.Fatal("sibling fixture has no old function call to preserve")
	}
	if err := fixture.database.UpsertWorldFile(store.WorldFileMeta{Path: fixture.legacyPath, Hash: "legacy-hash", Fingerprint: "legacy-generation"}); err != nil {
		testingT.Fatal(err)
	}
	if err := fixture.database.ReplaceWorldFactsForFile(fixture.legacyPath, "fast", "legacy-generation", []store.WorldFactInput{
		{Predicate: "file_topology", Args: []any{fixture.legacyPath, "legacy-hash", "/unknown", int64(42), "/false"}},
	}); err != nil {
		testingT.Fatal(err)
	}
	manifestBefore, err := os.ReadFile(fixture.manifestPath)
	if err != nil {
		testingT.Fatal(err)
	}
	databaseBefore := incrementalWorldGeneration(testingT, fixture.database)
	if err := os.WriteFile(filepath.Join(fixture.root, "a.go"), []byte("package p\n\ntype T int\n"), 0o644); err != nil {
		testingT.Fatal(err)
	}
	siblingPath := filepath.Join(fixture.root, "b.go")
	var removeErr error
	siblingProcessed := false
	options := IncrementalOptions{}
	options.runDeltaFile = func(path string, process func()) {
		process()
		if path == siblingPath {
			siblingProcessed = true
			removeErr = os.Remove(path)
		}
	}
	result, err := fixture.scanner.ScanWorkspaceIncremental(context.Background(), fixture.root, fixture.database, options)
	if !siblingProcessed || removeErr != nil || !errors.Is(err, fs.ErrNotExist) || result != nil {
		testingT.Fatalf("missing staged sibling read failure: processed=%v removal=%v result=%+v err=%v", siblingProcessed, removeErr, result, err)
	}
	assertIncrementalGenerationUnchanged(testingT, fixture, databaseBefore, manifestBefore)
	if err := os.WriteFile(siblingPath, []byte(sources["b.go"]), 0o644); err != nil {
		testingT.Fatal(err)
	}
	retried, err := fixture.scanner.ScanWorkspaceIncremental(context.Background(), fixture.root, fixture.database, IncrementalOptions{})
	if err != nil || retried == nil || retried.Full {
		testingT.Fatalf("failed declaration refresh did not recover: %+v, %v", retried, err)
	}
	if !hasFact(retried.RetractFacts, "code_calls", "fn:p.Use", "fn:p.T") {
		testingT.Fatal("retry did not retract the previously persisted sibling function call")
	}
	if loadDeepCalls(testingT, fixture.database, fixture.root, "b.go")[[2]string{"fn:p.Use", "fn:p.T"}] {
		testingT.Fatal("retry left a stale sibling function call after the declaration became a type")
	}
	if err := os.WriteFile(siblingPath, []byte("package p\n\nfunc Use(value int) int { return int(T(value)) + 1 }\n"), 0o644); err != nil {
		testingT.Fatal(err)
	}
	bodyOnly, err := fixture.scanner.ScanWorkspaceIncremental(context.Background(), fixture.root, fixture.database, IncrementalOptions{})
	if err != nil || bodyOnly == nil || bodyOnly.Full || !reflect.DeepEqual(topologyFiles(bodyOnly.NewFacts), []string{"b.go"}) {
		testingT.Fatalf("body edit after a recovered declaration refresh remapped untouched siblings: %+v, %v", bodyOnly, err)
	}
}
