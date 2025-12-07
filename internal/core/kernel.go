package core

import (
	"codenerd/internal/mangle"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/mangle/analysis"
	"github.com/google/mangle/ast"
	_ "github.com/google/mangle/builtin"
	"github.com/google/mangle/engine"
	"github.com/google/mangle/factstore"
	"github.com/google/mangle/parse"
)

// Fact represents a single logical fact (atom) in the EDB.
type Fact struct {
	Predicate string
	Args      []interface{}
}

// MangleAtom represents a Mangle name constant (starting with /).
// This explicit type avoids ambiguity between strings and atoms.
type MangleAtom string

// String returns the Datalog string representation of the fact.
func (f Fact) String() string {
	var args []string
	for _, arg := range f.Args {
		switch v := arg.(type) {
		case MangleAtom:
			args = append(args, string(v))
		case string:
			// Handle Mangle name constants (start with /)
			if strings.HasPrefix(v, "/") {
				args = append(args, v)
			} else {
				args = append(args, fmt.Sprintf("%q", v))
			}
		case int:
			args = append(args, fmt.Sprintf("%d", v))
		case int64:
			args = append(args, fmt.Sprintf("%d", v))
		case float64:
			args = append(args, fmt.Sprintf("%f", v))
		case bool:
			if v {
				args = append(args, "/true")
			} else {
				args = append(args, "/false")
			}
		default:
			args = append(args, fmt.Sprintf("%v", v))
		}
	}
	return fmt.Sprintf("%s(%s).", f.Predicate, strings.Join(args, ", "))
}

// ToAtom converts a Fact to a Mangle AST Atom for direct store insertion.
func (f Fact) ToAtom() (ast.Atom, error) {
	var terms []ast.BaseTerm
	for _, arg := range f.Args {
		switch v := arg.(type) {
		case MangleAtom:
			c, err := ast.Name(string(v))
			if err != nil {
				return ast.Atom{}, err
			}
			terms = append(terms, c)
		case string:
			if strings.HasPrefix(v, "/") {
				// Name constant
				c, err := ast.Name(v)
				if err != nil {
					return ast.Atom{}, err
				}
				terms = append(terms, c)
			} else {
				// String constant
				terms = append(terms, ast.String(v))
			}
		case int:
			terms = append(terms, ast.Number(int64(v)))
		case int64:
			terms = append(terms, ast.Number(v))
		case float64:
			terms = append(terms, ast.Float64(v))
		case bool:
			if v {
				terms = append(terms, ast.TrueConstant)
			} else {
				terms = append(terms, ast.FalseConstant)
			}
		default:
			terms = append(terms, ast.String(fmt.Sprintf("%v", v)))
		}
	}

	return ast.NewAtom(f.Predicate, terms...), nil
}

// Kernel defines the interface for the logic core.
type Kernel interface {
	LoadFacts(facts []Fact) error
	Query(predicate string) ([]Fact, error)
	QueryAll() (map[string][]Fact, error)
	Assert(fact Fact) error
	Retract(predicate string) error
	RetractFact(fact Fact) error // Retract a specific fact by predicate and first argument
}

// RealKernel wraps the google/mangle engine with proper EDB/IDB separation.
type RealKernel struct {
	mu              sync.RWMutex
	facts           []Fact
	store           factstore.FactStore
	programInfo     *analysis.ProgramInfo
	schemas         string
	policy          string
	learned         string // Learned rules (autopoiesis) - loaded from learned.mg
	schemaValidator *mangle.SchemaValidator
	initialized     bool
	manglePath      string // Path to mangle files directory
	policyDirty     bool   // True when schemas/policy changed and need reparse
}

// NewRealKernel creates a new kernel instance.
func NewRealKernel() *RealKernel {
	k := &RealKernel{
		facts:       make([]Fact, 0),
		store:       factstore.NewSimpleInMemoryStore(),
		policyDirty: true, // Need to parse on first use
	}

	// Find and load mangle files from the project
	k.loadMangleFiles()

	return k
}

// NewRealKernelWithPath creates a kernel with explicit mangle path.
func NewRealKernelWithPath(manglePath string) *RealKernel {
	k := &RealKernel{
		facts:       make([]Fact, 0),
		store:       factstore.NewSimpleInMemoryStore(),
		manglePath:  manglePath,
		policyDirty: true,
	}
	k.loadMangleFiles()
	return k
}

// loadMangleFiles loads schemas and policy from the mangle directory.
func (k *RealKernel) loadMangleFiles() {
	// Try multiple locations for mangle files
	searchPaths := []string{
		k.manglePath,
		"internal/mangle",
		"../internal/mangle",
		"../../internal/mangle",
	}

	for _, basePath := range searchPaths {
		if basePath == "" {
			continue
		}

		schemasPath := filepath.Join(basePath, "schemas.mg")
		policyPath := filepath.Join(basePath, "policy.mg")
		learnedPath := filepath.Join(basePath, "learned.mg")

		if data, err := os.ReadFile(schemasPath); err == nil {
			k.schemas = string(data)
		}
		if data, err := os.ReadFile(policyPath); err == nil {
			k.policy = string(data)
		}
		// Load learned rules (stratified trust layer)
		if data, err := os.ReadFile(learnedPath); err == nil {
			k.learned = string(data)
		}

		if k.schemas != "" && k.policy != "" {
			break
		}
	}

	// Initialize schema validator (Bug #18 Fix - Schema Drift Prevention)
	if k.schemas != "" {
		k.schemaValidator = mangle.NewSchemaValidator(k.schemas, k.learned)
		if err := k.schemaValidator.LoadDeclaredPredicates(); err != nil {
			// Log but don't fail - validator is defensive, not critical
			fmt.Printf("[Kernel] Warning: failed to load schema validator: %v\n", err)
		}
	}
}

// LoadFacts adds facts to the EDB and rebuilds the program.
func (k *RealKernel) LoadFacts(facts []Fact) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.facts = append(k.facts, facts...)
	return k.rebuild()
}

// Assert adds a single fact dynamically and re-evaluates derived facts.
func (k *RealKernel) Assert(fact Fact) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.facts = append(k.facts, fact)
	return k.evaluate()
}

// AssertBatch adds multiple facts and evaluates once (more efficient).
func (k *RealKernel) AssertBatch(facts []Fact) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.facts = append(k.facts, facts...)
	return k.evaluate()
}

// AssertWithoutEval adds a fact without re-evaluating.
// Use when batching many facts, then call Evaluate() once at the end.
func (k *RealKernel) AssertWithoutEval(fact Fact) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.facts = append(k.facts, fact)
}

// Evaluate forces re-evaluation of all rules. Call after AssertWithoutEval batch.
func (k *RealKernel) Evaluate() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.evaluate()
}

// Retract removes all facts of a given predicate.
func (k *RealKernel) Retract(predicate string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	filtered := make([]Fact, 0)
	for _, f := range k.facts {
		if f.Predicate != predicate {
			filtered = append(filtered, f)
		}
	}
	k.facts = filtered
	return k.rebuild()
}

// RetractFact removes a specific fact by matching predicate and first argument.
// This enables selective fact removal (e.g., removing all facts for a specific tool).
func (k *RealKernel) RetractFact(fact Fact) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if len(fact.Args) == 0 {
		return fmt.Errorf("fact must have at least one argument for matching")
	}

	filtered := make([]Fact, 0)
	for _, f := range k.facts {
		// Keep facts that don't match predicate OR don't match first argument
		if f.Predicate != fact.Predicate {
			filtered = append(filtered, f)
			continue
		}
		// Same predicate - check first argument
		if len(f.Args) > 0 && len(fact.Args) > 0 {
			if !argsEqual(f.Args[0], fact.Args[0]) {
				filtered = append(filtered, f)
			}
			// Matching predicate and first arg - don't add (retract it)
		} else {
			filtered = append(filtered, f)
		}
	}
	k.facts = filtered
	return k.rebuild()
}

// argsEqual compares two fact arguments for equality.
func argsEqual(a, b interface{}) bool {
	switch av := a.(type) {
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
	case int64:
		if bv, ok := b.(int64); ok {
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
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// rebuildProgram parses schemas+policy and caches programInfo.
// This is only called when policyDirty is true.
func (k *RealKernel) rebuildProgram() error {
	// Construct program from schemas + policy + learned (no facts)
	// STRATIFIED TRUST: Load order ensures Constitution has priority
	var sb strings.Builder

	if k.schemas != "" {
		sb.WriteString(k.schemas)
		sb.WriteString("\n")
	}

	if k.policy != "" {
		sb.WriteString(k.policy)
		sb.WriteString("\n")
	}

	// Load learned rules AFTER constitution (stratified trust)
	if k.learned != "" {
		sb.WriteString("# Learned Rules (Autopoiesis Layer - Stratified Trust)\n")
		sb.WriteString(k.learned)
	}

	programStr := sb.String()

	// Parse
	parsed, err := parse.Unit(strings.NewReader(programStr))
	if err != nil {
		return fmt.Errorf("failed to parse program: %w", err)
	}

	// Analyze
	programInfo, err := analysis.AnalyzeOneUnit(parsed, nil)
	if err != nil {
		return fmt.Errorf("failed to analyze program: %w", err)
	}

	k.programInfo = programInfo
	k.policyDirty = false
	return nil
}

// evaluate populates the store with facts and evaluates to fixpoint.
// Uses cached programInfo for efficiency.
func (k *RealKernel) evaluate() error {
	// Rebuild program if policy changed
	if k.policyDirty || k.programInfo == nil {
		if err := k.rebuildProgram(); err != nil {
			return err
		}
	}

	// Create fresh store and populate with EDB facts
	k.store = factstore.NewSimpleInMemoryStore()
	for _, f := range k.facts {
		atom, err := f.ToAtom()
		if err != nil {
			return fmt.Errorf("failed to convert fact %v: %w", f, err)
		}
		k.store.Add(atom)
	}

	// Evaluate to fixpoint using cached programInfo
	// BUG #17 FIX: Add gas limits to prevent halting problem in learned rules
	// Prevent fact explosions from recursive learned rules
	_, err := engine.EvalProgramWithStats(k.programInfo, k.store,
		engine.WithCreatedFactLimit(50000)) // Hard cap: max 50K derived facts
	if err != nil {
		return fmt.Errorf("failed to evaluate program: %w", err)
	}

	k.initialized = true
	return nil
}

// rebuild is kept for backward compatibility - now delegates to evaluate().
func (k *RealKernel) rebuild() error {
	return k.evaluate()
}

// Query retrieves all facts matching a predicate pattern.
func (k *RealKernel) Query(predicate string) ([]Fact, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if !k.initialized {
		return nil, fmt.Errorf("kernel not initialized")
	}

	results := make([]Fact, 0)

	// Get the predicate symbol from the program
	if k.programInfo == nil {
		return results, nil
	}

	// Find the predicate in the decls
	for pred := range k.programInfo.Decls {
		if pred.Symbol == predicate {
			// Query the store for all atoms of this predicate
			k.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
				fact := atomToFact(a)
				results = append(results, fact)
				return nil
			})
			break
		}
	}

	return results, nil
}

// QueryAll retrieves all derived facts organized by predicate.
func (k *RealKernel) QueryAll() (map[string][]Fact, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if !k.initialized {
		return nil, fmt.Errorf("kernel not initialized")
	}

	results := make(map[string][]Fact)

	if k.programInfo == nil {
		return results, nil
	}

	// Iterate through all declared predicates
	for pred := range k.programInfo.Decls {
		predName := pred.Symbol
		results[predName] = make([]Fact, 0)

		k.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
			fact := atomToFact(a)
			results[predName] = append(results[predName], fact)
			return nil
		})
	}

	return results, nil
}

// ParseSingleFact parses a single fact string safely.
func ParseSingleFact(content string) (Fact, error) {
	facts, err := ParseFactsFromString(content)
	if err != nil {
		return Fact{}, err
	}
	if len(facts) == 0 {
		return Fact{}, fmt.Errorf("no facts found")
	}
	if len(facts) > 1 {
		return Fact{}, fmt.Errorf("multiple facts found")
	}
	return facts[0], nil
}

// atomToFact converts a Mangle AST Atom back to our Fact type.
func atomToFact(a ast.Atom) Fact {
	args := make([]interface{}, len(a.Args))
	for i, term := range a.Args {
		args[i] = baseTermToValue(term)
	}
	return Fact{
		Predicate: a.Predicate.Symbol,
		Args:      args,
	}
}

// baseTermToValue extracts the Go value from a Mangle BaseTerm.
func baseTermToValue(term ast.BaseTerm) interface{} {
	switch t := term.(type) {
	case ast.Constant:
		switch t.Type {
		case ast.NameType:
			return t.Symbol
		case ast.StringType:
			return t.Symbol
		case ast.NumberType:
			return t.NumValue
		case ast.Float64Type:
			return t.Float64Value
		default:
			return t.Symbol
		}
	case ast.Variable:
		return fmt.Sprintf("?%s", t.Symbol)
	default:
		return fmt.Sprintf("%v", term)
	}
}

// GetStore returns the underlying FactStore for advanced operations.
func (k *RealKernel) GetStore() factstore.FactStore {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.store
}

// SetSchemas allows loading custom schemas (for testing or shard isolation).
func (k *RealKernel) SetSchemas(schemas string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.schemas = schemas
	k.policyDirty = true
}

// =============================================================================
// SCHEMA VALIDATION (Bug #18 Fix - Schema Drift Prevention)
// =============================================================================

// ValidateLearnedRule validates that a learned rule only uses declared predicates.
// This prevents "Schema Drift" where the agent invents predicates with no data source.
func (k *RealKernel) ValidateLearnedRule(ruleText string) error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		// Validator not initialized - allow (defensive)
		return nil
	}

	return k.schemaValidator.ValidateRule(ruleText)
}

// ValidateLearnedRules validates multiple learned rules.
// Returns a list of errors (one per invalid rule).
func (k *RealKernel) ValidateLearnedRules(rules []string) []error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return nil
	}

	return k.schemaValidator.ValidateRules(rules)
}

// ValidateLearnedProgram validates an entire learned program text.
func (k *RealKernel) ValidateLearnedProgram(programText string) error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return nil
	}

	return k.schemaValidator.ValidateProgram(programText)
}

// IsPredicateDeclared checks if a predicate is declared in schemas.
func (k *RealKernel) IsPredicateDeclared(predicate string) bool {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return false
	}

	return k.schemaValidator.IsDeclared(predicate)
}

// GetDeclaredPredicates returns all declared predicate names.
func (k *RealKernel) GetDeclaredPredicates() []string {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return nil
	}

	return k.schemaValidator.GetDeclaredPredicates()
}

// SetPolicy allows loading custom policy rules (for shard specialization).
func (k *RealKernel) SetPolicy(policy string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.policy = policy
	k.policyDirty = true
}

// AppendPolicy appends additional policy rules (for shard-specific policies).
func (k *RealKernel) AppendPolicy(additionalPolicy string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.policy = k.policy + "\n\n# Appended Policy\n" + additionalPolicy
	k.policyDirty = true
}

// LoadPolicyFile loads policy rules from a file and appends them.
func (k *RealKernel) LoadPolicyFile(path string) error {
	// Try multiple search paths
	searchPaths := []string{
		path,
		filepath.Join("internal/mangle", filepath.Base(path)),
		filepath.Join("../internal/mangle", filepath.Base(path)),
		filepath.Join("../../internal/mangle", filepath.Base(path)),
	}

	for _, p := range searchPaths {
		data, err := os.ReadFile(p)
		if err == nil {
			k.AppendPolicy(string(data))
			return nil
		}
	}

	return fmt.Errorf("policy file not found: %s", path)
}

// HotLoadRule dynamically loads a single Mangle rule at runtime.
// This is used by Autopoiesis to add new rules without restarting.
// FIX for Bug #8 (Suicide Rule): Uses a "Sandbox Compiler" to validate the rule
// before accepting it, preventing invalid rules from bricking the kernel.
func (k *RealKernel) HotLoadRule(rule string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if rule == "" {
		return fmt.Errorf("empty rule")
	}

	// 1. Create a Sandbox Kernel (Memory only)
	sandbox := &RealKernel{
		store:       factstore.NewSimpleInMemoryStore(),
		policyDirty: true,
	}

	// 2. Load CURRENT schemas and policy into sandbox
	sandbox.schemas = k.schemas
	sandbox.policy = k.policy

	// 3. Apply the NEW rule to the sandbox
	sandbox.policy = sandbox.policy + "\n\n# Sandbox Validation\n" + rule

	// 4. Try to compile (rebuildProgram)
	// This will fail with StratificationError if the rule creates a paradox
	if err := sandbox.rebuildProgram(); err != nil {
		return fmt.Errorf("rule rejected by sandbox compiler: %w", err)
	}

	// 5. If successful, apply to Real Kernel (in-memory)
	k.learned = k.learned + "\n\n# HotLoaded Rule\n" + rule
	k.policyDirty = true

	return nil
}

// HotLoadLearnedRule dynamically loads a learned rule and persists it to learned.mg.
// This is the primary method for Autopoiesis to add new learned rules.
// It validates the rule, loads it into memory, and writes it to disk for persistence.
func (k *RealKernel) HotLoadLearnedRule(rule string) error {
	// 1. Validate using sandbox (same as HotLoadRule)
	if err := k.HotLoadRule(rule); err != nil {
		return err
	}

	// 2. Persist to learned.mg file
	return k.appendToLearnedFile(rule)
}

// appendToLearnedFile appends a rule to learned.mg on disk.
func (k *RealKernel) appendToLearnedFile(rule string) error {
	// Find learned.mg path
	searchPaths := []string{
		k.manglePath,
		"internal/mangle",
		"../internal/mangle",
		"../../internal/mangle",
	}

	var learnedPath string
	for _, basePath := range searchPaths {
		if basePath == "" {
			continue
		}
		path := filepath.Join(basePath, "learned.mg")
		if _, err := os.Stat(path); err == nil {
			learnedPath = path
			break
		}
	}

	if learnedPath == "" {
		return fmt.Errorf("learned.mg file not found")
	}

	// Append rule to file
	f, err := os.OpenFile(learnedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open learned.mg: %w", err)
	}
	defer f.Close()

	// Write rule with proper formatting
	_, err = f.WriteString(fmt.Sprintf("\n# Autopoiesis-learned rule (added %s)\n%s\n",
		time.Now().Format("2006-01-02 15:04:05"), rule))
	if err != nil {
		return fmt.Errorf("failed to write to learned.mg: %w", err)
	}

	return nil
}

// GetLearned returns the current learned rules.
func (k *RealKernel) GetLearned() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.learned
}

// SetLearned allows loading custom learned rules (for testing).
func (k *RealKernel) SetLearned(learned string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.learned = learned
	k.policyDirty = true
}

// GetSchemas returns the current schemas.
func (k *RealKernel) GetSchemas() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.schemas
}

// GetPolicy returns the current policy.
func (k *RealKernel) GetPolicy() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.policy
}

// GetFactsSnapshot returns a copy of currently asserted facts.
func (k *RealKernel) GetFactsSnapshot() []Fact {
	k.mu.RLock()
	defer k.mu.RUnlock()
	facts := make([]Fact, len(k.facts))
	copy(facts, k.facts)
	return facts
}

// Clone creates a new kernel with the same schemas, policy, learned rules, and facts.
// The cloned kernel shares no mutable state with the original.
func (k *RealKernel) Clone() *RealKernel {
	clone := NewRealKernel()
	clone.SetSchemas(k.GetSchemas())
	clone.SetPolicy(k.GetPolicy())
	clone.SetLearned(k.GetLearned())

	// Copy facts into the clone
	facts := k.GetFactsSnapshot()
	if len(facts) > 0 {
		_ = clone.LoadFacts(facts)
	}

	return clone
}

// Clear resets the kernel to empty state (keeps cached programInfo).
func (k *RealKernel) Clear() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.facts = make([]Fact, 0)
	k.store = factstore.NewSimpleInMemoryStore()
	k.initialized = false
	// Note: programInfo and policyDirty preserved - only facts cleared
}

// Reset fully resets the kernel including cached program.
func (k *RealKernel) Reset() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.facts = make([]Fact, 0)
	k.store = factstore.NewSimpleInMemoryStore()
	k.programInfo = nil
	k.policyDirty = true
	k.initialized = false
}

// IsInitialized returns whether the kernel has been initialized.
func (k *RealKernel) IsInitialized() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.initialized
}

// FactCount returns the number of facts loaded.
func (k *RealKernel) FactCount() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return len(k.facts)
}

// GetAllFacts returns a copy of all facts in the kernel.
// SAFE FOR PERSISTENCE: This returns only the EDB (Base Facts) explicitly loaded.
// It does NOT return derived facts (IDB). Use this for saving state (Fix for Bug #9).
func (k *RealKernel) GetAllFacts() []Fact {
	k.mu.RLock()
	defer k.mu.RUnlock()
	result := make([]Fact, len(k.facts))
	copy(result, k.facts)
	return result
}

// GetDerivedFacts returns all facts derived by rules (IDB).
// WARNING: Do NOT persist these. They should be re-derived on boot.
func (k *RealKernel) GetDerivedFacts() (map[string][]Fact, error) {
	return k.QueryAll()
}

// LoadFactsFromFile loads facts from a .mg file into the kernel.
// This parses the file and extracts EDB facts to load.
func (k *RealKernel) LoadFactsFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read facts file: %w", err)
	}

	// Parse the facts from the file content
	facts, err := ParseFactsFromString(string(data))
	if err != nil {
		return fmt.Errorf("failed to parse facts: %w", err)
	}

	return k.LoadFacts(facts)
}

// UpdateSystemFacts updates transient system facts like time.
// This should be called ONCE per turn/request to avoid infinite loops
// in logic that depends on changing time (Fix for Bug #7).
func (k *RealKernel) UpdateSystemFacts() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	// 1. Retract old system facts
	newFacts := make([]Fact, 0, len(k.facts)+1)
	for _, f := range k.facts {
		if f.Predicate != "current_time" {
			newFacts = append(newFacts, f)
		}
	}
	k.facts = newFacts

	// 2. Add fresh system facts
	now := time.Now().Unix()
	k.facts = append(k.facts, Fact{
		Predicate: "current_time",
		Args:      []interface{}{now},
	})

	// 3. Re-evaluate
	// We use evaluate() directly since we already hold the lock
	return k.evaluate()
}

// ParseFactsFromString parses Mangle fact statements from a string.
// Uses the official Mangle parser to ensure safety (Fix for Bug #11).
func ParseFactsFromString(content string) ([]Fact, error) {
	// Use the official parser to parse the content as a Unit
	unit, err := parse.Unit(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse facts string: %w", err)
	}

	facts := make([]Fact, 0)
	for _, clause := range unit.Clauses {
		// A fact is a clause with no body
		if len(clause.Premises) > 0 {
			continue // Skip rules
		}

		// Convert the head atom to our Fact type
		facts = append(facts, atomToFact(clause.Head))
	}

	return facts, nil
}
