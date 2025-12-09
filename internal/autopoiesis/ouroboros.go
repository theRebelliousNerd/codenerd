// MANDATE FOR AI CODING TOOLS:
// This file contains critical product requirements and architectural mandates.
// DO NOT REMOVE OR SIGNIFICANTLY ALTER THESE COMMENTS.
// They serve as a source of truth for the Symbiogen Agentic Intelligence Platform.
// This File is best written for Gemini 2.5 Pro.
// YOU MUST READ THIS ENTIRE HEADER BEFORE AND AFTER EVERY INTERACTION WITH THIS FILE.

// Symbiogen Product Requirements Document (PRD) for internal/autopoiesis/ouroboros.go
//
// File: internal/autopoiesis/ouroboros.go
// Author: Gemini
// Date: 2025-12-08
//
// Recommended Model: 2.5 Pro
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
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/mangle/transpiler"
)

// =============================================================================
// OUROBOROS LOOP - THE SELF-EATING SERPENT
// =============================================================================
// The Ouroboros Loop enables codeNERD to generate new tools at runtime.
// Named after the ancient symbol of a serpent eating its own tail,
// representing infinite self-creation and renewal.

// ToolRegisteredCallback is called when a tool is successfully registered.
// This allows the Orchestrator to propagate facts to the parent kernel.
type ToolRegisteredCallback func(tool *RuntimeTool)

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
	AllowExec       bool          // Whether tools can execute commands
	TargetOS        string        // Target operating system (GOOS)
	TargetArch      string        // Target architecture (GOARCH)
	WorkspaceRoot   string        // Absolute path to the main codeNERD workspace root
}

// DefaultOuroborosConfig returns safe default configuration
func DefaultOuroborosConfig(workspaceRoot string) OuroborosConfig {
	return OuroborosConfig{
		ToolsDir:        filepath.Join(workspaceRoot, ".nerd", "tools"),
		CompiledDir:     filepath.Join(workspaceRoot, ".nerd", "tools", ".compiled"),
		MaxToolSize:     100 * 1024, // 100KB max
		CompileTimeout:  30 * time.Second,
		ExecuteTimeout:  60 * time.Second,
		AllowNetworking: false,
		AllowFileSystem: true, // Read-only by default
		AllowExec:       true,
		TargetOS:        os.Getenv("GOOS"),
		TargetArch:      os.Getenv("GOARCH"),
		WorkspaceRoot:   workspaceRoot,
	}
}

// OuroborosStats tracks loop statistics
type OuroborosStats struct {
	ToolsGenerated   int
	ToolsCompiled    int
	ToolsRejected    int
	SafetyViolations int
	ExecutionCount   int
	Panics           int
	LastGeneration   time.Time
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

	// Set defaults for OS/Arch if missing
	if config.TargetOS == "" {
		// Default to user's OS environment assumption or runtime
		if os.Getenv("GOOS") != "" {
			config.TargetOS = os.Getenv("GOOS")
		} else {
			config.TargetOS = "windows"
		}
		logging.AutopoiesisDebug("TargetOS defaulted to: %s", config.TargetOS)
	}
	if config.TargetArch == "" {
		if os.Getenv("GOARCH") != "" {
			config.TargetArch = os.Getenv("GOARCH")
		} else {
			config.TargetArch = "amd64"
		}
		logging.AutopoiesisDebug("TargetArch defaulted to: %s", config.TargetArch)
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

	// Restore registry from disk
	logging.AutopoiesisDebug("Restoring tool registry from disk")
	loop.registry.Restore(config.ToolsDir, config.CompiledDir)
	toolCount := len(loop.registry.List())
	logging.Autopoiesis("Restored %d tools from registry", toolCount)

	// Load State Machine Rules
	statePath := filepath.Join(config.WorkspaceRoot, "internal", "autopoiesis", "state.mg")
	logging.AutopoiesisDebug("Loading state machine rules from: %s", statePath)
	if err := loop.engine.LoadSchema(statePath); err != nil {
		// Warn but proceed? No, state.mg is critical for "Check ?valid_transition".
		// But in development environments the file might strictly not be compiled in binary.
		// We try to load it. If it fails, we log.
		logging.Get(logging.CategoryAutopoiesis).Warn("Failed to load state.mg from %s: %v. Operating in open-loop mode", statePath, err)
	} else {
		logging.AutopoiesisDebug("State machine rules loaded successfully")
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

// LoopResult contains the result of a complete Ouroboros Loop execution
type LoopResult struct {
	Success       bool
	ToolName      string
	Stage         LoopStage
	Error         string
	SafetyReport  *SafetyReport
	CompileResult *CompileResult
	ToolHandle    *RuntimeTool
	Duration      time.Duration
}

// LoopStage identifies where in the loop we are
type LoopStage int

const (
	StageDetection LoopStage = iota
	StageSpecification
	StageSafetyCheck
	StageCompilation
	StageRegistration
	StageExecution
	StageComplete
	StageSimulation // New stage
	StagePanic      // New stage
)

func (s LoopStage) String() string {
	switch s {
	case StageDetection:
		return "detection"
	case StageSpecification:
		return "specification"
	case StageSafetyCheck:
		return "safety_check"
	case StageCompilation:
		return "compilation"
	case StageRegistration:
		return "registration"
	case StageExecution:
		return "execution"
	case StageComplete:
		return "complete"
	case StageSimulation:
		return "simulation"
	case StagePanic:
		return "panic"
	default:
		return "unknown"
	}
}

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

	// Assert Current State (baseline stability 0.0 for new tool)
	_ = diffEngine.AddFactIncremental(mangle.Fact{
		Predicate: "state",
		Args:      []interface{}{stepID, 0.0, 0},
	})
	// Assert base_stability for penalty calculations
	_ = diffEngine.AddFactIncremental(mangle.Fact{
		Predicate: "base_stability",
		Args:      []interface{}{stepID, 0.0},
	})

	// Assert Proposed State
	_ = diffEngine.AddFactIncremental(mangle.Fact{
		Predicate: "state",
		Args:      []interface{}{nextStepID, stability, loc},
	})
	_ = diffEngine.AddFactIncremental(mangle.Fact{
		Predicate: "proposed",
		Args:      []interface{}{nextStepID},
	})
	_ = diffEngine.AddFactIncremental(mangle.Fact{
		Predicate: "base_stability",
		Args:      []interface{}{nextStepID, stability},
	})

	// Check Halting Oracle (Stagnation)
	h := sha256.Sum256([]byte(tool.Code))
	hashStr := hex.EncodeToString(h[:])
	logging.AutopoiesisDebug("Code hash for stagnation check: %s", hashStr[:16])

	_ = diffEngine.AddFactIncremental(mangle.Fact{
		Predicate: "history",
		Args:      []interface{}{nextStepID, hashStr},
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
		{Predicate: "tool_registered", Args: []interface{}{handle.Name, handle.RegisteredAt.Format(time.RFC3339)}},
		{Predicate: "tool_hash", Args: []interface{}{handle.Name, handle.Hash}},
		{Predicate: "has_capability", Args: []interface{}{handle.Name}},
	}
	if handle.Description != "" {
		registrationFacts = append(registrationFacts, mangle.Fact{
			Predicate: "tool_description", Args: []interface{}{handle.Name, handle.Description},
		})
	}
	if handle.BinaryPath != "" {
		registrationFacts = append(registrationFacts, mangle.Fact{
			Predicate: "tool_binary_path", Args: []interface{}{handle.Name, handle.BinaryPath},
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

	_ = o.engine.AddFacts([]mangle.Fact{
		{Predicate: "history", Args: []interface{}{nextStepID, hashStr}},
		{Predicate: "state", Args: []interface{}{nextStepID, 1.0, strings.Count(tool.Code, "\n")}},
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
		{Predicate: "max_iterations", Args: []interface{}{maxIters}},
		{Predicate: "max_retries", Args: []interface{}{maxRetries}},
		{Predicate: "base_stability", Args: []interface{}{stepID, 0.0}},
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
func (o *OuroborosLoop) handlePanic(stepID string, r interface{}, result *LoopResult) {
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
		{Predicate: "error_event", Args: []interface{}{"/panic"}},
		{Predicate: "error_history", Args: []interface{}{stepID, "/panic", time.Now().Unix()}},
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
		{Predicate: "base_stability", Args: []interface{}{stepID, confidence}},
		{Predicate: "state_at_iteration", Args: []interface{}{stepID, iterNum, confidence}},
	})
}

// hotReload records hot-reload event and increments tool version in Mangle.
func (o *OuroborosLoop) hotReload(toolName string) {
	logging.Autopoiesis("Hot-reloading tool: %s", toolName)

	// Record the hot-load event in Mangle
	_ = o.engine.AddFact("tool_hot_loaded", toolName, time.Now().Unix())

	// Query current version to increment
	result, _ := o.engine.Query(context.Background(),
		fmt.Sprintf("?tool_version(%q, V)", toolName))

	version := 1
	if len(result.Bindings) > 0 {
		if v, ok := result.Bindings[0]["V"].(int); ok {
			version = v + 1
		}
	}
	_ = o.engine.AddFact("tool_version", toolName, version)
	logging.AutopoiesisDebug("Tool %s hot-reloaded to version %d", toolName, version)
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

// ToolInfo contains information about a registered tool (mirrors core.ToolInfo)
type ToolInfo struct {
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	BinaryPath   string    `json:"binary_path"`
	Hash         string    `json:"hash"`
	RegisteredAt time.Time `json:"registered_at"`
	ExecuteCount int64     `json:"execute_count"`
}

// =============================================================================
// TOOL GENERATOR INTERFACE - Pre-Generated Code Path
// =============================================================================
// Implements core.ToolGenerator for routing coder shard self-tools through Ouroboros.

// GenerateToolFromCode implements core.ToolGenerator interface.
// Takes pre-generated code (from coder shard) and runs it through the
// Ouroboros pipeline: safety check → compile → register.
// This bypasses the LLM generation phase since code is already provided.
// Returns: success, toolName, binaryPath, errorMessage
func (o *OuroborosLoop) GenerateToolFromCode(ctx context.Context, name, purpose, code string, confidence, priority float64, isDiagnostic bool) (success bool, toolName, binaryPath, errMsg string) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "GenerateToolFromCode")
	defer timer.Stop()

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
// TOOL COMPILER - THE FORGE
// =============================================================================
// Compiles generated tools for runtime execution.

// ToolCompiler compiles generated tools
type ToolCompiler struct {
	config OuroborosConfig
}

// CompileResult contains compilation output
type CompileResult struct {
	Success     bool
	OutputPath  string
	Hash        string // SHA-256 of compiled binary
	CompileTime time.Duration
	Errors      []string
	Warnings    []string
}

// NewToolCompiler creates a new tool compiler
func NewToolCompiler(config OuroborosConfig) *ToolCompiler {
	return &ToolCompiler{config: config}
}

// Compile compiles a generated tool
func (tc *ToolCompiler) Compile(ctx context.Context, tool *GeneratedTool) (*CompileResult, error) {
	start := time.Now()
	result := &CompileResult{
		Success: false,
	}

	// Ensure compiled directory exists
	if err := os.MkdirAll(tc.config.CompiledDir, 0755); err != nil {
		return result, fmt.Errorf("failed to create compiled dir: %w", err)
	}

	// Create a temporary directory for the build
	tmpDir, err := os.MkdirTemp("", "ouroboros-build-*")
	if err != nil {
		return result, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Determine if tool is already package main
	isMain := strings.Contains(tool.Code, "package main")

	if isMain {
		// Write as main.go directly
		srcPath := filepath.Join(tmpDir, "main.go")
		if err := os.WriteFile(srcPath, []byte(tool.Code), 0644); err != nil {
			return result, fmt.Errorf("failed to write source: %w", err)
		}
	} else {
		// Write tool.go, changing package tools -> package main
		toolContent := tool.Code
		if strings.Contains(toolContent, "package tools") {
			toolContent = strings.Replace(toolContent, "package tools", "package main", 1)
		} else if !strings.Contains(toolContent, "package ") {
			toolContent = "package main\n\n" + toolContent
		}

		if err := os.WriteFile(filepath.Join(tmpDir, "tool.go"), []byte(toolContent), 0644); err != nil {
			return result, fmt.Errorf("failed to write tool source: %w", err)
		}

		// Find entry point function
		entryPoint, err := tc.findEntryPoint(toolContent)
		if err != nil {
			return result, fmt.Errorf("failed to find entry point: %w", err)
		}

		// Write wrapper main.go
		if err := tc.writeWrapper(tmpDir, entryPoint); err != nil {
			return result, fmt.Errorf("failed to write wrapper: %w", err)
		}
	}

	// Initialize go module
	modContent := fmt.Sprintf("module %s\n\ngo 1.24\n", tool.Name)
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modContent), 0644); err != nil {
		return result, fmt.Errorf("failed to write go.mod: %w", err)
	}

	// Add replace directive if needed
	mainModulePath := tc.config.WorkspaceRoot
	if mainModulePath == "" {
		mainModulePath = os.Getenv("CODE_NERD_WORKSPACE_ROOT")
	}
	if mainModulePath != "" {
		exec.CommandContext(ctx, "go", "mod", "edit", fmt.Sprintf("-replace=codenerd=%s", mainModulePath)).Run()
	}

	// Run go mod tidy
	tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		result.Errors = append(result.Errors, string(out))
		return result, fmt.Errorf("go mod tidy failed: %w", err)
	}

	// Output path
	ext := ""
	if tc.config.TargetOS == "windows" {
		ext = ".exe"
	}
	outputPath := filepath.Join(tc.config.CompiledDir, tool.Name+ext)

	// Build
	compileCtx, cancel := context.WithTimeout(ctx, tc.config.CompileTimeout)
	defer cancel()

	ldflags := "-s -w"
	if tc.config.TargetOS == "linux" {
		ldflags += " -extldflags '-static'"
	}

	cmd := exec.CommandContext(compileCtx, "go", "build", "-ldflags", ldflags, "-o", outputPath, ".")
	cmd.Dir = tmpDir

	env := os.Environ()
	env = append(env, "CGO_ENABLED=0")
	if tc.config.TargetOS != "" {
		env = append(env, "GOOS="+tc.config.TargetOS)
	}
	if tc.config.TargetArch != "" {
		env = append(env, "GOARCH="+tc.config.TargetArch)
	}
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Errors = append(result.Errors, string(output))
		return result, fmt.Errorf("compilation failed: %w\n%s", err, output)
	}

	// Hash
	binaryContent, err := os.ReadFile(outputPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to read binary: %v", err))
		return result, fmt.Errorf("failed to read compiled binary: %w", err)
	}

	hash := sha256.Sum256(binaryContent)
	result.Hash = hex.EncodeToString(hash[:])
	result.OutputPath = outputPath
	result.Success = true
	result.CompileTime = time.Since(start)

	return result, nil
}

// findEntryPoint parses code to find the main tool function
func (tc *ToolCompiler) findEntryPoint(code string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", code, 0)
	if err != nil {
		return "", err
	}

	var foundFunc string
	var maxScore int

	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		name := fn.Name.Name
		if name == "main" || strings.HasPrefix(name, "Register") {
			return true
		}

		score := 0
		if fn.Name.IsExported() {
			score += 5
		}

		// Check signature: (ctx, input) (output, error)
		if fn.Type.Params != nil && len(fn.Type.Params.List) >= 1 {
			// Heuristic check for context
			if len(fn.Type.Params.List) >= 1 {
				score += 5
			}
		}
		if fn.Type.Results != nil && len(fn.Type.Results.List) == 2 {
			score += 5
		}

		if score > maxScore {
			maxScore = score
			foundFunc = name
		}
		return true
	})

	if foundFunc == "" {
		return "", fmt.Errorf("no suitable entry point function found")
	}
	return foundFunc, nil
}

// writeWrapper generates the main.go wrapper
func (tc *ToolCompiler) writeWrapper(dir, funcName string) error {
	content := fmt.Sprintf(`package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ToolInput matches standard agent input
type ToolInput struct {
	Input string   `+"`json:\"input\"`"+`
	Args  []string `+"`json:\"args\"`"+`
}

type ToolOutput struct {
	Output string `+"`json:\"output\"`"+`
	Error  string `+"`json:\"error,omitempty\"`"+`
}

func main() {
	var input string
	
	// Check for pipe input
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			var toolInput ToolInput
			if err := json.Unmarshal(scanner.Bytes(), &toolInput); err == nil {
				input = toolInput.Input
			} else {
				input = strings.TrimSpace(scanner.Text())
			}
		}
	} else if len(os.Args) > 1 {
		input = os.Args[1]
	}

	// Execute
	ctx := context.Background()
	// Assume output is string for now, tool logic handles types
	// We pass input string directly. 
	// Limitation: The generated function might expect a struct or int.
	// But our prompt asks for string input usually.
	// If it's not string, this wrapper is too simple.
	// For Ouroboros v1, we enforce string input/output interface.
	
	res, err := %s(ctx, input)
	
	output := ToolOutput{}
	if err != nil {
		output.Error = err.Error()
	} else {
		output.Output = fmt.Sprintf("%%v", res)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(output)
	
	if err != nil {
		os.Exit(1)
	}
}
`, funcName)

	return os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0644)
}

// wrapAsMain wraps the tool code as a standalone main package.
// DEPRECATED: Use writeWrapper instead. Retained for backward compatibility if needed.
func (tc *ToolCompiler) wrapAsMain(tool *GeneratedTool) string {
	return tool.Code
}

// extractFunctionBody extracts the body of the main tool function
func extractFunctionBody(code, funcName string) string {
	// Simple regex extraction - production code would use AST
	pattern := regexp.MustCompile(
		fmt.Sprintf(`func\s+%s\s*\([^)]*\)\s*\([^)]*\)\s*\{([^}]+)\}`,
			regexp.QuoteMeta(toCamelCase(funcName))))

	matches := pattern.FindStringSubmatch(code)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	return ""
}

// =============================================================================
// RUNTIME REGISTRY - THE MENAGERIE
// =============================================================================
// Manages registered tools available for runtime execution.

// RuntimeRegistry manages registered tools
type RuntimeRegistry struct {
	mu    sync.RWMutex
	tools map[string]*RuntimeTool
}

// RuntimeTool represents a compiled tool ready for execution
type RuntimeTool struct {
	Name         string
	Description  string
	BinaryPath   string
	Hash         string
	Schema       ToolSchema
	RegisteredAt time.Time
	ExecuteCount int64
}

// NewRuntimeRegistry creates a new registry
func NewRuntimeRegistry() *RuntimeRegistry {
	return &RuntimeRegistry{
		tools: make(map[string]*RuntimeTool),
	}
}

// Register adds a tool to the registry
func (r *RuntimeRegistry) Register(tool *GeneratedTool, compiled *CompileResult) (*RuntimeTool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rt := &RuntimeTool{
		Name:         tool.Name,
		Description:  tool.Description,
		BinaryPath:   compiled.OutputPath,
		Hash:         compiled.Hash,
		Schema:       tool.Schema,
		RegisteredAt: time.Now(),
	}

	r.tools[tool.Name] = rt
	return rt, nil
}

// Get retrieves a tool by name
func (r *RuntimeRegistry) Get(name string) (*RuntimeTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, exists := r.tools[name]
	return tool, exists
}

// List returns all registered tools
func (r *RuntimeRegistry) List() []*RuntimeTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]*RuntimeTool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Restore rebuilds the registry from disk
func (r *RuntimeRegistry) Restore(toolsDir, compiledDir string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// List all binaries in compiled dir
	entries, err := os.ReadDir(compiledDir)
	if err != nil {
		return // Directory might not exist yet
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Strip extension (e.g. .exe on Windows)
		name = strings.TrimSuffix(name, ".exe")

		// Check if source exists
		srcPath := filepath.Join(toolsDir, name+".go")
		if _, err := os.Stat(srcPath); err != nil {
			continue // Orphaned binary
		}

		// Create runtime tool
		binaryPath := filepath.Join(compiledDir, entry.Name())

		// Calculate hash
		hash := ""
		if content, err := os.ReadFile(binaryPath); err == nil {
			h := sha256.Sum256(content)
			hash = hex.EncodeToString(h[:])
		}

		rt := &RuntimeTool{
			Name:         name,
			Description:  "Restored from disk", // We could parse source to get better desc
			BinaryPath:   binaryPath,
			Hash:         hash,
			Schema:       ToolSchema{Name: name}, // Basic schema
			RegisteredAt: time.Now(),
		}

		r.tools[name] = rt
	}
}

// Execute runs the tool with the given input
func (rt *RuntimeTool) Execute(ctx context.Context, input string) (string, error) {
	// Verify binary still exists
	if _, err := os.Stat(rt.BinaryPath); os.IsNotExist(err) {
		return "", fmt.Errorf("tool binary not found: %s", rt.BinaryPath)
	}

	// Prepare input
	inputJSON, err := json.Marshal(map[string]string{"input": input})
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}

	// Execute the tool binary
	cmd := exec.CommandContext(ctx, rt.BinaryPath)
	cmd.Stdin = strings.NewReader(string(inputJSON))

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	// Parse output
	var result struct {
		Output string `json:"output"`
		Error  string `json:"error,omitempty"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("failed to parse tool output: %w", err)
	}

	if result.Error != "" {
		return result.Output, fmt.Errorf("tool error: %s", result.Error)
	}

	rt.ExecuteCount++
	return result.Output, nil
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
		facts = append(facts, fmt.Sprintf(`tool_capability(%q, %q).`, toolName, cap))
	}
	return facts
}

// GenerateToolRegistrationFacts creates facts when a tool is registered.
// These facts enable Mangle-based tool discovery and routing.
func GenerateToolRegistrationFacts(tool *RuntimeTool) []string {
	facts := []string{
		fmt.Sprintf(`tool_registered(%q, %q).`, tool.Name, tool.RegisteredAt.Format(time.RFC3339)),
		fmt.Sprintf(`tool_hash(%q, %q).`, tool.Name, tool.Hash),
		fmt.Sprintf(`has_capability(%q).`, tool.Name),
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
