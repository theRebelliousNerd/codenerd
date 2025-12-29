package context_harness

import (
	"context"
	"fmt"
	"io"

	"codenerd/internal/core"
)

// Harness is the main orchestrator for context system testing.
type Harness struct {
	kernel    *core.RealKernel
	config    SimulatorConfig
	reporter  *Reporter
	scenarios map[string]*Scenario

	// Observability components (optional)
	promptInspector  *PromptInspector
	jitTracer        *JITTracer
	activationTracer *ActivationTracer
	compressionViz   *CompressionVisualizer
	contextEngine    *RealContextEngine
}

// NewHarness creates a new test harness.
func NewHarness(kernel *core.RealKernel, config SimulatorConfig, output io.Writer, outputFormat string) *Harness {
	// Load all scenarios - use ScenarioID (kebab-case) as key, not Name
	scenarios := make(map[string]*Scenario)
	for _, scenario := range AllScenarios() {
		scenarios[scenario.ScenarioID] = scenario
	}

	return &Harness{
		kernel:    kernel,
		config:    config,
		reporter:  NewReporter(output, outputFormat),
		scenarios: scenarios,
	}
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
	contextEngine *RealContextEngine,
) *Harness {
	h := NewHarness(kernel, config, output, outputFormat)

	// Store observability components
	h.promptInspector = promptInspector
	h.jitTracer = jitTracer
	h.activationTracer = activationTracer
	h.compressionViz = compressionViz
	h.contextEngine = contextEngine

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
	if h.promptInspector != nil || h.jitTracer != nil || h.activationTracer != nil || h.compressionViz != nil {
		simulator.SetObservability(
			h.promptInspector,
			h.jitTracer,
			h.activationTracer,
			h.compressionViz,
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

// RunAll runs all available scenarios.
func (h *Harness) RunAll(ctx context.Context) ([]*TestResult, error) {
	results := make([]*TestResult, 0, len(h.scenarios))

	for name := range h.scenarios {
		result, err := h.RunScenario(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("scenario %s failed: %w", name, err)
		}
		results = append(results, result)
	}

	// Report summary
	if err := h.reporter.ReportSummary(results); err != nil {
		return nil, fmt.Errorf("summary reporting failed: %w", err)
	}

	return results, nil
}

// ListScenarios returns the names of all available scenarios.
func (h *Harness) ListScenarios() []string {
	names := make([]string, 0, len(h.scenarios))
	for name := range h.scenarios {
		names = append(names, name)
	}
	return names
}
