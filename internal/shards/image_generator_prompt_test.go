package shards_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/prompt"
	"codenerd/internal/shards"
	"codenerd/internal/system"
	"codenerd/internal/types"
)

// newRolePrompts is the role prompt compiler a booted session holds
// (Cortex.RolePrompts): a real kernel and the embedded atom corpus.
func newRolePrompts(t *testing.T) types.RolePromptCompiler {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	embedded, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithKernel(system.NewKernelAdapter(kernel)),
		prompt.WithEmbeddedCorpus(embedded),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	roles, err := prompt.NewRoleCompiler(compiler, config.DefaultJITConfig())
	if err != nil {
		t.Fatalf("NewRoleCompiler: %v", err)
	}
	return roles
}

// imageWitness is an image model that records the system prompt it was asked
// under.
type imageWitness struct{ system, user string }

func (c *imageWitness) Complete(ctx context.Context, p string) (string, error) {
	return c.CompleteWithSystem(ctx, "", p)
}
func (c *imageWitness) CompleteWithSystem(_ context.Context, system, user string) (string, error) {
	c.system, c.user = system, user
	return "image-ok", nil
}
func (c *imageWitness) CompleteWithStreaming(context.Context, string, string, bool) (<-chan string, <-chan types.StreamResult) {
	return types.FailedStream(errors.New("not implemented"))
}
func (c *imageWitness) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, errors.New("not implemented")
}

// The registered image generator asks the image model under the prompt the
// session's role compiler builds from identity/image_generator/role: its own
// reply contract, no Piggyback envelope, and the task as the user message.
func TestImageGenerator_SystemPromptIsCompiledFromItsAtom(t *testing.T) {
	sm := coreshards.NewShardManager()
	witness := &imageWitness{}
	sm.SetImageLLMClient(witness)
	shards.RegisterAllShardFactories(sm, shards.RegistryContext{RolePrompts: newRolePrompts(t)})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := sm.Spawn(ctx, "image_generator", "a tiny green checkmark"); err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if !strings.Contains(witness.system, "You are an image generation assistant.") {
		t.Errorf("the image prompt lacks its role atom:\n%s", witness.system)
	}
	for _, leaked := range []string{"OUTPUT PROTOCOL: PIGGYBACK ENVELOPE", "CONTROL-PACKET FIELD ORDER"} {
		if strings.Contains(witness.system, leaked) {
			t.Errorf("the image prompt carries %q", leaked)
		}
	}
	if witness.user != "a tiny green checkmark" {
		t.Errorf("user message = %q, want the task", witness.user)
	}
}

// Without a role prompt compiler the registered image generator fails the
// request rather than asking the model under another prompt.
func TestImageGenerator_NoRolePromptCompilerFailsTheRequest(t *testing.T) {
	sm := coreshards.NewShardManager()
	witness := &imageWitness{}
	sm.SetImageLLMClient(witness)
	shards.RegisterAllShardFactories(sm, shards.RegistryContext{})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := sm.Spawn(ctx, "image_generator", "draw"); !errors.Is(err, types.ErrRolePromptUnavailable) {
		t.Fatalf("Spawn err = %v, want ErrRolePromptUnavailable", err)
	}
	if witness.user != "" {
		t.Fatalf("the image model was asked without a compiled prompt")
	}
}
