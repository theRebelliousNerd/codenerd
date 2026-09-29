// Package orient is the orientation engine: one Mangle program, not the
// action kernel, that decides what a repository is from measurements Go
// asserts. History, links, neighbour pairs and path ordinals are
// measurements. Eras, generations, what superseded what, and which
// documents must be read in full are rules.
package orient

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/config"
	"codenerd/internal/mangle"
	"codenerd/internal/types"
)

//go:embed *.mg
var policyFiles embed.FS

// Engine is the orientation program. Assert adds extensional rows,
// Evaluate runs to the fixpoint, Query reads one predicate back.
// Auto-evaluation is off: a scan asserts in more than one batch, and
// evaluating between them would publish judgments over a half-loaded
// repository.
type Engine struct {
	mu     sync.Mutex
	eng    *mangle.Engine
	closed bool
}

// NewEngine loads the embedded orientation rules and the orient thresholds.
// A nil config is refused. Check failures are refused. A threshold the
// rules declare required and the config does not supply is refused: a rule
// over a missing number derives nothing, which for a budget means an empty
// read set with no error.
func NewEngine(cfg *config.OrientConfig) (*Engine, error) {
	if cfg == nil {
		return nil, fmt.Errorf("orient config is nil")
	}
	if probs := cfg.Check("orient"); len(probs) > 0 {
		parts := make([]string, len(probs))
		for i, p := range probs {
			parts[i] = p.Path + ": " + p.Message
		}
		return nil, fmt.Errorf("orient config: %s", strings.Join(parts, "; "))
	}
	return newEngine(cfg.Params())
}

// newEngine is the load path with the threshold rows supplied by the
// caller, so a test can withhold one and see config_param_missing refuse
// the start.
func newEngine(params []config.Param) (*Engine, error) {
	src, err := policySource()
	if err != nil {
		return nil, err
	}
	mcfg := mangle.DefaultConfig()
	// Off so Assert can land history and documents before any judgment.
	mcfg.AutoEval = false
	// A cap here would drop history rows or stop era derivation without
	// saying which facts were cut. Similarity is already bounded by top-k,
	// and the read budget reports what it left out.
	mcfg.FactLimit = 0
	mcfg.DerivedFactsLimit = 0
	eng, err := mangle.NewEngine(mcfg, nil)
	if err != nil {
		return nil, err
	}
	if err = eng.LoadSchemaString(src); err != nil {
		_ = eng.Close()
		return nil, err
	}
	e := &Engine{eng: eng}
	if err = e.Assert(config.ParamFacts(params)); err != nil {
		_ = e.Close()
		return nil, fmt.Errorf("assert orient thresholds: %w", err)
	}
	if err = e.Evaluate(context.Background()); err != nil {
		_ = e.Close()
		return nil, err
	}
	missing, err := e.Query("config_param_missing")
	if err != nil {
		_ = e.Close()
		return nil, err
	}
	if len(missing) > 0 {
		_ = e.Close()
		keys := make([]string, 0, len(missing))
		for _, f := range missing {
			if len(f.Args) == 2 {
				keys = append(keys, types.ExtractString(f.Args[1]))
			}
		}
		sort.Strings(keys)
		return nil, fmt.Errorf("orient policy is missing %s", strings.Join(keys, ", "))
	}
	return e, nil
}

// policySource is schema.mg, then every other embedded .mg in name order.
// schema.mg holds
// the declarations; another lane can add a rule file and have it load
// without an edit here.
func policySource() (string, error) {
	entries, err := policyFiles.ReadDir(".")
	if err != nil {
		return "", err
	}
	schema, err := policyFiles.ReadFile("schema.mg")
	if err != nil {
		return "", fmt.Errorf("orient schema: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".mg") || e.Name() == "schema.mg" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	var b strings.Builder
	b.Write(schema)
	b.WriteString("\n")
	for _, name := range names {
		body, err := fs.ReadFile(policyFiles, name)
		if err != nil {
			return "", err
		}
		b.Write(body)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// Assert adds extensional rows. It does not evaluate. Names that the
// declaration bounds to /name should be types.MangleAtom; paths are plain
// strings and must not start with '/', or this fork stores them as names
// and the join against a /string column is empty.
func (e *Engine) Assert(facts []types.Fact) error {
	if len(facts) == 0 {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return fmt.Errorf("orient engine is closed")
	}
	rows := make([]mangle.Fact, len(facts))
	for i, f := range facts {
		args := make([]any, len(f.Args))
		copy(args, f.Args)
		rows[i] = mangle.Fact{Predicate: f.Predicate, Args: args}
	}
	return e.eng.AddFacts(rows)
}

// Evaluate runs the program to the fixpoint. Cancellation is honoured
// before the run; the fixpoint itself is not interrupted, because a
// half-derived store would answer the next Query as if it were complete.
func (e *Engine) Evaluate(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return fmt.Errorf("orient engine is closed")
	}
	return e.eng.Evaluate()
}

// Query returns every row of one predicate, base or derived. Name
// constants come back as plain strings with their leading slash, numbers
// as int64, the same as a kernel Query.
func (e *Engine) Query(predicate string) ([]types.Fact, error) {
	if predicate == "" {
		return nil, fmt.Errorf("orient query: empty predicate")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, fmt.Errorf("orient engine is closed")
	}
	rows, err := e.eng.GetFacts(predicate)
	if err != nil {
		return nil, err
	}
	out := make([]types.Fact, len(rows))
	for i, f := range rows {
		args := make([]any, len(f.Args))
		copy(args, f.Args)
		out[i] = types.Fact{Predicate: f.Predicate, Args: args}
	}
	return out, nil
}

// Close releases the underlying store.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}
	e.closed = true
	return e.eng.Close()
}
