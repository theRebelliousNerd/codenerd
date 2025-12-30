package shards

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/articulation"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// RequirementsInterrogatorShard is a Socratic shard that elicits missing requirements.
// It is intended for early-phase clarification before kicking off campaigns or large tasks.
type RequirementsInterrogatorShard struct {
	*coreshards.BaseShardAgent
	mu              sync.RWMutex
	llmClient       types.LLMClient
	promptAssembler *articulation.PromptAssembler
}

// NewRequirementsInterrogatorShard creates a new interrogator shard.
func NewRequirementsInterrogatorShard() *RequirementsInterrogatorShard {
	cfg := types.ShardConfig{
		Name: "requirements_interrogator",
		Type: types.ShardTypeEphemeral,
		Permissions: []types.ShardPermission{
			types.PermissionAskUser,
		},
		Model: types.ModelConfig{
			Capability: types.CapabilityHighReasoning,
		},
		Timeout: 5 * 60 * 1000000000, // 5 minutes
	}
	return &RequirementsInterrogatorShard{
		BaseShardAgent: coreshards.NewBaseShardAgent("requirements_interrogator", cfg),
	}
}

// SetLLMClient injects the LLM client (satisfies ShardAgent).
func (s *RequirementsInterrogatorShard) SetLLMClient(client types.LLMClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.llmClient = client
	if s.BaseShardAgent != nil {
		s.BaseShardAgent.SetLLMClient(client)
	}
}

// SetPromptAssembler sets the JIT prompt assembler for dynamic prompt generation.
func (s *RequirementsInterrogatorShard) SetPromptAssembler(assembler *articulation.PromptAssembler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.promptAssembler = assembler
	if assembler != nil {
		logging.SystemShards("[RequirementsInterrogator] PromptAssembler attached (JIT ready: %v)", assembler.JITReady())
	}
}

// getSystemPrompt returns the JIT-compiled system prompt.
// Returns empty string if JIT is unavailable.
func (s *RequirementsInterrogatorShard) getSystemPrompt(ctx context.Context) string {
	s.mu.RLock()
	pa := s.promptAssembler
	s.mu.RUnlock()

	if pa == nil {
		logging.SystemShards("[RequirementsInterrogator] [ERROR] No PromptAssembler configured")
		return ""
	}

	if !pa.JITReady() {
		logging.SystemShards("[RequirementsInterrogator] [ERROR] JIT not ready (ensure system/requirements_interrogator atoms exist)")
		return ""
	}

	pc := &articulation.PromptContext{
		ShardID:   "requirements_interrogator",
		ShardType: "requirements_interrogator",
	}
	jitPrompt, err := pa.AssembleSystemPrompt(ctx, pc)
	if err != nil {
		logging.SystemShards("[RequirementsInterrogator] [ERROR] JIT compilation failed: %v", err)
		return ""
	}
	if jitPrompt == "" {
		logging.SystemShards("[RequirementsInterrogator] [ERROR] JIT returned empty prompt")
		return ""
	}

	logging.SystemShards("[RequirementsInterrogator] [JIT] Using JIT-compiled system prompt (%d bytes)", len(jitPrompt))
	return jitPrompt
}

// Execute generates a concise set of clarifying questions for the given task/goal.
func (s *RequirementsInterrogatorShard) Execute(ctx context.Context, task string) (string, error) {
	s.SetState(types.ShardStateRunning)
	defer s.SetState(types.ShardStateCompleted)

	task = strings.TrimSpace(task)
	if task == "" {
		return "No task provided. Example: `/clarify build a refactor campaign for authentication`.", nil
	}

	s.mu.RLock()
	llmClient := s.llmClient
	s.mu.RUnlock()

	if llmClient == nil {
		// Fallback: static question template (no LLM available)
		return s.renderQuestions(task, []string{
			"What is the exact scope and success definition?",
			"What files/modules are in or out of scope?",
			"What constraints (performance, security, style, deadlines) apply?",
			"What tests or acceptance criteria must pass?",
			"Who signs off and what format do you want for the deliverable?",
		}), nil
	}

	// Get JIT-compiled system prompt (no fallback constant)
	systemPrompt := s.getSystemPrompt(ctx)
	if systemPrompt == "" {
		// If JIT fails, return error rather than using hardcoded prompt
		return "", fmt.Errorf("JIT prompt compilation failed - ensure atoms exist in internal/prompt/atoms/system/requirements_interrogator.yaml")
	}

	userPrompt := s.buildUserPrompt(task)
	rawResp, err := llmClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	// Process through Piggyback Protocol - extract surface response
	processed := articulation.ProcessLLMResponseAllowPlain(rawResp)
	resp := processed.Surface

	questions := s.extractQuestions(resp)
	if len(questions) == 0 {
		questions = []string{resp}
	}

	return s.renderQuestions(task, questions), nil
}

// buildUserPrompt builds the user prompt from the goal plus contextual hints.
func (s *RequirementsInterrogatorShard) buildUserPrompt(task string) string {
	var sb strings.Builder
	sb.WriteString("User goal:\n")
	sb.WriteString(task)
	sb.WriteString("\n\nContext:\n")
	sb.WriteString("- codeNERD stack: Cartographer (code graph), Dreamer (precog safety), Legislator (rules), campaigns (multi-phase autonomy)\n")
	sb.WriteString("- Ask for: goal type (greenfield/refactor/bug/ops), in/out of scope, constraints (perf/security/compliance/style/tooling), environments, data/privacy, external deps/APIs, owners/approvers, timelines, autonomy level, deliverables/DoD, acceptance tests, risks/rollback, success metrics.\n")
	sb.WriteString("- Code focus: target repos/files/modules, APIs/interfaces, migrations, test suites required.\n")
	sb.WriteString("- Autonomy: whether to run hands-free; check-in cadence; forbidden actions.\n")
	sb.WriteString("- Output now: only clarifying questions. No plan, no blueprint.\n")
	return sb.String()
}

func (s *RequirementsInterrogatorShard) extractQuestions(resp string) []string {
	lines := strings.Split(resp, "\n")
	var qs []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "-*1234567890. ")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		qs = append(qs, line)
	}
	return qs
}

func (s *RequirementsInterrogatorShard) renderQuestions(task string, questions []string) string {
	var sb strings.Builder
	sb.WriteString("## Requirements Interrogation\n\n")
	sb.WriteString(fmt.Sprintf("**Goal**: %s\n\n", task))
	sb.WriteString("Answer these to proceed:\n")
	for _, q := range questions {
		sb.WriteString(fmt.Sprintf("- %s\n", q))
	}
	return sb.String()
}

// NOTE: Legacy requirementsInterrogatorSystemPrompt constant has been DELETED.
// Requirements interrogator system prompts are now JIT-compiled from:
//   internal/prompt/atoms/system/requirements_interrogator.yaml
