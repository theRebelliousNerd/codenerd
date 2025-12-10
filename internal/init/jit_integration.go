// Package init - JIT Prompt Compiler Integration
// This file implements JIT prompt compilation for init phases.
package init

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"
)

// assembleJITPrompt attempts to use the JIT compiler to generate a prompt for an init phase.
// Falls back to a simple prompt if JIT compilation fails.
func (i *Initializer) assembleJITPrompt(ctx context.Context, phase, task string, profile *ProjectProfile) (string, error) {
	// Check if we have a JIT compiler available
	if i.config.LLMClient == nil {
		return "", fmt.Errorf("no LLM client available for prompt assembly")
	}

	// Try to create JIT compiler
	jitCompiler, err := i.createJITCompiler()
	if err != nil {
		logging.Boot("Failed to create JIT compiler: %v, using fallback", err)
		return i.buildFallbackPrompt(phase, task), nil
	}

	// Build compilation context for this phase
	cc := BuildInitCompilationContext(phase, task, profile)

	// Compile the prompt
	result, err := jitCompiler.Compile(ctx, cc)
	if err != nil {
		logging.Boot("JIT prompt compilation failed for phase %s: %v, using fallback", phase, err)
		return i.buildFallbackPrompt(phase, task), nil
	}

	logging.Boot("JIT compiled prompt for init phase %s: %d bytes, %d atoms", phase, len(result.Prompt), result.AtomsIncluded)

	// Combine compiled prompt with task
	fullPrompt := fmt.Sprintf("%s\n\nTask: %s", result.Prompt, task)
	return fullPrompt, nil
}

// createJITCompiler creates a JIT prompt compiler for init phases.
// This is a lazy initialization - we only create it when needed.
func (i *Initializer) createJITCompiler() (*prompt.JITPromptCompiler, error) {
	// Create init-specific atoms
	// These are hardcoded for now - in production, these would come from an embedded corpus
	atoms := []*prompt.PromptAtom{
		prompt.NewPromptAtom(
			"init_analysis_guidance",
			prompt.CategoryInit,
			"When analyzing codebases during initialization, focus on: language detection, framework identification, dependency mapping, and architectural patterns. Provide concise 2-3 sentence summaries.",
		),
		prompt.NewPromptAtom(
			"init_profile_guidance",
			prompt.CategoryInit,
			"When generating project profiles, extract: project name, language, framework, build system, entry points, test directories, and architectural patterns. Structure as JSON-compatible data.",
		),
		prompt.NewPromptAtom(
			"init_facts_guidance",
			prompt.CategoryInit,
			"When generating Mangle facts, use proper syntax: predicates are lowercase_with_underscores, name constants start with /, strings are quoted, statements end with periods. Example: project_language(/go).",
		),
		prompt.NewPromptAtom(
			"init_kb_agent_guidance",
			prompt.CategoryInit,
			"When creating agent knowledge bases, research topics deeply and extract: core concepts, best practices, code examples, anti-patterns, and common gotchas. Aim for 20-50 quality atoms per agent.",
		),
		prompt.NewPromptAtom(
			"init_agents_guidance",
			prompt.CategoryInit,
			"When recommending agents, analyze project dependencies and architecture. Suggest language experts, framework experts, and domain specialists. Prioritize agents that provide high-value knowledge.",
		),
		prompt.NewPromptAtom(
			"researcher_core_mission",
			prompt.CategoryIdentity,
			"You are the ResearcherShard, a deep research specialist. Your purpose: gather knowledge from documentation, analyze codebases, and build knowledge bases for specialist agents. Prioritize accuracy, relevance, and conciseness.",
		),
	}

	// Configure atom selectors
	atoms[0].Subcategory = "init"
	atoms[0].InitPhases = []string{"/analysis"}
	atoms[0].ShardTypes = []string{"/researcher"}
	atoms[0].Priority = 80

	atoms[1].Subcategory = "init"
	atoms[1].InitPhases = []string{"/profile"}
	atoms[1].ShardTypes = []string{"/researcher"}
	atoms[1].Priority = 80

	atoms[2].Subcategory = "init"
	atoms[2].InitPhases = []string{"/facts"}
	atoms[2].ShardTypes = []string{"/researcher"}
	atoms[2].Priority = 90
	atoms[2].IsMandatory = true

	atoms[3].Subcategory = "init"
	atoms[3].InitPhases = []string{"/kb_agent"}
	atoms[3].ShardTypes = []string{"/researcher"}
	atoms[3].Priority = 85

	atoms[4].Subcategory = "init"
	atoms[4].InitPhases = []string{"/agents"}
	atoms[4].ShardTypes = []string{"/researcher"}
	atoms[4].Priority = 75

	atoms[5].Subcategory = "mission"
	atoms[5].ShardTypes = []string{"/researcher"}
	atoms[5].Priority = 100
	atoms[5].IsMandatory = true

	// Create embedded corpus
	corpus := prompt.NewEmbeddedCorpus(atoms)

	// Create JIT compiler with embedded corpus
	// Note: We can't use WithKernel here because the kernel type doesn't match
	// the interface exactly (AssertBatch signature differs)
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithEmbeddedCorpus(corpus),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create JIT compiler: %w", err)
	}

	return compiler, nil
}

// buildFallbackPrompt creates a simple fallback prompt when JIT is unavailable.
func (i *Initializer) buildFallbackPrompt(phase, task string) string {
	basePrompt := `You are the ResearcherShard, a deep research specialist for codeNERD.

Your purpose: gather knowledge from documentation, analyze codebases, and build knowledge bases for specialist agents.

Core principles:
- Prioritize accuracy over speed
- Extract concrete, actionable knowledge
- Provide concise summaries (2-3 sentences)
- Focus on what's most useful for an AI coding agent
`

	// Phase-specific guidance
	phaseGuidance := map[string]string{
		"analysis": `
When analyzing codebases:
- Detect language, framework, and build system
- Identify architectural patterns
- Map key dependencies
- Locate entry points and test directories
- Provide a concise 2-3 sentence summary
`,
		"profile": `
When generating project profiles:
- Extract project metadata (name, language, framework)
- Identify build system and architecture
- List key dependencies with versions
- Structure as JSON-compatible data
`,
		"facts": `
When generating Mangle facts:
- Use proper syntax: lowercase_predicates, /name_constants, "quoted strings", ending periods
- Example: project_language(/go).
- Include project identity, language, framework, patterns
`,
		"agents": `
When recommending agents:
- Analyze project dependencies and tech stack
- Suggest language experts (GoExpert, PythonExpert, etc.)
- Suggest framework experts (WebAPIExpert, FrontendExpert, etc.)
- Suggest domain specialists (SecurityAuditor, TestArchitect, etc.)
- Prioritize high-value knowledge sources
`,
		"kb_agent": `
When creating agent knowledge bases:
- Research topics deeply (20-50 atoms per agent)
- Extract: core concepts, best practices, code examples, anti-patterns
- Focus on what makes the agent valuable
- Maintain high quality (score >= 0.5)
`,
		"kb_complete": `
When completing knowledge base creation:
- Ensure comprehensive coverage of agent's domain
- Verify atom quality (no duplicates, high relevance)
- Provide summary of knowledge coverage
`,
	}

	guidance := phaseGuidance[phase]
	if guidance == "" {
		guidance = "\n(No specific phase guidance available)\n"
	}

	return fmt.Sprintf("%s%s\nTask: %s", basePrompt, guidance, task)
}

// withJITPrompt wraps an LLM call with JIT prompt compilation.
// This is a helper for init phases that need to make LLM calls.
func (i *Initializer) withJITPrompt(ctx context.Context, phase, task string, profile *ProjectProfile, handler func(ctx context.Context, prompt string) (string, error)) (string, error) {
	// Assemble prompt using JIT (with fallback)
	prompt, err := i.assembleJITPrompt(ctx, phase, task, profile)
	if err != nil {
		logging.Boot("Failed to assemble JIT prompt: %v", err)
		// Use fallback
		prompt = i.buildFallbackPrompt(phase, task)
	}

	// Call the handler with the assembled prompt
	return handler(ctx, prompt)
}

// BuildInitCompilationContext creates a CompilationContext for an init phase.
// This can be used directly with the JIT compiler for advanced use cases.
func BuildInitCompilationContext(phase, task string, profile *ProjectProfile) *prompt.CompilationContext {
	cc := prompt.NewCompilationContext()

	// Set shard context (init uses researcher)
	cc.ShardType = "/researcher"
	cc.ShardID = "init_researcher"

	// Set init phase
	cc.InitPhase = "/" + phase

	// Set operational mode (init is always active)
	cc.OperationalMode = "/active"

	// Set language context if available
	if profile != nil {
		switch profile.Language {
		case "go", "Go", "golang":
			cc.Language = "/go"
		case "python", "Python":
			cc.Language = "/python"
		case "typescript", "TypeScript":
			cc.Language = "/typescript"
		case "javascript", "JavaScript":
			cc.Language = "/javascript"
		case "rust", "Rust":
			cc.Language = "/rust"
		}

		// Set framework if available
		if profile.Framework != "" {
			// Convert framework to selector format
			cc.Frameworks = []string{"/" + strings.ToLower(profile.Framework)}
		}
	}

	// Set token budget (init prompts can be larger)
	cc.TokenBudget = 120000
	cc.ReservedTokens = 10000

	// Use task as semantic query for vector search
	if task != "" {
		cc.SemanticQuery = task
	}

	return cc
}
