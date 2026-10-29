package main

import (
	"testing"

	"codenerd/internal/prompt"
)

// maxUnmarkedFailingExamples is a ratchet on the Mangle the prompt corpus
// teaches codeNERD's own agents. Measured 2026-09-18 on the pinned engine
// (every block loaded through a fresh engine, as `nerd check-mangle` does):
// 2,144 blocks, 284 loading, 888 failing without being marked as a wrong-way
// example. The corpus taught the Google 0.4.0 typed-variable declaration form
// (Decl p(X.Type<int>)) 2,295 times and the reducers capitalized (fn:Sum,
// fn:Count, ...) about 600 times; the pinned engine parses neither. After the
// rewrite: 2,102 blocks, 570 loading, 654 failing unmarked -- most of them
// examples whose facts or declarations live elsewhere in the atom, a query
// written as a clause, or a top-level `=`. 2026-09-19: the engine named the
// predicates each failing example left undeclared and those Decls were added
// (296 examples), `not p(X)` became `!p(X)` where that made the example load
// (71), and negations of an atom holding a wildcard -- which the engine
// deletes from the clause without a word -- became negations of a projection
// (`has_p(X) :- p(X, _).` then `!has_p(X)`, 35): 972 loading, 344 failing
// unmarked. 2026-10-28: capability/codedom_query_tools taught `:- goal.` as a
// query (seven blocks; a query is one atom, `? goal.`, and a conjunction needs
// a rule), and the two permitted derivations left their Decls out: 338. The
// number may only go down: lower it in the change that fixes
// more of them.
const maxUnmarkedFailingExamples = 338

func TestAtomCorpus_MangleExamplesTheEngineRejectsDoNotGrow(t *testing.T) {
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	// The measurement is prompt.CountMangleExamples, the same one
	// `nerd gates measure` reports as gate G8.
	count, err := prompt.CountMangleExamples(corpus.All())
	if err != nil {
		t.Fatalf("CountMangleExamples: %v", err)
	}
	failing := len(count.Failing)
	t.Logf("mangle examples: %d blocks, %d load on the pinned engine, %d fail unmarked (ceiling %d)", count.Total, count.Loading, failing, maxUnmarkedFailingExamples)
	if failing > maxUnmarkedFailingExamples {
		t.Errorf("%d Mangle examples in the prompt corpus fail to load on the pinned engine without being marked as wrong-way examples; the ceiling is %d. Fix the example (run it through `nerd check-mangle`) or mark it as the wrong way.", failing, maxUnmarkedFailingExamples)
	}
}
