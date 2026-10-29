package chat

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// TestE2E_RoutingPerformanceContract enforces the lane budgets for the codeNERD architecture.
// - Spreading Activation < 50ms
// - Transduction < 800ms
// - Total Turnaround to Stream Start < 1500ms
func TestE2E_RoutingPerformanceContract(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance contract test in short mode")
	}

	workspace := SetupLiveWorkspace(t)
	kernel, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatalf("Failed to create real kernel: %v", err)
	}

	// 1. Measure Spreading Activation Budget (< 50ms)
	startActivation := time.Now()
	results, err := kernel.Query("context_atom(?X)")
	if err != nil {
		t.Fatalf("Query context_atom failed: %v", err)
	}
	activationDuration := time.Since(startActivation)
	t.Logf("Spreading Activation took: %v (derived %d context atoms)", activationDuration, len(results))
	if activationDuration > 50*time.Millisecond {
		t.Errorf("PERFORMANCE CONTRACT VIOLATION: Spreading Activation took %v (Budget: < 50ms)", activationDuration)
	}

	// 2. Measure Transduction Budget (< 800ms)
	mockClient := NewMockLLMClient()
	// The classification envelope perception reads today. The retired
	// control_packet shape this fixture carried was never a classification: it
	// only appeared to work because an answer with no routing fields decoded as
	// a default /explain.
	mockClient.SetDefaultResponse(`{
		"understanding": {
			"primary_intent": "explain",
			"semantic_type": "definition",
			"action_type": "explain",
			"domain": "architecture",
			"scope": {"level": "codebase", "target": "architecture"},
			"confidence": 0.95,
			"signals": {"is_question": true}
		},
		"surface_response": "I will explain the code structure."
	}`)

	// Perception's classification prompt is compiled from its atoms; a booted
	// session wires the JIT assembler the same way (factory.go).
	compiler, promptKernel := newChatTestCompiler(t)
	assembler, err := articulation.NewPromptAssemblerWithJIT(promptKernel, compiler)
	if err != nil {
		t.Fatalf("NewPromptAssemblerWithJIT: %v", err)
	}
	tr := perception.NewUnderstandingTransducer(mockClient)
	tr.SetPromptAssembler(articulation.NewPromptAssemblerAdapter(assembler))

	startTransduction := time.Now()
	ctx := context.Background()
	intent, err := tr.ParseIntent(ctx, "Explain the architecture of the project")
	if err != nil {
		t.Fatalf("Transduction parsing intent failed: %v", err)
	}
	transductionDuration := time.Since(startTransduction)
	t.Logf("Transduction took: %v (verb parsed: %s)", transductionDuration, intent.Verb)
	if transductionDuration > 800*time.Millisecond {
		t.Errorf("PERFORMANCE CONTRACT VIOLATION: Transduction took %v (Budget: < 800ms)", transductionDuration)
	}

	// 3. Measure Total Turnaround to Stream Start Budget (< 1500ms)
	m := NewTestModel(WithSize(100, 50))
	m.kernel = kernel
	m.workspace = workspace
	m.client = mockClient
	m.transducer = tr
	m.virtualStore = core.NewVirtualStore(nil)

	startTurnaround := time.Now()
	m.textarea.SetValue("Explain the architecture")
	submitted, cmd := m.handleSubmit()
	m = submitted.(Model)

	if cmd == nil {
		t.Fatal("handleSubmit returned nil cmd")
	}

	// Execute processInput and wait for the resulting message using a custom collector for tea.BatchMsg
	ch := make(chan tea.Msg, 1)
	go func() {
		msg := cmd()
		if msg == nil {
			ch <- nil
			return
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, subCmd := range batch {
				if subCmd == nil {
					continue
				}
				subMsg := subCmd()
				if subMsg != nil {
					switch subMsg.(type) {
					case spinner.TickMsg:
						continue
					default:
						ch <- subMsg
						return
					}
				}
			}
		} else {
			ch <- msg
		}
	}()

	var msg tea.Msg
	select {
	case msg = <-ch:
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("PERFORMANCE CONTRACT VIOLATION: Total turnaround took longer than 1500ms")
	}

	turnaroundDuration := time.Since(startTurnaround)
	t.Logf("Total turnaround to stream start took: %v", turnaroundDuration)

	if turnaroundDuration > 1500*time.Millisecond {
		t.Errorf("PERFORMANCE CONTRACT VIOLATION: Total turnaround took %v (Budget: < 1500ms)", turnaroundDuration)
	}

	// Verify we got streamStartMsg
	if _, ok := msg.(streamStartMsg); !ok {
		t.Errorf("Expected streamStartMsg, got %T: %+v", msg, msg)
	}
}
