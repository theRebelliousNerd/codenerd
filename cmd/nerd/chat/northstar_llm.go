// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains LLM integration functions for the Northstar wizard.
package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/prompt"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// NORTHSTAR LLM INTEGRATION
// =============================================================================

// generateRequirementsWithLLM uses the LLM to intelligently generate requirements.
func (m Model) generateRequirementsWithLLM() tea.Cmd {
	return func() tea.Msg {
		w := m.northstarWizard

		// Build context for LLM
		var contextBuilder strings.Builder
		contextBuilder.WriteString("## Project Vision\n")
		contextBuilder.WriteString(fmt.Sprintf("Mission: %s\n\n", w.Mission))
		contextBuilder.WriteString(fmt.Sprintf("Problem: %s\n\n", w.Problem))
		contextBuilder.WriteString(fmt.Sprintf("Vision: %s\n\n", w.Vision))

		if len(w.Capabilities) > 0 {
			contextBuilder.WriteString("## Capabilities\n")
			for _, cap := range w.Capabilities {
				contextBuilder.WriteString(fmt.Sprintf("- [%s/%s] %s\n", cap.Timeline, cap.Priority, cap.Description))
			}
			contextBuilder.WriteString("\n")
		}

		if len(w.Risks) > 0 {
			contextBuilder.WriteString("## Risks\n")
			for _, risk := range w.Risks {
				contextBuilder.WriteString(fmt.Sprintf("- [%s/%s] %s\n", risk.Likelihood, risk.Impact, risk.Description))
				if risk.Mitigation != "" && risk.Mitigation != "none" {
					contextBuilder.WriteString(fmt.Sprintf("  Mitigation: %s\n", risk.Mitigation))
				}
			}
			contextBuilder.WriteString("\n")
		}

		if len(w.Personas) > 0 {
			contextBuilder.WriteString("## User Personas\n")
			for _, p := range w.Personas {
				contextBuilder.WriteString(fmt.Sprintf("- %s\n", p.Name))
				contextBuilder.WriteString(fmt.Sprintf("  Needs: %s\n", strings.Join(p.Needs, ", ")))
			}
			contextBuilder.WriteString("\n")
		}

		if len(w.ExtractedFacts) > 0 {
			contextBuilder.WriteString("## Research Insights\n")
			for _, fact := range w.ExtractedFacts {
				contextBuilder.WriteString(fmt.Sprintf("- %s\n", fact))
			}
			contextBuilder.WriteString("\n")
		}

		contextBuilder.WriteString(`
Based on the above context, generate concrete, actionable requirements.

For each requirement, provide:
1. A clear description
2. Type: functional, non-functional, or constraint
3. Priority: must-have, should-have, or nice-to-have
4. Rationale: why this requirement exists

Format each requirement as:
REQ-NNN | TYPE | PRIORITY | Description | Rationale

Generate between 5-15 requirements focusing on:
- Core functionality from capabilities
- Risk mitigations
- User needs from personas
- Constraints and non-functional requirements (performance, security, usability)`)

		ctx, cancel := context.WithTimeout(context.Background(), northstarLLMTimeout())
		defer cancel()

		// Build prompt using helper (supports JIT if available)
		systemPrompt, userPrompt := m.buildNorthstarPrompt(ctx, "requirements", contextBuilder.String())

		response, err := m.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			return requirementsGeneratedMsg{err: err}
		}

		// Parse LLM response into requirements
		requirements := parseGeneratedRequirements(response, len(w.Requirements))
		return requirementsGeneratedMsg{requirements: requirements}
	}
}

// buildNorthstarPrompt constructs system and user prompts for northstar LLM operations.
// It attempts to use the JIT compiler if available, otherwise falls back to static prompts.
func (m Model) buildNorthstarPrompt(ctx context.Context, phase, content string) (systemPrompt, userPrompt string) {
	// Prefer JIT compilation if available (northstar atoms are phase-tagged)
	if m.jitCompiler != nil {
		cc := m.buildNorthstarCompilationContext(phase)
		if res, err := m.jitCompiler.Compile(ctx, cc); err == nil && res != nil && strings.TrimSpace(res.Prompt) != "" {
			return res.Prompt, content
		}
	}

	// Fallback: static prompts based on phase
	return getNorthstarStaticPrompt(phase), content
}

// buildNorthstarCompilationContext creates a CompilationContext for the northstar phase.
func (m Model) buildNorthstarCompilationContext(phase string) *prompt.CompilationContext {
	cc := prompt.NewCompilationContext()

	// Set northstar phase
	cc.NorthstarPhase = "/" + phase

	// Set operational mode to planning
	cc.OperationalMode = "/planning"

	// Set shard type to researcher (for document analysis)
	cc.ShardType = "/researcher"
	cc.ShardID = "northstar_wizard"
	cc.ShardName = "Northstar Wizard"

	// Set intent verb
	cc.IntentVerb = "/research"

	// Token budget (80k for prompt, 20k for response)
	cc.TokenBudget = 100000
	cc.ReservedTokens = 20000

	return cc
}

// getNorthstarStaticPrompt returns a static prompt for the given northstar phase.
// This is used as a fallback when JIT compilation is not available.
func getNorthstarStaticPrompt(phase string) string {
	switch phase {
	case "doc_ingestion":
		return `You are analyzing research documents to extract key insights for defining a project's vision and requirements.

Analyze the provided documents and extract key insights in the following categories:
- Problem statements or pain points mentioned
- Target users or personas described
- Desired capabilities or features
- Risks or concerns raised
- Constraints or limitations noted
- Success criteria or goals

Return ONLY the extracted insights, one per line. Be concise but specific. Maximum 15 insights.`

	case "requirements":
		return `You are helping generate concrete requirements from a project's vision, capabilities, and risks.

Based on the provided context, generate specific, actionable requirements that:
- Are testable and measurable
- Derive from stated capabilities and risk mitigations
- Address user needs and pain points
- Are categorized as functional, non-functional, or constraints

Return requirements in a structured format with clear prioritization.`

	default:
		return `You are assisting with project vision and planning. Provide clear, actionable insights based on the context provided.`
	}
}

func northstarLLMTimeout() time.Duration {
	timeout := config.GetLLMTimeouts().PerCallTimeout
	if timeout <= 0 {
		return 10 * time.Minute
	}
	return timeout
}

// analyzeNorthstarDocs reads and analyzes research documents using the LLM
// to extract key insights for the northstar definition process.
func (m Model) analyzeNorthstarDocs(docPaths []string) tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return northstarDocsAnalyzedMsg{err: fmt.Errorf("LLM client not available")}
		}

		// Read all documents
		var docContents strings.Builder
		for _, path := range docPaths {
			content, err := os.ReadFile(path)
			if err != nil {
				continue // Skip unreadable files
			}
			docContents.WriteString(fmt.Sprintf("\n--- Document: %s ---\n", filepath.Base(path)))
			// Truncate very large files
			text := string(content)
			if len(text) > 10000 {
				text = text[:10000] + "\n... [truncated]"
			}
			docContents.WriteString(text)
			docContents.WriteString("\n")
		}

		if docContents.Len() == 0 {
			return northstarDocsAnalyzedMsg{err: fmt.Errorf("no documents could be read")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), northstarLLMTimeout())
		defer cancel()

		// Build prompt using helper (supports JIT if available)
		systemPrompt, userPrompt := m.buildNorthstarPrompt(ctx, "doc_ingestion", docContents.String())

		response, err := m.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			return northstarDocsAnalyzedMsg{err: err}
		}

		// Parse insights from response
		var insights []string
		for _, line := range strings.Split(response, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				// Remove leading bullets or numbers
				line = strings.TrimPrefix(line, "- ")
				line = strings.TrimPrefix(line, "* ")
				if len(line) > 2 && line[1] == '.' {
					line = strings.TrimSpace(line[2:])
				}
				if line != "" {
					insights = append(insights, line)
				}
			}
		}

		return northstarDocsAnalyzedMsg{facts: insights}
	}
}
