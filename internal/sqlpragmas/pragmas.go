// Package sqlpragmas: SQLite pragma tuning helper.
//
// ApplyDefaultPragmas applies a curated PRAGMA preset to a freshly-opened
// SQLite database handle. The presets are tuned for a workstation-class
// host (large RAM, NVMe storage); modest hosts will still work because
// SQLite treats mmap_size and cache_size as upper bounds, not requirements.
// Hosts that want smaller budgets anyway can select a HostClass via
// SetHostClass or the NERD_SQL_HOST_CLASS environment variable.
//
// Individual PRAGMA failures are logged at Debug and do NOT fail the open, so
// a driver or platform that refuses one pragma cannot turn into a failed open.
//
// This was long justified by "the modernc.org/sqlite driver rejects a subset of
// the pragmas mattn/go-sqlite3 accepts". Nobody had measured it. The
// build-tagged test in modernc_integration_test.go now does, and at
// modernc.org/sqlite v1.50.1 the pure-Go driver accepts the entire preset
// table — the reject set is empty. The soft-fail stays because it is still the
// right shape (read-only handles, unusual filesystems, future driver versions)
// and because the test pins the reject set, so a driver that starts refusing a
// pragma fails CI instead of silently degrading pure-Go builds.
//
// # Connection pools
//
// PRAGMAs are per-connection state. ApplyDefaultPragmas runs on whichever
// pooled connection database/sql hands it, so a pool that later opens more
// connections gets untuned ones. Two supported answers:
//
//   - Pin the pool: db.SetMaxOpenConns(1) — correct for the small agent DBs
//     that dominate this codebase.
//   - Use OpenWithPragmas / NewConnector, which install a connector hook so
//     *every* connection the pool creates is tuned at birth.
//
// This package is intentionally a leaf — it depends only on the standard
// library and internal/logging — so upstream packages (mcp, autopoiesis, etc.)
// can import it without creating cycles through internal/store. That
// invariant is enforced by TestPackageImports_WhenNewImportAdded_ShouldStayLeaf,
// not by this comment; configuration reaches the package by inversion of
// control (SetHostClass, SetMetricsEnabled) rather than by importing config.
package sqlpragmas

import (
	"database/sql"
	"fmt"

	"codenerd/internal/logging"
)

// PragmaProfile selects a pragma preset matched to a workload pattern.
type PragmaProfile int

const (
	// ProfileHot is the default for long-lived agent stores
	// (LocalStore, learned store, prompt cache, MCP store, northstar, etc.).
	// 2 GB page cache, 8 GB mmap window.
	ProfileHot PragmaProfile = iota

	// ProfileBulkBuild is for bulk-write tools that create a fresh database
	// in one pass (corpus_builder, prompt_builder, predicate_corpus_builder).
	// 4 GB page cache, 16 GB mmap window, larger WAL checkpoint window.
	ProfileBulkBuild

	// ProfileQuery is for short-lived read-mostly opens
	// (query-kb, predicate corpus lookup, init validation, agent atom count).
	// 512 MB page cache, 4 GB mmap window. Keeps WAL pragmas so a writer
	// running concurrently does not block this reader.
	ProfileQuery

	// ProfileReadOnly is for databases opened with "?mode=ro" or otherwise
	// guaranteed not to be written from this handle. Avoids attempting WAL,
	// synchronous, or wal_autocheckpoint pragmas (SQLite rejects writes on
	// a read-only connection and the failures would spam Debug logs on
	// every open). Still enables mmap and a generous page cache.
	ProfileReadOnly
)

// String names the profile for logs and metrics. Failure logs carry it so an
// operator reading "pragma ... failed" can tell which preset was in play
// without going back to the call site to read the argument.
func (p PragmaProfile) String() string {
	switch p {
	case ProfileHot:
		return "Hot"
	case ProfileBulkBuild:
		return "BulkBuild"
	case ProfileQuery:
		return "Query"
	case ProfileReadOnly:
		return "ReadOnly"
	default:
		return fmt.Sprintf("PragmaProfile(%d)", int(p))
	}
}

// Byte-size units. Named so the preset table below reads as sizes rather than
// as a column of multiplication.
const (
	kib int64 = 1024
	mib       = 1024 * kib
	gib       = 1024 * mib
)

// mmap windows per profile. SQLite treats mmap_size as an upper bound: asking
// for more than the host can map is not an error, it just maps less.
const (
	mmapHot       = 8 * gib
	mmapBulkBuild = 16 * gib
	mmapQuery     = 4 * gib
	mmapReadOnly  = 4 * gib
)

// Page-cache budgets, in KiB. SQLite's cache_size is a page count when
// positive and a KiB budget when negative, so these are emitted negated.
const (
	cacheHotKiB       = 2 * 1024 * 1024 // 2 GiB
	cacheBulkBuildKiB = 4 * 1024 * 1024 // 4 GiB
	cacheQueryKiB     = 512 * 1024      // 512 MiB
	cacheReadOnlyKiB  = 512 * 1024      // 512 MiB
)

// Timing knobs shared across the writable profiles.
const (
	// busyTimeoutMS is how long a connection waits on a locked DB before
	// returning SQLITE_BUSY. Agent stores see bursty concurrent writers.
	busyTimeoutMS = 10000

	// walCheckpointHot / walCheckpointBulk are WAL frame thresholds. Builders
	// get the larger window so a one-pass build fsyncs fewer times.
	walCheckpointHot   = 10000
	walCheckpointBulk  = 20000
	walCheckpointUnset = 0 // profile omits the pragma entirely
)

// ApplyDefaultPragmas applies the pragma preset for the given profile to db.
// Per-pragma failures are logged at Debug; the function never returns an
// error and never closes db.
//
// Callers should invoke this once, right after sql.Open(), before any
// schema initialization or first query, so the connection pool's first
// real connection is already tuned. For pools larger than one connection,
// prefer OpenWithPragmas so later connections are tuned too.
func ApplyDefaultPragmas(db *sql.DB, profile PragmaProfile) {
	if db == nil {
		return
	}
	logger := logging.Get(logging.CategoryStore)
	for _, p := range pragmasFor(profile) {
		if _, err := db.Exec(p); err != nil {
			recordPragmaFailure(profile, p)
			logger.Debug("pragma %q failed (profile %s): %v", p, profile, err)
		}
	}
}

// EnableForeignKeys turns on FK enforcement for db, returning the driver error
// if the pragma is rejected.
//
// This is deliberately NOT part of any profile: several schemas (northstar,
// strategies, prompt atoms) declare FOREIGN KEY constraints but have always
// run unenforced, so switching them on globally would start rejecting writes
// against existing user data. A schema opts in only once its own data is known
// clean — hence the separate call and, unlike ApplyDefaultPragmas, a returned
// error: a caller asking for FK enforcement wants to know if it did not happen.
//
// PRAGMA foreign_keys is per-connection and is a no-op inside a transaction.
// On a multi-connection pool, use OpenWithPragmas or pin MaxOpenConns to 1.
func EnableForeignKeys(db *sql.DB) error {
	if db == nil {
		return nil
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("enable foreign_keys: %w", err)
	}
	// SQLite silently ignores the pragma when the build omits FK support, so
	// confirm rather than trust the Exec.
	var on int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&on); err != nil {
		return fmt.Errorf("verify foreign_keys: %w", err)
	}
	if on != 1 {
		return fmt.Errorf("foreign_keys still off after PRAGMA (driver or build lacks FK support)")
	}
	return nil
}

// pragmasFor returns the ordered list of PRAGMA statements for a profile,
// scaled for the active host class.
func pragmasFor(profile PragmaProfile) []string {
	return pragmasForHost(profile, ActiveHostClass())
}

// pragmasForHost returns the ordered list of PRAGMA statements for a profile
// at a given host class. Order matters: journal_mode must be set before
// pragmas that depend on WAL semantics (wal_autocheckpoint, synchronous=NORMAL
// safety claims).
//
// Note: PRAGMA foreign_keys is intentionally omitted from defaults; see
// EnableForeignKeys for the reasoning and the opt-in path.
func pragmasForHost(profile PragmaProfile, host HostClass) []string {
	var (
		mmap       int64
		cacheKiB   int64
		checkpoint int
		writable   = true
	)

	switch profile {
	case ProfileBulkBuild:
		mmap, cacheKiB, checkpoint = mmapBulkBuild, cacheBulkBuildKiB, walCheckpointBulk
	case ProfileQuery:
		mmap, cacheKiB, checkpoint = mmapQuery, cacheQueryKiB, walCheckpointUnset
	case ProfileReadOnly:
		mmap, cacheKiB, checkpoint, writable = mmapReadOnly, cacheReadOnlyKiB, walCheckpointUnset, false
	default: // ProfileHot, and any future value that has not been given a case
		mmap, cacheKiB, checkpoint = mmapHot, cacheHotKiB, walCheckpointHot
	}

	mmap = host.scale(mmap)
	cacheKiB = host.scale(cacheKiB)

	out := make([]string, 0, 7)
	if writable {
		out = append(out,
			"PRAGMA journal_mode = WAL",
			"PRAGMA synchronous = NORMAL",
		)
	}
	out = append(out,
		"PRAGMA temp_store = MEMORY",
		fmt.Sprintf("PRAGMA busy_timeout = %d", busyTimeoutMS),
		fmt.Sprintf("PRAGMA mmap_size = %d", mmap),
		fmt.Sprintf("PRAGMA cache_size = -%d", cacheKiB),
	)
	if writable && checkpoint != walCheckpointUnset {
		out = append(out, fmt.Sprintf("PRAGMA wal_autocheckpoint = %d", checkpoint))
	}
	return out
}
