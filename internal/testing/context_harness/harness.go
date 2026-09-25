package context_harness

import (
	"context"
	"fmt"
	"io"
	"strings"

	"codenerd/internal/core"
)

// Harness is the main orchestrator for context system testing.
type Harness struct {
	kernel    *core.RealKernel
	config    SimulatorConfig
	reporter  *Reporter
	output    io.Writer
	scenarios map[string]*Scenario
	order     []string // scenario IDs in registry order

	// Engine (mock or real) - implements ContextEngine interface
	contextEngine ContextEngine

	// Observability components (optional)
	promptInspector  *PromptInspector
	jitTracer        *JITTracer
	activationTracer *ActivationTracer
	compressionViz   *CompressionVisualizer
	piggybackTracer  *PiggybackTracer
	feedbackTracer   *FeedbackTracer
}

// NewHarness creates a new test harness.
func NewHarness(kernel *core.RealKernel, config SimulatorConfig, output io.Writer, outputFormat string) *Harness {
	// Load all scenarios - use ScenarioID (kebab-case) as key, not Name
	scenarios := make(map[string]*Scenario)
	var order []string
	for _, scenario := range AllScenarios() {
		scenarios[scenario.ScenarioID] = scenario
		order = append(order, scenario.ScenarioID)
	}

	// Create engine based on mode
	var engine ContextEngine
	if config.Mode == RealMode {
		// Will be set via SetContextEngine for real mode
		engine = nil
	} else {
		// Default to mock mode
		engine = NewMockContextEngine(kernel)
	}

	return &Harness{
		kernel:        kernel,
		config:        config,
		reporter:      NewReporter(output, outputFormat),
		output:        output,
		scenarios:     scenarios,
		order:         order,
		contextEngine: engine,
	}
}

// SelectCategory narrows the harness to one scenario category ("mock" or
// "integration"). The CLI's --category flag was parsed and then ignored.
func (h *Harness) SelectCategory(category ScenarioCategory) error {
	selected := ScenariosByCategory(category)
	if len(selected) == 0 {
		return fmt.Errorf("no scenarios in category %q (known: %s, %s)", category, CategoryMock, CategoryIntegration)
	}
	h.scenarios = make(map[string]*Scenario, len(selected))
	h.order = h.order[:0]
	for _, sc := range selected {
		h.scenarios[sc.ScenarioID] = sc
		h.order = append(h.order, sc.ScenarioID)
	}
	return nil
}

// NewHarnessWithObservability creates a harness with full observability wired in.
func NewHarnessWithObservability(
	kernel *core.RealKernel,
	config SimulatorConfig,
	output io.Writer,
	outputFormat string,
	promptInspector *PromptInspector,
	jitTracer *JITTracer,
	activationTracer *ActivationTracer,
	compressionViz *CompressionVisualizer,
	piggybackTracer *PiggybackTracer,
	feedbackTracer *FeedbackTracer,
	contextEngine ContextEngine, // Changed from *RealContextEngine to interface
) *Harness {
	h := NewHarness(kernel, config, output, outputFormat)

	// Store observability components
	h.promptInspector = promptInspector
	h.jitTracer = jitTracer
	h.activationTracer = activationTracer
	h.compressionViz = compressionViz
	h.piggybackTracer = piggybackTracer
	h.feedbackTracer = feedbackTracer

	// Override engine if provided
	if contextEngine != nil {
		h.contextEngine = contextEngine
	}

	return h
}

// RunScenario runs a single named scenario.
func (h *Harness) RunScenario(ctx context.Context, scenarioName string) (*TestResult, error) {
	scenario, ok := h.scenarios[scenarioName]
	if !ok {
		return nil, fmt.Errorf("unknown scenario: %s", scenarioName)
	}

	simulator := NewSessionSimulator(h.kernel, h.config)

	// Wire in observability if available
	if h.promptInspector != nil || h.jitTracer != nil || h.activationTracer != nil || h.compressionViz != nil || h.piggybackTracer != nil || h.feedbackTracer != nil {
		simulator.SetObservability(
			h.promptInspector,
			h.jitTracer,
			h.activationTracer,
			h.compressionViz,
			h.piggybackTracer,
			h.feedbackTracer,
		)
	}

	// Wire in context engine if available
	if h.contextEngine != nil {
		simulator.SetContextEngine(h.contextEngine)
	}

	result, err := simulator.RunScenario(ctx, scenario)
	if err != nil {
		return nil, fmt.Errorf("scenario execution failed: %w", err)
	}

	// Report results
	if err := h.reporter.Report(result); err != nil {
		return nil, fmt.Errorf("reporting failed: %w", err)
	}

	return result, nil
}

// RunAll runs every selected scenario the engine can run, in registry order,
// resetting the engine before each so one scenario's facts cannot answer the
// next one's checkpoints (it used to run them in map order on one engine that
// kept every fact). Scenarios that need the real engine are skipped on the
// mock one, and the skip is written to the output, not passed over.
func (h *Harness) RunAll(ctx context.Context) ([]*TestResult, error) {
	results := make([]*TestResult, 0, len(h.scenarios))

	var skipped []string
	for _, name := range h.order {
		if h.scenarios[name].Mode == RealMode && (h.contextEngine == nil || h.contextEngine.GetMode() != RealMode) {
			skipped = append(skipped, name)
			continue
		}
		if h.contextEngine != nil {
			if err := h.contextEngine.Reset(); err != nil {
				return nil, fmt.Errorf("resetting the engine before %s: %w", name, err)
			}
		}
		result, err := h.RunScenario(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("scenario %s failed: %w", name, err)
		}
		results = append(results, result)
	}
	if len(skipped) > 0 && h.output != nil {
		fmt.Fprintf(h.output, "Skipped %d scenario(s) that need --mode=real: %s\n", len(skipped), strings.Join(skipped, ", "))
	}

	// Report summary
	if err := h.reporter.ReportSummary(results); err != nil {
		return nil, fmt.Errorf("summary reporting failed: %w", err)
	}

	return results, nil
}

// ListScenarios returns the IDs of the selected scenarios, in registry order.
func (h *Harness) ListScenarios() []string {
	return append([]string(nil), h.order...)
}
