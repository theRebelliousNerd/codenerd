// Package codedom provides the run_impacted_tests tool for smart test selection.
package codedom

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// TestDependencyAnalyzer is the interface for test dependency analysis.
// This interface is implemented by world.TestDependencyBuilder to break import cycles.
type TestDependencyAnalyzer interface {
	// Build constructs the test dependency graph.
	Build(ctx context.Context) error

	// GetImpactedTests returns tests affected by the given edited refs.
	GetImpactedTests(editedRefs []string) []ImpactedTestInfo

	// GetImpactedTestPackages returns Go packages containing impacted tests.
	GetImpactedTestPackages(editedRefs []string) []string

	// GetCoverageGaps returns public functions without test coverage.
	GetCoverageGaps() []string
}

// KernelQuerier is the interface for querying the kernel.
// This interface is implemented by core.RealKernel to break import cycles.
type KernelQuerier interface {
	// Query returns facts matching the predicate.
	Query(predicate string) ([]FactData, error)
}

// FactData represents a fact from the kernel.
type FactData struct {
	Predicate string
	Args      []interface{}
}

// ImpactedTestInfo represents a test affected by code changes.
type ImpactedTestInfo struct {
	TestRef    string
	TestFile   string
	Priority   string // "high", "medium", "low"
	Reason     string
	EditedRefs []string
}

// TestImpactProvider provides access to test impact analysis dependencies.
type TestImpactProvider interface {
	GetKernel() KernelQuerier
	GetProjectRoot() string
	NewTestDependencyAnalyzer() TestDependencyAnalyzer
}

// globalTestProvider is set by RegisterTestImpactProvider.
var globalTestProvider TestImpactProvider

// RegisterTestImpactProvider sets the provider for test impact analysis.
// This should be called during initialization.
func RegisterTestImpactProvider(provider TestImpactProvider) {
	globalTestProvider = provider
}

// RunImpactedTestsTool returns the tool definition for running impacted tests.
func RunImpactedTestsTool() *tools.Tool {
	return &tools.Tool{
		Name:        "run_impacted_tests",
		Description: "Run only the tests affected by recent code changes. Uses dependency analysis to select tests that need to run based on edited code elements.",
		Category:    tools.CategoryTest,
		Priority:    60,
		Schema: tools.ToolSchema{
			Required: []string{},
			Properties: map[string]tools.Property{
				"edited_refs": {
					Type:        "array",
					Description: "List of code element refs that were edited. If empty, uses plan_edit facts from kernel.",
				},
				"include_low_priority": {
					Type:        "boolean",
					Description: "Include low-priority tests (same package but no direct dependency). Default false.",
					Default:     false,
				},
				"dry_run": {
					Type:        "boolean",
					Description: "If true, only report which tests would run without executing them.",
					Default:     false,
				},
				"verbose": {
					Type:        "boolean",
					Description: "Show detailed test output.",
					Default:     false,
				},
				"timeout": {
					Type:        "string",
					Description: "Timeout for test execution (e.g., '5m', '30s'). Default '10m'.",
					Default:     "10m",
				},
			},
		},
		Execute: executeRunImpactedTests,
	}
}

// GetImpactedTestsTool returns a tool that only queries impacted tests without running them.
func GetImpactedTestsTool() *tools.Tool {
	return &tools.Tool{
		Name:        "get_impacted_tests",
		Description: "Query which tests would be affected by editing the specified code elements, without running them.",
		Category:    tools.CategoryTest,
		Priority:    55,
		Schema: tools.ToolSchema{
			Required: []string{},
			Properties: map[string]tools.Property{
				"edited_refs": {
					Type:        "array",
					Description: "List of code element refs to check. If empty, uses plan_edit facts from kernel.",
				},
				"include_coverage_gaps": {
					Type:        "boolean",
					Description: "Also report code elements without test coverage.",
					Default:     false,
				},
			},
		},
		Execute: executeGetImpactedTests,
	}
}

// executeRunImpactedTests runs tests affected by code changes.
func executeRunImpactedTests(ctx context.Context, args map[string]any) (string, error) {
	if globalTestProvider == nil {
		return "", fmt.Errorf("test impact provider not initialized")
	}

	// Parse arguments
	editedRefs := parseStringArray(args["edited_refs"])
	includeLowPriority := parseBool(args["include_low_priority"], false)
	dryRun := parseBool(args["dry_run"], false)
	verbose := parseBool(args["verbose"], false)
	timeout := parseString(args["timeout"], "10m")

	// If no refs provided, query kernel for plan_edit facts
	if len(editedRefs) == 0 {
		kernel := globalTestProvider.GetKernel()
		facts, err := kernel.Query("plan_edit")
		if err == nil {
			for _, fact := range facts {
				if len(fact.Args) >= 1 {
					if ref, ok := fact.Args[0].(string); ok {
						editedRefs = append(editedRefs, ref)
					}
				}
			}
		}
	}

	if len(editedRefs) == 0 {
		return "No edited refs specified and no plan_edit facts found in kernel.", nil
	}

	// Build test dependency graph
	analyzer := globalTestProvider.NewTestDependencyAnalyzer()
	if err := analyzer.Build(ctx); err != nil {
		return "", fmt.Errorf("failed to build test dependency graph: %w", err)
	}

	// Get impacted tests
	impactedTests := analyzer.GetImpactedTests(editedRefs)

	// Filter by priority
	var testsToRun []ImpactedTestInfo
	for _, test := range impactedTests {
		if test.Priority == "low" && !includeLowPriority {
			continue
		}
		testsToRun = append(testsToRun, test)
	}

	if len(testsToRun) == 0 {
		return "No impacted tests found for the edited code.", nil
	}

	// Build result
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d impacted tests:\n\n", len(testsToRun)))

	// Group by priority
	highPriority := filterByPriority(testsToRun, "high")
	mediumPriority := filterByPriority(testsToRun, "medium")
	lowPriority := filterByPriority(testsToRun, "low")

	if len(highPriority) > 0 {
		result.WriteString(fmt.Sprintf("High Priority (%d tests):\n", len(highPriority)))
		for _, t := range highPriority {
			result.WriteString(fmt.Sprintf("  - %s\n", t.TestRef))
		}
		result.WriteString("\n")
	}

	if len(mediumPriority) > 0 {
		result.WriteString(fmt.Sprintf("Medium Priority (%d tests):\n", len(mediumPriority)))
		for _, t := range mediumPriority {
			result.WriteString(fmt.Sprintf("  - %s\n", t.TestRef))
		}
		result.WriteString("\n")
	}

	if len(lowPriority) > 0 {
		result.WriteString(fmt.Sprintf("Low Priority (%d tests):\n", len(lowPriority)))
		for _, t := range lowPriority {
			result.WriteString(fmt.Sprintf("  - %s\n", t.TestRef))
		}
		result.WriteString("\n")
	}

	if dryRun {
		result.WriteString("(dry run - tests not executed)\n")
		return result.String(), nil
	}

	// Execute tests
	result.WriteString("Executing tests...\n\n")

	// Get unique packages
	packages := analyzer.GetImpactedTestPackages(editedRefs)

	if len(packages) > 0 {
		// Run go test on impacted packages
		testResult, err := runGoTests(ctx, globalTestProvider.GetProjectRoot(), packages, timeout, verbose)
		if err != nil {
			result.WriteString(fmt.Sprintf("Test execution failed: %v\n", err))
		}
		result.WriteString(testResult)
	}

	return result.String(), nil
}

// executeGetImpactedTests queries impacted tests without running them.
func executeGetImpactedTests(ctx context.Context, args map[string]any) (string, error) {
	if globalTestProvider == nil {
		return "", fmt.Errorf("test impact provider not initialized")
	}

	// Parse arguments
	editedRefs := parseStringArray(args["edited_refs"])
	includeCoverageGaps := parseBool(args["include_coverage_gaps"], false)

	// If no refs provided, query kernel for plan_edit facts
	if len(editedRefs) == 0 {
		kernel := globalTestProvider.GetKernel()
		facts, err := kernel.Query("plan_edit")
		if err == nil {
			for _, fact := range facts {
				if len(fact.Args) >= 1 {
					if ref, ok := fact.Args[0].(string); ok {
						editedRefs = append(editedRefs, ref)
					}
				}
			}
		}
	}

	// Build test dependency graph
	analyzer := globalTestProvider.NewTestDependencyAnalyzer()
	if err := analyzer.Build(ctx); err != nil {
		return "", fmt.Errorf("failed to build test dependency graph: %w", err)
	}

	// Build response structure
	response := struct {
		EditedRefs    []string `json:"edited_refs"`
		ImpactedTests []struct {
			Ref      string   `json:"ref"`
			File     string   `json:"file"`
			Priority string   `json:"priority"`
			Reason   string   `json:"reason"`
			Triggers []string `json:"triggers"`
		} `json:"impacted_tests"`
		ImpactedPackages []string `json:"impacted_packages"`
		CoverageGaps     []string `json:"coverage_gaps,omitempty"`
	}{
		EditedRefs: editedRefs,
	}

	// Get impacted tests
	impactedTests := analyzer.GetImpactedTests(editedRefs)
	for _, test := range impactedTests {
		response.ImpactedTests = append(response.ImpactedTests, struct {
			Ref      string   `json:"ref"`
			File     string   `json:"file"`
			Priority string   `json:"priority"`
			Reason   string   `json:"reason"`
			Triggers []string `json:"triggers"`
		}{
			Ref:      test.TestRef,
			File:     test.TestFile,
			Priority: test.Priority,
			Reason:   test.Reason,
			Triggers: test.EditedRefs,
		})
	}

	// Get impacted packages
	response.ImpactedPackages = analyzer.GetImpactedTestPackages(editedRefs)

	// Get coverage gaps if requested
	if includeCoverageGaps {
		response.CoverageGaps = analyzer.GetCoverageGaps()
	}

	// Marshal to JSON
	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(jsonBytes), nil
}

// runGoTests runs go test on the specified packages.
func runGoTests(ctx context.Context, projectRoot string, packages []string, timeout string, verbose bool) (string, error) {
	// Parse timeout
	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		timeoutDuration = 10 * time.Minute
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	// Build relative package paths
	var relPackages []string
	for _, pkg := range packages {
		relPkg, err := filepath.Rel(projectRoot, pkg)
		if err != nil {
			relPkg = pkg
		}
		// Convert to Go package path
		relPkg = "./" + filepath.ToSlash(relPkg)
		relPackages = append(relPackages, relPkg)
	}

	// Build command
	args := []string{"test"}
	if verbose {
		args = append(args, "-v")
	}
	args = append(args, "-timeout", timeout)
	args = append(args, relPackages...)

	logging.WorldDebug("Running: go %s", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Command: go %s\n", strings.Join(args, " ")))
	result.WriteString(fmt.Sprintf("Directory: %s\n\n", projectRoot))
	result.Write(output)

	if err != nil {
		result.WriteString(fmt.Sprintf("\nError: %v\n", err))
	}

	return result.String(), nil
}

// filterByPriority filters tests by priority level.
func filterByPriority(tests []ImpactedTestInfo, priority string) []ImpactedTestInfo {
	var filtered []ImpactedTestInfo
	for _, t := range tests {
		if t.Priority == priority {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// Helper functions for parsing tool arguments

func parseStringArray(v any) []string {
	if v == nil {
		return nil
	}

	switch arr := v.(type) {
	case []string:
		return arr
	case []interface{}:
		var result []string
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}

	return nil
}

func parseBool(v any, defaultVal bool) bool {
	if v == nil {
		return defaultVal
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return defaultVal
}

func parseString(v any, defaultVal string) string {
	if v == nil {
		return defaultVal
	}
	if s, ok := v.(string); ok {
		return s
	}
	return defaultVal
}
