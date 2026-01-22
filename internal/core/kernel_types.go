package core

import (
	"context"
	"embed"
	"sync"

	"codenerd/internal/mangle"
	"codenerd/internal/types"

	"github.com/google/mangle/analysis"
	"github.com/google/mangle/ast"
	"github.com/google/mangle/factstore"
)

// =============================================================================
// TYPE ALIASES - Import from internal/types to break import cycles
// =============================================================================
// These types are defined in internal/types and aliased here for backward compatibility.
// This breaks the core → autopoiesis → articulation → core import cycle.

// Fact represents a single logical fact (atom) in the EDB.
// Aliased from types package to break import cycles.
type Fact = types.Fact

// MangleAtom represents a Mangle name constant (starting with /).
// Aliased from types package to break import cycles.
type MangleAtom = types.MangleAtom

// Kernel is an alias to types.Kernel for backward compatibility.
type Kernel = types.Kernel

// LearnedRuleInterceptor intercepts learned rules before persistence.
// This allows the MangleRepairShard to validate and repair rules without import cycles.
type LearnedRuleInterceptor interface {
	// InterceptLearnedRule validates and optionally repairs a rule before persistence.
	// Returns the (possibly repaired) rule, or an error if the rule is rejected.
	InterceptLearnedRule(ctx context.Context, rule string) (string, error)
}

// RealKernel wraps the google/mangle engine with proper EDB/IDB separation.
type RealKernel struct {
	mu                sync.RWMutex
	facts             []Fact
	cachedAtoms       []ast.Atom          // OPTIMIZATION: Cached Mangle atoms to avoid O(N) ToAtom() conversions
	factIndex         map[string]struct{} // Canonical fact set for deduplication
	bootFacts         []Fact              // EDB facts extracted from hybrid .mg data sections
	bootIntents       []HybridIntent      // Canonical intents extracted from hybrid .mg files
	bootPrompts       []HybridPrompt      // Prompt atoms extracted from hybrid .mg files
	store             factstore.FactStore
	programInfo       *analysis.ProgramInfo
	schemas           string
	policy            string
	learned           string              // Learned rules (autopoiesis) - loaded from learned.mg
	loadedPolicyFiles map[string]struct{} // Idempotency: policy modules loaded via LoadPolicyFile (keyed by case-insensitive basename)
	schemaValidator   *mangle.SchemaValidator
	initialized       bool
	manglePath        string                 // Path to mangle files directory
	workspaceRoot     string                 // Explicit workspace root (for .nerd paths)
	policyDirty       bool                   // True when schemas/policy changed and need reparse
	userLearnedPath   string                 // Path to user learned.mg for self-healing persistence
	predicateCorpus   *PredicateCorpus       // Baked-in predicate corpus for validation
	repairInterceptor LearnedRuleInterceptor // Optional interceptor for rule repair before persistence
	virtualStore      *VirtualStore          // Virtual predicate source for query_* handlers
	derivedFactLimit  int                    // Configurable limit for derived facts (0 = use default)
}

// StartupValidationResult contains statistics from startup learned rule validation.
type StartupValidationResult struct {
	TotalRules        int
	ValidRules        int
	InvalidRules      int
	CommentedRules    int // Previously self-healed rules
	PreviouslyHealed  int // Rules with "# SELF-HEALED:" marker
	FilePath          string
	InvalidRuleErrors []string
}

// learnedValidationResult performs startup validation of learned rules.
// Returns validation statistics and optionally the healed text.
type learnedValidationResult struct {
	stats      StartupValidationResult
	healedText string
}

//go:embed defaults/*.mg defaults/schema/*.mg defaults/policy/*.mg
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
