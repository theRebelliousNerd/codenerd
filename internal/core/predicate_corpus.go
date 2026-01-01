// Package core provides the predicate corpus API for schema validation and JIT selection.
package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"codenerd/internal/core/defaults"

	_ "github.com/mattn/go-sqlite3"
)

// PredicateCorpus provides access to the predicate schema database.
// It supports:
// - Predicate lookup and validation
// - Domain-based filtering for JIT selection
// - Error pattern matching for repair guidance
// - Example retrieval for context injection
type PredicateCorpus struct {
	db       *sql.DB
	tempFile string // For cleanup
	mu       sync.RWMutex
}

// PredicateInfo contains metadata about a predicate.
type PredicateInfo struct {
	ID          int64
	Name        string
	Arity       int
	Type        string // "EDB" or "IDB"
	Category    string
	Description string
	SafetyLevel string
	Domain      string
	Section     string
	SourceFile  string
}

// PredicateArg describes a single argument of a predicate.
type PredicateArg struct {
	Position        int
	Name            string
	Type            string // "atom", "string", "number", "list", "map", "any"
	IsBoundRequired bool
}

// ErrorPatternInfo describes a common error pattern.
type ErrorPatternInfo struct {
	ID               int64
	Name             string
	ErrorRegex       string
	CauseDescription string
	FixTemplate      string
	Severity         string
}

// PredicateExampleInfo describes an example usage pattern.
type PredicateExampleInfo struct {
	ID            int64
	PredicateName string
	ExampleCode   string
	Explanation   string
	IsCorrect     bool
	ErrorType     string
	Source        string
}

// NewPredicateCorpus creates a new PredicateCorpus from the embedded database.
func NewPredicateCorpus() (*PredicateCorpus, error) {
	// Check if embedded corpus is available
	if !defaults.PredicateCorpusAvailable() {
		return nil, fmt.Errorf("predicate corpus not available (run predicate_corpus_builder)")
	}

	// Read embedded database
	data, err := defaults.PredicateCorpusDB.ReadFile("predicate_corpus.db")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded corpus: %w", err)
	}

	// Write to temp file for SQLite access
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "predicate_corpus.db")
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp corpus: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", tempFile+"?mode=ro")
	if err != nil {
		os.Remove(tempFile)
		return nil, fmt.Errorf("failed to open corpus database: %w", err)
	}

	return &PredicateCorpus{
		db:       db,
		tempFile: tempFile,
	}, nil
}

// NewPredicateCorpusFromPath creates a PredicateCorpus from a file path (for testing).
func NewPredicateCorpusFromPath(path string) (*PredicateCorpus, error) {
	db, err := sql.Open("sqlite3", path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("failed to open corpus database: %w", err)
	}

	return &PredicateCorpus{
		db: db,
	}, nil
}

// Close closes the corpus database and cleans up temp files.
func (pc *PredicateCorpus) Close() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.db != nil {
		pc.db.Close()
	}
	if pc.tempFile != "" {
		os.Remove(pc.tempFile)
	}
	return nil
}

// IsDeclared checks if a predicate name is declared in the corpus.
func (pc *PredicateCorpus) IsDeclared(name string) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	var count int
	err := pc.db.QueryRow("SELECT COUNT(*) FROM predicates WHERE name = ?", name).Scan(&count)
	return err == nil && count > 0
}

// GetPredicate retrieves info about a specific predicate.
func (pc *PredicateCorpus) GetPredicate(name string) (*PredicateInfo, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	var info PredicateInfo
	err := pc.db.QueryRow(`
		SELECT id, name, arity, type, category, description, safety_level, domain, section, source_file
		FROM predicates WHERE name = ?
	`, name).Scan(
		&info.ID, &info.Name, &info.Arity, &info.Type, &info.Category,
		&info.Description, &info.SafetyLevel, &info.Domain, &info.Section, &info.SourceFile,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// GetPredicateArgs retrieves the argument definitions for a predicate.
func (pc *PredicateCorpus) GetPredicateArgs(predicateID int64) ([]PredicateArg, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	rows, err := pc.db.Query(`
		SELECT arg_position, arg_name, arg_type, is_bound_required
		FROM predicate_args WHERE predicate_id = ?
		ORDER BY arg_position
	`, predicateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var args []PredicateArg
	for rows.Next() {
		var arg PredicateArg
		if err := rows.Scan(&arg.Position, &arg.Name, &arg.Type, &arg.IsBoundRequired); err != nil {
			return nil, err
		}
		args = append(args, arg)
	}
	return args, nil
}

// GetByDomain retrieves all predicates in a specific domain (for JIT selection).
func (pc *PredicateCorpus) GetByDomain(domain string) ([]PredicateInfo, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	rows, err := pc.db.Query(`
		SELECT p.id, p.name, p.arity, p.type, p.category, p.description, p.safety_level, p.domain, p.section, p.source_file
		FROM predicates p
		JOIN predicate_domains pd ON p.id = pd.predicate_id
		WHERE pd.domain = ?
		ORDER BY p.name
	`, domain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var predicates []PredicateInfo
	for rows.Next() {
		var info PredicateInfo
		if err := rows.Scan(
			&info.ID, &info.Name, &info.Arity, &info.Type, &info.Category,
			&info.Description, &info.SafetyLevel, &info.Domain, &info.Section, &info.SourceFile,
		); err != nil {
			return nil, err
		}
		predicates = append(predicates, info)
	}
	return predicates, nil
}

// GetByCategory retrieves all predicates in a specific category.
func (pc *PredicateCorpus) GetByCategory(category string) ([]PredicateInfo, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	rows, err := pc.db.Query(`
		SELECT id, name, arity, type, category, description, safety_level, domain, section, source_file
		FROM predicates WHERE category = ?
		ORDER BY name
	`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var predicates []PredicateInfo
	for rows.Next() {
		var info PredicateInfo
		if err := rows.Scan(
			&info.ID, &info.Name, &info.Arity, &info.Type, &info.Category,
			&info.Description, &info.SafetyLevel, &info.Domain, &info.Section, &info.SourceFile,
		); err != nil {
			return nil, err
		}
		predicates = append(predicates, info)
	}
	return predicates, nil
}

// GetAllPredicateNames returns all predicate names in the corpus.
func (pc *PredicateCorpus) GetAllPredicateNames() ([]string, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	rows, err := pc.db.Query("SELECT DISTINCT name FROM predicates ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

// GetAllPredicateSignatures returns all predicate signatures in the corpus as "name/arity".
func (pc *PredicateCorpus) GetAllPredicateSignatures() ([]string, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	rows, err := pc.db.Query("SELECT name, arity FROM predicates ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signatures []string
	for rows.Next() {
		var name string
		var arity int
		if err := rows.Scan(&name, &arity); err != nil {
			return nil, err
		}
		signatures = append(signatures, fmt.Sprintf("%s/%d", name, arity))
	}
	return signatures, nil
}

// ValidatePredicates checks if all predicates in the list are declared.
// Returns the list of undefined predicates.
func (pc *PredicateCorpus) ValidatePredicates(predicates []string) []string {
	var undefined []string
	for _, pred := range predicates {
		if !pc.IsDeclared(pred) {
			undefined = append(undefined, pred)
		}
	}
	return undefined
}

// GetErrorPatterns retrieves all error patterns for repair guidance.
func (pc *PredicateCorpus) GetErrorPatterns() ([]ErrorPatternInfo, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	rows, err := pc.db.Query(`
		SELECT id, pattern_name, error_regex, cause_description, fix_template, severity
		FROM error_patterns
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patterns []ErrorPatternInfo
	for rows.Next() {
		var p ErrorPatternInfo
		if err := rows.Scan(&p.ID, &p.Name, &p.ErrorRegex, &p.CauseDescription, &p.FixTemplate, &p.Severity); err != nil {
			return nil, err
		}
		patterns = append(patterns, p)
	}
	return patterns, nil
}

// FindErrorPattern finds an error pattern by name.
func (pc *PredicateCorpus) FindErrorPattern(name string) (*ErrorPatternInfo, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	var p ErrorPatternInfo
	err := pc.db.QueryRow(`
		SELECT id, pattern_name, error_regex, cause_description, fix_template, severity
		FROM error_patterns WHERE pattern_name = ?
	`, name).Scan(&p.ID, &p.Name, &p.ErrorRegex, &p.CauseDescription, &p.FixTemplate, &p.Severity)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetExamplesForPredicate retrieves examples for a specific predicate.
func (pc *PredicateCorpus) GetExamplesForPredicate(predicateName string, correctOnly bool) ([]PredicateExampleInfo, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	query := `
		SELECT id, predicate_name, example_code, explanation, is_correct, error_type, source
		FROM predicate_examples WHERE predicate_name = ?
	`
	if correctOnly {
		query += " AND is_correct = 1"
	}
	query += " ORDER BY is_correct DESC"

	rows, err := pc.db.Query(query, predicateName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var examples []PredicateExampleInfo
	for rows.Next() {
		var ex PredicateExampleInfo
		var errorType sql.NullString
		if err := rows.Scan(&ex.ID, &ex.PredicateName, &ex.ExampleCode, &ex.Explanation, &ex.IsCorrect, &errorType, &ex.Source); err != nil {
			return nil, err
		}
		if errorType.Valid {
			ex.ErrorType = errorType.String
		}
		examples = append(examples, ex)
	}
	return examples, nil
}

// GetAntiPatterns retrieves all anti-pattern examples (for repair context).
func (pc *PredicateCorpus) GetAntiPatterns() ([]PredicateExampleInfo, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	rows, err := pc.db.Query(`
		SELECT id, predicate_name, example_code, explanation, is_correct, error_type, source
		FROM predicate_examples WHERE is_correct = 0
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var examples []PredicateExampleInfo
	for rows.Next() {
		var ex PredicateExampleInfo
		var errorType sql.NullString
		if err := rows.Scan(&ex.ID, &ex.PredicateName, &ex.ExampleCode, &ex.Explanation, &ex.IsCorrect, &errorType, &ex.Source); err != nil {
			return nil, err
		}
		if errorType.Valid {
			ex.ErrorType = errorType.String
		}
		examples = append(examples, ex)
	}
	return examples, nil
}

// GetDomains returns all unique domains in the corpus.
func (pc *PredicateCorpus) GetDomains() ([]string, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	rows, err := pc.db.Query("SELECT DISTINCT domain FROM predicate_domains ORDER BY domain")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return nil, err
		}
		domains = append(domains, domain)
	}
	return domains, nil
}

// SearchPredicates searches for predicates by name pattern (SQL LIKE).
func (pc *PredicateCorpus) SearchPredicates(pattern string) ([]PredicateInfo, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	// Convert to SQL LIKE pattern
	if !strings.Contains(pattern, "%") {
		pattern = "%" + pattern + "%"
	}

	rows, err := pc.db.Query(`
		SELECT id, name, arity, type, category, description, safety_level, domain, section, source_file
		FROM predicates WHERE name LIKE ?
		ORDER BY name
		LIMIT 50
	`, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var predicates []PredicateInfo
	for rows.Next() {
		var info PredicateInfo
		if err := rows.Scan(
			&info.ID, &info.Name, &info.Arity, &info.Type, &info.Category,
			&info.Description, &info.SafetyLevel, &info.Domain, &info.Section, &info.SourceFile,
		); err != nil {
			return nil, err
		}
		predicates = append(predicates, info)
	}
	return predicates, nil
}

// Stats returns statistics about the corpus.
func (pc *PredicateCorpus) Stats() (map[string]int, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	stats := make(map[string]int)

	var count int
	if err := pc.db.QueryRow("SELECT COUNT(*) FROM predicates").Scan(&count); err == nil {
		stats["total_predicates"] = count
	}
	if err := pc.db.QueryRow("SELECT COUNT(*) FROM predicates WHERE type = 'EDB'").Scan(&count); err == nil {
		stats["edb_predicates"] = count
	}
	if err := pc.db.QueryRow("SELECT COUNT(*) FROM predicates WHERE type = 'IDB'").Scan(&count); err == nil {
		stats["idb_predicates"] = count
	}
	if err := pc.db.QueryRow("SELECT COUNT(*) FROM error_patterns").Scan(&count); err == nil {
		stats["error_patterns"] = count
	}
	if err := pc.db.QueryRow("SELECT COUNT(*) FROM predicate_examples").Scan(&count); err == nil {
		stats["total_examples"] = count
	}
	if err := pc.db.QueryRow("SELECT COUNT(*) FROM predicate_examples WHERE is_correct = 1").Scan(&count); err == nil {
		stats["correct_examples"] = count
	}
	if err := pc.db.QueryRow("SELECT COUNT(*) FROM predicate_examples WHERE is_correct = 0").Scan(&count); err == nil {
		stats["anti_patterns"] = count
	}

	return stats, nil
}

// FormatPredicateSignature formats a predicate as "name/arity" or "name(arg1, arg2, ...)"
func (pc *PredicateCorpus) FormatPredicateSignature(name string, detailed bool) (string, error) {
	info, err := pc.GetPredicate(name)
	if err != nil || info == nil {
		return name, err
	}

	if !detailed {
		return fmt.Sprintf("%s/%d", name, info.Arity), nil
	}

	args, err := pc.GetPredicateArgs(info.ID)
	if err != nil || len(args) == 0 {
		return fmt.Sprintf("%s/%d", name, info.Arity), nil
	}

	var argStrs []string
	for _, arg := range args {
		argStrs = append(argStrs, fmt.Sprintf("%s:%s", arg.Name, arg.Type))
	}
	return fmt.Sprintf("%s(%s)", name, strings.Join(argStrs, ", ")), nil
}

// GetPriorities returns a map of predicate name to activation priority.
// This is used by ActivationEngine for spreading activation scoring.
// Higher priority = more important for context selection (0-100 scale).
func (pc *PredicateCorpus) GetPriorities() (map[string]int, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	rows, err := pc.db.Query(`
		SELECT name, activation_priority
		FROM predicates
		WHERE activation_priority IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	priorities := make(map[string]int)
	for rows.Next() {
		var name string
		var priority int
		if err := rows.Scan(&name, &priority); err != nil {
			return nil, err
		}
		priorities[name] = priority
	}
	return priorities, nil
}

// GetSerializationOrder returns a map of predicate name to serialization order.
// This is used by FactSerializer to order predicates in output.
// Lower order = earlier in output (1=first).
func (pc *PredicateCorpus) GetSerializationOrder() (map[string]int, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	rows, err := pc.db.Query(`
		SELECT name, serialization_order
		FROM predicates
		WHERE serialization_order IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	order := make(map[string]int)
	for rows.Next() {
		var name string
		var ord int
		if err := rows.Scan(&name, &ord); err != nil {
			return nil, err
		}
		order[name] = ord
	}
	return order, nil
}

// GetPriority returns the activation priority for a single predicate.
// Returns default (50) if predicate not found.
func (pc *PredicateCorpus) GetPriority(name string) int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	var priority int
	err := pc.db.QueryRow(`
		SELECT activation_priority FROM predicates WHERE name = ?
	`, name).Scan(&priority)
	if err != nil {
		return 50 // Default priority
	}
	return priority
}
