package northstar

import (
	"fmt"
	"path/filepath"
	"sync"

	"codenerd/internal/logging"
)

// =============================================================================
// PROCESS-WIDE GUARDIAN REGISTRY
// =============================================================================
//
// Every consumer used to build its own Guardian, and each Guardian opens its
// own SQLite handle on .nerd/northstar_knowledge.db:
//
//	session_boot.go        one handle, never closed, lives for the session
//	session_shared_boot.go one handle, never closed
//	model_helpers.go       a fresh handle on EVERY /alignment invocation
//	BuildCampaignObserver  one handle per campaign, never closed
//
// Two consequences, both real. First, handle leak: /alignment in a long chat
// session opens an unbounded number of connections. Second, and worse, split
// brain: each Guardian caches vision and GuardianState in memory at Initialize.
// A /alignment guardian therefore recorded checks and drift into the same rows
// the boot guardian had already cached, so the boot guardian's
// TasksSinceCheck / OverallAlignment / ActiveDriftCount went stale and its
// periodic-check scheduling drifted from what the database actually said.
//
// AcquireGuardian returns the one Guardian per .nerd directory, refcounted.
// Callers still perform their own SetLLMClient / SetParentKernel / Initialize --
// the shape of every existing call site is preserved, and Initialize is
// idempotent (it reloads from the store and re-projects kernel facts).

type guardianRegistryEntry struct {
	guardian *Guardian
	refs     int
}

var (
	guardianRegistryMu sync.Mutex
	guardianRegistry   = map[string]*guardianRegistryEntry{}
)

func guardianRegistryKey(nerdDir string) string {
	abs, err := filepath.Abs(nerdDir)
	if err != nil {
		abs = nerdDir
	}
	return filepath.Clean(abs)
}

// AcquireGuardian returns the shared Guardian for nerdDir, creating it (and its
// Store) on first use. Each successful call must be paired with ReleaseGuardian.
//
// config is applied only when the Guardian is created; a later acquirer with a
// different config gets the existing Guardian and a debug line, because two
// different threshold sets over one database would mean the same score
// classified two different ways depending on which caller asked.
func AcquireGuardian(nerdDir string, config GuardianConfig) (*Guardian, error) {
	key := guardianRegistryKey(nerdDir)

	guardianRegistryMu.Lock()
	defer guardianRegistryMu.Unlock()

	if entry, ok := guardianRegistry[key]; ok {
		entry.refs++
		normalized := NormalizeGuardianConfig(config)
		entry.guardian.mu.RLock()
		existing := entry.guardian.config
		entry.guardian.mu.RUnlock()
		if existing.WarningThreshold != normalized.WarningThreshold ||
			existing.FailureThreshold != normalized.FailureThreshold ||
			existing.BlockThreshold != normalized.BlockThreshold ||
			existing.PeriodicCheckInterval != normalized.PeriodicCheckInterval {
			logging.Get(logging.CategoryNorthstar).Debug(
				"Reusing shared Northstar Guardian for %s; requested config differs from the one in force", key)
		}
		return entry.guardian, nil
	}

	store, err := NewStore(key)
	if err != nil {
		return nil, fmt.Errorf("open northstar store for %s: %w", key, err)
	}
	guardian := NewGuardian(store, config)
	guardian.registryKey = key
	guardianRegistry[key] = &guardianRegistryEntry{guardian: guardian, refs: 1}
	logging.Get(logging.CategoryNorthstar).Debug("Opened shared Northstar Guardian for %s", key)
	return guardian, nil
}

// ReleaseGuardian drops one reference. The underlying Store is closed when the
// last reference goes away. Releasing a Guardian that did not come from
// AcquireGuardian is a no-op, so callers can release unconditionally.
func ReleaseGuardian(g *Guardian) error {
	if g == nil || g.registryKey == "" {
		return nil
	}

	guardianRegistryMu.Lock()
	defer guardianRegistryMu.Unlock()

	entry, ok := guardianRegistry[g.registryKey]
	if !ok || entry.guardian != g {
		return nil
	}
	entry.refs--
	if entry.refs > 0 {
		return nil
	}
	delete(guardianRegistry, g.registryKey)
	logging.Get(logging.CategoryNorthstar).Debug("Closed shared Northstar Guardian for %s", g.registryKey)
	return entry.guardian.store.Close()
}

// GuardianRefCount reports how many outstanding references the registry holds
// for nerdDir. Exposed for tests and for `nerd northstar state` diagnostics.
func GuardianRefCount(nerdDir string) int {
	guardianRegistryMu.Lock()
	defer guardianRegistryMu.Unlock()
	if entry, ok := guardianRegistry[guardianRegistryKey(nerdDir)]; ok {
		return entry.refs
	}
	return 0
}

// ResetGuardianRegistry closes and forgets every shared Guardian. Test-only:
// production code has no point at which discarding another caller's live
// Guardian is correct.
func ResetGuardianRegistry() {
	guardianRegistryMu.Lock()
	defer guardianRegistryMu.Unlock()
	for key, entry := range guardianRegistry {
		if entry.guardian != nil && entry.guardian.store != nil {
			_ = entry.guardian.store.Close()
		}
		delete(guardianRegistry, key)
	}
}
