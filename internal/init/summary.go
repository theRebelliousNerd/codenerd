package init

import (
	"fmt"
	"strings"
)

// printSummary prints structural results and separately labels enrichment.
func (i *Initializer) printSummary(result *InitResult, profile ProjectProfile) {
	fmt.Fprintln(i.output(), "\n"+strings.Repeat("═", 60))
	if result.Success && result.LLMMetrics.Failed == 0 {
		fmt.Fprintln(i.output(), "✅ INITIALIZATION COMPLETE")
	} else if result.Success {
		fmt.Fprintln(i.output(), "⚠️ INITIALIZATION COMPLETE (ENRICHMENT DEGRADED)")
	} else {
		fmt.Fprintln(i.output(), "❌ INITIALIZATION INCOMPLETE")
	}
	fmt.Fprintln(i.output(), strings.Repeat("═", 60))

	fmt.Fprintf(i.output(), "\n📁 Project: %s\n", profile.Name)
	fmt.Fprintf(i.output(), "   Language: %s\n", profile.Language)
	if profile.Framework != "" {
		fmt.Fprintf(i.output(), "   Framework: %s\n", profile.Framework)
	}
	fmt.Fprintf(i.output(), "   Architecture: %s\n", profile.Architecture)
	fmt.Fprintf(i.output(), "   Files: %d | Directories: %d\n", profile.FileCount, profile.DirectoryCount)

	fmt.Fprintf(i.output(), "\n🧠 Logic Kernel:\n")
	fmt.Fprintf(i.output(), "   Facts generated: %d\n", result.FactsGenerated)

	if len(result.CreatedAgents) > 0 {
		fmt.Fprintf(i.output(), "\n🤖 Type 3 Agents Created:\n")
		for _, agent := range result.CreatedAgents {
			fmt.Fprintf(i.output(), "   • %s (%d KB atoms) - %s\n", agent.Name, agent.KBSize, agent.Status)
		}
	}

	if result.LLMMetrics.Attempts > 0 {
		fmt.Fprintf(i.output(), "\n🤖 LLM Enrichment: %d/%d succeeded; %d failed\n",
			result.LLMMetrics.Succeeded, result.LLMMetrics.Attempts, result.LLMMetrics.Failed)
		for _, row := range result.LLMMetrics.Clients {
			fmt.Fprintf(i.output(), "   • %s client %s: %d/%d succeeded (%s)\n",
				row.Slot, row.Identity(), row.Succeeded, row.Attempts, strings.Join(row.Phases, ", "))
		}
	} else {
		fmt.Fprintln(i.output(), "\n🤖 LLM Enrichment: no calls attempted")
	}

	if n := len(result.ProjectTools); n > 0 {
		fmt.Fprintf(i.output(), "\n🛠️  Project command tools: %s\n", strings.Join(result.ProjectTools, ", "))
	}

	fmt.Fprintf(i.output(), "\n📂 Files Created: %d\n", len(result.FilesCreated))
	fmt.Fprintf(i.output(), "⏱️ Duration: %.2fs\n", result.Duration.Seconds())

	if len(result.Warnings) > 0 {
		fmt.Fprintln(i.output(), "\n⚠️ Warnings:")
		for _, w := range result.Warnings {
			fmt.Fprintf(i.output(), "   - %s\n", w)
		}
	}
	if len(result.Failures) > 0 {
		fmt.Fprintln(i.output(), "\n❌ Required initialization failures:")
		for _, failure := range result.Failures {
			fmt.Fprintf(i.output(), "   - %s\n", failure)
		}
	}

	// Post-init recommendations based on project analysis
	fmt.Fprintln(i.output(), "\n"+strings.Repeat("─", 60))
	fmt.Fprintln(i.output(), "💡 Recommendations:")
	i.printRecommendations(result, profile)

	// Run post-init validation
	fmt.Fprintln(i.output(), "\n"+strings.Repeat("─", 60))
	fmt.Fprintln(i.output(), "🔍 Validating knowledge bases...")
	validationSummary := result.Validation
	if validationSummary == nil {
		fmt.Fprintln(i.output(), "   ✗ Structural validation did not return a result")
	} else {
		if validationSummary.TotalDBs == 0 {
			fmt.Fprintln(i.output(), "   ⓘ No specialist knowledge bases to validate")
		} else if validationSummary.OverallValid {
			fmt.Fprintf(i.output(), "   ✓ All %d knowledge bases structurally validated\n", validationSummary.TotalDBs)
		} else {
			fmt.Fprintf(i.output(), "   ⚠ %d/%d knowledge bases have issues\n", validationSummary.InvalidDBs, validationSummary.TotalDBs)
			for name, res := range validationSummary.Results {
				if !res.Valid {
					fmt.Fprintf(i.output(), "     - %s: %v\n", name, res.Errors)
				}
			}
		}

		// Report backup files
		if len(validationSummary.BackupFiles) > 0 {
			fmt.Fprintf(i.output(), "\n   📦 Found %d backup files from migration\n", len(validationSummary.BackupFiles))
			fmt.Fprintln(i.output(), "      After verifying your data, clean them up with:")
			fmt.Fprintln(i.output(), "      nerd init --cleanup-backups")
		}
	}

	fmt.Fprintln(i.output(), "\n"+strings.Repeat("─", 60))
	if result.Success {
		fmt.Fprintln(i.output(), "🚀 Next steps:")
		fmt.Fprintln(i.output(), "   • Run `nerd chat` to start interactive session")
		fmt.Fprintln(i.output(), "   • Review the derived orientation and project vision")
		fmt.Fprintln(i.output(), "   • Use `/agents` to see available agents")
		fmt.Fprintln(i.output(), "   • Use `/spawn <agent> <task>` to delegate tasks")
	} else {
		fmt.Fprintln(i.output(), "🛠️ Recovery:")
		fmt.Fprintln(i.output(), "   • Resolve the required failures above")
		fmt.Fprintln(i.output(), "   • Re-run `nerd init --force` before starting chat")
	}
	fmt.Fprintln(i.output(), strings.Repeat("─", 60))
}

// printRecommendations prints next steps that follow from what this run
// measured. It used to recommend fixed agents (GoExpert, SecurityAuditor,
// TestArchitect) whatever the orientation policy had derived or the user had
// curated away, and a KB-population branch that could never fire.
func (i *Initializer) printRecommendations(result *InitResult, profile ProjectProfile) {
	var recommendations []string
	for _, agent := range result.CreatedAgents {
		if agent.KBSize == 0 {
			recommendations = append(recommendations,
				fmt.Sprintf("%s has an empty knowledge base; run `nerd init --force` once research is reachable", agent.Name))
		}
	}
	if result.LLMMetrics.Attempts == 0 && i.config.LLMClient == nil && i.config.PlannerClient == nil {
		recommendations = append(recommendations,
			"No LLM client was configured: orientation, the north star and specialist prompts were not derived; configure a provider in .nerd/config.json and re-run `nerd init --force`")
	} else if result.LLMMetrics.Failed > 0 {
		recommendations = append(recommendations,
			"Some LLM enrichment failed (see warnings); `nerd init --force` re-runs only what is missing or changed")
	}
	if i.config.SkipResearch {
		recommendations = append(recommendations, "Research was skipped - run `nerd init --force` to populate agent KBs")
	}
	if !result.VisionDerived {
		recommendations = append(recommendations, "No north star was installed; review .nerd/orientation/README.md and set one with `/northstar`")
	}

	for _, rec := range recommendations {
		fmt.Fprintf(i.output(), "   • %s\n", rec)
	}
	if len(recommendations) == 0 {
		fmt.Fprintln(i.output(), "   • Your project is ready! Start with `/review` or `/test`")
	}
}
