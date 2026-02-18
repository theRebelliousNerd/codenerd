// perception.go implements the Perception Firewall system shard.
//
// The Perception Firewall is the entry point for all user input:
// - Transduces natural language to structured intent atoms
// - Resolves fuzzy references to concrete paths
// - Detects ambiguity and triggers clarification
// - Emits user_intent, focus_resolution, and ambiguity_flag facts
//
// This shard is AUTO-START and runs continuously. It is LLM-PRIMARY,
// using the model for semantic understanding with deterministic fallbacks.
package system

import (
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/types"
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Intent is the canonical perception intent type (re-exported for system shard compatibility).
type Intent = perception.Intent

// FocusResolution is the canonical focus resolution type (re-exported).
type FocusResolution = perception.FocusResolution

// PerceptionConfig holds configuration for the perception firewall.
type PerceptionConfig struct {
	// Thresholds
	ConfidenceThreshold float64 // Below this, trigger clarification (default: 0.85)
	AmbiguityThreshold  float64 // Below this, emit ambiguity_flag (default: 0.7)

	// Performance
	TickInterval time.Duration // How often to process pending inputs
	MaxQueueSize int           // Max pending inputs to queue

	// Fallback
	UseFallbackParsing bool // Use regex fallback if LLM fails

	// Learning candidates
	LearningCandidateThreshold   int  // Repeats required before candidate (default: 3)
	LearningCandidateAutoPromote bool // Require confirmation before promotion
}

// DefaultPerceptionConfig returns sensible defaults.
func DefaultPerceptionConfig() PerceptionConfig {
	return PerceptionConfig{
		ConfidenceThreshold:          0.85,
		AmbiguityThreshold:           0.7,
		TickInterval:                 50 * time.Millisecond,
		MaxQueueSize:                 100,
		UseFallbackParsing:           true,
		LearningCandidateThreshold:   3,
		LearningCandidateAutoPromote: false,
	}
}

// LearningCandidateStore records potential taxonomy learnings.
type LearningCandidateStore interface {
	RecordLearningCandidate(phrase, verb, target, reason string) (int, error)
}

// PerceptionFirewallShard transduces user input to structured atoms.
type PerceptionFirewallShard struct {
	*BaseSystemShard
	mu sync.RWMutex

	// Configuration
	config PerceptionConfig

	// Input queue
	pendingInputs chan string

	// State
	intentsProcessed int
	clarifications   int
	lastInput        time.Time

	// Verb corpus for fallback parsing
	verbPatterns map[string]*regexp.Regexp

	// Running state
	running bool
	// Note: patternSuccess, patternFailure, corrections, learningStore are inherited from BaseSystemShard

	// JIT prompt compilation support
	promptAssembler *articulation.PromptAssembler

	// Canonical Perception transducer (NL -> Piggyback -> intent)
	transducer perception.Transducer

	// Optional learning candidate store (SQLite-backed)
	candidateStore LearningCandidateStore
}

// NewPerceptionFirewallShard creates a new Perception Firewall shard.
func NewPerceptionFirewallShard() *PerceptionFirewallShard {
	return NewPerceptionFirewallShardWithConfig(DefaultPerceptionConfig())
}

// NewPerceptionFirewallShardWithConfig creates a perception firewall with custom config.
func NewPerceptionFirewallShardWithConfig(cfg PerceptionConfig) *PerceptionFirewallShard {
	logging.SystemShards("[PerceptionFirewall] Initializing perception firewall shard")
	base := NewBaseSystemShard("perception_firewall", StartupAuto)

	// Configure permissions
	base.Config.Permissions = []types.ShardPermission{
		types.PermissionReadFile,
		types.PermissionAskUser,
	}
	base.Config.Model = types.ModelConfig{
		Capability: types.CapabilityBalanced, // Need good NL understanding
	}

	shard := &PerceptionFirewallShard{
		BaseSystemShard: base,
		config:          cfg,
		pendingInputs:   make(chan string, cfg.MaxQueueSize),
		verbPatterns:    buildVerbPatterns(),
		// patternSuccess, patternFailure, corrections are in BaseSystemShard
	}

	logging.SystemShardsDebug("[PerceptionFirewall] Config: confidence_threshold=%.2f, ambiguity_threshold=%.2f, queue_size=%d",
		cfg.ConfidenceThreshold, cfg.AmbiguityThreshold, cfg.MaxQueueSize)
	return shard
}

// SetLearningStore sets the learning store for persistent autopoiesis.
// Delegates to BaseSystemShard which loads existing patterns.
func (p *PerceptionFirewallShard) SetLearningStore(ls core.LearningStore) {
	p.BaseSystemShard.SetLearningStore(ls)
}

// SetLearningCandidateStore wires a store for learning candidates (optional).
func (p *PerceptionFirewallShard) SetLearningCandidateStore(store LearningCandidateStore) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.candidateStore = store
}

// SetPromptAssembler sets the JIT prompt assembler for dynamic prompt generation.
// When set and ready, the shard will use JIT-compiled prompts instead of the legacy
// constant prompts, falling back to legacy if JIT compilation fails.
func (p *PerceptionFirewallShard) SetPromptAssembler(assembler *articulation.PromptAssembler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Keep BaseSystemShard in sync for generic JIT helpers.
	p.BaseSystemShard.SetPromptAssembler(assembler)
	p.promptAssembler = assembler
	if p.transducer != nil {
		p.transducer.SetPromptAssembler(assembler)
	}
	if assembler != nil {
		logging.SystemShards("[PerceptionFirewall] PromptAssembler attached (JIT ready: %v)", assembler.JITReady())
	}
}

// GetPromptAssembler returns the current prompt assembler, if any.
func (p *PerceptionFirewallShard) GetPromptAssembler() *articulation.PromptAssembler {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.promptAssembler
}

// guardedPerceptionClient routes LLM calls through the BaseSystemShard cost guard.
// This prevents Perception from bypassing system shard rate limits.
type guardedPerceptionClient struct {
	base *BaseSystemShard
}

func (g guardedPerceptionClient) Complete(ctx context.Context, prompt string) (string, error) {
	if g.base == nil {
		return "", fmt.Errorf("no base shard configured")
	}
	return g.base.GuardedLLMCall(ctx, "", prompt)
}

func (g guardedPerceptionClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if g.base == nil {
		return "", fmt.Errorf("no base shard configured")
	}
	return g.base.GuardedLLMCall(ctx, systemPrompt, userPrompt)
}

func (g guardedPerceptionClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if g.base == nil {
		return nil, fmt.Errorf("no base shard configured")
	}
	// Perception doesn't need tool calling - fall back to simple completion
	text, err := g.base.GuardedLLMCall(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}
	return &types.LLMToolResponse{
		Text:       text,
		StopReason: "end_turn",
	}, nil
}

func (p *PerceptionFirewallShard) ensureTransducer() perception.Transducer {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.transducer != nil {
		return p.transducer
	}

	t := perception.NewUnderstandingTransducer(guardedPerceptionClient{base: p.BaseSystemShard})
	if p.promptAssembler != nil {
		t.SetPromptAssembler(p.promptAssembler)
	}
	p.transducer = t
	return t
}

func normalizeAtom(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}
	if v == "none" {
		return v
	}
	if strings.HasPrefix(v, "/") {
		return v
	}
	return "/" + v
}

func (p *PerceptionFirewallShard) applyPerceptionMangleUpdates(updates []string) {
	if p.Kernel == nil || len(updates) == 0 {
		return
	}

	// Perception is an untrusted boundary: only allow a conservative subset of facts.
	policy := core.MangleUpdatePolicy{
		AllowedPredicates: map[string]struct{}{
			"ambiguity_flag":       {},
			"clarification_needed": {},
		},
		MaxUpdates: 50,
	}

	facts, blocked := core.FilterMangleUpdates(p.Kernel, updates, policy)
	for _, b := range blocked {
		logging.SystemShardsDebug("[PerceptionFirewall] Blocked mangle_update %q: %s", b.Update, b.Reason)
	}
	if len(facts) > 0 {
		_ = p.Kernel.AssertBatch(facts)
	}
}

// trackSuccess records a successful parse pattern.
func (p *PerceptionFirewallShard) trackSuccess(pattern string) {
	p.BaseSystemShard.trackSuccess(pattern)
}

// trackFailure records a failed or ambiguous pattern.
func (p *PerceptionFirewallShard) trackFailure(pattern string, reason string) {
	p.BaseSystemShard.trackFailure(pattern, reason)
}

// trackCorrection records a user correction.
func (p *PerceptionFirewallShard) trackCorrection(original, corrected string) {
	p.BaseSystemShard.trackCorrection(original, corrected)
}

func (p *PerceptionFirewallShard) isVerbKnown(verb string) bool {
	normalized := normalizeAtom(verb)
	if normalized == "" || normalized == "none" {
		return false
	}
	for _, entry := range perception.VerbCorpus {
		if normalizeAtom(entry.Verb) == normalized {
			return true
		}
	}
	return false
}

func (p *PerceptionFirewallShard) isVerbActionMapped(verb string) (bool, error) {
	if p.Kernel == nil {
		return true, nil
	}
	results, err := p.Kernel.Query("action_mapping")
	if err != nil {
		return false, err
	}
	normalized := normalizeAtom(verb)
	for _, fact := range results {
		if len(fact.Args) < 2 {
			continue
		}
		mappedVerb := normalizeAtom(types.ExtractString(fact.Args[0]))
		if mappedVerb == normalized {
			return true, nil
		}
	}
	return false, nil
}

func (p *PerceptionFirewallShard) classifyVerbMapping(verb string) (bool, string) {
	normalized := normalizeAtom(verb)
	if normalized == "" || normalized == "none" {
		return false, "/no_verb_match"
	}
	known := p.isVerbKnown(normalized)
	mapped, err := p.isVerbActionMapped(normalized)
	if err != nil {
		logging.SystemShardsDebug("[PerceptionFirewall] action_mapping query failed: %v", err)
		return true, ""
	}
	if !known && mapped {
		return true, ""
	}
	if !known {
		return false, "/unknown_verb"
	}
	if !mapped {
		return false, "/no_action_mapping"
	}
	return true, ""
}

func (p *PerceptionFirewallShard) recordLearningCandidate(phrase, verb, target, reason string) (int, error) {
	p.mu.RLock()
	store := p.candidateStore
	p.mu.RUnlock()
	if store == nil || strings.TrimSpace(phrase) == "" {
		return 0, nil
	}
	return store.RecordLearningCandidate(phrase, verb, target, reason)
}

func buildVerbPatterns() map[string]*regexp.Regexp {
	patterns := map[string]string{
		"explain":   `(?i)(explain|describe|what is|how does|tell me about)`,
		"review":    `(?i)(review|check|analyze|audit|inspect)`,
		"fix":       `(?i)(fix|repair|resolve|correct|patch)`,
		"refactor":  `(?i)(refactor|clean up|improve|optimize)`,
		"create":    `(?i)(create|make|generate|build|write|add)`,
		"delete":    `(?i)(delete|remove|drop|clear)`,
		"test":      `(?i)(test|verify|validate|check)`,
		"search":    `(?i)(search|find|look for|locate|grep)`,
		"debug":     `(?i)(debug|troubleshoot|diagnose|trace)`,
		"implement": `(?i)(implement|build|develop|code)`,
		"run":       `(?i)(run|execute|start|launch)`,
		"research":  `(?i)(research|investigate|explore|learn about)`,
	}

	result := make(map[string]*regexp.Regexp)
	for verb, pattern := range patterns {
		if re, err := regexp.Compile(pattern); err == nil {
			result[verb] = re
		}
	}
	return result
}

// Execute runs the Perception Firewall's continuous parsing loop.
func (p *PerceptionFirewallShard) Execute(ctx context.Context, task string) (string, error) {
	logging.SystemShards("[PerceptionFirewall] Starting continuous parsing loop")
	p.SetState(types.ShardStateRunning)
	p.mu.Lock()
	p.running = true
	p.StartTime = time.Now()
	p.mu.Unlock()

	defer func() {
		p.SetState(types.ShardStateCompleted)
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
		logging.SystemShards("[PerceptionFirewall] Parsing loop terminated")
	}()

	// Initialize kernel if not set
	if p.Kernel == nil {
		logging.SystemShardsDebug("[PerceptionFirewall] Creating new kernel (none attached)")
		kernel, err := core.NewRealKernel()
		if err != nil {
			return "", fmt.Errorf("failed to create kernel: %w", err)
		}
		p.Kernel = kernel
	}

	ticker := time.NewTicker(p.config.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logging.SystemShards("[PerceptionFirewall] Context cancelled, shutting down")
			return p.generateShutdownSummary("context cancelled"), ctx.Err()
		case <-p.StopCh:
			logging.SystemShards("[PerceptionFirewall] Stop signal received")
			return p.generateShutdownSummary("stopped"), nil
		case input := <-p.pendingInputs:
			// Process input
			logging.SystemShardsDebug("[PerceptionFirewall] Processing input: %s", truncateForLog(input, 80))
			if err := p.processInput(ctx, input); err != nil {
				logging.Get(logging.CategorySystemShards).Error("[PerceptionFirewall] Error processing input: %v", err)
				_ = p.Kernel.Assert(types.Fact{
					Predicate: "perception_error",
					Args:      []interface{}{err.Error(), time.Now().Unix()},
				})
			}
		case <-ticker.C:
			// Emit heartbeat
			_ = p.EmitHeartbeat()
		}
	}
}

// SubmitInput queues user input for processing.
func (p *PerceptionFirewallShard) SubmitInput(input string) error {
	select {
	case p.pendingInputs <- input:
		return nil
	default:
		return fmt.Errorf("input queue full")
	}
}

// processInput parses a single user input.
func (p *PerceptionFirewallShard) processInput(ctx context.Context, input string) error {
	timer := logging.StartTimer(logging.CategorySystemShards, "[PerceptionFirewall] processInput")
	defer timer.Stop()

	_, err := p.Perceive(ctx, input, nil)
	return err
}

// Perceive parses user input into an intent and emits canonical facts into the parent kernel.
//
// This is the synchronous entry point used by interactive UIs. It is designed to be resilient:
// if LLM parsing fails or is blocked by CostGuard, it degrades to deterministic fallback parsing.
func (p *PerceptionFirewallShard) Perceive(ctx context.Context, input string, history []perception.ConversationTurn) (Intent, error) {
	timer := logging.StartTimer(logging.CategorySystemShards, "[PerceptionFirewall] Perceive")
	defer timer.Stop()

	p.mu.Lock()
	p.lastInput = time.Now()
	p.mu.Unlock()

	// Ensure kernel is available.
	if p.Kernel == nil {
		logging.SystemShardsDebug("[PerceptionFirewall] Creating new kernel (none attached)")
		kernel, err := core.NewRealKernel()
		if err != nil {
			return Intent{}, fmt.Errorf("failed to create kernel: %w", err)
		}
		p.Kernel = kernel
	}

	transducer := p.ensureTransducer()

	// Prefer GCD so control_packet.mangle_updates are syntax-validated.
	maxRetries := 3
	if p.CostGuard != nil && p.CostGuard.MaxValidationRetries > 0 {
		maxRetries = p.CostGuard.MaxValidationRetries
	}

	intent, validatedUpdates, parseErr := transducer.ParseIntentWithGCD(ctx, input, history, maxRetries)
	parseFailed := parseErr != nil
	if parseErr != nil {
		logging.SystemShardsDebug("[PerceptionFirewall] Transducer parse failed (degrading): %v", parseErr)
		if p.config.UseFallbackParsing {
			intent = p.parseWithFallback(input)
			validatedUpdates = nil
			parseErr = nil
		}
	}

	// Normalize category/verb to name constants (leading '/').
	intent.Category = normalizeAtom(intent.Category)
	intent.Verb = normalizeAtom(intent.Verb)
	// NOTE: We intentionally leave intent.Response empty if perception didn't generate one.
	// This forces articulation to generate a proper response. The old fallback of "Understood."
	// was useless and would sometimes be returned directly to users, which is terrible UX.
	// Articulation will always generate a meaningful response.

	// Use a stable intent ID so policy can scope rules to the active user intent.
	// This prevents stale intent accumulation across turns.
	intentID := "/current_intent"
	phrase := strings.TrimSpace(input)

	// Clear stale Perception ephemera to avoid old ambiguity/clarification loops.
	// Use a transaction to batch all retracts+asserts into a single rebuild.
	tx := types.NewKernelTx(p.Kernel)
	tx.Retract("ambiguity_flag")
	tx.Retract("clarification_needed")
	tx.Retract("intent_unknown")
	tx.Retract("intent_unmapped")
	tx.Retract("no_action_reason")
	tx.Retract("clarification_question")
	tx.Retract("clarification_option")
	tx.Retract("learning_candidate")
	tx.Retract("learning_candidate_fact")
	tx.Retract("learning_candidate_count")
	tx.Retract("awaiting_clarification")
	tx.Retract("awaiting_user_input")
	tx.Retract("campaign_awaiting_clarification")
	tx.Retract("focus_resolution")
	tx.Retract("user_input_string")
	tx.RetractFact(types.Fact{Predicate: "user_intent", Args: []interface{}{intentID}})
	tx.RetractFact(types.Fact{Predicate: "processed_intent", Args: []interface{}{intentID}})
	tx.RetractFact(types.Fact{Predicate: "executive_processed_intent", Args: []interface{}{intentID}})

	if phrase != "" {
		tx.Assert(types.Fact{
			Predicate: "user_input_string",
			Args:      []interface{}{phrase},
		})
	}
	if err := tx.Commit(); err != nil {
		logging.Get(logging.CategoryPerception).Error("perception ephemera cleanup failed: %v", err)
	}

	unknownReason := ""
	if strings.TrimSpace(intent.Verb) == "" {
		unknownReason = "/no_verb_match"
		intent.Verb = "/explain"
		if intent.Category == "" || intent.Category == "/instruction" {
			intent.Category = "/query"
		}
		if intent.Confidence > 0.3 {
			intent.Confidence = 0.3
		}
	} else if parseFailed {
		unknownReason = "/llm_failed"
		if intent.Confidence > 0.4 {
			intent.Confidence = 0.4
		}
	} else if intent.Confidence < p.config.AmbiguityThreshold {
		unknownReason = "/heuristic_low"
	}

	if unknownReason != "" {
		_ = p.Kernel.Assert(types.Fact{
			Predicate: "intent_unknown",
			Args: []interface{}{
				truncateForLog(input, 120),
				types.MangleAtom(unknownReason),
			},
		})
	}

	if mapped, mapReason := p.classifyVerbMapping(intent.Verb); !mapped {
		_ = p.Kernel.Assert(types.Fact{
			Predicate: "intent_unmapped",
			Args: []interface{}{
				types.MangleAtom(intent.Verb),
				types.MangleAtom(mapReason),
			},
		})
		if intent.Confidence > 0.4 {
			intent.Confidence = 0.4
		}
		if count, err := p.recordLearningCandidate(phrase, intent.Verb, intent.Target, mapReason); err != nil {
			logging.SystemShardsDebug("[PerceptionFirewall] Failed to record learning candidate: %v", err)
		} else if p.config.LearningCandidateThreshold > 0 {
			if p.Kernel != nil {
				// Replace any stale count fact for this phrase.
				if existing, err := p.Kernel.Query("learning_candidate_count"); err == nil {
					for _, f := range existing {
						if len(f.Args) < 2 {
							continue
						}
						if existingPhrase, ok := f.Args[0].(string); ok && existingPhrase == phrase {
							_ = p.Kernel.RetractFact(f)
						}
					}
				}
				_ = p.Kernel.Assert(types.Fact{
					Predicate: "learning_candidate_count",
					Args:      []interface{}{phrase, count},
				})
			}
			if count >= p.config.LearningCandidateThreshold {
				_ = p.Kernel.Assert(types.Fact{
					Predicate: "learning_candidate",
					Args: []interface{}{
						phrase,
						types.MangleAtom(intent.Verb),
						intent.Target,
						types.MangleAtom(mapReason),
					},
				})
			}
		}
	}

	// Emit user_intent/5
	_ = p.Kernel.Assert(types.Fact{
		Predicate: "user_intent",
		Args: []interface{}{
			intentID,
			types.MangleAtom(intent.Category),
			types.MangleAtom(intent.Verb),
			intent.Target,
			intent.Constraint,
		},
	})

	p.mu.Lock()
	p.intentsProcessed++
	p.mu.Unlock()

	// Track success for high-confidence parses (autopoiesis)
	if intent.Confidence >= p.config.ConfidenceThreshold {
		pattern := fmt.Sprintf("%s:%s", intent.Verb, intent.Category)
		p.trackSuccess(pattern)
	}

	// Ambiguity handling
	if intent.Confidence < p.config.AmbiguityThreshold {
		logging.Get(logging.CategorySystemShards).Warn("[PerceptionFirewall] Ambiguous intent detected: confidence=%.2f", intent.Confidence)
		_ = p.Kernel.Assert(types.Fact{
			Predicate: "ambiguity_flag",
			Args: []interface{}{
				intentID,
				truncateForLog(input, 120),
				fmt.Sprintf("confidence=%.2f", intent.Confidence),
			},
		})

		// Track ambiguity pattern (autopoiesis)
		pattern := fmt.Sprintf("ambiguous:%s", intent.Verb)
		p.trackFailure(pattern, "low_confidence")
	}

	// Resolve focus if target present (drives clarification_needed/1 via policy)
	if strings.TrimSpace(intent.Target) != "" && intent.Target != "none" {
		resolution := p.resolveTarget(ctx, intent.Target)
		logging.SystemShardsDebug("[PerceptionFirewall] Target resolution: raw=%s, resolved=%s, confidence=%d",
			resolution.RawReference, resolution.ResolvedPath, resolution.ConfidencePercent)
		_ = p.Kernel.Assert(resolution.ToFact())
		if resolution.ConfidencePercent < 85 {
			p.mu.Lock()
			p.clarifications++
			p.mu.Unlock()
		}
	}

	// Apply a conservative subset of validated mangle_updates from the control packet.
	p.applyPerceptionMangleUpdates(validatedUpdates)

	// Mark as processed
	_ = p.Kernel.Assert(types.Fact{
		Predicate: "processed_intent",
		Args:      []interface{}{intentID},
	})

	logging.SystemShards("[PerceptionFirewall] Intent processed: id=%s, verb=%s", intentID, intent.Verb)
	return intent, parseErr
}

// parseWithFallback uses regex patterns for parsing.
func (p *PerceptionFirewallShard) parseWithFallback(input string) Intent {
	intent := Intent{
		Category:   "/instruction",
		Confidence: 0.6, // Lower confidence for fallback
	}

	// Match verb patterns
	for verb, pattern := range p.verbPatterns {
		if pattern.MatchString(input) {
			intent.Verb = normalizeAtom(verb)
			break
		}
	}

	// Extract potential target (file paths, symbols)
	pathPattern := regexp.MustCompile(`(?:in\s+|at\s+|file\s+|path\s+)?([a-zA-Z0-9_\-./]+\.[a-zA-Z]+)`)
	if matches := pathPattern.FindStringSubmatch(input); len(matches) > 1 {
		intent.Target = matches[1]
	}

	// Determine category
	switch intent.Verb {
	case "/explain", "/search", "/review":
		intent.Category = "/query"
	case "/fix", "/refactor", "/create", "/delete", "/implement":
		intent.Category = "/mutation"
	default:
		intent.Category = "/instruction"
	}

	// Store original as constraint if no specific constraint found
	if intent.Constraint == "" {
		intent.Constraint = input
	}

	return intent
}

// resolveTarget attempts to resolve a fuzzy reference to a concrete path.
func (p *PerceptionFirewallShard) resolveTarget(ctx context.Context, target string) FocusResolution {
	resolution := FocusResolution{
		RawReference:      target,
		ConfidencePercent: 50,
	}

	// Direct file path
	if strings.Contains(target, "/") || strings.Contains(target, "\\") || strings.Contains(target, ".") {
		resolution.ResolvedPath = target
		resolution.ConfidencePercent = 90
		return resolution
	}

	// Symbol reference (needs symbol_graph lookup)
	// Query the kernel for matching symbols (predicate only; filter in Go)
	results, err := p.Kernel.Query("symbol_graph")
	if err == nil {
		for _, fact := range results {
			if len(fact.Args) < 4 {
				continue
			}
			if name, ok := fact.Args[0].(string); ok && strings.EqualFold(name, target) {
				if path, ok := fact.Args[3].(string); ok {
					resolution.ResolvedPath = path
					resolution.SymbolName = target
					resolution.ConfidencePercent = 85
					return resolution
				}
			}
		}
	}

	// Partial match via file_topology
	results, err = p.Kernel.Query("file_topology")
	if err == nil {
		for _, fact := range results {
			if len(fact.Args) > 0 {
				if path, ok := fact.Args[0].(string); ok {
					if strings.Contains(strings.ToLower(path), strings.ToLower(target)) {
						resolution.ResolvedPath = path
						resolution.ConfidencePercent = 70
						return resolution
					}
				}
			}
		}
	}

	// Unable to resolve - emit for clarification
	resolution.ConfidencePercent = 30
	return resolution
}

// generateShutdownSummary creates a summary of the shard's activity.
func (p *PerceptionFirewallShard) generateShutdownSummary(reason string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return fmt.Sprintf(
		"Perception Firewall shutdown (%s). Intents: %d, Clarifications: %d, Runtime: %s",
		reason,
		p.intentsProcessed,
		p.clarifications,
		time.Since(p.StartTime).String(),
	)
}

// GetStats returns parsing statistics.
func (p *PerceptionFirewallShard) GetStats() map[string]int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return map[string]int{
		"intents_processed": p.intentsProcessed,
		"clarifications":    p.clarifications,
	}
}

// RecordCorrection allows external systems to record when a user corrects an intent.
// This enables the perception firewall to learn from mistakes.
func (p *PerceptionFirewallShard) RecordCorrection(originalIntent, correctedIntent Intent) {
	// Track the correction pattern
	original := fmt.Sprintf("%s:%s:%s", originalIntent.Verb, originalIntent.Category, originalIntent.Target)
	corrected := fmt.Sprintf("%s:%s:%s", correctedIntent.Verb, correctedIntent.Category, correctedIntent.Target)
	p.trackCorrection(original, corrected)

	// Also track specific verb corrections
	if originalIntent.Verb != correctedIntent.Verb {
		p.trackCorrection(
			fmt.Sprintf("verb:%s", originalIntent.Verb),
			fmt.Sprintf("verb:%s", correctedIntent.Verb),
		)
	}

	// Track category corrections
	if originalIntent.Category != correctedIntent.Category {
		p.trackCorrection(
			fmt.Sprintf("category:%s", originalIntent.Category),
			fmt.Sprintf("category:%s", correctedIntent.Category),
		)
	}
}

// GetLearnedPatterns returns learned patterns for inclusion in LLM prompts.
func (p *PerceptionFirewallShard) GetLearnedPatterns() map[string][]string {
	// Use base class mutex for accessing learning maps
	p.BaseSystemShard.mu.RLock()
	defer p.BaseSystemShard.mu.RUnlock()

	result := make(map[string][]string)

	// Successful patterns
	var successful []string
	for pattern, count := range p.BaseSystemShard.patternSuccess {
		if count >= 3 {
			successful = append(successful, pattern)
		}
	}
	result["successful"] = successful

	// Failed patterns
	var failed []string
	for pattern, count := range p.BaseSystemShard.patternFailure {
		if count >= 2 {
			failed = append(failed, pattern)
		}
	}
	result["failed"] = failed

	// Corrections
	var corrections []string
	for pattern, count := range p.BaseSystemShard.corrections {
		if count >= 2 {
			corrections = append(corrections, pattern)
		}
	}
	result["corrections"] = corrections

	return result
}

// NOTE: Legacy perceptionSystemPrompt and perceptionUserPrompt constants have been DELETED.
// Perception system prompts are now JIT-compiled from:
//   internal/prompt/atoms/system/perception.yaml
// The UnderstandingTransducer handles prompt assembly via its PromptAssembler.
