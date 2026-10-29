package campaign

import (
	"testing"

	"go.uber.org/goleak"

	"codenerd/internal/perception"
)

// TestMain wires goleak.VerifyTestMain so every test in this package is
// leak-checked at end of suite. Keep the ignore list MINIMAL — only
// process-lifetime background goroutines belong here. Don't paper over
// real leaks.

type testMainWrapper struct {
	m *testing.M
}

func (w testMainWrapper) Run() int {
	code := w.m.Run()
	perception.ShutdownSharedTaxonomy()
	return code
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(testMainWrapper{m},
		// database/sql's connection opener is started by sql.Open and lives
		// until *DB is GC'd; tests that legitimately use DB handles without
		// explicit Close (or that share a singleton DB) leave this dangling.
		// Same rationale as internal/core's TestMain.
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
	)
}
