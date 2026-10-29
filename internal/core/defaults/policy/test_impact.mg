# Test Impact Analysis Rules: Smart Test Selection
# Version: 1.0.0
# Philosophy: Run only tests affected by code changes
#
# This file implements test impact analysis using the dependency graph
# to determine which tests need to run when code is modified.
#
# NOTE: Test file and function identification is done in Go code
# (internal/world/test_dependency.go) because Mangle doesn't have
# string matching functions (fn:match, fn:basename, fn:dirname, etc.).
#
# NOTE (2026-09-28, W1; updated 2026-09-29, W3): the inventory below was
# verified 2026-09-11 and parts of it have since been wired. It is kept as
# the record of what was starved, with the current state of each entry.
#
#   is_test_function   WAS: no producer. NOW: CodeElement.ToFacts
#                      (internal/world/code_elements.go) asserts it for every
#                      test entrypoint -- Test*/Benchmark*/Fuzz*/Example* in a
#                      _test.go file (and the py/ts/rs equivalents) -- with
#                      the element's own ref, the exact spelling these rules
#                      join on. It rides the ScopeFacts/FileFacts batch, so
#                      scope replacement retracts it with code_element.
#
#   code_calls         The bare <pkg>.<Name> rows these rules join are now
#                      accompanied by dual fn:-prefixed rows from the Go
#                      Cartographer (internal/world/cartographer.go), because
#                      the call rule and the transitive call rule bind the
#                      same variable code_element does. The bare rows stay:
#                      impact.mg joins them against modified_function, and
#                      that contract is documented, not legacy. Non-Go
#                      mappers emit bare rows only (their refs key on file
#                      path + parentage the walker cannot reconstruct).
#
#   file_imports       WAS: no producer. NOW: the import resolver
#                      (internal/world/dependency_links.go) emits a
#                      file_imports row beside every resolved in-workspace
#                      dependency_link edge, on the full and incremental scan
#                      paths alike. Test Go files contribute their imports via
#                      a header-only parse (test_impact_facts.go), since the
#                      fast scans skip them in the tree-sitter walker.
#
#   directory join     The same-directory rules join file_dir
#                      (schemas_world.mg; emitted beside every file_topology
#                      row in fs.go and incremental_scan.go). canonicalDir
#                      spells that directory the same way for every file in
#                      it, so an external test file (package p_test) meets
#                      its sources. The pair relation is not stored: one
#                      directory of 274 Go files is 74,802 ordered pairs, and
#                      the repo total (302,284) exceeds the 250k EDB ceiling.
#
#   file_package       WAS: no producer. NOW: the tree-sitter Go walker's
#                      package_clause branch emits the row for non-test Go
#                      files and the header-only parse (test_impact_facts.go)
#                      for test Go files, both spelling (canonical path,
#                      package-clause name).
#
#   is_test_file       WAS: no producer. NOW: the fast scanner marks every
#                      file its isTestFile classification flags, all languages
#                      (internal/world/fs.go, incremental_scan.go). The gap
#                      rule no longer accuses the tests in a scanned workspace.
#
#   element_modified   The edit trigger. A CodeDOM edit asserts
#                      element_modified(Ref, SessionID, Timestamp) with the
#                      code_element ref these rules join
#                      (internal/core/virtual_store_codedom.go, and
#                      editedElementFacts in virtual_store_tool_facts.go).
#                      plan_edit was removed: nothing produced it. The
#                      transaction manager emits modified_file, a file path
#                      (see below), not an element ref.
#
#   modified_file      Its only Go producer is TransactionManager.ToFacts()
#                      (internal/core/transaction_manager.go), and nothing in
#                      the repository calls it -- VirtualStore.
#                      GetTransactionManager() has no callers at all, and no
#                      production code calls Begin or AddEdit, so no
#                      transaction is ever opened and ToFacts returns empty.
#
# What fires today, given the facts: every rule whose inputs exist --
# test_depends_on through calls (R2), same-directory (R3) and file imports
# (R1), its transitive closure, impacted_test and the priorities through
# element_modified, coverage and all three priorities -- proven by
# internal/world/test_impact_chain_test.go against the real corpus. The
# edit row in that test is hand-asserted in the producer's shape; the test
# does not dispatch a CodeDOM edit. method_of needs no shard attention: it
# is derived in-world from element_parent (codedom_core.mg), so the
# transitive method rule fires on the production kernel; the sharded-vs-single
# parity test guards the colocation. The modified_file impacted_test rule
# still waits on a producer.
#
# run_impacted_tests reads the same element_modified rows and walks the
# dependency graph in Go.

# =============================================================================
# SECTION 1: TEST IDENTIFICATION PREDICATES
# =============================================================================
# is_test_function is asserted by CodeElement.ToFacts
# (internal/world/code_elements.go), which runs wherever code_element facts
# are produced. It is declared here because it is declared nowhere else;
# the companion test-file predicates live in schemas_shards.mg
# (is_test_file, file_package); both are produced by the fast scanner
# (see above). Same-directory grouping joins file_dir (schemas_world.mg).

Decl is_test_function(Ref).


# =============================================================================
# SECTION 2: DIRECT TEST DEPENDENCIES
# =============================================================================
# Build the direct dependency graph between tests and source code.

# Test depends on source if test imports the source file
# Reordered: bind TestFile from TestRef, then file_imports joins to SourceFile,
# then code_element binds SourceRef sharing SourceFile.
test_depends_on(TestRef, SourceRef) :-
    is_test_function(TestRef),
    code_element(TestRef, _, TestFile, _, _),
    file_imports(TestFile, SourceFile),
    code_element(SourceRef, _, SourceFile, _, _).

# Test depends on source if test calls source function
test_depends_on(TestRef, SourceRef) :-
    is_test_function(TestRef),
    code_calls(TestRef, SourceRef).

# Test depends on source when they share a directory and the test references the source symbol.
# TestFile is bound above; file_dir binds the directory, then the other file in it.
# Premise order is load-bearing: the engine evaluates left to right and counts
# the intermediate solution set against the fact limit.
test_depends_on(TestRef, SourceRef) :-
    is_test_function(TestRef),
    code_element(TestRef, _, TestFile, _, _),
    file_dir(TestFile, Dir),
    file_dir(SourceFile, Dir),
    TestFile != SourceFile,
    code_element(SourceRef, _, SourceFile, _, _),
    test_references_symbol(TestRef, SourceRef).


# =============================================================================
# SECTION 3: TRANSITIVE DEPENDENCIES
# =============================================================================
# Compute transitive closure of test dependencies.

# Direct dependency is transitive
test_depends_on_transitive(TestRef, SourceRef) :-
    test_depends_on(TestRef, SourceRef).

# Transitive through call chain
test_depends_on_transitive(TestRef, SourceRef) :-
    test_depends_on(TestRef, MidRef),
    code_calls(MidRef, SourceRef).

# Transitive through method_of relationship
test_depends_on_transitive(TestRef, SourceRef) :-
    test_depends_on_transitive(TestRef, MethodRef),
    method_of(MethodRef, SourceRef).

# Transitive through struct embedding
test_depends_on_transitive(TestRef, SourceRef) :-
    test_depends_on_transitive(TestRef, TypeRef),
    type_embeds(TypeRef, SourceRef).


# =============================================================================
# SECTION 4: IMPACTED TEST DETECTION
# =============================================================================
# Determine which tests are affected by edited elements.

# A test is impacted if we're editing something it depends on
impacted_test(TestRef) :-
    element_modified(TargetRef, _, _),
    test_depends_on_transitive(TestRef, TargetRef).

# A test is impacted if it's in the same file as something we're editing
impacted_test(TestRef) :-
    element_modified(TargetRef, _, _),
    code_element(TargetRef, _, File, _, _),
    code_element(TestRef, _, File, _, _),
    is_test_function(TestRef).

# A test is impacted if it depends on a modified file (file-level granularity fallback)
# Reordered: bind TestRef first, get TestFile, then file_imports joins to File,
# then modified_file confirms File is modified.
impacted_test(TestRef) :-
    is_test_function(TestRef),
    code_element(TestRef, _, TestFile, _, _),
    file_imports(TestFile, File),
    modified_file(File).


# =============================================================================
# SECTION 5: PACKAGE-LEVEL TEST SELECTION
# =============================================================================
# For languages that run tests at package level (Go).

# Get the package containing a test function
# NOTE: Named test_func_package to avoid conflict with test_package/1 in tester.mg
test_func_package(TestRef, Pkg) :-
    is_test_function(TestRef),
    code_element(TestRef, _, File, _, _),
    file_package(File, Pkg).

# A package has impacted tests if any test in it is impacted
impacted_test_package(Pkg) :-
    impacted_test(TestRef),
    test_func_package(TestRef, Pkg).


# =============================================================================
# SECTION 6: TEST COVERAGE GAPS
# =============================================================================
# Identify code that lacks test coverage.

# A public function has test coverage if any test depends on it
has_test_coverage(Ref) :-
    code_element(Ref, /function, _, _, _),
    test_depends_on_transitive(_, Ref).

# A method has coverage through its receiver
has_test_coverage(Ref) :-
    method_of(Ref, TypeRef),
    has_test_coverage(TypeRef).

# Coverage gap: Public function without tests
coverage_gap(Ref, /no_direct_tests) :-
    code_element(Ref, /function, File, _, _),
    element_visibility(Ref, /public),
    !is_test_file(File),
    !has_test_coverage(Ref).


# =============================================================================
# SECTION 7: TEST PRIORITY SCORING
# =============================================================================
# Score tests for execution priority.

# High priority detection (helper predicate to avoid stratification)
# Reordered: impacted_test binds TestRef, test_depends_on shares TestRef and binds
# TargetRef, then element_modified shares TargetRef.
is_high_priority_test(TestRef) :-
    impacted_test(TestRef),
    test_depends_on(TestRef, TargetRef),
    element_modified(TargetRef, _, _).

# High priority: Test directly tests the edited function
test_priority(TestRef, /high) :-
    is_high_priority_test(TestRef).

# Medium priority: Test indirectly depends on edited function (impacted but not high)
test_priority(TestRef, /medium) :-
    impacted_test(TestRef),
    !is_high_priority_test(TestRef).

# Low priority: a test that shares the edited file's directory but is not impacted.
# TestFile is bound above; file_dir binds the directory, then the other file in it.
is_low_priority_test(TestRef) :-
    is_test_function(TestRef),
    !impacted_test(TestRef),
    code_element(TestRef, _, TestFile, _, _),
    file_dir(TestFile, Dir),
    file_dir(TargetFile, Dir),
    TestFile != TargetFile,
    code_element(TargetRef, _, TargetFile, _, _),
    element_modified(TargetRef, _, _).

# Low priority: test in the same directory but no dependency
test_priority(TestRef, /low) :-
    is_low_priority_test(TestRef).


# =============================================================================
# SECTION 8: AGGREGATION HELPERS
# =============================================================================
# Aggregate test results for reporting.

# Get all impacted test files
impacted_test_file(File) :-
    impacted_test(TestRef),
    code_element(TestRef, _, File, _, _).


# =============================================================================
# SECTION 9: HELPER PREDICATES
# =============================================================================
# Supporting predicates for test analysis.

# Test references a symbol (simplified - could be enhanced with AST analysis)
test_references_symbol(TestRef, SourceRef) :-
    code_calls(TestRef, SourceRef).

