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
		Policies: copyPolicySet("coder"),
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
			"browser_observe",
			"browser_act",
			"browser_mangle",
			"browser_wait",
			"browser_reason",
		},
		Policies: copyPolicySet("tester"),
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
		Policies: copyPolicySet("reviewer"),
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
			"browser_observe",
			"browser_act",
			"browser_mangle",
			"browser_wait",
			"browser_reason",
			"research_cache_get",
			"research_cache_set",
		},
		Policies: copyPolicySet("researcher"),
		Priority: 10,
	}
	registry.Register("/researcher", researcherAtom)
	registry.Register("/research", researcherAtom)
}
