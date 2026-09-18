package core

import (
	"context"
	"embed"
	"fmt"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/mangle"
	"codenerd/internal/types"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"codeberg.org/TauCeti/mangle-go/provenance"
)

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
	atomCacheStale    bool                // cachedAtoms were built without Decls (pre-first-eval) or under an older policy; force one reconversion
	factIndex         map[string]struct{} // Canonical fact set for deduplication
	bootFacts         []Fact              // EDB facts extracted from hybrid .mg data sections
	bootIntents       []HybridIntent      // Canonical intents extracted from hybrid .mg files
	bootPrompts       []HybridPrompt      // Prompt atoms extracted from hybrid .mg files
	store             factstore.FactStore
	programInfo       *analysis.ProgramInfo
	strata            []analysis.Nodeset       // Cached stratification for EvalStratifiedProgramWithStats
	predToStratum     map[ast.PredicateSym]int // Cached predicate-to-stratum mapping
	schemas           string
	policy            string
	learned           string              // Learned rules (autopoiesis) - loaded from learned.mg
	loadedPolicyFiles map[string]struct{} // Idempotency: policy modules loaded via LoadPolicyFile (keyed by case-insensitive basename)

	// sandbox marks a throwaway kernel used to trial-compile a candidate rule
	// (validateRuleSandbox). A parse or analysis failure there is an expected
	// outcome the caller handles — the Legislator repairs the rule and retries,
	// and usually succeeds on attempt 2 — not a production fault.
	//
	// Without this, every rejected candidate logged two ERROR lines
	// ("rebuildProgram: parse failed", "HotLoadRule: rule rejected by sandbox
	// compiler") indistinguishable from the real corpus failing to parse. They
	// were the loudest entries in the kernel log after `nerd logs` was fixed,
	// and cost a full investigation before the Legislator's own log revealed
	// "Rule auto-repaired by feedback loop sanitizer ... hot-loaded
	// successfully" seconds later. Zero value is false, so every other kernel
	// keeps logging these at ERROR.
	sandbox bool

	// undeclared tracks predicates already reported by warnIfUndeclaredLocked,
	// so each is named once per process instead of on every assert.
	undeclared undeclaredWarner

	schemaValidator *mangle.SchemaValidator
	initialized     bool
	manglePath      string      // Path to mangle files directory
	workspaceRoot   string      // Explicit workspace root (for .nerd paths)
	policyDirty     bool        // True when schemas/policy changed and need reparse
	factsDirty      atomic.Bool // True when EDB facts changed and need re-evaluation (lazy eval).
	// factsDirty is atomic so Query/QueryCallback/QueryAll can fast-path without
	// holding the kernel mutex when no facts have changed since the last evaluate.
	// Use ensureEvaluated() (kernel_eval.go) to drive lazy re-eval safely.
	evalSingleflight    sync.Mutex             // serializes lazy evaluate() so only one goroutine evaluates per dirty epoch
	userLearnedPath     string                 // Path to user learned.mg for self-healing persistence
	predicateCorpus     *PredicateCorpus       // Baked-in predicate corpus for validation
	predicateCorpusOnce sync.Once              // Lazily opens the embedded SQLite corpus on first consumer
	repairInterceptor   LearnedRuleInterceptor // Optional interceptor for rule repair before persistence
	virtualStore        *VirtualStore          // Virtual predicate source for query_* handlers
	derivedFactLimit    int                    // Configurable limit for derived facts (0 = use default)
	maxFacts            int                    // Hard limit for EDB facts (0 = use default 250000)
	simulateCommitErr   error                  // TEST ONLY: Simulates a transaction commit failure
	eventBus            *FactEventBus          // Pub/sub for fact mutations — replaces polling in system shards

	// proofRecorder, when non-nil, captures derivation events during evaluate()
	// so Explain() can answer "why was this fact derived?" via the Codeberg
	// mangle-go fork's provenance package. Off by default — enable via
	// EnableProvenance(). The recorder is reset at the start of every
	// evaluate() to bound memory usage.
	proofRecorder *provenance.MemoryRecorder

	lastEvaluation EvaluationStats
}

// EvaluationStats describes the cost of the most recent evaluate().
type EvaluationStats struct {
	InputFacts int
	Duration   time.Duration
}

func (k *RealKernel) LastEvaluation() EvaluationStats {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.lastEvaluation
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
// Path should be relative to defaults/ (e.g. "schemas.mg" or "schema/intent_queries.mg").
func GetDefaultContent(path string) (string, error) {
	data, err := coreLogic.ReadFile("defaults/" + path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// defaultSchemaFiles lists the modular schema files loaded after
// defaults/schemas.mg, in load order. It is shared by loadMangleFiles
// (kernel_init.go) and DefaultCorpusText so the schema inventory cannot drift
// between kernel boot and static analysis.
var defaultSchemaFiles = []string{
	"schemas_intent.mg",           // Intent & Focus Resolution
	"schemas_world.mg",            // File Topology, Symbol Graph, Diagnostics
	"schemas_execution.mg",        // TDD Loop & Action Execution
	"schemas_browser.mg",          // Browser Physics & Spatial Reasoning
	"schemas_project.mg",          // Project Profile, User Preferences, Session State
	"schemas_dreamer.mg",          // Speculative Dreamer & Cross-Module Support
	"schemas_memory.mg",           // Memory Tiers & Knowledge
	"schemas_knowledge.mg",        // Knowledge Atoms, LSP, Semantic Matching
	"schemas_learning.mg",         // Learned exemplars + intent overrides
	"schemas_state.mg",            // Ouroboros State Machine
	"chaos.mg",                    // Adversarial Testing (PanicMaker, Nemesis)
	"schemas_safety.mg",           // Constitution, Git Safety, Shadow Mode
	"schemas_analysis.mg",         // Spreading Activation, Strategy, Impact
	"schemas_misc.mg",             // Northstar, Continuation Protocol, Benchmarks
	"schemas_codedom.mg",          // Code DOM & Interactive Elements
	"schemas_codedom_polyglot.mg", // Polyglot Language Facts (Go, Python, TS, Rust)
	"schemas_testing.mg",          // Verification, Reasoning Traces, Pytest
	"schemas_campaign.mg",         // Campaign Orchestration
	"schemas_intelligence.mg",     // Campaign Intelligence & Context
	"schemas_tools.mg",            // Ouroboros, Tool Learning, Routing
	"schemas_mcp.mg",              // MCP integration schema
	"schemas_prompts.mg",          // Dynamic Prompt Composition & JIT
	"schemas_reviewer.mg",         // Static Analysis & Data Flow
	"schemas_shards.mg",           // Shard Delegation & Coordination
	"schemas_coder.mg",            // Coder Shard Declarations
	"schemas_projectdoc.mg",       // nerd.md project instructions (see internal/projectdoc)
	// NERD-EVOLVE-START: context_compilation_schemas_c1_c4
	"schemas_context.mg", // Context Compilation Pipeline (C1+C4)
	// NERD-EVOLVE-END: context_compilation_schemas_c1_c4
}

// DefaultCorpusText returns the concatenated embedded default corpus: the
// schema index plus every file in defaultSchemaFiles (the same list and order
// loadMangleFiles boots from), and the policy inventory from
// DefaultPolicyFiles (defaults/policy/*.mg in sorted order followed by
// DefaultCorePolicyModules).
//
// Sections are separated by "# Schema Module: <name>" / "# Policy Module:
// <name>" markers mirroring loadMangleFiles, plus markers for the root policy
// modules (which loadMangleFiles appends unmarked) so static analysis in
// BuildDerivationMap can attribute every clause to its file. Markers are
// Mangle comments, so the text stays semantically identical to the boot
// corpus. User extensions, northstar vision, and learned rules are excluded:
// they are runtime data, not the static default corpus.
//
// An error is returned when any listed constitution file cannot be read,
// which indicates a corrupt binary. Missing files are never skipped: a
// silently dropped schema module removes Decls and a dropped policy module
// removes rules, and both failure modes surface only as empty derivations
// far downstream.
func DefaultCorpusText() (schemas string, policy string, err error) {
	var schemasBuilder strings.Builder
	data, rerr := readRequiredEmbeddedFile(coreLogic, "defaults/schemas.mg")
	if rerr != nil {
		return "", "", rerr
	}
	schemasBuilder.Write(data)
	for _, schemaFile := range defaultSchemaFiles {
		data, rerr := readRequiredEmbeddedFile(coreLogic, "defaults/"+schemaFile)
		if rerr != nil {
			return "", "", rerr
		}
		schemasBuilder.WriteString("\n\n# Schema Module: ")
		schemasBuilder.WriteString(schemaFile)
		schemasBuilder.WriteString("\n")
		schemasBuilder.Write(data)
	}

	policyFiles, perr := DefaultPolicyFiles()
	if perr != nil {
		return "", "", perr
	}
	if len(policyFiles) == 0 {
		return "", "", fmt.Errorf("default corpus: embedded policy inventory is empty")
	}
	var policyBuilder strings.Builder
	for _, file := range policyFiles {
		data, rerr := readRequiredEmbeddedFile(coreLogic, "defaults/"+file)
		if rerr != nil {
			return "", "", rerr
		}
		policyBuilder.WriteString("\n\n# Policy Module: ")
		policyBuilder.WriteString(path.Base(file))
		policyBuilder.WriteString("\n")
		policyBuilder.Write(data)
	}
	return schemasBuilder.String(), policyBuilder.String(), nil
}
