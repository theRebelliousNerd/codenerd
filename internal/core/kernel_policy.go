package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/mangle/feedback"

	"github.com/google/mangle/factstore"
)

// unsafeNegationPattern matches negated atoms with anonymous variables
// e.g., !active_shard(/coder, _) or !foo(_, Bar, _)
var unsafeNegationPattern = regexp.MustCompile(`!\s*\w+\s*\([^)]*_[^)]*\)`)

// checkUnsafeNegation validates that a rule doesn't contain unsafe negation patterns.
// Unsafe patterns include anonymous variables (_) in negated atoms, which would be
// unbound and violate Mangle's negation safety requirement.
func checkUnsafeNegation(rule string) error {
	if unsafeNegationPattern.MatchString(rule) {
		// Extract the offending pattern for better error messages
		matches := unsafeNegationPattern.FindAllString(rule, -1)
		return fmt.Errorf("unsafe negation pattern detected: %v - anonymous variables in negated atoms are unbound. Use a helper predicate instead (e.g., has_active_shard(Type) instead of !active_shard(_, Type))", matches)
	}
	return nil
}

// =============================================================================
// POLICY MANAGEMENT
// =============================================================================

// SetPolicy allows loading custom policy rules (for shard specialization).
func (k *RealKernel) SetPolicy(policy string) {
	logging.KernelDebug("SetPolicy: loading custom policy (%d bytes)", len(policy))
	k.mu.Lock()
	defer k.mu.Unlock()
	k.policy = policy
	k.loadedPolicyFiles = make(map[string]struct{})
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
	key := strings.ToLower(baseName)

	k.mu.RLock()
	if _, ok := k.loadedPolicyFiles[key]; ok {
		k.mu.RUnlock()
		logging.KernelDebug("LoadPolicyFile: already loaded (skipping append): %s", baseName)
		return nil
	}
	k.mu.RUnlock()

	var (
		data       []byte
		sourceDesc string
	)

	// 1. Try Embedded Core first
	if bytes, err := coreLogic.ReadFile("defaults/" + baseName); err == nil {
		data = bytes
		sourceDesc = "embedded core: " + baseName
	}

	// 2. Try User Workspace (.nerd/mangle)
	if data == nil {
		userPath := filepath.Join(k.nerdPath("mangle"), baseName)
		if bytes, err := os.ReadFile(userPath); err == nil {
			data = bytes
			sourceDesc = "user workspace: " + userPath
		}
	}

	// 3. Try explicitly provided path
	if data == nil {
		if bytes, err := os.ReadFile(path); err == nil {
			data = bytes
			sourceDesc = "explicit path: " + path
		}
	}

	// 4. Try legacy search paths (fallback for existing behavior)
	if data == nil {
		searchPaths := []string{
			filepath.Join("internal/mangle", baseName),
			filepath.Join("../internal/mangle", baseName),
			filepath.Join("../../internal/mangle", baseName),
		}

		for _, p := range searchPaths {
			bytes, err := os.ReadFile(p)
			if err == nil {
				data = bytes
				sourceDesc = "legacy path: " + p
				break
			}
		}
	}

	if data == nil {
		logging.Get(logging.CategoryKernel).Error("LoadPolicyFile: policy file not found: %s", path)
		return fmt.Errorf("policy file not found: %s", path)
	}

	k.mu.Lock()
	defer k.mu.Unlock()
	if k.loadedPolicyFiles == nil {
		k.loadedPolicyFiles = make(map[string]struct{})
	}
	if _, ok := k.loadedPolicyFiles[key]; ok {
		logging.KernelDebug("LoadPolicyFile: already loaded after read (skipping append): %s", baseName)
		return nil
	}

	logging.Kernel("LoadPolicyFile: loaded from %s (%d bytes)", sourceDesc, len(data))
	prevLen := len(k.policy)
	k.policy = k.policy + "\n\n# Appended Policy (" + baseName + ")\n" + string(data)
	k.loadedPolicyFiles[key] = struct{}{}
	k.policyDirty = true
	logging.KernelDebug("LoadPolicyFile: policy grew from %d to %d bytes, policyDirty=true", prevLen, len(k.policy))
	return nil
}

// GetPolicy returns the current policy.
func (k *RealKernel) GetPolicy() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.policy
}

// =============================================================================
// LEARNED RULES & AUTOPOIESIS
// =============================================================================

// HotLoadRule dynamically loads a single Mangle rule at runtime.
// This is used by Autopoiesis to add new rules without restarting.
// FIX for Bug #8 (Suicide Rule): Uses a "Sandbox Compiler" to validate the rule
// before accepting it, preventing invalid rules from bricking the kernel.
func (k *RealKernel) HotLoadRule(rule string) error {
        timer := logging.StartTimer(logging.CategoryKernel, "HotLoadRule")

        if rule == "" {
                err := fmt.Errorf("empty rule")
                logging.Get(logging.CategoryKernel).Error("HotLoadRule: %v", err)
                return err
        }

	normalizedRule := feedback.NormalizeRuleInput(rule)
	if normalizedRule != rule {
		logging.KernelDebug("HotLoadRule: normalized rule input for parser compatibility")
	}
	rule = normalizedRule

	// Pre-validation: Check for unsafe negation patterns (unbound variables in negation)
	// Pattern: !predicate(..., _) where _ is an anonymous variable that would be unbound
	if err := checkUnsafeNegation(rule); err != nil {
		logging.Get(logging.CategoryKernel).Error("HotLoadRule: %v", err)
		return err
	}

	logging.Kernel("HotLoadRule: attempting to load rule (%d bytes)", len(rule))

	// Log the rule being loaded (truncated for readability)
	rulePreview := rule
	if len(rulePreview) > 100 {
		rulePreview = rulePreview[:100] + "..."
	}
	logging.KernelDebug("HotLoadRule: rule preview: %s", rulePreview)

        k.mu.RLock()
        schemas := k.schemas
        policy := k.policy
        learned := k.learned
        k.mu.RUnlock()

        // 1. Validate with a sandbox kernel (memory-only)
        logging.KernelDebug("HotLoadRule: creating sandbox kernel for validation")
        logging.KernelDebug("HotLoadRule: validating rule in sandbox...")
        if err := validateRuleSandbox(rule, schemas, policy, learned); err != nil {
                logging.Get(logging.CategoryKernel).Error("HotLoadRule: rule rejected by sandbox compiler: %v", err)
                return fmt.Errorf("rule rejected by sandbox compiler: %w", err)
        }
        logging.KernelDebug("HotLoadRule: sandbox validation passed")

        timer.StopWithInfo()
        logging.Kernel("HotLoadRule: rule validated successfully")
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
	if err := feedbackLoop.UpdateSchema(k.GetSchemas()); err != nil {
		logging.Get(logging.CategoryKernel).Warn("GenerateValidatedRule: failed to update sanitizer schema: %v", err)
	}

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

        normalizedRule := feedback.NormalizeRuleInput(rule)
        if normalizedRule != rule {
                logging.KernelDebug("HotLoadLearnedRule: normalized rule input for parser compatibility")
        }
        rule = normalizedRule

        if err := checkUnsafeNegation(rule); err != nil {
                logging.Get(logging.CategoryKernel).Error("HotLoadLearnedRule: %v", err)
                return err
        }

        k.mu.RLock()
        schemas := k.schemas
        policy := k.policy
        learned := k.learned
        k.mu.RUnlock()

        // 1. Validate using sandbox (compile-only)
        if err := validateRuleSandbox(rule, schemas, policy, learned); err != nil {
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

	// 1c. Pathological pattern check - prevent infinite loop rules
	// This catches rules like "next_action(X) :- current_time(_)" that always fire
	if loopErr := k.checkInfiniteLoopRisk(rule); loopErr != "" {
		logging.Get(logging.CategoryKernel).Error("HotLoadLearnedRule: pathological pattern detected: %s", loopErr)
		return fmt.Errorf("pathological rule rejected: %s", loopErr)
	}
	logging.KernelDebug("HotLoadLearnedRule: pathological pattern check passed")

	// 2. Persist to learned.mg file
        if err := k.appendToLearnedFile(rule); err != nil {
                logging.Get(logging.CategoryKernel).Error("HotLoadLearnedRule: failed to persist rule: %v", err)
                return err
        }

        k.mu.Lock()
        k.applyLearnedRuleLocked(rule)
        k.refreshSchemaValidatorLocked()
        k.mu.Unlock()

        logging.Kernel("HotLoadLearnedRule: rule loaded and persisted successfully")
        return nil
}

func validateRuleSandbox(rule, schemas, policy, learned string) error {
        sandbox := &RealKernel{
                store:             factstore.NewSimpleInMemoryStore(),
                loadedPolicyFiles: make(map[string]struct{}),
                policyDirty:       true,
        }
        sandbox.schemas = schemas
        sandbox.policy = policy
        sandbox.learned = learned
        if rule != "" {
                if sandbox.learned != "" {
                        sandbox.learned = sandbox.learned + "\n\n# Sandbox Validation\n" + rule
                } else {
                        sandbox.learned = "# Sandbox Validation\n" + rule
                }
        }
        return sandbox.rebuildProgram()
}

func (k *RealKernel) applyLearnedRuleLocked(rule string) {
        if rule == "" {
                return
        }
        if k.learned != "" {
                k.learned = k.learned + "\n\n# HotLoaded Rule\n" + rule
        } else {
                k.learned = "# HotLoaded Rule\n" + rule
        }
        k.policyDirty = true
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
	k.refreshSchemaValidatorLocked()
}
