package core

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/mangle/feedback"
	"codenerd/internal/types"

	"github.com/google/mangle/analysis"
	"github.com/google/mangle/ast"
	_ "github.com/google/mangle/builtin"
	"github.com/google/mangle/engine"
	"github.com/google/mangle/factstore"
	"github.com/google/mangle/parse"
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

// Kernel defines the interface for the logic core.
type Kernel interface {
	LoadFacts(facts []Fact) error
	Query(predicate string) ([]Fact, error)
	QueryAll() (map[string][]Fact, error)
	Assert(fact Fact) error
	Retract(predicate string) error
	RetractFact(fact Fact) error // Retract a specific fact by predicate and first argument
	UpdateSystemFacts() error    // Updates system-level facts (time, OS, etc.)
}

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
	factIndex         map[string]struct{} // Canonical fact set for deduplication
	bootFacts         []Fact              // EDB facts extracted from hybrid .mg data sections
	bootIntents       []HybridIntent      // Canonical intents extracted from hybrid .mg files
	bootPrompts       []HybridPrompt      // Prompt atoms extracted from hybrid .mg files
	store             factstore.FactStore
	programInfo       *analysis.ProgramInfo
	schemas           string
	policy            string
	learned           string // Learned rules (autopoiesis) - loaded from learned.mg
	schemaValidator   *mangle.SchemaValidator
	initialized       bool
	manglePath        string                 // Path to mangle files directory
	workspaceRoot     string                 // Explicit workspace root (for .nerd paths)
	policyDirty       bool                   // True when schemas/policy changed and need reparse
	userLearnedPath   string                 // Path to user learned.mg for self-healing persistence
	predicateCorpus   *PredicateCorpus       // Baked-in predicate corpus for validation
	repairInterceptor LearnedRuleInterceptor // Optional interceptor for rule repair before persistence
}

//go:embed defaults/*.mg defaults/schema/*.mg
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

// NewRealKernel creates a new kernel instance.
// Returns an error if the embedded constitution fails to compile (e.g., corrupted binary).
func NewRealKernel() (*RealKernel, error) {
	timer := logging.StartTimer(logging.CategoryKernel, "NewRealKernel")
	logging.Kernel("Initializing new RealKernel instance")

	k := &RealKernel{
		facts:       make([]Fact, 0),
		factIndex:   make(map[string]struct{}),
		bootFacts:   make([]Fact, 0),
		bootIntents: make([]HybridIntent, 0),
		bootPrompts: make([]HybridPrompt, 0),
		store:       factstore.NewSimpleInMemoryStore(),
		policyDirty: true, // Need to parse on first use
	}
	logging.KernelDebug("Kernel struct created, store initialized, policyDirty=true")

	// Find and load mangle files from the project
	if err := k.loadMangleFiles(); err != nil {
		timer.Stop()
		return nil, fmt.Errorf("failed to load mangle files: %w", err)
	}

	// Load the baked-in predicate corpus for validation
	k.loadPredicateCorpus()

	// Inject any EDB facts extracted from hybrid .mg files before first evaluation.
	if len(k.bootFacts) > 0 {
		k.facts = append(k.facts, k.bootFacts...)
	}
	k.rebuildFactIndexLocked()

	// Force initial evaluation to boot the Mangle engine.
	// The embedded core MUST compile, otherwise the binary is corrupt.
	logging.Kernel("Booting Mangle engine with embedded constitution...")
	if err := k.evaluate(); err != nil {
		logging.Get(logging.CategoryKernel).Error("CRITICAL: Kernel boot failed: %v", err)
		timer.Stop()
		return nil, fmt.Errorf("kernel failed to boot embedded constitution: %w", err)
	}

	timer.StopWithInfo()
	logging.Kernel("Kernel initialized successfully")
	return k, nil
}

// NewRealKernelWithPath creates a kernel with explicit mangle path.
// Returns an error if the kernel fails to boot.
func NewRealKernelWithPath(manglePath string) (*RealKernel, error) {
	timer := logging.StartTimer(logging.CategoryKernel, "NewRealKernelWithPath")
	logging.Kernel("Initializing RealKernel with explicit path: %s", manglePath)

	k := &RealKernel{
		facts:       make([]Fact, 0),
		factIndex:   make(map[string]struct{}),
		bootFacts:   make([]Fact, 0),
		bootIntents: make([]HybridIntent, 0),
		bootPrompts: make([]HybridPrompt, 0),
		store:       factstore.NewSimpleInMemoryStore(),
		manglePath:  manglePath,
		policyDirty: true,
	}
	logging.KernelDebug("Kernel struct created with manglePath=%s", manglePath)

	if err := k.loadMangleFiles(); err != nil {
		timer.Stop()
		return nil, fmt.Errorf("failed to load mangle files: %w", err)
	}

	// Load the baked-in predicate corpus for validation
	k.loadPredicateCorpus()

	// Inject any EDB facts extracted from hybrid .mg files before first evaluation.
	if len(k.bootFacts) > 0 {
		k.facts = append(k.facts, k.bootFacts...)
	}
	k.rebuildFactIndexLocked()

	// Force initial evaluation
	logging.Kernel("Booting Mangle engine...")
	if err := k.evaluate(); err != nil {
		logging.Get(logging.CategoryKernel).Error("CRITICAL: Kernel boot failed (path: %s): %v", manglePath, err)
		timer.Stop()
		return nil, fmt.Errorf("kernel failed to boot (path: %s): %w", manglePath, err)
	}

	timer.StopWithInfo()
	logging.Kernel("Kernel with path initialized successfully")
	return k, nil
}

// SetWorkspace sets the explicit workspace root path for .nerd directory resolution.
// This MUST be called after kernel creation to ensure .nerd paths resolve correctly.
// If not set, paths will be resolved relative to CWD (which may be incorrect).
func (k *RealKernel) SetWorkspace(root string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.workspaceRoot = root
}

// GetWorkspace returns the workspace root, or empty string if not set.
func (k *RealKernel) GetWorkspace() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.workspaceRoot
}

// SetRepairInterceptor sets the repair interceptor for learned rule validation.
// The interceptor is called before any learned rule is persisted, allowing
// the MangleRepairShard to validate and repair rules.
func (k *RealKernel) SetRepairInterceptor(interceptor LearnedRuleInterceptor) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.repairInterceptor = interceptor
	if interceptor != nil {
		logging.Kernel("Repair interceptor attached to kernel")
	}
}

// GetRepairInterceptor returns the current repair interceptor, or nil if not set.
func (k *RealKernel) GetRepairInterceptor() LearnedRuleInterceptor {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.repairInterceptor
}

// nerdPath returns the correct path for a .nerd subdirectory.
// Uses workspaceRoot if set, otherwise returns relative path (legacy behavior).
func (k *RealKernel) nerdPath(subpath string) string {
	if k.workspaceRoot != "" {
		return filepath.Join(k.workspaceRoot, ".nerd", subpath)
	}
	return filepath.Join(".nerd", subpath)
}

// loadMangleFiles loads schemas and policy from the embedded core and user extensions.
// Returns an error if critical embedded files cannot be loaded.
func (k *RealKernel) loadMangleFiles() error {
	timer := logging.StartTimer(logging.CategoryKernel, "loadMangleFiles")
	logging.Kernel("Loading Mangle files (schemas, policy, learned rules)")

	// 1. LOAD BAKED-IN CORE (Immutable Physics)
	// Always load these. They are the "Constitution".
	logging.KernelDebug("Loading baked-in core (Constitution)...")

	// Load Core Schemas
	if data, err := coreLogic.ReadFile("defaults/schemas.mg"); err == nil {
		k.schemas = string(data)
		logging.KernelDebug("Loaded core schemas (%d bytes)", len(data))
	} else {
		logging.Get(logging.CategoryKernel).Error("Failed to load core schemas: %v", err)
	}

	// Load JIT Prompt Schema (System 2)
	if data, err := coreLogic.ReadFile("defaults/schema/prompts.mg"); err == nil {
		k.schemas += "\n\n" + string(data)
		logging.KernelDebug("Loaded JIT prompt schema (%d bytes)", len(data))
	} else {
		logging.KernelDebug("JIT prompt schema not found (optional)")
	}

	// Load Core Policy
	if data, err := coreLogic.ReadFile("defaults/policy.mg"); err == nil {
		k.policy = string(data)
		logging.KernelDebug("Loaded core policy (%d bytes)", len(data))
	} else {
		logging.Get(logging.CategoryKernel).Error("Failed to load core policy: %v", err)
	}

	// Load other core modules into policy
	coreModules := []string{
		"doc_taxonomy.mg",
		"topology_planner.mg",
		"build_topology.mg",
		"campaign_rules.mg",
		"selection_policy.mg",
		"taxonomy.mg",
		"inference.mg",
		"jit_compiler.mg", // System 2 JIT Gatekeeper
	}

	loadedModules := 0
	for _, mod := range coreModules {
		if data, err := coreLogic.ReadFile("defaults/" + mod); err == nil {
			k.policy += "\n\n" + string(data)
			loadedModules++
			logging.KernelDebug("Loaded core module: %s (%d bytes)", mod, len(data))
		} else {
			logging.KernelDebug("Core module not found (optional): %s", mod)
		}
	}
	logging.KernelDebug("Loaded %d/%d core modules", loadedModules, len(coreModules))

	// Load base learned rules (if any)
	if data, err := coreLogic.ReadFile("defaults/learned.mg"); err == nil {
		k.learned = string(data)
		logging.KernelDebug("Loaded base learned rules (%d bytes)", len(data))
	} else {
		logging.KernelDebug("No base learned rules found (this is normal for fresh installs)")
	}

	// 2. LOAD USER EXTENSIONS (Project Specifics)
	// Look in the workspace's .nerd folder or explicit manglePath
	logging.KernelDebug("Loading user extensions...")
	workspacePaths := []string{
		k.nerdPath("mangle"),
		k.manglePath,
	}

	userExtensionsLoaded := 0
	for _, wsPath := range workspacePaths {
		if wsPath == "" {
			continue
		}
		logging.KernelDebug("Checking user extension path: %s", wsPath)

		// Append User Schemas (extensions.mg)
		extPath := filepath.Join(wsPath, "extensions.mg")
		if _, err := os.Stat(extPath); err == nil {
			if res, err := LoadHybridMangleFile(extPath); err == nil {
				k.schemas += "\n\n# User Extensions\n" + res.Logic
				k.bootFacts = append(k.bootFacts, res.Facts...)
				k.bootIntents = append(k.bootIntents, res.Intents...)
				k.bootPrompts = append(k.bootPrompts, res.Prompts...)
				userExtensionsLoaded++
				logging.Kernel("Loaded user schema extensions from %s (%d bytes, %d data facts, %d intents, %d prompts)", extPath, len(res.Logic), len(res.Facts), len(res.Intents), len(res.Prompts))
			} else {
				logging.Get(logging.CategoryKernel).Warn("Failed to load hybrid extensions from %s: %v", extPath, err)
			}
		}

		// Append User Policy (policy_overrides.mg)
		policyPath := filepath.Join(wsPath, "policy_overrides.mg")
		if _, err := os.Stat(policyPath); err == nil {
			if res, err := LoadHybridMangleFile(policyPath); err == nil {
				k.policy += "\n\n# User Policy Overrides\n" + res.Logic
				k.bootFacts = append(k.bootFacts, res.Facts...)
				k.bootIntents = append(k.bootIntents, res.Intents...)
				k.bootPrompts = append(k.bootPrompts, res.Prompts...)
				userExtensionsLoaded++
				logging.Kernel("Loaded user policy overrides from %s (%d bytes, %d data facts, %d intents, %d prompts)", policyPath, len(res.Logic), len(res.Facts), len(res.Intents), len(res.Prompts))
			} else {
				logging.Get(logging.CategoryKernel).Warn("Failed to load hybrid policy overrides from %s: %v", policyPath, err)
			}
		}

		// Append User Learned Rules (learned.mg)
		learnedPath := filepath.Join(wsPath, "learned.mg")
		if _, err := os.Stat(learnedPath); err == nil {
			res, err := LoadHybridMangleFile(learnedPath)
			if err != nil {
				logging.Get(logging.CategoryKernel).Warn("Failed to load hybrid learned rules from %s: %v", learnedPath, err)
				continue
			}
			userLearnedContent := res.Logic
			k.bootFacts = append(k.bootFacts, res.Facts...)
			k.bootIntents = append(k.bootIntents, res.Intents...)
			k.bootPrompts = append(k.bootPrompts, res.Prompts...)
			userExtensionsLoaded++
			logging.Kernel("Loaded user learned rules from %s (%d bytes, %d data facts, %d intents, %d prompts)", learnedPath, len(userLearnedContent), len(res.Facts), len(res.Intents), len(res.Prompts))

			// Track path and content for self-healing
			k.userLearnedPath = learnedPath

			// Initialize schema validator early so we can heal user rules before appending
			if k.schemas != "" && k.schemaValidator == nil {
				k.schemaValidator = mangle.NewSchemaValidator(k.schemas, k.learned+"\n"+userLearnedContent)
				if err := k.schemaValidator.LoadDeclaredPredicates(); err != nil {
					logging.Get(logging.CategoryKernel).Warn("Failed to load schema validator: %v", err)
				}
			}

			// Self-heal user learned rules BEFORE appending to k.learned
			if k.schemaValidator != nil {
				userLearnedContent = k.healLearnedRules(userLearnedContent, learnedPath)
			}

			// Append healed user rules to base learned rules
			k.learned += "\n\n# User Learned Rules\n" + userLearnedContent
		}
	}
	logging.KernelDebug("Loaded %d user extension files", userExtensionsLoaded)

	// Ensure schema validator is initialized (if not done above)
	if k.schemas != "" && k.schemaValidator == nil {
		logging.KernelDebug("Initializing schema validator...")
		k.schemaValidator = mangle.NewSchemaValidator(k.schemas, k.learned)
		if err := k.schemaValidator.LoadDeclaredPredicates(); err != nil {
			logging.Get(logging.CategoryKernel).Warn("Failed to load schema validator: %v", err)
		} else {
			logging.KernelDebug("Schema validator initialized successfully")
		}
	}

	timer.Stop()
	logging.Kernel("Mangle files loaded: schemas=%d bytes, policy=%d bytes, learned=%d bytes",
		len(k.schemas), len(k.policy), len(k.learned))
	return nil
}

// loadPredicateCorpus loads the baked-in predicate corpus for validation.
func (k *RealKernel) loadPredicateCorpus() {
	timer := logging.StartTimer(logging.CategoryKernel, "loadPredicateCorpus")
	logging.Kernel("Loading baked-in predicate corpus...")

	corpus, err := NewPredicateCorpus()
	if err != nil {
		logging.Get(logging.CategoryKernel).Warn("Predicate corpus not available: %v", err)
		timer.Stop()
		return
	}

	k.predicateCorpus = corpus
	if stats, err := corpus.Stats(); err == nil {
		logging.Kernel("Predicate corpus loaded: %d predicates, %d examples, %d error patterns",
			stats["total_predicates"], stats["examples"], stats["error_patterns"])
	} else {
		logging.Kernel("Predicate corpus loaded (stats unavailable: %v)", err)
	}
	timer.Stop()
}

// GetPredicateCorpus returns the baked-in predicate corpus (may be nil if not loaded).
func (k *RealKernel) GetPredicateCorpus() *PredicateCorpus {
	return k.predicateCorpus
}

// ConsumeBootPrompts returns any PROMPT directives extracted during boot
// and clears the internal buffer to avoid double-ingest.
func (k *RealKernel) ConsumeBootPrompts() []HybridPrompt {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.bootPrompts) == 0 {
		return nil
	}
	out := make([]HybridPrompt, len(k.bootPrompts))
	copy(out, k.bootPrompts)
	k.bootPrompts = nil
	return out
}

// ConsumeBootIntents returns any INTENT directives extracted during boot
// and clears the internal buffer.
func (k *RealKernel) ConsumeBootIntents() []HybridIntent {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.bootIntents) == 0 {
		return nil
	}
	out := make([]HybridIntent, len(k.bootIntents))
	copy(out, k.bootIntents)
	k.bootIntents = nil
	return out
}

// LoadFacts adds facts to the EDB and rebuilds the program.
func (k *RealKernel) LoadFacts(facts []Fact) error {
	timer := logging.StartTimer(logging.CategoryKernel, "LoadFacts")
	logging.Kernel("LoadFacts: loading %d facts into EDB", len(facts))

	k.mu.Lock()
	defer k.mu.Unlock()

	prevCount := len(k.facts)
	sanitizedFacts := make([]Fact, len(facts))
	for i, f := range facts {
		sanitizedFacts[i] = sanitizeFactForNumericPredicates(f)
	}
	added := 0
	for _, f := range sanitizedFacts {
		if k.addFactIfNewLocked(f) {
			added++
		}
	}
	logging.KernelDebug("LoadFacts: added %d/%d facts, EDB: %d -> %d facts", added, len(sanitizedFacts), prevCount, len(k.facts))

	// Count JIT-related facts for debugging
	jitCounts := make(map[string]int)
	for _, f := range sanitizedFacts {
		switch f.Predicate {
		case "is_mandatory", "atom_tag", "atom_priority", "current_context", "atom":
			jitCounts[f.Predicate]++
		}
	}
	if len(jitCounts) > 0 {
		logging.Kernel("LoadFacts JIT: is_mandatory=%d atom_tag=%d atom_priority=%d current_context=%d atom=%d",
			jitCounts["is_mandatory"], jitCounts["atom_tag"], jitCounts["atom_priority"],
			jitCounts["current_context"], jitCounts["atom"])
	}

	// Log sample of facts being loaded (first 5)
	if len(sanitizedFacts) > 0 && logging.IsDebugMode() {
		sampleSize := 5
		if len(sanitizedFacts) < sampleSize {
			sampleSize = len(sanitizedFacts)
		}
		for i := 0; i < sampleSize; i++ {
			logging.KernelDebug("  [%d] %s", i, sanitizedFacts[i].String())
		}
		if len(sanitizedFacts) > sampleSize {
			logging.KernelDebug("  ... and %d more facts", len(sanitizedFacts)-sampleSize)
		}
	}

	// If nothing new was added, skip rebuild.
	if added == 0 {
		timer.Stop()
		return nil
	}

	err := k.rebuild()
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("LoadFacts: rebuild failed: %v", err)
		return err
	}

	timer.Stop()
	return nil
}

// =============================================================================
// Fact Deduplication Helpers
// =============================================================================

// canonFact returns a stable string key for a fact.
// We use Fact.String() which is already canonical for Mangle serialization.
func (k *RealKernel) canonFact(f Fact) string {
	return f.String()
}

// ensureFactIndexLocked initializes the fact index if needed.
// Call only while holding k.mu.
func (k *RealKernel) ensureFactIndexLocked() {
	if k.factIndex == nil {
		k.rebuildFactIndexLocked()
	}
}

// rebuildFactIndexLocked rebuilds the dedupe index from current EDB.
// Call only while holding k.mu.
func (k *RealKernel) rebuildFactIndexLocked() {
	k.factIndex = make(map[string]struct{}, len(k.facts))
	for _, f := range k.facts {
		k.factIndex[k.canonFact(f)] = struct{}{}
	}
}

// addFactIfNewLocked appends a fact only if it is not already present.
// Returns true if added. Call only while holding k.mu.
func (k *RealKernel) addFactIfNewLocked(f Fact) bool {
	k.ensureFactIndexLocked()
	key := k.canonFact(f)
	if _, ok := k.factIndex[key]; ok {
		return false
	}
	k.facts = append(k.facts, f)
	k.factIndex[key] = struct{}{}
	return true
}

// Assert adds a single fact dynamically and re-evaluates derived facts.
func (k *RealKernel) Assert(fact Fact) error {
	logging.KernelDebug("Assert: %s", fact.String())
	logging.Audit().KernelAssert(fact.Predicate, len(fact.Args))

	k.mu.Lock()
	defer k.mu.Unlock()

	fact = sanitizeFactForNumericPredicates(fact)
	if !k.addFactIfNewLocked(fact) {
		logging.KernelDebug("Assert: duplicate fact skipped: %s", fact.String())
		return nil
	}
	if err := k.evaluate(); err != nil {
		logging.Get(logging.CategoryKernel).Error("Assert: evaluation failed after asserting %s: %v", fact.Predicate, err)
		return err
	}
	logging.KernelDebug("Assert: fact added successfully, total facts=%d", len(k.facts))
	return nil
}

// AssertString parses a Mangle fact string and asserts it.
// Format: predicate(arg1, arg2, ...) where args can be:
//   - Name constants: /foo, /bar
//   - Strings: "quoted text"
//   - Numbers: 42, 3.14
func (k *RealKernel) AssertString(factStr string) error {
	fact, err := ParseFactString(factStr)
	if err != nil {
		return fmt.Errorf("AssertString: failed to parse %q: %w", factStr, err)
	}
	return k.Assert(fact)
}

// AssertBatch adds multiple facts and evaluates once (more efficient).
func (k *RealKernel) AssertBatch(facts []Fact) error {
	timer := logging.StartTimer(logging.CategoryKernel, "AssertBatch")
	logging.KernelDebug("AssertBatch: adding %d facts", len(facts))

	k.mu.Lock()
	defer k.mu.Unlock()

	prevCount := len(k.facts)
	sanitized := make([]Fact, len(facts))
	for i, f := range facts {
		sanitized[i] = sanitizeFactForNumericPredicates(f)
	}
	added := 0
	for _, f := range sanitized {
		if k.addFactIfNewLocked(f) {
			added++
		}
	}
	if added == 0 {
		timer.Stop()
		logging.KernelDebug("AssertBatch: no new facts added (all duplicates)")
		return nil
	}

	if err := k.evaluate(); err != nil {
		logging.Get(logging.CategoryKernel).Error("AssertBatch: evaluation failed after adding %d facts: %v", len(facts), err)
		return err
	}

	timer.Stop()
	logging.KernelDebug("AssertBatch: added %d/%d facts, EDB: %d -> %d facts", added, len(facts), prevCount, len(k.facts))
	return nil
}

// AssertWithoutEval adds a fact without re-evaluating.
// Use when batching many facts, then call Evaluate() once at the end.
func (k *RealKernel) AssertWithoutEval(fact Fact) {
	logging.KernelDebug("AssertWithoutEval: %s (deferred evaluation)", fact.Predicate)
	k.mu.Lock()
	defer k.mu.Unlock()
	fact = sanitizeFactForNumericPredicates(fact)
	if !k.addFactIfNewLocked(fact) {
		logging.KernelDebug("AssertWithoutEval: duplicate fact skipped: %s", fact.String())
	}
}

// Evaluate forces re-evaluation of all rules. Call after AssertWithoutEval batch.
func (k *RealKernel) Evaluate() error {
	timer := logging.StartTimer(logging.CategoryKernel, "Evaluate")
	logging.KernelDebug("Evaluate: forcing re-evaluation of all rules")

	k.mu.Lock()
	defer k.mu.Unlock()

	err := k.evaluate()
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("Evaluate: failed: %v", err)
		return err
	}

	timer.Stop()
	return nil
}

// Retract removes all facts of a given predicate.
func (k *RealKernel) Retract(predicate string) error {
	logging.KernelDebug("Retract: removing all facts with predicate=%s", predicate)

	k.mu.Lock()
	defer k.mu.Unlock()

	prevCount := len(k.facts)
	retractedCount := 0
	newLen := 0
	for _, f := range k.facts {
		if f.Predicate != predicate {
			k.facts[newLen] = f
			newLen++
		} else {
			retractedCount++
		}
	}

	if retractedCount == 0 {
		logging.KernelDebug("Retract: no facts found for predicate=%s (EDB unchanged at %d facts)", predicate, prevCount)
		return nil
	}

	// Zero tail to release references for GC.
	for i := newLen; i < prevCount; i++ {
		k.facts[i] = Fact{}
	}
	k.facts = k.facts[:newLen]
	k.rebuildFactIndexLocked()

	logging.KernelDebug("Retract: removed %d facts (predicate=%s), EDB: %d -> %d facts",
		retractedCount, predicate, prevCount, len(k.facts))

	if err := k.rebuild(); err != nil {
		logging.Get(logging.CategoryKernel).Error("Retract: rebuild failed after retracting %s: %v", predicate, err)
		return err
	}
	return nil
}

// RetractFact removes a specific fact by matching predicate and first argument.
// This enables selective fact removal (e.g., removing all facts for a specific tool).
func (k *RealKernel) RetractFact(fact Fact) error {
	logging.KernelDebug("RetractFact: removing fact matching predicate=%s, firstArg=%v", fact.Predicate, fact.Args)

	k.mu.Lock()
	defer k.mu.Unlock()

	if len(fact.Args) == 0 {
		err := fmt.Errorf("fact must have at least one argument for matching")
		logging.Get(logging.CategoryKernel).Error("RetractFact: %v", err)
		return err
	}

	prevCount := len(k.facts)
	retractedCount := 0
	newLen := 0
	for _, f := range k.facts {
		// Keep facts that don't match predicate OR don't match first argument
		if f.Predicate != fact.Predicate {
			k.facts[newLen] = f
			newLen++
			continue
		}
		// Same predicate - check first argument
		if len(f.Args) > 0 && len(fact.Args) > 0 {
			if !argsEqual(f.Args[0], fact.Args[0]) {
				k.facts[newLen] = f
				newLen++
			} else {
				retractedCount++
			}
			// Matching predicate and first arg - don't add (retract it)
		} else {
			k.facts[newLen] = f
			newLen++
		}
	}

	if retractedCount == 0 {
		logging.KernelDebug("RetractFact: no matching facts found (predicate=%s firstArg=%v)", fact.Predicate, fact.Args[0])
		return nil
	}

	// Zero tail to release references for GC.
	for i := newLen; i < prevCount; i++ {
		k.facts[i] = Fact{}
	}
	k.facts = k.facts[:newLen]
	k.rebuildFactIndexLocked()

	logging.KernelDebug("RetractFact: removed %d facts, EDB: %d -> %d facts",
		retractedCount, prevCount, len(k.facts))

	if err := k.rebuild(); err != nil {
		logging.Get(logging.CategoryKernel).Error("RetractFact: rebuild failed: %v", err)
		return err
	}
	return nil
}

// RetractExactFact removes facts that exactly match predicate and all arguments.
// This is safer for multi-arity predicates where multiple facts may share a first arg.
// It does NOT replace RetractFact, which intentionally matches only the first arg.
func (k *RealKernel) RetractExactFact(fact Fact) error {
	logging.KernelDebug("RetractExactFact: removing exact fact predicate=%s args=%v", fact.Predicate, fact.Args)

	k.mu.Lock()
	defer k.mu.Unlock()

	if len(fact.Args) == 0 {
		err := fmt.Errorf("fact must have at least one argument for exact matching")
		logging.Get(logging.CategoryKernel).Error("RetractExactFact: %v", err)
		return err
	}

	prevCount := len(k.facts)
	filtered := make([]Fact, 0, prevCount)
	retractedCount := 0
	for _, f := range k.facts {
		if f.Predicate != fact.Predicate || !argsSliceEqual(f.Args, fact.Args) {
			filtered = append(filtered, f)
			continue
		}
		retractedCount++
	}
	k.facts = filtered
	if retractedCount > 0 {
		k.rebuildFactIndexLocked()
	}

	logging.KernelDebug("RetractExactFact: removed %d facts, EDB: %d -> %d facts",
		retractedCount, prevCount, len(k.facts))

	// Only rebuild if something changed
	if retractedCount > 0 {
		if err := k.rebuild(); err != nil {
			logging.Get(logging.CategoryKernel).Error("RetractExactFact: rebuild failed: %v", err)
			return err
		}
	}
	return nil
}

// RetractExactFactsBatch removes a batch of exact facts and rebuilds once.
// Useful for incremental world model updates on large repos.
func (k *RealKernel) RetractExactFactsBatch(facts []Fact) error {
	if len(facts) == 0 {
		return nil
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	canon := func(f Fact) string {
		argsJSON, _ := json.Marshal(f.Args)
		return f.Predicate + "|" + string(argsJSON)
	}

	toRemove := make(map[string]struct{}, len(facts))
	for _, f := range facts {
		toRemove[canon(f)] = struct{}{}
	}

	prevCount := len(k.facts)
	filtered := make([]Fact, 0, prevCount)
	retractedCount := 0
	for _, f := range k.facts {
		if _, ok := toRemove[canon(f)]; ok {
			retractedCount++
			continue
		}
		filtered = append(filtered, f)
	}
	k.facts = filtered
	if retractedCount > 0 {
		k.rebuildFactIndexLocked()
	}

	logging.KernelDebug("RetractExactFactsBatch: removed %d facts, EDB: %d -> %d facts",
		retractedCount, prevCount, len(k.facts))

	if retractedCount > 0 {
		if err := k.rebuild(); err != nil {
			logging.Get(logging.CategoryKernel).Error("RetractExactFactsBatch: rebuild failed: %v", err)
			return err
		}
	}
	return nil
}

// RemoveFactsByPredicateSet removes all facts whose predicate is in the given set.
// Rebuilds once if anything was removed.
func (k *RealKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	if len(predicates) == 0 {
		return nil
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	prevCount := len(k.facts)
	filtered := make([]Fact, 0, prevCount)
	retractedCount := 0
	for _, f := range k.facts {
		if _, ok := predicates[f.Predicate]; ok {
			retractedCount++
			continue
		}
		filtered = append(filtered, f)
	}
	k.facts = filtered
	if retractedCount > 0 {
		k.rebuildFactIndexLocked()
	}

	logging.KernelDebug("RemoveFactsByPredicateSet: removed %d facts, EDB: %d -> %d facts",
		retractedCount, prevCount, len(k.facts))

	if retractedCount > 0 {
		if err := k.rebuild(); err != nil {
			logging.Get(logging.CategoryKernel).Error("RemoveFactsByPredicateSet: rebuild failed: %v", err)
			return err
		}
	}
	return nil
}

// argsEqual compares two fact arguments for equality.
func argsEqual(a, b interface{}) bool {
	switch av := a.(type) {
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
	case int64:
		if bv, ok := b.(int64); ok {
			return av == bv
		}
	case float64:
		if bv, ok := b.(float64); ok {
			return av == bv
		}
	case bool:
		if bv, ok := b.(bool); ok {
			return av == bv
		}
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// argsSliceEqual compares two argument slices for full equality.
func argsSliceEqual(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !argsEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

// sanitizeFactForNumericPredicates coerces common priority atoms to numbers
// for predicates that participate in numeric comparisons.
// This prevents evaluation failures like "value /high is not a number" when
// LLMs emit priority atoms in numeric slots.
func sanitizeFactForNumericPredicates(f Fact) Fact {
	switch f.Predicate {
	case "agenda_item":
		// agenda_item(ItemID, Description, Priority, Status, Timestamp)
		if len(f.Args) > 2 {
			f.Args[2] = coercePriorityAtomToNumber(f.Args[2])
		}
	case "prompt_atom":
		// prompt_atom(AtomID, Category, Priority, TokenCount, IsMandatory)
		if len(f.Args) > 2 {
			f.Args[2] = coercePriorityAtomToNumber(f.Args[2])
		}
	case "atom_priority":
		// atom_priority(AtomID, Priority)
		if len(f.Args) > 1 {
			f.Args[1] = coercePriorityAtomToNumber(f.Args[1])
		}
	}
	return f
}

func coercePriorityAtomToNumber(v interface{}) interface{} {
	switch t := v.(type) {
	case string:
		atom := strings.TrimSpace(t)
		if atom == "" {
			return v
		}
		// Accept both "/high" and "high"
		trimmed := strings.TrimPrefix(atom, "/")
		switch trimmed {
		case "critical":
			return int64(100)
		case "high":
			return int64(80)
		case "medium", "normal":
			return int64(50)
		case "low":
			return int64(25)
		case "lowest":
			return int64(10)
		default:
			return v
		}
	default:
		return v
	}
}

// rebuildProgram parses schemas+policy and caches programInfo.
// This is only called when policyDirty is true.
func (k *RealKernel) rebuildProgram() error {
	timer := logging.StartTimer(logging.CategoryKernel, "rebuildProgram")
	logging.Kernel("Rebuilding Mangle program (parsing schemas+policy+learned)")

	// Construct program from schemas + policy + learned (no facts)
	// STRATIFIED TRUST: Load order ensures Constitution has priority
	var sb strings.Builder

	if k.schemas != "" {
		sb.WriteString(k.schemas)
		sb.WriteString("\n")
		logging.KernelDebug("rebuildProgram: included schemas (%d bytes)", len(k.schemas))
	}

	if k.policy != "" {
		sb.WriteString(k.policy)
		sb.WriteString("\n")
		logging.KernelDebug("rebuildProgram: included policy (%d bytes)", len(k.policy))
	}

	// Load learned rules AFTER constitution (stratified trust)
	if k.learned != "" {
		sb.WriteString("# Learned Rules (Autopoiesis Layer - Stratified Trust)\n")
		sb.WriteString(k.learned)
		logging.KernelDebug("rebuildProgram: included learned rules (%d bytes)", len(k.learned))
	}

	programStr := sb.String()
	logging.KernelDebug("rebuildProgram: total program size = %d bytes", len(programStr))

	// Parse
	parseTimer := logging.StartTimer(logging.CategoryKernel, "rebuildProgram.parse")
	parsed, err := parse.Unit(strings.NewReader(programStr))
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("rebuildProgram: parse failed: %v", err)
		return fmt.Errorf("failed to parse program: %w", err)
	}
	parseTimer.Stop()
	logging.KernelDebug("rebuildProgram: parsed %d clauses", len(parsed.Clauses))

	// Analyze
	analyzeTimer := logging.StartTimer(logging.CategoryKernel, "rebuildProgram.analyze")
	programInfo, err := analysis.AnalyzeOneUnit(parsed, nil)
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("rebuildProgram: analysis failed: %v", err)
		// DEBUG: Dump program when analysis fails
		_ = os.WriteFile("debug_program_ERROR.mg", []byte(programStr), 0644)
		logging.KernelDebug("Dumped failed program to debug_program_ERROR.mg")
		return fmt.Errorf("failed to analyze program: %w", err)
	}
	analyzeTimer.Stop()

	k.programInfo = programInfo
	k.policyDirty = false

	// Log predicate count
	declCount := 0
	if programInfo.Decls != nil {
		declCount = len(programInfo.Decls)
	}
	logging.KernelDebug("rebuildProgram: analysis complete, %d predicates declared", declCount)

	timer.StopWithInfo()
	logging.Kernel("Mangle program rebuilt successfully")
	return nil
}

// evaluate populates the store with facts and evaluates to fixpoint.
// Uses cached programInfo for efficiency.
func (k *RealKernel) evaluate() error {
	timer := logging.StartTimer(logging.CategoryKernel, "evaluate")

	// Rebuild program if policy changed
	if k.policyDirty || k.programInfo == nil {
		logging.KernelDebug("evaluate: policy dirty or programInfo nil, rebuilding program")
		if err := k.rebuildProgram(); err != nil {
			return err
		}
	} else {
		logging.KernelDebug("evaluate: using cached programInfo")
	}

	// Create fresh store and populate with EDB facts
	logging.KernelDebug("evaluate: populating store with %d EDB facts", len(k.facts))
	k.store = factstore.NewSimpleInMemoryStore()
	factConversionErrors := 0
	for _, f := range k.facts {
		atom, err := f.ToAtom()
		if err != nil {
			factConversionErrors++
			logging.Get(logging.CategoryKernel).Error("evaluate: failed to convert fact %s: %v", f.Predicate, err)
			return fmt.Errorf("failed to convert fact %v: %w", f, err)
		}
		k.store.Add(atom)
	}

	// Evaluate to fixpoint using cached programInfo
	// BUG #17 FIX: Add gas limits to prevent halting problem in learned rules
	// Prevent fact explosions from recursive learned rules
	const derivedFactLimit = 500000
	logging.KernelDebug("evaluate: running fixpoint evaluation (derivedFactLimit=%d)", derivedFactLimit)

	evalTimer := logging.StartTimer(logging.CategoryKernel, "evaluate.fixpoint")
	stats, err := engine.EvalProgramWithStats(k.programInfo, k.store,
		engine.WithCreatedFactLimit(derivedFactLimit)) // Hard cap: max 500K derived facts
	evalDuration := evalTimer.Stop()

	if err != nil {
		logging.Get(logging.CategoryKernel).Error("evaluate: fixpoint evaluation failed: %v", err)
		// Check if this is a derived fact limit error
		if strings.Contains(err.Error(), "limit") || strings.Contains(err.Error(), "exceeded") {
			logging.Get(logging.CategoryKernel).Warn("evaluate: POSSIBLE FACT EXPLOSION - derived facts exceeded %d limit", derivedFactLimit)
		}
		return fmt.Errorf("failed to evaluate program: %w", err)
	}

	// Log evaluation stats
	totalDuration := time.Duration(0)
	for _, d := range stats.Duration {
		totalDuration += d
	}
	strataCount := len(stats.Strata)
	logging.KernelDebug("evaluate: fixpoint reached - strata=%d, evalTime=%v, wallTime=%v",
		strataCount, totalDuration, evalDuration)

	k.initialized = true
	timer.Stop()
	logging.KernelDebug("evaluate: complete, kernel initialized")
	return nil
}

// rebuild is kept for backward compatibility - now delegates to evaluate().
func (k *RealKernel) rebuild() error {
	logging.KernelDebug("rebuild: delegating to evaluate()")
	return k.evaluate()
}

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
		logging.KernelDebug("Query: predicate '%s' not found in declarations", predicateName)
	}

	// JIT-related predicate debugging - log at INFO level for visibility
	jitPredicates := map[string]bool{
		"selected_result": true, "is_mandatory": true, "mandatory_selection": true,
		"blocked_by_context": true, "final_valid": true, "tentative": true,
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
	return reflect.DeepEqual(normalizeQueryValue(pattern), normalizeQueryValue(value))
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

// GetStore returns the underlying FactStore for advanced operations.
func (k *RealKernel) GetStore() factstore.FactStore {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.store
}

// SetSchemas allows loading custom schemas (for testing or shard isolation).
func (k *RealKernel) SetSchemas(schemas string) {
	logging.KernelDebug("SetSchemas: loading custom schemas (%d bytes)", len(schemas))
	k.mu.Lock()
	defer k.mu.Unlock()
	k.schemas = schemas
	k.policyDirty = true
	logging.KernelDebug("SetSchemas: policyDirty set to true, will rebuild on next evaluate")
}

// =============================================================================
// SCHEMA VALIDATION (Bug #18 Fix - Schema Drift Prevention)
// =============================================================================

// ValidateLearnedRule validates that a learned rule only uses declared predicates.
// This prevents "Schema Drift" where the agent invents predicates with no data source.
func (k *RealKernel) ValidateLearnedRule(ruleText string) error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		// Validator not initialized - allow (defensive)
		return nil
	}

	return k.schemaValidator.ValidateRule(ruleText)
}

// ValidateLearnedRules validates multiple learned rules.
// Returns a list of errors (one per invalid rule).
func (k *RealKernel) ValidateLearnedRules(rules []string) []error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return nil
	}

	return k.schemaValidator.ValidateRules(rules)
}

// ValidateLearnedProgram validates an entire learned program text.
func (k *RealKernel) ValidateLearnedProgram(programText string) error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return nil
	}

	return k.schemaValidator.ValidateProgram(programText)
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

// healLearnedRules validates learned rules and comments out invalid ones.
// This is a self-healing mechanism to recover from corrupted learned.mg files.
// Returns the healed rules text with invalid rules commented out.
// If filePath is provided and rules were healed, persists the healed version to disk.
func (k *RealKernel) healLearnedRules(learnedText string, filePath string) string {
	result := k.validateLearnedRulesContent(learnedText, filePath, true)
	return result.healedText
}

// validateLearnedRulesContent performs startup validation of learned rules.
// Returns validation statistics and optionally the healed text.
type learnedValidationResult struct {
	stats      StartupValidationResult
	healedText string
}

func (k *RealKernel) validateLearnedRulesContent(learnedText string, filePath string, heal bool) learnedValidationResult {
	result := learnedValidationResult{
		stats: StartupValidationResult{
			FilePath: filePath,
		},
		healedText: learnedText,
	}

	if k.schemaValidator == nil || learnedText == "" {
		return result
	}

	lines := strings.Split(learnedText, "\n")
	var healedLines []string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			healedLines = append(healedLines, line)
			continue
		}

		// Track previously self-healed rules
		if strings.HasPrefix(trimmed, "# SELF-HEALED:") {
			result.stats.PreviouslyHealed++
			healedLines = append(healedLines, line)
			continue
		}

		// Track commented-out rules (potential previous self-healing)
		if strings.HasPrefix(trimmed, "#") {
			// Check if this is a commented-out rule (starts with # and contains :-)
			commentContent := strings.TrimPrefix(trimmed, "#")
			commentContent = strings.TrimSpace(commentContent)
			if strings.Contains(commentContent, ":-") && !strings.HasPrefix(commentContent, "SELF-HEALED") {
				result.stats.CommentedRules++
			}
			healedLines = append(healedLines, line)
			continue
		}

		// Check if this is a rule (contains :-) or a fact (no :-)
		isRule := strings.Contains(trimmed, ":-")
		isFact := !isRule && strings.Contains(trimmed, "(") && strings.HasSuffix(trimmed, ").")

		if isRule || isFact {
			result.stats.TotalRules++

			// Schema validation for rules with bodies
			if isRule {
				if err := k.schemaValidator.ValidateRule(trimmed); err != nil {
					result.stats.InvalidRules++
					errMsg := fmt.Sprintf("line %d: %v", i+1, err)
					result.stats.InvalidRuleErrors = append(result.stats.InvalidRuleErrors, errMsg)
					logging.Get(logging.CategoryKernel).Warn("Startup validation: invalid learned rule at %s", errMsg)

					if heal {
						healedLines = append(healedLines, "# SELF-HEALED: "+err.Error())
						healedLines = append(healedLines, "# "+line)
					} else {
						healedLines = append(healedLines, line)
					}
					continue
				}
			}

			// Infinite loop risk detection for next_action rules
			if loopErr := k.checkInfiniteLoopRisk(trimmed); loopErr != "" {
				result.stats.InvalidRules++
				errMsg := fmt.Sprintf("line %d: %s", i+1, loopErr)
				result.stats.InvalidRuleErrors = append(result.stats.InvalidRuleErrors, errMsg)
				logging.Get(logging.CategoryKernel).Warn("Startup validation: %s", errMsg)

				if heal {
					healedLines = append(healedLines, "# SELF-HEALED: "+loopErr)
					healedLines = append(healedLines, "# "+line)
				} else {
					healedLines = append(healedLines, line)
				}
				continue
			}

			result.stats.ValidRules++
		}

		// Valid line - keep as is
		healedLines = append(healedLines, line)
	}

	result.healedText = strings.Join(healedLines, "\n")

	// Log validation summary
	if result.stats.TotalRules > 0 {
		logging.Kernel("Startup validation: %d rules total, %d valid, %d invalid, %d previously healed",
			result.stats.TotalRules, result.stats.ValidRules, result.stats.InvalidRules, result.stats.PreviouslyHealed)
	}

	if result.stats.InvalidRules > 0 && heal {
		logging.Kernel("Self-healing: commented out %d invalid learned rules", result.stats.InvalidRules)

		// Persist healed rules back to disk if we have a file path
		if filePath != "" {
			if err := os.WriteFile(filePath, []byte(result.healedText), 0644); err != nil {
				logging.Get(logging.CategoryKernel).Error("Self-healing: failed to persist healed rules to %s: %v", filePath, err)
			} else {
				logging.Kernel("Self-healing: persisted healed rules to %s", filePath)
			}
		}
	}

	if result.stats.PreviouslyHealed > 0 {
		logging.Get(logging.CategoryKernel).Warn("Startup validation: %d rules were previously self-healed (may indicate recurring issues)", result.stats.PreviouslyHealed)
	}

	return result
}

// checkInfiniteLoopRisk detects rules that could cause infinite derivation loops.
// Returns an error message if the rule is problematic, empty string if OK.
func (k *RealKernel) checkInfiniteLoopRisk(rule string) string {
	// Skip comments
	trimmed := strings.TrimSpace(rule)
	if strings.HasPrefix(trimmed, "#") {
		return ""
	}

	// Only check next_action rules
	if !strings.Contains(rule, "next_action(") {
		return ""
	}

	// Parse head and body
	parts := strings.SplitN(rule, ":-", 2)
	head := strings.TrimSpace(parts[0])

	// Check 1: Unconditional next_action fact (no body) for system actions
	if len(parts) == 1 && strings.HasPrefix(head, "next_action(") {
		if strings.Contains(head, "/system_start") || strings.Contains(head, "/initialize") {
			return "infinite loop risk: unconditional next_action for system action will fire every tick"
		}
	}

	// Check 2: next_action depending on always-true system predicates
	if len(parts) == 2 {
		body := strings.TrimSpace(parts[1])

		// Predicates that are always true at system startup
		alwaysTruePredicates := []string{
			"system_startup(", "system_shard_state(", "entry_point(",
			"current_phase(", "build_system(",
		}

		for _, pred := range alwaysTruePredicates {
			if strings.Contains(body, pred) {
				// Check for wildcards which make it always-true
				if strings.Contains(body, "_,_)") || strings.Contains(body, "(_,") || strings.Contains(body, ",_)") {
					// Count predicates - if only 1-2, it's likely always-true
					predCount := strings.Count(body, "(")
					if predCount <= 2 {
						return fmt.Sprintf("infinite loop risk: next_action depends on always-true predicate %s with wildcards", strings.TrimSuffix(pred, "("))
					}
				}
			}
		}
	}

	return ""
}

// GetStartupValidationResult returns the result of the last startup validation.
// This can be called after kernel initialization to check learned rule health.
func (k *RealKernel) GetStartupValidationResult() *StartupValidationResult {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.userLearnedPath == "" {
		return nil
	}

	// Re-validate current learned rules (read-only, no healing)
	data, err := os.ReadFile(k.userLearnedPath)
	if err != nil {
		return nil
	}

	result := k.validateLearnedRulesContent(string(data), k.userLearnedPath, false)
	return &result.stats
}

// IsPredicateDeclared checks if a predicate is declared in schemas.
func (k *RealKernel) IsPredicateDeclared(predicate string) bool {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return false
	}

	return k.schemaValidator.IsDeclared(predicate)
}

// GetDeclaredPredicates returns all declared predicate signatures.
// Each signature is in the format "predicate_name/arity" (e.g., "user_intent/5").
// This method satisfies the feedback.RuleValidator interface.
func (k *RealKernel) GetDeclaredPredicates() []string {
	k.mu.RLock()
	defer k.mu.RUnlock()

	// Prefer programInfo.Decls for accurate arity information
	if k.programInfo != nil && k.programInfo.Decls != nil {
		signatures := make([]string, 0, len(k.programInfo.Decls))
		for predSym := range k.programInfo.Decls {
			signatures = append(signatures, fmt.Sprintf("%s/%d", predSym.Symbol, predSym.Arity))
		}
		return signatures
	}

	// Fallback to schema validator (names only, no arity)
	if k.schemaValidator == nil {
		return nil
	}

	return k.schemaValidator.GetDeclaredPredicates()
}

// SetPolicy allows loading custom policy rules (for shard specialization).
func (k *RealKernel) SetPolicy(policy string) {
	logging.KernelDebug("SetPolicy: loading custom policy (%d bytes)", len(policy))
	k.mu.Lock()
	defer k.mu.Unlock()
	k.policy = policy
	k.policyDirty = true
	logging.KernelDebug("SetPolicy: policyDirty set to true")
}

// AppendPolicy appends additional policy rules (for shard-specific policies).
func (k *RealKernel) AppendPolicy(additionalPolicy string) {
	logging.KernelDebug("AppendPolicy: appending %d bytes to existing policy", len(additionalPolicy))
	k.mu.Lock()
	defer k.mu.Unlock()
	prevLen := len(k.policy)
	k.policy = k.policy + "\n\n# Appended Policy\n" + additionalPolicy
	k.policyDirty = true
	logging.KernelDebug("AppendPolicy: policy grew from %d to %d bytes, policyDirty=true", prevLen, len(k.policy))
}

// LoadPolicyFile loads policy rules from a file and appends them.
func (k *RealKernel) LoadPolicyFile(path string) error {
	logging.KernelDebug("LoadPolicyFile: attempting to load %s", path)
	baseName := filepath.Base(path)

	// 1. Try Embedded Core first
	if data, err := coreLogic.ReadFile("defaults/" + baseName); err == nil {
		logging.Kernel("LoadPolicyFile: loaded from embedded core: %s (%d bytes)", baseName, len(data))
		k.AppendPolicy(string(data))
		return nil
	}

	// 2. Try User Workspace (.nerd/mangle)
	userPath := filepath.Join(k.nerdPath("mangle"), baseName)
	if data, err := os.ReadFile(userPath); err == nil {
		logging.Kernel("LoadPolicyFile: loaded from user workspace: %s (%d bytes)", userPath, len(data))
		k.AppendPolicy(string(data))
		return nil
	}

	// 3. Try explicitly provided path
	if data, err := os.ReadFile(path); err == nil {
		logging.Kernel("LoadPolicyFile: loaded from explicit path: %s (%d bytes)", path, len(data))
		k.AppendPolicy(string(data))
		return nil
	}

	// 4. Try legacy search paths (fallback for existing behavior)
	searchPaths := []string{
		filepath.Join("internal/mangle", baseName),
		filepath.Join("../internal/mangle", baseName),
		filepath.Join("../../internal/mangle", baseName),
	}

	for _, p := range searchPaths {
		data, err := os.ReadFile(p)
		if err == nil {
			logging.Kernel("LoadPolicyFile: loaded from legacy path: %s (%d bytes)", p, len(data))
			k.AppendPolicy(string(data))
			return nil
		}
	}

	logging.Get(logging.CategoryKernel).Error("LoadPolicyFile: policy file not found: %s", path)
	return fmt.Errorf("policy file not found: %s", path)
}

// HotLoadRule dynamically loads a single Mangle rule at runtime.
// This is used by Autopoiesis to add new rules without restarting.
// FIX for Bug #8 (Suicide Rule): Uses a "Sandbox Compiler" to validate the rule
// before accepting it, preventing invalid rules from bricking the kernel.
func (k *RealKernel) HotLoadRule(rule string) error {
	timer := logging.StartTimer(logging.CategoryKernel, "HotLoadRule")
	logging.Kernel("HotLoadRule: attempting to load rule (%d bytes)", len(rule))

	k.mu.Lock()
	defer k.mu.Unlock()

	if rule == "" {
		err := fmt.Errorf("empty rule")
		logging.Get(logging.CategoryKernel).Error("HotLoadRule: %v", err)
		return err
	}

	// Log the rule being loaded (truncated for readability)
	rulePreview := rule
	if len(rulePreview) > 100 {
		rulePreview = rulePreview[:100] + "..."
	}
	logging.KernelDebug("HotLoadRule: rule preview: %s", rulePreview)

	// 1. Create a Sandbox Kernel (Memory only)
	logging.KernelDebug("HotLoadRule: creating sandbox kernel for validation")
	sandbox := &RealKernel{
		store:       factstore.NewSimpleInMemoryStore(),
		policyDirty: true,
	}

	// 2. Load CURRENT schemas and policy into sandbox
	sandbox.schemas = k.schemas
	sandbox.policy = k.policy

	// 3. Apply the NEW rule to the sandbox
	sandbox.policy = sandbox.policy + "\n\n# Sandbox Validation\n" + rule

	// 4. Try to compile (rebuildProgram)
	// This will fail with StratificationError if the rule creates a paradox
	logging.KernelDebug("HotLoadRule: validating rule in sandbox...")
	if err := sandbox.rebuildProgram(); err != nil {
		logging.Get(logging.CategoryKernel).Error("HotLoadRule: rule rejected by sandbox compiler: %v", err)
		return fmt.Errorf("rule rejected by sandbox compiler: %w", err)
	}
	logging.KernelDebug("HotLoadRule: sandbox validation passed")

	// 5. If successful, apply to Real Kernel (in-memory)
	k.learned = k.learned + "\n\n# HotLoaded Rule\n" + rule
	k.policyDirty = true

	timer.StopWithInfo()
	logging.Kernel("HotLoadRule: rule loaded successfully, policyDirty=true")
	return nil
}

// GenerateValidatedRule uses an LLM to generate a Mangle rule and validates it
// through the feedback loop system. This method implements the neuro-symbolic
// pattern: LLM creativity + deterministic validation.
//
// Parameters:
//   - ctx: Context for cancellation and timeout propagation
//   - llmClient: Client that implements feedback.LLMClient interface
//   - purpose: Description of what the rule should accomplish
//   - contextMap: Additional context for rule generation (injected into prompt)
//   - domain: Rule domain for example selection (e.g., "executive", "action", "selection")
//
// Returns the validated rule string or an error if generation/validation fails.
func (k *RealKernel) GenerateValidatedRule(
	ctx context.Context,
	llmClient feedback.LLMClient,
	purpose string,
	contextMap map[string]string,
	domain string,
) (string, error) {
	timer := logging.StartTimer(logging.CategoryKernel, "GenerateValidatedRule")
	logging.Kernel("GenerateValidatedRule: generating rule for purpose=%q domain=%q", purpose, domain)

	if purpose == "" {
		return "", fmt.Errorf("purpose cannot be empty")
	}

	if llmClient == nil {
		return "", fmt.Errorf("llmClient cannot be nil")
	}

	// Build the system prompt with Mangle syntax guidance
	predicates := k.GetDeclaredPredicates()
	systemPrompt := feedback.BuildEnhancedSystemPrompt(mangleRuleSystemPrompt, predicates)

	// Build user prompt from purpose and context
	var userPromptBuilder strings.Builder
	userPromptBuilder.WriteString("Generate a Mangle rule for the following purpose:\n\n")
	userPromptBuilder.WriteString(purpose)

	if len(contextMap) > 0 {
		userPromptBuilder.WriteString("\n\n## Additional Context:\n")
		for key, value := range contextMap {
			userPromptBuilder.WriteString(fmt.Sprintf("- %s: %s\n", key, value))
		}
	}

	userPrompt := userPromptBuilder.String()
	logging.KernelDebug("GenerateValidatedRule: user prompt length=%d, available predicates=%d",
		len(userPrompt), len(predicates))

	// Create feedback loop with default config
	feedbackLoop := feedback.NewFeedbackLoop(feedback.DefaultConfig())

	// Run generation with validation
	result, err := feedbackLoop.GenerateAndValidate(
		ctx,
		llmClient,
		k, // RealKernel implements RuleValidator via HotLoadRule and GetDeclaredPredicates
		systemPrompt,
		userPrompt,
		domain,
	)
	if err != nil {
		attempts := 0
		if result != nil {
			attempts = result.Attempts
		}
		logging.Get(logging.CategoryKernel).Error("GenerateValidatedRule: generation failed after %d attempts: %v",
			attempts, err)
		return "", fmt.Errorf("rule generation failed: %w", err)
	}

	if result == nil || !result.Valid {
		errorCount := 0
		if result != nil {
			errorCount = len(result.Errors)
		}
		logging.Get(logging.CategoryKernel).Error("GenerateValidatedRule: validation failed, errors=%d",
			errorCount)
		return "", fmt.Errorf("generated rule failed validation")
	}

	timer.StopWithInfo()
	logging.Kernel("GenerateValidatedRule: success after %d attempts, autoFixed=%v",
		result.Attempts, result.AutoFixed)

	return result.Rule, nil
}

// mangleRuleSystemPrompt is the base system prompt for rule generation.
const mangleRuleSystemPrompt = `You are an expert Mangle/Datalog programmer for codeNERD, a neuro-symbolic coding agent.

Your task is to generate syntactically correct Mangle rules that will compile successfully.

## Critical Mangle Syntax Rules:
1. Variables are UPPERCASE: X, Y, File, Action
2. Name constants start with /: /fix, /review, /coder
3. Strings are double-quoted: "hello world"
4. Every rule MUST end with a period (.)
5. Rule format: head(Args) :- body1(Args), body2(Args).
6. Negation uses !predicate(X), NOT \+ or not
7. Aggregation: Result = fn:count(X) |> do predicate(X).

## Common Mistakes to Avoid:
- Using lowercase variables (wrong: x, correct: X)
- Missing the terminating period
- Using strings instead of atoms for constants
- Prolog-style negation (\+) instead of !

Output ONLY the rule, no explanation. The rule must compile.`

// HotLoadLearnedRule dynamically loads a learned rule and persists it to learned.mg.
// This is the primary method for Autopoiesis to add new learned rules.
// It validates the rule, loads it into memory, and writes it to disk for persistence.
func (k *RealKernel) HotLoadLearnedRule(rule string) error {
	logging.Kernel("HotLoadLearnedRule: loading and persisting learned rule")

	// 0. If repair interceptor is set, use it for validation and repair FIRST
	// This allows MangleRepairShard to fix rules before we even try to load them
	k.mu.RLock()
	interceptor := k.repairInterceptor
	k.mu.RUnlock()

	if interceptor != nil {
		logging.Kernel("HotLoadLearnedRule: invoking repair interceptor")
		ctx := context.Background()
		repairedRule, err := interceptor.InterceptLearnedRule(ctx, rule)
		if err != nil {
			logging.Get(logging.CategoryKernel).Error("HotLoadLearnedRule: repair interceptor rejected rule: %v", err)
			return fmt.Errorf("rule rejected by repair interceptor: %w", err)
		}
		if repairedRule != rule {
			logging.Kernel("HotLoadLearnedRule: rule was repaired by interceptor")
			rule = repairedRule
		}
	}

	// 1. Validate using sandbox (same as HotLoadRule)
	if err := k.HotLoadRule(rule); err != nil {
		logging.Get(logging.CategoryKernel).Error("HotLoadLearnedRule: validation failed: %v", err)
		return err
	}

	// 1b. Schema validation - ensure all predicates in rule body are declared
	// This prevents "Schema Drift" where rules use hallucinated predicates
	if err := k.ValidateLearnedRule(rule); err != nil {
		logging.Get(logging.CategoryKernel).Error("HotLoadLearnedRule: schema validation failed: %v", err)
		return fmt.Errorf("rule uses undeclared predicates: %w", err)
	}
	logging.KernelDebug("HotLoadLearnedRule: schema validation passed")

	// 2. Persist to learned.mg file
	if err := k.appendToLearnedFile(rule); err != nil {
		logging.Get(logging.CategoryKernel).Error("HotLoadLearnedRule: failed to persist rule: %v", err)
		return err
	}

	logging.Kernel("HotLoadLearnedRule: rule loaded and persisted successfully")
	return nil
}

// appendToLearnedFile appends a rule to learned.mg on disk.
func (k *RealKernel) appendToLearnedFile(rule string) error {
	logging.KernelDebug("appendToLearnedFile: persisting rule to disk")

	// Determine workspace path for persistence
	// Priority: explicit manglePath > workspace-based .nerd/mangle > relative .nerd/mangle
	targetDir := k.nerdPath("mangle")
	if k.manglePath != "" {
		targetDir = k.manglePath
	}
	logging.KernelDebug("appendToLearnedFile: target directory: %s", targetDir)

	// Ensure directory exists
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		logging.Get(logging.CategoryKernel).Error("appendToLearnedFile: failed to create directory: %v", err)
		return fmt.Errorf("failed to create directory for learned rules: %w", err)
	}

	learnedPath := filepath.Join(targetDir, "learned.mg")

	// Append rule to file
	f, err := os.OpenFile(learnedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("appendToLearnedFile: failed to open learned.mg: %v", err)
		return fmt.Errorf("failed to open learned.mg: %w", err)
	}
	defer f.Close()

	// Write rule with proper formatting
	_, err = f.WriteString(fmt.Sprintf("\n# Autopoiesis-learned rule (added %s)\n%s\n",
		time.Now().Format("2006-01-02 15:04:05"), rule))
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("appendToLearnedFile: failed to write: %v", err)
		return fmt.Errorf("failed to write to learned.mg: %w", err)
	}

	logging.Kernel("appendToLearnedFile: rule persisted to %s", learnedPath)
	return nil
}

// GetLearned returns the current learned rules.
func (k *RealKernel) GetLearned() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.learned
}

// SetLearned allows loading custom learned rules (for testing).
func (k *RealKernel) SetLearned(learned string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.learned = learned
	k.policyDirty = true
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

// GetFactsSnapshot returns a copy of currently asserted facts.
func (k *RealKernel) GetFactsSnapshot() []Fact {
	k.mu.RLock()
	defer k.mu.RUnlock()
	facts := make([]Fact, len(k.facts))
	copy(facts, k.facts)
	return facts
}

// Clone creates a new kernel with the same schemas, policy, learned rules, and facts.
// Optimized to avoid disk I/O and re-parsing by sharing the immutable programInfo.
func (k *RealKernel) Clone() *RealKernel {
	timer := logging.StartTimer(logging.CategoryKernel, "Clone")
	logging.KernelDebug("Clone: creating kernel clone")

	k.mu.RLock()
	defer k.mu.RUnlock()

	// Create bare struct without triggering loadMangleFiles
	clone := &RealKernel{
		facts:           make([]Fact, len(k.facts)),
		factIndex:       make(map[string]struct{}),
		store:           factstore.NewSimpleInMemoryStore(),
		schemas:         k.schemas,
		policy:          k.policy,
		learned:         k.learned,
		manglePath:      k.manglePath,
		workspaceRoot:   k.workspaceRoot,   // Preserve workspace for .nerd paths
		programInfo:     k.programInfo,     // Share immutable analysis
		schemaValidator: k.schemaValidator, // Share immutable validator
		policyDirty:     k.policyDirty,     // Inherit dirty state (likely false)
		initialized:     false,             // Will initialize on Evaluate
	}

	// copy(clone.facts, k.facts) - simpler to just re-assert if we want independence
	// But for performance, deep copy the slice
	copy(clone.facts, k.facts)
	clone.rebuildFactIndexLocked()

	// Note: We do NOT define a shared ViewLayer here because Mangle needs
	// a unified store for fixpoint. Fast copying of the slice is reasonably cheap
	// (12GB RAM budget allows for this). The main win is skipping Parse/Analyze.

	timer.Stop()
	logging.KernelDebug("Clone: created clone with %d facts, shared programInfo", len(clone.facts))
	return clone
}

// Clear resets the kernel to empty state (keeps cached programInfo).
func (k *RealKernel) Clear() {
	logging.Kernel("Clear: resetting kernel to empty state (preserving programInfo)")
	k.mu.Lock()
	defer k.mu.Unlock()
	prevFactCount := len(k.facts)
	k.facts = make([]Fact, 0)
	k.rebuildFactIndexLocked()
	k.store = factstore.NewSimpleInMemoryStore()
	k.initialized = false
	// Note: programInfo and policyDirty preserved - only facts cleared
	logging.KernelDebug("Clear: cleared %d facts, programInfo preserved", prevFactCount)
}

// Reset fully resets the kernel including cached program.
func (k *RealKernel) Reset() {
	logging.Kernel("Reset: fully resetting kernel (including programInfo)")
	k.mu.Lock()
	defer k.mu.Unlock()
	prevFactCount := len(k.facts)
	k.facts = make([]Fact, 0)
	k.rebuildFactIndexLocked()
	k.store = factstore.NewSimpleInMemoryStore()
	k.programInfo = nil
	k.policyDirty = true
	k.initialized = false
	logging.KernelDebug("Reset: cleared %d facts, programInfo cleared, policyDirty=true", prevFactCount)
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

// GetAllFacts returns a copy of all facts in the kernel.
// SAFE FOR PERSISTENCE: This returns only the EDB (Base Facts) explicitly loaded.
// It does NOT return derived facts (IDB). Use this for saving state (Fix for Bug #9).
func (k *RealKernel) GetAllFacts() []Fact {
	k.mu.RLock()
	defer k.mu.RUnlock()
	result := make([]Fact, len(k.facts))
	copy(result, k.facts)
	return result
}

// GetDerivedFacts returns all facts derived by rules (IDB).
// WARNING: Do NOT persist these. They should be re-derived on boot.
func (k *RealKernel) GetDerivedFacts() (map[string][]Fact, error) {
	return k.QueryAll()
}

// LoadFactsFromFile loads facts from a .mg file into the kernel.
// This parses the file and extracts EDB facts to load.
func (k *RealKernel) LoadFactsFromFile(path string) error {
	timer := logging.StartTimer(logging.CategoryKernel, "LoadFactsFromFile")
	logging.Kernel("LoadFactsFromFile: loading facts from %s", path)

	data, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("LoadFactsFromFile: failed to read file: %v", err)
		return fmt.Errorf("failed to read facts file: %w", err)
	}
	logging.KernelDebug("LoadFactsFromFile: read %d bytes from %s", len(data), path)

	// Parse the facts from the file content
	facts, err := ParseFactsFromString(string(data))
	if err != nil {
		logging.Get(logging.CategoryKernel).Error("LoadFactsFromFile: failed to parse facts: %v", err)
		return fmt.Errorf("failed to parse facts: %w", err)
	}
	logging.KernelDebug("LoadFactsFromFile: parsed %d facts from file", len(facts))

	if err := k.LoadFacts(facts); err != nil {
		return err
	}

	timer.StopWithInfo()
	logging.Kernel("LoadFactsFromFile: loaded %d facts from %s", len(facts), path)
	return nil
}

// UpdateSystemFacts updates transient system facts like time.
// This should be called ONCE per turn/request to avoid infinite loops
// in logic that depends on changing time (Fix for Bug #7).
func (k *RealKernel) UpdateSystemFacts() error {
	timer := logging.StartTimer(logging.CategoryKernel, "UpdateSystemFacts")
	logging.KernelDebug("UpdateSystemFacts: updating transient system facts")

	k.mu.Lock()
	defer k.mu.Unlock()

	// 1. Retract old system facts
	prevCount := len(k.facts)
	newFacts := make([]Fact, 0, len(k.facts)+1)
	retractedCount := 0
	for _, f := range k.facts {
		if f.Predicate != "current_time" {
			newFacts = append(newFacts, f)
		} else {
			retractedCount++
		}
	}
	k.facts = newFacts
	k.rebuildFactIndexLocked()
	logging.KernelDebug("UpdateSystemFacts: retracted %d old system facts", retractedCount)

	// 2. Add fresh system facts
	now := time.Now().Unix()
	k.addFactIfNewLocked(Fact{
		Predicate: "current_time",
		Args:      []interface{}{now},
	})
	logging.KernelDebug("UpdateSystemFacts: added current_time=%d", now)

	// 3. Re-evaluate
	// We use evaluate() directly since we already hold the lock
	if err := k.evaluate(); err != nil {
		logging.Get(logging.CategoryKernel).Error("UpdateSystemFacts: evaluation failed: %v", err)
		return err
	}

	timer.Stop()
	logging.KernelDebug("UpdateSystemFacts: complete, EDB: %d -> %d facts", prevCount, len(k.facts))
	return nil
}

// ParseFactString parses a single Mangle fact string.
// Example: "northstar_mission(/ns_mission, \"The mission\")"
func ParseFactString(factStr string) (Fact, error) {
	// Ensure the fact ends with a period for the parser
	normalized := strings.TrimSpace(factStr)
	if !strings.HasSuffix(normalized, ".") {
		normalized += "."
	}

	facts, err := ParseFactsFromString(normalized)
	if err != nil {
		return Fact{}, err
	}
	if len(facts) == 0 {
		return Fact{}, fmt.Errorf("no fact found in %q", factStr)
	}
	return facts[0], nil
}

// ParseFactsFromString parses Mangle fact statements from a string.
// Uses the official Mangle parser to ensure safety (Fix for Bug #11).
func ParseFactsFromString(content string) ([]Fact, error) {
	// Use the official parser to parse the content as a Unit
	unit, err := parse.Unit(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse facts string: %w", err)
	}

	facts := make([]Fact, 0)
	for _, clause := range unit.Clauses {
		// A fact is a clause with no body
		if len(clause.Premises) > 0 {
			continue // Skip rules
		}

		// Convert the head atom to our Fact type
		facts = append(facts, atomToFact(clause.Head))
	}

	return facts, nil
}

// =============================================================================
// AUTOPOIESIS BRIDGE (Formerly Kernel Adapter)
// =============================================================================

// AutopoiesisBridge wraps RealKernel to implement types.KernelInterface.
type AutopoiesisBridge struct {
	kernel *RealKernel
}

// NewAutopoiesisBridge creates an adapter that implements types.KernelInterface.
func NewAutopoiesisBridge(kernel *RealKernel) *AutopoiesisBridge {
	return &AutopoiesisBridge{kernel: kernel}
}

// AssertFact implements types.KernelInterface.
func (ab *AutopoiesisBridge) AssertFact(fact types.KernelFact) error {
	coreFact := Fact{
		Predicate: fact.Predicate,
		Args:      fact.Args,
	}
	return ab.kernel.Assert(coreFact)
}

// QueryPredicate implements types.KernelInterface.
func (ab *AutopoiesisBridge) QueryPredicate(predicate string) ([]types.KernelFact, error) {
	facts, err := ab.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}

	result := make([]types.KernelFact, len(facts))
	for i, f := range facts {
		result[i] = types.KernelFact{
			Predicate: f.Predicate,
			Args:      f.Args,
		}
	}
	return result, nil
}

// QueryBool implements types.KernelInterface.
func (ab *AutopoiesisBridge) QueryBool(predicate string) bool {
	facts, err := ab.kernel.Query(predicate)
	if err != nil {
		return false
	}
	return len(facts) > 0
}

// RetractFact implements types.KernelInterface.
func (ab *AutopoiesisBridge) RetractFact(fact types.KernelFact) error {
	coreFact := Fact{
		Predicate: fact.Predicate,
		Args:      fact.Args,
	}
	return ab.kernel.RetractFact(coreFact)
}

// Ensure AutopoiesisBridge implements KernelInterface at compile time.
var _ types.KernelInterface = (*AutopoiesisBridge)(nil)

// Ensure RealKernel implements feedback.RuleValidator at compile time.
var _ feedback.RuleValidator = (*RealKernel)(nil)
