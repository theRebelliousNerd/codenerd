package context_harness

import (
	"context"
	"fmt"
	"io"

	"codenerd/internal/core"
)

// Harness is the main orchestrator for context system testing.
type Harness struct {
	kernel    *core.Kernel
	config    SimulatorConfig
	reporter  *Reporter
	scenarios map[string]*Scenario
}

// NewHarness creates a new test harness.
func NewHarness(kernel *core.Kernel, config SimulatorConfig, output io.Writer, outputFormat string) *Harness {
	// Load all scenarios
	scenarios := make(map[string]*Scenario)
	for _, scenario := range AllScenarios() {
		scenarios[scenario.Name] = scenario
	}

	return &Harness{
		kernel:    kernel,
		config:    config,
		reporter:  NewReporter(output, outputFormat),
		scenarios: scenarios,
	}
}

// RunScenario runs a single named scenario.
func (h *Harness) RunScenario(ctx context.Context, scenarioName string) (*TestResult, error) {
	scenario, ok := h.scenarios[scenarioName]
	if !ok {
		return nil, fmt.Errorf("unknown scenario: %s", scenarioName)
	}

	simulator := NewSessionSimulator(h.kernel, h.config)
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
