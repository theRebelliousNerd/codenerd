package world

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/store"
)

func importMembershipRows(facts []core.Fact, owner string) []string {
	set := map[string]bool{}
	for _, f := range facts {
		if f.Predicate != "dependency_link" && f.Predicate != "import_unit" && f.Predicate != "file_unit" && f.Predicate != "imports_project_file" {
			continue
		}
		if owner != "" && (len(f.Args) == 0 || f.Args[0] != owner) {
			continue
		}
		set[f.String()] = true
	}
	rows := make([]string, 0, len(set))
	for row := range set {
		rows = append(rows, row)
	}
	sort.Strings(rows)
	return rows
}

func TestIncrementalImports_TargetMembershipMatchesFull(t *testing.T) {
	for _, mode := range []string{"persisted", "reopened", "no-db"} {
		for _, change := range []string{"add_to_package", "delete_from_package", "first_target", "last_target", "body_only", "unrelated_add", "test_only", "excluded_add"} {
			t.Run(mode+"/"+change, func(t *testing.T) {
				// Each deletion starts from its own full, correct baseline. It
				// never inherits a missing edge from a preceding failed add.
				root := t.TempDir()
				writeWorkspaceFile(t, root, "go.mod", "module example.com/ws\n\ngo 1.26\n")
				importer := writeWorkspaceFile(t, root, "app/a.go", "package app\nimport \"example.com/ws/pkg\"\nfunc A(){pkg.B()}\n")
				writeWorkspaceFile(t, root, "external/external.go", "package external\nimport \"fmt\"\nfunc E(){fmt.Println()}\n")
				if change != "first_target" {
					writeWorkspaceFile(t, root, "pkg/b.go", "package pkg\nfunc B(){}\n")
				}
				if change == "delete_from_package" {
					writeWorkspaceFile(t, root, "pkg/c.go", "package pkg\nfunc C(){}\n")
				}
				original, err := os.ReadFile(importer)
				if err != nil {
					t.Fatal(err)
				}
				var db *store.LocalStore
				dbPath := filepath.Join(t.TempDir(), "world.db")
				if mode != "no-db" {
					db, err = store.NewLocalStore(dbPath)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = db.Close() })
				}
				cfg := DefaultScannerConfig()
				cfg.IgnorePatterns = append(cfg.IgnorePatterns, "pkg/ignored.go")
				makeScanner := func() *Scanner { return NewScannerWithConfig(cfg) }
				scanner := makeScanner()
				initial, err := scanner.ScanWorkspaceIncremental(t.Context(), root, db, IncrementalOptions{})
				if err != nil || !initial.Full {
					t.Fatalf("initial full scan: %+v / %v", initial, err)
				}
				fresh := func() []core.Fact {
					t.Helper()
					facts, err := makeScanner().ScanWorkspaceCtx(t.Context(), root)
					if err != nil {
						t.Fatal(err)
					}
					return facts
				}
				if !reflect.DeepEqual(importMembershipRows(initial.NewFacts, ""), importMembershipRows(fresh(), "")) {
					t.Fatal("fixture baseline differs from a fresh full scan")
				}
				for _, target := range []string{"pkg/b.go", "pkg/c.go"} {
					_, statErr := os.Stat(filepath.Join(root, target))
					if hasEdge(dependencyEdges(initial.NewFacts), "app/a.go", target) != (statErr == nil) {
						t.Fatalf("baseline has incorrect importer edge to %s", target)
					}
				}
				kernel, err := core.NewRealKernelWithWorkspace(root)
				if err != nil {
					t.Fatal(err)
				}
				if err := ApplyIncrementalResult(kernel, initial); err != nil {
					t.Fatal(err)
				}
				if mode == "reopened" {
					if err := db.Close(); err != nil {
						t.Fatal(err)
					}
					db, err = store.NewLocalStore(dbPath)
					if err != nil {
						t.Fatal(err)
					}
					scanner = makeScanner()
				}
				switch change {
				case "add_to_package":
					writeWorkspaceFile(t, root, "pkg/c.go", "package pkg\nfunc C(){}\n")
				case "delete_from_package":
					err = os.Remove(filepath.Join(root, "pkg/c.go"))
				case "first_target":
					writeWorkspaceFile(t, root, "pkg/b.go", "package pkg\nfunc B(){}\n")
				case "last_target":
					err = os.Remove(filepath.Join(root, "pkg/b.go"))
				case "body_only":
					writeWorkspaceFile(t, root, "pkg/b.go", "package pkg\nfunc B(){_ = 1}\n")
				case "unrelated_add":
					writeWorkspaceFile(t, root, "other/new.go", "package other\nfunc N(){}\n")
				case "test_only":
					writeWorkspaceFile(t, root, "pkg/b_test.go", "package pkg\nfunc TestB(){}\n")
				case "excluded_add":
					writeWorkspaceFile(t, root, "pkg/ignored.go", "package pkg\nfunc Hidden(){}\n")
				}
				if err != nil {
					t.Fatal(err)
				}
				delta, err := scanner.ScanWorkspaceIncremental(t.Context(), root, db, IncrementalOptions{SkipWhenUnchanged: true})
				if err != nil {
					t.Fatal(err)
				}
				if db != nil && delta.Full {
					t.Fatal("persisted scan unnecessarily replaced the full snapshot")
				}
				if err := ApplyIncrementalResult(kernel, delta); err != nil {
					t.Fatal(err)
				}
				var applied []core.Fact
				for _, predicate := range []string{"dependency_link", "import_unit", "file_unit", "imports_project_file"} {
					facts, err := kernel.Query(predicate)
					if err != nil {
						t.Fatal(err)
					}
					applied = append(applied, facts...)
				}
				full := fresh()
				if got, want := importMembershipRows(applied, ""), importMembershipRows(full, ""); !reflect.DeepEqual(got, want) {
					t.Errorf("incremental imports differ from full scan:\ngot %v\nwant %v", got, want)
				}
				if db != nil {
					rows, _, err := db.LoadWorldFactsForFile("app/a.go", "fast")
					if err != nil {
						t.Fatal(err)
					}
					var stored []core.Fact
					for _, row := range rows {
						stored = append(stored, core.Fact{Predicate: row.Predicate, Args: row.Args})
					}
					if got, want := importMembershipRows(stored, "app/a.go"), importMembershipRows(full, "app/a.go"); !reflect.DeepEqual(got, want) {
						t.Errorf("persisted importer differs from full scan:\ngot %v\nwant %v", got, want)
					}
					if change == "body_only" || change == "unrelated_add" || change == "test_only" || change == "excluded_add" {
						for _, f := range delta.NewFacts {
							if f.Predicate == "file_topology" && f.Args[0] == "app/a.go" {
								t.Error("unaffected importer was reparsed")
							}
						}
					}
				}
				if now, err := os.ReadFile(importer); err != nil || !bytes.Equal(original, now) {
					t.Fatal("fixture importer changed")
				}
				quiet, err := scanner.ScanWorkspaceIncremental(t.Context(), root, db, IncrementalOptions{SkipWhenUnchanged: true})
				if err != nil || !quiet.Unchanged {
					t.Fatalf("next scan not unchanged: %+v / %v", quiet, err)
				}
			})
		}
	}
}
