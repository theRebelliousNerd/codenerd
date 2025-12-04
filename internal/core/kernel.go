package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/mangle/analysis"
	"github.com/google/mangle/ast"
	"github.com/google/mangle/engine"
	"github.com/google/mangle/factstore"
	"github.com/google/mangle/parse"
)

// Fact represents a single logical fact (atom) in the EDB.
type Fact struct {
	Predicate string
	Args      []interface{}
}

// String returns the Datalog string representation of the fact.
func (f Fact) String() string {
	var args []string
	for _, arg := range f.Args {
		switch v := arg.(type) {
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
}

// RealKernel wraps the google/mangle engine with proper EDB/IDB separation.
type RealKernel struct {
	mu          sync.RWMutex
	facts       []Fact
	store       factstore.FactStore
	programInfo *analysis.ProgramInfo
	schemas     string
	policy      string
	initialized bool
	manglePath  string // Path to mangle files directory
}

// NewRealKernel creates a new kernel instance.
func NewRealKernel() *RealKernel {
	k := &RealKernel{
		facts: make([]Fact, 0),
		store: factstore.NewSimpleInMemoryStore(),
	}

	// Find and load mangle files from the project
	k.loadMangleFiles()

	return k
}

// NewRealKernelWithPath creates a kernel with explicit mangle path.
func NewRealKernelWithPath(manglePath string) *RealKernel {
	k := &RealKernel{
		facts:      make([]Fact, 0),
		store:      factstore.NewSimpleInMemoryStore(),
		manglePath: manglePath,
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

		schemasPath := filepath.Join(basePath, "schemas.gl")
		policyPath := filepath.Join(basePath, "policy.gl")

		if data, err := os.ReadFile(schemasPath); err == nil {
			k.schemas = string(data)
		}
		if data, err := os.ReadFile(policyPath); err == nil {
			k.policy = string(data)
		}

		if k.schemas != "" && k.policy != "" {
			break
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

// Assert adds a single fact dynamically.
func (k *RealKernel) Assert(fact Fact) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.facts = append(k.facts, fact)

	// Also add directly to the store for immediate availability
	atom, err := fact.ToAtom()
	if err != nil {
		return err
	}
	k.store.Add(atom)
	return nil
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

// rebuild reconstructs the program and evaluates to fixpoint.
func (k *RealKernel) rebuild() error {
	// 1. Construct the complete program
	var sb strings.Builder

	// Schemas first (declarations)
	if k.schemas != "" {
		sb.WriteString(k.schemas)
		sb.WriteString("\n")
	}

	// Facts (EDB)
	for _, f := range k.facts {
		sb.WriteString(f.String())
		sb.WriteString("\n")
	}

	// Policy rules (IDB)
	if k.policy != "" {
		sb.WriteString(k.policy)
	}

	programStr := sb.String()

	// 2. Parse
	parsed, err := parse.Unit(strings.NewReader(programStr))
	if err != nil {
		return fmt.Errorf("failed to parse program: %w", err)
	}

	// 3. Analyze
	programInfo, err := analysis.AnalyzeOneUnit(parsed, nil)
	if err != nil {
		return fmt.Errorf("failed to analyze program: %w", err)
	}
	k.programInfo = programInfo

	// 4. Create fresh FactStore
	k.store = factstore.NewSimpleInMemoryStore()

	// 5. Evaluate to fixpoint using stratified evaluation
	_, err = engine.EvalStratifiedProgramWithStats(programInfo, nil, nil, k.store)
	if err != nil {
		return fmt.Errorf("failed to evaluate program: %w", err)
	}

	k.initialized = true
	return nil
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
}

// SetPolicy allows loading custom policy rules (for shard specialization).
func (k *RealKernel) SetPolicy(policy string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.policy = policy
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

// Clear resets the kernel to empty state.
func (k *RealKernel) Clear() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.facts = make([]Fact, 0)
	k.store = factstore.NewSimpleInMemoryStore()
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
