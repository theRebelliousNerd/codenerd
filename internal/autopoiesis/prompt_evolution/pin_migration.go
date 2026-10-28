package prompt_evolution

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"codenerd/internal/logging"

	"gopkg.in/yaml.v3"
)

// UnpinnedAtom is a stored evolved atom that broke the pinning rules and
// could not be re-pinned, because its record names no provider, model or role
// to pin it to. It is kept on disk under evolved/unpinned/, which the JIT
// loader never reads, and listed here for the operator.
type UnpinnedAtom struct {
	ID     string
	Path   string
	Reason string
}

// UnpinnedAtoms lists the quarantined-for-lack-of-a-pin atoms, oldest
// migration first.
func (pe *PromptEvolver) UnpinnedAtoms() []UnpinnedAtom {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return slices.Clone(pe.unpinned)
}

// enforceStoredPins brings atoms stored by earlier versions under the current
// pin rules. For each loaded atom, the selectors it should carry follow from
// its recorded serving pin (provider, model) and source role under the
// configured scope:
//
//   - where the record has them, the atom is re-pinned in place (exact model
//     by default, source role only) and its file rewritten;
//   - where it does not, the atom is moved to unpinned/ -- unselected, kept,
//     listed -- instead of being deleted or left serving every model.
//
// Must run before the atom set is shared (construction) or hold pe.mu.
func (pe *PromptEvolver) enforceStoredPins() {
	scope := pe.config.AtomPinScope
	ids := make([]string, 0, len(pe.evolvedAtoms))
	for id := range pe.evolvedAtoms {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	for _, id := range ids {
		ga := pe.evolvedAtoms[id]
		pin := ServingPin{Provider: ga.Provider, Model: ga.Model}
		if err := requirePin(scope, ga.ShardType, pin); err != nil {
			pe.quarantineUnpinnedLocked(ga, err.Error())
			continue
		}
		before := selectorsOf(ga.Atom)
		applyPin(ga.Atom, scope, ga.ShardType, pin)
		if before.equal(selectorsOf(ga.Atom)) && ga.PinScope == scope {
			continue
		}
		ga.PinScope = scope
		if err := pe.persistAtomLocked(ga); err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("Re-pinned %s in memory but could not rewrite it: %v", id, err)
			continue
		}
		logging.Autopoiesis("Evolved atom %s re-pinned to %s/%s role=%s (scope=%s)",
			id, strings.Join(ga.Atom.Providers, ","), strings.Join(ga.Atom.Models, ","),
			strings.Join(ga.Atom.ShardTypes, ","), scope)
	}
	pe.listUnpinnedOnDisk()
}

// quarantineUnpinnedLocked moves an atom's file to unpinned/ and drops it from
// the served set. The new copy is written before the old one is removed, so a
// crash leaves the atom present in both places rather than neither.
func (pe *PromptEvolver) quarantineUnpinnedLocked(ga *GeneratedAtom, reason string) {
	id := ga.Atom.ID
	data, err := yaml.Marshal(wrapperOf(ga))
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Cannot quarantine unpinned atom %s: %v", id, err)
		return
	}
	dest := filepath.Join(pe.unpinnedDir, atomFileName(id))
	if err := os.WriteFile(dest, data, 0644); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Cannot quarantine unpinned atom %s: %v", id, err)
		return
	}
	for _, dir := range []string{pe.promotedDir, pe.pendingDir} {
		if err := os.Remove(filepath.Join(dir, atomFileName(id))); err != nil && !os.IsNotExist(err) {
			logging.Get(logging.CategoryAutopoiesis).Warn("Quarantined %s but could not remove %s copy: %v", id, dir, err)
		}
	}
	delete(pe.evolvedAtoms, id)
	pe.unpinned = append(pe.unpinned, UnpinnedAtom{ID: id, Path: dest, Reason: reason})
	logging.Get(logging.CategoryAutopoiesis).Warn(
		"Evolved atom %s quarantined unselected in %s: %s", id, pe.unpinnedDir, reason)
}

// listUnpinnedOnDisk adds the atoms quarantined by an earlier run to the
// listing. Must hold pe.mu or run at construction.
func (pe *PromptEvolver) listUnpinnedOnDisk() {
	known := make(map[string]bool, len(pe.unpinned))
	for _, u := range pe.unpinned {
		known[u.Path] = true
	}
	entries, err := os.ReadDir(pe.unpinnedDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(pe.unpinnedDir, entry.Name())
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") || known[path] {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".yaml")
		if data, err := os.ReadFile(path); err == nil {
			var w evolvedAtomWrapper
			if yaml.Unmarshal(data, &w) == nil && w.Atom != nil {
				id = w.Atom.ID
			}
		}
		pe.unpinned = append(pe.unpinned, UnpinnedAtom{
			ID: id, Path: path, Reason: fmt.Sprintf("quarantined by an earlier run: %s", path),
		})
	}
}
