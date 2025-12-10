// Package reviewer provides code review functionality with multi-shard orchestration.
// This file contains LLM prompts for the creative enhancement pipeline.
package reviewer

import (
	"fmt"
	"strings"
)

// buildFirstPassPrompt creates the prompt for initial creative analysis.
func (r *ReviewerShard) buildFirstPassPrompt(
	fileContents map[string]string,
	holoCtx *HolographicContext,
	findings []ReviewFinding,
) string {
	var sb strings.Builder

	sb.WriteString(`You are a senior software architect performing creative code analysis.

Your task is to generate actionable improvement suggestions at multiple levels:
1. FILE-LEVEL: Specific improvements to individual files
2. MODULE-LEVEL: Improvements to packages/modules as a whole
3. SYSTEM-LEVEL: Architectural and cross-cutting insights
4. FEATURE IDEAS: New capabilities that would enhance the codebase

Guidelines:
- Focus on actionable, specific suggestions
- Consider maintainability, performance, security, and readability
- Don't repeat issues already found in the review findings
- Be creative but practical
- Rate effort as: trivial, small, medium, large

`)

	// Add existing findings for context (to avoid duplicates)
	if len(findings) > 0 {
		sb.WriteString("## Existing Review Findings (DO NOT DUPLICATE)\n\n")
		for _, f := range findings {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", f.Severity, f.File, f.Message))
		}
		sb.WriteString("\n")
	}

	// Add holographic context if available
	if holoCtx != nil {
		sb.WriteString("## Project Context\n\n")
		sb.WriteString(fmt.Sprintf("- **Target Package:** %s\n", holoCtx.TargetPkg))
		sb.WriteString(fmt.Sprintf("- **Layer:** %s\n", holoCtx.Layer))
		sb.WriteString(fmt.Sprintf("- **Module:** %s\n", holoCtx.Module))
		sb.WriteString(fmt.Sprintf("- **Role:** %s\n", holoCtx.Role))
		if holoCtx.SystemPurpose != "" {
			sb.WriteString(fmt.Sprintf("- **System Purpose:** %s\n", holoCtx.SystemPurpose))
		}
		sb.WriteString(fmt.Sprintf("- **Has Tests:** %v\n\n", holoCtx.HasTests))

		if len(holoCtx.PackageSiblings) > 0 {
			sb.WriteString("### Package Siblings\n")
			for _, sib := range holoCtx.PackageSiblings {
				sb.WriteString(fmt.Sprintf("- %s\n", sib))
			}
			sb.WriteString("\n")
		}

		if len(holoCtx.PackageSignatures) > 0 {
			sb.WriteString("### Package Signatures\n")
			for _, sig := range holoCtx.PackageSignatures {
				exported := ""
				if sig.Exported {
					exported = " (exported)"
				}
				if sig.Receiver != "" {
					sb.WriteString(fmt.Sprintf("- `(%s) %s%s%s`%s\n", sig.Receiver, sig.Name, sig.Params, sig.Returns, exported))
				} else {
					sb.WriteString(fmt.Sprintf("- `%s%s%s`%s\n", sig.Name, sig.Params, sig.Returns, exported))
				}
			}
			sb.WriteString("\n")
		}

		if len(holoCtx.PackageTypes) > 0 {
			sb.WriteString("### Package Types\n")
			for _, t := range holoCtx.PackageTypes {
				sb.WriteString(fmt.Sprintf("- `%s` (%s)\n", t.Name, t.Kind))
			}
			sb.WriteString("\n")
		}
	}

	// Add file contents
	sb.WriteString("## Files to Analyze\n\n")
	for path, content := range fileContents {
		sb.WriteString(fmt.Sprintf("### %s\n\n```\n", path))
		// Truncate very long files
		if len(content) > 10000 {
			sb.WriteString(content[:10000])
			sb.WriteString("\n... (truncated)\n")
		} else {
			sb.WriteString(content)
		}
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString(`## Output Format

Respond with a JSON object in this exact format:
{
  "file_suggestions": [
    {
      "file": "path/to/file.go",
      "category": "refactor|performance|readability|testing|error_handling",
      "title": "Brief title",
      "description": "Detailed explanation",
      "code_example": "optional code snippet",
      "effort": "trivial|small|medium|large"
    }
  ],
  "module_suggestions": [
    {
      "package": "package/path",
      "category": "api_design|coupling|cohesion|naming|abstraction",
      "title": "Brief title",
      "description": "Detailed explanation",
      "affected_files": ["file1.go", "file2.go"],
      "effort": "trivial|small|medium|large"
    }
  ],
  "system_insights": [
    {
      "category": "architecture|security|scalability|maintainability",
      "title": "Brief title",
      "description": "Detailed explanation",
      "impact": "low|medium|high",
      "related_modules": ["module1", "module2"]
    }
  ],
  "feature_ideas": [
    {
      "title": "Feature name",
      "description": "What it does",
      "rationale": "Why it would help",
      "complexity": "trivial|small|medium|large|epic",
      "prerequisites": ["optional", "list"]
    }
  ]
}

Generate 3-5 suggestions per category where applicable. Be specific and actionable.
`)

	return sb.String()
}

// buildSelfQuestionPrompt creates the prompt for generating clarifying questions.
func (r *ReviewerShard) buildSelfQuestionPrompt(
	firstPass *CreativeFirstPass,
	inspiration []PastSuggestion,
) string {
	var sb strings.Builder

	sb.WriteString(`You are a Requirements Interrogator examining code improvement suggestions.

Your task is to generate 3-5 clarifying questions that would help refine and improve these suggestions. These questions will be answered by analyzing the code context - they are NOT for the user.

Focus on questions that would:
- Clarify implementation constraints
- Identify potential conflicts or dependencies
- Validate assumptions about the codebase
- Explore alternative approaches
- Uncover hidden complexity

`)

	// Add first pass suggestions
	sb.WriteString("## Initial Suggestions to Examine\n\n")

	if len(firstPass.FileSuggestions) > 0 {
		sb.WriteString("### File Suggestions\n")
		for _, fs := range firstPass.FileSuggestions {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", fs.Category, fs.Title, fs.Description))
		}
		sb.WriteString("\n")
	}

	if len(firstPass.ModuleSuggestions) > 0 {
		sb.WriteString("### Module Suggestions\n")
		for _, ms := range firstPass.ModuleSuggestions {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", ms.Category, ms.Title, ms.Description))
		}
		sb.WriteString("\n")
	}

	if len(firstPass.SystemInsights) > 0 {
		sb.WriteString("### System Insights\n")
		for _, si := range firstPass.SystemInsights {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", si.Category, si.Title, si.Description))
		}
		sb.WriteString("\n")
	}

	if len(firstPass.FeatureIdeas) > 0 {
		sb.WriteString("### Feature Ideas\n")
		for _, fi := range firstPass.FeatureIdeas {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", fi.Title, fi.Description))
		}
		sb.WriteString("\n")
	}

	// Add inspiration from past suggestions
	if len(inspiration) > 0 {
		sb.WriteString("## Inspiration from Past Reviews\n\n")
		for _, ps := range inspiration {
			status := "not implemented"
			if ps.WasImplemented {
				status = "implemented"
			}
			sb.WriteString(fmt.Sprintf("- (%.0f%% similar, %s) %s\n", ps.Similarity*100, status, ps.Summary))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`## Output Format

Respond with a JSON array of questions:
[
  {"question": "What are the current test coverage levels for the files being modified?"},
  {"question": "Are there any circular dependencies that would complicate this refactoring?"},
  {"question": "Does the suggested API change break backward compatibility?"}
]

Generate 3-5 specific, answerable questions.
`)

	return sb.String()
}

// buildSelfAnswerPrompt creates the prompt for answering self-interrogation questions.
func (r *ReviewerShard) buildSelfAnswerPrompt(
	question string,
	fileContents map[string]string,
	firstPass *CreativeFirstPass,
) string {
	var sb strings.Builder

	sb.WriteString(`You are analyzing code to answer a clarifying question about improvement suggestions.

## Question
`)
	sb.WriteString(question)
	sb.WriteString("\n\n")

	// Add relevant file contents (truncated for context)
	sb.WriteString("## Code Context\n\n")
	for path, content := range fileContents {
		sb.WriteString(fmt.Sprintf("### %s\n\n```\n", path))
		// More aggressive truncation for answers
		if len(content) > 5000 {
			sb.WriteString(content[:5000])
			sb.WriteString("\n... (truncated)\n")
		} else {
			sb.WriteString(content)
		}
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString(`## Instructions

Answer the question based ONLY on the code context provided. Be specific and cite evidence from the code.

If you cannot determine the answer from the provided context, say so.

Keep your answer concise (2-4 sentences).
`)

	return sb.String()
}

// buildSecondPassPrompt creates the prompt for enhanced creative synthesis.
func (r *ReviewerShard) buildSecondPassPrompt(
	fileContents map[string]string,
	holoCtx *HolographicContext,
	firstPass *CreativeFirstPass,
	inspiration []PastSuggestion,
	selfQA []SelfQuestion,
	findings []ReviewFinding,
) string {
	var sb strings.Builder

	sb.WriteString(`You are a senior software architect performing enhanced creative code analysis.

This is a SECOND PASS analysis. You have access to:
1. First pass suggestions (baseline)
2. Historically similar suggestions from past reviews
3. Self-interrogation Q&A that clarified key aspects

Your task is to SYNTHESIZE these inputs into refined, higher-quality suggestions.
- Build on the first pass insights
- Incorporate lessons from past suggestions (especially implemented ones)
- Use Q&A insights to resolve ambiguities
- Generate MORE SPECIFIC and ACTIONABLE suggestions

`)

	// Add first pass for reference
	sb.WriteString("## First Pass Suggestions (BASELINE)\n\n")

	if len(firstPass.FileSuggestions) > 0 {
		sb.WriteString("### File Suggestions\n")
		for _, fs := range firstPass.FileSuggestions {
			sb.WriteString(fmt.Sprintf("- [%s] %s in %s: %s (effort: %s)\n",
				fs.Category, fs.Title, fs.File, fs.Description, fs.Effort))
		}
		sb.WriteString("\n")
	}

	if len(firstPass.ModuleSuggestions) > 0 {
		sb.WriteString("### Module Suggestions\n")
		for _, ms := range firstPass.ModuleSuggestions {
			sb.WriteString(fmt.Sprintf("- [%s] %s in %s: %s (effort: %s)\n",
				ms.Category, ms.Title, ms.Package, ms.Description, ms.Effort))
		}
		sb.WriteString("\n")
	}

	if len(firstPass.SystemInsights) > 0 {
		sb.WriteString("### System Insights\n")
		for _, si := range firstPass.SystemInsights {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s (impact: %s)\n",
				si.Category, si.Title, si.Description, si.Impact))
		}
		sb.WriteString("\n")
	}

	if len(firstPass.FeatureIdeas) > 0 {
		sb.WriteString("### Feature Ideas\n")
		for _, fi := range firstPass.FeatureIdeas {
			sb.WriteString(fmt.Sprintf("- %s: %s (complexity: %s)\n",
				fi.Title, fi.Description, fi.Complexity))
		}
		sb.WriteString("\n")
	}

	// Add historical inspiration
	if len(inspiration) > 0 {
		sb.WriteString("## Historical Inspiration\n\n")
		sb.WriteString("Similar suggestions from past reviews:\n\n")
		for _, ps := range inspiration {
			status := "not implemented"
			if ps.WasImplemented {
				status = "IMPLEMENTED - consider similar approaches"
			}
			sb.WriteString(fmt.Sprintf("- (%.0f%% similar, %s)\n  %s\n\n",
				ps.Similarity*100, status, ps.Summary))
		}
	}

	// Add self-Q&A insights
	if len(selfQA) > 0 {
		sb.WriteString("## Self-Interrogation Insights\n\n")
		sb.WriteString("Key questions and answers that clarify the suggestions:\n\n")
		for _, qa := range selfQA {
			sb.WriteString(fmt.Sprintf("**Q: %s**\n", qa.Question))
			sb.WriteString(fmt.Sprintf("A: %s\n", qa.Answer))
			if qa.Insight != "" {
				sb.WriteString(fmt.Sprintf("*Insight: %s*\n", qa.Insight))
			}
			sb.WriteString("\n")
		}
	}

	// Add existing findings to avoid duplicates
	if len(findings) > 0 {
		sb.WriteString("## Existing Review Findings (DO NOT DUPLICATE)\n\n")
		for _, f := range findings {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", f.Severity, f.File, f.Message))
		}
		sb.WriteString("\n")
	}

	// Add file contents (truncated)
	sb.WriteString("## Files for Reference\n\n")
	for path, content := range fileContents {
		sb.WriteString(fmt.Sprintf("### %s\n\n```\n", path))
		if len(content) > 8000 {
			sb.WriteString(content[:8000])
			sb.WriteString("\n... (truncated)\n")
		} else {
			sb.WriteString(content)
		}
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString(`## Output Format

Generate REFINED suggestions in this JSON format:
{
  "file_suggestions": [
    {
      "file": "path/to/file.go",
      "category": "refactor|performance|readability|testing|error_handling",
      "title": "Brief title",
      "description": "Detailed explanation incorporating insights",
      "code_example": "optional code snippet showing the improvement",
      "effort": "trivial|small|medium|large",
      "inspired_by": "optional: ID of past suggestion that inspired this"
    }
  ],
  "module_suggestions": [...],
  "system_insights": [...],
  "feature_ideas": [...]
}

IMPORTANT:
- Generate MORE SPECIFIC suggestions than the first pass
- Include code examples where helpful
- Reference past implemented suggestions when applicable
- Use Q&A insights to validate and refine suggestions
- Aim for 3-7 suggestions per category
`)

	return sb.String()
}
