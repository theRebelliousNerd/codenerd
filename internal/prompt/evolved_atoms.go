package prompt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/logging"

	"gopkg.in/yaml.v3"
)

// EvolvedLifecycleState is where a learned candidate stands. Directory
// membership decides it: pending/ is quarantine, promoted/ is deployed.
// Rejected/ is never loaded.
//
// Pending used to be a storage category and nothing more: Reload loaded
// pending and promoted alike and GetMatching served both, so a candidate
// that nobody had reviewed (AutoPromote defaults off precisely so that a
// human reviews it) reached ordinary production selection anyway. The state
// is now retained on load and decides admission.
type EvolvedLifecycleState string

const (
	// EvolvedQuarantined is a generated candidate awaiting review. It never
	// reaches a normal compile; it reaches a trial compile only when the
	// kernel derives prompt_trial_admitted for its exact version.
	EvolvedQuarantined EvolvedLifecycleState = "quarantined"

	// EvolvedPromoted is a reviewed candidate deployed to every matching
	// compile.
	EvolvedPromoted EvolvedLifecycleState = "promoted"
)

// EvolvedCandidate is one loaded candidate with the lifecycle metadata its
// wrapper file carried, not just the atom body.
type EvolvedCandidate struct {
	Atom *PromptAtom
	// State is quarantined or promoted, from the directory it was loaded from.
	State EvolvedLifecycleState
	// Version is EvolvedCandidateHash of the loaded atom: the identity a trial
	// envelope pins and an attribution credits.
	Version string
	// Provenance from the wrapper.
	Source      string
	SourceIDs   []string
	ProblemType string
	CreatedAt   time.Time
	PromotedAt  time.Time
}

// EvolvedCandidateHash identifies one exact version of a candidate: its body
// and every field that decides where and how it is served. A trial envelope
// pins this value, so editing a candidate's content or widening its
// selectors after approval ends its admission instead of inheriting it.
//
// Computed over a normalized copy, so the generator (before normalization)
// and the loader (after) agree on the same file. Derived fields (token count,
// content hash, creation time) are excluded because the loader backfills them.
func EvolvedCandidateHash(atom *PromptAtom) string {
	if atom == nil {
		return ""
	}
	c := *atom
	c.OperationalModes = slices.Clone(atom.OperationalModes)
	c.CampaignPhases = slices.Clone(atom.CampaignPhases)
	c.BuildLayers = slices.Clone(atom.BuildLayers)
	c.InitPhases = slices.Clone(atom.InitPhases)
	c.NorthstarPhases = slices.Clone(atom.NorthstarPhases)
	c.OuroborosStages = slices.Clone(atom.OuroborosStages)
	c.IntentVerbs = slices.Clone(atom.IntentVerbs)
	c.ShardTypes = slices.Clone(atom.ShardTypes)
	c.Languages = slices.Clone(atom.Languages)
	c.Frameworks = slices.Clone(atom.Frameworks)
	c.Models = slices.Clone(atom.Models)
	c.Providers = slices.Clone(atom.Providers)
	c.NormalizeSelectors()
	sorted := func(v []string) []string {
		v = slices.Clone(v)
		slices.Sort(v)
		return v
	}
	identity := struct {
		ID, Category, Subcategory           string
		Content, ContentConcise, ContentMin string
		Selectors                           [][]string
		Priority                            int
		IsMandatory                         bool
		IsExclusive                         string
		DependsOn, ConflictsWith, Requires  []string
	}{
		ID: c.ID, Category: string(c.Category), Subcategory: c.Subcategory,
		Content: c.Content, ContentConcise: c.ContentConcise, ContentMin: c.ContentMin,
		Selectors: [][]string{
			sorted(c.OperationalModes), sorted(c.CampaignPhases), sorted(c.BuildLayers),
			sorted(c.InitPhases), sorted(c.NorthstarPhases), sorted(c.OuroborosStages),
			sorted(c.IntentVerbs), sorted(c.ShardTypes), sorted(c.Languages),
			sorted(c.Frameworks), sorted(c.Models), sorted(c.Providers), sorted(c.WorldStates),
		},
		Priority: c.Priority, IsMandatory: c.IsMandatory, IsExclusive: c.IsExclusive,
		DependsOn: sorted(c.DependsOn), ConflictsWith: sorted(c.ConflictsWith), Requires: sorted(c.RequiresTools),
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return ""
	}
	return HashContent(string(encoded))
}

// PromptTrialAdmittedPredicate is the kernel predicate that admits one exact
// candidate version to a trial compile (policy/prompt_trials.mg). Go
// materializes the operator-approved envelope; the kernel decides admission.
const PromptTrialAdmittedPredicate = "prompt_trial_admitted"

// EvolvedAtomManager manages evolved atoms for JIT compilation.
// This integrates with the prompt_evolution package to provide
// learned atoms as an additional source for the JIT compiler.
type EvolvedAtomManager struct {
	mu sync.RWMutex

	// Directory containing evolved atoms
	evolvedDir string

	// Loaded candidates, quarantined and promoted, in load order.
	candidates []*EvolvedCandidate
	byID       map[string]*EvolvedCandidate

	// generation advances after every Reload, once the new set is visible.
	// The JIT compiler folds it into its source vector so a prompt compiled
	// from one evolved set is never served as current for another.
	generation atomic.Uint64
}

// NewEvolvedAtomManager creates a new evolved atom manager.
func NewEvolvedAtomManager(nerdDir string) *EvolvedAtomManager {
	eam := &EvolvedAtomManager{
		evolvedDir: filepath.Join(nerdDir, "prompts", "evolved"),
		byID:       make(map[string]*EvolvedCandidate),
	}
	eam.Reload()
	return eam
}

// Reload reloads evolved atoms from disk.
func (eam *EvolvedAtomManager) Reload() error {
	eam.mu.Lock()
	defer eam.mu.Unlock()

	eam.candidates = nil
	eam.byID = make(map[string]*EvolvedCandidate)

	// Promoted first: when one ID sits in both directories (a crash between
	// a promotion's write and its removal), the reviewed copy wins.
	eam.loadFromDir(filepath.Join(eam.evolvedDir, "promoted"), EvolvedPromoted)
	eam.loadFromDir(filepath.Join(eam.evolvedDir, "pending"), EvolvedQuarantined)

	// Advance after the new set is in place (still under the write lock), so
	// a reader that observes the new generation also observes the new atoms.
	eam.generation.Add(1)
	logging.JITDebug("Evolved atoms reloaded: count=%d", len(eam.candidates))
	return nil
}

// Generation identifies the in-memory evolved set. It changes on every
// Reload; two reads returning the same value saw the same candidates.
func (eam *EvolvedAtomManager) Generation() uint64 {
	if eam == nil {
		return 0
	}
	return eam.generation.Load()
}

// loadFromDir loads candidates from a directory.
func (eam *EvolvedAtomManager) loadFromDir(dir string, state EvolvedLifecycleState) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // Directory might not exist yet
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		candidate := eam.loadCandidateFromFile(filepath.Join(dir, entry.Name()), state)
		if candidate == nil {
			continue
		}
		if _, dup := eam.byID[candidate.Atom.ID]; dup {
			continue
		}
		eam.candidates = append(eam.candidates, candidate)
		eam.byID[candidate.Atom.ID] = candidate
	}
}

// loadCandidateFromFile loads a candidate from a YAML wrapper file.
//
// Evolved files use a wrapper shape the canonical AtomDefinition parser does
// not speak, so this path unmarshals PromptAtom directly and then applies
// the canonical post-conditions by hand: structural validation, selector
// normalization, and computed-field backfill. A corrupt file is skipped with
// a warning -- the evolved directory is a derived cache, and one bad file
// must not fail compilation -- but never silently.
func (eam *EvolvedAtomManager) loadCandidateFromFile(path string, state EvolvedLifecycleState) *EvolvedCandidate {
	data, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryJIT).Warn("Evolved atom %s unreadable: %v (skipped)", path, err)
		return nil
	}

	var wrapper struct {
		Atom        *PromptAtom `yaml:"atom"`
		Source      string      `yaml:"source"`
		SourceIDs   []string    `yaml:"source_ids"`
		ProblemType string      `yaml:"problem_type"`
		CreatedAt   time.Time   `yaml:"created_at"`
		PromotedAt  time.Time   `yaml:"promoted_at"`
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
	return &EvolvedCandidate{
		Atom:        atom,
		State:       state,
		Version:     EvolvedCandidateHash(atom),
		Source:      wrapper.Source,
		SourceIDs:   wrapper.SourceIDs,
		ProblemType: wrapper.ProblemType,
		CreatedAt:   wrapper.CreatedAt,
		PromotedAt:  wrapper.PromotedAt,
	}
}

// GetAll returns every loaded candidate atom, quarantined or promoted. It is
// an inventory, never a serving path.
func (eam *EvolvedAtomManager) GetAll() []*PromptAtom {
	eam.mu.RLock()
	defer eam.mu.RUnlock()
	result := make([]*PromptAtom, 0, len(eam.candidates))
	for _, c := range eam.candidates {
		result = append(result, c.Atom)
	}
	return result
}

// Candidates returns a snapshot of the loaded candidates with their state.
func (eam *EvolvedAtomManager) Candidates() []EvolvedCandidate {
	eam.mu.RLock()
	defer eam.mu.RUnlock()
	result := make([]EvolvedCandidate, 0, len(eam.candidates))
	for _, c := range eam.candidates {
		result = append(result, *c)
	}
	return result
}

// GetMatching returns the evolved atoms admitted to this compilation.
//
// Promoted candidates are admitted on context match alone. A quarantined
// candidate is admitted only when trialAdmitted contains its exact version,
// and then as a non-mandatory copy: a trial competes for budget like any
// optional atom and can never enter the mandatory skeleton. Admission grants
// no tool and no permission; RequiresTools still gates selection downstream.
func (eam *EvolvedAtomManager) GetMatching(cc *CompilationContext, trialAdmitted map[string]string) []*PromptAtom {
	eam.mu.RLock()
	defer eam.mu.RUnlock()

	var matching []*PromptAtom
	for _, c := range eam.candidates {
		if !c.Atom.MatchesContext(cc) {
			continue
		}
		switch c.State {
		case EvolvedPromoted:
			matching = append(matching, c.Atom)
		case EvolvedQuarantined:
			if version, ok := trialAdmitted[c.Atom.ID]; ok && version != "" && version == c.Version {
				trial := *c.Atom
				trial.IsMandatory = false
				matching = append(matching, &trial)
			}
		}
	}
	return matching
}

// Get returns an atom by ID, whatever its state.
func (eam *EvolvedAtomManager) Get(id string) *PromptAtom {
	eam.mu.RLock()
	defer eam.mu.RUnlock()
	if c := eam.byID[id]; c != nil {
		return c.Atom
	}
	return nil
}

// State returns a loaded candidate's lifecycle state.
func (eam *EvolvedAtomManager) State(id string) (EvolvedLifecycleState, bool) {
	eam.mu.RLock()
	defer eam.mu.RUnlock()
	if c := eam.byID[id]; c != nil {
		return c.State, true
	}
	return "", false
}

// Count returns the number of loaded candidates.
func (eam *EvolvedAtomManager) Count() int {
	eam.mu.RLock()
	defer eam.mu.RUnlock()
	return len(eam.candidates)
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

// collectEvolvedAtoms gathers the evolved atoms admitted to this compile.
//
// Trial admission is read from cc.Kernel, which inside compile is the
// compile's private kernel scope: the same snapshot whose fingerprint is in
// the cache identity, so a trial that expires or exhausts its exposures
// changes the identity instead of being served from cache. No kernel, or a
// failed query, admits no trial (fail closed); promoted atoms are unaffected.
func (c *JITPromptCompiler) collectEvolvedAtoms(cc *CompilationContext) []*PromptAtom {
	c.dbMu.RLock()
	mgr := c.evolvedAtomMgr
	c.dbMu.RUnlock()

	if mgr == nil || cc == nil {
		return nil
	}
	kernel, _ := cc.Kernel.(KernelQuerier)
	return mgr.GetMatching(cc, trialAdmissions(kernel))
}

// trialAdmissions reads prompt_trial_admitted(AtomID, Version) rows.
func trialAdmissions(kernel KernelQuerier) map[string]string {
	if kernel == nil {
		return nil
	}
	facts, err := kernel.Query(PromptTrialAdmittedPredicate)
	if err != nil {
		logging.Get(logging.CategoryJIT).Debug("Trial admission query failed; no trial candidate admitted: %v", err)
		return nil
	}
	admitted := make(map[string]string, len(facts))
	for _, fact := range facts {
		if len(fact.Args) < 2 {
			continue
		}
		id, err1 := extractStringArgFast(fact.Args[0])
		version, err2 := extractStringArgFast(fact.Args[1])
		if err1 != nil || err2 != nil || id == "" || version == "" {
			continue
		}
		admitted[id] = version
	}
	return admitted
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
