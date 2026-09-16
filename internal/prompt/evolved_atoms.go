package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"codenerd/internal/logging"

	"gopkg.in/yaml.v3"
)

// EvolvedAtomManager manages evolved atoms for JIT compilation.
// This integrates with the prompt_evolution package to provide
// learned atoms as an additional source for the JIT compiler.
type EvolvedAtomManager struct {
	mu sync.RWMutex

	// Directory containing evolved atoms
	evolvedDir string

	// Cached atoms (loaded from pending + promoted directories)
	atoms     []*PromptAtom
	atomsByID map[string]*PromptAtom

	// Last refresh time for cache invalidation
	lastRefresh int64
}

// NewEvolvedAtomManager creates a new evolved atom manager.
func NewEvolvedAtomManager(nerdDir string) *EvolvedAtomManager {
	evolvedDir := filepath.Join(nerdDir, "prompts", "evolved")

	eam := &EvolvedAtomManager{
		evolvedDir: evolvedDir,
		atoms:      make([]*PromptAtom, 0),
		atomsByID:  make(map[string]*PromptAtom),
	}

	// Initial load
	eam.Reload()

	return eam
}

// Reload reloads evolved atoms from disk.
func (eam *EvolvedAtomManager) Reload() error {
	eam.mu.Lock()
	defer eam.mu.Unlock()

	// Clear existing
	eam.atoms = make([]*PromptAtom, 0)
	eam.atomsByID = make(map[string]*PromptAtom)

	// Load from pending directory (atoms awaiting promotion)
	pendingDir := filepath.Join(eam.evolvedDir, "pending")
	eam.loadFromDir(pendingDir)

	// Load from promoted directory (confirmed atoms)
	promotedDir := filepath.Join(eam.evolvedDir, "promoted")
	eam.loadFromDir(promotedDir)

	logging.JITDebug("Evolved atoms reloaded: count=%d", len(eam.atoms))
	return nil
}

// loadFromDir loads atoms from a directory.
func (eam *EvolvedAtomManager) loadFromDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // Directory might not exist yet
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		atom := eam.loadAtomFromFile(path)
		if atom != nil {
			eam.atoms = append(eam.atoms, atom)
			eam.atomsByID[atom.ID] = atom
		}
	}
}

// loadAtomFromFile loads an atom from a YAML file.
//
// Evolved files use a wrapper shape the canonical AtomDefinition parser does
// not speak, so this path unmarshals PromptAtom directly and then applies
// the canonical post-conditions by hand: structural validation, selector
// normalization, and computed-field backfill. A corrupt file is skipped with
// a warning -- the evolved directory is a derived cache, and one bad file
// must not fail compilation -- but never silently.
func (eam *EvolvedAtomManager) loadAtomFromFile(path string) *PromptAtom {
	data, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryJIT).Warn("Evolved atom %s unreadable: %v (skipped)", path, err)
		return nil
	}

	// The evolved atom files have a wrapper structure
	var wrapper struct {
		Atom *PromptAtom `yaml:"atom"`
	}

	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		logging.Get(logging.CategoryJIT).Warn("Evolved atom %s unparseable: %v (skipped)", path, err)
		return nil
	}
	if wrapper.Atom == nil {
		logging.Get(logging.CategoryJIT).Warn("Evolved atom %s has no atom body (skipped)", path)
		return nil
	}

	atom := wrapper.Atom
	if err := atom.Validate(); err != nil {
		logging.Get(logging.CategoryJIT).Warn("Evolved atom %s invalid: %v (skipped)", path, err)
		return nil
	}
	atom.NormalizeSelectors()
	if atom.TokenCount <= 0 {
		atom.TokenCount = EstimateTokens(atom.Content)
	}
	if atom.ContentHash == "" {
		atom.ContentHash = HashContent(atom.Content)
	}
	return atom
}

// GetAll returns all evolved atoms.
func (eam *EvolvedAtomManager) GetAll() []*PromptAtom {
	eam.mu.RLock()
	defer eam.mu.RUnlock()

	// Return a copy to prevent modification
	result := make([]*PromptAtom, len(eam.atoms))
	copy(result, eam.atoms)
	return result
}

// GetMatching returns evolved atoms that match the compilation context.
func (eam *EvolvedAtomManager) GetMatching(cc *CompilationContext) []*PromptAtom {
	eam.mu.RLock()
	defer eam.mu.RUnlock()

	var matching []*PromptAtom
	for _, atom := range eam.atoms {
		if atom.MatchesContext(cc) {
			matching = append(matching, atom)
		}
	}

	return matching
}

// Get returns an atom by ID.
func (eam *EvolvedAtomManager) Get(id string) *PromptAtom {
	eam.mu.RLock()
	defer eam.mu.RUnlock()

	return eam.atomsByID[id]
}

// Count returns the number of evolved atoms.
func (eam *EvolvedAtomManager) Count() int {
	eam.mu.RLock()
	defer eam.mu.RUnlock()

	return len(eam.atoms)
}

// =============================================================================
// JITPromptCompiler integration
// =============================================================================

// RegisterEvolvedAtomManager registers an evolved atom manager with the compiler.
func (c *JITPromptCompiler) RegisterEvolvedAtomManager(eam *EvolvedAtomManager) {
	c.dbMu.Lock()
	defer c.dbMu.Unlock()

	c.evolvedAtomMgr = eam
	logging.JIT("Registered evolved atom manager: %d atoms", eam.Count())
}

// collectEvolvedAtoms gathers evolved atoms matching the context.
func (c *JITPromptCompiler) collectEvolvedAtoms(cc *CompilationContext) []*PromptAtom {
	c.dbMu.RLock()
	defer c.dbMu.RUnlock()

	if c.evolvedAtomMgr == nil {
		return nil
	}

	return c.evolvedAtomMgr.GetMatching(cc)
}

// ReloadEvolvedAtoms reloads evolved atoms from disk.
func (c *JITPromptCompiler) ReloadEvolvedAtoms() error {
	c.dbMu.RLock()
	mgr := c.evolvedAtomMgr
	c.dbMu.RUnlock()

	if mgr == nil {
		return nil
	}

	return mgr.Reload()
}

// RefreshEvolvedAtoms reloads evolved atoms and clears the prompt cache so
// subsequent compilations see the updated corpus immediately.
func (c *JITPromptCompiler) RefreshEvolvedAtoms() error {
	if err := c.ReloadEvolvedAtoms(); err != nil {
		return err
	}

	c.clearPromptCache("evolved atoms refreshed")
	return nil
}

// GetEvolvedAtomCount returns the number of evolved atoms loaded.
func (c *JITPromptCompiler) GetEvolvedAtomCount() int {
	c.dbMu.RLock()
	defer c.dbMu.RUnlock()

	if c.evolvedAtomMgr == nil {
		return 0
	}

	return c.evolvedAtomMgr.Count()
}
