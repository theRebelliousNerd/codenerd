package world

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"codenerd/internal/store"
)

func TestIncrementalImports_FailedRefreshPreservesGeneration(t *testing.T) {
	for _, failure := range []string{"importer_read", "prior_fact_read", "replacement_census", "replacement_cancel"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			writeWorkspaceFile(t, root, "go.mod", "module example.com/ws\n\ngo 1.26\n")
			source := "package app\nimport \"example.com/ws/pkg\"\nfunc A(){pkg.B()}\n"
			importer := writeWorkspaceFile(t, root, "app/a.go", source)
			writeWorkspaceFile(t, root, "pkg/b.go", "package pkg\nfunc B(){}\n")
			var db *store.LocalStore
			var databaseBefore []byte
			dbPath := filepath.Join(t.TempDir(), "world.db")
			if failure == "importer_read" || failure == "prior_fact_read" {
				var err error
				db, err = store.NewLocalStore(dbPath)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = db.Close() })
			}
			scanner := NewScanner()
			if _, err := scanner.ScanWorkspaceIncremental(t.Context(), root, db, IncrementalOptions{}); err != nil {
				t.Fatal(err)
			}
			if db != nil {
				databaseBefore = incrementalWorldGeneration(t, db)
			}
			manifest := NewFileCache(root).path
			before, err := os.ReadFile(manifest)
			if err != nil {
				t.Fatal(err)
			}
			added := writeWorkspaceFile(t, root, "pkg/c.go", "package pkg\nfunc C(){}\n")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			injected, firstCensus := false, false
			walkFailure := errors.New("replacement census failed")
			opts := IncrementalOptions{}
			opts.walkDir = func(root string, visit fs.WalkDirFunc) error {
				err := filepath.WalkDir(root, visit)
				firstCensus = err == nil
				return err
			}
			switch failure {
			case "prior_fact_read":
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
				injected = true
			case "importer_read":
				opts.runDeltaFile = func(path string, process func()) {
					if path == importer {
						injected = true
						if err := os.Remove(importer); err != nil {
							t.Error(err)
						}
						defer func() {
							if err := os.WriteFile(importer, []byte(source), 0o644); err != nil {
								t.Error(err)
							}
						}()
					}
					process()
				}
			case "replacement_census":
				opts.fullScan.walk = func(root string, visit filepath.WalkFunc) error {
					return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
						if path == added {
							injected = firstCensus
							return walkFailure
						}
						return visit(path, info, err)
					})
				}
			case "replacement_cancel":
				opts.fullScan.runFile = func(path string, process func()) {
					if path == added {
						injected = firstCensus
						cancel()
					}
					process()
				}
			}
			result, err := scanner.ScanWorkspaceIncremental(ctx, root, db, opts)
			wantErr := error(fs.ErrNotExist)
			if failure == "replacement_census" {
				wantErr = walkFailure
			} else if failure == "replacement_cancel" {
				wantErr = context.Canceled
			}
			matches := errors.Is(err, wantErr)
			if failure == "prior_fact_read" {
				matches = err != nil && strings.Contains(err.Error(), "load prior import facts")
				var reopenErr error
				db, reopenErr = store.NewLocalStore(dbPath)
				if reopenErr != nil {
					t.Fatal(reopenErr)
				}
			}
			if !injected || result != nil || !matches {
				t.Fatalf("missing staged failure: injected=%v result=%+v error=%v", injected, result, err)
			}
			after, err := os.ReadFile(manifest)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("failed import refresh replaced the manifest")
			}
			if db != nil && !bytes.Equal(databaseBefore, incrementalWorldGeneration(t, db)) {
				t.Fatal("failed import refresh changed persisted world rows")
			}
			if retry, err := scanner.ScanWorkspaceIncremental(t.Context(), root, db, IncrementalOptions{}); err != nil || retry == nil {
				t.Fatalf("retry failed: %+v / %v", retry, err)
			}
		})
	}
}

func TestIncrementalImports_SharedManifestPreservesOtherRoots(t *testing.T) {
	owner := t.TempDir()
	writeWorkspaceFile(t, owner, ".nerd/config.json", "{}")
	outside := writeWorkspaceFile(t, owner, "other/keep.go", "package other\n")
	root := filepath.Join(owner, "sub")
	writeWorkspaceFile(t, root, "go.mod", "module example.com/ws\n\ngo 1.26\n")
	writeWorkspaceFile(t, root, "app/a.go", "package app\nimport \"example.com/ws/pkg\"\nfunc A(){pkg.B()}\n")
	target := writeWorkspaceFile(t, root, "pkg/b.go", "package pkg\nfunc B(){}\n")
	cache := NewFileCache(root)
	info, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}
	cache.Update(outside, info, "unrelated-generation")
	if err := cache.Save(); err != nil {
		t.Fatal(err)
	}
	otherBefore := cache.Entries[outside]
	scanner := NewScanner()
	if _, err := scanner.ScanWorkspaceIncremental(t.Context(), root, nil, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	result, err := scanner.ScanWorkspaceIncremental(t.Context(), root, nil, IncrementalOptions{})
	if err != nil || !result.Full || !reflect.DeepEqual(result.DeletedFiles, []string{target}) {
		t.Fatalf("scoped replacement: %+v / %v", result, err)
	}
	cache = NewFileCache(root)
	if !reflect.DeepEqual(cache.Entries[outside], otherBefore) {
		t.Fatal("subtree replacement changed another root's cache entry")
	}
	if _, retained := cache.Entries[target]; retained {
		t.Fatal("deleted target retained in the completed manifest")
	}
	quiet, err := scanner.ScanWorkspaceIncremental(t.Context(), root, nil, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil || !quiet.Unchanged {
		t.Fatalf("other root caused a perpetual replacement: %+v / %v", quiet, err)
	}
}
