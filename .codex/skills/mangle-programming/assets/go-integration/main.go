// Mangle Go Integration Boilerplate
// A minimal example showing how to embed Mangle in a Go application
//
// Usage:
//   go mod init myproject
//   go get github.com/google/mangle@v0.4.0
//   go run main.go
//
// Mangle v0.4.0 compatible

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/mangle/analysis"
	"github.com/google/mangle/ast"
	_ "github.com/google/mangle/builtin"
	"github.com/google/mangle/engine"
	"github.com/google/mangle/factstore"
	"github.com/google/mangle/parse"
)

// MangleEngine wraps the Mangle evaluation engine
type MangleEngine struct {
	store       factstore.FactStore
	programInfo *analysis.ProgramInfo
}

// NewMangleEngine creates a new engine from Mangle source
func NewMangleEngine(source string) (*MangleEngine, error) {
	// Parse the source
	unit, err := parse.Unit(strings.NewReader(source))
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Analyze the program
	programInfo, err := analysis.AnalyzeOneUnit(unit, nil)
	if err != nil {
		return nil, fmt.Errorf("analysis error: %w", err)
	}

	// Create fact store
	store := factstore.NewSimpleInMemoryStore()

	// Evaluate to fixed point
	if _, err := engine.EvalProgramWithStats(programInfo, store); err != nil {
		return nil, fmt.Errorf("evaluation error: %w", err)
	}

	return &MangleEngine{
		store:       store,
		programInfo: programInfo,
	}, nil
}

// AddFact adds a new fact to the engine
func (e *MangleEngine) AddFact(predicate string, args ...interface{}) error {
	terms := make([]ast.BaseTerm, len(args))
	for i, arg := range args {
		term, err := convertToTerm(arg)
		if err != nil {
			return fmt.Errorf("arg %d: %w", i, err)
		}
		terms[i] = term
	}

	atom := ast.NewAtom(predicate, terms...)
	e.store.Add(atom)

	// Re-evaluate
	_, err := engine.EvalProgramWithStats(e.programInfo, e.store)
	return err
}

// Query returns all facts matching a predicate
func (e *MangleEngine) Query(predicate string, arity int) ([][]interface{}, error) {
	pred := ast.PredicateSym{Symbol: predicate, Arity: arity}
	query := ast.NewQuery(pred)

	var results [][]interface{}
	err := e.store.GetFacts(query, func(atom ast.Atom) error {
		row := make([]interface{}, len(atom.Args))
		for i, arg := range atom.Args {
			row[i] = termToValue(arg)
		}
		results = append(results, row)
		return nil
	})

	return results, err
}

// convertToTerm converts a Go value to a Mangle AST term
func convertToTerm(v interface{}) (ast.BaseTerm, error) {
	switch val := v.(type) {
	case string:
		if strings.HasPrefix(val, "/") {
			return ast.Name(val)
		}
		return ast.String(val), nil
	case int:
		return ast.Number(int64(val)), nil
	case int64:
		return ast.Number(val), nil
	case float64:
		return ast.Float64(val), nil
	case bool:
		if val {
			return ast.TrueConstant, nil
		}
		return ast.FalseConstant, nil
	default:
		return nil, fmt.Errorf("unsupported type: %T", v)
	}
}

// termToValue converts a Mangle AST term to a Go value
func termToValue(term ast.BaseTerm) interface{} {
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
			return t.String()
		}
	case ast.Variable:
		return "?" + t.Symbol
	default:
		return fmt.Sprintf("%v", term)
	}
}

func main() {
	// Example Mangle program
	program := `
		# Schema
		Decl parent(Parent.Type<n>, Child.Type<n>).
		Decl ancestor(Ancestor.Type<n>, Descendant.Type<n>).

		# Base facts
		parent(/oedipus, /antigone).
		parent(/oedipus, /ismene).
		parent(/antigone, /thersander).

		# Recursive rule
		ancestor(A, D) :- parent(A, D).
		ancestor(A, D) :- parent(A, C), ancestor(C, D).
	`

	// Create engine
	eng, err := NewMangleEngine(program)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	// Query parent relationship
	fmt.Println("=== Parents ===")
	parents, err := eng.Query("parent", 2)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	for _, row := range parents {
		fmt.Printf("  %v is parent of %v\n", row[0], row[1])
	}

	// Query derived ancestors
	fmt.Println("\n=== Ancestors ===")
	ancestors, err := eng.Query("ancestor", 2)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	for _, row := range ancestors {
		fmt.Printf("  %v is ancestor of %v\n", row[0], row[1])
	}

	// Add a new fact dynamically
	fmt.Println("\n=== Adding new fact ===")
	if err := eng.AddFact("parent", "/thersander", "/tisamenus"); err != nil {
		log.Fatalf("AddFact failed: %v", err)
	}

	// Query again to see new derived facts
	fmt.Println("=== Updated Ancestors ===")
	ancestors, _ = eng.Query("ancestor", 2)
	for _, row := range ancestors {
		fmt.Printf("  %v is ancestor of %v\n", row[0], row[1])
	}

	// Demonstrate timeout handling
	fmt.Println("\n=== With Timeout ===")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ctx // Use with QueryContext for timeout support
}
