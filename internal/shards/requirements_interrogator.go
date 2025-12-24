package shards

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/articulation"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/types"
)

// RequirementsInterrogatorShard is a Socratic shard that elicits missing requirements.
// It is intended for early-phase clarification before kicking off campaigns or large tasks.
type RequirementsInterrogatorShard struct {
	*coreshards.BaseShardAgent
	llmClient types.LLMClient
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
	s.llmClient = client
	if s.BaseShardAgent != nil {
		s.BaseShardAgent.SetLLMClient(client)
	}
}

// Execute generates a concise set of clarifying questions for the given task/goal.
func (s *RequirementsInterrogatorShard) Execute(ctx context.Context, task string) (string, error) {
	s.SetState(types.ShardStateRunning)
	defer s.SetState(types.ShardStateCompleted)

	task = strings.TrimSpace(task)
	if task == "" {
		return "No task provided. Example: `/clarify build a refactor campaign for authentication`.", nil
	}

	if s.llmClient == nil {
		// Fallback: static question template
		return s.renderQuestions(task, []string{
			"What is the exact scope and success definition?",
			"What files/modules are in or out of scope?",
			"What constraints (performance, security, style, deadlines) apply?",
			"What tests or acceptance criteria must pass?",
			"Who signs off and what format do you want for the deliverable?",
		}), nil
	}

	userPrompt := s.buildUserPrompt(task)
	rawResp, err := s.llmClient.CompleteWithSystem(ctx, requirementsInterrogatorSystemPrompt, userPrompt)
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

// requirementsInterrogatorSystemPrompt is adapted from the imported spec, tuned for codeNERD.
// It enforces ASCII-only output and focuses on clarifying dialogue (not full blueprints).
const requirementsInterrogatorSystemPrompt = `
You are the Requirements Interrogator for codeNERD. Produce 8-12 sharp, implementation-ready questions to turn a vague goal into a concrete plan or campaign. Be thorough but concise.

Rules:
- ASCII only. No emojis or Unicode.
- Do NOT generate a full plan; ask questions only.
- Focus on: problem framing, goal type (greenfield/refactor/bug/ops), scope in/out, constraints (perf/security/compliance/style/tooling), environments, data/privacy, ownership and approvers, timelines, autonomy level, deliverables/DoD, acceptance tests, risks/rollback, success metrics.
- Include code-facing probes: target repos/files/modules, interfaces/APIs, external deps, critical migrations, expected test suites.
- Include campaign/autonomy probes: hands-free vs checkpoints, budget/guardrails, allowed/forbidden actions.
- Align with codeNERD architecture: Cartographer (code graph), Dreamer (precog safety), Legislator (rules), campaigns (multi-phase execution), shards (coder/tester/reviewer/researcher).
- Challenge assumptions politely; keep it concise.
`
