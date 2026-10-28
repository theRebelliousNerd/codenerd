package system_test

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/mangle"
	"codenerd/internal/prompt"
	shardsystem "codenerd/internal/shards/system"
	"codenerd/internal/system"
	"codenerd/internal/types"
)

// newRepairAssembler is the JIT-ready prompt assembler the shard registrar
// hands the repair shard: a real kernel and the embedded atom corpus.
func newRepairAssembler(t *testing.T) *articulation.PromptAssembler {
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
	pa, err := articulation.NewPromptAssemblerWithJIT(kernel, compiler)
	if err != nil {
		t.Fatalf("NewPromptAssemblerWithJIT: %v", err)
	}
	jit := config.DefaultJITConfig()
	pa.SetJITBudgets(jit.TokenBudget, jit.ReservedTokens, jit.SemanticTopK, jit.ReservedTokensFallbackRatio)
	return pa
}

// promptCapture is a schema-capable model that records the system prompt it
// was asked under.
type promptCapture struct {
	schemaCalls, systemCalls int
	systemPrompt             string
	response                 string
}

func (m *promptCapture) SchemaCapable() bool { return true }
func (m *promptCapture) Complete(context.Context, string) (string, error) {
	return m.response, nil
}
func (m *promptCapture) CompleteWithSystem(_ context.Context, systemPrompt, _ string) (string, error) {
	m.systemCalls++
	m.systemPrompt = systemPrompt
	return m.response, nil
}
func (m *promptCapture) CompleteWithSchema(_ context.Context, systemPrompt, _, _ string) (string, error) {
	m.schemaCalls++
	m.systemPrompt = systemPrompt
	return m.response, nil
}
func (m *promptCapture) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{}, nil
}
func (m *promptCapture) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, _ bool) (<-chan string, <-chan types.StreamResult) {
	return types.StreamWhole(ctx, func(ctx context.Context) (string, error) {
		return m.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	})
}

// A repair asks the model under the prompt compiled from the /mangle_repair
// atoms, through the schema-enforced call when the client offers one. The
// compiled prompt carries the repair syntax atom (system/mangle_repair/syntax,
// once the Go fallback prompt), and its negation example parses with the
// production parser -- the fallback wrote "not negative(X)", which Mangle
// does not parse, so a repair made under it failed on its own advice.
func TestMangleRepairPrompt_IsCompiledFromTheRepairAtoms(t *testing.T) {
	shard := shardsystem.NewMangleRepairShard()
	shard.SetPromptAssembler(newRepairAssembler(t))
	mock := &promptCapture{
		response: `{"format":"mangle_synth_v1","program":{"clauses":[{"head":{"pred":"next_action","args":[{"kind":"name","value":"/run_tests"}]}}]}}`,
	}
	shard.SetLLMClient(mock)

	result, err := shard.ValidateAndRepair(context.Background(), "next_action(/run_tests) :- not test_state(X).")
	if err != nil {
		t.Fatalf("ValidateAndRepair: %v", err)
	}
	if result.Rejected || !result.WasRepaired {
		t.Fatalf("want a repair, got %+v", result)
	}
	if mock.schemaCalls == 0 || mock.systemCalls != 0 {
		t.Fatalf("schema calls %d, plain calls %d; want the schema-enforced call only", mock.schemaCalls, mock.systemCalls)
	}

	got := mock.systemPrompt
	for _, atom := range []string{
		"You are the MangleRepair shard.",                      // system/mangle_repair/identity
		"Key Mangle syntax rules:",                             // system/mangle_repair/syntax
		"Output ONLY a MangleSynth JSON object",                // system/mangle_repair/output
		"Please provide a CORRECTED version of the rule that:", // system/mangle_repair/correction
	} {
		if !strings.Contains(got, atom) {
			t.Errorf("the repair prompt lacks %q:\n%s", atom, got)
		}
	}
	const lead = "its variables must be bound first: "
	i := strings.Index(got, lead)
	if i < 0 {
		t.Fatalf("the repair prompt shows no negation example:\n%s", got)
	}
	example := strings.TrimSpace(strings.SplitN(got[i+len(lead):], "\n", 2)[0])
	program := "Decl positive(X) bound [/string].\nDecl negative(X) bound [/string].\nDecl repaired(X) bound [/string].\n" +
		"repaired(X) :- " + example + ".\n"
	if _, err := mangle.ParseUnit(strings.NewReader(program)); err != nil {
		t.Fatalf("the repair prompt's negation example does not parse (%q): %v", example, err)
	}
}

// The legislator's compiled prompt carries its task and its reply contract:
// both were Go text in its request, and the contract atom, filed under
// protocol, was stripped from its structured-output compile.
func TestLegislatorPrompt_CarriesItsTaskAndContract(t *testing.T) {
	got, err := newRepairAssembler(t).AssembleSystemPrompt(context.Background(), &articulation.PromptContext{
		ShardID:   "legislator",
		ShardType: "legislator",
	})
	if err != nil {
		t.Fatalf("AssembleSystemPrompt: %v", err)
	}
	for _, atom := range []string{
		"Translate the constraint into a single MangleSynth JSON rule spec.", // system/legislator/task
		"Output ONLY a MangleSynth JSON object.",                             // system/legislator/output
	} {
		if !strings.Contains(got, atom) {
			t.Errorf("the legislator prompt lacks %q", atom)
		}
	}
}
