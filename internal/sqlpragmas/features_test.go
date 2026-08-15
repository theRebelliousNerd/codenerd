package sqlpragmas

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

// ---------------------------------------------------------------------------
// Idempotency (TODO.md P3: idempotency tests for BulkBuild / Query / ReadOnly)
// ---------------------------------------------------------------------------

// Re-applying a profile has to be free of side effects: several code paths
// (autopoiesis startup, prompt cache rebuild, migration runners that reopen a
// path already held elsewhere) apply pragmas more than once against the same
// file. Only ProfileHot was covered; a profile whose second application
// changed observable state would have gone unnoticed.
func TestApplyDefaultPragmas_WhenReapplied_ShouldBeIdempotent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		profile PragmaProfile
	}{
		{"BulkBuild", ProfileBulkBuild},
		{"Query", ProfileQuery},
		{"ReadOnly", ProfileReadOnly},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openFreshTempDB(t)
			ApplyDefaultPragmas(db, tc.profile)

			before := snapshotPragmas(t, db)
			ApplyDefaultPragmas(db, tc.profile)
			ApplyDefaultPragmas(db, tc.profile)
			after := snapshotPragmas(t, db)

			require.Equal(t, before, after,
				"%s: pragma state drifted across re-application", tc.name)
		})
	}
}

// snapshotPragmas reads back every pragma this package sets, so the
// idempotency check is over the full preset rather than a chosen subset.
func snapshotPragmas(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, name := range []string{
		"journal_mode", "synchronous", "temp_store",
		"busy_timeout", "mmap_size", "cache_size", "wal_autocheckpoint",
	} {
		var v string
		require.NoError(t, db.QueryRow("PRAGMA "+name).Scan(&v), "PRAGMA %s read", name)
		out[name] = v
	}
	return out
}

// ---------------------------------------------------------------------------
// EnableForeignKeys (TODO.md P3)
// ---------------------------------------------------------------------------

func TestEnableForeignKeys_WhenCalled_ShouldEnforceConstraints(t *testing.T) {
	db := openFreshTempDB(t)
	ApplyDefaultPragmas(db, ProfileHot)

	_, err := db.Exec(`
		CREATE TABLE parent (id INTEGER PRIMARY KEY);
		CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent(id));
	`)
	require.NoError(t, err, "schema")

	// Before opting in, the dangling reference is accepted — that is the
	// historical behavior EnableForeignKeys exists to leave untouched by default.
	_, err = db.Exec("INSERT INTO child (id, parent_id) VALUES (1, 999)")
	require.NoError(t, err, "FK enforcement must be off until opted in")

	require.NoError(t, EnableForeignKeys(db))

	var on int
	require.NoError(t, db.QueryRow("PRAGMA foreign_keys").Scan(&on))
	require.Equal(t, 1, on, "foreign_keys should read back ON")

	_, err = db.Exec("INSERT INTO child (id, parent_id) VALUES (2, 999)")
	require.Error(t, err, "violating insert must be rejected once FKs are enforced")
	require.Contains(t, strings.ToUpper(err.Error()), "FOREIGN KEY")
}

func TestEnableForeignKeys_WhenDBIsNil_ShouldNotPanic(t *testing.T) {
	require.NoError(t, EnableForeignKeys(nil))
}

func TestApplyDefaultPragmas_WhenApplied_ShouldLeaveForeignKeysOff(t *testing.T) {
	// The omission is load-bearing: schemas with declared-but-unenforced FKs
	// would start rejecting writes against existing user data.
	for _, profile := range []PragmaProfile{ProfileHot, ProfileBulkBuild, ProfileQuery, ProfileReadOnly} {
		db := openFreshTempDB(t)
		ApplyDefaultPragmas(db, profile)

		var on int
		require.NoError(t, db.QueryRow("PRAGMA foreign_keys").Scan(&on))
		require.Equal(t, 0, on, "profile %s must not enable foreign_keys", profile)
	}
}

// ---------------------------------------------------------------------------
// Host class overrides (TODO.md P4)
// ---------------------------------------------------------------------------

func TestActiveHostClass_WhenUnset_ShouldBeWorkstation(t *testing.T) {
	t.Setenv(EnvHostClass, "")
	ClearHostClass()
	require.Equal(t, HostWorkstation, ActiveHostClass())
}

func TestSetHostClass_WhenLaptop_ShouldShrinkMemoryBudgetsOnly(t *testing.T) {
	t.Setenv(EnvHostClass, "")
	ClearHostClass()
	workstation := pragmasFor(ProfileHot)

	SetHostClass(HostLaptop)
	t.Cleanup(ClearHostClass)
	laptop := pragmasFor(ProfileHot)

	require.Len(t, laptop, len(workstation), "host class must not add or drop pragmas")
	require.Contains(t, laptop, "PRAGMA cache_size = -524288", "laptop quarters the 2 GiB Hot cache")
	require.Contains(t, laptop, "PRAGMA mmap_size = 2147483648", "laptop quarters the 8 GiB Hot mmap window")

	// Correctness/latency pragmas are identical on every host — only capacity scales.
	for i, p := range workstation {
		if strings.Contains(p, "cache_size") || strings.Contains(p, "mmap_size") {
			continue
		}
		require.Equal(t, p, laptop[i], "non-capacity pragma changed with host class")
	}
}

func TestActiveHostClass_WhenEnvSet_ShouldFollowEnv(t *testing.T) {
	ClearHostClass()
	t.Cleanup(ClearHostClass)

	t.Setenv(EnvHostClass, "micro")
	require.Equal(t, HostMicro, ActiveHostClass())
	require.Contains(t, pragmasFor(ProfileHot), "PRAGMA cache_size = -131072")

	// An explicit SetHostClass outranks the environment: config resolved by the
	// process beats an inherited shell variable.
	SetHostClass(HostWorkstation)
	require.Equal(t, HostWorkstation, ActiveHostClass())
	require.Contains(t, pragmasFor(ProfileHot), "PRAGMA cache_size = -2097152")
}

func TestActiveHostClass_WhenEnvIsGarbage_ShouldFallBackToWorkstation(t *testing.T) {
	ClearHostClass()
	t.Setenv(EnvHostClass, "laptopp")

	// A typo in an env var must never be why a database opens untuned.
	require.Equal(t, HostWorkstation, ActiveHostClass())

	_, ok := ParseHostClass("laptopp")
	require.False(t, ok, "ParseHostClass should still report the typo to a config loader")
}

func TestParseHostClass_WhenGivenAliases_ShouldResolve(t *testing.T) {
	for in, want := range map[string]HostClass{
		"":               HostWorkstation,
		"  Workstation ": HostWorkstation,
		"SERVER":         HostWorkstation,
		"laptop":         HostLaptop,
		"Notebook":       HostLaptop,
		"micro":          HostMicro,
		"container":      HostMicro,
		"ci":             HostMicro,
	} {
		got, ok := ParseHostClass(in)
		require.True(t, ok, "ParseHostClass(%q)", in)
		require.Equal(t, want, got, "ParseHostClass(%q)", in)
	}
}

// ---------------------------------------------------------------------------
// Connector hook (TODO.md P4) — every pooled connection, not just the first
// ---------------------------------------------------------------------------

func TestOpenWithPragmas_WhenPoolGrows_ShouldTuneEveryConnection(t *testing.T) {
	ClearHostClass()
	dsn := filepath.Join(t.TempDir(), "connector.db")

	db, err := OpenWithPragmas("sqlite3", dsn, ProfileHot)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	for _, got := range cacheSizeOnEachConn(t, db, 4) {
		require.Equal(t, int64(-2097152), got,
			"every connection the pool creates must carry the profile's cache_size")
	}
}

// This is the defect OpenWithPragmas exists to fix, pinned so nobody "simplifies"
// the connector away: ApplyDefaultPragmas tunes the one connection it is handed,
// and the pool's later connections come up with SQLite's defaults.
func TestApplyDefaultPragmas_WhenPoolGrows_ShouldLeaveLaterConnectionsUntuned(t *testing.T) {
	ClearHostClass()
	dsn := filepath.Join(t.TempDir(), "pool.db")

	db, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	ApplyDefaultPragmas(db, ProfileHot)

	sizes := cacheSizeOnEachConn(t, db, 4)
	untuned := 0
	for _, got := range sizes {
		if got != -2097152 {
			untuned++
		}
	}
	require.NotZero(t, untuned,
		"expected sql.Open+ApplyDefaultPragmas to leave later pool connections untuned (got %v); "+
			"if database/sql started replaying session state, OpenWithPragmas may no longer be needed", sizes)
}

// cacheSizeOnEachConn holds n connections open simultaneously — which forces
// the pool to create n distinct ones — and reads cache_size on each.
func cacheSizeOnEachConn(t *testing.T, db *sql.DB, n int) []int64 {
	t.Helper()
	ctx := context.Background()

	conns := make([]*sql.Conn, 0, n)
	t.Cleanup(func() {
		for _, c := range conns {
			_ = c.Close()
		}
	})

	out := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		c, err := db.Conn(ctx)
		require.NoError(t, err, "acquire connection %d", i)
		conns = append(conns, c)

		var v int64
		require.NoError(t, c.QueryRowContext(ctx, "PRAGMA cache_size").Scan(&v))
		out = append(out, v)
	}
	return out
}

func TestNewConnector_WhenDriverIsNil_ShouldError(t *testing.T) {
	_, err := NewConnector(nil, "x.db", ProfileHot)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Pragma failure metrics (TODO.md P4)
// ---------------------------------------------------------------------------

func TestPragmaMetrics_WhenEnabledAndPragmasFail_ShouldCountPerProfile(t *testing.T) {
	ClearHostClass()
	db := openFreshTempDB(t)
	require.NoError(t, db.Close()) // every subsequent Exec fails deterministically

	ResetPragmaMetrics()
	SetMetricsEnabled(true)
	t.Cleanup(func() {
		SetMetricsEnabled(false)
		ResetPragmaMetrics()
	})

	want := uint64(len(pragmasFor(ProfileHot)))
	ApplyDefaultPragmas(db, ProfileHot)

	require.Equal(t, want, PragmaFailureTotal())
	require.Equal(t, want, PragmaFailuresByProfile()["Hot"])
	require.Len(t, FailingPragmas(), int(want), "each distinct statement should be recorded once")
	require.Equal(t, uint64(1), PragmaFailuresByStatement()["PRAGMA journal_mode = WAL"])
}

func TestPragmaMetrics_WhenDisabled_ShouldStayZero(t *testing.T) {
	db := openFreshTempDB(t)
	require.NoError(t, db.Close())

	SetMetricsEnabled(false)
	ResetPragmaMetrics()
	t.Cleanup(ResetPragmaMetrics)

	ApplyDefaultPragmas(db, ProfileQuery)

	require.False(t, MetricsEnabled())
	require.Zero(t, PragmaFailureTotal(), "counting must be free when observability is off")
	require.Empty(t, PragmaFailuresByProfile())
}

// ---------------------------------------------------------------------------
// Profile naming (TODO.md P2: Debug log should carry the profile on failure)
// ---------------------------------------------------------------------------

func TestPragmaProfile_String_ShouldNameEveryProfile(t *testing.T) {
	require.Equal(t, "Hot", ProfileHot.String())
	require.Equal(t, "BulkBuild", ProfileBulkBuild.String())
	require.Equal(t, "Query", ProfileQuery.String())
	require.Equal(t, "ReadOnly", ProfileReadOnly.String())
	require.Equal(t, "PragmaProfile(42)", PragmaProfile(42).String(),
		"an undeclared value must still be identifiable in a log line")
}
