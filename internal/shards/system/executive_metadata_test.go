package system

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

type executiveMetadataModel struct {
	replies []string
	calls   int
}

func (m *executiveMetadataModel) Complete(context.Context, string) (string, error) {
	return "", fmt.Errorf("unexpected completion surface")
}
func (m *executiveMetadataModel) CompleteWithSystem(context.Context, string, string) (string, error) {
	i := m.calls
	m.calls++
	if i >= len(m.replies) {
		return "", fmt.Errorf("unexpected retry")
	}
	return m.replies[i], nil
}
func (*executiveMetadataModel) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, fmt.Errorf("unexpected tool call")
}
func (*executiveMetadataModel) CompleteWithStreaming(context.Context, string, string, bool) (<-chan string, <-chan types.StreamResult) {
	panic("unexpected streaming")
}

func TestExecutiveAutopoiesisPreservesProposalMetadata(t *testing.T) {
	const rule = "executive_metadata_fixture(/candidate)."
	for _, tc := range []struct {
		name, confidence, wrapper string
		want                      float64
		applied                   bool
		retry                     bool
	}{
		{name: "low", confidence: "0.1", want: 0.1},
		{name: "zero", confidence: "0"},
		{name: "missing"},
		{name: "malformed", confidence: "not-a-score"},
		{name: "out_of_range", confidence: "1.5"},
		{name: "non_finite", confidence: "+Inf"},
		{name: "nan", confidence: "NaN"},
		{name: "negative", confidence: "-0.5"},
		{name: "trailing_garbage", confidence: "0.9 extra"},
		{name: "duplicate", confidence: "0.1\nCONFIDENCE: 0.99"},
		{name: "json_envelope", confidence: "0.2", wrapper: "json", want: 0.2},
		{name: "high", confidence: "0.95", want: 0.95, applied: true},
		{name: "retry_metadata", confidence: "0.15", want: 0.15, retry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			kernel.SetSchemas(kernel.GetSchemas() + "\nDecl executive_metadata_fixture(Value) bound [/name].\n")
			compiler, err := prompt.NewJITPromptCompiler()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = compiler.Close() })
			assembler, err := articulation.NewPromptAssemblerWithJIT(kernel, compiler)
			if err != nil {
				t.Fatal(err)
			}
			shard := NewExecutivePolicyShard()
			shard.SetParentKernel(kernel)
			shard.SetPromptAssembler(assembler)
			response := "RULE: " + rule + "\n"
			if tc.confidence != "" {
				response += "CONFIDENCE: " + tc.confidence + "\n"
			}
			response += "RATIONALE: retain this exact reason"
			if tc.wrapper == "json" {
				b, err := json.Marshal(map[string]any{"surface_response": response, "control_packet": map[string]any{}})
				if err != nil {
					t.Fatal(err)
				}
				response = string(b)
			}
			model := &executiveMetadataModel{replies: []string{response}}
			if tc.retry {
				model.replies = append([]string{"not a rule\nCONFIDENCE: 0.99\nRATIONALE: rejected attempt"}, model.replies...)
			}
			shard.SetLLMClient(model)
			shard.Autopoiesis.RecordUnhandled("metadata fixture", map[string]string{"task": "synthetic"}, nil)
			shard.handleAutopoiesis(t.Context())
			if model.calls != len(model.replies) {
				t.Fatalf("handler did not reach expected model attempts: %d want %d", model.calls, len(model.replies))
			}
			if len(shard.Autopoiesis.ProposedRules) != 1 {
				t.Fatalf("proposals=%+v", shard.Autopoiesis.ProposedRules)
			}
			got := shard.Autopoiesis.ProposedRules[0]
			if got.Confidence != tc.want || got.Rationale != "retain this exact reason" || strings.TrimSpace(got.MangleCode) != rule {
				t.Errorf("proposal=%+v want confidence=%g, original reason and validated rule", got, tc.want)
			}
			rows, err := kernel.Query("executive_metadata_fixture")
			if err != nil {
				t.Fatal(err)
			}
			pending, err := kernel.Query("rule_proposal_pending")
			if err != nil {
				t.Fatal(err)
			}
			if tc.applied {
				if len(rows) != 1 || len(shard.Autopoiesis.AppliedRules) != 1 || !strings.Contains(kernel.GetLearned(), rule) || len(pending) != 0 {
					t.Fatalf("high-confidence adoption missing: derived=%v applied=%v pending=%v", rows, shard.Autopoiesis.AppliedRules, pending)
				}
			} else if len(rows) != 0 || len(shard.Autopoiesis.AppliedRules) != 0 || strings.Contains(kernel.GetLearned(), rule) || len(pending) != 1 {
				t.Fatalf("low/unknown proposal auto-applied: derived=%v applied=%v pending=%v", rows, shard.Autopoiesis.AppliedRules, pending)
			} else if len(pending[0].Args) != 5 || pending[0].Args[2] != "retain this exact reason" || pending[0].Args[3] != types.PercentFromRatio(tc.want) {
				t.Fatalf("pending metadata lost or wrong scale: %+v", pending)
			}
		})
	}
}
