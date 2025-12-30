package system

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/world"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// HolographicCodeScope wraps world.FileScope and ensures deep (Cartographer) facts are
// incrementally maintained in the kernel for the current in-scope files.
//
// Why this exists:
// - core.VirtualStore cannot import world (cycle), but policies depend on deep facts like
//   code_defines/5 and code_calls/2 ("Holographic Retrieval").
// - world.EnsureDeepFacts already supports caching + retraction, but had no wiring.
type HolographicCodeScope struct {
	scope       *world.FileScope
	kernel      *core.RealKernel
	localDB     *store.LocalStore
	deepWorkers int

	mu        sync.Mutex
	memCache  map[string]deepCacheEntry
	cartograph *world.Cartographer
}

type deepCacheEntry struct {
	fingerprint string
	facts       []core.Fact
}

// NewHolographicCodeScope constructs a CodeScope that keeps deep facts in sync.
// deepWorkers <= 0 uses a small CPU-based default.
func NewHolographicCodeScope(projectRoot string, kernel *core.RealKernel, localDB *store.LocalStore, deepWorkers int) *HolographicCodeScope {
	if deepWorkers <= 0 {
		deepWorkers = runtime.NumCPU()
		if deepWorkers > 8 {
			deepWorkers = 8
		}
		if deepWorkers < 2 {
			deepWorkers = 2
		}
	}
	return &HolographicCodeScope{
		scope:       world.NewFileScope(projectRoot),
		kernel:      kernel,
		localDB:     localDB,
		deepWorkers: deepWorkers,
		memCache:    make(map[string]deepCacheEntry),
		cartograph:  world.NewCartographer(),
	}
}

// Open opens a file and loads its 1-hop dependency scope, then ensures deep facts.
func (h *HolographicCodeScope) Open(path string) error {
	if err := h.scope.Open(path); err != nil {
		return err
	}
	h.ensureDeepFacts(context.Background(), h.scope.GetInScopeFiles())
	return nil
}

// Refresh re-parses all in-scope files and ensures deep facts.
func (h *HolographicCodeScope) Refresh() error {
	if err := h.scope.Refresh(); err != nil {
		return err
	}
	h.ensureDeepFacts(context.Background(), h.scope.GetInScopeFiles())
	return nil
}

// Close clears the current scope.
func (h *HolographicCodeScope) Close() { h.scope.Close() }

func (h *HolographicCodeScope) GetCoreElement(ref string) *core.CodeElement {
	return h.scope.GetCoreElement(ref)
}

func (h *HolographicCodeScope) GetElementBody(ref string) string { return h.scope.GetElementBody(ref) }

func (h *HolographicCodeScope) GetCoreElementsByFile(path string) []core.CodeElement {
	return h.scope.GetCoreElementsByFile(path)
}

func (h *HolographicCodeScope) IsInScope(path string) bool { return h.scope.IsInScope(path) }

func (h *HolographicCodeScope) ScopeFacts() []core.Fact { return h.scope.ScopeFacts() }

func (h *HolographicCodeScope) GetActiveFile() string { return h.scope.GetActiveFile() }

func (h *HolographicCodeScope) GetInScopeFiles() []string { return h.scope.GetInScopeFiles() }

func (h *HolographicCodeScope) VerifyFileHash(path string) (bool, error) { return h.scope.VerifyFileHash(path) }

func (h *HolographicCodeScope) RefreshWithRetry(maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := h.Refresh(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func (h *HolographicCodeScope) ensureDeepFacts(ctx context.Context, paths []string) {
	if h.kernel == nil || len(paths) == 0 {
		return
	}

	if h.localDB != nil {
		res, err := world.EnsureDeepFacts(ctx, paths, h.localDB, h.deepWorkers)
		if err != nil || res == nil {
			if err != nil {
				logging.Get(logging.CategoryWorld).Warn("Deep scan failed: %v", err)
			}
			return
		}
		if len(res.RetractFacts) > 0 {
			_ = h.kernel.RetractExactFactsBatch(res.RetractFacts)
		}
		if len(res.NewFacts) > 0 {
			_ = h.kernel.AssertBatch(res.NewFacts)
		}
		return
	}

	// No LocalStore available (still keep deep facts consistent within this session).
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, path := range paths {
		if filepath.Ext(path) != ".go" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		fp := fmt.Sprintf("%d:%d", info.Size(), info.ModTime().Unix())

		if prev, ok := h.memCache[path]; ok && prev.fingerprint == fp {
			continue
		}

		var oldFacts []core.Fact
		if prev, ok := h.memCache[path]; ok {
			oldFacts = prev.facts
		}

		newFacts, err := h.cartograph.MapFile(path)
		if err != nil {
			continue
		}

		if len(oldFacts) > 0 {
			_ = h.kernel.RetractExactFactsBatch(oldFacts)
		}
		if len(newFacts) > 0 {
			_ = h.kernel.AssertBatch(newFacts)
		}

		h.memCache[path] = deepCacheEntry{
			fingerprint: fp,
			facts:       newFacts,
		}
	}
}
