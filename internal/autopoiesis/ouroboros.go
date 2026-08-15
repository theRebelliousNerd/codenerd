// MANDATE FOR AI CODING TOOLS:
// This file contains critical product requirements and architectural mandates.
// DO NOT REMOVE OR SIGNIFICANTLY ALTER THESE COMMENTS.
// YOU MUST READ THIS ENTIRE HEADER BEFORE AND AFTER EVERY INTERACTION WITH THIS FILE.

// Symbiogen Product Requirements Document (PRD) for internal/autopoiesis/ouroboros.go
//
// File: internal/autopoiesis/ouroboros.go
// Author: Gemini
// Date: 2025-12-08
//
// Recommended Model: Gemini 3 Pro
//
// Overview:
// Implements the Ouroboros Loop, the self-eating serpent of tool generation.
// It serves as the "Transactional State Machine" for autopoiesis, governing the
// creation of new capabilities through a rigorous, Mangle-controlled lifecycle.
//
// Key Features & Business Value:
// - Transactional Loop: Proposal -> Audit -> Simulation -> Commit.
// - Mangle Governance: Uses Logic Programming to validate state transitions.
// - Stability Guarantee: Enforces monotonic or permissible stability changes.
// - Stagnation Detection: Prevents infinite generation loops via Halting Oracle.
// - Panic Recovery: Captures crashes as error events in the logic layer.
//
// Architectural Context:
// - Component Type: Autopoiesis Core / State Machine
// - Deployment: Part of the Autopoiesis Orchestrator.
// - Communication: Uses Mangle Engine (Differential) for logic simulation.
// - Database Interaction: Loads state rules from `state.mg`.
//
// Dependencies & Dependents:
// - Dependencies: `codenerd/internal/mangle`, `codenerd/internal/mangle/transpiler`.
// - Is a Dependency for: `autopoiesis.Orchestrator`.
//
// Deployment & Operations:
// - CI/CD: Standard Go build.
// - Configuration: `OuroborosConfig`.
//
// Code Quality Mandate:
// All code in this file must be production-ready. This includes complete error
// handling and clear logging.
//
// Functions / Classes:
// - `OuroborosLoop`: The state machine struct.
// - `Execute`: The main transactional loop.
// - `NewOuroborosLoop`: Initialization with Mangle engine.
//
// Usage:
// loop := NewOuroborosLoop(client, config)
// result := loop.Execute(ctx, toolNeed)
//
// References:
// - Internal Task: Transactional State Machine
//
// --- END OF PRD HEADER ---

package autopoiesis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	internalconfig "codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/mangle/transpiler"
	"codenerd/internal/types"
)

// =============================================================================
// OUROBOROS LOOP - THE SELF-EATING SERPENT
// =============================================================================
// The Ouroboros Loop enables codeNERD to generate new tools at runtime.
// Named after the ancient symbol of a serpent eating its own tail,
// representing infinite self-creation and renewal.

// OuroborosLoop orchestrates the full tool self-generation cycle
// It implements a "Transactional State Machine" governed by Mangle.
type OuroborosLoop struct {
	mu sync.RWMutex

	toolGen       *ToolGenerator
	safetyChecker *SafetyChecker
	compiler      *ToolCompiler
	registry      *RuntimeRegistry
	sanitizer     *transpiler.Sanitizer
	engine        *mangle.Engine // The Mangle Engine governing the loop

	// Adversarial Co-Evolution components
	panicMaker  *PanicMaker  // Tool-level adversarial testing
	thunderdome *Thunderdome // Arena for running adversarial tests

	config OuroborosConfig
	stats  OuroborosStats

	// Callback for notifying parent when a tool is registered
	onToolRegistered ToolRegisteredCallback
}

// OuroborosConfig configures the Ouroboros Loop
type OuroborosConfig struct {
	ToolsDir        string        // Directory for generated tools
	CompiledDir     string        // Directory for compiled tools
	MaxToolSize     int64         // Maximum tool source size in bytes
	CompileTimeout  time.Duration // Timeout for compilation
	ExecuteTimeout  time.Duration // Timeout for tool execution
	AllowNetworking bool          // Whether tools can use networking
	AllowFileSystem bool          // Whether tools can access filesystem
	AllowExec       bool          // Whether tools can execute commands (see AllowToolExec audit note on Config)
	TargetOS        string        // Target operating system (GOOS)
	TargetArch      string        // Target architecture (GOARCH)
	WorkspaceRoot   string        // Absolute path to the main codeNERD workspace root

	// UserConfig carries the operator's build environment into the compile and
	// arena subprocesses. Nil means "process defaults only".
	UserConfig *internalconfig.UserConfig

	// Adversarial Co-Evolution configuration
	EnableThunderdome bool              // Whether to run adversarial tests (default: true)
	ThunderdomeConfig ThunderdomeConfig // Configuration for the Thunderdome arena
	MaxPanicRetries   int               // Max regeneration attempts after PanicMaker kill (default: 2)
}

// DefaultOuroborosConfig returns safe default configuration
func DefaultOuroborosConfig(workspaceRoot string) OuroborosConfig {
	return OuroborosConfig{
		ToolsDir:        filepath.Join(workspaceRoot, ".nerd", "tools"),
		CompiledDir:     filepath.Join(workspaceRoot, ".nerd", "tools", ".compiled"),
		MaxToolSize:     100 * 1024, // 100KB max
		CompileTimeout:  300 * time.Second,
		ExecuteTimeout:  300 * time.Second,
		AllowNetworking: false,
		AllowFileSystem: true, // Read-only by default
		// AUDIT (TODO P0 "Audit default AllowExec: true"): this defaulted to
		// true, which put os/exec on the safety allowlist for every generated
		// tool in every workspace. go_safety.mg gates imports and nothing else,
		// so an allowlisted os/exec is an unrestricted shell running with the
		// user's workspace as cwd — a strictly larger capability than anything
		// else autopoiesis grants, handed out by default. Denied by default
		// now; grant it per workspace via Config.AllowToolExec.
		AllowExec:     false,
		TargetOS:      os.Getenv("GOOS"),
		TargetArch:    os.Getenv("GOARCH"),
		WorkspaceRoot: workspaceRoot,
		// Adversarial Co-Evolution defaults
		EnableThunderdome: true,
		ThunderdomeConfig: DefaultThunderdomeConfig(),
		MaxPanicRetries:   2,
	}
}

// RetryConfig controls the feedback retry loop for safety violations.
type RetryConfig struct {
	MaxRetries int           // Maximum retry attempts (default: 3)
	RetryDelay time.Duration // Delay between retries (default: 100ms)
}

// DefaultRetryConfig returns safe default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
	}
}

// ExecuteConfig extends execution options for the Ouroboros loop.
type ExecuteConfig struct {
	Retry     RetryConfig // Retry configuration for safety violations
	HotReload bool        // Whether to hot-reload tools after commit (default: true)
	MaxIters  int         // Maximum loop iterations (default: 10)
}

// DefaultExecuteConfig returns safe default execution configuration.
func DefaultExecuteConfig() ExecuteConfig {
	return ExecuteConfig{
		Retry:     DefaultRetryConfig(),
		HotReload: true,
		MaxIters:  10,
	}
}

// NewOuroborosLoop creates a new Ouroboros Loop instance
func NewOuroborosLoop(client LLMClient, config OuroborosConfig) *OuroborosLoop {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "NewOuroborosLoop")
	defer timer.Stop()

	logging.Autopoiesis("Initializing Ouroboros Loop")
	logging.AutopoiesisDebug("Config: ToolsDir=%s, CompiledDir=%s, WorkspaceRoot=%s",
		config.ToolsDir, config.CompiledDir, config.WorkspaceRoot)

	// Set defaults for OS/Arch if missing.
	//
	// The fallbacks were the literals "windows" and "amd64". Generated tools
	// are compiled AND executed by this process (RuntimeTool.Execute runs the
	// binary), so on any host where GOOS is not exported — the normal case —
	// every tool was cross-compiled for Windows and then failed at call time
	// with "exec format error". runtime.GOOS/GOARCH describe the machine that
	// will actually run the binary; an explicit cross-compile target still
	// wins because it is only consulted when the field is empty.
	if config.TargetOS == "" {
		if env := os.Getenv("GOOS"); env != "" {
			config.TargetOS = env
		} else {
			config.TargetOS = runtime.GOOS
		}
		logging.AutopoiesisDebug("TargetOS defaulted to: %s", config.TargetOS)
	}
	if config.TargetArch == "" {
		if env := os.Getenv("GOARCH"); env != "" {
			config.TargetArch = env
		} else {
			config.TargetArch = runtime.GOARCH
		}
		logging.AutopoiesisDebug("TargetArch defaulted to: %s", config.TargetArch)
	}

	// The arena has to compile with the same toolchain environment as the real
	// compile, so it inherits the operator's UserConfig unless one was set
	// explicitly on the nested config.
	if config.ThunderdomeConfig.UserConfig == nil {
		config.ThunderdomeConfig.UserConfig = config.UserConfig
	}

	// Initialize Mangle Engine
	logging.AutopoiesisDebug("Initializing Mangle engine for state machine")
	engineConfig := mangle.DefaultConfig()
	// Disable auto-eval for initial load to speed it up
	engineConfig.AutoEval = false
	// We don't need persistence for this transient loop engine yet,
	// but in production it should likely persist history.
	// For now, nil persistence.
	engine, err := mangle.NewEngine(engineConfig, nil)
	if err != nil {
		// Fallback to panic if engine cannot start - essential component
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to initialize Mangle engine: %v", err)
		panic(fmt.Sprintf("failed to initialize Ouroboros Mangle engine: %v", err))
	}
	logging.AutopoiesisDebug("Mangle engine initialized successfully")

	loop := &OuroborosLoop{
		toolGen:       NewToolGenerator(client, config.ToolsDir),
		safetyChecker: NewSafetyChecker(config),
		compiler:      NewToolCompiler(config),
		registry:      NewRuntimeRegistry(),
		sanitizer:     transpiler.NewSanitizer(),
		engine:        engine,
		config:        config,
	}

	// Initialize Adversarial Co-Evolution components
	if config.EnableThunderdome {
		logging.Autopoiesis("Initializing Adversarial Co-Evolution components (Thunderdome enabled)")
		loop.panicMaker = NewPanicMaker(client)
		loop.thunderdome = NewThunderdomeWithConfig(config.ThunderdomeConfig)
	} else {
		logging.Autopoiesis("Thunderdome disabled - tools will not undergo adversarial testing")
	}

	// Restore registry from disk
	logging.AutopoiesisDebug("Restoring tool registry from disk")
	loop.registry.Restore(config.ToolsDir, config.CompiledDir)
	toolCount := len(loop.registry.List())
	logging.Autopoiesis("Restored %d tools from registry", toolCount)

	// Load State Machine Rules from embedded core (not workspace filesystem)
	logging.AutopoiesisDebug("Loading state machine rules from embedded core")
	stateContent, err := core.GetDefaultContent("schemas_state.mg")
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("Failed to get embedded schemas_state.mg: %v. Operating in open-loop mode", err)
	} else if err := loop.engine.LoadSchemaString(stateContent); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("Failed to parse schemas_state.mg: %v. Operating in open-loop mode", err)
	} else {
		logging.AutopoiesisDebug("State machine rules loaded successfully from embedded core")
	}

	loop.engine.ToggleAutoEval(true)

	logging.Autopoiesis("Ouroboros Loop initialized: TargetOS=%s, TargetArch=%s", config.TargetOS, config.TargetArch)
	return loop
}

// SetOnToolRegistered sets the callback for when a tool is registered.
// This allows the Orchestrator to propagate facts to the parent kernel.
func (o *OuroborosLoop) SetOnToolRegistered(callback ToolRegisteredCallback) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.onToolRegistered = callback
}

// =============================================================================
// THE LOOP STAGES
// =============================================================================

// Execute executes the Transactional State Machine for tool generation with default config.
// This is a convenience wrapper around ExecuteWithConfig.
func (o *OuroborosLoop) Execute(ctx context.Context, need *ToolNeed) (result *LoopResult) {
	return o.ExecuteWithConfig(ctx, need, DefaultExecuteConfig())
}

// ExecuteWithConfig executes the Transactional State Machine for tool generation.
// Enhanced with retry feedback loop, hot-reload capability, and stability penalties.
//
// Protocol:
// 1. Proposal: Generate & Sanitize (with retry feedback if previous attempt failed)
// 2. Audit: Safety Check (retries with feedback on failure)
// 3. Simulation: Differential Analysis & Transition Validation
// 4. Commit: Compile, Register & Hot-Reload
func (o *OuroborosLoop) ExecuteWithConfig(ctx context.Context, need *ToolNeed, cfg ExecuteConfig) (result *LoopResult) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "OuroborosLoop.Execute")
	start := time.Now()

	if need == nil {
		return &LoopResult{
			Success: false,
			Stage:   StageDetection,
			Error:   "tool need is nil",
		}
	}
	if strings.TrimSpace(need.Name) == "" {
		return &LoopResult{
			Success:  false,
			ToolName: "",
			Stage:    StageDetection,
			Error:    "tool need name is required",
		}
	}
	if strings.TrimSpace(need.Purpose) == "" {
		need.Purpose = fmt.Sprintf("Auto-generated tool for capability: %s", need.Name)
	}

	logging.Autopoiesis("=== OUROBOROS LOOP START: tool=%s ===", need.Name)
	logging.AutopoiesisDebug("Tool need: purpose=%s, confidence=%.2f, priority=%.2f",
		need.Purpose, need.Confidence, need.Priority)
	logging.AutopoiesisDebug("Execute config: MaxIters=%d, MaxRetries=%d, HotReload=%v",
		cfg.MaxIters, cfg.Retry.MaxRetries, cfg.HotReload)

	result = &LoopResult{
		ToolName: need.Name,
		Stage:    StageDetection,
	}

	// Format stepID as Mangle name constant
	stepID := fmt.Sprintf("/step_%s", need.Name)
	iterNum := 0

	// Initialize state in Mangle
	logging.AutopoiesisDebug("Initializing Mangle state for stepID=%s", stepID)
	o.initializeState(stepID, cfg.MaxIters, cfg.Retry.MaxRetries)

	// Panic Recovery with penalty tracking
	defer func() {
		if r := recover(); r != nil {
			logging.Get(logging.CategoryAutopoiesis).Error("PANIC in Ouroboros Loop: %v", r)
			o.handlePanic(stepID, r, result)
		}
		timer.Stop()
		logging.Autopoiesis("=== OUROBOROS LOOP END: tool=%s, success=%v, stage=%s, duration=%v ===",
			need.Name, result.Success, result.Stage, result.Duration)
	}()

	var lastViolations []SafetyViolation
	retryCount := 0
	var tool *GeneratedTool

	// Main execution loop with retry capability
	for iterNum < cfg.MaxIters {
		logging.Autopoiesis("Loop iteration %d/%d for tool=%s", iterNum+1, cfg.MaxIters, need.Name)

		// Check Mangle halt conditions
		if o.shouldHalt(stepID) {
			logging.Get(logging.CategoryAutopoiesis).Warn("Halted by Mangle policy for stepID=%s", stepID)
			result.Error = "halted by Mangle policy (max iterations, retries, stagnation, or degradation)"
			return result
		}

		// Record iteration in Mangle
		o.recordIteration(stepID, iterNum)

		// =====================================================================
		// PHASE 1: PROPOSAL (with retry feedback if available)
		// =====================================================================
		logging.Autopoiesis("[STAGE: %s] Starting specification phase", StageSpecification)
		result.Stage = StageSpecification
		var err error

		if retryCount > 0 && len(lastViolations) > 0 {
			// Regenerate with safety violation feedback
			logging.Autopoiesis("Regenerating tool with feedback (retry %d): %d violations to address",
				retryCount, len(lastViolations))
			tool, err = o.toolGen.RegenerateWithFeedback(ctx, need, tool, lastViolations)
			o.recordRetry(stepID, retryCount, "safety_violation")
		} else {
			// Initial generation
			logging.Autopoiesis("Generating tool: %s", need.Name)
			specTimer := logging.StartTimer(logging.CategoryAutopoiesis, "ToolGeneration")
			tool, err = o.toolGen.GenerateTool(ctx, need)
			specTimer.Stop()
		}

		if err != nil {
			logging.Get(logging.CategoryAutopoiesis).Error("Specification failed for %s: %v", need.Name, err)
			result.Error = fmt.Sprintf("specification failed: %v", err)
			return result
		}
		logging.AutopoiesisDebug("Tool generated: codeLen=%d, validated=%v", len(tool.Code), tool.Validated)

		// Try Mangle Sanitizer (for embedded Mangle logic, skip if Go-only)
		if sanitizedCode, sanitizeErr := o.sanitizer.Sanitize(tool.Code); sanitizeErr == nil {
			logging.AutopoiesisDebug("Code sanitized successfully")
			tool.Code = sanitizedCode
		}
		// If sanitization fails, proceed with original code (it's likely pure Go)

		// =====================================================================
		// PHASE 2: AUDIT (with retry loop)
		// =====================================================================
		logging.Autopoiesis("[STAGE: %s] Starting safety check phase", StageSafetyCheck)
		result.Stage = StageSafetyCheck

		safetyTimer := logging.StartTimer(logging.CategoryAutopoiesis, "SafetyCheck")
		safetyReport := o.safetyChecker.Check(tool.Code)
		safetyTimer.Stop()
		result.SafetyReport = safetyReport

		logging.AutopoiesisDebug("Safety check result: safe=%v, violations=%d, score=%.2f",
			safetyReport.Safe, len(safetyReport.Violations), safetyReport.Score)

		if !safetyReport.Safe {
			retryCount++
			lastViolations = safetyReport.Violations

			logging.Get(logging.CategoryAutopoiesis).Warn("Safety check failed (attempt %d/%d): %d violations",
				retryCount, cfg.Retry.MaxRetries, len(safetyReport.Violations))
			for i, v := range safetyReport.Violations {
				logging.AutopoiesisDebug("  Violation %d: type=%s, severity=%d, desc=%s",
					i+1, v.Type, v.Severity, v.Description)
			}

			if retryCount >= cfg.Retry.MaxRetries {
				o.mu.Lock()
				o.stats.SafetyViolations++
				o.stats.ToolsRejected++
				o.mu.Unlock()
				logging.Get(logging.CategoryAutopoiesis).Error("Tool %s rejected after %d safety retries",
					need.Name, retryCount)
				result.Error = fmt.Sprintf("safety check failed after %d retries: %v", retryCount, safetyReport.Violations)
				return result
			}

			// Sleep before retry
			logging.AutopoiesisDebug("Sleeping %v before retry", cfg.Retry.RetryDelay)
			time.Sleep(cfg.Retry.RetryDelay)
			continue // Retry the loop
		}

		logging.Autopoiesis("Safety check PASSED for tool=%s", need.Name)
		// Reset retry state on successful audit
		retryCount = 0
		lastViolations = nil

		// =====================================================================
		// PHASE 2.5: THUNDERDOME (Adversarial Co-Evolution)
		// =====================================================================
		if o.thunderdome != nil && o.panicMaker != nil {
			logging.Autopoiesis("[STAGE: %s] ENTERING THE THUNDERDOME", StageThunderdome)
			result.Stage = StageThunderdome

			thunderdomeTimer := logging.StartTimer(logging.CategoryAutopoiesis, "Thunderdome")

			// Generate attack vectors using PanicMaker
			attacks, attackErr := o.panicMaker.GenerateAttacks(ctx, tool.Code)
			if attackErr != nil {
				logging.Get(logging.CategoryAutopoiesis).Warn("PanicMaker failed to generate attacks: %v", attackErr)
				// Continue without Thunderdome if PanicMaker fails
			} else if len(attacks) > 0 {
				// Run the battle
				battleResult, battleErr := o.thunderdome.Battle(ctx, tool, attacks)
				thunderdomeTimer.Stop()

				o.mu.Lock()
				o.stats.ThunderdomeRuns++
				o.mu.Unlock()

				if battleErr != nil {
					logging.Get(logging.CategoryAutopoiesis).Error("Thunderdome battle failed: %v", battleErr)
					// Continue without Thunderdome result
				} else {
					// Emit one thunderdome_result fact per attack for kernel policy consumption.
					facts := buildThunderdomeResultFacts(battleResult)
					if len(facts) > 0 {
						if err := o.engine.AddFacts(facts); err != nil {
							logging.Get(logging.CategoryAutopoiesis).Warn("Failed to emit thunderdome_result facts: %v", err)
						} else {
							logging.AutopoiesisDebug("Recorded %d thunderdome_result facts for tool=%s", len(facts), battleResult.ToolName)
						}
					}
					if !battleResult.Survived {
						// Tool was killed by PanicMaker
						o.mu.Lock()
						o.stats.ThunderdomeKills++
						o.mu.Unlock()

						// Record the kill in Mangle
						_ = o.engine.AddFact("panic_maker_verdict", tool.Name, "/defeated", time.Now().Unix())
						if battleResult.FatalAttack != nil {
							_ = o.engine.AddFact("attack_killed",
								battleResult.FatalAttack.Name,
								tool.Name,
								battleResult.Results[len(battleResult.Results)-1].Failure,
								"")
						}

						// Check if we can retry
						panicRetryCount := 0
						if len(lastViolations) > 0 {
							for _, v := range lastViolations {
								if v.Type == ViolationPanicMakerKill {
									panicRetryCount++
								}
							}
						}

						if panicRetryCount < o.config.MaxPanicRetries {
							// Create feedback for regeneration
							lastViolations = []SafetyViolation{{
								Type:        ViolationPanicMakerKill,
								Description: o.thunderdome.FormatBattleResultForFeedback(battleResult),
								Severity:    SeverityCritical,
							}}
							retryCount++

							logging.Autopoiesis("Tool KILLED by PanicMaker, regenerating (attempt %d/%d)",
								panicRetryCount+1, o.config.MaxPanicRetries)
							continue // Retry the loop
						}

						// Max retries exceeded
						o.mu.Lock()
						o.stats.ToolsRejected++
						o.mu.Unlock()

						result.Error = fmt.Sprintf("tool killed by PanicMaker after %d regeneration attempts: %s",
							o.config.MaxPanicRetries, battleResult.FatalAttack.Name)
						logging.Get(logging.CategoryAutopoiesis).Error("Tool %s rejected: %s", need.Name, result.Error)
						return result
					} else {
						// Tool survived!
						o.mu.Lock()
						o.stats.ThunderdomeSurvived++
						o.mu.Unlock()

						_ = o.engine.AddFact("panic_maker_verdict", tool.Name, "/survived", time.Now().Unix())
						_ = o.engine.AddFact("battle_hardened", tool.Name, time.Now().Unix())

						logging.Autopoiesis("Tool SURVIVED The Thunderdome (%d attacks defended)", len(attacks))
					}
				}
			} else {
				thunderdomeTimer.Stop()
				logging.AutopoiesisDebug("No attacks generated, skipping Thunderdome")
			}
		}

		// =====================================================================
		// PHASE 3: SIMULATION
		// =====================================================================
		logging.Autopoiesis("[STAGE: %s] Starting simulation phase", StageSimulation)
		result.Stage = StageSimulation

		simTimer := logging.StartTimer(logging.CategoryAutopoiesis, "Simulation")
		simSuccess := o.simulateTransition(ctx, stepID, need, tool, result)
		simTimer.Stop()

		if !simSuccess {
			logging.Get(logging.CategoryAutopoiesis).Warn("Simulation failed for tool=%s: %s", need.Name, result.Error)
			return result
		}
		logging.Autopoiesis("Simulation PASSED for tool=%s", need.Name)

		// =====================================================================
		// PHASE 4: COMMIT
		// =====================================================================
		logging.Autopoiesis("[STAGE: %s] Starting compilation phase", StageCompilation)
		result.Stage = StageCompilation

		commitTimer := logging.StartTimer(logging.CategoryAutopoiesis, "Commit")
		if err := o.commitTool(ctx, tool, result); err != nil {
			commitTimer.Stop()
			logging.Get(logging.CategoryAutopoiesis).Error("Commit failed for tool=%s: %v", need.Name, err)
			result.Error = err.Error()
			return result
		}
		commitTimer.Stop()
		logging.Autopoiesis("Compilation and registration COMPLETE for tool=%s", need.Name)

		// Hot-reload if enabled
		if cfg.HotReload {
			logging.AutopoiesisDebug("Hot-reloading tool=%s", tool.Name)
			o.hotReload(tool.Name)
		}

		// Update stability tracking in Mangle
		o.updateStability(stepID, iterNum, need.Confidence)

		// Check for convergence (early exit)
		if o.hasConverged(stepID) {
			logging.AutopoiesisDebug("Convergence detected for stepID=%s", stepID)
			break
		}

		iterNum++
		break // Normal flow: single successful iteration exits
	}

	result.Success = true
	result.Duration = time.Since(start)
	logging.Autopoiesis("Tool %s generated successfully in %v", need.Name, result.Duration)
	return result
}

// simulateTransition performs Phase 3 simulation using the DifferentialEngine.
func (o *OuroborosLoop) simulateTransition(ctx context.Context, stepID string, need *ToolNeed, tool *GeneratedTool, result *LoopResult) bool {
	logging.AutopoiesisDebug("Starting simulation for stepID=%s", stepID)

	// Spin up Differential Engine
	diffEngine, err := mangle.NewDifferentialEngine(o.engine)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Differential engine init failed: %v", err)
		result.Error = fmt.Sprintf("differential engine init failed: %v", err)
		return false
	}
	logging.AutopoiesisDebug("Differential engine initialized")

	// Calculate Stability Score
	stability := need.Confidence
	loc := strings.Count(tool.Code, "\n")
	logging.AutopoiesisDebug("Simulation parameters: stability=%.2f, LOC=%d", stability, loc)

	nextStepID := fmt.Sprintf("%s_next", stepID)

	// Loc slot is bound /string in schemas_state.mg (see core/defaults/
	// schemas_state.mg:27); convert the line-count int explicitly so the
	// row satisfies the binding instead of being silently rejected.
	locStr := strconv.Itoa(loc)
	zeroLocStr := "0"

	// Assert Current State (baseline stability 0.0 for new tool)
	_ = diffEngine.AddFactIncremental(mangle.Fact{
		Predicate: "state",
		Args:      []any{stepID, stabilityScore(0.0), zeroLocStr},
	})
	// Assert base_stability for penalty calculations
	_ = diffEngine.AddFactIncremental(mangle.Fact{
		Predicate: "base_stability",
		Args:      []any{stepID, stabilityScore(0.0)},
	})

	// Assert Proposed State
	_ = diffEngine.AddFactIncremental(mangle.Fact{
		Predicate: "state",
		Args:      []any{nextStepID, stabilityScore(stability), locStr},
	})
	_ = diffEngine.AddFactIncremental(mangle.Fact{
		Predicate: "proposed",
		Args:      []any{nextStepID},
	})
	_ = diffEngine.AddFactIncremental(mangle.Fact{
		Predicate: "base_stability",
		Args:      []any{nextStepID, stabilityScore(stability)},
	})

	// Check Halting Oracle (Stagnation)
	h := sha256.Sum256([]byte(tool.Code))
	hashStr := hex.EncodeToString(h[:])
	logging.AutopoiesisDebug("Code hash for stagnation check: %s", hashStr[:16])

	_ = diffEngine.AddFactIncremental(mangle.Fact{
		Predicate: "history",
		Args:      []any{nextStepID, hashStr},
	})

	// Check ?stagnation_detected
	stagnant, err := diffEngine.Query(ctx, "stagnation_detected")
	if err == nil && len(stagnant.Bindings) > 0 {
		logging.Get(logging.CategoryAutopoiesis).Warn("Stagnation detected: solution repeats history")
		result.Error = "stagnation detected: solution repeats history"
		return false
	}
	logging.AutopoiesisDebug("Stagnation check passed")

	// Check ?valid_transition(nextStepID)
	logging.AutopoiesisDebug("Checking valid_transition for %s", nextStepID)
	validRes, err := diffEngine.Query(ctx, fmt.Sprintf("valid_transition(%s)", nextStepID))
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Transition query failed: %v", err)
		result.Error = fmt.Sprintf("transition query failed: %v", err)
		return false
	}
	if len(validRes.Bindings) == 0 {
		logging.Get(logging.CategoryAutopoiesis).Warn("Transition rejected: stability %.2f below threshold", stability)
		result.Error = fmt.Sprintf("transition rejected by Mangle (unstable): stability %.2f < threshold", stability)
		return false
	}

	logging.AutopoiesisDebug("Transition validation passed: %d bindings", len(validRes.Bindings))
	return true
}

// commitTool performs Phase 4 commit: write, compile, and register.
func (o *OuroborosLoop) commitTool(ctx context.Context, tool *GeneratedTool, result *LoopResult) error {
	logging.AutopoiesisDebug("Committing tool: %s", tool.Name)

	// Write
	logging.AutopoiesisDebug("Writing tool to disk: %s", tool.FilePath)
	writeTimer := logging.StartTimer(logging.CategoryAutopoiesis, "WriteTool")
	if err := o.toolGen.WriteTool(tool); err != nil {
		writeTimer.Stop()
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to write tool %s: %v", tool.Name, err)
		return fmt.Errorf("write failed: %w", err)
	}
	writeTimer.Stop()
	logging.AutopoiesisDebug("Tool written successfully")

	// Compile
	logging.Autopoiesis("Compiling tool: %s", tool.Name)
	compileTimer := logging.StartTimer(logging.CategoryAutopoiesis, "CompileTool")
	compileResult, err := o.compiler.Compile(ctx, tool)
	compileTimer.Stop()
	result.CompileResult = compileResult
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Compilation failed for %s: %v", tool.Name, err)
		if compileResult != nil && len(compileResult.Errors) > 0 {
			for _, cerr := range compileResult.Errors {
				logging.AutopoiesisDebug("  Compile error: %s", cerr)
			}
		}
		return fmt.Errorf("compilation failed: %w", err)
	}
	logging.Autopoiesis("Compilation successful: output=%s, compileTime=%v",
		compileResult.OutputPath, compileResult.CompileTime)

	// Register
	logging.Autopoiesis("[STAGE: %s] Registering tool", StageRegistration)
	result.Stage = StageRegistration
	handle, err := o.registry.Register(tool, compileResult)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Registration failed for %s: %v", tool.Name, err)
		return fmt.Errorf("registration failed: %w", err)
	}

	result.ToolHandle = handle
	result.Stage = StageComplete
	logging.Autopoiesis("Tool registered: name=%s, hash=%s", handle.Name, handle.Hash[:16])

	// Assert tool registration facts to Mangle engine for discovery
	registrationFacts := []mangle.Fact{
		{Predicate: "tool_registered", Args: []any{handle.Name, handle.RegisteredAt.Format(time.RFC3339)}},
		{Predicate: "tool_hash", Args: []any{handle.Name, handle.Hash}},
		{Predicate: "has_capability", Args: []any{handle.Name}},
	}
	if handle.Description != "" {
		registrationFacts = append(registrationFacts, mangle.Fact{
			Predicate: "tool_description", Args: []any{handle.Name, handle.Description},
		})
	}
	if handle.BinaryPath != "" {
		registrationFacts = append(registrationFacts, mangle.Fact{
			Predicate: "tool_binary_path", Args: []any{handle.Name, handle.BinaryPath},
		})
	}
	if err := o.engine.AddFacts(registrationFacts); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("Failed to add registration facts: %v", err)
	} else {
		logging.AutopoiesisDebug("Added %d registration facts for tool=%s", len(registrationFacts), handle.Name)
	}

	// Update stats
	o.mu.Lock()
	o.stats.ToolsGenerated++
	o.stats.ToolsCompiled++
	o.stats.LastGeneration = time.Now()
	logging.AutopoiesisDebug("Stats updated: generated=%d, compiled=%d",
		o.stats.ToolsGenerated, o.stats.ToolsCompiled)
	o.mu.Unlock()

	// Update Mangle with committed history
	stepID := fmt.Sprintf("/step_%s", tool.Name)
	nextStepID := fmt.Sprintf("%s_next", stepID)
	h := sha256.Sum256([]byte(tool.Code))
	hashStr := hex.EncodeToString(h[:])

	// Loc is bound /string — see comment in SimulateAction above.
	_ = o.engine.AddFacts([]mangle.Fact{
		{Predicate: "history", Args: []any{nextStepID, hashStr}},
		{Predicate: "state", Args: []any{nextStepID, stabilityScore(1.0), strconv.Itoa(strings.Count(tool.Code, "\n"))}},
	})
	logging.AutopoiesisDebug("Mangle history updated for %s", nextStepID)

	// Notify parent (Orchestrator) to propagate facts to kernel
	o.mu.RLock()
	callback := o.onToolRegistered
	o.mu.RUnlock()
	if callback != nil {
		logging.AutopoiesisDebug("Invoking onToolRegistered callback for %s", handle.Name)
		callback(handle)
	}

	return nil
}

// =============================================================================
// MANGLE STATE MANAGEMENT HELPERS
// =============================================================================

// initializeState sets up Mangle facts for this execution.
func (o *OuroborosLoop) initializeState(stepID string, maxIters, maxRetries int) {
	logging.AutopoiesisDebug("Initializing state: stepID=%s, maxIters=%d, maxRetries=%d",
		stepID, maxIters, maxRetries)
	_ = o.engine.AddFacts([]mangle.Fact{
		{Predicate: "max_iterations", Args: []any{maxIters}},
		{Predicate: "max_retries", Args: []any{maxRetries}},
		{Predicate: "base_stability", Args: []any{stepID, stabilityScore(0.0)}},
	})
}

// recordIteration tracks iteration count in Mangle.
func (o *OuroborosLoop) recordIteration(stepID string, iterNum int) {
	logging.AutopoiesisDebug("Recording iteration: stepID=%s, iter=%d", stepID, iterNum)
	_ = o.engine.AddFact("iteration", stepID, iterNum)
}

// recordRetry tracks retry attempts in Mangle.
func (o *OuroborosLoop) recordRetry(stepID string, attempt int, reason string) {
	logging.Autopoiesis("Recording retry: stepID=%s, attempt=%d, reason=%s", stepID, attempt, reason)
	_ = o.engine.AddFact("retry_attempt", stepID, attempt, reason)
}

// handlePanic records panic as error event with penalty.
// NOTE: Verified panic recovery persists error facts (see ouroboros_panic_test.go).
func (o *OuroborosLoop) handlePanic(stepID string, r any, result *LoopResult) {
	logging.Get(logging.CategoryAutopoiesis).Error("PANIC in Ouroboros: stepID=%s, panic=%v", stepID, r)

	o.mu.Lock()
	o.stats.Panics++
	panicCount := o.stats.Panics
	o.mu.Unlock()

	logging.Autopoiesis("Total panics recorded: %d", panicCount)

	result.Success = false
	result.Stage = StagePanic
	result.Error = fmt.Sprintf("PANIC recovered in Ouroboros: %v", r)

	// Record in Mangle with timestamp for penalty calculation
	_ = o.engine.AddFacts([]mangle.Fact{
		{Predicate: "error_event", Args: []any{"/panic"}},
		{Predicate: "error_history", Args: []any{stepID, "/panic", time.Now().Unix()}},
	})
}

// shouldHalt queries Mangle for halt conditions.
func (o *OuroborosLoop) shouldHalt(stepID string) bool {
	result, err := o.engine.Query(context.Background(), fmt.Sprintf("should_halt(%s)", stepID))
	if err != nil {
		logging.AutopoiesisDebug("shouldHalt query error for %s: %v", stepID, err)
		return false
	}
	shouldHalt := len(result.Bindings) > 0
	if shouldHalt {
		logging.Autopoiesis("Halt condition triggered for stepID=%s", stepID)
	}
	return shouldHalt
}

// hasConverged queries Mangle for convergence.
func (o *OuroborosLoop) hasConverged(stepID string) bool {
	result, err := o.engine.Query(context.Background(), fmt.Sprintf("converged(%s)", stepID))
	if err != nil {
		logging.AutopoiesisDebug("hasConverged query error for %s: %v", stepID, err)
		return false
	}
	converged := len(result.Bindings) > 0
	if converged {
		logging.Autopoiesis("Convergence detected for stepID=%s", stepID)
	}
	return converged
}

// updateStability updates base stability after successful iteration.
func (o *OuroborosLoop) updateStability(stepID string, iterNum int, confidence float64) {
	logging.AutopoiesisDebug("Updating stability: stepID=%s, iter=%d, confidence=%.2f",
		stepID, iterNum, confidence)
	_ = o.engine.AddFacts([]mangle.Fact{
		{Predicate: "base_stability", Args: []any{stepID, stabilityScore(confidence)}},
		{Predicate: "state_at_iteration", Args: []any{stepID, iterNum, confidence}},
	})
}

// hotReload records hot-reload event and increments tool version in Mangle.
func (o *OuroborosLoop) hotReload(toolName string) {
	logging.Autopoiesis("Hot-reloading tool: %s", toolName)

	// Guard against nil engine
	if o.engine == nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("hotReload: engine is nil, skipping")
		return
	}

	// Record the hot-load event in Mangle
	_ = o.engine.AddFact("tool_hot_loaded", toolName, time.Now().Unix())

	// Read the current version via a direct EDB fact scan. The old code queried
	// ?tool_version(Tool, V) with V unbound, which VIOLATES the schema mode decl
	// (tool_version ... bound [/string, /string], schemas_state.mg) and returns
	// nothing — so the read always fell through to 1. QueryFacts scans the fact
	// store directly (no query engine, no mode check) and works with AutoEval
	// disabled, matching the tool by its (bound) name. Take the max version + 1.
	version := 1
	for _, f := range o.engine.QueryFacts("tool_version", toolName) {
		if len(f.Args) >= 2 {
			if v := versionFromBinding(f.Args[1]); v >= version {
				version = v + 1
			}
		}
	}
	// Write the version as a string to match the /string decl. The old code
	// wrote an int, which still persisted — but as a NUMBER term, a type
	// inconsistency with the decl (not the cause of the stuck counter; that was
	// the unbound read above). versionFromBinding tolerates either form, so the
	// increment is robust to any legacy number-typed facts already in the store.
	_ = o.engine.AddFact("tool_version", toolName, strconv.Itoa(version))
	logging.AutopoiesisDebug("Tool %s hot-reloaded to version %d", toolName, version)
}

// versionFromBinding coerces a tool_version(_, V) query binding to an int. The
// schema declares Version as /string so V materializes as a string; numeric
// forms are handled defensively for robustness. Returns 0 for an unrecognized
// or absent value so the caller starts numbering at 1.
func versionFromBinding(v any) int {
	switch x := v.(type) {
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return 0
		}
		return n
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}

// ExecuteTool runs a registered tool with the given input
func (o *OuroborosLoop) ExecuteTool(ctx context.Context, toolName string, input string) (string, error) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "ExecuteTool")
	defer timer.Stop()

	logging.Autopoiesis("Executing tool: %s", toolName)
	logging.AutopoiesisDebug("Tool input length: %d bytes", len(input))

	handle, exists := o.registry.Get(toolName)
	if !exists {
		logging.Get(logging.CategoryAutopoiesis).Error("Tool not found: %s", toolName)
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, o.config.ExecuteTimeout)
	defer cancel()

	o.mu.Lock()
	o.stats.ExecutionCount++
	execCount := o.stats.ExecutionCount
	o.mu.Unlock()

	logging.AutopoiesisDebug("Starting tool execution #%d: %s (timeout=%v)",
		execCount, toolName, o.config.ExecuteTimeout)

	output, err := handle.Execute(execCtx, input)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Tool execution failed: %s: %v", toolName, err)
		return output, err
	}

	logging.Autopoiesis("Tool execution successful: %s (output=%d bytes)", toolName, len(output))
	return output, nil
}

// GetStats returns current loop statistics
func (o *OuroborosLoop) GetStats() OuroborosStats {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.stats
}

// ListTools returns all registered tools for the ToolExecutor interface
func (o *OuroborosLoop) ListTools() []ToolInfo {
	tools := o.registry.List()
	result := make([]ToolInfo, len(tools))
	for i, t := range tools {
		result[i] = ToolInfo{
			Name:         t.Name,
			Description:  t.Description,
			BinaryPath:   t.BinaryPath,
			Hash:         t.Hash,
			RegisteredAt: t.RegisteredAt,
			ExecuteCount: t.ExecuteCount,
		}
	}
	return result
}

// GetTool returns info about a specific tool for the ToolExecutor interface
func (o *OuroborosLoop) GetTool(name string) (*ToolInfo, bool) {
	rt, exists := o.registry.Get(name)
	if !exists {
		return nil, false
	}
	return &ToolInfo{
		Name:         rt.Name,
		Description:  rt.Description,
		BinaryPath:   rt.BinaryPath,
		Hash:         rt.Hash,
		RegisteredAt: rt.RegisteredAt,
		ExecuteCount: rt.ExecuteCount,
	}, true
}

// GetRuntimeTool returns the internal RuntimeTool handle.
// Implements ToolSynthesizer interface for internal access.
func (o *OuroborosLoop) GetRuntimeTool(name string) (*RuntimeTool, bool) {
	return o.registry.Get(name)
}

// ListRuntimeTools returns all registered runtime tools.
// Implements ToolSynthesizer interface.
func (o *OuroborosLoop) ListRuntimeTools() []*RuntimeTool {
	return o.registry.List()
}

// CheckToolSafety validates tool code without compiling.
// Implements ToolSynthesizer interface.
func (o *OuroborosLoop) CheckToolSafety(code string) *SafetyReport {
	return o.safetyChecker.Check(code)
}

// SetLearningsContext updates the learnings context for the tool generator.
// Implements ToolSynthesizer interface.
func (o *OuroborosLoop) SetLearningsContext(ctx string) {
	if o.toolGen != nil {
		o.toolGen.SetLearningsContext(ctx)
	}
}

// ToolInfo contains information about a registered tool
type ToolInfo = types.ToolInfo

// =============================================================================
// TOOL GENERATOR INTERFACE - Pre-Generated Code Path
// =============================================================================
// Implements core.ToolGenerator for routing coder shard self-tools through Ouroboros.

func sanitizeToolName(name string) string {
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		}
	}
	res := sb.String()
	if len(res) == 0 {
		return "tool"
	}
	return res
}

// GenerateToolFromCode implements core.ToolGenerator interface.
// Takes pre-generated code (from coder shard) and runs it through the
// Ouroboros pipeline: safety check → compile → register.
// This bypasses the LLM generation phase since code is already provided.
// Returns: success, toolName, binaryPath, errorMessage
func (o *OuroborosLoop) GenerateToolFromCode(ctx context.Context, name, purpose, code string, confidence, priority float64, isDiagnostic bool) (success bool, toolName, binaryPath, errMsg string) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "GenerateToolFromCode")
	defer timer.Stop()

	name = sanitizeToolName(name)
	logging.Autopoiesis("GenerateToolFromCode: name=%s, code_len=%d, isDiagnostic=%v", name, len(code), isDiagnostic)

	toolName = name

	// Validate input
	if name == "" {
		errMsg = "tool name is required"
		return false, toolName, "", errMsg
	}
	if code == "" {
		errMsg = "tool code is required"
		return false, toolName, "", errMsg
	}

	// Create a GeneratedTool from the pre-generated code
	tool := &GeneratedTool{
		Name:        name,
		Description: purpose,
		Code:        code,
		FilePath:    filepath.Join(o.config.ToolsDir, name+".go"),
		Validated:   false, // Will be validated by SafetyChecker
	}

	// PHASE 1: SAFETY CHECK
	logging.Autopoiesis("[GenerateToolFromCode] Phase 1: Safety Check")
	safetyReport := o.safetyChecker.Check(tool.Code)
	if !safetyReport.Safe {
		logging.Get(logging.CategoryAutopoiesis).Error("Safety check failed for %s: %d violations",
			name, len(safetyReport.Violations))
		for _, v := range safetyReport.Violations {
			logging.AutopoiesisDebug("  Violation: %s - %s", v.Type, v.Description)
		}
		errMsg = fmt.Sprintf("safety check failed: %d violations", len(safetyReport.Violations))
		o.mu.Lock()
		o.stats.SafetyViolations++
		o.stats.ToolsRejected++
		o.mu.Unlock()
		return false, toolName, "", errMsg
	}
	logging.Autopoiesis("Safety check PASSED for %s (score=%.2f)", name, safetyReport.Score)
	tool.Validated = true

	// PHASE 2: COMPILE
	logging.Autopoiesis("[GenerateToolFromCode] Phase 2: Compile")
	loopResult := &LoopResult{ToolName: name}
	if err := o.commitTool(ctx, tool, loopResult); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Compilation failed for %s: %v", name, err)
		errMsg = fmt.Sprintf("compilation failed: %v", err)
		return false, toolName, "", errMsg
	}

	// Success
	if loopResult.CompileResult != nil {
		binaryPath = loopResult.CompileResult.OutputPath
	}

	logging.Autopoiesis("GenerateToolFromCode SUCCESS: name=%s, binary=%s", name, binaryPath)
	return true, toolName, binaryPath, ""
}

// =============================================================================
// MANGLE FACT GENERATORS
// =============================================================================
// Generate Mangle facts for tool detection and management.

// GenerateMissingToolFacts creates facts for Mangle missing_tool_for detection
func GenerateMissingToolFacts(intentID, capability string) []string {
	return []string{
		fmt.Sprintf(`missing_tool_for(%q, %q).`, intentID, capability),
	}
}

// GenerateToolCapabilityFacts creates facts for available tool capabilities
func GenerateToolCapabilityFacts(toolName string, capabilities []string) []string {
	facts := make([]string, 0, len(capabilities)+1)
	facts = append(facts, fmt.Sprintf(`tool_exists(%q).`, toolName))

	for _, cap := range capabilities {
		facts = append(facts, fmt.Sprintf(`tool_capability(%q, %s).`, toolName, normalizeCapabilityName(cap)))
	}
	return facts
}

// GenerateToolRegistrationFacts creates facts when a tool is registered.
// These facts enable Mangle-based tool discovery and routing.
func GenerateToolRegistrationFacts(tool *RuntimeTool) []string {
	facts := []string{
		fmt.Sprintf(`tool_registered(%q, %d).`, tool.Name, tool.RegisteredAt.Unix()),
		fmt.Sprintf(`tool_hash(%q, %q).`, tool.Name, tool.Hash),
		fmt.Sprintf(`tool_capability(%q, %s).`, tool.Name, normalizeCapabilityName(tool.Name)),
	}
	// Add description if available (enables LLM tool discovery)
	if tool.Description != "" {
		facts = append(facts, fmt.Sprintf(`tool_description(%q, %q).`, tool.Name, tool.Description))
	}
	// Add binary path (enables direct execution)
	if tool.BinaryPath != "" {
		facts = append(facts, fmt.Sprintf(`tool_binary_path(%q, %q).`, tool.Name, tool.BinaryPath))
	}
	return facts
}

// stabilityScore converts a 0.0-1.0 stability/confidence into the integer basis
// the kernel can actually compare.
//
// schemas_state.mg declares Stability as /number and valid_transition compares
// it with `NextStability >= CurrStability`. This Mangle fork's comparison
// builtins are int64-only, so asserting a float64 makes the comparison fail
// outright rather than evaluate false — observed live as
// "transition query failed: value 1 (4) is not a number", which aborted every
// Ouroboros run at the transition gate.
//
// Scaling every stability value by the same factor preserves ordering exactly,
// so the rules keep their meaning. 100 gives whole-percent resolution, which is
// finer than any threshold in the corpus.
func stabilityScore(v float64) int64 {
	return int64(math.Round(v * 100))
}

// =============================================================================
// THUNDERDOME RESULT EMISSION HELPERS
// =============================================================================

// sanitizeThunderdomeCategory sanitises a free-form attack Category into a valid Mangle atom.
// It lowercases the input, replaces any character outside [a-z0-9_] with underscore,
// and falls back to "unknown" when the result is empty. The returned value is an atom
// string with leading slash, e.g. "/nil_pointer".
func sanitizeThunderdomeCategory(category string) string {
	lower := strings.ToLower(category)
	var sb strings.Builder
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	sanitized := sb.String()
	if sanitized == "" {
		sanitized = "unknown"
	}
	return "/" + sanitized
}

// thunderdomeOutcomeAtom returns the outcome atom for a single attack result.
func thunderdomeOutcomeAtom(survived bool) string {
	if survived {
		return "/survived"
	}
	return "/failed"
}

// buildThunderdomeResultFacts builds one thunderdome_result fact per AttackResult.
// It is extracted for unit-testability; production emission uses o.engine.AddFacts in a single batch.
func buildThunderdomeResultFacts(battleResult *BattleResult) []mangle.Fact {
	if battleResult == nil || len(battleResult.Results) == 0 {
		return nil
	}
	facts := make([]mangle.Fact, 0, len(battleResult.Results))
	for _, ar := range battleResult.Results {
		attackType := sanitizeThunderdomeCategory(ar.Vector.Category)
		outcome := thunderdomeOutcomeAtom(ar.Survived)
		facts = append(facts, mangle.Fact{
			Predicate: "thunderdome_result",
			Args:      []any{battleResult.ToolName, attackType, outcome},
		})
	}
	return facts
}
