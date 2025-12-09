package core

import (
	"codenerd/internal/autopoiesis"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"embed"
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
	UpdateSystemFacts() error    // Updates system-level facts (time, OS, etc.)
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
	workspaceRoot   string // Explicit workspace root (for .nerd paths)
	policyDirty     bool   // True when schemas/policy changed and need reparse
}

//go:embed defaults/*.mg defaults/schema/*.mg
var coreLogic embed.FS

// GetDefaultContent returns the content of an embedded default file.
// Path should be relative to defaults/ (e.g. "schemas.mg" or "schema/intent.mg").
func GetDefaultContent(path string) (string, error) {
	data, err := coreLogic.ReadFile("defaults/" + path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// NewRealKernel creates a new kernel instance.
func NewRealKernel() *RealKernel {
	timer := logging.StartTimer(logging.CategoryKernel, "NewRealKernel")
	logging.Kernel("Initializing new RealKernel instance")

	k := &RealKernel{
		facts:       make([]Fact, 0),
		store:       factstore.NewSimpleInMemoryStore(),
		policyDirty: true, // Need to parse on first use
	}
	logging.KernelDebug("Kernel struct created, store initialized, policyDirty=true")

	// Find and load mangle files from the project
	k.loadMangleFiles()

	// Force initial evaluation to boot the Mangle engine.
	// The embedded core MUST compile, otherwise the binary is corrupt.
	logging.Kernel("Booting Mangle engine with embedded constitution...")
	if err := k.evaluate(); err != nil {
		logging.Get(logging.CategoryKernel).Error("CRITICAL: Kernel boot failed: %v", err)
		panic(fmt.Sprintf("CRITICAL: Kernel failed to boot embedded constitution: %v", err))
	}

	timer.StopWithInfo()
	logging.Kernel("Kernel initialized successfully")
	return k
}

// NewRealKernelWithPath creates a kernel with explicit mangle path.
func NewRealKernelWithPath(manglePath string) *RealKernel {
	timer := logging.StartTimer(logging.CategoryKernel, "NewRealKernelWithPath")
	logging.Kernel("Initializing RealKernel with explicit path: %s", manglePath)

	k := &RealKernel{
		facts:       make([]Fact, 0),
		store:       factstore.NewSimpleInMemoryStore(),
		manglePath:  manglePath,
		policyDirty: true,
	}
	logging.KernelDebug("Kernel struct created with manglePath=%s", manglePath)

	k.loadMangleFiles()

	// Force initial evaluation
	logging.Kernel("Booting Mangle engine...")
	if err := k.evaluate(); err != nil {
		logging.Get(logging.CategoryKernel).Error("CRITICAL: Kernel boot failed (path: %s): %v", manglePath, err)
		panic(fmt.Sprintf("CRITICAL: Kernel failed to boot (path: %s): %v", manglePath, err))
	}

	timer.StopWithInfo()
	logging.Kernel("Kernel with path initialized successfully")
	return k
}

// SetWorkspace sets the explicit workspace root path for .nerd directory resolution.
// This MUST be called after kernel creation to ensure .nerd paths resolve correctly.
// If not set, paths will be resolved relative to CWD (which may be incorrect).
func (k *RealKernel) SetWorkspace(root string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.workspaceRoot = root
}

// GetWorkspace returns the workspace root, or empty string if not set.
func (k *RealKernel) GetWorkspace() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.workspaceRoot
}

// nerdPath returns the correct path for a .nerd subdirectory.
// Uses workspaceRoot if set, otherwise returns relative path (legacy behavior).
func (k *RealKernel) nerdPath(subpath string) string {
	if k.workspaceRoot != "" {
		return filepath.Join(k.workspaceRoot, ".nerd", subpath)
	}
	return filepath.Join(".nerd", subpath)
}

// loadMangleFiles loads schemas and policy from the embedded core and user extensions.
func (k *RealKernel) loadMangleFiles() {
	timer := logging.StartTimer(logging.CategoryKernel, "loadMangleFiles")
	logging.Kernel("Loading Mangle files (schemas, policy, learned rules)")

	// 1. LOAD BAKED-IN CORE (Immutable Physics)
	// Always load these. They are the "Constitution".
	logging.KernelDebug("Loading baked-in core (Constitution)...")

	// Load Core Schemas
	if data, err := coreLogic.ReadFile("defaults/schemas.mg"); err == nil {
		k.schemas = string(data)
		logging.KernelDebug("Loaded core schemas (%d bytes)", len(data))
	} else {
		logging.Get(logging.CategoryKernel).Error("Failed to load core schemas: %v", err)
	}

	// Load Core Policy
	if data, err := coreLogic.ReadFile("defaults/policy.mg"); err == nil {
		k.policy = string(data)
		logging.KernelDebug("Loaded core policy (%d bytes)", len(data))
	} else {
		logging.Get(logging.CategoryKernel).Error("Failed to load core policy: %v", err)
	}

	// Load other core modules into policy
	coreModules := []string{
		"doc_taxonomy.mg",
		"topology_planner.mg",
		"build_topology.mg",
		"campaign_rules.mg",
		"selection_policy.mg",
		"taxonomy.mg",
		"inference.mg",
	}

	loadedModules := 0
	for _, mod := range coreModules {
		if data, err := coreLogic.ReadFile("defaults/" + mod); err == nil {
			k.policy += "\n\n" + string(data)
			loadedModules++
			logging.KernelDebug("Loaded core module: %s (%d bytes)", mod, len(data))
		} else {
			logging.KernelDebug("Core module not found (optional): %s", mod)
		}
	}
	logging.KernelDebug("Loaded %d/%d core modules", loadedModules, len(coreModules))

	// Load base learned rules (if any)
	if data, err := coreLogic.ReadFile("defaults/learned.mg"); err == nil {
		k.learned = string(data)
		logging.KernelDebug("Loaded base learned rules (%d bytes)", len(data))
	} else {
		logging.KernelDebug("No base learned rules found (this is normal for fresh installs)")
	}

	// 2. LOAD USER EXTENSIONS (Project Specifics)
	// Look in the workspace's .nerd folder or explicit manglePath
	logging.KernelDebug("Loading user extensions...")
	workspacePaths := []string{
		k.nerdPath("mangle"),
		k.manglePath,
	}

	userExtensionsLoaded := 0
	for _, wsPath := range workspacePaths {
		if wsPath == "" {
			continue
		}
		logging.KernelDebug("Checking user extension path: %s", wsPath)

		// Append User Schemas (extensions.mg)
		extPath := filepath.Join(wsPath, "extensions.mg")
		if data, err := os.ReadFile(extPath); err == nil {
			k.schemas += "\n\n# User Extensions\n" + string(data)
			userExtensionsLoaded++
			logging.Kernel("Loaded user schema extensions from %s (%d bytes)", extPath, len(data))
		}

		// Append User Policy (policy_overrides.mg)
		policyPath := filepath.Join(wsPath, "policy_overrides.mg")
		if data, err := os.ReadFile(policyPath); err == nil {
			k.policy += "\n\n# User Policy Overrides\n" + string(data)
			userExtensionsLoaded++
			logging.Kernel("Loaded user policy overrides from %s (%d bytes)", policyPath, len(data))
		}

		// Append User Learned Rules (learned.mg)
		learnedPath := filepath.Join(wsPath, "learned.mg")
		if data, err := os.ReadFile(learnedPath); err == nil {
			// User learned rules append to base learned rules
			k.learned += "\n\n# User Learned Rules\n" + string(data)
			userExtensionsLoaded++
			logging.Kernel("Loaded user learned rules from %s (%d bytes)", learnedPath, len(data))
		}
	}
	logging.KernelDebug("Loaded %d user extension files", userExtensionsLoaded)

	// Initialize schema validator (Bug #18 Fix - Schema Drift Prevention)
	if k.schemas != "" {
		logging.KernelDebug("Initializing schema validator...")
		k.schemaValidator = mangle.NewSchemaValidator(k.schemas, k.learned)
		if err := k.schemaValidator.LoadDeclaredPredicates(); err != nil {
			// Log but don't fail - validator is defensive, not critical
			logging.Get(logging.CategoryKernel).Warn("Failed to load schema validator: %v", err)
		} else {
			logging.KernelDebug("Schema validator initialized successfully")
		}
	}

	timer.Stop()
	logging.Kernel("Mangle files loaded: schemas=%d bytes, policy=%d bytes, learned=%d bytes",
		len(k.schemas), len(k.policy), len(k.learned))
}

// LoadFacts adds facts to the EDB and rebuilds the program.
func (k *RealKernel) LoadFacts(facts []Fact) error {
	timer := logging.StartTimer(logging.CategoryKernel, "LoadFacts")
	logging.Kernel("LoadFacts: loading %d facts into EDB", len(facts))

	k.mu.Lock()
	defer k.mu.Unlock()

	prevCount := len(k.facts)
	k.facts = append(k.facts, facts...)
	logging.KernelDebug("LoadFacts: EDB grew from %d to %d facts", prevCount, len(k.facts))

	// Log sample of facts being loaded (first 5)
	if len(facts) > 0 && logging.IsDebugMode() {
		sampleSize := 5
		if len(facts) < sampleSize {
			sampleSize = len(facts)
		}
		for i := 0; i < sampleSize; i++ {
			logging.KernelDebug("  [%d] %s", i, facts[i].String())
		}
		if len(facts) > sampleSize {
			logging.KernelDebug("  ... and %d more facts", len(facts)-sampleSize)
		}
	}

	err := k.rebuild()
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("LoadFacts: rebuild failed: %v", err)
		return err
	}

	timer.Stop()
	return nil
}

// Assert adds a single fact dynamically and re-evaluates derived facts.
func (k *RealKernel) Assert(fact Fact) error {
	logging.KernelDebug("Assert: %s", fact.String())

	k.mu.Lock()
	defer k.mu.Unlock()

	k.facts = append(k.facts, fact)
	if err := k.evaluate(); err != nil {
		logging.Get(logging.CategoryKernel).Error("Assert: evaluation failed after asserting %s: %v", fact.Predicate, err)
		return err
	}
	logging.KernelDebug("Assert: fact added successfully, total facts=%d", len(k.facts))
	return nil
}

// AssertBatch adds multiple facts and evaluates once (more efficient).
func (k *RealKernel) AssertBatch(facts []Fact) error {
	timer := logging.StartTimer(logging.CategoryKernel, "AssertBatch")
	logging.KernelDebug("AssertBatch: adding %d facts", len(facts))

	k.mu.Lock()
	defer k.mu.Unlock()

	prevCount := len(k.facts)
	k.facts = append(k.facts, facts...)

	if err := k.evaluate(); err != nil {
		logging.Get(logging.CategoryKernel).Error("AssertBatch: evaluation failed after adding %d facts: %v", len(facts), err)
		return err
	}

	timer.Stop()
	logging.KernelDebug("AssertBatch: EDB grew from %d to %d facts", prevCount, len(k.facts))
	return nil
}

// AssertWithoutEval adds a fact without re-evaluating.
// Use when batching many facts, then call Evaluate() once at the end.
func (k *RealKernel) AssertWithoutEval(fact Fact) {
	logging.KernelDebug("AssertWithoutEval: %s (deferred evaluation)", fact.Predicate)
	k.mu.Lock()
	defer k.mu.Unlock()
	k.facts = append(k.facts, fact)
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
func (k *RealKernel) Retract(predicate string) error {
	logging.KernelDebug("Retract: removing all facts with predicate=%s", predicate)

	k.mu.Lock()
	defer k.mu.Unlock()

	prevCount := len(k.facts)
	filtered := make([]Fact, 0)
	retractedCount := 0
	for _, f := range k.facts {
		if f.Predicate != predicate {
			filtered = append(filtered, f)
		} else {
			retractedCount++
		}
	}
	k.facts = filtered

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
	filtered := make([]Fact, 0)
	retractedCount := 0
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
			} else {
				retractedCount++
			}
			// Matching predicate and first arg - don't add (retract it)
		} else {
			filtered = append(filtered, f)
		}
	}
	k.facts = filtered

	logging.KernelDebug("RetractFact: removed %d facts, EDB: %d -> %d facts",
		retractedCount, prevCount, len(k.facts))

	if err := k.rebuild(); err != nil {
		logging.Get(logging.CategoryKernel).Error("RetractFact: rebuild failed: %v", err)
		return err
	}
	return nil
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
	timer := logging.StartTimer(logging.CategoryKernel, "rebuildProgram")
	logging.Kernel("Rebuilding Mangle program (parsing schemas+policy+learned)")

	// Construct program from schemas + policy + learned (no facts)
	// STRATIFIED TRUST: Load order ensures Constitution has priority
	var sb strings.Builder

	if k.schemas != "" {
		sb.WriteString(k.schemas)
		sb.WriteString("\n")
		logging.KernelDebug("rebuildProgram: included schemas (%d bytes)", len(k.schemas))
	}

	if k.policy != "" {
		sb.WriteString(k.policy)
		sb.WriteString("\n")
		logging.KernelDebug("rebuildProgram: included policy (%d bytes)", len(k.policy))
	}

	// Load learned rules AFTER constitution (stratified trust)
	if k.learned != "" {
		sb.WriteString("# Learned Rules (Autopoiesis Layer - Stratified Trust)\n")
		sb.WriteString(k.learned)
		logging.KernelDebug("rebuildProgram: included learned rules (%d bytes)", len(k.learned))
	}

	programStr := sb.String()
	logging.KernelDebug("rebuildProgram: total program size = %d bytes", len(programStr))

	// Parse
	parseTimer := logging.StartTimer(logging.CategoryKernel, "rebuildProgram.parse")
	parsed, err := parse.Unit(strings.NewReader(programStr))
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("rebuildProgram: parse failed: %v", err)
		return fmt.Errorf("failed to parse program: %w", err)
	}
	parseTimer.Stop()
	logging.KernelDebug("rebuildProgram: parsed %d clauses", len(parsed.Clauses))

	// Analyze
	analyzeTimer := logging.StartTimer(logging.CategoryKernel, "rebuildProgram.analyze")
	programInfo, err := analysis.AnalyzeOneUnit(parsed, nil)
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("rebuildProgram: analysis failed: %v", err)
		return fmt.Errorf("failed to analyze program: %w", err)
	}
	analyzeTimer.Stop()

	k.programInfo = programInfo
	k.policyDirty = false

	// Log predicate count
	declCount := 0
	if programInfo.Decls != nil {
		declCount = len(programInfo.Decls)
	}
	logging.KernelDebug("rebuildProgram: analysis complete, %d predicates declared", declCount)

	timer.StopWithInfo()
	logging.Kernel("Mangle program rebuilt successfully")
	return nil
}

// evaluate populates the store with facts and evaluates to fixpoint.
// Uses cached programInfo for efficiency.
func (k *RealKernel) evaluate() error {
	timer := logging.StartTimer(logging.CategoryKernel, "evaluate")

	// Rebuild program if policy changed
	if k.policyDirty || k.programInfo == nil {
		logging.KernelDebug("evaluate: policy dirty or programInfo nil, rebuilding program")
		if err := k.rebuildProgram(); err != nil {
			return err
		}
	} else {
		logging.KernelDebug("evaluate: using cached programInfo")
	}

	// Create fresh store and populate with EDB facts
	logging.KernelDebug("evaluate: populating store with %d EDB facts", len(k.facts))
	k.store = factstore.NewSimpleInMemoryStore()
	factConversionErrors := 0
	for _, f := range k.facts {
		atom, err := f.ToAtom()
		if err != nil {
			factConversionErrors++
			logging.Get(logging.CategoryKernel).Error("evaluate: failed to convert fact %s: %v", f.Predicate, err)
			return fmt.Errorf("failed to convert fact %v: %w", f, err)
		}
		k.store.Add(atom)
	}

	// Evaluate to fixpoint using cached programInfo
	// BUG #17 FIX: Add gas limits to prevent halting problem in learned rules
	// Prevent fact explosions from recursive learned rules
	const derivedFactLimit = 500000
	logging.KernelDebug("evaluate: running fixpoint evaluation (derivedFactLimit=%d)", derivedFactLimit)

	evalTimer := logging.StartTimer(logging.CategoryKernel, "evaluate.fixpoint")
	stats, err := engine.EvalProgramWithStats(k.programInfo, k.store,
		engine.WithCreatedFactLimit(derivedFactLimit)) // Hard cap: max 500K derived facts
	evalDuration := evalTimer.Stop()

	if err != nil {
		logging.Get(logging.CategoryKernel).Error("evaluate: fixpoint evaluation failed: %v", err)
		// Check if this is a derived fact limit error
		if strings.Contains(err.Error(), "limit") || strings.Contains(err.Error(), "exceeded") {
			logging.Get(logging.CategoryKernel).Warn("evaluate: POSSIBLE FACT EXPLOSION - derived facts exceeded %d limit", derivedFactLimit)
		}
		return fmt.Errorf("failed to evaluate program: %w", err)
	}

	// Log evaluation stats
	totalDuration := time.Duration(0)
	for _, d := range stats.Duration {
		totalDuration += d
	}
	strataCount := len(stats.Strata)
	logging.KernelDebug("evaluate: fixpoint reached - strata=%d, evalTime=%v, wallTime=%v",
		strataCount, totalDuration, evalDuration)

	k.initialized = true
	timer.Stop()
	logging.KernelDebug("evaluate: complete, kernel initialized")
	return nil
}

// rebuild is kept for backward compatibility - now delegates to evaluate().
func (k *RealKernel) rebuild() error {
	logging.KernelDebug("rebuild: delegating to evaluate()")
	return k.evaluate()
}

// Query retrieves all facts matching a predicate pattern.
func (k *RealKernel) Query(predicate string) ([]Fact, error) {
	timer := logging.StartTimer(logging.CategoryKernel, "Query")
	logging.KernelDebug("Query: predicate=%s", predicate)

	k.mu.RLock()
	defer k.mu.RUnlock()

	if !k.initialized {
		err := fmt.Errorf("kernel not initialized")
		logging.Get(logging.CategoryKernel).Error("Query: %v", err)
		return nil, err
	}

	results := make([]Fact, 0)

	// Get the predicate symbol from the program
	if k.programInfo == nil {
		logging.KernelDebug("Query: programInfo is nil, returning empty results")
		timer.Stop()
		return results, nil
	}

	// Find the predicate in the decls
	predicateFound := false
	for pred := range k.programInfo.Decls {
		if pred.Symbol == predicate {
			predicateFound = true
			// Query the store for all atoms of this predicate
			k.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
				fact := atomToFact(a)
				results = append(results, fact)
				return nil
			})
			break
		}
	}

	if !predicateFound {
		logging.KernelDebug("Query: predicate '%s' not found in declarations", predicate)
	}

	timer.Stop()
	logging.KernelDebug("Query: predicate=%s returned %d results", predicate, len(results))
	return results, nil
}

// QueryAll retrieves all derived facts organized by predicate.
func (k *RealKernel) QueryAll() (map[string][]Fact, error) {
	timer := logging.StartTimer(logging.CategoryKernel, "QueryAll")
	logging.KernelDebug("QueryAll: retrieving all derived facts")

	k.mu.RLock()
	defer k.mu.RUnlock()

	if !k.initialized {
		err := fmt.Errorf("kernel not initialized")
		logging.Get(logging.CategoryKernel).Error("QueryAll: %v", err)
		return nil, err
	}

	results := make(map[string][]Fact)

	if k.programInfo == nil {
		logging.KernelDebug("QueryAll: programInfo is nil, returning empty results")
		timer.Stop()
		return results, nil
	}

	// Iterate through all declared predicates
	totalFacts := 0
	for pred := range k.programInfo.Decls {
		predName := pred.Symbol
		results[predName] = make([]Fact, 0)

		k.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
			fact := atomToFact(a)
			results[predName] = append(results[predName], fact)
			totalFacts++
			return nil
		})
	}

	timer.Stop()
	logging.KernelDebug("QueryAll: returned %d predicates with %d total facts", len(results), totalFacts)
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
	logging.KernelDebug("SetSchemas: loading custom schemas (%d bytes)", len(schemas))
	k.mu.Lock()
	defer k.mu.Unlock()
	k.schemas = schemas
	k.policyDirty = true
	logging.KernelDebug("SetSchemas: policyDirty set to true, will rebuild on next evaluate")
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
	logging.KernelDebug("SetPolicy: loading custom policy (%d bytes)", len(policy))
	k.mu.Lock()
	defer k.mu.Unlock()
	k.policy = policy
	k.policyDirty = true
	logging.KernelDebug("SetPolicy: policyDirty set to true")
}

// AppendPolicy appends additional policy rules (for shard-specific policies).
func (k *RealKernel) AppendPolicy(additionalPolicy string) {
	logging.KernelDebug("AppendPolicy: appending %d bytes to existing policy", len(additionalPolicy))
	k.mu.Lock()
	defer k.mu.Unlock()
	prevLen := len(k.policy)
	k.policy = k.policy + "\n\n# Appended Policy\n" + additionalPolicy
	k.policyDirty = true
	logging.KernelDebug("AppendPolicy: policy grew from %d to %d bytes, policyDirty=true", prevLen, len(k.policy))
}

// LoadPolicyFile loads policy rules from a file and appends them.
func (k *RealKernel) LoadPolicyFile(path string) error {
	logging.KernelDebug("LoadPolicyFile: attempting to load %s", path)
	baseName := filepath.Base(path)

	// 1. Try Embedded Core first
	if data, err := coreLogic.ReadFile("defaults/" + baseName); err == nil {
		logging.Kernel("LoadPolicyFile: loaded from embedded core: %s (%d bytes)", baseName, len(data))
		k.AppendPolicy(string(data))
		return nil
	}

	// 2. Try User Workspace (.nerd/mangle)
	userPath := filepath.Join(k.nerdPath("mangle"), baseName)
	if data, err := os.ReadFile(userPath); err == nil {
		logging.Kernel("LoadPolicyFile: loaded from user workspace: %s (%d bytes)", userPath, len(data))
		k.AppendPolicy(string(data))
		return nil
	}

	// 3. Try explicitly provided path
	if data, err := os.ReadFile(path); err == nil {
		logging.Kernel("LoadPolicyFile: loaded from explicit path: %s (%d bytes)", path, len(data))
		k.AppendPolicy(string(data))
		return nil
	}

	// 4. Try legacy search paths (fallback for existing behavior)
	searchPaths := []string{
		filepath.Join("internal/mangle", baseName),
		filepath.Join("../internal/mangle", baseName),
		filepath.Join("../../internal/mangle", baseName),
	}

	for _, p := range searchPaths {
		data, err := os.ReadFile(p)
		if err == nil {
			logging.Kernel("LoadPolicyFile: loaded from legacy path: %s (%d bytes)", p, len(data))
			k.AppendPolicy(string(data))
			return nil
		}
	}

	logging.Get(logging.CategoryKernel).Error("LoadPolicyFile: policy file not found: %s", path)
	return fmt.Errorf("policy file not found: %s", path)
}

// HotLoadRule dynamically loads a single Mangle rule at runtime.
// This is used by Autopoiesis to add new rules without restarting.
// FIX for Bug #8 (Suicide Rule): Uses a "Sandbox Compiler" to validate the rule
// before accepting it, preventing invalid rules from bricking the kernel.
func (k *RealKernel) HotLoadRule(rule string) error {
	timer := logging.StartTimer(logging.CategoryKernel, "HotLoadRule")
	logging.Kernel("HotLoadRule: attempting to load rule (%d bytes)", len(rule))

	k.mu.Lock()
	defer k.mu.Unlock()

	if rule == "" {
		err := fmt.Errorf("empty rule")
		logging.Get(logging.CategoryKernel).Error("HotLoadRule: %v", err)
		return err
	}

	// Log the rule being loaded (truncated for readability)
	rulePreview := rule
	if len(rulePreview) > 100 {
		rulePreview = rulePreview[:100] + "..."
	}
	logging.KernelDebug("HotLoadRule: rule preview: %s", rulePreview)

	// 1. Create a Sandbox Kernel (Memory only)
	logging.KernelDebug("HotLoadRule: creating sandbox kernel for validation")
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
	logging.KernelDebug("HotLoadRule: validating rule in sandbox...")
	if err := sandbox.rebuildProgram(); err != nil {
		logging.Get(logging.CategoryKernel).Error("HotLoadRule: rule rejected by sandbox compiler: %v", err)
		return fmt.Errorf("rule rejected by sandbox compiler: %w", err)
	}
	logging.KernelDebug("HotLoadRule: sandbox validation passed")

	// 5. If successful, apply to Real Kernel (in-memory)
	k.learned = k.learned + "\n\n# HotLoaded Rule\n" + rule
	k.policyDirty = true

	timer.StopWithInfo()
	logging.Kernel("HotLoadRule: rule loaded successfully, policyDirty=true")
	return nil
}

// HotLoadLearnedRule dynamically loads a learned rule and persists it to learned.mg.
// This is the primary method for Autopoiesis to add new learned rules.
// It validates the rule, loads it into memory, and writes it to disk for persistence.
func (k *RealKernel) HotLoadLearnedRule(rule string) error {
	logging.Kernel("HotLoadLearnedRule: loading and persisting learned rule")

	// 1. Validate using sandbox (same as HotLoadRule)
	if err := k.HotLoadRule(rule); err != nil {
		logging.Get(logging.CategoryKernel).Error("HotLoadLearnedRule: validation failed: %v", err)
		return err
	}

	// 2. Persist to learned.mg file
	if err := k.appendToLearnedFile(rule); err != nil {
		logging.Get(logging.CategoryKernel).Error("HotLoadLearnedRule: failed to persist rule: %v", err)
		return err
	}

	logging.Kernel("HotLoadLearnedRule: rule loaded and persisted successfully")
	return nil
}

// appendToLearnedFile appends a rule to learned.mg on disk.
func (k *RealKernel) appendToLearnedFile(rule string) error {
	logging.KernelDebug("appendToLearnedFile: persisting rule to disk")

	// Determine workspace path for persistence
	// Priority: explicit manglePath > workspace-based .nerd/mangle > relative .nerd/mangle
	targetDir := k.nerdPath("mangle")
	if k.manglePath != "" {
		targetDir = k.manglePath
	}
	logging.KernelDebug("appendToLearnedFile: target directory: %s", targetDir)

	// Ensure directory exists
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		logging.Get(logging.CategoryKernel).Error("appendToLearnedFile: failed to create directory: %v", err)
		return fmt.Errorf("failed to create directory for learned rules: %w", err)
	}

	learnedPath := filepath.Join(targetDir, "learned.mg")

	// Append rule to file
	f, err := os.OpenFile(learnedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("appendToLearnedFile: failed to open learned.mg: %v", err)
		return fmt.Errorf("failed to open learned.mg: %w", err)
	}
	defer f.Close()

	// Write rule with proper formatting
	_, err = f.WriteString(fmt.Sprintf("\n# Autopoiesis-learned rule (added %s)\n%s\n",
		time.Now().Format("2006-01-02 15:04:05"), rule))
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("appendToLearnedFile: failed to write: %v", err)
		return fmt.Errorf("failed to write to learned.mg: %w", err)
	}

	logging.Kernel("appendToLearnedFile: rule persisted to %s", learnedPath)
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
// Optimized to avoid disk I/O and re-parsing by sharing the immutable programInfo.
func (k *RealKernel) Clone() *RealKernel {
	timer := logging.StartTimer(logging.CategoryKernel, "Clone")
	logging.KernelDebug("Clone: creating kernel clone")

	k.mu.RLock()
	defer k.mu.RUnlock()

	// Create bare struct without triggering loadMangleFiles
	clone := &RealKernel{
		facts:           make([]Fact, len(k.facts)),
		store:           factstore.NewSimpleInMemoryStore(),
		schemas:         k.schemas,
		policy:          k.policy,
		learned:         k.learned,
		manglePath:      k.manglePath,
		workspaceRoot:   k.workspaceRoot,   // Preserve workspace for .nerd paths
		programInfo:     k.programInfo,     // Share immutable analysis
		schemaValidator: k.schemaValidator, // Share immutable validator
		policyDirty:     k.policyDirty,     // Inherit dirty state (likely false)
		initialized:     false,             // Will initialize on Evaluate
	}

	// copy(clone.facts, k.facts) - simpler to just re-assert if we want independence
	// But for performance, deep copy the slice
	copy(clone.facts, k.facts)

	// Note: We do NOT define a shared ViewLayer here because Mangle needs
	// a unified store for fixpoint. Fast copying of the slice is reasonably cheap
	// (12GB RAM budget allows for this). The main win is skipping Parse/Analyze.

	timer.Stop()
	logging.KernelDebug("Clone: created clone with %d facts, shared programInfo", len(clone.facts))
	return clone
}

// Clear resets the kernel to empty state (keeps cached programInfo).
func (k *RealKernel) Clear() {
	logging.Kernel("Clear: resetting kernel to empty state (preserving programInfo)")
	k.mu.Lock()
	defer k.mu.Unlock()
	prevFactCount := len(k.facts)
	k.facts = make([]Fact, 0)
	k.store = factstore.NewSimpleInMemoryStore()
	k.initialized = false
	// Note: programInfo and policyDirty preserved - only facts cleared
	logging.KernelDebug("Clear: cleared %d facts, programInfo preserved", prevFactCount)
}

// Reset fully resets the kernel including cached program.
func (k *RealKernel) Reset() {
	logging.Kernel("Reset: fully resetting kernel (including programInfo)")
	k.mu.Lock()
	defer k.mu.Unlock()
	prevFactCount := len(k.facts)
	k.facts = make([]Fact, 0)
	k.store = factstore.NewSimpleInMemoryStore()
	k.programInfo = nil
	k.policyDirty = true
	k.initialized = false
	logging.KernelDebug("Reset: cleared %d facts, programInfo cleared, policyDirty=true", prevFactCount)
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
	timer := logging.StartTimer(logging.CategoryKernel, "LoadFactsFromFile")
	logging.Kernel("LoadFactsFromFile: loading facts from %s", path)

	data, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("LoadFactsFromFile: failed to read file: %v", err)
		return fmt.Errorf("failed to read facts file: %w", err)
	}
	logging.KernelDebug("LoadFactsFromFile: read %d bytes from %s", len(data), path)

	// Parse the facts from the file content
	facts, err := ParseFactsFromString(string(data))
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("LoadFactsFromFile: failed to parse facts: %v", err)
		return fmt.Errorf("failed to parse facts: %w", err)
	}
	logging.KernelDebug("LoadFactsFromFile: parsed %d facts from file", len(facts))

	if err := k.LoadFacts(facts); err != nil {
		return err
	}

	timer.StopWithInfo()
	logging.Kernel("LoadFactsFromFile: loaded %d facts from %s", len(facts), path)
	return nil
}

// UpdateSystemFacts updates transient system facts like time.
// This should be called ONCE per turn/request to avoid infinite loops
// in logic that depends on changing time (Fix for Bug #7).
func (k *RealKernel) UpdateSystemFacts() error {
	timer := logging.StartTimer(logging.CategoryKernel, "UpdateSystemFacts")
	logging.KernelDebug("UpdateSystemFacts: updating transient system facts")

	k.mu.Lock()
	defer k.mu.Unlock()

	// 1. Retract old system facts
	prevCount := len(k.facts)
	newFacts := make([]Fact, 0, len(k.facts)+1)
	retractedCount := 0
	for _, f := range k.facts {
		if f.Predicate != "current_time" {
			newFacts = append(newFacts, f)
		} else {
			retractedCount++
		}
	}
	k.facts = newFacts
	logging.KernelDebug("UpdateSystemFacts: retracted %d old system facts", retractedCount)

	// 2. Add fresh system facts
	now := time.Now().Unix()
	k.facts = append(k.facts, Fact{
		Predicate: "current_time",
		Args:      []interface{}{now},
	})
	logging.KernelDebug("UpdateSystemFacts: added current_time=%d", now)

	// 3. Re-evaluate
	// We use evaluate() directly since we already hold the lock
	if err := k.evaluate(); err != nil {
		logging.Get(logging.CategoryKernel).Error("UpdateSystemFacts: evaluation failed: %v", err)
		return err
	}

	timer.Stop()
	logging.KernelDebug("UpdateSystemFacts: complete, EDB: %d -> %d facts", prevCount, len(k.facts))
	return nil
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

// =============================================================================
// AUTOPOIESIS BRIDGE (Formerly Kernel Adapter)
// =============================================================================

// AutopoiesisBridge wraps RealKernel to implement autopoiesis.KernelInterface.
type AutopoiesisBridge struct {
	kernel *RealKernel
}

// NewAutopoiesisBridge creates an adapter that implements autopoiesis.KernelInterface.
func NewAutopoiesisBridge(kernel *RealKernel) *AutopoiesisBridge {
	return &AutopoiesisBridge{kernel: kernel}
}

// AssertFact implements autopoiesis.KernelInterface.
func (ab *AutopoiesisBridge) AssertFact(fact autopoiesis.KernelFact) error {
	coreFact := Fact{
		Predicate: fact.Predicate,
		Args:      fact.Args,
	}
	return ab.kernel.Assert(coreFact)
}

// QueryPredicate implements autopoiesis.KernelInterface.
func (ab *AutopoiesisBridge) QueryPredicate(predicate string) ([]autopoiesis.KernelFact, error) {
	facts, err := ab.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}

	result := make([]autopoiesis.KernelFact, len(facts))
	for i, f := range facts {
		result[i] = autopoiesis.KernelFact{
			Predicate: f.Predicate,
			Args:      f.Args,
		}
	}
	return result, nil
}

// QueryBool implements autopoiesis.KernelInterface.
func (ab *AutopoiesisBridge) QueryBool(predicate string) bool {
	facts, err := ab.kernel.Query(predicate)
	if err != nil {
		return false
	}
	return len(facts) > 0
}

// RetractFact implements autopoiesis.KernelInterface.
func (ab *AutopoiesisBridge) RetractFact(fact autopoiesis.KernelFact) error {
	coreFact := Fact{
		Predicate: fact.Predicate,
		Args:      fact.Args,
	}
	return ab.kernel.RetractFact(coreFact)
}

// Ensure AutopoiesisBridge implements KernelInterface at compile time.
var _ autopoiesis.KernelInterface = (*AutopoiesisBridge)(nil)
