package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/mangle"

	"github.com/google/mangle/factstore"
)

// NewRealKernel creates a new kernel instance.
// Returns an error if the embedded constitution fails to compile (e.g., corrupted binary).
func NewRealKernel() (*RealKernel, error) {
	timer := logging.StartTimer(logging.CategoryKernel, "NewRealKernel")
	logging.Kernel("Initializing new RealKernel instance")

	k := &RealKernel{
		facts:             make([]Fact, 0),
		factIndex:         make(map[string]struct{}),
		bootFacts:         make([]Fact, 0),
		bootIntents:       make([]HybridIntent, 0),
		bootPrompts:       make([]HybridPrompt, 0),
		store:             factstore.NewSimpleInMemoryStore(),
		loadedPolicyFiles: make(map[string]struct{}),
		policyDirty:       true, // Need to parse on first use
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
		factIndex:         make(map[string]struct{}),
		bootFacts:         make([]Fact, 0),
		bootIntents:       make([]HybridIntent, 0),
		bootPrompts:       make([]HybridPrompt, 0),
		store:             factstore.NewSimpleInMemoryStore(),
		workspaceRoot:     workspaceRoot,
		loadedPolicyFiles: make(map[string]struct{}),
		policyDirty:       true, // Need to parse on first use
	}
	logging.KernelDebug("Kernel struct created with workspaceRoot=%s, policyDirty=true", workspaceRoot)

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
		facts:             make([]Fact, 0),
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

	// Load Core Schemas (Modular)
	// Load the index file first (contains core predicates and documentation)
	if data, err := coreLogic.ReadFile("defaults/schemas.mg"); err == nil {
		k.schemas = string(data)
		logging.KernelDebug("Loaded schema index (%d bytes)", len(data))
	} else {
		logging.Get(logging.CategoryKernel).Error("Failed to load schema index: %v", err)
	}

	// Load all modular schema files (schemas_*.mg)
	// This allows selective loading and better organization (18 modules, all under 600 lines)
	schemaFiles := []string{
		"schemas_intent.mg",     // Intent & Focus Resolution
		"schemas_world.mg",      // File Topology, Symbol Graph, Diagnostics
		"schemas_execution.mg",  // TDD Loop & Action Execution
		"schemas_browser.mg",    // Browser Physics & Spatial Reasoning
		"schemas_project.mg",    // Project Profile, User Preferences, Session State
		"schemas_dreamer.mg",    // Speculative Dreamer & Cross-Module Support
		"schemas_memory.mg",     // Memory Tiers & Knowledge
		"schemas_knowledge.mg",  // Knowledge Atoms, LSP, Semantic Matching
		"schemas_safety.mg",     // Constitution, Git Safety, Shadow Mode
		"schemas_analysis.mg",   // Spreading Activation, Strategy, Impact
		"schemas_misc.mg",       // Northstar, Continuation Protocol, Benchmarks
		"schemas_codedom.mg",    // Code DOM & Interactive Elements
		"schemas_testing.mg",    // Verification, Reasoning Traces, Pytest
		"schemas_campaign.mg",   // Campaign Orchestration
		"schemas_tools.mg",      // Ouroboros, Tool Learning, Routing
		"schemas_prompts.mg",    // Dynamic Prompt Composition & JIT
		"schemas_reviewer.mg",   // Static Analysis & Data Flow
		"schemas_shards.mg",     // Shard Delegation & Coordination
	}

	loadedSchemaBytes := 0
	for _, schemaFile := range schemaFiles {
		path := "defaults/" + schemaFile
		if data, err := coreLogic.ReadFile(path); err == nil {
			k.schemas += "\n\n# Schema Module: " + schemaFile + "\n" + string(data)
			loadedSchemaBytes += len(data)
			logging.KernelDebug("Loaded schema module: %s (%d bytes)", schemaFile, len(data))
		} else {
			logging.Get(logging.CategoryKernel).Warn("Failed to read schema module %s: %v", path, err)
		}
	}
	logging.KernelDebug("Loaded modular schemas (%d bytes from %d files)", loadedSchemaBytes, len(schemaFiles))

	// Load legacy JIT Prompt Schema if it exists (for backward compatibility)
	if data, err := coreLogic.ReadFile("defaults/schema/prompts.mg"); err == nil {
		k.schemas += "\n\n" + string(data)
		logging.KernelDebug("Loaded legacy JIT prompt schema (%d bytes)", len(data))
	}

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
					k.policy += "\n\n# Policy Module: " + entry.Name() + "\n" + string(data)
					loadedPolicyBytes += len(data)
					logging.KernelDebug("Loaded policy module: %s (%d bytes)", entry.Name(), len(data))
				} else {
					logging.Get(logging.CategoryKernel).Warn("Failed to read policy module %s: %v", path, err)
				}
			}
		}
		logging.KernelDebug("Loaded stratified policy (%d bytes from %d files)", loadedPolicyBytes, len(policyEntries))
	} else {
		// Fallback for backward compatibility or if directory missing
		logging.Get(logging.CategoryKernel).Warn("Failed to read policy directory: %v, falling back to legacy policy.mg", err)
		if data, err := coreLogic.ReadFile("defaults/policy.mg"); err == nil {
			k.policy = string(data)
			logging.KernelDebug("Loaded legacy core policy (%d bytes)", len(data))
		} else {
			logging.Get(logging.CategoryKernel).Error("Failed to load core policy: %v", err)
		}
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
		"jit_config.mg", // System 2 JIT Configuration
		"jit_logic.mg",  // System 2 JIT Logic
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
