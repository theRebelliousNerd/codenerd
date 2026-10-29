package system

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/prompt"
)

// perceptionAtomAssembler serves the firewall's classification prompt built
// from the shipped perception atoms. The firewall's transducer has no embedded
// prompt: without a ready assembler a turn degrades with
// ErrRolePromptUnavailable. This package cannot import internal/system (it
// imports this one) for the kernel adapter a real JIT compile needs; the real
// compile is pinned in internal/perception/perception_jit_contract_test.go.
type perceptionAtomAssembler struct{ prompt string }

func (a perceptionAtomAssembler) AssembleSystemPrompt(context.Context, string, string) (string, error) {
	return a.prompt, nil
}

func (a perceptionAtomAssembler) JITReady() bool { return true }

// attachPerceptionPrompt wires the perception atoms' prompt into the shard's
// transducer, the state a registered firewall reaches.
func attachPerceptionPrompt(t *testing.T, shard *PerceptionFirewallShard) {
	t.Helper()
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	var parts []string
	for _, id := range []string{
		"system/perception/identity",
		"system/perception/output_format",
		"system/perception/verb_categories",
		"perception_understanding",
	} {
		atom, ok := corpus.Get(id)
		if !ok {
			t.Fatalf("perception atom %q is missing from the embedded corpus", id)
		}
		parts = append(parts, strings.TrimSpace(atom.Content))
	}
	shard.ensureTransducer().SetPromptAssembler(perceptionAtomAssembler{prompt: strings.Join(parts, "\n\n")})
}
