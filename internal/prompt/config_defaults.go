package prompt

// RegisterDefaultConfigAtoms populates the registry with standard codeNERD configurations.
func RegisterDefaultConfigAtoms(registry *SimpleRegistry) {
	// Coder Intent (/coder or /fix, /refactor, /create)
	coderAtom := ConfigAtom{
		Tools: []string{
			"read_file",
			"write_file",
			"replace",
			"list_directory",
			"search_file_content",
			"run_shell_command",
		},
		Policies: []string{
			"base.mg",
			"policy/coder_classification.mg",
			"policy/coder_language.mg",
			"policy/coder_impact.mg",
			"policy/coder_safety.mg",
			"policy/coder_diagnostics.mg",
			"policy/coder_workflow.mg",
			"policy/coder_context.mg",
			"policy/coder_tdd.mg",
			"policy/coder_quality.mg",
			"policy/coder_learning.mg",
			"policy/coder_campaign.mg",
			"policy/coder_observability.mg",
			"policy/coder_patterns.mg",
		},
		Priority: 10,
	}
	registry.Register("/coder", coderAtom)
	registry.Register("/fix", coderAtom)
	registry.Register("/refactor", coderAtom)
	registry.Register("/create", coderAtom)

	// Tester Intent (/tester or /test)
	testerAtom := ConfigAtom{
		Tools: []string{
			"read_file",
			"run_shell_command",
		},
		Policies: []string{
			"base.mg",
			"tester.mg",
		},
		Priority: 10,
	}
	registry.Register("/tester", testerAtom)
	registry.Register("/test", testerAtom)

	// Reviewer Intent (/reviewer or /review)
	reviewerAtom := ConfigAtom{
		Tools: []string{
			"read_file",
			"list_directory",
			"search_file_content",
		},
		Policies: []string{
			"base.mg",
			"reviewer.mg",
		},
		Priority: 10,
	}
	registry.Register("/reviewer", reviewerAtom)
	registry.Register("/review", reviewerAtom)

	// Researcher Intent (/researcher or /research)
	researcherAtom := ConfigAtom{
		Tools: []string{
			"context7_fetch",
			"web_search",
			"web_fetch",
			"browser_navigate",
			"browser_extract",
			"research_cache_get",
			"research_cache_set",
		},
		Policies: []string{
			"base.mg",
			"researcher.mg",
		},
		Priority: 10,
	}
	registry.Register("/researcher", researcherAtom)
	registry.Register("/research", researcherAtom)
}
