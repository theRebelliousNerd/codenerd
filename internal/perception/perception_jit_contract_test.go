package perception_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/system"
	"codenerd/internal/types"
)

type perceptionRequestCapture struct {
	system string
	calls  int
}

var _ perception.LLMClient = (*perceptionRequestCapture)(nil)

func (capture *perceptionRequestCapture) Complete(context.Context, string) (string, error) {
	return "", fmt.Errorf("perception must use its system-prompt channel")
}

func (capture *perceptionRequestCapture) CompleteWithSystem(_ context.Context, systemPrompt, _ string) (string, error) {
	capture.system = systemPrompt
	capture.calls++
	response, err := json.Marshal(perception.UnderstandingEnvelope{
		Understanding: perception.Understanding{
			PrimaryIntent: "explain",
			SemanticType:  "mechanism",
			ActionType:    "explain",
			Domain:        "testing",
			Scope: perception.Scope{
				Level: "function", Target: "calculator.Sum", File: "calculator.go", Symbol: "Sum",
			},
			Confidence: 0.9,
			Signals:    perception.Signals{IsQuestion: true, Urgency: "normal"},
			SuggestedApproach: perception.SuggestedApproach{
				Mode: "normal", PrimaryShard: "coder",
			},
		},
		SurfaceResponse: "fixture response",
	})
	return string(response), err
}

func (capture *perceptionRequestCapture) CompleteWithStreaming(context.Context, string, string, bool) (<-chan string, <-chan error) {
	responses := make(chan string)
	failures := make(chan error, 1)
	failures <- fmt.Errorf("perception must use its system-prompt channel")
	close(responses)
	close(failures)
	return responses, failures
}

func (capture *perceptionRequestCapture) CompleteWithTools(context.Context, string, string, []perception.ToolDefinition) (*perception.LLMToolResponse, error) {
	return nil, fmt.Errorf("perception classification must not request tool execution")
}

func TestPerceptionJIT_RequestCaptureRefusesOtherChannels(test *testing.T) {
	capture := &perceptionRequestCapture{}
	if _, err := capture.CompleteWithSystem(context.Background(), "classification witness", "user input"); err != nil {
		test.Fatalf("CompleteWithSystem: %v", err)
	}
	if _, err := capture.Complete(context.Background(), "other request"); err == nil {
		test.Fatal("unstructured completion was accepted")
	}
	if _, err := capture.CompleteWithTools(context.Background(), "other prompt", "user input", nil); err == nil {
		test.Fatal("tool execution was accepted")
	}
	responses, failures := capture.CompleteWithStreaming(context.Background(), "other prompt", "user input", true)
	select {
	case _, open := <-responses:
		if open {
			test.Fatal("streaming returned model content")
		}
	default:
		test.Fatal("streaming response channel was not closed")
	}
	select {
	case refusal, open := <-failures:
		if !open || refusal == nil {
			test.Fatal("streaming did not report its refusal")
		}
	default:
		test.Fatal("streaming refusal was not immediately available")
	}
	select {
	case _, open := <-failures:
		if open {
			test.Fatal("streaming failure channel was not closed after its refusal")
		}
	default:
		test.Fatal("streaming failure channel was left open")
	}
	if capture.system != "classification witness" || capture.calls != 1 {
		test.Fatal("refused channels changed the captured classification request")
	}
}

func newPerceptionJITBridge(test *testing.T, corpus *prompt.EmbeddedCorpus) (*prompt.JITPromptCompiler, *articulation.PromptAssembler, *core.RealKernel) {
	test.Helper()
	kernel, err := core.NewRealKernelWithWorkspace(test.TempDir())
	if err != nil {
		test.Fatalf("NewRealKernelWithWorkspace: %v", err)
	}
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithEmbeddedCorpus(corpus),
		prompt.WithKernel(system.NewKernelAdapter(kernel)),
	)
	if err != nil {
		test.Fatalf("NewJITPromptCompiler: %v", err)
	}
	test.Cleanup(func() {
		if err := compiler.Close(); err != nil {
			test.Errorf("compiler.Close: %v", err)
		}
	})
	assembler, err := articulation.NewPromptAssemblerWithJIT(kernel, compiler)
	if err != nil {
		test.Fatalf("NewPromptAssemblerWithJIT: %v", err)
	}
	assembler.SetJITBudgets(200000, 4000, 10, 0)
	return compiler, assembler, kernel
}

func loadPerceptionJITCorpus(test *testing.T) *prompt.EmbeddedCorpus {
	test.Helper()
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		test.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	return corpus
}

func capturePerceptionRequest(test *testing.T, assembler *articulation.PromptAssembler, kernel *core.RealKernel) *perceptionRequestCapture {
	test.Helper()
	client := &perceptionRequestCapture{}
	transducer := perception.NewUnderstandingTransducer(client)
	transducer.SetPromptAssembler(articulation.NewPromptAssemblerAdapter(assembler))
	transducer.(perception.TransducerWithKernel).SetKernel(kernel)
	intent, err := transducer.ParseIntentWithContext(context.Background(), "Explain calculator.Sum and identify its existing tests.", nil)
	if err != nil {
		test.Fatalf("ParseIntentWithContext: %v", err)
	}
	if client.calls != 1 {
		test.Fatalf("captured %d classification calls, want 1", client.calls)
	}
	if intent.Target != "calculator.Sum" || intent.Response != "fixture response" {
		test.Fatalf("canonical response did not reach the intent consumer: target=%q response=%q", intent.Target, intent.Response)
	}
	return client
}

func assertPerceptionSelection(test *testing.T, result *prompt.CompilationResult, corpus *prompt.EmbeddedCorpus) {
	test.Helper()
	if result == nil || len(result.IncludedAtoms) == 0 {
		test.Fatal("no actual compiled selection; assembler fallback cannot prove this contract")
	}
	ids := make(map[string]bool, len(result.IncludedAtoms))
	for _, atom := range result.IncludedAtoms {
		ids[atom.ID] = true
		if strings.HasPrefix(atom.ID, "protocol/piggyback/") || strings.HasPrefix(atom.ID, "protocol/reasoning/") || strings.HasPrefix(atom.ID, "perception/transducer/") {
			test.Errorf("incompatible instruction %q reached perception", atom.ID)
		}
		if atom.Category == prompt.CategoryProtocol && atom.ID != "system/perception/output_format" {
			test.Errorf("competing protocol %q reached perception", atom.ID)
		}
	}
	for _, id := range []string{"system/perception/identity", "system/perception/output_format", "system/perception/verb_categories", "system/perception/precision"} {
		if !ids[id] {
			test.Errorf("canonical owner/support %q was not selected", id)
		}
		atom, exists := corpus.Get(id)
		if !exists || !strings.Contains(result.Prompt, strings.TrimSpace(atom.Content)) {
			test.Errorf("canonical atom %q was not rendered whole", id)
		}
	}
	for _, atom := range result.IncludedAtoms {
		for _, dependency := range atom.DependsOn {
			if !ids[dependency] {
				test.Errorf("selected atom %q has a missing dependency %q", atom.ID, dependency)
			}
		}
	}
	if strings.Contains(result.Prompt, `"control_packet":`) || strings.Contains(result.Prompt, "OUTPUT PROTOCOL: PIGGYBACK") {
		test.Error("compiled perception prompt contains a competing envelope")
	}
}

func TestPerceptionJIT_ProductionBridgeUsesCanonicalContract(test *testing.T) {
	corpus := loadPerceptionJITCorpus(test)
	compiler, assembler, kernel := newPerceptionJITBridge(test, corpus)
	client := capturePerceptionRequest(test, assembler, kernel)
	result := compiler.GetLastResult()
	assertPerceptionSelection(test, result, corpus)
	if client.system != result.Prompt {
		test.Fatal("actual classification request differs from the JIT result: adapter fallback or assembler suffix was used")
	}
	selectedUnderstanding := false
	for _, atom := range result.IncludedAtoms {
		selectedUnderstanding = selectedUnderstanding || atom.ID == "perception_understanding"
	}
	if !selectedUnderstanding {
		test.Fatal("the shipped mandatory understanding atom was omitted; this would hide a conflicting schema")
	}
	if !strings.Contains(client.system, `"target": "<specific target>"`) {
		test.Fatal("canonical nested scope.target did not reach the model")
	}
}

func TestPerceptionJIT_FirewallBridgeKeepsCanonicalClosure(test *testing.T) {
	corpus := loadPerceptionJITCorpus(test)
	compiler, assembler, _ := newPerceptionJITBridge(test, corpus)
	assembled, err := articulation.NewPromptAssemblerAdapter(assembler).AssembleSystemPrompt(context.Background(), "perception-firewall", "perception_firewall")
	if err != nil {
		test.Fatalf("AssembleSystemPrompt: %v", err)
	}
	result := compiler.GetLastResult()
	assertPerceptionSelection(test, result, corpus)
	client := &perceptionRequestCapture{}
	transducer := perception.NewLLMTransducer(client, nil, assembled)
	understanding, err := transducer.Understand(context.Background(), "Explain calculator.Sum.", nil, nil, nil, "")
	if err != nil {
		test.Fatalf("Understand: %v", err)
	}
	if client.calls != 1 || client.system != result.Prompt || understanding.Scope.Target != "calculator.Sum" {
		test.Fatal("firewall's compiled contract did not reach the model and typed consumer")
	}
}

func TestPerceptionJIT_ProductionBridgeFallsBackForInvalidContracts(test *testing.T) {
	base := loadPerceptionJITCorpus(test)
	piggyback, exists := base.Get("protocol/piggyback/envelope")
	if !exists {
		test.Fatal("missing Piggyback negative control")
	}
	for _, mode := range []string{"missing", "malformed", "conflicting"} {
		test.Run(mode, func(test *testing.T) {
			var atoms []*prompt.PromptAtom
			for _, atom := range base.All() {
				if atom.ID == "perception_understanding" {
					continue
				}
				if atom.ID == "system/perception/output_format" {
					if mode == "missing" {
						continue
					}
					copy := *atom
					if mode == "malformed" {
						copy.Content = strings.Replace(copy.Content, `"domain":`, `domain:`, 1)
					} else {
						copy.Content += "\n" + piggyback.Content
					}
					copy.ContentHash = prompt.HashContent(copy.Content)
					copy.TokenCount = prompt.EstimateTokens(copy.Content)
					atom = &copy
				}
				atoms = append(atoms, atom)
			}
			compiler, assembler, kernel := newPerceptionJITBridge(test, prompt.NewEmbeddedCorpus(atoms))
			client := capturePerceptionRequest(test, assembler, kernel)
			result := compiler.GetLastResult()
			if result == nil {
				test.Fatal("no JIT result; this control must exercise adapter rejection, not a compiler error")
			}
			if client.system == result.Prompt || !strings.HasPrefix(client.system, "## Your Role: Perception Layer") {
				test.Fatal("invalid compiled contract reached the model instead of the intentional embedded fallback")
			}
		})
	}
}

func TestPerceptionJIT_ProductionAssemblerPreservesCampaignAndConversation(test *testing.T) {
	corpus := loadPerceptionJITCorpus(test)
	compiler, assembler, _ := newPerceptionJITBridge(test, corpus)
	for _, shard := range []string{"planner", "coder"} {
		test.Run(shard, func(test *testing.T) {
			assembled, err := assembler.AssembleSystemPrompt(context.Background(), &articulation.PromptContext{
				ShardID: shard, ShardType: shard,
				CampaignID: "perception-contract-campaign",
				SessionCtx: &types.SessionContext{CampaignActive: true, CampaignPhase: "/planning"},
			})
			if err != nil {
				test.Fatalf("AssembleSystemPrompt: %v", err)
			}
			result := compiler.GetLastResult()
			if result == nil || assembled != result.Prompt {
				test.Fatal("assembler did not consume the actual JIT result unchanged")
			}
			if shard == "planner" {
				var atomIDs []string
				for _, atom := range result.IncludedAtoms {
					if len(atomIDs) == 24 {
						break
					}
					atomIDs = append(atomIDs, atom.ID)
				}
				if !strings.Contains(assembled, `"phases":`) {
					test.Errorf("planner lacks its phase schema; selected_atoms=%q selected_count=%d", atomIDs, len(result.IncludedAtoms))
				}
				if strings.Contains(assembled, `"control_packet":`) {
					test.Errorf("planner contains a competing Piggyback envelope; selected_atoms=%q selected_count=%d", atomIDs, len(result.IncludedAtoms))
				}
			} else if !strings.Contains(assembled, `"control_packet":`) || !strings.Contains(assembled, `"tool_requests":`) {
				test.Fatal("conversational coder lost its Piggyback tool-request contract")
			}
		})
	}
}
