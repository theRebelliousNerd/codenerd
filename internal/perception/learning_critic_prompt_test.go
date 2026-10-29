package perception_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/system"
	"codenerd/internal/types"
)

// learningCriticCapture records the critic call and answers with one fact.
type learningCriticCapture struct {
	perceptionRequestCapture
	critic string
}

func (c *learningCriticCapture) CompleteWithSystem(_ context.Context, systemPrompt, _ string) (string, error) {
	c.system = systemPrompt
	c.calls++
	return c.critic, nil
}

// newRolePrompts is the role compiler a booted session wires: a real kernel
// and the embedded atom corpus the binary ships.
func newRolePrompts(test *testing.T) *prompt.RoleCompiler {
	test.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		test.Fatalf("NewRealKernel: %v", err)
	}
	corpus := loadPerceptionJITCorpus(test)
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithKernel(system.NewKernelAdapter(kernel)),
		prompt.WithEmbeddedCorpus(corpus),
	)
	if err != nil {
		test.Fatalf("NewJITPromptCompiler: %v", err)
	}
	test.Cleanup(func() { _ = compiler.Close() })
	roles, err := prompt.NewRoleCompiler(compiler, config.DefaultJITConfig())
	if err != nil {
		test.Fatalf("NewRoleCompiler: %v", err)
	}
	return roles
}

func newLearningEngine(test *testing.T, client perception.LLMClient, roles types.RolePromptCompiler) *perception.TaxonomyEngine {
	test.Helper()
	engine, err := perception.NewTaxonomyEngine()
	if err != nil {
		test.Fatalf("NewTaxonomyEngine: %v", err)
	}
	test.Cleanup(engine.StopWorker)
	engine.SetClient(client)
	if roles != nil {
		engine.SetRolePrompts(roles)
	}
	return engine
}

var learningCorrection = []perception.ReasoningTrace{
	{UserPrompt: "Nuke it.", Response: "???", Success: false},
	{UserPrompt: "Nuke it means delete the database.", Response: "ok", Success: true},
}

// The critic's system prompt is compiled from its role atoms
// (atoms/perception/learning_critic.yaml) and is a critic's prompt: its own
// one-fact output contract, no Piggyback envelope, no worker intent atoms.
func TestLearningCritic_SystemPromptIsCompiledFromItsAtoms(test *testing.T) {
	corpus := loadPerceptionJITCorpus(test)
	var atoms []*prompt.PromptAtom
	for _, id := range []string{"perception/learning_critic/mission", "perception/learning_critic/contract"} {
		atom, ok := corpus.Get(id)
		if !ok {
			test.Fatalf("the learning critic's atom %q is missing from the embedded corpus", id)
		}
		atoms = append(atoms, atom)
	}
	client := &learningCriticCapture{critic: `learned_exemplar("Nuke it", /delete, "database", "", 0.95).`}
	engine := newLearningEngine(test, client, newRolePrompts(test))

	fact, err := engine.LearnFromInteraction(context.Background(), learningCorrection)
	if err != nil {
		test.Fatalf("LearnFromInteraction: %v", err)
	}
	if !strings.HasPrefix(fact, `learned_exemplar("Nuke it"`) {
		test.Fatalf("fact = %q, want the critic's learned_exemplar", fact)
	}
	if client.calls != 1 {
		test.Fatalf("critic calls = %d, want 1", client.calls)
	}
	for _, atom := range atoms {
		if !strings.Contains(client.system, strings.TrimSpace(atom.Content)) {
			test.Errorf("the compiled critic prompt does not carry %q whole", atom.ID)
		}
	}
	for _, leaked := range []string{
		"OUTPUT PROTOCOL: PIGGYBACK ENVELOPE", // protocol/piggyback/envelope
		"CONTROL-PACKET FIELD ORDER",          // protocol/piggyback/control_first
		"DECISION SUMMARY",                    // protocol/decision_summary/requirements
	} {
		if strings.Contains(client.system, leaked) {
			test.Errorf("the critic's prompt carries %q, a contract its one-fact reply cannot keep", leaked)
		}
	}
	if est := prompt.EstimateTokens(client.system); est > 4000 {
		test.Errorf("the critic's prompt is %d tokens; a role compile is ~1.5k-3k -- worker atoms leaked in", est)
	}
}

// Without a role compiler the critic cannot be asked: the error wraps
// ErrRolePromptUnavailable and the model is never called.
func TestLearningCritic_NoCompilerIsUnavailable(test *testing.T) {
	client := &learningCriticCapture{}
	engine := newLearningEngine(test, client, nil)
	_, err := engine.LearnFromInteraction(context.Background(), learningCorrection)
	if !errors.Is(err, types.ErrRolePromptUnavailable) {
		test.Fatalf("err = %v, want ErrRolePromptUnavailable", err)
	}
	if client.calls != 0 {
		test.Fatalf("the critic was asked %d times without its prompt", client.calls)
	}
}

// failingRoles is a compiler whose compile fails.
type failingRoles struct{}

func (failingRoles) CompileRolePrompt(context.Context, types.RolePromptRequest) (string, error) {
	return "", errors.New("compile failed")
}

// A failed compile is ErrRolePromptUnavailable too, never a different prompt.
func TestLearningCritic_FailedCompileIsUnavailable(test *testing.T) {
	client := &learningCriticCapture{}
	engine := newLearningEngine(test, client, failingRoles{})
	_, err := engine.LearnFromInteraction(context.Background(), learningCorrection)
	if !errors.Is(err, types.ErrRolePromptUnavailable) {
		test.Fatalf("err = %v, want ErrRolePromptUnavailable", err)
	}
	if client.calls != 0 {
		test.Fatalf("the critic was asked %d times under a failed compile", client.calls)
	}
}
