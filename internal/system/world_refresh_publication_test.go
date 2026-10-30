package system

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
	"codenerd/internal/world"
)

// Called by the real Cortex write regression, after its action and modular
// controls establish the production scanner, kernel and graph wiring.
func exerciseWorldPublicationRetry(t *testing.T, ctx context.Context, cortex *Cortex, path, body string) {
	t.Helper()
	ws := cortex.Workspace
	registry := cortex.VirtualStore.GetModularTools()

	// A projection failure after a successful write must retain the retirement
	// delta. Before ordered publication, Refresh had already advanced the cache
	// and the next unchanged call skipped projection forever. Exercise both
	// production completion routes with a real SQL transaction failure.
	withoutImport := "package main\nfunc After() int { return 0 }\n"
	for _, route := range []string{"action", "modular"} {
		t.Run("publication-retry-"+route, func(t *testing.T) {
			added, err := registry.Execute(ctx, "write_file", map[string]any{"path": path, "content": body})
			if err != nil || added == nil || added.Error != nil {
				t.Fatalf("add import: result=%+v err=%v", added, err)
			}
			assertDependency := func(want bool) {
				t.Helper()
				links, err := cortex.VirtualStore.QueryKnowledgeGraph("main.go", "outgoing")
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, link := range links {
					if link.Args[1] == "/depends_on" && link.Args[2] == "lib/lib.go" {
						found = true
					}
				}
				if found != want {
					t.Fatalf("dependency present=%v, want %v: %v", found, want, links)
				}
			}
			assertDependency(true)
			if _, err := cortex.LocalDB.GetDB().Exec(`CREATE TRIGGER fail_world_retirement
				BEFORE DELETE ON knowledge_graph
				WHEN OLD.entity_a = 'main.go' AND OLD.relation = 'depends_on'
				BEGIN SELECT RAISE(ABORT, 'projection unavailable'); END`); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = cortex.LocalDB.GetDB().Exec(`DROP TRIGGER IF EXISTS fail_world_retirement`) })
			if route == "modular" {
				removed, err := registry.Execute(ctx, "write_file", map[string]any{"path": path, "content": withoutImport})
				if err != nil || removed == nil || removed.Error != nil {
					t.Fatalf("landed modular write must remain successful: result=%+v err=%v", removed, err)
				}
			} else {
				params := map[string]any{"content": withoutImport}
				encoded, err := json.Marshal(params)
				if err != nil {
					t.Fatal(err)
				}
				id := "world-remove-import"
				if err := cortex.Kernel.Assert(core.Fact{Predicate: "pending_action", Args: []any{
					id, types.MangleAtom("/write_file"), path, string(encoded), time.Now().Unix(),
				}}); err != nil {
					t.Fatal(err)
				}
				removed, err := cortex.VirtualStore.RouteActionResult(ctx, core.Fact{Predicate: "next_action",
					Args: []any{id, "write_file", path, params}})
				if err != nil || !removed.Success || !strings.Contains(removed.Output, "World refresh failed") {
					t.Fatalf("landed action must expose projection failure: result=%+v err=%v", removed, err)
				}
			}
			if rows, err := cortex.Kernel.Query(`dependency_link("main.go", "lib/lib.go", I)`); err != nil || len(rows) != 0 {
				t.Fatalf("world application failed while graph was pending: %v, err=%v", rows, err)
			}
			assertDependency(true) // the SQL abort rolled back the complete replacement
			stats := cortex.Scanner.RefreshStats()
			if !stats.PublicationPending || stats.PublicationFailures == 0 {
				t.Fatalf("failed projection was reported current: %+v", stats)
			}
			// A repeated failure blocks new observations instead of dropping the
			// only receipt or growing an unbounded queue.
			if _, err := cortex.Scanner.Refresh(ctx, ws, cortex.LocalDB, cortex.Kernel,
				world.IncrementalOptions{SkipWhenUnchanged: true}); err == nil {
				t.Fatal("pending projection failure was hidden by unchanged source")
			}
			if retried := cortex.Scanner.RefreshStats(); retried.FullScans != stats.FullScans ||
				retried.DeltaScans != stats.DeltaScans || retried.UnchangedScans != stats.UnchangedScans {
				t.Fatalf("a new observation advanced past failed publication: before=%+v after=%+v", stats, retried)
			}
			if _, err := cortex.LocalDB.GetDB().Exec(`DROP TRIGGER fail_world_retirement`); err != nil {
				t.Fatal(err)
			}
			res, err := cortex.Scanner.Refresh(ctx, ws, cortex.LocalDB, cortex.Kernel,
				world.IncrementalOptions{SkipWhenUnchanged: true})
			if err != nil || res == nil || !res.Unchanged {
				t.Fatalf("recovery should replay then observe unchanged source: res=%+v err=%v", res, err)
			}
			assertDependency(false)
			if stats := cortex.Scanner.RefreshStats(); stats.PublicationPending {
				t.Fatalf("successful replay left publication pending: %+v", stats)
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != withoutImport {
				t.Fatalf("projection replay changed the landed write: %q, err=%v", got, err)
			}
		})
	}
	// Cold storage is a separate upsert mirror. Its failure must remain
	// visible without preventing an otherwise healthy graph retirement.
	if added, err := registry.Execute(ctx, "write_file", map[string]any{"path": path, "content": body}); err != nil || added == nil || added.Error != nil {
		t.Fatalf("restore import before mirror failure: result=%+v err=%v", added, err)
	}
	if _, err := cortex.LocalDB.GetDB().Exec(`CREATE TRIGGER fail_world_mirror
		BEFORE INSERT ON cold_storage BEGIN SELECT RAISE(ABORT, 'mirror unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = cortex.LocalDB.GetDB().Exec(`DROP TRIGGER IF EXISTS fail_world_mirror`) }()
	if removed, err := registry.Execute(ctx, "write_file", map[string]any{"path": path, "content": withoutImport}); err != nil || removed == nil || removed.Error != nil {
		t.Fatalf("landed write with mirror failure: result=%+v err=%v", removed, err)
	}
	links, err := cortex.VirtualStore.QueryKnowledgeGraph("main.go", "outgoing")
	if err != nil {
		t.Fatal(err)
	}
	for _, link := range links {
		if link.Args[1] == "/depends_on" && link.Args[2] == "lib/lib.go" {
			t.Fatalf("cold-mirror failure suppressed graph retirement: %v", links)
		}
	}
	if stats := cortex.Scanner.RefreshStats(); !stats.PublicationPending {
		t.Fatalf("mirror failure lost its pending receipt: %+v", stats)
	}
	if _, err := cortex.LocalDB.GetDB().Exec(`DROP TRIGGER fail_world_mirror`); err != nil {
		t.Fatal(err)
	}
	if _, err := cortex.Scanner.Refresh(ctx, ws, cortex.LocalDB, cortex.Kernel, world.IncrementalOptions{SkipWhenUnchanged: true}); err != nil {
		t.Fatal(err)
	}
}
