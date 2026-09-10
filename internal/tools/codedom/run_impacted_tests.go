// Package codedom provides the run_impacted_tests tool for smart test selection.
package codedom

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
	Args      []any
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

// globalTestProvider is set by RegisterTestImpactProvider and read by the two
// impacted-test tools.
//
// It is guarded because a process can hold more than one Cortex — every
// system.NewCortex in the test suite builds another one, and the boot path
// registers from each. An unguarded package global written during one boot and
// read by a tool call on another goroutine is a data race that -race will find
// eventually and production will find first.
var (
	testProviderMu     sync.RWMutex
	globalTestProvider TestImpactProvider
)

// RegisterTestImpactProvider sets the provider for test impact analysis.
//
// Called from internal/system.wireTestImpactProvider during Cortex boot. Until
// 2026-09-09 nothing called it, which meant run_impacted_tests and
// get_impacted_tests were registered in both tool registries, advertised to the
// model in internal/prompt/atoms/capability/codedom_tools.yaml, and returned
// "test impact provider not initialized" every single time they were invoked —
// a whole turn spent to learn a capability does not exist.
//
// A nil provider clears the slot rather than panicking, so a boot path that
// could not build a kernel leaves the tools failing honestly instead of
// dereferencing nil.
func RegisterTestImpactProvider(provider TestImpactProvider) {
	testProviderMu.Lock()
	defer testProviderMu.Unlock()
	globalTestProvider = provider
}

// testImpactProvider returns the registered provider, or nil.
func testImpactProvider() TestImpactProvider {
	testProviderMu.RLock()
	defer testProviderMu.RUnlock()
	return globalTestProvider
}

// RunImpactedTestsTool returns the tool definition for running impacted tests.
func RunImpactedTestsTool() *tools.Tool {
	return &tools.Tool{
		Name:          "run_impacted_tests",
		AltCategories: []tools.ToolCategory{tools.CategoryAttack},
		Description:   "Run only the tests affected by recent code changes. Uses dependency analysis to select tests that need to run based on edited code elements.",
		Category:      tools.CategoryTest,
		Priority:      60,
		Schema: tools.ToolSchema{
			Required: []string{},
			Properties: map[string]tools.Property{
				"edited_refs": {
					Type:        "array",
					Description: "List of code element refs that were edited. If empty, uses plan_edit facts from kernel.",
					Items:       &tools.PropertyItems{Type: "string"},
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
		Name:          "get_impacted_tests",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral},
		Description:   "Query which tests would be affected by editing the specified code elements, without running them.",
		Category:      tools.CategoryTest,
		Priority:      55,
		Schema: tools.ToolSchema{
			Required: []string{},
			Properties: map[string]tools.Property{
				"edited_refs": {
					Type:        "array",
					Description: "List of code element refs to check. If empty, uses plan_edit facts from kernel.",
					Items:       &tools.PropertyItems{Type: "string"},
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

// editedRefsFromKernel collects the CodeDOM refs of everything edited so far.
//
// element_modified(Ref, SessionID, Timestamp) is emitted by every CodeDOM edit
// handler (internal/core/virtual_store_codedom.go) and carries a real ref, so
// it is the reliable source. plan_edit(Ref) is checked too because the
// predicate is declared for exactly this and a future producer may fill it;
// until 2026-09-09 its only producer emitted file paths into it, which matched
// nothing here and made both tools return "no impacted tests" for every real
// invocation.
func editedRefsFromKernel(kernel KernelQuerier) []string {
	if kernel == nil {
		return nil
	}
	seen := make(map[string]struct{}, 8)
	var refs []string
	for _, predicate := range []string{"element_modified", "plan_edit"} {
		facts, err := kernel.Query(predicate)
		if err != nil {
			logging.ToolsDebug("impacted tests: %s query failed: %v", predicate, err)
			continue
		}
		for _, fact := range facts {
			if len(fact.Args) == 0 {
				continue
			}
			ref, ok := fact.Args[0].(string)
			if !ok || ref == "" {
				continue
			}
			if _, dup := seen[ref]; dup {
				continue
			}
			seen[ref] = struct{}{}
			refs = append(refs, ref)
		}
	}
	return refs
}

// executeRunImpactedTests runs tests affected by code changes.
func executeRunImpactedTests(ctx context.Context, args map[string]any) (string, error) {
	provider := testImpactProvider()
	if provider == nil {
		return "", fmt.Errorf("test impact provider not initialized: run this from a booted workspace (nerd run/fix/chat), not a bare tool registry")
	}

	// Parse arguments
	editedRefs := parseStringArray(args["edited_refs"])
	includeLowPriority := parseBool(args["include_low_priority"], false)
	dryRun := parseBool(args["dry_run"], false)
	verbose := parseBool(args["verbose"], false)
	timeout := parseString(args["timeout"], "10m")

	// If no refs provided, ask the kernel what has been edited.
	if len(editedRefs) == 0 {
		editedRefs = editedRefsFromKernel(provider.GetKernel())
	}

	if len(editedRefs) == 0 {
		return "No edited refs specified and no plan_edit facts found in kernel.", nil
	}

	// Build test dependency graph
	analyzer := provider.NewTestDependencyAnalyzer()
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
		testResult, err := runGoTests(ctx, provider.GetProjectRoot(), packages, timeout, verbose)
		if err != nil {
			result.WriteString(fmt.Sprintf("Test execution failed: %v\n", err))
			result.WriteString(testResult)
			return result.String(), err
		}
		result.WriteString(testResult)
	} else {
		return result.String(), fmt.Errorf("no supported Go test packages found; verification was not executed")
	}

	return result.String(), nil
}

// executeGetImpactedTests queries impacted tests without running them.
func executeGetImpactedTests(ctx context.Context, args map[string]any) (string, error) {
	provider := testImpactProvider()
	if provider == nil {
		return "", fmt.Errorf("test impact provider not initialized: run this from a booted workspace (nerd run/fix/chat), not a bare tool registry")
	}

	// Parse arguments
	editedRefs := parseStringArray(args["edited_refs"])
	includeCoverageGaps := parseBool(args["include_coverage_gaps"], false)

	// If no refs provided, ask the kernel what has been edited.
	if len(editedRefs) == 0 {
		editedRefs = editedRefsFromKernel(provider.GetKernel())
	}

	// Build test dependency graph
	analyzer := provider.NewTestDependencyAnalyzer()
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
	// The timeout is a model-supplied string. A value that does not parse, or
	// that is outside the sane window, falls back to the default rather than
	// refusing the run: the packages and the workspace check below are what
	// bound what executes, and a test run that was asked for should happen.
	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil || timeoutDuration <= 0 || timeoutDuration > 24*time.Hour {
		timeoutDuration = 10 * time.Minute
	}
	if len(packages) == 0 {
		return "", fmt.Errorf("no test packages specified")
	}
	workspaceCtx := tools.WithWorkspaceRoot(ctx, projectRoot)
	projectRoot, err = tools.ResolveWorkspacePath(workspaceCtx, "", ".")
	if err != nil {
		return "", err
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	// Build relative package paths
	var relPackages []string
	for _, pkg := range packages {
		abs, err := tools.ResolveWorkspacePath(workspaceCtx, "", pkg)
		if err != nil {
			return "", err
		}
		relPkg, err := filepath.Rel(projectRoot, abs)
		if err != nil {
			return "", err
		}
		// Convert to Go package path
		relPkg = "./" + filepath.ToSlash(relPkg)
		relPackages = append(relPackages, relPkg)
	}

	// Build command
	args := []string{"test", "-count=1"}
	if verbose {
		args = append(args, "-v")
	}
	// The parsed duration, not the model's string: the raw value was handed
	// to `go test -timeout` verbatim, so an unparseable timeout "fell back"
	// for the context deadline and still made go test exit 2 on flag parsing.
	// The swallowed exit status hid that until the run's error was propagated.
	args = append(args, "-timeout", timeoutDuration.String())
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

	return result.String(), err
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
	case []any:
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
