// Package store: re-exports of the sqlpragmas leaf package.
//
// The actual pragma implementation lives in codenerd/internal/sqlpragmas
// (a deliberately cycle-free leaf), but the call sites across the
// codebase reach for it as store.ApplyDefaultPragmas / store.ProfileXxx.
// This file forwards those names so the existing surface keeps working
// without forcing every caller to import sqlpragmas directly.
//
// Packages that already have an import-cycle relationship with store
// (e.g. internal/mcp) should import sqlpragmas directly instead.
package store

import "codenerd/internal/sqlpragmas"

// PragmaProfile re-exports sqlpragmas.PragmaProfile so callers can refer
// to it as store.PragmaProfile.
type PragmaProfile = sqlpragmas.PragmaProfile

// Pragma profile presets, re-exported from sqlpragmas. See the
// definitions there for the per-profile cache_size / mmap_size /
// synchronous / wal tuning.
const (
	ProfileHot       = sqlpragmas.ProfileHot
	ProfileBulkBuild = sqlpragmas.ProfileBulkBuild
	ProfileQuery     = sqlpragmas.ProfileQuery
	ProfileReadOnly  = sqlpragmas.ProfileReadOnly
)

// ApplyDefaultPragmas re-exports sqlpragmas.ApplyDefaultPragmas. See
// that function for the contract (Debug-logged per-pragma failures,
// never returns an error, never closes db).
var ApplyDefaultPragmas = sqlpragmas.ApplyDefaultPragmas
