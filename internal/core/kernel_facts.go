package core

import (
	"encoding/json"
	"fmt"
	"iter"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"codenerd/internal/logging"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// =============================================================================
// FACT MANAGEMENT
// =============================================================================

// LoadFactsSeq adds facts to the EDB from an iterator and rebuilds the program.
func (k *RealKernel) LoadFactsSeq(seq iter.Seq[Fact]) error {
	timer := logging.StartTimer(logging.CategoryKernel, "LoadFactsSeq")

	var added int
	var prevCount int

	k.mu.Lock()
	prevCount = len(k.facts)
	k.mu.Unlock()

	for f := range seq {
		f = sanitizeFactForNumericPredicates(f)
		k.mu.Lock()
		if k.addFactIfNewLocked(f) {
			added++
		}
		k.mu.Unlock()
	}

	k.mu.RLock()
	newCount := len(k.facts)
	k.mu.RUnlock()

	logging.KernelDebug("LoadFactsSeq: added %d facts, EDB: %d -> %d facts", added, prevCount, newCount)

	if added > 0 {
		k.mu.Lock()
		err := k.rebuild()
		k.mu.Unlock()
		if err != nil {
			logging.Get(logging.CategoryKernel).Error("LoadFactsSeq: rebuild failed: %v", err)
			return err
		}
	}
	timer.Stop()
	return nil
}

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
		sampleSize := min(len(sanitizedFacts), 5)
		for i := range sampleSize {
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

	// LoadFacts is the boot path — evaluate eagerly to ensure initialization.
	k.cachedAtoms = nil // Invalidate cache before full re-evaluation
	err := k.evaluate()
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("LoadFacts: evaluate failed: %v", err)
		return err
	}
	k.factsDirty.Store(false)

	timer.Stop()
	return nil
}

// =============================================================================
// Fact Deduplication Helpers
// =============================================================================

// canonFact returns a stable string key for a fact.
func (k *RealKernel) canonFact(f Fact) string {
	var sb strings.Builder
	sb.WriteString(f.Predicate)
	sb.WriteString("(")
	for i, arg := range f.Args {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(canonValue(arg))
	}
	sb.WriteString(")")
	return sb.String()
}

func canonValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case MangleAtom:
		return canonString(string(t))
	case string:
		return canonString(t)
	case bool:
		if t {
			return "/true"
		}
		return "/false"
	case int:
		return strconv.FormatInt(int64(t), 10)
	case int8:
		return strconv.FormatInt(int64(t), 10)
	case int16:
		return strconv.FormatInt(int64(t), 10)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint:
		return strconv.FormatUint(uint64(t), 10)
	case uint8:
		return strconv.FormatUint(uint64(t), 10)
	case uint16:
		return strconv.FormatUint(uint64(t), 10)
	case uint32:
		return strconv.FormatUint(uint64(t), 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case float32:
		return canonFloat64(float64(t))
	case float64:
		return canonFloat64(t)
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return strconv.FormatInt(i, 10)
		}
		if f, err := t.Float64(); err == nil {
			return canonFloat64(f)
		}
		return strconv.Quote(t.String())
	case []byte:
		return strconv.Quote(string(t))
	case []any:
		return canonSliceInterface(t)
	case []string:
		return canonSliceString(t)
	case []int:
		return canonSliceInt(t)
	case []int64:
		return canonSliceInt64(t)
	case []float64:
		return canonSliceFloat64(t)
	case map[string]any:
		return canonMapStringInterface(t)
	case map[string]string:
		return canonMapStringString(t)
	default:
		rv := reflect.ValueOf(v)
		if rv.IsValid() {
			switch rv.Kind() {
			case reflect.Slice, reflect.Array:
				return canonSliceReflect(rv)
			case reflect.Map:
				return canonMapReflect(rv)
			}
		}
		return strconv.Quote(fmt.Sprintf("%v", v))
	}
}

func canonString(v string) string {
	if isValidMangleNameConstant(v) {
		return v
	}
	return strconv.Quote(v)
}

// canonFloat64 returns a canonical string representation of a float64 value.
func canonFloat64(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func canonSliceInterface(values []any) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, v := range values {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(canonValue(v))
	}
	sb.WriteString("]")
	return sb.String()
}

func canonSliceString(values []string) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, v := range values {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(canonString(v))
	}
	sb.WriteString("]")
	return sb.String()
}

func canonSliceInt(values []int) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, v := range values {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.FormatInt(int64(v), 10))
	}
	sb.WriteString("]")
	return sb.String()
}

func canonSliceInt64(values []int64) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, v := range values {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.FormatInt(v, 10))
	}
	sb.WriteString("]")
	return sb.String()
}

func canonSliceFloat64(values []float64) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, v := range values {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(canonFloat64(v))
	}
	sb.WriteString("]")
	return sb.String()
}

func canonSliceReflect(value reflect.Value) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < value.Len(); i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(canonValue(value.Index(i).Interface()))
	}
	sb.WriteString("]")
	return sb.String()
}

func canonMapStringInterface(values map[string]any) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.Quote(k))
		sb.WriteString(":")
		sb.WriteString(canonValue(values[k]))
	}
	sb.WriteString("}")
	return sb.String()
}

func canonMapStringString(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.Quote(k))
		sb.WriteString(":")
		sb.WriteString(canonString(values[k]))
	}
	sb.WriteString("}")
	return sb.String()
}

func canonMapReflect(value reflect.Value) string {
	keys := value.MapKeys()
	keyStrings := make([]string, 0, len(keys))
	keyMap := make(map[string]reflect.Value, len(keys))
	for _, k := range keys {
		ks := fmt.Sprint(k.Interface())
		keyStrings = append(keyStrings, ks)
		keyMap[ks] = k
	}
	sort.Strings(keyStrings)
	var sb strings.Builder
	sb.WriteString("{")
	for i, ks := range keyStrings {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.Quote(ks))
		sb.WriteString(":")
		sb.WriteString(canonValue(value.MapIndex(keyMap[ks]).Interface()))
	}
	sb.WriteString("}")
	return sb.String()
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
// SAFETY: Enforces MaxFactsInKernel limit and rejects facts that fail ToAtom().
func (k *RealKernel) addFactIfNewLocked(f Fact) bool {
	// Enforce EDB size limit to prevent unbounded memory growth
	maxFacts := k.maxFacts
	if maxFacts <= 0 {
		maxFacts = defaultMaxFacts
	}
	if len(k.facts) >= maxFacts {
		logging.Get(logging.CategoryKernel).Warn("EDB fact limit reached (%d/%d), rejecting fact: %s",
			len(k.facts), maxFacts, f.Predicate)
		return false
	}

	// Intern predicate name + string args so repeated values across
	// the EDB share a single backing string (stdlib unique.Handle).
	f = internFact(f)

	k.ensureFactIndexLocked()
	key := k.canonFact(f)
	if _, ok := k.factIndex[key]; ok {
		return false
	}

	// Convert to atom once and cache it.
	// SAFETY: Reject facts that fail conversion to prevent cache desync.
	// Previously, failed facts were added to k.facts but skipped k.cachedAtoms,
	// causing evaluate() to detect a length mismatch and attempt a full rebuild
	// that could also fail, soft-bricking the kernel.
	atom, err := f.ToAtom()
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("addFactIfNewLocked: rejecting fact that fails ToAtom: %s - %v", f.Predicate, err)
		return false
	}

	k.facts = append(k.facts, f)
	k.cachedAtoms = append(k.cachedAtoms, atom)
	k.factIndex[key] = struct{}{}

	// Track per-fact delta for the differential-eval fast path.
	// Only meaningful when the diff engine is active; otherwise these slices
	// just grow and get reset on the next full evaluate(). Cost is negligible
	// vs. ToAtom(), which we already paid above.
	if diffEvalEnabled() {
		k.factsSinceLastEval = append(k.factsSinceLastEval, f)
		k.markStratumDirtyLocked(atom.Predicate)
	}
	return true
}

// markStratumDirtyLocked sets the stratum that owns predicateSym and every
// stratum that depends on it (i.e. all higher-numbered strata) as dirty.
// This is the correct-but-pessimistic policy from the task: marking
// [s, maxStratum] guarantees no derived fact in a dependent stratum is left
// stale, at the cost of re-evaluating some strata that may not actually
// depend on the changed one. Caller must hold k.mu.
func (k *RealKernel) markStratumDirtyLocked(predicate ast.PredicateSym) {
	if k.predToStratum == nil {
		return
	}
	s, ok := k.predToStratum[predicate]
	if !ok {
		// Unknown predicate: treat conservatively as stratum 0 so it
		// triggers a full re-eval of derived strata.
		s = 0
	}
	if k.dirtyStrata == nil {
		k.dirtyStrata = make(map[int]bool)
	}
	maxStratum := len(k.strata) - 1
	if maxStratum < s {
		maxStratum = s
	}
	for i := s; i <= maxStratum; i++ {
		k.dirtyStrata[i] = true
	}
}

// Assert adds a single fact dynamically and re-evaluates derived facts.
func (k *RealKernel) Assert(fact Fact) error {
	// Skip per-assert debug for high-frequency predicates (heartbeats)
	if fact.Predicate != "system_heartbeat" {
		logging.KernelDebug("Assert: %s", fact.String())
	}
	logging.Audit().KernelAssert(fact.Predicate, len(fact.Args))

	k.mu.Lock()

	fact = sanitizeFactForNumericPredicates(fact)
	if !k.addFactIfNewLocked(fact) {
		// Duplicate assert is a no-op — suppress debug to avoid log spam
		k.mu.Unlock()
		return nil
	}
	k.factsDirty.Store(true)
	logging.KernelDebug("Assert: fact added successfully, total facts=%d", len(k.facts))
	k.mu.Unlock()

	// Publish AFTER releasing lock to avoid holding mutex during channel sends
	if k.eventBus != nil {
		k.eventBus.Publish(fact.Predicate)
	}
	return nil
}

// AssertBatch adds multiple facts and re-evaluates once.
// OPTIMIZATION: This is significantly faster than calling Assert() in a loop.
// For M assertions, Assert loop = O(M*N) evaluations, AssertBatch = O(N) evaluation.
func (k *RealKernel) AssertBatch(facts []Fact) error {
	if len(facts) == 0 {
		return nil
	}

	logging.KernelDebug("AssertBatch: asserting %d facts", len(facts))

	k.mu.Lock()

	addedCount := 0
	addedPredicates := make(map[string]struct{}) // Track unique predicates for event bus
	for _, fact := range facts {
		fact = sanitizeFactForNumericPredicates(fact)
		if k.addFactIfNewLocked(fact) {
			addedCount++
			addedPredicates[fact.Predicate] = struct{}{}
			logging.Audit().KernelAssert(fact.Predicate, len(fact.Args))
		}
	}

	if addedCount == 0 {
		logging.KernelDebug("AssertBatch: all %d facts were duplicates", len(facts))
		k.mu.Unlock()
		return nil
	}

	// Mark dirty for lazy evaluation (single evaluate on next query)
	k.factsDirty.Store(true)

	logging.KernelDebug("AssertBatch: successfully added %d/%d facts, total facts=%d",
		addedCount, len(facts), len(k.facts))
	k.mu.Unlock()

	// Publish AFTER releasing lock — one event per unique predicate
	if k.eventBus != nil {
		for pred := range addedPredicates {
			k.eventBus.Publish(pred)
		}
	}
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
	k.factsDirty.Store(false)

	timer.Stop()
	return nil
}

// Retract removes all facts of a given predicate.
// OPTIMIZATION: Maintains atom cache instead of rebuilding entire index.
func (k *RealKernel) Retract(predicate string) error {
	// Skip per-retract debug for high-frequency no-op retractions

	k.mu.Lock()
	defer k.mu.Unlock()

	prevCount := len(k.facts)
	retractedCount := 0
	newFactsLen := 0
	newAtomsLen := 0

	// Filter facts and atoms in parallel
	hasCachedAtoms := len(k.cachedAtoms) == prevCount && prevCount > 0
	for i, f := range k.facts {
		if f.Predicate != predicate {
			k.facts[newFactsLen] = f
			if hasCachedAtoms {
				k.cachedAtoms[newAtomsLen] = k.cachedAtoms[i]
				newAtomsLen++
			}
			newFactsLen++
		} else {
			retractedCount++
		}
	}

	if retractedCount == 0 {
		// Empty retract is a no-op — suppress debug to avoid log spam
		return nil
	}

	// Zero tail to release references for GC.
	for i := newFactsLen; i < prevCount; i++ {
		k.facts[i] = Fact{}
		if hasCachedAtoms {
			k.cachedAtoms[i] = ast.Atom{} // Zero value for ast.Atom
		}
	}
	k.facts = k.facts[:newFactsLen]
	if hasCachedAtoms {
		k.cachedAtoms = k.cachedAtoms[:newAtomsLen]
	} else {
		k.cachedAtoms = nil
	}

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
		firstArg := any(nil)
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

	toRemove := make(map[string]struct{}, len(facts))
	for _, f := range facts {
		toRemove[k.canonFact(f)] = struct{}{}
	}

	prevCount := len(k.facts)
	filtered := make([]Fact, 0, prevCount)
	retractedCount := 0
	for _, f := range k.facts {
		if _, ok := toRemove[k.canonFact(f)]; ok {
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

// =============================================================================
// GENERAL UTILITY FUNCTIONS
// =============================================================================

// argsEqual compares two fact arguments for equality.
// OPTIMIZATION: Uses type switches instead of expensive fmt.Sprintf fallback.
func argsEqual(a, b any) bool {
	// Check for nil
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Use type switch to handle specific types (avoids panic on non-comparable types like maps)
	switch av := a.(type) {
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
		// Check for MangleAtom equality (symmetry)
		if bv, ok := b.(MangleAtom); ok {
			return av == string(bv)
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
	case uint:
		if bv, ok := b.(uint); ok {
			return av == bv
		}
		if bv, ok := b.(uint64); ok {
			return uint64(av) == bv
		}
	case uint64:
		if bv, ok := b.(uint64); ok {
			return av == bv
		}
		if bv, ok := b.(uint); ok {
			return av == uint64(bv)
		}
	case int32:
		if bv, ok := b.(int32); ok {
			return av == bv
		}
	case uint32:
		if bv, ok := b.(uint32); ok {
			return av == bv
		}
	case float64:
		if bv, ok := b.(float64); ok {
			return av == bv
		}
	case bool:
		if bv, ok := b.(bool); ok {
			return av == bv
		}
	case map[string]any:
		// Maps are not comparable with ==, use reflect.DeepEqual
		if bv, ok := b.(map[string]any); ok {
			return reflect.DeepEqual(av, bv)
		}
	case []any:
		// Slices are not comparable with ==, use reflect.DeepEqual
		if bv, ok := b.([]any); ok {
			return reflect.DeepEqual(av, bv)
		}
	default:
		// SLOW PATH: Use reflect.DeepEqual for truly unknown types
		// This handles any comparable and non-comparable types safely
		return reflect.DeepEqual(a, b)
	}

	// Type mismatch (e.g., string vs int)
	return false
}

// argsSliceEqual compares two argument slices for full equality.
func argsSliceEqual(a, b []any) bool {
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

func isValidMangleNameConstant(v string) bool {
	if !strings.HasPrefix(v, "/") {
		return false
	}

	// Whitespace is never valid in Mangle name constants.
	if strings.ContainsAny(v, " \t\n\r") {
		return false
	}

	// File paths should NOT be treated as name constants.
	// More than 2 path segments indicates a file path.
	if strings.Count(v, "/") > 2 {
		return false
	}

	// Common file extensions indicate a file path.
	if hasFileExtension(v) {
		return false
	}

	_, err := ast.Name(v)
	return err == nil
}

func hasFileExtension(v string) bool {
	commonExts := []string{
		".go", ".md", ".py", ".js", ".ts", ".tsx", ".jsx",
		".yaml", ".yml", ".json", ".txt", ".mg", ".html", ".css",
		".sh", ".bash", ".ps1", ".bat", ".exe", ".dll", ".so",
		".c", ".h", ".cpp", ".hpp", ".rs", ".rb", ".java",
		".xml", ".toml", ".ini", ".cfg", ".conf", ".log",
	}
	lowerV := strings.ToLower(v)
	for _, ext := range commonExts {
		if strings.HasSuffix(lowerV, ext) {
			return true
		}
	}
	return false
}

// sanitizeFactForNumericPredicates coerces common priority atoms to numbers
// for predicates that participate in numeric comparisons.
// This prevents evaluation failures like "value /high is not a number" when
// LLMs emit priority atoms in numeric slots.
//
// Also performs poisoned-fact rescue: when a numeric slot contains a Go
// pointer string leak (`0x7ff63be770e0` shape — see types.go ToAtom
// guard for the upstream fix), the slot is rewritten to 0 with a Warn
// log. This protects the kernel from crashing on facts that were
// PERSISTED to shard knowledge DBs before the upstream fix landed; new
// emissions go through types.go and never reach here.
func sanitizeFactForNumericPredicates(f Fact) Fact {
	switch f.Predicate {
	case "agenda_item":
		// agenda_item(ItemID, Description, Priority, Status, Timestamp)
		if len(f.Args) > 2 {
			f.Args[2] = coercePriorityAtomToNumber(f.Args[2])
			f.Args[2] = scrubPointerLeak(f.Predicate, 2, f.Args[2])
		}
	case "prompt_atom":
		// prompt_atom(AtomID, Category, Priority, TokenCount, IsMandatory)
		if len(f.Args) > 2 {
			f.Args[2] = coercePriorityAtomToNumber(f.Args[2])
			f.Args[2] = scrubPointerLeak(f.Predicate, 2, f.Args[2])
		}
		if len(f.Args) > 3 {
			f.Args[3] = scrubPointerLeak(f.Predicate, 3, f.Args[3])
		}
	case "atom_priority":
		// atom_priority(AtomID, Priority)
		if len(f.Args) > 1 {
			f.Args[1] = coercePriorityAtomToNumber(f.Args[1])
			f.Args[1] = scrubPointerLeak(f.Predicate, 1, f.Args[1])
		}
	}
	return f
}

// scrubPointerLeak detects strings that look like Go memory addresses
// (`0x` + 6+ hex digits, no spaces, no other content) and rewrites them
// to int64(0) with a one-line Warn. This is the persisted-fact
// counterpart of the structured error in types.go ToAtom: that one
// catches new leaks at the assertion site; this one catches facts
// loaded from .nerd/shards/*.db that were poisoned BEFORE the upstream
// fix. The warn includes predicate and arg-index so the offending row
// can be located and cleaned out of the DB.
func scrubPointerLeak(predicate string, argIdx int, v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	if !looksLikePointerHex(s) {
		return v
	}
	logging.Kernel("WARN: scrubbed pointer-leak %q from %s arg #%d (numeric slot); coerced to 0 — clean shard knowledge DBs to remove the root entry", s, predicate, argIdx)
	return int64(0)
}

func looksLikePointerHex(s string) bool {
	if len(s) < 8 || len(s) > 20 {
		return false
	}
	if s[0] != '0' || (s[1] != 'x' && s[1] != 'X') {
		return false
	}
	for i := 2; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

func coercePriorityAtomToNumber(v any) any {
	switch t := v.(type) {
	case string:
		return parsePriorityString(t, v)
	case MangleAtom:
		return parsePriorityString(string(t), v)
	default:
		return v
	}
}

func parsePriorityString(atom string, original any) any {
	atom = strings.TrimSpace(atom)
	if atom == "" {
		return original
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
		return original
	}
}

// GetFactsSnapshot returns a copy of the current facts (thread-safe).
// Deprecated: use GetFactsSnapshotSeq for memory efficiency.
func (k *RealKernel) GetFactsSnapshot() []Fact {
	k.mu.RLock()
	defer k.mu.RUnlock()
	snapshot := make([]Fact, len(k.facts))
	copy(snapshot, k.facts)
	return snapshot
}

// GetFactsSnapshotSeq returns an iterator of the current facts (thread-safe).
func (k *RealKernel) GetFactsSnapshotSeq() iter.Seq[Fact] {
	return func(yield func(Fact) bool) {
		k.mu.RLock()
		defer k.mu.RUnlock()
		for _, f := range k.facts {
			if !yield(f) {
				return
			}
		}
	}
}

// FactCount returns the number of facts in the EDB.
func (k *RealKernel) FactCount() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return len(k.facts)
}

// GetAllFacts returns all facts in the EDB (thread-safe snapshot).
// Deprecated: use GetAllFactsSeq for memory efficiency.
func (k *RealKernel) GetAllFacts() []Fact {
	k.mu.RLock()
	defer k.mu.RUnlock()
	result := make([]Fact, len(k.facts))
	copy(result, k.facts)
	return result
}

// GetAllFactsSeq returns all facts in the EDB as an iterator (thread-safe snapshot).
func (k *RealKernel) GetAllFactsSeq() iter.Seq[Fact] {
	return k.GetFactsSnapshotSeq()
}

// IsDirty returns whether the kernel's EDB has been mutated since the last evaluation.
// When true, the next Query/QueryAll will trigger a lazy re-evaluation.
func (k *RealKernel) IsDirty() bool {
	return k.factsDirty.Load()
}

// LoadSchemas replaces the kernel's schema content and marks it for reparse.
// This is used by KernelShard to load domain-specific schemas.
func (k *RealKernel) LoadSchemas(schemaContent string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.schemas = schemaContent
	k.policyDirty = true // Force reparse since schemas changed
	logging.KernelDebug("LoadSchemas: replaced schemas (%d bytes), policyDirty=true", len(schemaContent))
}

// AppendSchema appends additional schema declarations to the kernel's existing schemas.
// Unlike LoadSchemas, this preserves all existing schemas (e.g., the 277KB Cortex defaults)
// and adds new declarations on top. Use this for tests or extensions that need to add
// one or two predicates without wiping out the entire schema corpus.
func (k *RealKernel) AppendSchema(schemaContent string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.schemas += "\n" + schemaContent
	k.policyDirty = true // Force reparse since schemas changed
	logging.KernelDebug("AppendSchema: appended %d bytes to schemas (total %d bytes), policyDirty=true", len(schemaContent), len(k.schemas))
}

// LoadPolicy replaces the kernel's policy content and marks it for reparse.
// This is used by KernelShard to load domain-specific policy rules.
func (k *RealKernel) LoadPolicy(policyContent string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.policy = policyContent
	k.policyDirty = true // Force reparse since policy changed
	logging.KernelDebug("LoadPolicy: replaced policy (%d bytes), policyDirty=true", len(policyContent))
}
