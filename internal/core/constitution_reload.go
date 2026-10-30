package core

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"codeberg.org/TauCeti/mangle-go/provenance"
)

// ConstitutionReload identifies an atomic reload's disposition. An error is
// always a rejection before publication: Active still names the prior corpus.
// Candidate names captured source bytes, not an editor's multi-file transaction.
type ConstitutionReload struct {
	Active      string
	Candidate   string
	Stage       string
	Disposition string // rejected or adopted
}

type constitutionSnapshot struct {
	schemas, policy string
	modules         []string
	generation      string
}

func constitutionIdentity(schemas, policy string) string {
	if schemas == "" {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(schemas+"\x00"+policy)))
}

// Require matching captures, rejecting observed source instability without an
// unbounded retry loop. Subsequent work uses the first capture, including routing.
func captureConstitution(fsys fs.FS) (constitutionSnapshot, error) {
	s, p, modules, err := assembleConstitution(fsys)
	if err != nil {
		return constitutionSnapshot{}, err
	}
	s2, p2, _, err := assembleConstitution(fsys)
	if err != nil {
		return constitutionSnapshot{}, err
	}
	if s != s2 || p != p2 {
		return constitutionSnapshot{}, fmt.Errorf("constitution changed during capture; awaiting the next edit")
	}
	return constitutionSnapshot{s, p, modules, constitutionIdentity(s, p)}, nil
}

// stageConstitutionLocked never mutates the live program or EDB. The caller
// holds the live kernel's mutex through staging and publication, so learned
// rules, limits, facts and program suffixes cannot race validation.
func (k *RealKernel) stageConstitutionLocked(src constitutionSnapshot) (*RealKernel, error) {
	if k.constSchemas == "" || !strings.HasPrefix(k.schemas, k.constSchemas) || !strings.HasPrefix(k.policy, k.constPolicy) {
		return nil, ErrConstitutionReplaced
	}
	next := k.cloneLocked()
	next.schemas = src.schemas + strings.TrimPrefix(k.schemas, k.constSchemas)
	next.policy = src.policy + strings.TrimPrefix(k.policy, k.constPolicy)
	next.constSchemas, next.constPolicy = src.schemas, src.policy
	for _, mod := range src.modules {
		next.loadedPolicyFiles[strings.ToLower(mod)] = struct{}{}
	}
	next.markPolicyDirtyLocked()
	if err := next.rebuildProgram(); err != nil {
		return nil, fmt.Errorf("the edited constitution does not compile: %w", err)
	}
	next.refreshSchemaValidatorLocked()
	if next.schemaValidator == nil {
		return nil, fmt.Errorf("the edited constitution has no usable schema validator")
	}
	if k.proofRecorder != nil {
		next.proofRecorder = provenance.NewMemoryRecorder()
	}
	return next, nil
}

func evaluateConstitution(next *RealKernel) error {
	// The ordinary evaluator evicts malformed boot facts. A reload must instead
	// reject the candidate, preserving every fact admitted by the old program.
	for _, f := range next.facts {
		if _, err := next.factToAtomLocked(f); err != nil {
			return fmt.Errorf("retained fact %s is incompatible with the candidate: %w", f.Predicate, err)
		}
	}
	return next.evaluate()
}

// publishConstitutionLocked transfers only program/evaluation state, keeping
// the live kernel's identity, mutexes, subscriptions and external wiring.
// All fallible work has completed. Caller holds both kernels exclusively.
func (k *RealKernel) publishConstitutionLocked(next *RealKernel) {
	k.schemas, k.policy = next.schemas, next.policy
	k.constSchemas, k.constPolicy = next.constSchemas, next.constPolicy
	k.loadedPolicyFiles = next.loadedPolicyFiles
	k.schemaValidator, k.admission = next.schemaValidator, next.admission
	k.programInfo, k.programKey = next.programInfo, next.programKey
	k.strata, k.predToStratum, k.cone = next.strata, next.predToStratum, next.cone
	k.facts, k.cachedAtoms, k.factIndex = next.facts, next.cachedAtoms, next.factIndex
	k.store, k.initialized = next.store, next.initialized
	k.atomCacheStale, k.policyDirty, k.evalErr = false, false, nil
	k.lastEvaluation, k.proofRecorder = next.lastEvaluation, next.proofRecorder
	k.generation.Add(1)
	k.clearDirtyLocked()
	k.storeShared.Store(false)
	k.factsDirty.Store(false)
}

// ReloadConstitution stages and evaluates a candidate before publishing it.
// Every error preserves the live program, EDB, evaluation and read readiness.
func (k *RealKernel) ReloadConstitution() (ConstitutionReload, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	out := ConstitutionReload{Active: constitutionIdentity(k.constSchemas, k.constPolicy), Stage: "capture", Disposition: "rejected"}
	src, err := captureConstitution(constitutionFS())
	if err != nil {
		return out, err
	}
	out.Candidate, out.Stage = src.generation, "prepare"
	next, err := k.stageConstitutionLocked(src)
	if err != nil {
		return out, err
	}
	out.Stage = "evaluate"
	if err := evaluateConstitution(next); err != nil {
		return out, err
	}
	k.publishConstitutionLocked(next)
	out.Active, out.Stage, out.Disposition = src.generation, "ready", "adopted"
	return out, nil
}

// ReloadConstitution locks the routing boundary and every participating kernel
// in domain order. Existing Cortex operations finish first; direct kernel
// writes finish before their snapshot or wait until publication. A specialized
// shard rejects the entire attempt. No rollback or partial adoption is needed.
func (c *CortexKernel) ReloadConstitution() (ConstitutionReload, error) {
	c.adoptionMu.Lock()
	defer c.adoptionMu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	shards := make([]*KernelShard, 0, len(c.shards))
	for _, s := range c.shards {
		shards = append(shards, s)
	}
	slices.SortFunc(shards, func(a, b *KernelShard) int { return strings.Compare(a.domain, b.domain) })
	for _, s := range shards {
		s.kernel.mu.Lock()
	}
	defer func() {
		for i := len(shards) - 1; i >= 0; i-- {
			shards[i].kernel.mu.Unlock()
		}
	}()
	out := ConstitutionReload{Stage: "capture", Disposition: "rejected"}
	primary := c.shards[c.cortexDomain]
	if primary == nil || len(shards) == 0 {
		return out, fmt.Errorf("reload requires a catch-all shard")
	}
	out.Active = constitutionIdentity(primary.kernel.constSchemas, primary.kernel.constPolicy)
	src, err := captureConstitution(constitutionFS())
	if err != nil {
		return out, err
	}
	out.Candidate, out.Stage = src.generation, "prepare"
	staged := make([]*RealKernel, 0, len(shards))
	for _, s := range shards {
		next, err := s.kernel.stageConstitutionLocked(src)
		if err != nil {
			return out, fmt.Errorf("shard %s: %w", s.domain, err)
		}
		staged = append(staged, next)
	}
	out.Stage = "routing"
	dm, err := c.stagedDerivationMap(staged)
	if err != nil {
		return out, err
	}
	out.Stage = "evaluate"
	for i, next := range staged {
		if err := c.hydrateSharedLocked(next, shards[i].domain, primary.kernel.facts, dm); err != nil {
			return out, fmt.Errorf("shard %s: hydrate shared inventory: %w", shards[i].domain, err)
		}
		if err := evaluateConstitution(next); err != nil {
			return out, fmt.Errorf("shard %s: evaluate candidate: %w", shards[i].domain, err)
		}
	}
	for i, next := range staged {
		shards[i].kernel.publishConstitutionLocked(next)
	}
	c.derivationMap = dm
	out.Active, out.Stage, out.Disposition = src.generation, "ready", "adopted"
	return out, nil
}

// Different per-shard extensions cannot be analyzed as if every shard ran
// their union. In that case explicitly use broadcast/fan-out routing, whose
// correctness does not depend on a static map. Common programs are analyzed
// including their appended schemas, policy and learned rules.
func (c *CortexKernel) stagedDerivationMap(staged []*RealKernel) (*DerivationMap, error) {
	first := staged[0]
	for _, k := range staged[1:] {
		if k.schemas != first.schemas || k.policy != first.policy || k.learned != first.learned {
			return nil, nil
		}
	}
	var routes []PredicateConsumerRoute
	for pred, consumers := range c.predicateConsumers {
		for domain := range consumers {
			routes = append(routes, PredicateConsumerRoute{Predicate: pred, Consumer: domain})
		}
	}
	return BuildDerivationMapWithConsumerRoutes(first.schemas+"\n"+first.policy+"\n"+first.learned,
		IntentCorpusPredicates(), c.predicateOwner, c.sharedPredicates, routes, c.cortexDomain)
}

// Replace replicas from canonical EDB, rather than unioning stale replicas
// into the new generation. No derived facts are copied into the EDB.
func (c *CortexKernel) hydrateSharedLocked(next *RealKernel, domain string, canonical []Fact, dm *DerivationMap) error {
	kept := make([]Fact, 0, len(next.facts))
	for _, f := range next.facts {
		if _, shared := c.sharedPredicates[f.Predicate]; !shared {
			kept = append(kept, f)
		}
	}
	next.facts, next.cachedAtoms = kept, nil
	next.rebuildFactIndexLocked()
	for _, f := range canonical {
		if _, shared := c.sharedPredicates[f.Predicate]; !shared {
			continue
		}
		if dm != nil && domain != c.cortexDomain {
			if _, consumes := dm.Consumes[domain][f.Predicate]; !consumes {
				continue
			}
		}
		if _, err := next.addFactIfNewLockedErr(f); err != nil {
			return err
		}
	}
	return nil
}
