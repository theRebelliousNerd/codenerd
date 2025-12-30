package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"codenerd/internal/logging"

	"github.com/google/mangle/ast"
	"github.com/google/mangle/parse"
)

// =============================================================================
// QUERY METHODS
// =============================================================================

// Query retrieves facts for a predicate, optionally filtering by a pattern.
// Accepts either a bare predicate name (e.g., "user_intent") or a pattern with
// arguments (e.g., "selected_result(Atom, Priority, Source)" or "next_action(/generate_tool)").
//
// Pattern filtering rules:
// - Variables (e.g., Atom, X, _) are treated as wildcards.
// - Constants (name constants like /foo, strings like "bar", numbers) must match.
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

	// Parse optional pattern form, using the official Mangle parser for correctness.
	// If parsing fails, fall back to predicate-only query.
	var (
		patternFact   Fact
		hasPattern    bool
		desiredArity  int
		predicateName = predicate
	)
	if idx := strings.Index(predicate, "("); idx > 0 {
		// Fast path: extract predicate name even if full parse fails.
		predicateName = strings.TrimSpace(predicate[:idx])
		if parsedFact, err := ParseFactString(predicate); err == nil {
			patternFact = parsedFact
			hasPattern = true
			desiredArity = len(parsedFact.Args)
			predicateName = parsedFact.Predicate
		}
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
		if pred.Symbol == predicateName && (!hasPattern || pred.Arity == desiredArity) {
			predicateFound = true
			// Query the store for all atoms of this predicate
			k.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
				fact := atomToFact(a)
				// If a pattern was provided, filter by constants.
				if !hasPattern || factMatchesPattern(fact, patternFact) {
					results = append(results, fact)
				}
				return nil
			})
			break
		}
	}

	if !predicateFound {
		// Upgraded from Debug to Warn: missing predicates may indicate
		// schema drift or missing declarations, which can cause silent bugs.
		logging.Get(logging.CategoryKernel).Warn("Query: predicate '%s' not found in declarations", predicateName)
	}

	// JIT-related predicate debugging - log at INFO level for visibility
	jitPredicates := map[string]bool{
		"selected_atom": true, "is_mandatory": true, "mandatory_atom": true,
		"prohibited_atom": true, "compilation_valid": true, "candidate_atom": true,
	}
	if jitPredicates[predicateName] {
		logging.Kernel("JIT-Query: %s found=%v results=%d", predicateName, predicateFound, len(results))
	}

	elapsed := timer.Stop()
	logging.KernelDebug("Query: predicate=%s returned %d results", predicate, len(results))
	logging.Audit().KernelQuery(predicate, len(results), elapsed.Milliseconds())
	return results, nil
}

func factMatchesPattern(f Fact, pattern Fact) bool {
	if f.Predicate != pattern.Predicate {
		return false
	}
	if len(f.Args) != len(pattern.Args) {
		return false
	}
	for i := range pattern.Args {
		if !patternArgMatches(pattern.Args[i], f.Args[i]) {
			return false
		}
	}
	return true
}

func patternArgMatches(pattern interface{}, value interface{}) bool {
	// Variables are represented as strings like "?X" by atomToFact/baseTermToValue.
	if s, ok := pattern.(string); ok && strings.HasPrefix(s, "?") {
		return true
	}

	// OPTIMIZATION: Replace reflect.DeepEqual with type switches
	// Normalize both values first
	normPattern := normalizeQueryValue(pattern)
	normValue := normalizeQueryValue(value)

	// Fast path: pointer equality
	if normPattern == normValue {
		return true
	}

	// Type-based comparison (avoid reflection)
	switch p := normPattern.(type) {
	case int64:
		if v, ok := normValue.(int64); ok {
			return p == v
		}
	case string:
		if v, ok := normValue.(string); ok {
			return p == v
		}
	case MangleAtom:
		if v, ok := normValue.(MangleAtom); ok {
			return p == v
		}
		// Cross-compare with string
		if v, ok := normValue.(string); ok {
			return string(p) == v
		}
	case bool:
		if v, ok := normValue.(bool); ok {
			return p == v
		}
	default:
		// FALLBACK: Only for truly unknown types
		// This should rarely execute with well-typed facts
		return reflect.DeepEqual(normPattern, normValue)
	}

	return false
}

func normalizeQueryValue(v interface{}) interface{} {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int64:
		return t
	case float64:
		// Mangle numeric constants are integers; normalize defensively.
		return int64(t)
	default:
		return v
	}
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

// GetDerivedFacts returns all derived facts organized by predicate (alias for QueryAll).
func (k *RealKernel) GetDerivedFacts() (map[string][]Fact, error) {
	return k.QueryAll()
}

// LoadFactsFromFile loads facts from a .mg file and adds them to the EDB.
func (k *RealKernel) LoadFactsFromFile(path string) error {
	logging.KernelDebug("LoadFactsFromFile: loading facts from %s", path)

	data, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("LoadFactsFromFile: failed to read %s: %v", path, err)
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	facts, err := ParseFactsFromString(string(data))
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("LoadFactsFromFile: failed to parse facts from %s: %v", path, err)
		return fmt.Errorf("failed to parse facts from %s: %w", path, err)
	}

	if len(facts) == 0 {
		logging.KernelDebug("LoadFactsFromFile: no facts found in %s", path)
		return nil
	}

	logging.Kernel("LoadFactsFromFile: parsed %d facts from %s", len(facts), path)
	return k.LoadFacts(facts)
}

// =============================================================================
// PARSING HELPERS
// =============================================================================

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
		case ast.BytesType:
			return t.Symbol
		case ast.NumberType:
			return t.NumValue
		case ast.Float64Type:
			return t.Float64Value
		default:
			// DEFENSIVE: Log unknown constant types to catch new AST types early
			logging.Kernel("baseTermToValue: unknown constant type %v, using Symbol fallback", t.Type)
			return t.Symbol
		}
	case ast.Variable:
		return fmt.Sprintf("?%s", t.Symbol)
	default:
		return fmt.Sprintf("%v", term)
	}
}

// ParseFactString parses a Mangle fact string into a Fact.
// Format: predicate(arg1, arg2, ...) where args can be:
//   - Name constants: /foo, /bar
//   - Strings: "quoted text"
//   - Numbers: 42, 3.14
func ParseFactString(factStr string) (Fact, error) {
	// Wrap in a minimal program to allow parsing
	programStr := factStr + "."
	parsed, err := parse.Unit(strings.NewReader(programStr))
	if err != nil {
		return Fact{}, fmt.Errorf("failed to parse fact string: %w", err)
	}

	if len(parsed.Clauses) == 0 {
		return Fact{}, fmt.Errorf("no clauses found in fact string")
	}

	// Extract the first clause's head atom
	clause := parsed.Clauses[0]
	// In Mangle AST, Head is an ast.Atom struct, not a pointer
	atom := clause.Head

	return atomToFact(atom), nil
}

// ParseFactsFromString parses multiple facts from a string (one per line or separated by '.').
func ParseFactsFromString(content string) ([]Fact, error) {
	// Parse as a Mangle program
	parsed, err := parse.Unit(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse facts: %w", err)
	}

	facts := make([]Fact, 0, len(parsed.Clauses))
	for _, clause := range parsed.Clauses {
		// Only extract facts from clauses with no body (ground facts)
		if len(clause.Premises) > 0 {
			continue // Skip rules
		}

		// In Mangle AST, Head is an ast.Atom struct, not a pointer
		atom := clause.Head
		facts = append(facts, atomToFact(atom))
	}

	return facts, nil
}

// UpdateSystemFacts updates system-level facts (e.g., time, git state).
// This is a placeholder for dynamic system fact injection.
func (k *RealKernel) UpdateSystemFacts() error {
	now := time.Now().Unix()

	if err := k.Retract("current_time"); err != nil {
		return err
	}
	if assertErr := k.Assert(Fact{Predicate: "current_time", Args: []interface{}{now}}); assertErr != nil {
		return assertErr
	}

	workspaceRoot := strings.TrimSpace(k.workspaceRoot)
	if workspaceRoot == "" {
		logging.KernelDebug("UpdateSystemFacts: workspace root not set, skipping git facts")
		return nil
	}
	if abs, err := filepath.Abs(workspaceRoot); err == nil {
		workspaceRoot = abs
	}
	if info, err := os.Stat(workspaceRoot); err != nil || !info.IsDir() {
		logging.KernelDebug("UpdateSystemFacts: invalid workspace root: %s", workspaceRoot)
		return nil
	}

	gitRoot, err := gitRepoRoot(workspaceRoot)
	if err != nil {
		logging.KernelDebug("UpdateSystemFacts: git root not found: %v", err)
		return nil
	}

	branch, _ := gitCmd(gitRoot, "rev-parse", "--abbrev-ref", "HEAD")
	statusOutput, _ := gitCmd(gitRoot, "status", "--porcelain")
	commitOutput, _ := gitCmd(gitRoot, "log", "-n", "5", "--pretty=format:%s")

	modifiedFiles, unstagedCount := parseGitStatus(statusOutput)
	recentCommits := splitLinesTrimmed(commitOutput)

	_ = k.Retract("git_state")
	_ = k.Retract("git_branch")

	var facts []Fact
	if branch != "" {
		facts = append(facts, Fact{Predicate: "git_state", Args: []interface{}{"branch", branch}})
		facts = append(facts, Fact{Predicate: "git_branch", Args: []interface{}{branch}})
	}
	if len(modifiedFiles) > 0 {
		facts = append(facts, Fact{Predicate: "git_state", Args: []interface{}{"modified_files", strings.Join(modifiedFiles, "\n")}})
	}
	if len(recentCommits) > 0 {
		facts = append(facts, Fact{Predicate: "git_state", Args: []interface{}{"recent_commits", strings.Join(recentCommits, "\n")}})
	}
	facts = append(facts, Fact{Predicate: "git_state", Args: []interface{}{"unstaged_count", strconv.Itoa(unstagedCount)}})

	for _, fact := range facts {
		if assertErr := k.Assert(fact); assertErr != nil {
			return assertErr
		}
	}

	return nil
}

func gitRepoRoot(workspaceRoot string) (string, error) {
	out, err := gitCmd(workspaceRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return out, nil
}

func gitCmd(workspaceRoot string, args ...string) (string, error) {
	if workspaceRoot == "" {
		return "", fmt.Errorf("workspace root is empty")
	}
	cmd := exec.Command("git", append([]string{"-C", workspaceRoot}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func parseGitStatus(statusOutput string) ([]string, int) {
	lines := splitLinesTrimmed(statusOutput)
	files := make([]string, 0, len(lines))
	unstaged := 0

	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		status := line[:2]
		path := strings.TrimSpace(line[2:])
		if path == "" {
			continue
		}
		if strings.Contains(path, " -> ") {
			parts := strings.Split(path, " -> ")
			path = strings.TrimSpace(parts[len(parts)-1])
		}
		files = append(files, path)

		if status == "??" {
			unstaged++
			continue
		}
		if len(status) == 2 && status[1] != ' ' {
			unstaged++
		}
	}

	return dedupeStrings(files), unstaged
}

func splitLinesTrimmed(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
