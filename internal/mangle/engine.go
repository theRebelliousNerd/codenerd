// Package mangle provides a production-grade Google Mangle engine wrapper.
// Adapted from code-graph-mcp-server for the Cortex 1.5.0 Neuro-Symbolic Architecture.
package mangle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"

	"github.com/google/mangle/analysis"
	"github.com/google/mangle/ast"
	_ "github.com/google/mangle/builtin"
	mengine "github.com/google/mangle/engine"
	"github.com/google/mangle/factstore"
	_ "github.com/google/mangle/packages"
	"github.com/google/mangle/parse"
	"github.com/google/mangle/unionfind"
)

// Config holds Mangle engine configuration.
type Config struct {
	FactLimit         int    `json:"fact_limit"`
	DerivedFactsLimit int    `json:"derived_facts_limit"` // Gas limit for inference (0 = unlimited)
	QueryTimeout      int    `json:"query_timeout"`       // seconds
	AutoEval          bool   `json:"auto_eval"`
	SchemaPath        string `json:"schema_path"`
	PolicyPath        string `json:"policy_path"`
}

// DefaultConfig returns production defaults.
func DefaultConfig() Config {
	return Config{
		FactLimit:         100000,
		DerivedFactsLimit: 100000, // Gas limit for inference
		QueryTimeout:      30,

		AutoEval: os.Getenv("MANGLE_AUTO_EVAL") != "0", // Default to true, disable with "0"
	}
}

// ErrDerivedFactsLimitExceeded is returned when inference exceeds the gas limit.
var ErrDerivedFactsLimitExceeded = fmt.Errorf("derived facts limit exceeded (inference gas limit)")

// errNoSchemas is the sentinel error for operations attempted before schema loading.
var errNoSchemas = fmt.Errorf("no schemas loaded; call LoadSchema first")

// Engine wraps the production-grade Google Mangle engine.
// Implements the Hollow Kernel pattern from Cortex 1.5.0 Section 2.1.
type Engine struct {
	config Config

	mu              sync.RWMutex
	store           factstore.ConcurrentFactStore
	baseStore       factstore.FactStoreWithRemove
	programInfo     *analysis.ProgramInfo
	strata          []analysis.Nodeset       // Cached stratification from last rebuildProgramLocked
	predToStratum   map[ast.PredicateSym]int // Cached predicate-to-stratum mapping
	queryContext    *mengine.QueryContext
	predicateIndex  map[string]ast.PredicateSym
	schemaFragments []parse.SourceUnit
	factCount       int
	derivedCount    int // Tracks derived facts for gas limit enforcement
	factLimitWarned bool
	autoEval        bool
	persistence     Persistence
	fileFacts       map[string][]ast.Atom
}

// Fact represents a single fact in the knowledge graph.
type Fact struct {
	Predicate string        `json:"predicate"`
	Args      []interface{} `json:"args"`
	Line      int           `json:"line,omitempty"`
	Timestamp time.Time     `json:"timestamp,omitempty"`
}

// String returns the Datalog representation of the fact.
func (f Fact) String() string {
	var args []string
	for _, arg := range f.Args {
		switch v := arg.(type) {
		case string:
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

// QueryResult represents the result of a Mangle query.
type QueryResult struct {
	Bindings []map[string]interface{} `json:"bindings"`
	Duration time.Duration            `json:"duration"`
}

// Stats contains engine statistics.
type Stats struct {
	TotalFacts      int            `json:"total_facts"`
	PredicateCounts map[string]int `json:"predicate_counts"`
	LastUpdate      time.Time      `json:"last_update"`
}

// Persistence describes the minimal durability operations the engine relies on.
type Persistence interface {
	ReplaceFactsForFile(ctx context.Context, file string, facts []Fact, contentHash string) error
	LoadFacts(ctx context.Context) ([]Fact, error)
	GetFileStates(ctx context.Context) (map[string]string, error)
}

// NewEngine creates a new Mangle engine instance.
func NewEngine(cfg Config, persistence Persistence) (*Engine, error) {
	baseStore := factstore.NewSimpleInMemoryStore()
	return &Engine{
		config:         cfg,
		baseStore:      baseStore,
		store:          factstore.NewConcurrentFactStore(baseStore),
		predicateIndex: make(map[string]ast.PredicateSym),
		autoEval:       cfg.AutoEval,
		persistence:    persistence,
		fileFacts:      make(map[string][]ast.Atom),
	}, nil
}

// GetPersistence returns the configured persistence layer.
func (e *Engine) GetPersistence() Persistence {
	return e.persistence
}

// ToggleAutoEval enables or disables automatic rule evaluation after fact insertion.
// When disabled, facts are inserted but rules are not re-evaluated until RecomputeRules is called.
func (e *Engine) ToggleAutoEval(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.autoEval = enabled
}

// RecomputeRules forces a re-evaluation of all rules against the current fact store.
// Useful when auto-eval is disabled for bulk insertion.
func (e *Engine) RecomputeRules() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.programInfo == nil {
		return errNoSchemas
	}

	logging.Kernel("Starting Mangle rule recomputation...")
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		start := time.Now()
		for {
			select {
			case <-ticker.C:
				logging.KernelDebug("...still recomputing rules (%v elapsed)...", time.Since(start).Round(time.Second))
			case <-done:
				return
			}
		}
	}()

	// Use EvalProgramWithStats for visibility with gas limit enforcement
	stats, err := e.evalWithGasLimit()
	close(done)

	if err != nil {
		logging.Get(logging.CategoryKernel).Error("Rule recomputation failed: %v", err)
		return err
	}

	logging.Kernel("Recomputation complete. Stats: %+v", stats)
	return nil
}

// evalWithGasLimit wraps EvalStratifiedProgramWithStats with derived facts gas limit enforcement.
// This prevents runaway inference from exhausting memory.
// Uses pre-computed strata from rebuildProgramLocked() for proper stratified evaluation.
func (e *Engine) evalWithGasLimit() (mengine.Stats, error) {
	if e.strata == nil || e.predToStratum == nil {
		return mengine.Stats{}, fmt.Errorf("stratification not computed; call LoadSchema first")
	}

	// Count facts before evaluation
	beforeCount := e.store.EstimateFactCount()

	// Build eval options
	var opts []mengine.EvalOption
	if e.config.DerivedFactsLimit > 0 {
		opts = append(opts, mengine.WithCreatedFactLimit(e.config.DerivedFactsLimit))
	}

	// Run stratified evaluation (replaces deprecated EvalProgramWithStats)
	stats, err := mengine.EvalStratifiedProgramWithStats(e.programInfo, e.strata, e.predToStratum, e.store, opts...)
	if err != nil {
		return mengine.Stats{}, err
	}

	// Telemetry: track derived facts for monitoring
	afterCount := e.store.EstimateFactCount()
	derivedThisRound := afterCount - beforeCount
	e.derivedCount += derivedThisRound

	if derivedThisRound > 0 {
		logging.KernelDebug("Evaluation derived %d new facts (total derived: %d, limit: %d)",
			derivedThisRound, e.derivedCount, e.config.DerivedFactsLimit)
		logging.Get(logging.CategoryKernel).Info("Mangle Inference: +%d facts, total %d", derivedThisRound, e.derivedCount)
	}

	return stats, nil
}

// GetDerivedFactCount returns the current count of derived facts (for monitoring).
func (e *Engine) GetDerivedFactCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.derivedCount
}

// ResetDerivedFactCount resets the derived fact counter (e.g., at session start).
func (e *Engine) ResetDerivedFactCount() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.derivedCount = 0
	logging.KernelDebug("Derived fact counter reset to 0")
}

// LoadSchema loads and compiles a Mangle schema file (.mg).
func (e *Engine) LoadSchema(path string) error {
	logging.KernelDebug("Loading schema file: %s", path)
	data, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("Failed to read schema file %s: %v", path, err)
		return fmt.Errorf("failed to read schema file %s: %w", path, err)
	}

	return e.LoadSchemaString(string(data))
}

// LoadSchemaString loads and compiles a Mangle schema from string.
func (e *Engine) LoadSchemaString(schema string) error {
	unit, err := parse.Unit(bytes.NewReader([]byte(schema)))
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("Failed to parse schema: %v", err)
		return fmt.Errorf("failed to parse schema: %w", err)
	}
	logging.KernelDebug("Parsed schema: %d clauses, %d declarations", len(unit.Clauses), len(unit.Decls))

	e.mu.Lock()
	defer e.mu.Unlock()

	e.schemaFragments = append(e.schemaFragments, unit)
	if err := e.rebuildProgramLocked(); err != nil {
		return fmt.Errorf("failed to analyze schema: %w", err)
	}

	return nil
}

// rebuildProgramLocked analyzes all loaded schema fragments and refreshes predicate indexes.
func (e *Engine) rebuildProgramLocked() error {
	if len(e.schemaFragments) == 0 {
		return errNoSchemas
	}

	var clauses []ast.Clause
	var decls []ast.Decl
	for _, fragment := range e.schemaFragments {
		clauses = append(clauses, fragment.Clauses...)
		decls = append(decls, fragment.Decls...)
	}

	unit := parse.SourceUnit{
		Clauses: clauses,
		Decls:   decls,
	}

	programInfo, err := analysis.AnalyzeOneUnit(unit, nil)
	if err != nil {
		return err
	}

	e.programInfo = programInfo

	// Cache stratification for EvalStratifiedProgramWithStats
	strata, predToStratum, err := analysis.Stratify(analysis.Program{
		EdbPredicates: programInfo.EdbPredicates,
		IdbPredicates: programInfo.IdbPredicates,
		Rules:         programInfo.Rules,
	})
	if err != nil {
		return fmt.Errorf("stratification failed: %w", err)
	}
	e.strata = strata
	e.predToStratum = predToStratum

	e.predicateIndex = make(map[string]ast.PredicateSym, len(programInfo.Decls))

	predToDecl := make(map[ast.PredicateSym]*ast.Decl, len(programInfo.Decls))
	for sym, decl := range programInfo.Decls {
		e.predicateIndex[sym.Symbol] = sym
		predToDecl[sym] = decl
	}

	predToRules := make(map[ast.PredicateSym][]ast.Clause)
	for _, clause := range programInfo.Rules {
		predToRules[clause.Head.Predicate] = append(predToRules[clause.Head.Predicate], clause)
	}

	ctx := &mengine.QueryContext{
		PredToRules: predToRules,
		PredToDecl:  predToDecl,
		Store:       e.store,
	}

	e.queryContext = ctx
	return nil
}

// WarmFromPersistence hydrates the in-memory fact store from the persistence layer.
func (e *Engine) WarmFromPersistence(ctx context.Context) error {
	if e.persistence == nil || isNilPersistence(e.persistence) {
		return nil
	}

	facts, err := e.persistence.LoadFacts(ctx)
	if err != nil {
		return fmt.Errorf("load persisted facts: %w", err)
	}
	if len(facts) == 0 {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.programInfo == nil {
		return errNoSchemas
	}

	wasAuto := e.autoEval
	e.autoEval = false
	for _, fact := range facts {
		if err := e.insertFactLocked(fact); err != nil {
			return fmt.Errorf("hydrate fact %s: %w", fact.Predicate, err)
		}
	}
	e.autoEval = wasAuto

	if e.autoEval {
		_, err := e.evalWithGasLimit()
		if err != nil {
			return fmt.Errorf("recompute rules after warm start: %w", err)
		}
	}

	return nil
}

// AddFact inserts a single fact into the knowledge graph.
func (e *Engine) AddFact(predicate string, args ...interface{}) error {
	return e.AddFacts([]Fact{{Predicate: predicate, Args: args}})
}

// AddFacts inserts multiple facts (batched).
func (e *Engine) AddFacts(facts []Fact) error {
	if len(facts) == 0 {
		return nil
	}

	logging.KernelDebug("Adding %d facts", len(facts))

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.programInfo == nil {
		return errNoSchemas
	}

	for _, fact := range facts {
		if err := e.insertFactLocked(fact); err != nil {
			logging.Get(logging.CategoryKernel).Error("Failed to insert fact %s: %v", fact.Predicate, err)
			return err
		}
	}

	if e.autoEval {
		_, err := e.evalWithGasLimit()
		if err != nil {
			logging.Get(logging.CategoryKernel).Error("Rule evaluation failed after fact insertion: %v", err)
		}
		return err
	}
	return nil
}

// AddFactsContext is context-aware version of AddFacts for EngineSink interface.
func (e *Engine) AddFactsContext(ctx context.Context, facts []Fact) error {
	return e.AddFacts(facts)
}

// ReplaceFactsForFile removes previously stored facts for a file before inserting new ones.
func (e *Engine) ReplaceFactsForFile(file string, facts []Fact) error {
	return e.replaceFactsForFileImpl(file, facts, "")
}

// ReplaceFactsForFileWithHash is like ReplaceFactsForFile but allows passing a content hash.
func (e *Engine) ReplaceFactsForFileWithHash(file string, facts []Fact, contentHash string) error {
	return e.replaceFactsForFileImpl(file, facts, contentHash)
}

// replaceFactsForFileImpl is the shared implementation for ReplaceFactsForFile variants.
func (e *Engine) replaceFactsForFileImpl(file string, facts []Fact, contentHash string) error {
	target := canonicalPath(file)

	e.mu.Lock()
	if e.programInfo == nil {
		e.mu.Unlock()
		return errNoSchemas
	}

	removed := e.removeFactsLocked(target)
	for _, fact := range facts {
		if err := e.insertFactLocked(fact); err != nil {
			e.mu.Unlock()
			return err
		}
	}

	if removed > 0 && (e.config.FactLimit == 0 || float64(e.factCount) < float64(e.config.FactLimit)*0.7) {
		e.factLimitWarned = false
	}

	if e.autoEval {
		_, err := e.evalWithGasLimit()
		if err != nil {
			e.mu.Unlock()
			return err
		}
	}

	shouldPersist := e.persistence != nil && !isNilPersistence(e.persistence)
	e.mu.Unlock()

	if shouldPersist {
		if err := e.persistence.ReplaceFactsForFile(context.Background(), target, facts, contentHash); err != nil {
			return fmt.Errorf("persist facts for %s: %w", target, err)
		}
	}
	return nil
}

// isNilPersistence guards against typed nil persistence implementations.
func isNilPersistence(p Persistence) bool {
	if p == nil {
		return true
	}
	val := reflect.ValueOf(p)
	return val.Kind() == reflect.Ptr && val.IsNil()
}

func (e *Engine) insertFactLocked(fact Fact) error {
	if e.config.FactLimit > 0 && e.factCount >= e.config.FactLimit {
		return fmt.Errorf("fact limit exceeded: %d", e.config.FactLimit)
	}

	atom, err := e.factToAtomLocked(fact)
	if err != nil {
		return err
	}

	if e.store.Add(atom) {
		e.factCount++
		e.maybeWarnFactLimit()

		// Update reverse index if this fact applies to a file
		if len(atom.Args) > 0 {
			if str, ok := convertBaseTermToInterface(atom.Args[0]).(string); ok {
				target := canonicalPath(str)
				if target != "" {
					e.fileFacts[target] = append(e.fileFacts[target], atom)
				}
			}
		}
	}
	return nil
}

func (e *Engine) maybeWarnFactLimit() {
	if e.config.FactLimit == 0 || e.factLimitWarned {
		return
	}

	if e.config.FactLimit > 0 {
		utilization := float64(e.factCount) / float64(e.config.FactLimit)
		if utilization >= 0.85 {
			logging.Get(logging.CategoryKernel).Warn("Fact store is %.1f%% of configured capacity (%d / %d)", utilization*100, e.factCount, e.config.FactLimit)
			e.factLimitWarned = true
		}
	}
}

func (e *Engine) factToAtomLocked(fact Fact) (ast.Atom, error) {
	sym, ok := e.predicateIndex[fact.Predicate]
	if !ok {
		return ast.Atom{}, fmt.Errorf("predicate %s is not declared in schemas", fact.Predicate)
	}

	if len(fact.Args) != sym.Arity {
		return ast.Atom{}, fmt.Errorf("predicate %s expects %d args, got %d", fact.Predicate, sym.Arity, len(fact.Args))
	}

	// Fetch the declaration to get expected types
	var decl *ast.Decl
	if e.queryContext != nil {
		decl = e.queryContext.PredToDecl[sym]
	}

	args := make([]ast.BaseTerm, len(fact.Args))
	for i, raw := range fact.Args {
		var expectedType ast.ConstantType = -1 // -1 means unknown/any
		if decl != nil && len(decl.Bounds) > 0 {
			// Iterate over bounds to find a matching type constraint
			// For simplicity, we check the first bound declaration
			bounds := decl.Bounds[0].Bounds
			if len(bounds) > i {
				if c, ok := bounds[i].(ast.Constant); ok {
					switch c.Symbol {
					case "/name":
						expectedType = ast.NameType
					case "/string":
						expectedType = ast.StringType
					case "/number":
						expectedType = ast.NumberType
					case "/float64":
						expectedType = ast.Float64Type
					case "/time":
						expectedType = ast.TimeType
					case "/duration":
						expectedType = ast.DurationType
					case "/bytes":
						expectedType = ast.BytesType
					}
				}
			}
		}

		term, err := convertValueToTypedTerm(raw, expectedType)
		if err != nil {
			return ast.Atom{}, fmt.Errorf("predicate %s arg %d: %w", fact.Predicate, i, err)
		}
		args[i] = term
	}

	return ast.Atom{Predicate: sym, Args: args}, nil
}

// convertValueToTypedTerm converts a value to a Mangle BaseTerm, enforcing expected type if known.
func convertValueToTypedTerm(value interface{}, expectedType ast.ConstantType) (ast.BaseTerm, error) {
	// 1. If we have a strict type expectation, try to coerce or validate
	switch expectedType {
	case ast.NameType:
		if s, ok := value.(string); ok {
			// Force conversion to Name constant (Atom)
			if !strings.HasPrefix(s, "/") {
				return ast.Name("/" + s)
			}
			return ast.Name(s)
		}
		// If it's already a NameType constant, let it fall through
	case ast.StringType:
		if s, ok := value.(string); ok {
			// Force conversion to String constant, IGNORING identifier heuristics
			return ast.String(s), nil
		}
	}

	// 2. Fall back to type matching
	// Note: Complex nested structs or map recursion currently handled by default JSON marshalling
	switch v := value.(type) {
	case ast.BaseTerm:
		return v, nil
	case string:
		if strings.HasPrefix(v, "/") {
			// Explicit Name syntax in string ALWAYS wins
			name, err := ast.Name(v)
			if err != nil {
				return nil, err
			}
			return name, nil
		}

		// Heuristics (only used if type is NOT strictly StringType)
		if expectedType != ast.StringType {
			// Auto-Atomizer: Promote identifier-like strings to Atoms if we expect Name or it's unknown
			if isIdentifier(v) {
				name, err := ast.Name("/" + v)
				if err == nil {
					return name, nil
				}
			}
		}
		return ast.String(v), nil
	case fmt.Stringer:
		return ast.String(v.String()), nil
	case int:
		return ast.Number(int64(v)), nil
	case int32:
		return ast.Number(int64(v)), nil
	case int64:
		return ast.Number(v), nil
	case float32:
		return ast.Float64(float64(v)), nil
	case float64:
		return ast.Float64(v), nil
	case bool:
		if v {
			return ast.TrueConstant, nil
		}
		return ast.FalseConstant, nil
	case []string:
		constants := make([]ast.Constant, len(v))
		for i, item := range v {
			constants[i] = ast.String(item)
		}
		return ast.List(constants), nil
	case []interface{}:
		constants := make([]ast.Constant, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				constants = append(constants, ast.String(s))
			}
		}
		return ast.List(constants), nil
	case map[string]string:
		encoded, _ := json.Marshal(v)
		return ast.String(string(encoded)), nil
	case map[string]interface{}:
		encoded, _ := json.Marshal(v)
		return ast.String(string(encoded)), nil
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("unsupported fact argument type %T", v)
		}
		return ast.String(string(encoded)), nil
	}
}

// Query evaluates a query expressed in Mangle notation.
func (e *Engine) Query(ctx context.Context, query string) (*QueryResult, error) {
	logging.KernelDebug("Query: %s", query)
	shape, err := parseQueryShape(query)
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("Query parse failed: %s - %v", query, err)
		return nil, err
	}

	e.mu.RLock()
	queryContext := e.queryContext
	if queryContext == nil {
		e.mu.RUnlock()
		return nil, errNoSchemas
	}

	decl, ok := queryContext.PredToDecl[shape.atom.Predicate]
	if !ok {
		e.mu.RUnlock()
		return nil, fmt.Errorf("predicate %s is not declared", shape.atom.Predicate.Symbol)
	}
	if len(decl.Modes()) == 0 {
		e.mu.RUnlock()
		return nil, fmt.Errorf("predicate %s has no modes declared", shape.atom.Predicate.Symbol)
	}
	mode := decl.Modes()[0]
	e.mu.RUnlock()

	// Use a reasonable default timeout if not provided
	var timeoutDuration time.Duration
	if e.config.QueryTimeout > 0 {
		timeoutDuration = time.Duration(e.config.QueryTimeout) * time.Second
	} else {
		timeoutDuration = 5 * time.Second
	}

	// If context doesn't have a deadline, apply our default
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeoutDuration)
		defer cancel()
	}

	start := time.Now()
	resultChan := make(chan []map[string]interface{}, 1)
	errChan := make(chan error, 1)

	go func() {
		var results []map[string]interface{}
		err := queryContext.EvalQuery(shape.atom, mode, unionfind.New(), func(fact ast.Atom) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			row := make(map[string]interface{}, len(shape.variables))
			for _, binding := range shape.variables {
				if binding.Index >= len(fact.Args) {
					continue
				}
				row[binding.Name] = convertBaseTermToInterface(fact.Args[binding.Index])
			}
			results = append(results, row)
			return nil
		})
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- results
	}()

	select {
	case results := <-resultChan:
		logging.KernelDebug("Query completed: %s -> %d results in %s", query, len(results), time.Since(start))
		return &QueryResult{
			Bindings: results,
			Duration: time.Since(start),
		}, nil
	case err := <-errChan:
		logging.Get(logging.CategoryKernel).Error("Query execution failed: %s - %v", query, err)
		return nil, err
	case <-ctx.Done():
		logging.Get(logging.CategoryKernel).Warn("Query timeout: %s after %v", query, time.Since(start))
		return nil, fmt.Errorf("query execution timed out after %v: %w", time.Since(start), ctx.Err())
	}
}

// GetFacts retrieves all facts for a given predicate.
func (e *Engine) GetFacts(predicate string) ([]Fact, error) {
	e.mu.RLock()
	sym, ok := e.predicateIndex[predicate]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("predicate %s is not declared", predicate)
	}

	var results []Fact
	err := e.store.GetFacts(ast.NewQuery(sym), func(atom ast.Atom) error {
		args := make([]interface{}, len(atom.Args))
		for i, arg := range atom.Args {
			args[i] = convertBaseTermToInterface(arg)
		}
		results = append(results, Fact{
			Predicate: predicate,
			Args:      args,
			Line:      0,
		})
		return nil
	})

	return results, err
}

// GetStats returns overall statistics for the fact store.
func (e *Engine) GetStats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	counts := make(map[string]int)
	for _, sym := range e.store.ListPredicates() {
		pred := sym.Symbol
		localCount := 0
		_ = e.store.GetFacts(ast.NewQuery(sym), func(ast.Atom) error {
			localCount++
			return nil
		})
		counts[pred] = localCount
	}

	return Stats{
		TotalFacts:      e.store.EstimateFactCount(),
		PredicateCounts: counts,
		LastUpdate:      time.Now(),
	}
}

// Clear removes all facts from the store.
func (e *Engine) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.baseStore = factstore.NewSimpleInMemoryStore()
	e.store = factstore.NewConcurrentFactStore(e.baseStore)
	e.factCount = 0
	e.fileFacts = make(map[string][]ast.Atom)
}

// Reset clears all facts AND schema definitions, restoring the engine to a blank slate.
func (e *Engine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.baseStore = factstore.NewSimpleInMemoryStore()
	e.store = factstore.NewConcurrentFactStore(e.baseStore)
	e.factCount = 0
	e.fileFacts = make(map[string][]ast.Atom)
	e.programInfo = nil
	e.strata = nil
	e.predToStratum = nil
	e.queryContext = nil
	e.predicateIndex = make(map[string]ast.PredicateSym)
	e.schemaFragments = nil
	e.derivedCount = 0
}

// Close cleans up engine resources.
func (e *Engine) Close() error {
	return nil
}

type queryVariable struct {
	Name  string
	Index int
}

type queryShape struct {
	atom      ast.Atom
	variables []queryVariable
}

func parseQueryShape(query string) (*queryShape, error) {
	clean := strings.TrimSpace(query)
	if clean == "" {
		return nil, fmt.Errorf("empty query")
	}

	if strings.HasPrefix(clean, "?") {
		clean = strings.TrimSpace(clean[1:])
	}
	if strings.HasSuffix(clean, ".") {
		clean = strings.TrimSpace(clean[:len(clean)-1])
	}

	atom, err := parse.Atom(clean)
	if err != nil {
		// Attempt again with a trailing period
		atom, err = parse.Atom(clean + ".")
		if err != nil {
			return nil, fmt.Errorf("failed to parse query %q: %w", query, err)
		}
	}

	variables := make([]queryVariable, 0, len(atom.Args))
	for idx, arg := range atom.Args {
		if variable, ok := arg.(ast.Variable); ok {
			variables = append(variables, queryVariable{
				Name:  variable.Symbol,
				Index: idx,
			})
		}
	}

	return &queryShape{
		atom:      atom,
		variables: variables,
	}, nil
}

// isIdentifier checks if a string is a valid Mangle identifier.
func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	// Simple check: starts with lowercase, alphanumeric + underscore
	// Mangle identifier: [a-z][a-zA-Z0-9_]*
	c := s[0]
	if !((c >= 'a' && c <= 'z') || c == '_') {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func convertBaseTermToInterface(term ast.BaseTerm) interface{} {
	switch v := term.(type) {
	case ast.Constant:
		return constantToInterface(v)
	case ast.Variable:
		return v.Symbol
	case ast.ApplyFn:
		return v.String()
	default:
		return fmt.Sprintf("%v", term)
	}
}

func constantToInterface(constant ast.Constant) interface{} {
	switch constant.Type {
	case ast.StringType:
		return constant.Symbol
	case ast.NameType:
		return constant.Symbol
	case ast.BytesType:
		return constant.Symbol
	case ast.NumberType:
		return constant.NumValue
	case ast.Float64Type:
		return math.Float64frombits(uint64(constant.NumValue))
	case ast.TimeType:
		return time.Unix(0, constant.NumValue).UTC()
	case ast.DurationType:
		return time.Duration(constant.NumValue)
	default:
		// DEFENSIVE: Fallback to Symbol instead of String() to avoid quotes/formatting
		// Log warning for unknown types to catch new AST types early
		logging.Get(logging.CategoryKernel).Warn("constantToInterface: unknown constant type %v, using Symbol fallback", constant.Type)
		return constant.Symbol
	}
}

func (e *Engine) removeFactsLocked(file string) int {
	if file == "" {
		return 0
	}

	target := canonicalPath(file)
	removed := 0

	// Fast path: use reverse index
	if atoms, ok := e.fileFacts[target]; ok {
		for _, atom := range atoms {
			if e.baseStore.Remove(atom) {
				if e.factCount > 0 {
					e.factCount--
				}
				removed++
			}
		}
		delete(e.fileFacts, target)
	}

	// Optimization: Fallback path removed.
	// The fileFacts index is guaranteed to be consistent for all explicitly added facts
	// via insertFactLocked. Derived facts are not tracked in fileFacts and are not
	// removed by ReplaceFactsForFile, which is the intended behavior (only source facts replaced).
	// This avoids an O(N) scan of the entire fact store.

	return removed
}

func canonicalPath(path string) string {
	if path == "" {
		return ""
	}
	clean := filepath.Clean(path)
	return strings.ReplaceAll(clean, "\\", "/")
}

// PushFact is a convenience method for adding a single fact by predicate and args.
// This method name matches the interface expected by browser automation.
func (e *Engine) PushFact(predicate string, args ...interface{}) error {
	return e.AddFact(predicate, args...)
}

// QueryFacts returns facts matching a predicate pattern (for compatibility with browser).
func (e *Engine) QueryFacts(predicate string, args ...string) []Fact {
	facts, _ := e.GetFacts(predicate)

	// Filter by args if provided
	if len(args) == 0 {
		return facts
	}

	var filtered []Fact
	for _, f := range facts {
		match := true
		for i, arg := range args {
			if i < len(f.Args) && arg != "" {
				stored := fmt.Sprintf("%v", f.Args[i])
				// Handle Mangle named constants (prefixed with /)
				// Compare both the raw value and stripped version
				if stored != arg && stored != "/"+arg && strings.TrimPrefix(stored, "/") != arg {
					match = false
					break
				}
			}
		}
		if match {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// EvaluateRule evaluates a specific rule and returns matching facts.
func (e *Engine) EvaluateRule(predicate string) []Fact {
	facts, _ := e.GetFacts(predicate)
	return facts
}
