package world

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"codenerd/internal/tools/codedom"
)

// referenceDependencyBuilder is the graph as it was built before file-level
// edges were kept unexpanded: every edge materialized, each expansion a rescan
// of every element. Kept as the oracle for the builder that replaced it.
type referenceDependencyBuilder struct {
	kernel    codedom.KernelQuerier
	testFiles map[string]bool
	testFuncs map[string]bool
	elements  []codedom.FactData
	aliases   map[string]string
	reference map[string][]string
}

// buildDependencyEdges constructs edges from tests to source code.
func (b *referenceDependencyBuilder) buildDependencyEdges(ctx context.Context) error {
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
func (b *referenceDependencyBuilder) addDependency(testRef, sourceRef string) {
	// Don't add self-dependencies or test-to-test dependencies
	if testRef == sourceRef || b.testFuncs[sourceRef] {
		return
	}

	deps := b.reference[testRef]
	// Check for duplicates
	if slices.Contains(deps, sourceRef) {
		return
	}
	b.reference[testRef] = append(deps, sourceRef)
}

// addFileLevelDependency adds dependencies from all tests in testFile to all elements in sourceFile.
func (b *referenceDependencyBuilder) addFileLevelDependency(testFile, sourceFile string) {
	// Query elements in source file
	facts := b.elements

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
func (b *referenceDependencyBuilder) buildPackageDependencies() {
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

// GetImpactedTests returns tests affected by the given edited refs.
// Implements codedom.TestDependencyAnalyzer.
func (b *referenceDependencyBuilder) GetImpactedTests(editedRefs []string) []codedom.ImpactedTestInfo {

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

		deps := b.reference[testRef]
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
				depDeps := b.reference[dep]
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
func (b *referenceDependencyBuilder) getTestFile(testRef string) string {
	facts := b.elements

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

// GetCoverageGaps returns public functions without test coverage.
// Implements codedom.TestDependencyAnalyzer.
func (b *referenceDependencyBuilder) GetCoverageGaps() []string {

	// Build set of covered refs
	covered := make(map[string]bool)
	for _, deps := range b.reference {
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

func dependencyFixture(seed int64, dirs, filesPerDir, funcsPerFile int) *mockKernelQuerier {
	rng := rand.New(rand.NewSource(seed))
	k := newMockKernel()
	var sourceRefs, testRefs, files []string
	for d := range dirs {
		for f := range filesPerDir {
			for _, test := range []bool{false, true} {
				name := fmt.Sprintf("pkg%02d/file%02d.go", d, f)
				if test {
					name = fmt.Sprintf("pkg%02d/file%02d_test.go", d, f)
				}
				files = append(files, name)
				k.facts["file_topology"] = append(k.facts["file_topology"], codedom.FactData{Predicate: "file_topology", Args: []any{name, "h", "/go", int64(1), false}})
				for n := range funcsPerFile {
					ref := fmt.Sprintf("fn:pkg%02d.F%02d_%02d", d, f, n)
					if test {
						ref = fmt.Sprintf("fn:pkg%02d.TestF%02d_%02d", d, f, n)
						testRefs = append(testRefs, ref)
					} else {
						sourceRefs = append(sourceRefs, ref)
						k.facts["element_visibility"] = append(k.facts["element_visibility"], codedom.FactData{Predicate: "element_visibility", Args: []any{ref, "/public"}})
					}
					k.facts["code_element"] = append(k.facts["code_element"], codedom.FactData{Predicate: "code_element", Args: []any{ref, "/function", name, int64(n * 10), int64(n*10 + 5)}})
				}
			}
		}
	}
	pick := func(from []string) string { return from[rng.Intn(len(from))] }
	for range len(sourceRefs) * 2 {
		// source calls source, test calls source, and a few repeats and self-calls
		k.facts["code_calls"] = append(k.facts["code_calls"], codedom.FactData{Predicate: "code_calls", Args: []any{pick(sourceRefs), pick(sourceRefs)}})
		k.facts["code_calls"] = append(k.facts["code_calls"], codedom.FactData{Predicate: "code_calls", Args: []any{pick(testRefs), pick(sourceRefs)}})
	}
	k.facts["code_calls"] = append(k.facts["code_calls"], k.facts["code_calls"][:5]...)
	k.facts["code_calls"] = append(k.facts["code_calls"], codedom.FactData{Predicate: "code_calls", Args: []any{sourceRefs[0], sourceRefs[0]}})
	for range len(files) {
		// The oracle reads the pair; the builder reads the same edge in
		// factors, each imported file its own unit.
		from, to := pick(files), pick(files)
		k.facts["file_imports"] = append(k.facts["file_imports"], codedom.FactData{Predicate: "file_imports", Args: []any{from, to}})
		k.facts["import_unit"] = append(k.facts["import_unit"], codedom.FactData{Predicate: "import_unit", Args: []any{from, "unit:" + to}})
		k.facts["file_unit"] = append(k.facts["file_unit"], codedom.FactData{Predicate: "file_unit", Args: []any{to, "unit:" + to}})
	}
	k.facts["file_imports"] = append(k.facts["file_imports"], k.facts["file_imports"][:3]...)
	k.facts["import_unit"] = append(k.facts["import_unit"], k.facts["import_unit"][:3]...)
	return k
}

type impactedKey struct{ ref, file, priority, edits string }

func impactedSet(infos []codedom.ImpactedTestInfo) []impactedKey {
	out := make([]impactedKey, 0, len(infos))
	for _, in := range infos {
		edits := slices.Clone(in.EditedRefs)
		sort.Strings(edits)
		out = append(out, impactedKey{in.TestRef, in.TestFile, in.Priority, fmt.Sprint(edits)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ref < out[j].ref })
	return out
}

// The builder that keeps file-level edges unexpanded answers every question
// as the builder that materialized every edge did: the same impacted tests
// with the same priority and the same edited elements, and the same gaps.
func TestTestDependencyBuilder_MatchesTheMaterializedGraph(t *testing.T) {
	for _, seed := range []int64{1, 2, 3} {
		kernel := dependencyFixture(seed, 4, 5, 4)
		b := NewTestDependencyBuilder(kernel, "/ws")
		if err := b.Build(context.Background()); err != nil {
			t.Fatal(err)
		}
		ref := &referenceDependencyBuilder{
			kernel: kernel, testFiles: b.testFiles, testFuncs: b.testFuncs,
			elements: b.elements, aliases: b.aliases, reference: make(map[string][]string),
		}
		if err := ref.buildDependencyEdges(context.Background()); err != nil {
			t.Fatal(err)
		}
		if len(b.testFuncs) == 0 || len(ref.reference) == 0 {
			t.Fatalf("seed %d: the fixture built nothing", seed)
		}

		rng := rand.New(rand.NewSource(seed))
		var all []string
		for _, f := range b.elements {
			all = append(all, f.Args[0].(string))
		}
		sort.Strings(all)
		edits := [][]string{{all[0]}, {all[len(all)-1]}, {"fn:no.such"}, nil}
		for range 12 {
			var e []string
			for range 1 + rng.Intn(3) {
				e = append(e, all[rng.Intn(len(all))])
			}
			edits = append(edits, e)
		}
		for _, e := range edits {
			got, want := impactedSet(b.GetImpactedTests(e)), impactedSet(ref.GetImpactedTests(e))
			if !slices.Equal(got, want) {
				t.Fatalf("seed %d, edit %v: %d impacted tests, the materialized graph gives %d\n got %v\nwant %v", seed, e, len(got), len(want), got, want)
			}
		}
		gotGaps, wantGaps := b.GetCoverageGaps(), ref.GetCoverageGaps()
		sort.Strings(gotGaps)
		sort.Strings(wantGaps)
		if !slices.Equal(gotGaps, wantGaps) {
			t.Fatalf("seed %d: coverage gaps differ: %d vs %d", seed, len(gotGaps), len(wantGaps))
		}
	}
}

// A package with many test files and many source files builds without
// expanding the product of their symbols.
func TestTestDependencyBuilder_KeepsFileLevelEdgesUnexpanded(t *testing.T) {
	kernel := dependencyFixture(7, 2, 60, 10)
	b := NewTestDependencyBuilder(kernel, "/ws")
	if err := b.Build(context.Background()); err != nil {
		t.Fatal(err)
	}
	// 2 dirs x 60 test files x 60 source files, each pair once, plus imports:
	// far below the 2 x (600 x 600) symbol pairs materializing would store.
	if edges := b.countDependencies(); edges > 20000 {
		t.Fatalf("%d edges recorded: file-level edges are being expanded", edges)
	}
	impacted := b.GetImpactedTests([]string{"fn:pkg00.F00_00"})
	samePackage := 0
	for _, in := range impacted {
		if filepath.Dir(in.TestFile) == "pkg00" && in.Priority == "high" {
			samePackage++
		}
	}
	if samePackage != 600 {
		t.Fatalf("an edit to a package's element impacts its %d test functions directly, got %d", 600, samePackage)
	}
}
