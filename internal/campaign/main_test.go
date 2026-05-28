package campaign

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain wires goleak.VerifyTestMain so every test in this package is
// leak-checked at end of suite. Keep the ignore list MINIMAL — only
// process-lifetime background goroutines belong here. Don't paper over
// real leaks.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// codenerd/internal/perception spawns SharedTaxonomy in its package
		// init(), which starts a ConsolidationWorker goroutine with no
		// shutdown hook reachable from test code. Process-lifetime by design.
		goleak.IgnoreTopFunction("codenerd/internal/perception.(*ConsolidationWorker).Start.func1"),
		// database/sql's connection opener is started by sql.Open and lives
		// until *DB is GC'd; tests that legitimately use DB handles without
		// explicit Close (or that share a singleton DB) leave this dangling.
		// Same rationale as internal/core's TestMain.
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
	)
}
