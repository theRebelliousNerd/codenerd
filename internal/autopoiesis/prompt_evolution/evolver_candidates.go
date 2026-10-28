package prompt_evolution

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"

	"gopkg.in/yaml.v3"
)

// Candidate lifecycle, as directories under .nerd/prompts/evolved:
//
//	generated -> pending/ (quarantined) -> [approved trial] -> promoted/
//	                                    \-> rejected/
//
// A quarantined candidate is never served to a normal compile. An approved
// trial is not a directory: it is an envelope in the evidence ledger,
// materialized as kernel facts, from which the kernel derives admission of
// that one exact version. Promotion and rejection end any trial.

// evolvedAtomWrapper is the on-disk shape of a candidate file.
type evolvedAtomWrapper struct {
	Atom         *prompt.PromptAtom `yaml:"atom"`
	Source       string             `yaml:"source"`
	SourceIDs    []string           `yaml:"source_ids"`
	ShardType    string             `yaml:"shard_type"`
	ProblemType  string             `yaml:"problem_type"`
	Provider     string             `yaml:"provider,omitempty"`
	Model        string             `yaml:"model,omitempty"`
	PinScope     PinScope           `yaml:"pin_scope,omitempty"`
	Confidence   float64            `yaml:"confidence"`
	UsageCount   int                `yaml:"usage_count"`
	SuccessCount int                `yaml:"success_count"`
	CreatedAt    time.Time          `yaml:"created_at"`
	PromotedAt   time.Time          `yaml:"promoted_at,omitempty"`
}

// storeEvolvedAtom saves a candidate and, once the write succeeds, adds it to
// the in-memory set. Must hold pe.mu.
func (pe *PromptEvolver) storeEvolvedAtom(ga *GeneratedAtom) error {
	if err := pe.persistAtomLocked(ga); err != nil {
		return err
	}
	pe.evolvedAtoms[ga.Atom.ID] = ga
	logging.Autopoiesis("Evolved atom stored: id=%s", ga.Atom.ID)
	return nil
}

// persistAtomLocked writes a candidate's file in the directory its state
// names. The counts written are a display cache of the ledger.
func (pe *PromptEvolver) persistAtomLocked(ga *GeneratedAtom) error {
	path := pe.atomStoragePathLocked(ga.Atom.ID, !ga.PromotedAt.IsZero())
	data, err := yaml.Marshal(evolvedAtomWrapper{
		Atom: ga.Atom, Source: ga.Source, SourceIDs: ga.SourceIDs, ShardType: ga.ShardType,
		ProblemType: ga.ProblemType, Provider: ga.Provider, Model: ga.Model, PinScope: ga.PinScope,
		Confidence: ga.Confidence, UsageCount: ga.UsageCount, SuccessCount: ga.SuccessCount,
		CreatedAt: ga.CreatedAt, PromotedAt: ga.PromotedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal atom: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write atom file: %w", err)
	}
	return nil
}

func atomFileName(atomID string) string {
	return strings.ReplaceAll(atomID, "/", "_") + ".yaml"
}

func (pe *PromptEvolver) atomStoragePathLocked(atomID string, promoted bool) string {
	if promoted {
		return filepath.Join(pe.promotedDir, atomFileName(atomID))
	}
	return filepath.Join(pe.pendingDir, atomFileName(atomID))
}

// autoPromoteAtoms promotes candidates that meet the confidence threshold on
// ledger-derived evidence, except those the promotion check held
// (jev_promotion.go). Must hold pe.mu.
func (pe *PromptEvolver) autoPromoteAtoms(held map[string]string) ([]promotionEvent, int) {
	promoted := 0
	var pending []promotionEvent
	for id, ga := range pe.evolvedAtoms {
		if _, hold := held[id]; hold {
			continue
		}
		if ga.ShouldPromote(pe.config.ConfidenceThreshold) && ga.PromotedAt.IsZero() {
			cb, promotedAt, err := pe.promoteAtomLocked(id)
			if err == nil {
				promoted++
				if cb != nil {
					pending = append(pending, promotionEvent{atomID: id, promotedAt: promotedAt, cb: cb})
				}
			}
		}
	}
	return pending, promoted
}

// PromoteAtom moves a quarantined candidate to promoted: the operator's
// review decision. It ends any trial of the candidate.
func (pe *PromptEvolver) PromoteAtom(atomID string) error {
	pe.mu.Lock()
	cb, promotedAt, err := pe.promoteAtomLocked(atomID)
	pe.mu.Unlock()
	if err != nil {
		return err
	}
	pe.publishTrials()
	if cb != nil {
		// Callback invoked without pe.mu held to avoid deadlock
		// if the callback re-enters the evolver.
		cb(atomID, promotedAt)
	}
	return nil
}

// promoteAtomLocked moves an atom from pending to promoted (must hold lock).
func (pe *PromptEvolver) promoteAtomLocked(atomID string) (AtomPromotedCallback, time.Time, error) {
	ga, exists := pe.evolvedAtoms[atomID]
	if !exists {
		return nil, time.Time{}, fmt.Errorf("atom not found: %s", atomID)
	}
	if !ga.PromotedAt.IsZero() {
		return nil, time.Time{}, fmt.Errorf("atom %s is already promoted", atomID)
	}

	srcPath := filepath.Join(pe.pendingDir, atomFileName(atomID))
	ga.PromotedAt = time.Now()
	if err := pe.persistAtomLocked(ga); err != nil {
		ga.PromotedAt = time.Time{}
		return nil, time.Time{}, err
	}
	if err := os.Remove(srcPath); err != nil && !os.IsNotExist(err) {
		logging.Get(logging.CategoryAutopoiesis).Warn("Promoted %s but could not remove its quarantined copy: %v", atomID, err)
	}
	if _, err := pe.ledger.DeleteTrial(atomID); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("Promoted %s but could not end its trial: %v", atomID, err)
	}
	logging.Autopoiesis("Atom promoted: id=%s", atomID)
	// Snapshot callback while holding lock so caller can invoke it
	// outside the lock. Invoking outside avoids deadlock if the
	// callback re-enters the evolver (e.g., calls GetPromotedAtoms
	// which takes RLock) while the exclusive lock is held.
	return pe.onAtomPromoted, ga.PromotedAt, nil
}

// RejectAtom moves a quarantined candidate to rejected/ and ends its trial.
func (pe *PromptEvolver) RejectAtom(atomID string) error {
	pe.mu.Lock()
	filename := atomFileName(atomID)
	if err := os.Rename(filepath.Join(pe.pendingDir, filename), filepath.Join(pe.rejectedDir, filename)); err != nil {
		pe.mu.Unlock()
		return err
	}
	delete(pe.evolvedAtoms, atomID)
	pe.rejectedCount++
	pe.mu.Unlock()
	if _, err := pe.ledger.DeleteTrial(atomID); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("Rejected %s but could not end its trial: %v", atomID, err)
	}
	pe.publishTrials()
	logging.Autopoiesis("Atom rejected: id=%s", atomID)
	return nil
}

// GetEvolvedAtoms returns all evolved atoms.
func (pe *PromptEvolver) GetEvolvedAtoms() []*GeneratedAtom {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	result := make([]*GeneratedAtom, 0, len(pe.evolvedAtoms))
	for _, ga := range pe.evolvedAtoms {
		result = append(result, ga)
	}
	return result
}

// GetPendingAtoms returns quarantined candidates awaiting review.
func (pe *PromptEvolver) GetPendingAtoms() []*GeneratedAtom {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	var result []*GeneratedAtom
	for _, ga := range pe.evolvedAtoms {
		if ga.PromotedAt.IsZero() {
			result = append(result, ga)
		}
	}
	return result
}

// GetPromotedAtoms returns promoted atoms.
func (pe *PromptEvolver) GetPromotedAtoms() []*GeneratedAtom {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	var result []*GeneratedAtom
	for _, ga := range pe.evolvedAtoms {
		if !ga.PromotedAt.IsZero() {
			result = append(result, ga)
		}
	}
	return result
}

// loadEvolvedAtoms loads existing candidates from disk. Pending files load
// as quarantined, whatever their history: under the old lifecycle they were
// served unreviewed, and migration never promotes them.
func (pe *PromptEvolver) loadEvolvedAtoms() {
	pe.loadAtomsFromDir(pe.promotedDir, true)
	pe.loadAtomsFromDir(pe.pendingDir, false)
	if entries, err := os.ReadDir(pe.rejectedDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
				pe.rejectedCount++
			}
		}
	}
	logging.Autopoiesis("Loaded %d evolved atoms from disk", len(pe.evolvedAtoms))
}

func (pe *PromptEvolver) loadAtomsFromDir(dir string, promoted bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var w evolvedAtomWrapper
		if err := yaml.Unmarshal(data, &w); err != nil || w.Atom == nil {
			continue
		}
		if _, dup := pe.evolvedAtoms[w.Atom.ID]; dup {
			continue // promoted copy wins over a stale quarantined one
		}
		ga := &GeneratedAtom{
			Atom: w.Atom, Source: w.Source, SourceIDs: w.SourceIDs, ShardType: w.ShardType,
			ProblemType: w.ProblemType, Provider: w.Provider, Model: w.Model, PinScope: w.PinScope,
			Confidence: w.Confidence, UsageCount: w.UsageCount, SuccessCount: w.SuccessCount,
			CreatedAt: w.CreatedAt, PromotedAt: w.PromotedAt,
		}
		if promoted && ga.PromotedAt.IsZero() {
			if info, _ := entry.Info(); info != nil {
				ga.PromotedAt = info.ModTime()
			} else {
				ga.PromotedAt = time.Now()
			}
		}
		if !promoted {
			ga.PromotedAt = time.Time{}
		}
		pe.evolvedAtoms[w.Atom.ID] = ga
	}
}

// =============================================================================
// Trials
// =============================================================================

// SetTrialFactSink installs the receiver of live trial envelopes and hands it
// the current set at once, so trials approved in an earlier session are
// materialized at boot.
func (pe *PromptEvolver) SetTrialFactSink(sink TrialFactSink) {
	pe.mu.Lock()
	pe.trialSink = sink
	pe.mu.Unlock()
	pe.publishTrials()
}

// ApproveTrial is the operator's approval of a bounded trial of one
// quarantined candidate's current version. ttl and maxExposures default to
// the configured trial bounds when non-positive. The candidate stays
// quarantined: a trial earns evidence, it is not deployment.
func (pe *PromptEvolver) ApproveTrial(atomID string, ttl time.Duration, maxExposures int) (TrialEnvelope, error) {
	pe.mu.RLock()
	ga := pe.evolvedAtoms[atomID]
	cfg := pe.config
	pe.mu.RUnlock()
	if ga == nil || ga.Atom == nil {
		return TrialEnvelope{}, fmt.Errorf("atom not found: %s", atomID)
	}
	if !ga.PromotedAt.IsZero() {
		return TrialEnvelope{}, fmt.Errorf("atom %s is already promoted; trials are for quarantined candidates", atomID)
	}
	if ttl <= 0 {
		ttl = cfg.TrialTTL
	}
	if maxExposures <= 0 {
		maxExposures = cfg.TrialMaxExposures
	}
	now := time.Now()
	envelope := TrialEnvelope{
		AtomID:       atomID,
		Version:      prompt.EvolvedCandidateHash(ga.Atom),
		ApprovedAt:   now,
		ExpiresAt:    now.Add(ttl),
		MaxExposures: maxExposures,
	}
	if err := pe.ledger.PutTrial(envelope); err != nil {
		return TrialEnvelope{}, err
	}
	exposures, err := pe.ledger.ExposureCount(atomID, envelope.Version)
	if err == nil {
		envelope.Exposures = exposures
	}
	pe.publishTrials()
	logging.Autopoiesis("Trial approved: id=%s version=%s expires=%s max_exposures=%d",
		atomID, envelope.Version[:12], envelope.ExpiresAt.Format(time.RFC3339), maxExposures)
	return envelope, nil
}

// RevokeTrial ends a trial.
func (pe *PromptEvolver) RevokeTrial(atomID string) error {
	existed, err := pe.ledger.DeleteTrial(atomID)
	if err != nil {
		return err
	}
	if !existed {
		return fmt.Errorf("no trial for %s", atomID)
	}
	pe.publishTrials()
	return nil
}

// Trials lists every trial envelope with its exposure count.
func (pe *PromptEvolver) Trials() ([]TrialEnvelope, error) {
	return pe.ledger.Trials()
}

// publishTrials hands the sink every live envelope of a still-quarantined
// candidate. Expired and exhausted envelopes are left out here and refused by
// the kernel rule as well; either alone fails closed.
func (pe *PromptEvolver) publishTrials() {
	pe.mu.RLock()
	sink := pe.trialSink
	quarantined := make(map[string]bool, len(pe.evolvedAtoms))
	for id, ga := range pe.evolvedAtoms {
		if ga.PromotedAt.IsZero() {
			quarantined[id] = true
		}
	}
	pe.mu.RUnlock()
	if sink == nil {
		return
	}
	trials, err := pe.ledger.Trials()
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("Trial envelopes unreadable; publishing none: %v", err)
		trials = nil
	}
	now := time.Now()
	live := make([]TrialEnvelope, 0, len(trials))
	for _, trial := range trials {
		if quarantined[trial.AtomID] && trial.Live(now) {
			live = append(live, trial)
		}
	}
	sink(live)
}
