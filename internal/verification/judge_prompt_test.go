package verification

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/prompt"
	"codenerd/internal/system"
)

// newJudgeCompiler is the JIT compiler a booted session hands the verifier: a
// real kernel and the embedded atom corpus the binary ships.
func newJudgeCompiler(t *testing.T) *prompt.JITPromptCompiler {
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
	return compiler
}

// The judge's system prompt is compiled from its atoms, per rubric, and is a
// judge's prompt: its own contract, no Piggyback envelope (its reply is one
// bare JSON object), and no worker's intent framing. Measured 2026-09-23: 7
// atoms, ~1.5k tokens (identity, the constitution, the judge's contract);
// without the /judge intent the compile also took eight intent/*/core worker
// atoms and the refactoring discipline, ~8k tokens.
func TestJudgeSystemPrompt_IsCompiledFromTheJudgesAtoms(t *testing.T) {
	v := NewTaskVerifier(nil, nil, newJudgeCompiler(t), config.DefaultJITConfig())
	for rubric, contract := range map[string]string{
		"/implementation": "You are a strict code quality verifier.",
		"/review":         "You are verifying a CODE REVIEW task.",
	} {
		t.Run(rubric, func(t *testing.T) {
			got, err := v.judgeSystemPrompt(context.Background(), rubric)
			if err != nil {
				t.Fatalf("judgeSystemPrompt(%s): %v", rubric, err)
			}
			if !strings.Contains(got, contract) {
				t.Errorf("the %s judge's prompt lacks its contract %q", rubric, contract)
			}
			for _, leaked := range []string{
				"OUTPUT PROTOCOL: PIGGYBACK ENVELOPE", // protocol/piggyback/envelope
				"THOUGHT-FIRST ORDERING",              // protocol/piggyback/thought_first
				"REASONING TRACE REQUIREMENTS",        // protocol/reasoning/requirements
			} {
				if strings.Contains(got, leaked) {
					t.Errorf("the %s judge's prompt carries %q, a contract its JSON-only reply cannot keep", rubric, leaked)
				}
			}
			if est := prompt.EstimateTokens(got); est > 4000 {
				t.Errorf("the %s judge's prompt is %d tokens; a judge compile is ~1.5k -- worker atoms leaked in", rubric, est)
			}
		})
	}
}

// Without a compiler the judge cannot be asked: the verdict fails closed.
func TestJudgeSystemPrompt_NoCompilerIsUnavailable(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, config.DefaultJITConfig())
	if _, err := v.judgeSystemPrompt(context.Background(), "/implementation"); !errors.Is(err, ErrVerificationUnavailable) {
		t.Errorf("err = %v, want ErrVerificationUnavailable", err)
	}
}
