package transpiler

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"codenerd/internal/mangle"

	"github.com/google/mangle/ast"
	"github.com/google/mangle/parse"
)

// Sanitizer acts as a Compiler Frontend for LLM-generated Mangle logic.
type Sanitizer struct {
	validator *mangle.AtomValidator
}

// NewSanitizer creates a new Sanitizer with core schemas loaded.
func NewSanitizer() *Sanitizer {
	return &Sanitizer{
		validator: mangle.NewAtomValidator(),
	}
}

// LoadPolicy parses a policy.mg file and updates the TypeMap.
func (s *Sanitizer) LoadPolicy(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read policy file: %w", err)
	}
	return s.validator.UpdateFromSchema(string(data))
}

// Sanitize cleans up LLM-generated Mangle code.
// Pipeline:
// 1. Preprocess: Fix "=" assignments for aggregations (SQL style -> Temp Predicate)
// 2. Parse: Convert to AST
// 3. Pass 1 - Atom Interning: "string" -> /atom based on Schema
// 4. Pass 2 - Aggregation Repair: temp_agg -> |> do fn:group_by(...)...
// 5. Pass 3 - Safety Injection: unsafe(X) :- not safe(X) -> unsafe(X) :- candidate(X), not safe(X)
// 6. Serialize: specific string formatting
func (s *Sanitizer) Sanitize(raw string) (string, error) {
	// 1. Preprocess SQL-style aggregations
	// Pattern: Res = count(Var) -> llm_agg("count", Res, Var)
	preprocessed := s.preprocessAggregations(raw)

	// 2. Parse
	unit, err := parse.Unit(strings.NewReader(preprocessed))
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}

	var newClauses []ast.Clause
	for _, clause := range unit.Clauses {
		c := clause
		// Pass 1: Atom Interning
		c = s.transformClauseAtoms(c)
		// Pass 2: Aggregation Repair
		c, err = s.repairAggregations(c)
		if err != nil {
			return "", fmt.Errorf("aggregation repair failed: %w", err)
		}
		// Pass 3: Safety Injection
		c = s.rectifySafety(c)

		newClauses = append(newClauses, c)
	}

	// 4. Serialize
	return s.serializeUnit(unit.Decls, newClauses)
}

// preprocessAggregations converts invalid `VAR = AGG(VAR)` syntax to a temporary valid predicate `llm_agg`.
func (s *Sanitizer) preprocessAggregations(raw string) string {
	// Regex for: VAR = count(VAR) or VAR = sum(VAR)
	// Limitiation: Simple cases only.
	// Groups: 1=ResVar, 2=Func, 3=ArgVar
	re := regexp.MustCompile(`([A-Z][a-zA-Z0-9_]*)\s*=\s*(count|sum|min|max|avg)\(([A-Z][a-zA-Z0-9_]*)\)`)

	// Replacement: llm_agg("FUNC", ResVar, ArgVar)
	// We quote the func name to make it a string constant
	return re.ReplaceAllString(raw, `llm_agg("$2", $1, $3)`)
}

// SanitizeAtoms acts as the public entry point for just Atom Interning (Pass 1).
func (s *Sanitizer) SanitizeAtoms(raw string) (string, error) {
	unit, err := parse.Unit(strings.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}

	var newClauses []ast.Clause
	for _, clause := range unit.Clauses {
		newClauses = append(newClauses, s.transformClauseAtoms(clause))
	}

	return s.serializeUnit(unit.Decls, newClauses)
}

func (s *Sanitizer) transformClauseAtoms(clause ast.Clause) ast.Clause {
	newHead := s.transformAtom(clause.Head)

	var newPremises []ast.Term
	for _, term := range clause.Premises {
		newTerm := s.transformTerm(term)
		newPremises = append(newPremises, newTerm)
	}

	return ast.Clause{
		Head:      newHead,
		Premises:  newPremises,
		Transform: clause.Transform,
	}
}

func (s *Sanitizer) transformTerm(term ast.Term) ast.Term {
	switch t := term.(type) {
	case ast.Atom:
		return s.transformAtom(t)
	case ast.NegAtom:
		return ast.NegAtom{Atom: s.transformAtom(t.Atom)}
	default:
		return t
	}
}

func (s *Sanitizer) transformAtom(atom ast.Atom) ast.Atom {
	predName := atom.Predicate.Symbol
	spec, known := s.validator.ValidPredicates[predName]

	if !known {
		return atom
	}

	var newArgs []ast.BaseTerm
	for i, argBase := range atom.Args {
		newTerm := argBase

		switch t := argBase.(type) {
		case ast.Constant:
			if i < len(spec.Args) {
				expectedType := spec.Args[i].Type
				if expectedType == mangle.ArgTypeName {
					// Check if it's currently a String but needs to be a Name
					if t.Type == ast.StringType {
						val := t.Symbol
						cleanVal := strings.Trim(val, "\"")
						// Ensure slash prefix
						if !strings.HasPrefix(cleanVal, "/") {
							cleanVal = "/" + cleanVal
						}
						// Create new Name constant
						// Note: ast.Name() constuctor might return error, but here we manually construct
						// assuming internal structure is similar or use ast factory if available.
						// Mangle ast.Constant is a struct.
						newTerm = ast.Constant{
							Type:   ast.NameType,
							Symbol: cleanVal,
						}
					}
				}
			}
		}
		newArgs = append(newArgs, newTerm.(ast.BaseTerm))
	}

	return ast.Atom{
		Predicate: atom.Predicate,
		Args:      newArgs,
	}
}

// repairAggregations handles the "Pipe Fix".
// It looks for `llm_agg` in premises and moves them to |> do fn:group_by(...), let ...
func (s *Sanitizer) repairAggregations(clause ast.Clause) (ast.Clause, error) {
	var cleanPremises []ast.Term
	var aggInfo *aggDetails

	for _, term := range clause.Premises {
		if atom, ok := term.(ast.Atom); ok {
			if atom.Predicate.Symbol == "llm_agg" {
				// Found a marker!
				// Args: "Func", ResVar, ArgVar
				if len(atom.Args) == 3 {
					funcNameStr := atom.Args[0].(ast.Constant).Symbol
					// Remove quotes
					funcName := strings.Trim(funcNameStr, "\"")

					resVar := atom.Args[1].(ast.Variable)
					argVar := atom.Args[2].(ast.Variable)

					aggInfo = &aggDetails{
						Fn:     funcName,
						Result: resVar,
						Arg:    argVar,
					}
					// Do NOT append to cleanPremises
					continue
				}
			}
		}
		cleanPremises = append(cleanPremises, term)
	}

	if aggInfo == nil {
		return clause, nil
	}

	// We have an aggregation. We need to construct the transform.
	// Since we can't easily construct ast.Transform nodes (private/complex),
	// we will use a "Synthetic Transform" strategy:
	// We will inject a special "Comment" atom that serializeUnit will look for
	// and write as a pipe.
	// Or better: We rely on the fact that we return a Clause, and we can't create Transform easily.
	// So we will Inject a SPECIAL PREMISE that serializeUnit detects.
	// Marker: sys_emit_pipe("group_by_vars", "func", "res", "arg")

	// infer group_by keys: All variables in Head EXCEPT Result
	headVars := make(map[string]bool)
	collectVarsFromBaseTerm := func(bt ast.BaseTerm) {
		if v, ok := bt.(ast.Variable); ok {
			headVars[v.Symbol] = true
		}
	}
	for _, arg := range clause.Head.Args {
		collectVarsFromBaseTerm(arg)
	}
	delete(headVars, aggInfo.Result.Symbol)

	var groupByKeys []string
	for k := range headVars {
		groupByKeys = append(groupByKeys, k)
	}

	// Create marker atom
	markerPred := ast.PredicateSym{Symbol: "sys_emit_pipe", Arity: 4}
	args := []ast.BaseTerm{
		ast.Constant{Type: ast.StringType, Symbol: strings.Join(groupByKeys, ",")},
		ast.Constant{Type: ast.StringType, Symbol: aggInfo.Fn},
		ast.Constant{Type: ast.StringType, Symbol: aggInfo.Result.Symbol},
		ast.Constant{Type: ast.StringType, Symbol: aggInfo.Arg.Symbol},
	}

	cleanPremises = append(cleanPremises, ast.Atom{Predicate: markerPred, Args: args})

	return ast.Clause{
		Head:      clause.Head,
		Premises:  cleanPremises,
		Transform: clause.Transform,
	}, nil
}

type aggDetails struct {
	Fn     string
	Result ast.Variable
	Arg    ast.Variable
}

func (s *Sanitizer) rectifySafety(clause ast.Clause) ast.Clause {
	positiveVars := make(map[string]bool)

	for _, term := range clause.Premises {
		// Only look at user atoms, ignore system markers
		if atom, ok := term.(ast.Atom); ok && !strings.HasPrefix(atom.Predicate.Symbol, "sys_") {
			s.collectVars(atom, positiveVars)
		}
	}

	var injectClauses []ast.Term
	seenUnbound := make(map[string]bool)

	for _, term := range clause.Premises {
		switch t := term.(type) {
		case ast.NegAtom:
			negVars := make(map[string]bool)
			s.collectVars(t.Atom, negVars)

			for v := range negVars {
				if !positiveVars[v] && !seenUnbound[v] {
					seenUnbound[v] = true
					// Inject candidate_node(v)
					// We construct it manually to avoid parser overhead
					pred := ast.PredicateSym{Symbol: "candidate_node", Arity: 1}
					args := []ast.BaseTerm{ast.Variable{Symbol: v}}
					injectClauses = append(injectClauses, ast.Atom{Predicate: pred, Args: args})
				}
			}
		}
	}

	// Prepend injected clauses
	newPremises := append(injectClauses, clause.Premises...)

	return ast.Clause{
		Head:      clause.Head,
		Premises:  newPremises,
		Transform: clause.Transform,
	}
}

func (s *Sanitizer) collectVars(atom ast.Atom, vars map[string]bool) {
	for _, arg := range atom.Args {
		switch t := arg.(type) {
		case ast.Variable:
			vars[t.Symbol] = true
		}
	}
}

// serializeUnit converts AST back to string, handling the special sys_emit_pipe marker.
func (s *Sanitizer) serializeUnit(decls []ast.Decl, clauses []ast.Clause) (string, error) {
	var sb strings.Builder
	for _, d := range decls {
		// Defensive: Skip invalid/synthetic declarations
		// The Mangle parser generates a synthetic "Package()" decl for bare facts
		sym := d.DeclaredAtom.Predicate.Symbol
		if sym == "" || sym == "Package" || d.DeclaredAtom.String() == "." {
			continue
		}
		sb.WriteString("Decl ")
		sb.WriteString(d.DeclaredAtom.String())
		sb.WriteString(".\n")
	}
	if sb.Len() > 0 && len(clauses) > 0 {
		sb.WriteString("\n")
	}

	for i, c := range clauses {
		// Check for sys_emit_pipe in premises
		var params *aggDetails
		var groupBy string
		var normalPremises []ast.Term

		for _, p := range c.Premises {
			if atom, ok := p.(ast.Atom); ok && atom.Predicate.Symbol == "sys_emit_pipe" {
				// Parse marker
				groupBy = atom.Args[0].(ast.Constant).Symbol
				fn := atom.Args[1].(ast.Constant).Symbol
				res := atom.Args[2].(ast.Constant).Symbol
				arg := atom.Args[3].(ast.Constant).Symbol
				params = &aggDetails{
					Fn:     fn,
					Result: ast.Variable{Symbol: res},
					Arg:    ast.Variable{Symbol: arg},
				}
			} else {
				normalPremises = append(normalPremises, p)
			}
		}

		// Construct base string
		tempClause := ast.Clause{Head: c.Head, Premises: normalPremises, Transform: c.Transform}
		clauseStr := tempClause.String()

		// Robustly remove trailing period if present
		clauseStr = strings.TrimSpace(clauseStr)
		if strings.HasSuffix(clauseStr, ".") {
			clauseStr = clauseStr[:len(clauseStr)-1]
		}

		sb.WriteString(clauseStr)

		// Append Pipe if needed
		if params != nil {
			sb.WriteString(fmt.Sprintf(" |> do fn:group_by(%s), let %s = fn:%s(%s)",
				groupBy, params.Result.Symbol, params.Fn, params.Arg.Symbol))
		}

		sb.WriteString(".\n")

		if i < len(clauses)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}
