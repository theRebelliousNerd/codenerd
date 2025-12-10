package autopoiesis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/logging"
)

// panicMakerIdentity would be loaded via JIT prompt system, but the key
// identity is baked into the prompt construction below.

// PanicMaker generates adversarial inputs designed to break generated tools.
// It analyzes source code to find weak spots and crafts targeted attacks.
type PanicMaker struct {
	client LLMClient // Uses local LLMClient interface from complexity.go
	config PanicMakerConfig
}

// PanicMakerConfig holds configuration for the PanicMaker.
type PanicMakerConfig struct {
	// MaxAttacks is the maximum number of attack vectors to generate per tool.
	MaxAttacks int
	// EnableResourceAttacks allows memory/CPU exhaustion attacks.
	EnableResourceAttacks bool
	// EnableConcurrencyAttacks allows race condition/deadlock attacks.
	EnableConcurrencyAttacks bool
	// MaxInputSize is the maximum size of generated attack inputs (bytes).
	MaxInputSize int
}

// DefaultPanicMakerConfig returns sensible defaults.
func DefaultPanicMakerConfig() PanicMakerConfig {
	return PanicMakerConfig{
		MaxAttacks:               5,
		EnableResourceAttacks:    true,
		EnableConcurrencyAttacks: true,
		MaxInputSize:             1024 * 1024, // 1MB max input
	}
}

// AttackVector represents a single adversarial test case.
type AttackVector struct {
	Name            string `json:"name"`
	Category        string `json:"category"` // nil_pointer, boundary, resource, concurrency, format
	Input           string `json:"input"`
	Description     string `json:"description"`
	ExpectedFailure string `json:"expected_failure"` // panic, deadlock, oom, timeout
}

// AttackResult represents the outcome of running an attack.
type AttackResult struct {
	Vector    AttackVector
	Survived  bool
	Failure   string // Empty if survived
	Duration  int64  // Milliseconds
	MemoryMB  int    // Peak memory during attack
	StackDump string // If panicked
}

// NewPanicMaker creates a new PanicMaker instance.
func NewPanicMaker(client LLMClient) *PanicMaker {
	return NewPanicMakerWithConfig(client, DefaultPanicMakerConfig())
}

// NewPanicMakerWithConfig creates a new PanicMaker with custom configuration.
func NewPanicMakerWithConfig(client LLMClient, config PanicMakerConfig) *PanicMaker {
	logging.Autopoiesis("Creating PanicMaker with config: maxAttacks=%d, resourceAttacks=%v, concurrencyAttacks=%v",
		config.MaxAttacks, config.EnableResourceAttacks, config.EnableConcurrencyAttacks)
	return &PanicMaker{
		client: client,
		config: config,
	}
}

// GenerateAttacks analyzes the tool code and generates targeted attack vectors.
func (p *PanicMaker) GenerateAttacks(ctx context.Context, toolCode string) ([]AttackVector, error) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "PanicMaker.GenerateAttacks")
	defer timer.Stop()

	logging.Autopoiesis("PanicMaker analyzing code (%d bytes) to generate attacks", len(toolCode))

	// Build the analysis prompt
	prompt := p.buildAttackPrompt(toolCode)

	// Query LLM for attack vectors
	logging.AutopoiesisDebug("Sending attack generation prompt to LLM")
	response, err := p.client.Complete(ctx, prompt)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("PanicMaker LLM call failed: %v", err)
		return nil, fmt.Errorf("failed to generate attacks: %w", err)
	}

	// Parse the response
	attacks, err := p.parseAttackResponse(response)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("PanicMaker failed to parse response, using fallback attacks: %v", err)
		return p.generateFallbackAttacks(), nil
	}

	// Filter attacks based on config
	attacks = p.filterAttacks(attacks)

	logging.Autopoiesis("PanicMaker generated %d attack vectors", len(attacks))
	for i, attack := range attacks {
		logging.AutopoiesisDebug("Attack %d: %s (%s) - %s", i+1, attack.Name, attack.Category, attack.ExpectedFailure)
	}

	return attacks, nil
}

// buildAttackPrompt constructs the prompt for attack generation.
func (p *PanicMaker) buildAttackPrompt(toolCode string) string {
	var sb strings.Builder

	sb.WriteString("# PanicMaker Attack Generation\n\n")
	sb.WriteString("You are the PanicMaker. Analyze the following Go code and generate ")
	sb.WriteString(fmt.Sprintf("%d specific attack vectors designed to make it crash.\n\n", p.config.MaxAttacks))

	sb.WriteString("## TARGET CODE:\n```go\n")
	sb.WriteString(toolCode)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## ANALYSIS STRATEGY:\n")
	sb.WriteString("1. Look for unchecked pointers - where can nil cause a panic?\n")
	sb.WriteString("2. Look for unbounded slice/map operations - where can allocation explode?\n")
	sb.WriteString("3. Look for channel operations - where can deadlock occur?\n")
	sb.WriteString("4. Look for input parsing - where are format assumptions violated?\n")
	sb.WriteString("5. Look for loops - where can iteration counts go infinite?\n\n")

	sb.WriteString("## ENABLED ATTACK CATEGORIES:\n")
	sb.WriteString("- nil_pointer: Always enabled\n")
	sb.WriteString("- boundary: Always enabled\n")
	if p.config.EnableResourceAttacks {
		sb.WriteString("- resource: Enabled (OOM/CPU attacks)\n")
	}
	if p.config.EnableConcurrencyAttacks {
		sb.WriteString("- concurrency: Enabled (race/deadlock attacks)\n")
	}
	sb.WriteString("- format: Always enabled\n\n")

	sb.WriteString("## OUTPUT FORMAT:\n")
	sb.WriteString("Output ONLY a JSON array of attack vectors. No explanation, no markdown fences:\n")
	sb.WriteString(`[
  {
    "name": "Nil Input",
    "category": "nil_pointer",
    "input": "",
    "description": "Targets potential nil pointer dereference in main function",
    "expected_failure": "panic"
  }
]`)
	sb.WriteString("\n\n")

	sb.WriteString("Generate attacks now. Be specific to THIS code, not generic fuzzing.\n")

	return sb.String()
}

// parseAttackResponse parses the LLM response into AttackVector slice.
func (p *PanicMaker) parseAttackResponse(response string) ([]AttackVector, error) {
	// Clean up response - remove markdown fences if present
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Find JSON array bounds
	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON array found in response")
	}
	response = response[start : end+1]

	var attacks []AttackVector
	if err := json.Unmarshal([]byte(response), &attacks); err != nil {
		return nil, fmt.Errorf("failed to parse attack JSON: %w", err)
	}

	return attacks, nil
}

// filterAttacks removes attacks that don't match the current config.
func (p *PanicMaker) filterAttacks(attacks []AttackVector) []AttackVector {
	filtered := make([]AttackVector, 0, len(attacks))

	for _, attack := range attacks {
		// Skip resource attacks if disabled
		if attack.Category == "resource" && !p.config.EnableResourceAttacks {
			logging.AutopoiesisDebug("Filtering out resource attack: %s", attack.Name)
			continue
		}

		// Skip concurrency attacks if disabled
		if attack.Category == "concurrency" && !p.config.EnableConcurrencyAttacks {
			logging.AutopoiesisDebug("Filtering out concurrency attack: %s", attack.Name)
			continue
		}

		// Truncate oversized inputs
		if len(attack.Input) > p.config.MaxInputSize {
			attack.Input = attack.Input[:p.config.MaxInputSize]
			logging.AutopoiesisDebug("Truncated attack input: %s", attack.Name)
		}

		filtered = append(filtered, attack)
	}

	// Limit total attacks
	if len(filtered) > p.config.MaxAttacks {
		filtered = filtered[:p.config.MaxAttacks]
	}

	return filtered
}

// generateFallbackAttacks returns generic attacks if LLM parsing fails.
func (p *PanicMaker) generateFallbackAttacks() []AttackVector {
	logging.AutopoiesisDebug("Using fallback attack vectors")

	attacks := []AttackVector{
		{
			Name:            "Nil Input",
			Category:        "nil_pointer",
			Input:           "",
			Description:     "Empty/nil input to trigger nil pointer dereference",
			ExpectedFailure: "panic",
		},
		{
			Name:            "Max Int Boundary",
			Category:        "boundary",
			Input:           "9223372036854775807", // math.MaxInt64
			Description:     "Maximum integer value to trigger overflow",
			ExpectedFailure: "panic",
		},
		{
			Name:            "Negative Index",
			Category:        "boundary",
			Input:           "-1",
			Description:     "Negative index to trigger bounds check failure",
			ExpectedFailure: "panic",
		},
		{
			Name:            "Malformed JSON",
			Category:        "format",
			Input:           `{"key": {"nested": {"deep": {"deeper": {"deepest":`,
			Description:     "Truncated nested JSON to trigger parse error",
			ExpectedFailure: "panic",
		},
	}

	// Add resource attack if enabled
	if p.config.EnableResourceAttacks {
		attacks = append(attacks, AttackVector{
			Name:            "Large Allocation",
			Category:        "resource",
			Input:           strings.Repeat("A", 100000),
			Description:     "Large input to trigger memory allocation spike",
			ExpectedFailure: "oom",
		})
	}

	return attacks
}

// FormatAttackResultsForFeedback creates a human-readable description of attack results
// suitable for feeding back to the ToolGenerator for code regeneration.
func FormatAttackResultsForFeedback(results []AttackResult) string {
	var sb strings.Builder

	survivors := 0
	failures := 0
	for _, r := range results {
		if r.Survived {
			survivors++
		} else {
			failures++
		}
	}

	if failures == 0 {
		sb.WriteString("THUNDERDOME RESULT: SURVIVED\n")
		sb.WriteString(fmt.Sprintf("All %d attacks were defended successfully.\n", len(results)))
		return sb.String()
	}

	sb.WriteString("THUNDERDOME RESULT: DEFEATED\n\n")
	sb.WriteString(fmt.Sprintf("Attacks: %d total, %d survived, %d fatal\n\n", len(results), survivors, failures))
	sb.WriteString("## Fatal Attacks:\n\n")

	for i, r := range results {
		if r.Survived {
			continue
		}

		sb.WriteString(fmt.Sprintf("### Attack %d: %s (%s)\n", i+1, r.Vector.Name, r.Vector.Category))
		sb.WriteString(fmt.Sprintf("**Input:** `%s`\n", truncateString(r.Vector.Input, 100)))
		sb.WriteString(fmt.Sprintf("**Failure:** %s\n", r.Failure))
		if r.StackDump != "" {
			sb.WriteString(fmt.Sprintf("**Stack Trace:**\n```\n%s\n```\n", truncateString(r.StackDump, 500)))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## REGENERATION REQUIREMENTS:\n")
	sb.WriteString("Fix the vulnerabilities above. Key patterns to add:\n")
	sb.WriteString("- Nil checks before pointer dereference\n")
	sb.WriteString("- Bounds checking before slice/array access\n")
	sb.WriteString("- Input size limits before allocation\n")
	sb.WriteString("- Timeouts on blocking operations\n")
	sb.WriteString("- Panic recovery in goroutines\n")

	return sb.String()
}

// truncateString truncates a string to maxLen, adding "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
