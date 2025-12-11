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
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Intent represents a parsed user intent.
type Intent struct {
	ID         string  `json:"id"`
	Category   string  `json:"category"`   // query, mutation, instruction
	Verb       string  `json:"verb"`       // explain, refactor, debug, generate, etc.
	Target     string  `json:"target"`     // file path or symbol
	Constraint string  `json:"constraint"` // additional constraints
	Confidence float64 `json:"confidence"`
}

// FocusResolution represents a resolved reference.
type FocusResolution struct {
	RawReference string
	ResolvedPath string
	SymbolName   string
	Confidence   float64
}

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
}

// DefaultPerceptionConfig returns sensible defaults.
func DefaultPerceptionConfig() PerceptionConfig {
	return PerceptionConfig{
		ConfidenceThreshold: 0.85,
		AmbiguityThreshold:  0.7,
		TickInterval:        50 * time.Millisecond,
		MaxQueueSize:        100,
		UseFallbackParsing:  true,
	}
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
	base.Config.Permissions = []core.ShardPermission{
		core.PermissionReadFile,
		core.PermissionAskUser,
	}
	base.Config.Model = core.ModelConfig{
		Capability: core.CapabilityBalanced, // Need good NL understanding
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

// SetPromptAssembler sets the JIT prompt assembler for dynamic prompt generation.
// When set and ready, the shard will use JIT-compiled prompts instead of the legacy
// constant prompts, falling back to legacy if JIT compilation fails.
func (p *PerceptionFirewallShard) SetPromptAssembler(assembler *articulation.PromptAssembler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.promptAssembler = assembler
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
	p.SetState(core.ShardStateRunning)
	p.mu.Lock()
	p.running = true
	p.StartTime = time.Now()
	p.mu.Unlock()

	defer func() {
		p.SetState(core.ShardStateCompleted)
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
				_ = p.Kernel.Assert(core.Fact{
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

	p.mu.Lock()
	p.lastInput = time.Now()
	p.mu.Unlock()

	// Try LLM-based parsing first
	intent, err := p.parseWithLLM(ctx, input)
	if err != nil {
		// Fallback to regex-based parsing
		if p.config.UseFallbackParsing {
			logging.SystemShardsDebug("[PerceptionFirewall] LLM parsing failed, using fallback: %v", err)
			intent = p.parseWithFallback(input)
		} else {
			logging.Get(logging.CategorySystemShards).Error("[PerceptionFirewall] LLM parsing failed, no fallback: %v", err)
			return err
		}
	}

	logging.SystemShardsDebug("[PerceptionFirewall] Parsed intent: id=%s, verb=%s, category=%s, confidence=%.2f",
		intent.ID, intent.Verb, intent.Category, intent.Confidence)

	// Emit user_intent fact
	_ = p.Kernel.Assert(core.Fact{
		Predicate: "user_intent",
		Args: []interface{}{
			intent.ID,
			intent.Category,
			intent.Verb,
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

	// Check confidence thresholds
	if intent.Confidence < p.config.AmbiguityThreshold {
		logging.Get(logging.CategorySystemShards).Warn("[PerceptionFirewall] Ambiguous intent detected: id=%s, confidence=%.2f", intent.ID, intent.Confidence)
		// Emit ambiguity_flag
		_ = p.Kernel.Assert(core.Fact{
			Predicate: "ambiguity_flag",
			Args:      []interface{}{intent.ID, intent.Confidence, time.Now().Unix()},
		})

		// Track ambiguity pattern (autopoiesis)
		pattern := fmt.Sprintf("ambiguous:%s", intent.Verb)
		p.trackFailure(pattern, "low_confidence")
	}

	if intent.Confidence < p.config.ConfidenceThreshold {
		logging.SystemShardsDebug("[PerceptionFirewall] Clarification needed for intent: %s", intent.ID)
		// Emit clarification_needed
		_ = p.Kernel.Assert(core.Fact{
			Predicate: "clarification_needed",
			Args:      []interface{}{intent.ID, "low_confidence", time.Now().Unix()},
		})

		p.mu.Lock()
		p.clarifications++
		p.mu.Unlock()
	}

	// Resolve focus if target present
	if intent.Target != "" {
		resolution := p.resolveTarget(ctx, intent.Target)
		logging.SystemShardsDebug("[PerceptionFirewall] Target resolution: raw=%s, resolved=%s, confidence=%.2f",
			resolution.RawReference, resolution.ResolvedPath, resolution.Confidence)
		_ = p.Kernel.Assert(core.Fact{
			Predicate: "focus_resolution",
			Args: []interface{}{
				resolution.RawReference,
				resolution.ResolvedPath,
				resolution.SymbolName,
				resolution.Confidence,
			},
		})
	}

	// Mark as processed
	_ = p.Kernel.Assert(core.Fact{
		Predicate: "processed_intent",
		Args:      []interface{}{intent.ID},
	})

	logging.SystemShards("[PerceptionFirewall] Intent processed: id=%s, verb=%s", intent.ID, intent.Verb)
	return nil
}

// parseWithLLM uses the LLM to parse natural language input.
func (p *PerceptionFirewallShard) parseWithLLM(ctx context.Context, input string) (Intent, error) {
	if p.LLMClient == nil {
		return Intent{}, fmt.Errorf("no LLM client")
	}

	can, reason := p.CostGuard.CanCall()
	if !can {
		return Intent{}, fmt.Errorf("LLM blocked: %s", reason)
	}

	// Build system prompt - try JIT first, fall back to legacy
	var systemPrompt string
	p.mu.RLock()
	assembler := p.promptAssembler
	p.mu.RUnlock()

	if assembler != nil && assembler.JITReady() {
		pc := &articulation.PromptContext{
			ShardID:   p.ID,
			ShardType: "perception",
		}
		if jitPrompt, err := assembler.AssembleSystemPrompt(ctx, pc); err == nil {
			systemPrompt = jitPrompt
			logging.SystemShards("[PerceptionFirewall] [JIT] Using JIT-compiled system prompt (%d bytes)", len(systemPrompt))
		} else {
			logging.SystemShards("[PerceptionFirewall] JIT compilation failed, falling back to legacy: %v", err)
		}
	}

	// Fallback to legacy prompt if JIT is not available or failed
	if systemPrompt == "" {
		systemPrompt = p.buildSystemPromptWithLearning()
		logging.SystemShards("[PerceptionFirewall] [FALLBACK] Using legacy system prompt (%d bytes)", len(systemPrompt))
	}

	userPrompt := fmt.Sprintf(perceptionUserPrompt, input)

	result, err := p.GuardedLLMCall(ctx, systemPrompt, userPrompt)
	if err != nil {
		return Intent{}, err
	}

	// Parse JSON response
	intent := p.parseIntentJSON(result, input)
	return intent, nil
}

// buildSystemPromptWithLearning builds the system prompt with learned patterns.
func (p *PerceptionFirewallShard) buildSystemPromptWithLearning() string {
	basePrompt := perceptionSystemPrompt

	// Use base class mutex for accessing learning maps
	p.BaseSystemShard.mu.RLock()
	defer p.BaseSystemShard.mu.RUnlock()

	// Add learned corrections if any
	if len(p.BaseSystemShard.corrections) > 0 {
		basePrompt += "\n\nLEARNED CORRECTIONS (from user feedback):\n"
		for pattern, count := range p.BaseSystemShard.corrections {
			if count >= 2 {
				basePrompt += fmt.Sprintf("- %s\n", pattern)
			}
		}
	}

	// Add patterns to avoid
	if len(p.BaseSystemShard.patternFailure) > 0 {
		basePrompt += "\n\nPATTERNS TO AVOID (low confidence/ambiguous):\n"
		for pattern, count := range p.BaseSystemShard.patternFailure {
			if count >= 2 {
				basePrompt += fmt.Sprintf("- %s\n", pattern)
			}
		}
	}

	return basePrompt
}

// parseIntentJSON extracts intent from LLM JSON output.
func (p *PerceptionFirewallShard) parseIntentJSON(output, originalInput string) Intent {
	intent := Intent{
		ID:         fmt.Sprintf("intent-%d", time.Now().UnixNano()),
		Confidence: 0.5, // Default
	}

	// Try to parse as JSON
	var parsed struct {
		Category   string  `json:"category"`
		Verb       string  `json:"verb"`
		Target     string  `json:"target"`
		Constraint string  `json:"constraint"`
		Confidence float64 `json:"confidence"`
	}

	// Find JSON in output
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start >= 0 && end > start {
		jsonStr := output[start : end+1]
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			intent.Category = parsed.Category
			intent.Verb = parsed.Verb
			intent.Target = parsed.Target
			intent.Constraint = parsed.Constraint
			if parsed.Confidence > 0 {
				intent.Confidence = parsed.Confidence
			}
		}
	}

	// Fallback if parsing failed
	if intent.Verb == "" {
		intent = p.parseWithFallback(originalInput)
	}

	return intent
}

// parseWithFallback uses regex patterns for parsing.
func (p *PerceptionFirewallShard) parseWithFallback(input string) Intent {
	intent := Intent{
		ID:         fmt.Sprintf("intent-%d", time.Now().UnixNano()),
		Category:   "instruction",
		Confidence: 0.6, // Lower confidence for fallback
	}

	// Match verb patterns
	for verb, pattern := range p.verbPatterns {
		if pattern.MatchString(input) {
			intent.Verb = verb
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
	case "explain", "search", "review":
		intent.Category = "query"
	case "fix", "refactor", "create", "delete", "implement":
		intent.Category = "mutation"
	default:
		intent.Category = "instruction"
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
		RawReference: target,
		Confidence:   0.5,
	}

	// Direct file path
	if strings.Contains(target, "/") || strings.Contains(target, "\\") || strings.Contains(target, ".") {
		resolution.ResolvedPath = target
		resolution.Confidence = 0.9
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
					resolution.Confidence = 0.85
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
						resolution.Confidence = 0.7
						return resolution
					}
				}
			}
		}
	}

	// Unable to resolve - emit for clarification
	resolution.Confidence = 0.3
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

// perceptionSystemPrompt is the system prompt for intent parsing.
//
// DEPRECATED: This legacy prompt is retained for fallback when JIT compilation
// is not available or fails. New prompt features should be added via the JIT
// prompt compiler in internal/articulation. The JIT system provides dynamic
// context injection, session awareness, and learned pattern integration.
const perceptionSystemPrompt = `You are the Perception Firewall of the codeNERD agent.
Your role is to transduce natural language input into structured intent atoms.

Output a JSON object with these fields:
{
  "category": "query" | "mutation" | "instruction",
  "verb": "<action verb>",
  "target": "<file path, symbol, or empty>",
  "constraint": "<additional constraints>",
  "confidence": 0.0-1.0
}

Verb categories:
- Query: explain, describe, search, find, show, list, analyze
- Mutation: fix, refactor, create, delete, implement, add, modify
- Instruction: run, test, build, deploy, review, debug

Be precise:
- Extract file paths and symbols exactly as mentioned
- Identify the core action being requested
- Note any constraints or conditions
- Rate your confidence based on clarity of the request

If the request is ambiguous, set confidence < 0.7 and note ambiguity in constraint.`

// perceptionUserPrompt is the template for user input.
//
// DEPRECATED: This legacy user prompt template is retained for fallback.
// The user prompt is typically short and context-specific, so it remains
// as a simple template. System prompt customization should use JIT.
const perceptionUserPrompt = `Parse the following user input into a structured intent:

"%s"

Respond with only the JSON object.`
