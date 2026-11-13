// Package world provides test dependency analysis for smart test selection.
// The TestDependencyBuilder constructs a graph mapping test functions to the
// source code they exercise, enabling impact-based test selection.
package world

import (
	"context"
	"fmt"
	"iter"
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
	testFiles map[string]bool // File path → is test file
	testFuncs map[string]bool // Ref → is test function
	elements  []codedom.FactData
	aliases   map[string]string

	// The graph (buildDependencyEdges). callDeps is the call edges, each
	// caller's callees in first-seen order; fileDeps is the file-level
	// edges, a test file's source files in first-seen order, unexpanded.
	// refsByFile, testRefsByFile and fileOfRef index the elements once.
	callDeps       map[string][]string
	callSeen       map[string]map[string]struct{}
	fileDeps       map[string][]string
	fileDepSeen    map[string]map[string]struct{}
	refsByFile     map[string][]string
	testRefsByFile map[string][]string
	fileOfRef      map[string]string
}

// Verify TestDependencyBuilder implements the interface
var _ codedom.TestDependencyAnalyzer = (*TestDependencyBuilder)(nil)

// Test file detection patterns
var (
	goTestPattern   = regexp.MustCompile(`_test\.go$`)
	pyTestPattern   = regexp.MustCompile(`(^test_.*\.py$|_test\.py$)`)
	tsTestPattern   = regexp.MustCompile(`\.(test|spec)\.(ts|tsx|js|jsx)$`)
	rustTestPattern = regexp.MustCompile(`/tests/.*\.rs$`)

	// goTestFuncPattern matches the bare symbol name, not the whole ref.
	//
	// It used to be `:Test[A-Z]`, matched against the raw ref. The Go parser
	// builds refs as `fn:<pkg>.<Name>` (go_parser.go:174-178) — so a real test
	// function is `fn:world.TestFoo`, the character after the colon is `w`, and
	// the pattern matched nothing this parser has ever produced. The only refs
	// it did match are the unqualified `fn:<name>` form from the tree-sitter
	// path (ast_treesitter.go:487). Two producers, two ref shapes, one
	// consumer that only handled the rarer one: testFuncs stayed empty, so
	// buildDependencyEdges added no edges, so GetImpactedTests returned nothing
	// for every input. Matching the symbol name handles both shapes.
	//
	// Benchmark/Fuzz/Example are included because `go test` builds and can run
	// them: they live in the same file and are impacted by the same edit.
	goTestFuncPattern = regexp.MustCompile(`^(Test|Benchmark|Fuzz|Example)([^a-z]|$)`)
	pyTestFuncPattern = regexp.MustCompile(`^test(_|$)`)
	rsTestFuncPattern = regexp.MustCompile(`^test(_|$)`)
	tsTestFuncPattern = regexp.MustCompile(`^(test|it|describe|suite)([^a-z]|$)`)
)

// SymbolBareName returns the bare symbol name from a CodeDOM ref or a
// symbol_graph SymbolID (func:Name, method:Recv.Name, field:Type.Name).
//
// Refs come in three shapes across the parsers:
//
//	fn:world.TestFoo                 (Go function,  go_parser.go:178)
//	fn:world.Provider.GetContext     (Go method,    go_parser.go:176)
//	fn:TestFoo                       (tree-sitter,  ast_treesitter.go:487)
//
// Taking the text after the last ':' strips the type prefix and also the '::'
// of a Rust path; taking the text after the last '.' strips package and
// receiver qualification. What is left is the name a naming convention is
// actually about.
func SymbolBareName(ref string) string {
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		ref = ref[i+1:]
	}
	if i := strings.LastIndex(ref, "."); i >= 0 {
		ref = ref[i+1:]
	}
	return ref
}

// NewTestDependencyBuilder creates a new TestDependencyBuilder.
func NewTestDependencyBuilder(kernel codedom.KernelQuerier, projectRoot string) *TestDependencyBuilder {
	return &TestDependencyBuilder{
		kernel:      kernel,
		projectRoot: projectRoot,
		testFiles:   make(map[string]bool),
		testFuncs:   make(map[string]bool),
	}
}

// Build constructs the test dependency graph from the current codebase state.
func (b *TestDependencyBuilder) Build(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	logging.WorldDebug("Building test dependency graph for %s", b.projectRoot)
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.kernel == nil {
		return fmt.Errorf("test dependency kernel is unavailable")
	}
	b.testFiles = make(map[string]bool)
	b.testFuncs = make(map[string]bool)
	if err := b.loadElements(); err != nil {
		return err
	}

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
	facts := b.elements

	for _, fact := range facts {
		if len(fact.Args) < 5 {
			continue
		}

		ref, ok := fact.Args[0].(string)
		if !ok {
			continue
		}

		elemType := fmt.Sprint(fact.Args[1])

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
//
// The conventions are about the symbol name, so the ref is reduced to its name
// first. Matching the raw ref made every Go and Python check fail — see
// goTestFuncPattern.
func (b *TestDependencyBuilder) isTestFunction(ref, file string) bool {
	name := SymbolBareName(ref)
	if name == "" {
		return false
	}

	switch filepath.Ext(file) {
	case ".go":
		return goTestFuncPattern.MatchString(name)
	case ".py":
		return pyTestFuncPattern.MatchString(name)
	case ".ts", ".tsx", ".js", ".jsx":
		return tsTestFuncPattern.MatchString(name)
	case ".rs":
		return rsTestFuncPattern.MatchString(name)
	}

	return false
}

// buildDependencyEdges records what each test depends on.
//
// Two kinds of edge, kept apart. A call edge is recorded as it is. A
// file-level edge -- a test file imports a file, or sits beside it in a Go
// package -- says every test function of the one depends on every element of
// the other; it is recorded as the pair of files and expanded when asked.
//
// It used to be expanded at build time, and each expansion rescanned every
// element to find the two files' symbols, once per import edge of a test file
// and once per same-directory pair found by comparing every test file with
// every file; each resulting edge was then added with a linear duplicate scan
// of the test's list. On a workspace of a few thousand test files that is
// billions of steps and millions of stored edges before the first question
// is answered. The answers are the same (see the reference builder in the
// tests).
func (b *TestDependencyBuilder) buildDependencyEdges(ctx context.Context) error {
	b.indexElements()

	// Method 1: Direct call relationships
	callFacts, err := b.kernel.Query("code_calls")
	if err == nil {
		for _, fact := range callFacts {
			if len(fact.Args) < 2 {
				continue
			}
			caller, _ := fact.Args[0].(string)
			callee, _ := fact.Args[1].(string)
			if caller == "" {
				caller = fmt.Sprint(fact.Args[0])
			}
			if callee == "" {
				callee = fmt.Sprint(fact.Args[1])
			}
			if ref := b.aliases[caller]; ref != "" {
				caller = ref
			}
			if ref := b.aliases[callee]; ref != "" {
				callee = ref
			}
			if caller != "" && callee != "" {
				b.addCallDependency(caller, callee)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
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
				// All test functions in importer depend on all elements in imported
				b.addFileLevelDependency(importer, imported)
			}
		}
	}

	// Method 3: Same package access (Go-specific)
	b.buildPackageDependencies()

	return nil
}

// indexElements groups the elements by file, in element order, and records
// each ref's first file. Every later step reads these instead of rescanning.
func (b *TestDependencyBuilder) indexElements() {
	b.refsByFile = make(map[string][]string)
	b.testRefsByFile = make(map[string][]string)
	b.fileOfRef = make(map[string]string)
	b.callDeps = make(map[string][]string)
	b.callSeen = make(map[string]map[string]struct{})
	b.fileDeps = make(map[string][]string)
	b.fileDepSeen = make(map[string]map[string]struct{})
	for _, fact := range b.elements {
		if len(fact.Args) < 5 {
			continue
		}
		ref, _ := fact.Args[0].(string)
		file, _ := fact.Args[2].(string)
		b.refsByFile[file] = append(b.refsByFile[file], ref)
		if b.testFuncs[ref] {
			b.testRefsByFile[file] = append(b.testRefsByFile[file], ref)
		}
		if _, known := b.fileOfRef[ref]; !known {
			b.fileOfRef[ref] = file
		}
	}
}

// addCallDependency adds a caller->callee edge, once.
func (b *TestDependencyBuilder) addCallDependency(from, to string) {
	// Don't add self-dependencies or dependencies on a test.
	if from == to || b.testFuncs[to] {
		return
	}
	seen := b.callSeen[from]
	if seen == nil {
		seen = make(map[string]struct{})
		b.callSeen[from] = seen
	}
	if _, dup := seen[to]; dup {
		return
	}
	seen[to] = struct{}{}
	b.callDeps[from] = append(b.callDeps[from], to)
}

// addFileLevelDependency records that every test function of testFile depends
// on every element of sourceFile.
func (b *TestDependencyBuilder) addFileLevelDependency(testFile, sourceFile string) {
	seen := b.fileDepSeen[testFile]
	if seen == nil {
		seen = make(map[string]struct{})
		b.fileDepSeen[testFile] = seen
	}
	if _, dup := seen[sourceFile]; dup {
		return
	}
	seen[sourceFile] = struct{}{}
	b.fileDeps[testFile] = append(b.fileDeps[testFile], sourceFile)
}

// buildPackageDependencies adds same-package dependencies (Go-specific): each
// test file depends on the non-test files of its directory.
func (b *TestDependencyBuilder) buildPackageDependencies() {
	facts, _ := b.kernel.Query("file_topology")
	filesByDir := make(map[string][]string)
	listed := make(map[string]bool)
	for _, fact := range facts {
		if len(fact.Args) >= 1 {
			if f, ok := fact.Args[0].(string); ok && !listed[f] {
				listed[f] = true
				filesByDir[filepath.Dir(f)] = append(filesByDir[filepath.Dir(f)], f)
			}
		}
	}

	for testFile := range b.testFiles {
		for _, sourceFile := range filesByDir[filepath.Dir(testFile)] {
			if !b.testFiles[sourceFile] {
				b.addFileLevelDependency(testFile, sourceFile)
			}
		}
	}
}

// fileLevelRefs yields the elements of one of ref's file-level dependencies:
// a test function depends on every element of sourceFile that is not itself
// and not a test.
func (b *TestDependencyBuilder) fileLevelRefs(ref, sourceFile string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, sourceRef := range b.refsByFile[sourceFile] {
			if sourceRef == ref || b.testFuncs[sourceRef] {
				continue
			}
			if !yield(sourceRef) {
				return
			}
		}
	}
}

// fileDepsOf is the file-level dependencies of ref: its file's, when ref is
// one of that file's test functions.
func (b *TestDependencyBuilder) fileDepsOf(ref string) []string {
	if !b.testFuncs[ref] {
		return nil
	}
	return b.fileDeps[b.fileOfRef[ref]]
}

// countDependencies returns the number of recorded edges: call edges, and
// file-level pairs unexpanded.
func (b *TestDependencyBuilder) countDependencies() int {
	count := 0
	for _, deps := range b.callDeps {
		count += len(deps)
	}
	for _, files := range b.fileDeps {
		count += len(files)
	}
	return count
}

// GetImpactedTests returns tests affected by the given edited refs.
// Implements codedom.TestDependencyAnalyzer.
func (b *TestDependencyBuilder) GetImpactedTests(editedRefs []string) []codedom.ImpactedTestInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()

	editedSet := make(map[string]bool)
	// editedFiles are the files holding an edited element: a test's file-level
	// dependency on one of them is the only way that kind of edge can match.
	editedFiles := make(map[string]bool)
	for _, ref := range editedRefs {
		editedSet[ref] = true
	}
	for file, refs := range b.refsByFile {
		for _, ref := range refs {
			if editedSet[ref] {
				editedFiles[file] = true
				break
			}
		}
	}
	// editedCallees is, per element, the edited elements it calls, in call
	// order: the one-step transitive match. viaFiles are the files holding
	// such an element.
	editedCallees := make(map[string][]string)
	viaFiles := make(map[string]bool)
	for from, callees := range b.callDeps {
		for _, callee := range callees {
			if editedSet[callee] {
				editedCallees[from] = append(editedCallees[from], callee)
			}
		}
	}
	for file, refs := range b.refsByFile {
		for _, ref := range refs {
			if len(editedCallees[ref]) > 0 {
				viaFiles[file] = true
				break
			}
		}
	}

	var impacted []codedom.ImpactedTestInfo

	for testRef := range b.testFuncs {
		var matchedEdits []string
		priority := "low"

		// Direct dependencies, call edges first, each edited element once.
		matched := make(map[string]bool)
		for _, dep := range b.callDeps[testRef] {
			if editedSet[dep] && !matched[dep] {
				matched[dep] = true
				matchedEdits = append(matchedEdits, dep)
				priority = "high"
			}
		}
		for _, sourceFile := range b.fileDepsOf(testRef) {
			if !editedFiles[sourceFile] {
				continue
			}
			for dep := range b.fileLevelRefs(testRef, sourceFile) {
				if editedSet[dep] && !matched[dep] {
					matched[dep] = true
					matchedEdits = append(matchedEdits, dep)
					priority = "high"
				}
			}
		}

		// One step further when nothing matched directly: a dependency that
		// calls an edited element. Each dependency is consulted once.
		if len(matchedEdits) == 0 && len(editedCallees) > 0 {
			consulted := make(map[string]bool)
			via := func(dep string) {
				if consulted[dep] {
					return
				}
				consulted[dep] = true
				if hits := editedCallees[dep]; len(hits) > 0 {
					matchedEdits = append(matchedEdits, hits...)
					priority = "medium"
				}
			}
			for _, dep := range b.callDeps[testRef] {
				via(dep)
			}
			for _, sourceFile := range b.fileDepsOf(testRef) {
				if !viaFiles[sourceFile] {
					continue
				}
				for dep := range b.fileLevelRefs(testRef, sourceFile) {
					via(dep)
				}
			}
		}

		if len(matchedEdits) > 0 {
			impacted = append(impacted, codedom.ImpactedTestInfo{
				TestRef:    testRef,
				TestFile:   b.fileOfRef[testRef],
				Priority:   priority,
				Reason:     "depends_on_edited_code",
				EditedRefs: matchedEdits,
			})
		}
	}

	return impacted
}

// GetImpactedTestPackages returns Go packages containing impacted tests.
// Implements codedom.TestDependencyAnalyzer.
func (b *TestDependencyBuilder) GetImpactedTestPackages(editedRefs []string) []string {
	impacted := b.GetImpactedTests(editedRefs)

	pkgSet := make(map[string]bool)
	for _, test := range impacted {
		if filepath.Ext(test.TestFile) == ".go" {
			pkg := filepath.Dir(test.TestFile)
			pkgSet[pkg] = true
		}
	}

	packages := make([]string, 0, len(pkgSet))
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

	// Covered: the target of a call edge, or an element of a file some test
	// function depends on at file level.
	covered := make(map[string]bool)
	for _, deps := range b.callDeps {
		for _, dep := range deps {
			covered[dep] = true
		}
	}
	for testFile, sourceFiles := range b.fileDeps {
		tests := b.testRefsByFile[testFile]
		if len(tests) == 0 {
			continue
		}
		for _, sourceFile := range sourceFiles {
			for _, ref := range b.refsByFile[sourceFile] {
				if b.testFuncs[ref] {
					continue
				}
				covered[ref] = true
			}
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

	files := make([]string, 0, len(b.testFiles))
	for f := range b.testFiles {
		files = append(files, f)
	}
	return files
}

// GetTestFunctions returns all identified test function refs.
func (b *TestDependencyBuilder) GetTestFunctions() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	refs := make([]string, 0, len(b.testFuncs))
	for r := range b.testFuncs {
		refs = append(refs, r)
	}
	return refs
}
