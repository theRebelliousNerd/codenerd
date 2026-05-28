// Package sqlpragmas: SQLite pragma tuning helper.
//
// ApplyDefaultPragmas applies a curated PRAGMA preset to a freshly-opened
// SQLite database handle. The presets are tuned for a workstation-class
// host (large RAM, NVMe storage); modest hosts will still work because
// SQLite treats mmap_size and cache_size as upper bounds, not requirements.
//
// Individual PRAGMA failures are logged at Debug and do NOT fail the open.
// This matches the existing pattern across the codebase (the modernc.org/sqlite
// driver rejects a small subset of pragmas that the mattn/go-sqlite3 driver
// accepts, and we want both drivers to coexist without spam).
//
// This package is intentionally a leaf — it depends only on database/sql,
// fmt, and internal/logging — so upstream packages (mcp, autopoiesis, etc.)
// can import it without creating cycles through internal/store.
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

// ApplyDefaultPragmas applies the pragma preset for the given profile to db.
// Per-pragma failures are logged at Debug; the function never returns an
// error and never closes db.
//
// Callers should invoke this once, right after sql.Open(), before any
// schema initialization or first query, so the connection pool's first
// real connection is already tuned.
func ApplyDefaultPragmas(db *sql.DB, profile PragmaProfile) {
	if db == nil {
		return
	}
	logger := logging.Get(logging.CategoryStore)
	for _, p := range pragmasFor(profile) {
		if _, err := db.Exec(p); err != nil {
			logger.Debug("pragma %q failed: %v", p, err)
		}
	}
}

// pragmasFor returns the ordered list of PRAGMA statements for a profile.
// Order matters: journal_mode must be set before pragmas that depend on
// WAL semantics (wal_autocheckpoint, synchronous=NORMAL safety claims).
//
// Note: PRAGMA foreign_keys is intentionally omitted from defaults. Several
// schemas declare FOREIGN KEY constraints (northstar, strategies, prompt
// atoms) but historically ran without enforcement; enabling FK checks here
// would be a behavior change against existing user data. Sites that want
// FK enforcement should call db.Exec("PRAGMA foreign_keys = ON") themselves.
func pragmasFor(profile PragmaProfile) []string {
	switch profile {

	case ProfileBulkBuild:
		return []string{
			"PRAGMA journal_mode = WAL",
			"PRAGMA synchronous = NORMAL",
			"PRAGMA temp_store = MEMORY",
			"PRAGMA busy_timeout = 10000",
			fmt.Sprintf("PRAGMA mmap_size = %d", 16*1024*1024*1024), // 16 GB
			"PRAGMA cache_size = -4194304",                          // 4 GB (negative = KiB)
			"PRAGMA wal_autocheckpoint = 20000",
		}

	case ProfileQuery:
		return []string{
			"PRAGMA journal_mode = WAL",
			"PRAGMA synchronous = NORMAL",
			"PRAGMA temp_store = MEMORY",
			"PRAGMA busy_timeout = 10000",
			fmt.Sprintf("PRAGMA mmap_size = %d", 4*1024*1024*1024), // 4 GB
			"PRAGMA cache_size = -524288",                          // 512 MB
		}

	case ProfileReadOnly:
		return []string{
			"PRAGMA temp_store = MEMORY",
			"PRAGMA busy_timeout = 10000",
			fmt.Sprintf("PRAGMA mmap_size = %d", 4*1024*1024*1024), // 4 GB
			"PRAGMA cache_size = -524288",                          // 512 MB
		}

	default: // ProfileHot
		return []string{
			"PRAGMA journal_mode = WAL",
			"PRAGMA synchronous = NORMAL",
			"PRAGMA temp_store = MEMORY",
			"PRAGMA busy_timeout = 10000",
			fmt.Sprintf("PRAGMA mmap_size = %d", 8*1024*1024*1024), // 8 GB
			"PRAGMA cache_size = -2097152",                         // 2 GB
			"PRAGMA wal_autocheckpoint = 10000",
		}
	}
}
