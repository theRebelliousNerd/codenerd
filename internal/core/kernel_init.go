package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/mangle"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/factstore"
)

// filterBootFacts removes ephemeral facts from boot facts to ensure quiescent boot.
// This prevents stale user_intent, pending_action, etc. from triggering action derivation at boot.
func filterBootFacts(bootFacts []Fact) []Fact {
	if len(bootFacts) == 0 {
		return bootFacts
	}

	persistentFacts := make([]Fact, 0, len(bootFacts))
	filteredCount := 0
	for _, fact := range bootFacts {
		if IsEphemeral(fact.Predicate) {
			logging.KernelDebug("Filtering ephemeral fact from boot: %s", fact.Predicate)
			filteredCount++
			continue
		}
		persistentFacts = append(persistentFacts, fact)
	}
	if filteredCount > 0 {
		logging.Kernel("Filtered %d ephemeral facts from boot (quiescent boot)", filteredCount)
	}
	return persistentFacts
}

// NewRealKernel creates a new kernel instance.
// Returns an error if the embedded constitution fails to compile (e.g., corrupted binary).
func NewRealKernel() (*RealKernel, error) {
	timer := logging.StartTimer(logging.CategoryKernel, "NewRealKernel")
	logging.Kernel("Initializing new RealKernel instance")

	k := &RealKernel{
		facts:             make([]Fact, 0),
		cachedAtoms:       make([]ast.Atom, 0), // OPTIMIZATION: Initialize atom cache
		factIndex:         make(map[string]struct{}),
		bootFacts:         make([]Fact, 0),
		bootIntents:       make([]HybridIntent, 0),
		bootPrompts:       make([]HybridPrompt, 0),
		store:             factstore.NewSimpleInMemoryStore(),
		loadedPolicyFiles: make(map[string]struct{}),
		policyDirty:       true, // Need to parse on first use
		eventBus:          NewFactEventBus(),
	}
	logging.KernelDebug("Kernel struct created, store initialized, policyDirty=true")

	// Find and load mangle files from the project
	if err := k.loadMangleFiles(); err != nil {
		timer.Stop()
		return nil, fmt.Errorf("failed to load mangle files: %w", err)
	}

	// Inject any EDB facts extracted from hybrid .mg files before first evaluation.
	// QUIESCENT BOOT: Filter ephemeral facts to prevent stale actions at boot.
	if len(k.bootFacts) > 0 {
		k.facts = append(k.facts, filterBootFacts(k.bootFacts)...)
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

// NewRealKernelWithWorkspace creates a kernel rooted at an explicit workspace directory.
// This ensures `.nerd/...` resolution is stable even when the process CWD is not the workspace root.
func NewRealKernelWithWorkspace(workspaceRoot string) (*RealKernel, error) {
	timer := logging.StartTimer(logging.CategoryKernel, "NewRealKernelWithWorkspace")
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot != "" {
		if abs, err := filepath.Abs(workspaceRoot); err == nil {
			workspaceRoot = abs
		}
	}
	logging.Kernel("Initializing RealKernel with workspace root: %s", workspaceRoot)

	k := &RealKernel{
		facts:             make([]Fact, 0),
		cachedAtoms:       make([]ast.Atom, 0), // OPTIMIZATION: Initialize atom cache
		factIndex:         make(map[string]struct{}),
		bootFacts:         make([]Fact, 0),
		bootIntents:       make([]HybridIntent, 0),
		bootPrompts:       make([]HybridPrompt, 0),
		store:             factstore.NewSimpleInMemoryStore(),
		workspaceRoot:     workspaceRoot,
		loadedPolicyFiles: make(map[string]struct{}),
		policyDirty:       true, // Need to parse on first use
		eventBus:          NewFactEventBus(),
	}
	logging.KernelDebug("Kernel struct created with workspaceRoot=%s, policyDirty=true", workspaceRoot)

	// Find and load mangle files from the project
	if err := k.loadMangleFiles(); err != nil {
		timer.Stop()
		return nil, fmt.Errorf("failed to load mangle files: %w", err)
	}

	// Inject any EDB facts extracted from hybrid .mg files before first evaluation.
	// QUIESCENT BOOT: Filter ephemeral facts to prevent stale actions at boot.
	if len(k.bootFacts) > 0 {
		k.facts = append(k.facts, filterBootFacts(k.bootFacts)...)
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
		facts:             make([]Fact, 0),
		cachedAtoms:       make([]ast.Atom, 0), // OPTIMIZATION: Initialize atom cache
		factIndex:         make(map[string]struct{}),
		bootFacts:         make([]Fact, 0),
		bootIntents:       make([]HybridIntent, 0),
		bootPrompts:       make([]HybridPrompt, 0),
		store:             factstore.NewSimpleInMemoryStore(),
		manglePath:        manglePath,
		loadedPolicyFiles: make(map[string]struct{}),
		policyDirty:       true,
	}
	logging.KernelDebug("Kernel struct created with manglePath=%s", manglePath)

	if err := k.loadMangleFiles(); err != nil {
		timer.Stop()
		return nil, fmt.Errorf("failed to load mangle files: %w", err)
	}

	// Inject any EDB facts extracted from hybrid .mg files before first evaluation.
	// QUIESCENT BOOT: Filter ephemeral facts to prevent stale actions at boot.
	if len(k.bootFacts) > 0 {
		k.facts = append(k.facts, filterBootFacts(k.bootFacts)...)
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

// defaultDerivedFactLimit is the kernel-wide fallback for every evaluation mode.
// Keep full and differential evaluation on this single source of truth so an
// unset limit cannot become path-dependent.
const defaultDerivedFactLimit = 500_000

// effectiveDerivedFactLimitLocked resolves the configured inference ceiling.
// The caller must hold k.mu for reading unless the kernel is not yet shared.
func (k *RealKernel) effectiveDerivedFactLimitLocked() int {
	if k.derivedFactLimit <= 0 {
		return defaultDerivedFactLimit
	}
	return k.derivedFactLimit
}

// SetDerivedFactLimit sets the maximum number of derived facts during evaluation.
// Set to 0 or negative to use the default (500,000).
func (k *RealKernel) SetDerivedFactLimit(limit int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.derivedFactLimit = limit
}

// GetDerivedFactLimit returns the current derived fact limit.
func (k *RealKernel) GetDerivedFactLimit() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.effectiveDerivedFactLimitLocked()
}

// defaultMaxFacts is the default limit for EDB facts in the kernel.
const defaultMaxFacts = 250_000

// SetMaxFacts sets the maximum number of EDB facts the kernel will accept.
// Set to 0 or negative to use the default (250,000).
func (k *RealKernel) SetMaxFacts(limit int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.maxFacts = limit
}

// GetMaxFacts returns the current EDB fact limit.
func (k *RealKernel) GetMaxFacts() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.maxFacts <= 0 {
		return defaultMaxFacts
	}
	return k.maxFacts
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

	var schemasBuilder strings.Builder
	var policyBuilder strings.Builder
	var learnedBuilder strings.Builder

	// 1. LOAD BAKED-IN CORE (Immutable Physics)
	// Always load these. They are the "Constitution".
	logging.KernelDebug("Loading baked-in core (Constitution)...")

	// Load Core Schemas (Modular)
	// Load the index file first (contains core predicates and documentation)
	if data, err := coreLogic.ReadFile("defaults/schemas.mg"); err == nil {
		schemasBuilder.Write(data)
		logging.KernelDebug("Loaded schema index (%d bytes)", len(data))
	} else {
		logging.Get(logging.CategoryKernel).Error("Failed to load schema index: %v", err)
	}

	// Load all modular schema files (schemas_*.mg) plus learning schema.
	// This allows selective loading and better organization (modular schemas under 600 lines).
	schemaFiles := []string{
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

	loadedSchemaBytes := 0
	for _, schemaFile := range schemaFiles {
		path := "defaults/" + schemaFile
		if data, err := coreLogic.ReadFile(path); err == nil {
			schemasBuilder.WriteString("\n\n# Schema Module: ")
			schemasBuilder.WriteString(schemaFile)
			schemasBuilder.WriteString("\n")
			schemasBuilder.Write(data)
			loadedSchemaBytes += len(data)
			logging.KernelDebug("Loaded schema module: %s (%d bytes)", schemaFile, len(data))
		} else {
			logging.Get(logging.CategoryKernel).Warn("Failed to read schema module %s: %v", path, err)
		}
	}
	logging.KernelDebug("Loaded modular schemas (%d bytes from %d files)", loadedSchemaBytes, len(schemaFiles))

	// Load Core Policy (Stratified)
	// Iterate over the split policy files in defaults/policy/
	policyDir := "defaults/policy"
	policyEntries, err := coreLogic.ReadDir(policyDir)
	if err == nil {
		loadedPolicyBytes := 0
		for _, entry := range policyEntries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".mg") {
				path := policyDir + "/" + entry.Name()
				if data, err := coreLogic.ReadFile(path); err == nil {
					policyBuilder.WriteString("\n\n# Policy Module: ")
					policyBuilder.WriteString(entry.Name())
					policyBuilder.WriteString("\n")
					policyBuilder.Write(data)
					loadedPolicyBytes += len(data)
					logging.KernelDebug("Loaded policy module: %s (%d bytes)", entry.Name(), len(data))
				} else {
					logging.Get(logging.CategoryKernel).Warn("Failed to read policy module %s: %v", path, err)
				}
			}
		}
		logging.KernelDebug("Loaded stratified policy (%d bytes from %d files)", loadedPolicyBytes, len(policyEntries))
	} else {
		// Policy directory is required — no fallback to monolithic policy.mg
		logging.Get(logging.CategoryKernel).Error("Failed to read policy directory %s: %v", policyDir, err)
		return fmt.Errorf("critical: failed to read policy directory: %w", err)
	}

	// Load other core modules into policy
	coreModules := DefaultCorePolicyModules()

	loadedModules := 0
	for _, mod := range coreModules {
		if data, err := coreLogic.ReadFile("defaults/" + mod); err == nil {
			policyBuilder.WriteString("\n\n")
			policyBuilder.Write(data)
			k.loadedPolicyFiles[strings.ToLower(mod)] = struct{}{}
			loadedModules++
			logging.KernelDebug("Loaded core module: %s (%d bytes)", mod, len(data))
		} else {
			logging.KernelDebug("Core module not found (optional): %s", mod)
		}
	}
	logging.KernelDebug("Loaded %d/%d core modules", loadedModules, len(coreModules))

	// Load embedded intent corpus facts for routing and semantic classification.
	k.loadEmbeddedIntentFacts()

	// Load base learned rules (if any)
	if data, err := coreLogic.ReadFile("defaults/learned.mg"); err == nil {
		learnedBuilder.Write(data)
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
				schemasBuilder.WriteString("\n\n# User Extensions\n")
				schemasBuilder.WriteString(res.Logic)
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
				policyBuilder.WriteString("\n\n# User Policy Overrides\n")
				policyBuilder.WriteString(res.Logic)
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
			// Finalize schemas and policy before validator usage
			if learnedBuilder.Len() > 0 {
				k.learned = learnedBuilder.String()
			}
			if schemasBuilder.Len() > 0 {
				k.schemas = schemasBuilder.String()
			}
			if policyBuilder.Len() > 0 {
				k.policy = policyBuilder.String()
			}

			// Initialize schema validator early so we can heal user rules before appending
			if k.schemas != "" && k.schemaValidator == nil {
				k.schemaValidator = mangle.NewSchemaValidator(k.schemas, learnedBuilder.String()+"\n"+userLearnedContent)
				if err := k.schemaValidator.LoadDeclaredPredicates(); err != nil {
					logging.Get(logging.CategoryKernel).Warn("Failed to load schema validator: %v", err)
				}
			}

			// Self-heal user learned rules BEFORE appending to k.learned
			if k.schemaValidator != nil {
				userLearnedContent = k.healLearnedRules(userLearnedContent, learnedPath)
			}

			// Append healed user rules to base learned rules
			learnedBuilder.WriteString("\n\n# User Learned Rules\n")
			learnedBuilder.WriteString(userLearnedContent)
		}
	}
	logging.KernelDebug("Loaded %d user extension files", userExtensionsLoaded)
	// Finalize strings if not already done
	if learnedBuilder.Len() > 0 {
		k.learned = learnedBuilder.String()
	}
	if schemasBuilder.Len() > 0 {
		k.schemas = schemasBuilder.String()
	}
	if policyBuilder.Len() > 0 {
		k.policy = policyBuilder.String()
	}

	// Ensure schema validator reflects the final schemas + learned rules.

	k.refreshSchemaValidatorLocked()

	timer.Stop()
	logging.Kernel("Mangle files loaded: schemas=%d bytes, policy=%d bytes, learned=%d bytes",
		len(k.schemas), len(k.policy), len(k.learned))
	return nil
}

// loadPredicateCorpus opens the baked-in predicate corpus for validation.
// It is called through predicateCorpusOnce so ordinary kernel boot and clones do
// not allocate a SQLite connection and temp file when no consumer needs corpus
// metadata.
func (k *RealKernel) loadPredicateCorpus() {
	timer := logging.StartTimer(logging.CategoryKernel, "loadPredicateCorpus")
	logging.Kernel("Loading baked-in predicate corpus...")

	corpus, err := NewPredicateCorpus()
	if err != nil {
		logging.Get(logging.CategoryKernel).Warn("Predicate corpus not available: %v", err)
		timer.Stop()
		return
	}

	k.mu.Lock()
	k.predicateCorpus = corpus
	k.mu.Unlock()
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
	k.mu.RLock()
	corpus := k.predicateCorpus
	k.mu.RUnlock()
	if corpus != nil {
		return corpus
	}

	k.predicateCorpusOnce.Do(k.loadPredicateCorpus)
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.predicateCorpus
}

// GetEventBus returns the kernel's fact event bus for pub/sub subscriptions.
// System shards use this to subscribe to specific predicates instead of polling.
func (k *RealKernel) GetEventBus() *FactEventBus {
	return k.eventBus
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
