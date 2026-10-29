package perception

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// atomPromptAssembler serves a classification prompt built from the shipped
// perception atoms, for tests whose stub clients answer without reading it.
// The production compile through the JIT is pinned in
// perception_jit_contract_test.go; package perception cannot import
// internal/system (it imports this package) for the kernel adapter that
// compile needs.
type atomPromptAssembler struct{ prompt string }

func (a atomPromptAssembler) AssembleSystemPrompt(context.Context, string, string) (string, error) {
	return a.prompt, nil
}

func (a atomPromptAssembler) JITReady() bool { return true }

var (
	perceptionAtomPromptOnce sync.Once
	perceptionAtomPrompt     string
	perceptionAtomPromptErr  error
)

// perceptionAtomsPrompt joins the mandatory perception atoms' content in
// their compile order.
func perceptionAtomsPrompt(t testing.TB) string {
	t.Helper()
	perceptionAtomPromptOnce.Do(func() {
		corpus, err := prompt.LoadEmbeddedCorpus()
		if err != nil {
			perceptionAtomPromptErr = err
			return
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
				perceptionAtomPromptErr = fmt.Errorf("perception atom %q is missing from the embedded corpus", id)
				return
			}
			parts = append(parts, strings.TrimSpace(atom.Content))
		}
		perceptionAtomPrompt = strings.Join(parts, "\n\n")
	})
	if perceptionAtomPromptErr != nil {
		t.Fatalf("perception atoms prompt: %v", perceptionAtomPromptErr)
	}
	return perceptionAtomPrompt
}

// newCompiledUnderstandingTransducer is NewUnderstandingTransducer with the
// perception atoms' prompt wired, the state a booted session reaches.
func newCompiledUnderstandingTransducer(t testing.TB, client LLMClient) Transducer {
	t.Helper()
	tr := NewUnderstandingTransducer(client)
	tr.SetPromptAssembler(atomPromptAssembler{prompt: perceptionAtomsPrompt(t)})
	return tr
}

// atomRoleCompiler stands in for prompt.RoleCompiler in package perception's
// tests: it serves the content of the role's own mandatory atoms from the
// shipped corpus. The real JIT compile of the learning critic is pinned in
// learning_critic_prompt_test.go (package perception_test).
type atomRoleCompiler struct{ t testing.TB }

func (c atomRoleCompiler) CompileRolePrompt(_ context.Context, req types.RolePromptRequest) (string, error) {
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		return "", err
	}
	var parts []string
	for _, atom := range corpus.All() {
		if !atom.IsMandatory {
			continue
		}
		for _, shard := range atom.ShardTypes {
			if strings.TrimPrefix(shard, "/") == req.Role {
				parts = append(parts, strings.TrimSpace(atom.Content))
			}
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("%w: no atoms declare role %q", types.ErrRolePromptUnavailable, req.Role)
	}
	return strings.Join(parts, "\n\n"), nil
}
