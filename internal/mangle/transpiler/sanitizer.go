package transpiler

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"codenerd/internal/mangle"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
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

// UpdateFromProgramInfo refreshes the sanitizer's predicate/type map from parsed ProgramInfo.
// This is preferred over UpdateSchema when analysis.ProgramInfo is available.
func (s *Sanitizer) UpdateFromProgramInfo(info *analysis.ProgramInfo) {
	s.validator.UpdateFromProgramInfo(info)
}

// Sanitize cleans up LLM-generated Mangle code.
// Pipeline:
// 1. Preprocess: Fix "=" assignments for aggregations (SQL style -> Temp Predicate)
// 2. Parse: Convert to AST
// 3. Pass 1 - Atom Interning: "string" -> /atom based on Schema
// 4. Pass 2 - Aggregation Repair: temp_agg -> |> do fn:group_by(...)...
// 5. Pass 3 - Safety Injection: unsafe(X) :- not safe(X) -> unsafe(X) :- candidate(X), not safe(X)
// 6. Serialize: specific string formatting
// 7. Reparse gate: the output must parse, or Sanitize fails
func (s *Sanitizer) Sanitize(raw string) (string, error) {
	// 1. Preprocess SQL-style aggregations
	// Pattern: Res = count(Var) -> llm_agg("count", Res, Var)
	preprocessed := s.preprocessAggregations(raw)

	// 2. Parse
	unit, err := mangle.ParseUnit(strings.NewReader(preprocessed))
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
	out, err := s.serializeUnit(unit.Decls, newClauses)
	if err != nil {
		return "", err
	}
	// 5. Reparse gate: a repair that does not parse (e.g. interning a
	// string with spaces into a bare /atom) must fail here with a clear
	// error, not travel downstream to fail the sandbox with the evidence
	// of what the sanitizer did already lost.
	if _, err := mangle.ParseUnit(strings.NewReader(out)); err != nil {
		return "", fmt.Errorf("sanitized output does not parse: %w", err)
	}
	return out, nil
}

// aggPattern matches: VAR = count(VAR) or VAR = sum(VAR)
// Limitiation: Simple cases only.
// Groups: 1=ResVar, 2=Func, 3=ArgVar
var aggPattern = regexp.MustCompile(`([A-Z][a-zA-Z0-9_]*)\s*=\s*(count|sum|min|max|avg)\(([A-Z][a-zA-Z0-9_]*)\)`)

// preprocessAggregations converts invalid `VAR = AGG(VAR)` syntax to a temporary valid predicate `llm_agg`.
func (s *Sanitizer) preprocessAggregations(raw string) string {
	// Replacement: llm_agg("FUNC", ResVar, ArgVar)
	// We quote the func name to make it a string constant.
	// Matches inside string literals are left alone: rewriting
	// desc("N = sum(X)") would corrupt data into code.
	matches := aggPattern.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		return raw
	}
	var sb strings.Builder
	sb.Grow(len(raw) + len(matches)*8)
	pos := 0
	for _, m := range matches {
		if isInsideString(raw, m[0]) {
			continue
		}
		sb.WriteString(raw[pos:m[0]])
		sb.WriteString(`llm_agg("`)
		sb.WriteString(raw[m[4]:m[5]])
		sb.WriteString(`", `)
		sb.WriteString(raw[m[2]:m[3]])
		sb.WriteString(`, `)
		sb.WriteString(raw[m[6]:m[7]])
		sb.WriteString(`)`)
		pos = m[1]
	}
	sb.WriteString(raw[pos:])
	return sb.String()
}

// isInsideString reports whether offset falls inside a double-quoted span,
// honouring backslash escapes. Unbalanced quotes fail closed (in string).
func isInsideString(s string, offset int) bool {
	inString := false
	escaped := false
	for i := 0; i < offset && i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
		}
	}
	return inString
}

// SanitizeAtoms acts as the public entry point for just Atom Interning (Pass 1).
func (s *Sanitizer) SanitizeAtoms(raw string) (string, error) {
	unit, err := mangle.ParseUnit(strings.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}

	var newClauses []ast.Clause
	for _, clause := range unit.Clauses {
		newClauses = append(newClauses, s.transformClauseAtoms(clause))
	}

	out, err := s.serializeUnit(unit.Decls, newClauses)
	if err != nil {
		return "", err
	}
	if _, err := mangle.ParseUnit(strings.NewReader(out)); err != nil {
		return "", fmt.Errorf("sanitized output does not parse: %w", err)
	}
	return out, nil
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
						// Build via the constructor so the constant is valid
						// (hash included). If the string cannot be a name at
						// all, keep the original: inventing garbage helps no one.
						if name, err := ast.Name(cleanVal); err == nil {
							newTerm = name
						}
					}
				}
			}
		}
		newArgs = append(newArgs, newTerm)
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
	var aggInfos []*aggDetails

	for _, term := range clause.Premises {
		if atom, ok := term.(ast.Atom); ok {
			if atom.Predicate.Symbol == "llm_agg" {
				// Found a marker!
				// Args: "Func", ResVar, ArgVar
				if len(atom.Args) == 3 {
					// DEFENSIVE: Safe type assertions to prevent panics
					funcConst, funcOk := atom.Args[0].(ast.Constant)
					resVar, resOk := atom.Args[1].(ast.Variable)
					argVar, argOk := atom.Args[2].(ast.Variable)

					if funcOk && resOk && argOk {
						// Remove quotes from function name
						funcName := strings.Trim(funcConst.Symbol, "\"")

						aggInfos = append(aggInfos, &aggDetails{
							Fn:     funcName,
							Result: resVar,
							Arg:    argVar,
						})
						// Do NOT append to cleanPremises
						continue
					}
					// If type assertions fail, fall through to append as regular premise
				}
			}
		}
		cleanPremises = append(cleanPremises, term)
	}

	if len(aggInfos) == 0 {
		return clause, nil
	}

	// We have aggregations. We need to construct the transform.
	// Since we can't easily construct ast.Transform nodes (private/complex),
	// we will use a "Synthetic Transform" strategy: inject a SPECIAL PREMISE
	// that serializeUnit detects and writes as a pipe.
	// Marker: sys_emit_pipe("group_by_vars", "func", "res", "arg", ...)

	// infer group_by keys: All variables in Head EXCEPT aggregation results
	headVars := make(map[string]bool)
	collectVarsFromBaseTerm := func(bt ast.BaseTerm) {
		if v, ok := bt.(ast.Variable); ok {
			headVars[v.Symbol] = true
		}
	}
	for _, arg := range clause.Head.Args {
		collectVarsFromBaseTerm(arg)
	}
	for _, info := range aggInfos {
		delete(headVars, info.Result.Symbol)
	}

	groupByKeys := make([]string, 0, len(headVars))
	for k := range headVars {
		groupByKeys = append(groupByKeys, k)
	}
	sort.Strings(groupByKeys)

	// Marker args: group_by, then one (func, res, arg) triple per aggregation.
	markerPred := ast.PredicateSym{Symbol: "sys_emit_pipe", Arity: 1 + 3*len(aggInfos)}
	args := []ast.BaseTerm{
		ast.Constant{Type: ast.StringType, Symbol: strings.Join(groupByKeys, ",")},
	}
	for _, info := range aggInfos {
		args = append(args,
			ast.Constant{Type: ast.StringType, Symbol: info.Fn},
			ast.Constant{Type: ast.StringType, Symbol: info.Result.Symbol},
			ast.Constant{Type: ast.StringType, Symbol: info.Arg.Symbol},
		)
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

// parsePipeMarker reads a sys_emit_pipe marker premise: a group-by key list
// followed by one (func, result, arg) triple per aggregation. Every arg must
// be a string constant; anything else is a malformed (likely hand-written)
// marker and fails closed.
func parsePipeMarker(atom ast.Atom) (string, []*aggDetails, error) {
	if len(atom.Args) < 4 || (len(atom.Args)-1)%3 != 0 {
		return "", nil, fmt.Errorf("malformed sys_emit_pipe marker: want 1+3n args, got %d", len(atom.Args))
	}
	consts := make([]string, len(atom.Args))
	for i, arg := range atom.Args {
		c, ok := arg.(ast.Constant)
		if !ok || c.Type != ast.StringType {
			return "", nil, fmt.Errorf("malformed sys_emit_pipe marker: arg %d is not a string constant", i)
		}
		consts[i] = c.Symbol
	}
	var aggs []*aggDetails
	for i := 1; i < len(consts); i += 3 {
		aggs = append(aggs, &aggDetails{
			Fn:     consts[i],
			Result: ast.Variable{Symbol: consts[i+1]},
			Arg:    ast.Variable{Symbol: consts[i+2]},
		})
	}
	return consts[0], aggs, nil
}

func (s *Sanitizer) rectifySafety(clause ast.Clause) ast.Clause {
	// Only inject bindings the schema actually provides. An unconditional
	// candidate_node premise invents a predicate the LLM never wrote and no
	// schema declares, so the rule fails downstream with a mystifying
	// "undeclared predicate" instead of the real, actionable unsafe-variable
	// error. When no generator is known, the rule passes through untouched.
	if _, known := s.validator.ValidPredicates["candidate_node"]; !known {
		return clause
	}

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
		var groupBy string
		var pipeAggs []*aggDetails
		var normalPremises []ast.Term

		for _, p := range c.Premises {
			if atom, ok := p.(ast.Atom); ok && atom.Predicate.Symbol == "sys_emit_pipe" {
				// Parse marker defensively: the input is untrusted LLM text
				// and a hand-written sys_emit_pipe with the wrong shape must
				// error, not panic on a failed type assertion.
				gb, aggs, err := parsePipeMarker(atom)
				if err != nil {
					return "", err
				}
				groupBy = gb
				pipeAggs = append(pipeAggs, aggs...)
			} else {
				normalPremises = append(normalPremises, p)
			}
		}

		// Construct base string
		tempClause := ast.Clause{Head: c.Head, Premises: normalPremises, Transform: c.Transform}
		clauseStr := tempClause.String()

		// Robustly remove trailing period if present
		clauseStr = strings.TrimSuffix(strings.TrimSpace(clauseStr), ".")

		sb.WriteString(clauseStr)

		// Append Pipe if needed
		if len(pipeAggs) > 0 {
			sb.WriteString(fmt.Sprintf(" |> do fn:group_by(%s)", groupBy))
			for _, agg := range pipeAggs {
				sb.WriteString(fmt.Sprintf(", let %s = fn:%s(%s)",
					agg.Result.Symbol, agg.Fn, agg.Arg.Symbol))
			}
		}

		sb.WriteString(".\n")

		if i < len(clauses)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}
