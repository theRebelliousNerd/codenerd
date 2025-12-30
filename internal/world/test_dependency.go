// Package world provides test dependency analysis for smart test selection.
// The TestDependencyBuilder constructs a graph mapping test functions to the
// source code they exercise, enabling impact-based test selection.
package world

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/tools/codedom"
)

// TestDependencyBuilder builds and maintains the test→source dependency graph.
// It implements codedom.TestDependencyAnalyzer.
type TestDependencyBuilder struct {
	mu          sync.RWMutex
	kernel      codedom.KernelQuerier
	projectRoot string

	// Caches
	testFiles    map[string]bool     // File path → is test file
	testFuncs    map[string]bool     // Ref → is test function
	dependencies map[string][]string // Test Ref → Source Refs
}

// Verify TestDependencyBuilder implements the interface
var _ codedom.TestDependencyAnalyzer = (*TestDependencyBuilder)(nil)

// Test file detection patterns
var (
	goTestPattern     = regexp.MustCompile(`_test\.go$`)
	pyTestPattern     = regexp.MustCompile(`(^test_.*\.py$|_test\.py$)`)
	tsTestPattern     = regexp.MustCompile(`\.(test|spec)\.(ts|tsx|js|jsx)$`)
	rustTestPattern   = regexp.MustCompile(`/tests/.*\.rs$`)
	goTestFuncPattern = regexp.MustCompile(`:Test[A-Z]`)
	pyTestFuncPattern = regexp.MustCompile(`:test_`)
)

// NewTestDependencyBuilder creates a new TestDependencyBuilder.
func NewTestDependencyBuilder(kernel codedom.KernelQuerier, projectRoot string) *TestDependencyBuilder {
	return &TestDependencyBuilder{
		kernel:       kernel,
		projectRoot:  projectRoot,
		testFiles:    make(map[string]bool),
		testFuncs:    make(map[string]bool),
		dependencies: make(map[string][]string),
	}
}

// Build constructs the test dependency graph from the current codebase state.
func (b *TestDependencyBuilder) Build(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	logging.WorldDebug("Building test dependency graph for %s", b.projectRoot)

	// Phase 1: Identify all test files
	if err := b.identifyTestFiles(ctx); err != nil {
		return err
	}

	// Phase 2: Identify test functions
	if err := b.identifyTestFunctions(ctx); err != nil {
		return err
	}

	// Phase 3: Build dependency edges
	if err := b.buildDependencyEdges(ctx); err != nil {
		return err
	}

	logging.WorldDebug("Test dependency graph built: %d test files, %d test funcs, %d dependency edges",
		len(b.testFiles), len(b.testFuncs), b.countDependencies())

	return nil
}

// identifyTestFiles scans for test files based on naming conventions.
func (b *TestDependencyBuilder) identifyTestFiles(ctx context.Context) error {
	// Query all files from kernel
	facts, err := b.kernel.Query("file_topology")
	if err != nil {
		return err
	}

	for _, fact := range facts {
		if len(fact.Args) < 1 {
			continue
		}
		filePath, ok := fact.Args[0].(string)
		if !ok {
			continue
		}

		if b.isTestFile(filePath) {
			b.testFiles[filePath] = true
		}
	}

	return nil
}

// isTestFile checks if a file is a test file based on naming conventions.
func (b *TestDependencyBuilder) isTestFile(path string) bool {
	basename := filepath.Base(path)

	switch {
	case goTestPattern.MatchString(path):
		return true
	case pyTestPattern.MatchString(basename):
		return true
	case tsTestPattern.MatchString(path):
		return true
	case rustTestPattern.MatchString(path):
		return true
	}

	return false
}

// identifyTestFunctions identifies test functions by convention.
func (b *TestDependencyBuilder) identifyTestFunctions(ctx context.Context) error {
	// Query all code elements
	facts, err := b.kernel.Query("code_element")
	if err != nil {
		return err
	}

	for _, fact := range facts {
		if len(fact.Args) < 5 {
			continue
		}

		ref, ok := fact.Args[0].(string)
		if !ok {
			continue
		}

		elemType, ok := fact.Args[1].(string)
		if !ok {
			continue
		}

		file, ok := fact.Args[2].(string)
		if !ok {
			continue
		}

		// Only consider functions
		if elemType != "/function" && elemType != "function" {
			continue
		}

		// Check if in test file
		if b.testFiles[file] {
			// Check if function name matches test pattern
			if b.isTestFunction(ref, file) {
				b.testFuncs[ref] = true
			}
		}
	}

	return nil
}

// isTestFunction checks if a function ref matches test naming conventions.
func (b *TestDependencyBuilder) isTestFunction(ref, file string) bool {
	ext := filepath.Ext(file)

	switch ext {
	case ".go":
		return goTestFuncPattern.MatchString(ref)
	case ".py":
		return pyTestFuncPattern.MatchString(ref)
	case ".ts", ".tsx", ".js", ".jsx":
		// TypeScript/JS: functions named "test", "it", "describe" or matching patterns
		return strings.Contains(ref, ":test") || strings.Contains(ref, ":it(") || strings.Contains(ref, ":describe(")
	case ".rs":
		// Rust: functions with #[test] attribute (detected by parser)
		return strings.Contains(ref, "::test_")
	}

	return false
}

// buildDependencyEdges constructs edges from tests to source code.
func (b *TestDependencyBuilder) buildDependencyEdges(ctx context.Context) error {
	// Method 1: Direct call relationships
	callFacts, err := b.kernel.Query("code_calls")
	if err == nil {
		for _, fact := range callFacts {
			if len(fact.Args) < 2 {
				continue
			}
			caller, _ := fact.Args[0].(string)
			callee, _ := fact.Args[1].(string)

			if b.testFuncs[caller] {
				b.addDependency(caller, callee)
			}
		}
	}

	// Method 2: Import relationships
	importFacts, err := b.kernel.Query("file_imports")
	if err == nil {
		for _, fact := range importFacts {
			if len(fact.Args) < 2 {
				continue
			}
			importer, _ := fact.Args[0].(string)
			imported, _ := fact.Args[1].(string)

			if b.testFiles[importer] {
				// All test functions in importer depend on all public functions in imported
				b.addFileLevelDependency(importer, imported)
			}
		}
	}

	// Method 3: Same package access (Go-specific)
	b.buildPackageDependencies()

	return nil
}

// addDependency adds a test→source dependency edge.
func (b *TestDependencyBuilder) addDependency(testRef, sourceRef string) {
	// Don't add self-dependencies or test-to-test dependencies
	if testRef == sourceRef || b.testFuncs[sourceRef] {
		return
	}

	deps := b.dependencies[testRef]
	// Check for duplicates
	for _, existing := range deps {
		if existing == sourceRef {
			return
		}
	}
	b.dependencies[testRef] = append(deps, sourceRef)
}

// addFileLevelDependency adds dependencies from all tests in testFile to all elements in sourceFile.
func (b *TestDependencyBuilder) addFileLevelDependency(testFile, sourceFile string) {
	// Query elements in source file
	facts, err := b.kernel.Query("code_element")
	if err != nil {
		return
	}

	var sourceRefs []string
	var testRefs []string

	for _, fact := range facts {
		if len(fact.Args) < 5 {
			continue
		}
		ref, _ := fact.Args[0].(string)
		file, _ := fact.Args[2].(string)

		if file == sourceFile {
			sourceRefs = append(sourceRefs, ref)
		}
		if file == testFile && b.testFuncs[ref] {
			testRefs = append(testRefs, ref)
		}
	}

	// Add cross-product dependencies
	for _, testRef := range testRefs {
		for _, sourceRef := range sourceRefs {
			b.addDependency(testRef, sourceRef)
		}
	}
}

// buildPackageDependencies adds same-package dependencies (Go-specific).
func (b *TestDependencyBuilder) buildPackageDependencies() {
	// For each test file, add dependency to non-test files in same package
	facts, _ := b.kernel.Query("file_topology")
	allFiles := make(map[string]bool)
	for _, fact := range facts {
		if len(fact.Args) >= 1 {
			if f, ok := fact.Args[0].(string); ok {
				allFiles[f] = true
			}
		}
	}

	for testFile := range b.testFiles {
		dir := filepath.Dir(testFile)
		for sourceFile := range allFiles {
			if filepath.Dir(sourceFile) == dir && !b.testFiles[sourceFile] {
				b.addFileLevelDependency(testFile, sourceFile)
			}
		}
	}
}

// countDependencies returns the total number of dependency edges.
func (b *TestDependencyBuilder) countDependencies() int {
	count := 0
	for _, deps := range b.dependencies {
		count += len(deps)
	}
	return count
}

// GetImpactedTests returns tests affected by the given edited refs.
// Implements codedom.TestDependencyAnalyzer.
func (b *TestDependencyBuilder) GetImpactedTests(editedRefs []string) []codedom.ImpactedTestInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()

	editedSet := make(map[string]bool)
	for _, ref := range editedRefs {
		editedSet[ref] = true
	}

	var impacted []codedom.ImpactedTestInfo
	seen := make(map[string]bool)

	for testRef := range b.testFuncs {
		if seen[testRef] {
			continue
		}

		deps := b.dependencies[testRef]
		var matchedEdits []string
		priority := "low"

		// Check direct dependencies
		for _, dep := range deps {
			if editedSet[dep] {
				matchedEdits = append(matchedEdits, dep)
				priority = "high"
			}
		}

		// Check transitive dependencies (simplified - could query kernel for full transitivity)
		if len(matchedEdits) == 0 {
			for _, dep := range deps {
				depDeps := b.dependencies[dep]
				for _, transitive := range depDeps {
					if editedSet[transitive] {
						matchedEdits = append(matchedEdits, transitive)
						if priority != "high" {
							priority = "medium"
						}
					}
				}
			}
		}

		if len(matchedEdits) > 0 {
			impacted = append(impacted, codedom.ImpactedTestInfo{
				TestRef:    testRef,
				TestFile:   b.getTestFile(testRef),
				Priority:   priority,
				Reason:     "depends_on_edited_code",
				EditedRefs: matchedEdits,
			})
			seen[testRef] = true
		}
	}

	return impacted
}

// getTestFile returns the file containing a test function.
func (b *TestDependencyBuilder) getTestFile(testRef string) string {
	facts, err := b.kernel.Query("code_element")
	if err != nil {
		return ""
	}

	for _, fact := range facts {
		if len(fact.Args) >= 5 {
			ref, _ := fact.Args[0].(string)
			file, _ := fact.Args[2].(string)
			if ref == testRef {
				return file
			}
		}
	}

	return ""
}

// GetImpactedTestPackages returns Go packages containing impacted tests.
// Implements codedom.TestDependencyAnalyzer.
func (b *TestDependencyBuilder) GetImpactedTestPackages(editedRefs []string) []string {
	impacted := b.GetImpactedTests(editedRefs)

	pkgSet := make(map[string]bool)
	for _, test := range impacted {
		if test.TestFile != "" {
			pkg := filepath.Dir(test.TestFile)
			pkgSet[pkg] = true
		}
	}

	var packages []string
	for pkg := range pkgSet {
		packages = append(packages, pkg)
	}

	return packages
}

// GetCoverageGaps returns public functions without test coverage.
// Implements codedom.TestDependencyAnalyzer.
func (b *TestDependencyBuilder) GetCoverageGaps() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Build set of covered refs
	covered := make(map[string]bool)
	for _, deps := range b.dependencies {
		for _, dep := range deps {
			covered[dep] = true
		}
	}

	// Query public functions
	facts, err := b.kernel.Query("element_visibility")
	if err != nil {
		return nil
	}

	var gaps []string
	for _, fact := range facts {
		if len(fact.Args) < 2 {
			continue
		}
		ref, _ := fact.Args[0].(string)
		vis, _ := fact.Args[1].(string)

		if (vis == "/public" || vis == "public") && !covered[ref] && !b.testFuncs[ref] {
			gaps = append(gaps, ref)
		}
	}

	return gaps
}

// GetTestFiles returns all identified test files.
func (b *TestDependencyBuilder) GetTestFiles() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var files []string
	for f := range b.testFiles {
		files = append(files, f)
	}
	return files
}

// GetTestFunctions returns all identified test function refs.
func (b *TestDependencyBuilder) GetTestFunctions() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var refs []string
	for r := range b.testFuncs {
		refs = append(refs, r)
	}
	return refs
}
