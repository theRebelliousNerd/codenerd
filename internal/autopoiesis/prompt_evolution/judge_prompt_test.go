package prompt_evolution_test

import (
	"context"
	"strings"
	"testing"

	pe "codenerd/internal/autopoiesis/prompt_evolution"
	nerdconfig "codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/prompt"
	"codenerd/internal/system"
)

// recordingJudgeClient keeps the system prompt the judge sends and answers
// with a valid verdict.
type recordingJudgeClient struct{ system string }

func (c *recordingJudgeClient) Complete(context.Context, string) (string, error) {
	return "", nil
}

func (c *recordingJudgeClient) CompleteWithSystem(_ context.Context, system, _ string) (string, error) {
	c.system = system
	return `{"verdict": "PASS", "explanation": "done", "category": "CORRECT"}`, nil
}

// The judge's system prompt is its atom (eval/judge/task_evaluator), compiled
// as a booted session compiles it (a kernel, the embedded corpus): the
// evaluator's contract, and no Piggyback envelope or thought-first ordering --
// its reply is one JSON verdict. Until 2026-09-23 the judge sent a Go copy of
// the atom that had drifted from it.
func TestTaskJudge_SendsTheCompiledEvaluatorAtom(t *testing.T) {
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

	client := &recordingJudgeClient{}
	judge := pe.NewTaskJudge(client, "test-model", compiler, nerdconfig.DefaultJITConfig())
	if _, err := judge.Evaluate(context.Background(), &pe.ExecutionRecord{TaskID: "t1", ShardType: "/coder"}); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !strings.Contains(client.system, "You are an expert evaluator for an AI coding agent.") {
		t.Error("the judge's system prompt lacks the evaluator's contract")
	}
	for _, leaked := range []string{"OUTPUT PROTOCOL: PIGGYBACK ENVELOPE", "THOUGHT-FIRST ORDERING", "REASONING TRACE REQUIREMENTS"} {
		if strings.Contains(client.system, leaked) {
			t.Errorf("the judge's system prompt carries %q, a contract its JSON verdict cannot keep", leaked)
		}
	}
	if est := prompt.EstimateTokens(client.system); est > 4000 {
		t.Errorf("the judge's system prompt is %d tokens; worker atoms leaked in", est)
	}
}

// Without a compiler the judge has no prompt, and says so.
func TestTaskJudge_NoCompilerIsAnError(t *testing.T) {
	judge := pe.NewTaskJudge(&recordingJudgeClient{}, "test-model", nil, nerdconfig.DefaultJITConfig())
	if _, err := judge.Evaluate(context.Background(), &pe.ExecutionRecord{TaskID: "t1"}); err == nil {
		t.Error("a judge with no prompt compiler evaluated")
	}
}
