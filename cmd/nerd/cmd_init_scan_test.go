package main

import (
	"strings"
	"testing"

	nerdinit "codenerd/internal/init"
)

// The embedding engine is a hard prerequisite — init returns an error rather
// than degrading to an empty vector index — and it is configured only from the
// "embedding" block of config.json, never from --api-key or the chat provider.
// Operators had no way to learn that from the CLI, so it belongs in the help
// text and has to stay there.
func TestInitCmdHelp_WhenRead_ShouldStateEmbeddingPrerequisites(t *testing.T) {
	help := initCmd.Long

	for _, want := range []string{
		"embedding",
		".nerd/config.json",
		"ollama",
		"embeddinggemma:300m",
		"genai",
		"CGO",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("`nerd init --help` does not mention %q; operators cannot diagnose an embedding failure from the CLI", want)
		}
	}
	if !strings.Contains(strings.ToLower(help), "hard error") {
		t.Error("help text does not say embedding failure is a hard error rather than a degraded mode")
	}
}

func TestInitCmdFlags_WhenRegistered_ShouldExposeAgentCurationFlags(t *testing.T) {
	for _, name := range []string{"define-agent", "no-interactive"} {
		if initCmd.Flags().Lookup(name) == nil {
			t.Errorf("`nerd init` has no --%s flag", name)
		}
	}
}

// --define-agent parses through the same Type U validator init uses, so a
// malformed value is rejected before a workspace is written.
func TestDefineAgentFlag_WhenParsed_ShouldProduceTypeUAgentsOrErrors(t *testing.T) {
	defs, errs := nerdinit.ParseTypeUAgentFlags([]string{
		"K8sExpert:Kubernetes deployment specialist:kubernetes,helm,kubectl",
	})
	if len(errs) != 0 {
		t.Fatalf("valid definition rejected: %v", errs)
	}
	if len(defs) != 1 || defs[0].Name != "K8sExpert" || len(defs[0].Topics) != 3 {
		t.Fatalf("parsed definition = %+v", defs)
	}

	if _, errs := nerdinit.ParseTypeUAgentFlags([]string{"NoTopics:just a role"}); len(errs) != 1 {
		t.Fatalf("malformed definition should produce exactly one error, got %v", errs)
	}
}
