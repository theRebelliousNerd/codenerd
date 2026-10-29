package context_harness

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	internalcontext "codenerd/internal/context"
	"codenerd/internal/core"
)

// stubEngine is a ContextEngine whose retrieval and breakdowns a test sets.
type stubEngine struct {
	mode       EngineMode
	retrieved  []core.Fact
	breakdowns map[string]*ActivationBreakdown
	seeded     []core.Fact
	resets     int
}

func (e *stubEngine) CompressTurn(context.Context, *Turn) ([]core.Fact, int, error) {
	return nil, 0, nil
}
func (e *stubEngine) RetrieveContext(context.Context, string, int) ([]core.Fact, error) {
	return e.retrieved, nil
}
func (e *stubEngine) GetCompressionStats() (int, int) { return 0, 0 }
func (e *stubEngine) GetActivationBreakdown(f core.Fact) *ActivationBreakdown {
	return e.breakdowns[f.String()]
}
func (e *stubEngine) SeedFacts(facts []core.Fact) error {
	e.seeded = append(e.seeded, facts...)
	return nil
}
func (e *stubEngine) SetCampaignContext(*internalcontext.CampaignActivationContext)           {}
func (e *stubEngine) SetIssueContext(*internalcontext.IssueActivationContext)                 {}
func (e *stubEngine) SetBackReferenceContext(*internalcontext.BackReferenceActivationContext) {}
func (e *stubEngine) Reset() error                                                            { e.resets++; return nil }
func (e *stubEngine) GetMode() EngineMode                                                     { return e.mode }

// Checkpoints fire after the turn whose TurnID they name, and one that never
// fires fails the scenario. They were matched against the turn's slice index,
// so in seven of the eight mock scenarios (sparse turn IDs) no checkpoint ever
// ran and the scenario was judged on averages alone.
func TestRunScenario_ShouldFireCheckpointsByTurnIDAndFailUnreachedOnes(t *testing.T) {
	scenario := &Scenario{
		ScenarioID: "sparse",
		Mode:       MockMode,
		Category:   CategoryMock,
		Turns: []Turn{
			{TurnID: 0, Speaker: "user", Message: "the build fails", Intent: "debug"},
			{TurnID: 10, Speaker: "user", Message: "what failed?", Intent: "recall"},
		},
		Checkpoints: []Checkpoint{
			{AfterTurn: 10, Query: "what failed"},
			{AfterTurn: 99, Query: "never"},
		},
	}
	sim := NewSessionSimulator(nil, SimulatorConfig{TokenBudget: 8000, CompressionEnabled: true})
	sim.SetContextEngine(&stubEngine{mode: MockMode, retrieved: []core.Fact{{Predicate: "turn_topic", Args: []any{0, "build"}}}})

	result, err := sim.RunScenario(context.Background(), scenario)
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if len(result.CheckpointResults) != 1 || result.CheckpointResults[0].Checkpoint.AfterTurn != 10 {
		t.Fatalf("checkpoint after turn 10 (the second turn) did not fire: %+v", result.CheckpointResults)
	}
	if result.Passed || !strings.Contains(strings.Join(result.FailureReasons, "\n"), "after turn 99 never ran") {
		t.Fatalf("a checkpoint the scenario never reached did not fail it: %v", result.FailureReasons)
	}
}

// A scenario's InitialFacts reach the engine before the first turn.
func TestRunScenario_ShouldSeedTheScenariosInitialFacts(t *testing.T) {
	engine := &stubEngine{mode: MockMode}
	sim := NewSessionSimulator(nil, SimulatorConfig{TokenBudget: 8000})
	sim.SetContextEngine(engine)
	scenario := &Scenario{
		ScenarioID:   "seeded",
		Mode:         MockMode,
		InitialFacts: []string{`current_campaign("auth-migration")`, `campaign_phase("auth-migration", "planning", 1)`},
		Turns:        []Turn{{TurnID: 0, Speaker: "user", Message: "hi"}},
	}
	if _, err := sim.RunScenario(context.Background(), scenario); err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if len(engine.seeded) != 2 || engine.seeded[1].Predicate != "campaign_phase" || engine.seeded[1].Args[2] != 1 {
		t.Fatalf("initial facts not seeded as declared: %+v", engine.seeded)
	}
}

// A scenario written for the real engine is refused on the mock one instead
// of being judged by validators that read breakdowns the mock never computes.
func TestRunScenario_WhenARealScenarioMeetsTheMockEngine_ShouldRefuse(t *testing.T) {
	sim := NewSessionSimulator(nil, SimulatorConfig{TokenBudget: 8000})
	sim.SetContextEngine(NewMockContextEngine(nil))
	_, err := sim.RunScenario(context.Background(), CampaignPhaseTransitionScenario())
	if !errors.Is(err, ErrRequiresRealEngine) {
		t.Fatalf("err = %v, want ErrRequiresRealEngine", err)
	}
}

// Declared validators are enforced. They were typed on the checkpoint and
// called by nothing, so every integration checkpoint passed them unread.
func TestValidateCheckpoint_ShouldEnforceDeclaredValidators(t *testing.T) {
	phase := core.Fact{Predicate: "campaign_phase", Args: []any{"auth", "implementation", 2}}
	engine := &stubEngine{
		mode:       RealMode,
		retrieved:  []core.Fact{phase},
		breakdowns: map[string]*ActivationBreakdown{phase.String(): {CampaignBoost: 5, TotalScore: 60}},
	}
	sim := NewSessionSimulator(nil, SimulatorConfig{TokenBudget: 8000})
	sim.SetContextEngine(engine)

	result := sim.validateCheckpoint(context.Background(), &Checkpoint{
		Query:              "phase",
		ValidateActivation: &ActivationValidation{FactPattern: "campaign_phase.*implementation", MinCampaignBoost: 20},
		ValidateCompression: &CompressionCheckpoint{
			ExpectTriggered: true,
		},
		ValidateFeedback: &FeedbackValidation{MinFeedbackSamples: 10, ExpectedHelpful: []string{"campaign_phase"}, MinHelpfulBoost: 5},
	})
	if result.Passed {
		t.Fatal("a checkpoint whose validators all fail passed")
	}
	for _, want := range []string{
		"CampaignBoost validation failed: expected >= 20.00, got 5.00",
		"compression trigger not observable",
		"feedback sample count not observable",
		"feedback boost of helpful campaign_phase 0.00 < 5.00",
	} {
		if !strings.Contains(result.FailureReason, want) {
			t.Errorf("failure reason lacks %q:\n%s", want, result.FailureReason)
		}
	}

	// And a breakdown that meets the floor passes that validator.
	engine.breakdowns[phase.String()].CampaignBoost = 25
	ok := sim.validateCheckpoint(context.Background(), &Checkpoint{
		Query:              "phase",
		ValidateActivation: &ActivationValidation{FactPattern: "campaign_phase.*implementation", MinCampaignBoost: 20},
	})
	if !ok.Passed {
		t.Fatalf("a met activation floor failed: %s", ok.FailureReason)
	}
}

// RunAll runs the selected scenarios in registry order, resets the engine
// before each, and says which ones it skipped.
func TestHarnessRunAll_ShouldResetBetweenScenariosAndNameTheSkipped(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	var out bytes.Buffer
	mock := NewMockContextEngine(nil)
	resets := &resetCounter{MockContextEngine: mock}
	h := NewHarnessWithObservability(kernel, SimulatorConfig{TokenBudget: 8000, CompressionEnabled: true}, &out, "json",
		nil, nil, nil, nil, nil, nil, resets)

	results, err := h.RunAll(context.Background())
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	mockIDs := h.ListScenarios()[:len(ScenariosByCategory(CategoryMock))]
	if len(results) != len(mockIDs) {
		t.Fatalf("ran %d scenarios, want the %d mock ones", len(results), len(mockIDs))
	}
	for i, r := range results {
		if r.Scenario.ScenarioID != mockIDs[i] {
			t.Fatalf("result %d is %s, want %s (registry order)", i, r.Scenario.ScenarioID, mockIDs[i])
		}
	}
	if resets.n != len(results) {
		t.Errorf("engine reset %d times for %d scenarios", resets.n, len(results))
	}
	if !strings.Contains(out.String(), "Skipped 7 scenario(s) that need --mode=real") {
		t.Errorf("the skipped integration scenarios were not named:\n%s", out.String())
	}
}

type resetCounter struct {
	*MockContextEngine
	n int
}

func (r *resetCounter) Reset() error { r.n++; return r.MockContextEngine.Reset() }

// The registry is one list: IDs are unique, every scenario says its category
// and engine mode, and the two agree. The CLI's --category filter found no
// mock scenario because none declared its category.
func TestScenarioRegistry_ShouldBeCompleteAndConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, sc := range AllScenarios() {
		if sc.ScenarioID == "" || seen[sc.ScenarioID] {
			t.Errorf("scenario ID %q is empty or duplicated", sc.ScenarioID)
		}
		seen[sc.ScenarioID] = true
		switch sc.Category {
		case CategoryMock:
			if sc.Mode != MockMode {
				t.Errorf("%s: mock scenario with mode %q", sc.ScenarioID, sc.Mode)
			}
		case CategoryIntegration:
			if sc.Mode != RealMode {
				t.Errorf("%s: integration scenario with mode %q", sc.ScenarioID, sc.Mode)
			}
		default:
			t.Errorf("%s: no category", sc.ScenarioID)
		}
	}
	if !seen["context-feedback-learning"] {
		t.Error("context-feedback-learning is not in the registry")
	}

	h := NewHarness(nil, SimulatorConfig{}, &bytes.Buffer{}, "json")
	if err := h.SelectCategory(CategoryMock); err != nil || len(h.ListScenarios()) != 8 {
		t.Errorf("--category=mock selected %v (err %v), want the 8 mock scenarios", h.ListScenarios(), err)
	}
	if err := h.SelectCategory("adversarial"); err == nil {
		t.Error("an unknown category was accepted")
	}
}
