package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

var (
	// Regex patterns for parsing test output.
	// Compiled once at package level for performance.
	goFailRegex    = regexp.MustCompile(`--- FAIL: (\w+)`)
	goErrorRegex   = regexp.MustCompile(`^(.+\.go):(\d+):(\d+): (.+)$`)
	pyErrorRegex   = regexp.MustCompile(`File "(.+)", line (\d+), in (.+)`)
	rustErrorRegex = regexp.MustCompile(`error\[(E\d+)\]: (.+)`)
	rustLocRegex   = regexp.MustCompile(`\s+--> (.+):(\d+):(\d+)`)
)

// TDDState represents the current state of the TDD repair loop.
type TDDState string

const (
	TDDStateIdle         TDDState = "idle"
	TDDStateRunning      TDDState = "running_tests"
	TDDStatePassing      TDDState = "passing"
	TDDStateFailing      TDDState = "failing"
	TDDStateAnalyzing    TDDState = "analyzing"
	TDDStateGenerating   TDDState = "generating_patch"
	TDDStateApplying     TDDState = "applying_patch"
	TDDStateEscalated    TDDState = "escalated"
	TDDStateCompiling    TDDState = "compiling"
	TDDStateCompileError TDDState = "compile_error"
)

// TDDAction represents an action in the TDD loop.
type TDDAction string

const (
	TDDActionRunTests      TDDAction = "run_tests"
	TDDActionReadErrorLog  TDDAction = "read_error_log"
	TDDActionAnalyzeRoot   TDDAction = "analyze_root_cause"
	TDDActionGeneratePatch TDDAction = "generate_patch"
	TDDActionApplyPatch    TDDAction = "apply_patch"
	TDDActionBuild         TDDAction = "build"
	TDDActionEscalate      TDDAction = "escalate_to_user"
	TDDActionComplete      TDDAction = "complete"
)

// Diagnostic represents a compiler/test error.
type Diagnostic struct {
	Severity  string // error, warning
	FilePath  string
	Line      int
	Column    int
	Code      string // Error code (e.g., E0308 for Rust)
	Message   string
	RawOutput string
}

// ToFact converts a diagnostic to a Mangle fact.
func (d Diagnostic) ToFact() Fact {
	return Fact{
		Predicate: "diagnostic",
		Args: []interface{}{
			"/" + d.Severity,
			d.FilePath,
			int64(d.Line),
			d.Code,
			d.Message,
		},
	}
}

// Patch represents a code change to be applied.
type Patch struct {
	FilePath   string
	OldContent string
	NewContent string
	Rationale  string
}

// ToFact converts a patch to a Mangle fact.
func (p Patch) ToFact() Fact {
	return Fact{
		Predicate: "patch",
		Args: []interface{}{
			p.FilePath,
			p.OldContent,
			p.NewContent,
			p.Rationale,
		},
	}
}

// TDDLoopConfig holds configuration for the TDD loop.
type TDDLoopConfig struct {
	MaxRetries   int
	TestCommand  string
	BuildCommand string
	TestTimeout  time.Duration
	BuildTimeout time.Duration
	WorkingDir   string
}

// DefaultTDDLoopConfig returns sensible defaults.
func DefaultTDDLoopConfig() TDDLoopConfig {
	return TDDLoopConfig{
		MaxRetries:   3,
		TestCommand:  "go test ./...",
		BuildCommand: "go build ./...",
		TestTimeout:  15 * time.Minute,
		BuildTimeout: 10 * time.Minute,
		WorkingDir:   ".",
	}
}

// TDDLoop implements the TDD repair loop state machine.
// Based on Cortex 1.5.0 §3.2 TDD Repair Loop (OODA Loop)
type TDDLoop struct {
	mu sync.RWMutex

	// Current state
	state      TDDState
	retryCount int
	maxRetries int

	// Configuration
	config TDDLoopConfig

	// Dependencies
	virtualStore *VirtualStore
	kernel       Kernel
	llmClient    LLMClient

	// State data
	diagnostics []Diagnostic
	lastOutput  string
	patches     []Patch
	hypothesis  string

	// State history for debugging
	history []TDDStateTransition
}

// TDDStateTransition records a state transition.
type TDDStateTransition struct {
	FromState TDDState
	ToState   TDDState
	Action    TDDAction
	Timestamp time.Time
	Metadata  map[string]interface{}
}

// NewTDDLoop creates a new TDD loop with default configuration.
func NewTDDLoop(vs *VirtualStore, kernel Kernel, llmClient LLMClient) *TDDLoop {
	return NewTDDLoopWithConfig(vs, kernel, llmClient, DefaultTDDLoopConfig())
}

// NewTDDLoopWithConfig creates a new TDD loop with custom configuration.
func NewTDDLoopWithConfig(vs *VirtualStore, kernel Kernel, llmClient LLMClient, config TDDLoopConfig) *TDDLoop {
	return &TDDLoop{
		state:        TDDStateIdle,
		retryCount:   0,
		maxRetries:   config.MaxRetries,
		config:       config,
		virtualStore: vs,
		kernel:       kernel,
		llmClient:    llmClient,
		diagnostics:  make([]Diagnostic, 0),
		patches:      make([]Patch, 0),
		history:      make([]TDDStateTransition, 0),
	}
}

// GetState returns the current state.
func (t *TDDLoop) GetState() TDDState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// GetRetryCount returns the current retry count.
func (t *TDDLoop) GetRetryCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.retryCount
}

// GetDiagnostics returns the current diagnostics.
func (t *TDDLoop) GetDiagnostics() []Diagnostic {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]Diagnostic{}, t.diagnostics...)
}

// GetHistory returns the state transition history.
func (t *TDDLoop) GetHistory() []TDDStateTransition {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]TDDStateTransition{}, t.history...)
}

// transition records a state transition.
func (t *TDDLoop) transition(newState TDDState, action TDDAction, meta map[string]interface{}) {
	t.history = append(t.history, TDDStateTransition{
		FromState: t.state,
		ToState:   newState,
		Action:    action,
		Timestamp: time.Now(),
		Metadata:  meta,
	})
	t.state = newState

	// Inject state into kernel for logic-driven decisions
	if t.kernel != nil {
		// Use transaction to batch retract+assert into a single rebuild
		tx := types.NewKernelTx(t.kernel)
		tx.Retract("test_state")
		tx.Retract("retry_count")
		tx.Assert(Fact{
			Predicate: "test_state",
			Args:      []interface{}{"/" + string(newState)},
		})
		tx.Assert(Fact{
			Predicate: "retry_count",
			Args:      []interface{}{int64(t.retryCount)},
		})
		if err := tx.Commit(); err != nil {
			logging.Get(logging.CategoryKernel).Warn("Failed to commit TDD state transition: %v", err)
		}
	}
}

// NextAction determines the next action based on the current state.
func (t *TDDLoop) NextAction() TDDAction {
	t.mu.RLock()
	defer t.mu.RUnlock()

	switch t.state {
	case TDDStateIdle:
		return TDDActionRunTests

	case TDDStateFailing:
		if t.retryCount < t.maxRetries {
			return TDDActionReadErrorLog
		}
		return TDDActionEscalate

	case TDDStateAnalyzing:
		return TDDActionAnalyzeRoot

	case TDDStateGenerating:
		return TDDActionGeneratePatch

	case TDDStateApplying:
		return TDDActionApplyPatch

	case TDDStateCompiling:
		return TDDActionBuild

	case TDDStateCompileError:
		if t.retryCount < t.maxRetries {
			return TDDActionAnalyzeRoot
		}
		return TDDActionEscalate

	case TDDStatePassing:
		return TDDActionComplete

	case TDDStateEscalated:
		return TDDActionComplete

	default:
		return TDDActionRunTests
	}
}

// Run executes a single step of the TDD loop.
func (t *TDDLoop) Run(ctx context.Context) error {
	action := t.NextAction()

	switch action {
	case TDDActionRunTests:
		return t.runTests(ctx)
	case TDDActionReadErrorLog:
		return t.readErrorLog(ctx)
	case TDDActionAnalyzeRoot:
		return t.analyzeRootCause(ctx)
	case TDDActionGeneratePatch:
		return t.generatePatch(ctx)
	case TDDActionApplyPatch:
		return t.applyPatch(ctx)
	case TDDActionBuild:
		return t.build(ctx)
	case TDDActionEscalate:
		return t.escalate(ctx)
	case TDDActionComplete:
		return nil
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

// RunToCompletion runs the TDD loop until completion or escalation.
func (t *TDDLoop) RunToCompletion(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		state := t.GetState()
		if state == TDDStatePassing || state == TDDStateEscalated {
			return nil
		}

		if err := t.Run(ctx); err != nil {
			return fmt.Errorf("TDD loop error in state %s: %w", state, err)
		}
	}
}

// Reset resets the TDD loop to initial state.
func (t *TDDLoop) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.state = TDDStateIdle
	t.retryCount = 0
	t.diagnostics = make([]Diagnostic, 0)
	t.patches = make([]Patch, 0)
	t.lastOutput = ""
	t.hypothesis = ""
	// Keep history for debugging
}

// runTests executes the test suite.
func (t *TDDLoop) runTests(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.transition(TDDStateRunning, TDDActionRunTests, nil)

	// Execute tests via VirtualStore
	// BUG FIX: Action facts require 3+ args (ActionID, Type, Target)
	action := Fact{
		Predicate: "next_action",
		Args: []interface{}{
			fmt.Sprintf("tdd-test-%d", time.Now().UnixNano()),
			"/run_tests",
			t.config.TestCommand,
		},
	}

	output, err := t.virtualStore.RouteAction(ctx, action)
	t.lastOutput = output

	if err != nil || strings.Contains(output, "FAIL") || strings.Contains(output, "error") || strings.Contains(output, "FAILED") {
		t.retryCount++
		t.diagnostics = t.parseTestOutput(output)
		t.transition(TDDStateFailing, TDDActionRunTests, map[string]interface{}{
			"error_count": len(t.diagnostics),
			"retry":       t.retryCount,
		})
		return nil
	}

	t.transition(TDDStatePassing, TDDActionRunTests, nil)
	return nil
}

// readErrorLog parses the error output.
func (t *TDDLoop) readErrorLog(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	// The diagnostics are already parsed from the test output
	// Inject them into the kernel for analysis
	for _, diag := range t.diagnostics {
		if t.kernel != nil {
			if err := t.kernel.Assert(diag.ToFact()); err != nil {
				logging.Get(logging.CategoryKernel).Warn("Failed to assert diagnostic: %v", err)
			}
		}
	}

	t.transition(TDDStateAnalyzing, TDDActionReadErrorLog, map[string]interface{}{
		"diagnostic_count": len(t.diagnostics),
	})

	return nil
}

// analyzeRootCause performs abductive reasoning to find the root cause.
func (t *TDDLoop) analyzeRootCause(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	// Build a hypothesis based on diagnostics
	if len(t.diagnostics) == 0 {
		t.hypothesis = "unknown error - no diagnostics available"
	} else {
		// Analyze the first diagnostic
		diag := t.diagnostics[0]
		t.hypothesis = fmt.Sprintf("Error in %s at line %d: %s", diag.FilePath, diag.Line, diag.Message)
	}

	// Inject hypothesis into kernel
	if t.kernel != nil {
		if err := t.kernel.Assert(Fact{
			Predicate: "hypothesis",
			Args:      []interface{}{t.hypothesis},
		}); err != nil {
			logging.Get(logging.CategoryKernel).Warn("Failed to assert hypothesis: %v", err)
		}
	}

	t.transition(TDDStateGenerating, TDDActionAnalyzeRoot, map[string]interface{}{
		"hypothesis": t.hypothesis,
	})

	return nil
}

// generatePatch creates a patch to fix the issue using LLM.
func (t *TDDLoop) generatePatch(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.llmClient == nil {
		// Fallback for no LLM
		if len(t.diagnostics) > 0 {
			diag := t.diagnostics[0]
			t.patches = []Patch{
				{
					FilePath:  diag.FilePath,
					Rationale: fmt.Sprintf("Fix: %s (LLM not configured)", diag.Message),
				},
			}
		}
		t.transition(TDDStateApplying, TDDActionGeneratePatch, map[string]interface{}{"patch_count": 1})
		return nil
	}

	// Construct prompt for LLM
	var sb strings.Builder
	sb.WriteString("You are an expert software engineer fixing a bug.\n\n")
	sb.WriteString("Hypothesis: " + t.hypothesis + "\n\n")
	sb.WriteString("Diagnostics:\n")
	for i, d := range t.diagnostics {
		if i >= 5 {
			break
		} // Limit context
		sb.WriteString(fmt.Sprintf("- %s:%d: %s\n", d.FilePath, d.Line, d.Message))
	}
	sb.WriteString("\nPlease generate a patch to fix this issue. Return ONLY the code change in the following format:\n")
	sb.WriteString("FILE: <file_path>\n")
	sb.WriteString("OLD:\n<old_code>\n")
	sb.WriteString("NEW:\n<new_code>\n")
	sb.WriteString("RATIONALE: <explanation>\n")

	// Call LLM
	resp, err := t.llmClient.Complete(ctx, sb.String())
	if err != nil {
		return fmt.Errorf("LLM patch generation failed: %w", err)
	}

	// Parse LLM response
	t.patches = t.parseLLMPatch(resp)

	t.transition(TDDStateApplying, TDDActionGeneratePatch, map[string]interface{}{
		"patch_count": len(t.patches),
	})

	return nil
}

// parseLLMPatch parses the LLM response into Patch structs.
func (t *TDDLoop) parseLLMPatch(response string) []Patch {
	patches := make([]Patch, 0)

	// Simple parsing logic (robustness could be improved)
	// Expecting blocks separated by FILE:
	parts := strings.Split(response, "FILE:")
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}

		lines := strings.Split(part, "\n")
		filePath := strings.TrimSpace(lines[0])

		oldIdx := strings.Index(part, "OLD:")
		newIdx := strings.Index(part, "NEW:")
		ratIdx := strings.Index(part, "RATIONALE:")

		if oldIdx != -1 && newIdx != -1 && ratIdx != -1 {
			oldContent := strings.TrimSpace(part[oldIdx+4 : newIdx])
			newContent := strings.TrimSpace(part[newIdx+4 : ratIdx])
			rationale := strings.TrimSpace(part[ratIdx+10:])

			patches = append(patches, Patch{
				FilePath:   filePath,
				OldContent: oldContent,
				NewContent: newContent,
				Rationale:  rationale,
			})
		}
	}

	return patches
}

// applyPatch applies the generated patches.
func (t *TDDLoop) applyPatch(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, patch := range t.patches {
		if patch.OldContent == "" || patch.NewContent == "" {
			continue
		}

		// Apply via VirtualStore
		// BUG FIX: Action facts require 3+ args (ActionID, Type, Target)
		action := Fact{
			Predicate: "next_action",
			Args: []interface{}{
				fmt.Sprintf("tdd-edit-%d", time.Now().UnixNano()),
				"/edit_file",
				patch.FilePath,
				map[string]interface{}{
					"old": patch.OldContent,
					"new": patch.NewContent,
				},
			},
		}

		_, err := t.virtualStore.RouteAction(ctx, action)
		if err != nil {
			// Mark as needing analysis
			t.transition(TDDStateAnalyzing, TDDActionApplyPatch, map[string]interface{}{
				"error": err.Error(),
			})
			return nil
		}
	}

	t.transition(TDDStateCompiling, TDDActionApplyPatch, nil)
	return nil
}

// build compiles the project.
func (t *TDDLoop) build(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// BUG FIX: Action facts require 3+ args (ActionID, Type, Target)
	action := Fact{
		Predicate: "next_action",
		Args: []interface{}{
			fmt.Sprintf("tdd-build-%d", time.Now().UnixNano()),
			"/build_project",
			t.config.BuildCommand,
		},
	}

	output, err := t.virtualStore.RouteAction(ctx, action)
	t.lastOutput = output

	if err != nil || strings.Contains(output, "error") {
		t.diagnostics = t.parseBuildOutput(output)
		t.transition(TDDStateCompileError, TDDActionBuild, map[string]interface{}{
			"error_count": len(t.diagnostics),
		})
		return nil
	}

	t.transition(TDDStateIdle, TDDActionBuild, nil)
	return nil
}

// escalate escalates to the user.
func (t *TDDLoop) escalate(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	reason := fmt.Sprintf("TDD loop exhausted after %d retries. Last diagnostics: %d errors",
		t.retryCount, len(t.diagnostics))

	// BUG FIX: Action facts require 3+ args (ActionID, Type, Target)
	action := Fact{
		Predicate: "next_action",
		Args: []interface{}{
			fmt.Sprintf("tdd-escalate-%d", time.Now().UnixNano()),
			"/escalate",
			reason,
		},
	}

	_, _ = t.virtualStore.RouteAction(ctx, action)

	t.transition(TDDStateEscalated, TDDActionEscalate, map[string]interface{}{
		"reason": reason,
	})

	return nil
}

// parseTestOutput parses test output into diagnostics.
func (t *TDDLoop) parseTestOutput(output string) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	lines := strings.Split(output, "\n")

	var lastRustError *Diagnostic

	for _, line := range lines {
		// Go Test Fail
		if matches := goFailRegex.FindStringSubmatch(line); len(matches) > 1 {
			diagnostics = append(diagnostics, Diagnostic{
				Severity:  "error",
				Message:   line,
				RawOutput: output,
			})
		}

		// Go Compile Error
		if matches := goErrorRegex.FindStringSubmatch(line); len(matches) > 4 {
			lineNum := 0
			colNum := 0
			fmt.Sscanf(matches[2], "%d", &lineNum)
			fmt.Sscanf(matches[3], "%d", &colNum)
			diagnostics = append(diagnostics, Diagnostic{
				Severity: "error",
				FilePath: matches[1],
				Line:     lineNum,
				Column:   colNum,
				Message:  matches[4],
			})
		}

		// Python Traceback
		if matches := pyErrorRegex.FindStringSubmatch(line); len(matches) > 3 {
			lineNum := 0
			fmt.Sscanf(matches[2], "%d", &lineNum)
			diagnostics = append(diagnostics, Diagnostic{
				Severity: "error",
				FilePath: matches[1],
				Line:     lineNum,
				Message:  "Python error in " + matches[3],
			})
		}

		// Rust Error
		if matches := rustErrorRegex.FindStringSubmatch(line); len(matches) > 2 {
			lastRustError = &Diagnostic{
				Severity: "error",
				Code:     matches[1],
				Message:  matches[2],
			}
		}
		if lastRustError != nil {
			if matches := rustLocRegex.FindStringSubmatch(line); len(matches) > 3 {
				lineNum := 0
				colNum := 0
				fmt.Sscanf(matches[2], "%d", &lineNum)
				fmt.Sscanf(matches[3], "%d", &colNum)
				lastRustError.FilePath = matches[1]
				lastRustError.Line = lineNum
				lastRustError.Column = colNum
				diagnostics = append(diagnostics, *lastRustError)
				lastRustError = nil
			}
		}
	}

	return diagnostics
}

// parseBuildOutput parses build output into diagnostics.
func (t *TDDLoop) parseBuildOutput(output string) []Diagnostic {
	// Reuse test output parser as it covers compile errors too
	return t.parseTestOutput(output)
}

// InjectPatch allows external code (e.g., LLM) to inject a patch.
func (t *TDDLoop) InjectPatch(patch Patch) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.patches = append(t.patches, patch)

	if t.kernel != nil {
		if err := t.kernel.Assert(patch.ToFact()); err != nil {
			logging.Get(logging.CategoryKernel).Warn("Failed to assert patch fact: %v", err)
		}
	}
}

// SetHypothesis allows external code to set the root cause hypothesis.
func (t *TDDLoop) SetHypothesis(hypothesis string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.hypothesis = hypothesis

	if t.kernel != nil {
		if err := t.kernel.Assert(Fact{
			Predicate: "hypothesis",
			Args:      []interface{}{hypothesis},
		}); err != nil {
			logging.Get(logging.CategoryKernel).Warn("Failed to assert hypothesis: %v", err)
		}
	}
}

// TDDLoopToFacts converts the current TDD state to Mangle facts.
func (t *TDDLoop) ToFacts() []Fact {
	t.mu.RLock()
	defer t.mu.RUnlock()

	facts := []Fact{
		{Predicate: "test_state", Args: []interface{}{"/" + string(t.state)}},
		{Predicate: "retry_count", Args: []interface{}{int64(t.retryCount)}},
		{Predicate: "max_retries", Args: []interface{}{int64(t.maxRetries)}},
	}

	for _, diag := range t.diagnostics {
		facts = append(facts, diag.ToFact())
	}

	for _, patch := range t.patches {
		facts = append(facts, patch.ToFact())
	}

	if t.hypothesis != "" {
		facts = append(facts, Fact{
			Predicate: "hypothesis",
			Args:      []interface{}{t.hypothesis},
		})
	}

	return facts
}

// BlockCommit returns true if there are blocking errors.
// Implements Cortex 1.5.0 §2.2 "The Barrier".
// Delegates decision to Mangle rule: block_commit().
func (t *TDDLoop) BlockCommit() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.kernel != nil {
		// Ask the Oracle
		// block_commit logic is now in tdd_logic.mg
		// Note: diagnostics must be asserted by runTests/readErrorLog before calling this.
		results, err := t.kernel.Query("block_commit")
		if err == nil && len(results) > 0 {
			return true
		}
	}

	// Fallback if kernel not available or query fails
	for _, diag := range t.diagnostics {
		if diag.Severity == "error" {
			return true
		}
	}
	return false
}
