package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/types"

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

func canonValue(v interface{}) string {
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
	case []interface{}:
		return canonSliceInterface(t)
	case []string:
		return canonSliceString(t)
	case []int:
		return canonSliceInt(t)
	case []int64:
		return canonSliceInt64(t)
	case []float64:
		return canonSliceFloat64(t)
	case map[string]interface{}:
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

func canonSliceInterface(values []interface{}) string {
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

func canonMapStringInterface(values map[string]interface{}) string {
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

// AssertBatch adds multiple facts and re-evaluates once.
// OPTIMIZATION: This is significantly faster than calling Assert() in a loop.
// For M assertions, Assert loop = O(M*N) evaluations, AssertBatch = O(N) evaluation.
func (k *RealKernel) AssertBatch(facts []Fact) error {
	if len(facts) == 0 {
		return nil
	}

	logging.KernelDebug("AssertBatch: asserting %d facts", len(facts))

	k.mu.Lock()
	defer k.mu.Unlock()

	addedCount := 0
	for _, fact := range facts {
		fact = sanitizeFactForNumericPredicates(fact)
		if k.addFactIfNewLocked(fact) {
			addedCount++
			logging.Audit().KernelAssert(fact.Predicate, len(fact.Args))
		}
	}

	if addedCount == 0 {
		logging.KernelDebug("AssertBatch: all %d facts were duplicates", len(facts))
		return nil
	}

	// Evaluate ONCE for all added facts
	if err := k.evaluate(); err != nil {
		logging.Get(logging.CategoryKernel).Error("AssertBatch: evaluation failed after asserting %d facts: %v", addedCount, err)
		return err
	}

	logging.KernelDebug("AssertBatch: successfully added %d/%d facts, total facts=%d",
		addedCount, len(facts), len(k.facts))
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
// TRANSACTION API
// =============================================================================

// KernelTransaction buffers retract + assert operations and executes them
// atomically with a single rebuild(). This avoids the performance penalty
// of N separate retracts/asserts each triggering a full fixpoint evaluation.
//
// Usage:
//
//	tx := kernel.Transaction()
//	tx.Retract("user_intent")
//	tx.Retract("pending_action")
//	tx.Assert(Fact{Predicate: "user_intent", Args: [...]})
//	if err := tx.Commit(); err != nil { ... }
type KernelTransaction struct {
	kernel *RealKernel

	// Pending operations
	retractPredicates   []string            // Retract all facts of predicate
	retractFacts        []Fact              // Retract by predicate + first arg
	retractExactFacts   []Fact              // Retract by predicate + all args
	retractPredicateSet map[string]struct{} // Retract all facts for predicate set
	assertFacts         []Fact              // Facts to assert

	committed bool
}

// Transaction creates a new kernel transaction.
// Buffer retract/assert operations, then call Commit() to apply them atomically.
// Implements types.KernelTransactor.
func (k *RealKernel) Transaction() types.KernelTransaction {
	return &KernelTransaction{
		kernel:              k,
		retractPredicateSet: make(map[string]struct{}),
	}
}

// Retract queues removal of all facts with the given predicate.
func (tx *KernelTransaction) Retract(predicate string) {
	tx.retractPredicates = append(tx.retractPredicates, predicate)
}

// RetractFact queues removal of facts matching predicate + first argument.
func (tx *KernelTransaction) RetractFact(fact Fact) {
	tx.retractFacts = append(tx.retractFacts, fact)
}

// RetractExactFact queues removal of facts matching predicate + all arguments.
func (tx *KernelTransaction) RetractExactFact(fact Fact) {
	tx.retractExactFacts = append(tx.retractExactFacts, fact)
}

// RetractPredicateSet queues removal of all facts in a predicate set.
func (tx *KernelTransaction) RetractPredicateSet(predicates map[string]struct{}) {
	for p := range predicates {
		tx.retractPredicateSet[p] = struct{}{}
	}
}

// Assert queues a fact for insertion.
func (tx *KernelTransaction) Assert(fact Fact) {
	tx.assertFacts = append(tx.assertFacts, fact)
}

// Commit applies all buffered operations atomically under a single lock,
// then triggers exactly one rebuild()/evaluate().
func (tx *KernelTransaction) Commit() error {
	if tx.committed {
		return fmt.Errorf("transaction already committed")
	}
	tx.committed = true

	k := tx.kernel
	k.mu.Lock()
	defer k.mu.Unlock()

	timer := logging.StartTimer(logging.CategoryKernel, "Transaction.Commit")
	defer timer.Stop()

	mutated := false

	// Phase 1: Retracts (by full predicate)
	for _, pred := range tx.retractPredicates {
		if tx.retractByPredicateLocked(k, pred) {
			mutated = true
		}
	}

	// Phase 2: Retracts (by predicate set)
	if len(tx.retractPredicateSet) > 0 {
		for _, f := range k.facts {
			if _, ok := tx.retractPredicateSet[f.Predicate]; ok {
				mutated = true
				break
			}
		}
		if mutated {
			filtered := make([]Fact, 0, len(k.facts))
			for _, f := range k.facts {
				if _, ok := tx.retractPredicateSet[f.Predicate]; !ok {
					filtered = append(filtered, f)
				}
			}
			k.facts = filtered
		}
	}

	// Phase 3: Retracts (by predicate + first arg)
	for _, rf := range tx.retractFacts {
		if tx.retractFactLocked(k, rf) {
			mutated = true
		}
	}

	// Phase 4: Retracts (exact match)
	for _, rf := range tx.retractExactFacts {
		if tx.retractExactFactLocked(k, rf) {
			mutated = true
		}
	}

	// Rebuild index after all retracts
	if mutated {
		k.cachedAtoms = nil // Invalidate atom cache
		k.rebuildFactIndexLocked()
	}

	// Phase 5: Asserts
	assertCount := 0
	for _, f := range tx.assertFacts {
		f = sanitizeFactForNumericPredicates(f)
		if k.addFactIfNewLocked(f) {
			assertCount++
		}
	}

	totalOps := len(tx.retractPredicates) + len(tx.retractPredicateSet) +
		len(tx.retractFacts) + len(tx.retractExactFacts) + len(tx.assertFacts)

	logging.KernelDebug("Transaction.Commit: %d ops (%d retracts, %d asserts, %d new), single rebuild",
		totalOps,
		len(tx.retractPredicates)+len(tx.retractPredicateSet)+len(tx.retractFacts)+len(tx.retractExactFacts),
		len(tx.assertFacts),
		assertCount)

	// Phase 6: Single rebuild/evaluate
	if mutated || assertCount > 0 {
		if err := k.rebuild(); err != nil {
			logging.Get(logging.CategoryKernel).Error("Transaction.Commit: rebuild failed: %v", err)
			return err
		}
	}

	return nil
}

// retractByPredicateLocked removes all facts with a predicate. Caller holds k.mu.
func (tx *KernelTransaction) retractByPredicateLocked(k *RealKernel, predicate string) bool {
	n := 0
	retracted := false
	for _, f := range k.facts {
		if f.Predicate != predicate {
			k.facts[n] = f
			n++
		} else {
			retracted = true
		}
	}
	// Zero tail for GC
	for i := n; i < len(k.facts); i++ {
		k.facts[i] = Fact{}
	}
	k.facts = k.facts[:n]
	return retracted
}

// retractFactLocked removes facts matching predicate + first arg. Caller holds k.mu.
func (tx *KernelTransaction) retractFactLocked(k *RealKernel, fact Fact) bool {
	if len(fact.Args) == 0 {
		return tx.retractByPredicateLocked(k, fact.Predicate)
	}
	n := 0
	retracted := false
	for _, f := range k.facts {
		if f.Predicate == fact.Predicate && len(f.Args) > 0 && argsEqual(f.Args[0], fact.Args[0]) {
			retracted = true
		} else {
			k.facts[n] = f
			n++
		}
	}
	for i := n; i < len(k.facts); i++ {
		k.facts[i] = Fact{}
	}
	k.facts = k.facts[:n]
	return retracted
}

// retractExactFactLocked removes facts matching predicate + all args. Caller holds k.mu.
func (tx *KernelTransaction) retractExactFactLocked(k *RealKernel, fact Fact) bool {
	n := 0
	retracted := false
	for _, f := range k.facts {
		if f.Predicate == fact.Predicate && len(f.Args) == len(fact.Args) {
			match := true
			for j := range f.Args {
				if !argsEqual(f.Args[j], fact.Args[j]) {
					match = false
					break
				}
			}
			if match {
				retracted = true
				continue
			}
		}
		k.facts[n] = f
		n++
	}
	for i := n; i < len(k.facts); i++ {
		k.facts[i] = Fact{}
	}
	k.facts = k.facts[:n]
	return retracted
}

// =============================================================================
// GENERAL UTILITY FUNCTIONS
// =============================================================================

// argsEqual compares two fact arguments for equality.
// OPTIMIZATION: Uses type switches instead of expensive fmt.Sprintf fallback.
func argsEqual(a, b interface{}) bool {
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
	case map[string]interface{}:
		// Maps are not comparable with ==, use reflect.DeepEqual
		if bv, ok := b.(map[string]interface{}); ok {
			return reflect.DeepEqual(av, bv)
		}
	case []interface{}:
		// Slices are not comparable with ==, use reflect.DeepEqual
		if bv, ok := b.([]interface{}); ok {
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
		return parsePriorityString(t, v)
	case MangleAtom:
		return parsePriorityString(string(t), v)
	default:
		return v
	}
}

func parsePriorityString(atom string, original interface{}) interface{} {
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
