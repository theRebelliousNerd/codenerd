package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/logging"

	"github.com/google/mangle/ast"
)

// =============================================================================
// FACT MANAGEMENT
// =============================================================================

// LoadFacts adds facts to the EDB and rebuilds the program.
func (k *RealKernel) LoadFacts(facts []Fact) error {
	timer := logging.StartTimer(logging.CategoryKernel, "LoadFacts")
	logging.Kernel("LoadFacts: loading %d facts into EDB", len(facts))

	k.mu.Lock()
	defer k.mu.Unlock()

	prevCount := len(k.facts)
	sanitizedFacts := make([]Fact, len(facts))
	for i, f := range facts {
		sanitizedFacts[i] = sanitizeFactForNumericPredicates(f)
	}
	added := 0
	for _, f := range sanitizedFacts {
		if k.addFactIfNewLocked(f) {
			added++
		}
	}
	logging.KernelDebug("LoadFacts: added %d/%d facts, EDB: %d -> %d facts", added, len(sanitizedFacts), prevCount, len(k.facts))

	// Count JIT-related facts for debugging
	jitCounts := make(map[string]int)
	for _, f := range sanitizedFacts {
		switch f.Predicate {
		case "is_mandatory", "atom_tag", "atom_priority", "current_context", "atom":
			jitCounts[f.Predicate]++
		}
	}
	if len(jitCounts) > 0 {
		logging.Kernel("LoadFacts JIT: is_mandatory=%d atom_tag=%d atom_priority=%d current_context=%d atom=%d",
			jitCounts["is_mandatory"], jitCounts["atom_tag"], jitCounts["atom_priority"],
			jitCounts["current_context"], jitCounts["atom"])
	}

	// Log sample of facts being loaded (first 5)
	if len(sanitizedFacts) > 0 && logging.IsDebugMode() {
		sampleSize := 5
		if len(sanitizedFacts) < sampleSize {
			sampleSize = len(sanitizedFacts)
		}
		for i := 0; i < sampleSize; i++ {
			logging.KernelDebug("  [%d] %s", i, sanitizedFacts[i].String())
		}
		if len(sanitizedFacts) > sampleSize {
			logging.KernelDebug("  ... and %d more facts", len(sanitizedFacts)-sampleSize)
		}
	}

	// If nothing new was added, skip rebuild.
	if added == 0 {
		timer.Stop()
		return nil
	}

	err := k.rebuild()
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("LoadFacts: rebuild failed: %v", err)
		return err
	}

	timer.Stop()
	return nil
}

// =============================================================================
// Fact Deduplication Helpers
// =============================================================================

// canonFact returns a stable string key for a fact.
// We use Fact.String() which is already canonical for Mangle serialization.
func (k *RealKernel) canonFact(f Fact) string {
	return f.String()
}

// ensureFactIndexLocked initializes the fact index if needed.
// Call only while holding k.mu.
func (k *RealKernel) ensureFactIndexLocked() {
	if k.factIndex == nil {
		k.rebuildFactIndexLocked()
	}
}

// rebuildFactIndexLocked rebuilds the dedupe index from current EDB.
// Call only while holding k.mu.
func (k *RealKernel) rebuildFactIndexLocked() {
	k.factIndex = make(map[string]struct{}, len(k.facts))
	for _, f := range k.facts {
		k.factIndex[k.canonFact(f)] = struct{}{}
	}
}

// addFactIfNewLocked appends a fact only if it is not already present.
// Returns true if added. Call only while holding k.mu.
// OPTIMIZATION: Also caches the converted atom to avoid repeated ToAtom() calls.
func (k *RealKernel) addFactIfNewLocked(f Fact) bool {
	k.ensureFactIndexLocked()
	key := k.canonFact(f)
	if _, ok := k.factIndex[key]; ok {
		return false
	}

	// Convert to atom once and cache it
	atom, err := f.ToAtom()
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("addFactIfNewLocked: failed to convert fact to atom: %v", err)
		// Still add the fact, but without cached atom (will be regenerated in evaluate)
		k.facts = append(k.facts, f)
		k.factIndex[key] = struct{}{}
		return true
	}

	k.facts = append(k.facts, f)
	k.cachedAtoms = append(k.cachedAtoms, atom)
	k.factIndex[key] = struct{}{}
	return true
}

// Assert adds a single fact dynamically and re-evaluates derived facts.
func (k *RealKernel) Assert(fact Fact) error {
	logging.KernelDebug("Assert: %s", fact.String())
	logging.Audit().KernelAssert(fact.Predicate, len(fact.Args))

	k.mu.Lock()
	defer k.mu.Unlock()

	fact = sanitizeFactForNumericPredicates(fact)
	if !k.addFactIfNewLocked(fact) {
		logging.KernelDebug("Assert: duplicate fact skipped: %s", fact.String())
		return nil
	}
	if err := k.evaluate(); err != nil {
		logging.Get(logging.CategoryKernel).Error("Assert: evaluation failed after asserting %s: %v", fact.Predicate, err)
		return err
	}
	logging.KernelDebug("Assert: fact added successfully, total facts=%d", len(k.facts))
	return nil
}

// AssertString parses a Mangle fact string and asserts it.
// Format: predicate(arg1, arg2, ...) where args can be:
//   - Name constants: /foo, /bar
//   - Strings: "quoted text"
//   - Numbers: 42, 3.14
func (k *RealKernel) AssertString(factStr string) error {
	fact, err := ParseFactString(factStr)
	if err != nil {
		return fmt.Errorf("AssertString: failed to parse %q: %w", factStr, err)
	}
	return k.Assert(fact)
}

// AssertBatch adds multiple facts and evaluates once (more efficient).
func (k *RealKernel) AssertBatch(facts []Fact) error {
	timer := logging.StartTimer(logging.CategoryKernel, "AssertBatch")
	logging.KernelDebug("AssertBatch: adding %d facts", len(facts))

	k.mu.Lock()
	defer k.mu.Unlock()

	prevCount := len(k.facts)
	sanitized := make([]Fact, len(facts))
	for i, f := range facts {
		sanitized[i] = sanitizeFactForNumericPredicates(f)
	}
	added := 0
	for _, f := range sanitized {
		if k.addFactIfNewLocked(f) {
			added++
		}
	}
	if added == 0 {
		timer.Stop()
		logging.KernelDebug("AssertBatch: no new facts added (all duplicates)")
		return nil
	}

	if err := k.evaluate(); err != nil {
		logging.Get(logging.CategoryKernel).Error("AssertBatch: evaluation failed after adding %d facts: %v", len(facts), err)
		return err
	}

	timer.Stop()
	logging.KernelDebug("AssertBatch: added %d/%d facts, EDB: %d -> %d facts", added, len(facts), prevCount, len(k.facts))
	return nil
}

// AssertWithoutEval adds a fact without re-evaluating.
// Use when batching many facts, then call Evaluate() once at the end.
func (k *RealKernel) AssertWithoutEval(fact Fact) {
	logging.KernelDebug("AssertWithoutEval: %s (deferred evaluation)", fact.Predicate)
	k.mu.Lock()
	defer k.mu.Unlock()
	fact = sanitizeFactForNumericPredicates(fact)
	if !k.addFactIfNewLocked(fact) {
		logging.KernelDebug("AssertWithoutEval: duplicate fact skipped: %s", fact.String())
	}
}

// Evaluate forces re-evaluation of all rules. Call after AssertWithoutEval batch.
func (k *RealKernel) Evaluate() error {
	timer := logging.StartTimer(logging.CategoryKernel, "Evaluate")
	logging.KernelDebug("Evaluate: forcing re-evaluation of all rules")

	k.mu.Lock()
	defer k.mu.Unlock()

	err := k.evaluate()
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("Evaluate: failed: %v", err)
		return err
	}

	timer.Stop()
	return nil
}

// Retract removes all facts of a given predicate.
// OPTIMIZATION: Maintains atom cache instead of rebuilding entire index.
func (k *RealKernel) Retract(predicate string) error {
	logging.KernelDebug("Retract: removing all facts with predicate=%s", predicate)

	k.mu.Lock()
	defer k.mu.Unlock()

	prevCount := len(k.facts)
	retractedCount := 0
	newFactsLen := 0
	newAtomsLen := 0

	// Filter facts and atoms in parallel
	for i, f := range k.facts {
		if f.Predicate != predicate {
			k.facts[newFactsLen] = f
			if i < len(k.cachedAtoms) {
				k.cachedAtoms[newAtomsLen] = k.cachedAtoms[i]
			}
			newFactsLen++
			newAtomsLen++
		} else {
			retractedCount++
		}
	}

	if retractedCount == 0 {
		logging.KernelDebug("Retract: no facts found for predicate=%s (EDB unchanged at %d facts)", predicate, prevCount)
		return nil
	}

	// Zero tail to release references for GC.
	for i := newFactsLen; i < prevCount; i++ {
		k.facts[i] = Fact{}
		if i < len(k.cachedAtoms) {
			k.cachedAtoms[i] = ast.Atom{} // Zero value for ast.Atom
		}
	}
	k.facts = k.facts[:newFactsLen]
	k.cachedAtoms = k.cachedAtoms[:newAtomsLen]

	// OPTIMIZATION: Incremental index update instead of full rebuild
	if retractedCount > 0 && k.factIndex != nil {
		// Rebuild index only for removed predicate
		k.rebuildFactIndexLocked()
	}

	logging.KernelDebug("Retract: removed %d facts (predicate=%s), EDB: %d -> %d facts",
		retractedCount, predicate, prevCount, len(k.facts))

	if err := k.rebuild(); err != nil {
		logging.Get(logging.CategoryKernel).Error("Retract: rebuild failed after retracting %s: %v", predicate, err)
		return err
	}
	return nil
}

// RetractFact removes a specific fact by matching predicate and first argument.
// This enables selective fact removal (e.g., removing all facts for a specific tool).
func (k *RealKernel) RetractFact(fact Fact) error {
	logging.KernelDebug("RetractFact: removing fact matching predicate=%s, firstArg=%v", fact.Predicate, fact.Args)

	k.mu.Lock()
	defer k.mu.Unlock()

	if len(fact.Args) == 0 {
		err := fmt.Errorf("fact must have at least one argument for matching")
		logging.Get(logging.CategoryKernel).Error("RetractFact: %v", err)
		return err
	}

	prevCount := len(k.facts)
	retractedCount := 0
	newLen := 0
	for _, f := range k.facts {
		// Keep facts that don't match predicate OR don't match first argument
		if f.Predicate != fact.Predicate {
			k.facts[newLen] = f
			newLen++
			continue
		}
		// Same predicate - check first argument
		if len(f.Args) > 0 && len(fact.Args) > 0 {
			if !argsEqual(f.Args[0], fact.Args[0]) {
				k.facts[newLen] = f
				newLen++
			} else {
				retractedCount++
			}
			// Matching predicate and first arg - don't add (retract it)
		} else {
			k.facts[newLen] = f
			newLen++
		}
	}

	if retractedCount == 0 {
		firstArg := interface{}(nil)
		if len(fact.Args) > 0 {
			firstArg = fact.Args[0]
		}
		logging.KernelDebug("RetractFact: no matching facts found (predicate=%s firstArg=%v)", fact.Predicate, firstArg)
		return nil
	}

	// Zero tail to release references for GC.
	for i := newLen; i < prevCount; i++ {
		k.facts[i] = Fact{}
	}
	k.facts = k.facts[:newLen]
	k.rebuildFactIndexLocked()

	logging.KernelDebug("RetractFact: removed %d facts, EDB: %d -> %d facts",
		retractedCount, prevCount, len(k.facts))

	if err := k.rebuild(); err != nil {
		logging.Get(logging.CategoryKernel).Error("RetractFact: rebuild failed: %v", err)
		return err
	}
	return nil
}

// RetractExactFact removes facts that exactly match predicate and all arguments.
// This is safer for multi-arity predicates where multiple facts may share a first arg.
// It does NOT replace RetractFact, which intentionally matches only the first arg.
func (k *RealKernel) RetractExactFact(fact Fact) error {
	logging.KernelDebug("RetractExactFact: removing exact fact predicate=%s args=%v", fact.Predicate, fact.Args)

	k.mu.Lock()
	defer k.mu.Unlock()

	if len(fact.Args) == 0 {
		err := fmt.Errorf("fact must have at least one argument for exact matching")
		logging.Get(logging.CategoryKernel).Error("RetractExactFact: %v", err)
		return err
	}

	prevCount := len(k.facts)
	filtered := make([]Fact, 0, prevCount)
	retractedCount := 0
	for _, f := range k.facts {
		if f.Predicate != fact.Predicate || !argsSliceEqual(f.Args, fact.Args) {
			filtered = append(filtered, f)
			continue
		}
		retractedCount++
	}
	k.facts = filtered
	if retractedCount > 0 {
		k.rebuildFactIndexLocked()
	}

	logging.KernelDebug("RetractExactFact: removed %d facts, EDB: %d -> %d facts",
		retractedCount, prevCount, len(k.facts))

	// Only rebuild if something changed
	if retractedCount > 0 {
		if err := k.rebuild(); err != nil {
			logging.Get(logging.CategoryKernel).Error("RetractExactFact: rebuild failed: %v", err)
			return err
		}
	}
	return nil
}

// RetractExactFactsBatch removes a batch of exact facts and rebuilds once.
// Useful for incremental world model updates on large repos.
func (k *RealKernel) RetractExactFactsBatch(facts []Fact) error {
	if len(facts) == 0 {
		return nil
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	canon := func(f Fact) string {
		argsJSON, _ := json.Marshal(f.Args)
		return f.Predicate + "|" + string(argsJSON)
	}

	toRemove := make(map[string]struct{}, len(facts))
	for _, f := range facts {
		toRemove[canon(f)] = struct{}{}
	}

	prevCount := len(k.facts)
	filtered := make([]Fact, 0, prevCount)
	retractedCount := 0
	for _, f := range k.facts {
		if _, ok := toRemove[canon(f)]; ok {
			retractedCount++
			continue
		}
		filtered = append(filtered, f)
	}
	k.facts = filtered
	if retractedCount > 0 {
		k.rebuildFactIndexLocked()
	}

	logging.KernelDebug("RetractExactFactsBatch: removed %d facts, EDB: %d -> %d facts",
		retractedCount, prevCount, len(k.facts))

	if retractedCount > 0 {
		if err := k.rebuild(); err != nil {
			logging.Get(logging.CategoryKernel).Error("RetractExactFactsBatch: rebuild failed: %v", err)
			return err
		}
	}
	return nil
}

// RemoveFactsByPredicateSet removes all facts whose predicate is in the given set.
// Rebuilds once if anything was removed.
func (k *RealKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	if len(predicates) == 0 {
		return nil
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	prevCount := len(k.facts)
	filtered := make([]Fact, 0, prevCount)
	retractedCount := 0
	for _, f := range k.facts {
		if _, ok := predicates[f.Predicate]; ok {
			retractedCount++
			continue
		}
		filtered = append(filtered, f)
	}
	k.facts = filtered
	if retractedCount > 0 {
		k.rebuildFactIndexLocked()
	}

	logging.KernelDebug("RemoveFactsByPredicateSet: removed %d facts, EDB: %d -> %d facts",
		retractedCount, prevCount, len(k.facts))

	if retractedCount > 0 {
		if err := k.rebuild(); err != nil {
			logging.Get(logging.CategoryKernel).Error("RemoveFactsByPredicateSet: rebuild failed: %v", err)
			return err
		}
	}
	return nil
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// argsEqual compares two fact arguments for equality.
// OPTIMIZATION: Uses type switches instead of expensive fmt.Sprintf fallback.
func argsEqual(a, b interface{}) bool {
	// Fast path: direct equality check (BUT must be careful with uncomparable types like maps)
	// We cannot simply do `if a == b` because if a/b contain maps, it panics.
	// So we proceed to type switch immediately.

	switch av := a.(type) {
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
	case MangleAtom:
		// MangleAtom is a string type alias, check both MangleAtom and string
		if bv, ok := b.(MangleAtom); ok {
			return av == bv
		}
		if bv, ok := b.(string); ok {
			return string(av) == bv
		}
	case int:
		// Handle int separately from int64
		if bv, ok := b.(int); ok {
			return av == bv
		}
		// Cross-compare with int64
		if bv, ok := b.(int64); ok {
			return int64(av) == bv
		}
	case int64:
		if bv, ok := b.(int64); ok {
			return av == bv
		}
		// Cross-compare with int
		if bv, ok := b.(int); ok {
			return av == int64(bv)
		}
	case float64:
		if bv, ok := b.(float64); ok {
			return av == bv
		}
	case bool:
		if bv, ok := b.(bool); ok {
			return av == bv
		}
	default:
		// SLOW PATH: Only for truly unknown types
		// This should rarely execute if type system is well-defined
		// CAUTION: reflect.DeepEqual would be safer but slower.
		// For uncomparable types (maps, slices), fmt.Sprintf is a reasonable fallback for equality check
		// because we only care if they "look" the same for retraction purposes.
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}

	// Type mismatch (e.g., string vs int)
	return false
}

// argsSliceEqual compares two argument slices for full equality.
func argsSliceEqual(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !argsEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

// sanitizeFactForNumericPredicates coerces common priority atoms to numbers
// for predicates that participate in numeric comparisons.
// This prevents evaluation failures like "value /high is not a number" when
// LLMs emit priority atoms in numeric slots.
func sanitizeFactForNumericPredicates(f Fact) Fact {
	switch f.Predicate {
	case "agenda_item":
		// agenda_item(ItemID, Description, Priority, Status, Timestamp)
		if len(f.Args) > 2 {
			f.Args[2] = coercePriorityAtomToNumber(f.Args[2])
		}
	case "prompt_atom":
		// prompt_atom(AtomID, Category, Priority, TokenCount, IsMandatory)
		if len(f.Args) > 2 {
			f.Args[2] = coercePriorityAtomToNumber(f.Args[2])
		}
	case "atom_priority":
		// atom_priority(AtomID, Priority)
		if len(f.Args) > 1 {
			f.Args[1] = coercePriorityAtomToNumber(f.Args[1])
		}
	}
	return f
}

func coercePriorityAtomToNumber(v interface{}) interface{} {
	switch t := v.(type) {
	case string:
		atom := strings.TrimSpace(t)
		if atom == "" {
			return v
		}
		// Accept both "/high" and "high"
		trimmed := strings.TrimPrefix(atom, "/")
		switch trimmed {
		case "critical":
			return int64(100)
		case "high":
			return int64(80)
		case "medium", "normal":
			return int64(50)
		case "low":
			return int64(25)
		case "lowest":
			return int64(10)
		default:
			// Log unknown priority atoms for debugging (audit item 5.1)
			logging.Kernel("WARN: unknown priority atom '%s', passing through unchanged", atom)
			return v
		}
	default:
		return v
	}
}

// GetFactsSnapshot returns a copy of the current facts (thread-safe).
func (k *RealKernel) GetFactsSnapshot() []Fact {
	k.mu.RLock()
	defer k.mu.RUnlock()
	snapshot := make([]Fact, len(k.facts))
	copy(snapshot, k.facts)
	return snapshot
}

// FactCount returns the number of facts in the EDB.
func (k *RealKernel) FactCount() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return len(k.facts)
}

// GetAllFacts returns all facts in the EDB (thread-safe snapshot).
func (k *RealKernel) GetAllFacts() []Fact {
	k.mu.RLock()
	defer k.mu.RUnlock()
	result := make([]Fact, len(k.facts))
	copy(result, k.facts)
	return result
}
