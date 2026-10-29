//go:build sqlite_modernc

package sqlpragmas

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

// The whole soft-fail design of this package rests on one claim: the pure-Go
// modernc.org/sqlite driver rejects a subset of the pragmas the CGO
// mattn/go-sqlite3 driver accepts, and those rejections are harmless. That
// claim has never been tested — the existing tests import mattn only, so the
// modernc reject set was known solely from Debug logs in production, if anyone
// happened to be looking.
//
// This test measures it. It applies every profile through the modernc driver
// with failure metrics on, and compares the observed reject set against
// knownModerncRejects. A driver upgrade that starts rejecting a new pragma
// fails here rather than silently degrading every pure-Go build; a driver
// upgrade that FIXES one fails too, so the pin (and any workaround built on it)
// gets retired.
//
// Build-tagged because it pulls the pure-Go driver into the test binary:
//
//	go test -tags sqlite_modernc ./internal/sqlpragmas/

// knownModerncRejects maps a PRAGMA statement to why modernc.org/sqlite
// refuses it. Empty means the pure-Go driver accepts the entire preset table.
var knownModerncRejects = map[string]string{}

func TestModerncDriver_WhenApplyingProfiles_ShouldMatchKnownRejectSet(t *testing.T) {
	ClearHostClass()
	t.Cleanup(ClearHostClass)

	ResetPragmaMetrics()
	SetMetricsEnabled(true)
	t.Cleanup(func() {
		SetMetricsEnabled(false)
		ResetPragmaMetrics()
	})

	profiles := []PragmaProfile{ProfileHot, ProfileBulkBuild, ProfileQuery, ProfileReadOnly}
	for _, profile := range profiles {
		db, err := OpenWithPragmas("sqlite", filepath.Join(t.TempDir(), "modernc.db"), profile)
		require.NoError(t, err, "open with %s", profile)

		// Force a connection so the connector hook actually runs.
		require.NoError(t, db.Ping(), "ping with %s", profile)
		require.NoError(t, db.Close())
	}

	observed := FailingPragmas()
	t.Logf("modernc.org/sqlite rejected %d of the emitted pragmas: %s",
		len(observed), strings.Join(observed, " | "))

	expected := make([]string, 0, len(knownModerncRejects))
	for stmt := range knownModerncRejects {
		expected = append(expected, stmt)
	}
	sort.Strings(expected)

	observedSet := map[string]bool{}
	for _, s := range observed {
		observedSet[s] = true
		if _, ok := knownModerncRejects[s]; !ok {
			t.Errorf("modernc.org/sqlite newly rejects %q.\n"+
				"Either the driver regressed or the preset changed. Decide whether the pragma still belongs "+
				"in the preset, then pin it in knownModerncRejects with the reason.", s)
		}
	}
	for stmt, reason := range knownModerncRejects {
		if !observedSet[stmt] {
			t.Errorf("stale knownModerncRejects entry %q (%s): the driver now accepts it. Remove the pin.", stmt, reason)
		}
	}
}

// TestModerncDriver_WhenProfileApplied_ShouldReachTunedState checks the
// outcome, not just the absence of errors: a pragma can be accepted and
// silently ignored by a driver, which is worse than a rejection because
// nothing is logged.
func TestModerncDriver_WhenProfileApplied_ShouldReachTunedState(t *testing.T) {
	ClearHostClass()
	t.Cleanup(ClearHostClass)

	db, err := OpenWithPragmas("sqlite", filepath.Join(t.TempDir(), "tuned.db"), ProfileHot)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)

	var journal string
	require.NoError(t, db.QueryRow("PRAGMA journal_mode").Scan(&journal))
	require.Equal(t, "wal", strings.ToLower(journal), "ProfileHot must reach WAL on the pure-Go driver")

	var cache int64
	require.NoError(t, db.QueryRow("PRAGMA cache_size").Scan(&cache))
	require.Equal(t, int64(-2097152), cache, "ProfileHot cache_size must land on the pure-Go driver")

	var mmap int64
	require.NoError(t, db.QueryRow("PRAGMA mmap_size").Scan(&mmap))
	require.Positive(t, mmap, "mmap should be enabled on the pure-Go driver")
}
