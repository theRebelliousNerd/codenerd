package core

import (
	"slices"
	"testing"

	"codenerd/internal/types"
)

// External audit N01 (2026-09-19): turn_verified demanded a green build and
// test gate for every write, and the executor runs those gates only for Go, so
// a turn that wrote a document, a policy file or another language's code had no
// affirmative completion path. What a write owes is now read from its
// extension: Go owes the executor's gates, documentation owes nothing
// mechanical, anything else owes a test run that passed after the last write.
func TestCorpus_AWriteOwesTheEvidenceItsClassOwes(t *testing.T) {
	passing, failing := types.MangleAtom("/passing"), types.MangleAtom("/failing")
	type file struct{ path, ext string }
	type gate struct {
		name    string
		verdict types.MangleAtom
	}
	for _, tc := range []struct {
		name        string
		written     []file
		underDocs   []string // written paths under a path nerd.md declares as docs
		gates       []gate
		wantDone    bool
		wantMissing []string
	}{
		{name: "a document owes no gate", written: []file{{"Docs/guide.md", ".md"}}, wantDone: true},
		{name: "a document with a gate that failed elsewhere still owes none", written: []file{{"README.rst", ".rst"}},
			gates: []gate{{"/test", failing}}, wantDone: true},
		{name: "Go owes the build and the tests", written: []file{{"internal/a/a.go", ".go"}},
			wantMissing: []string{"/build_not_green", "/tests_not_green"}},
		{name: "Go with both gates green is done", written: []file{{"internal/a/a.go", ".go"}},
			gates: []gate{{"/build", passing}, {"/test", passing}}, wantDone: true},
		{name: "a policy file owes a test run", written: []file{{"internal/core/defaults/policy/x.mg", ".mg"}},
			wantMissing: []string{"/test_run_not_green"}},
		{name: "a policy file with a passing test run after it is done", written: []file{{"internal/core/defaults/policy/x.mg", ".mg"}},
			gates: []gate{{"/test_run", passing}}, wantDone: true},
		{name: "a failed test run is named", written: []file{{"app/main.py", ".py"}},
			gates: []gate{{"/test_run", failing}}, wantMissing: []string{"/test_run_not_green"}},
		{name: "a golden .txt file is data, not documentation", written: []file{{"internal/a/testdata/golden.txt", ".txt"}},
			wantMissing: []string{"/test_run_not_green"}},
		{name: "a file with no extension is not documentation", written: []file{{"Makefile", ""}},
			wantMissing: []string{"/test_run_not_green"}},
		{name: "a document beside code owes what the code owes", written: []file{{"Docs/guide.md", ".md"}, {"app/main.py", ".py"}},
			wantMissing: []string{"/test_run_not_green"}},
		{name: "a data file under a docs path is documentation", written: []file{{"Docs/architecture/features/corpus.toml", ".toml"}},
			underDocs: []string{"Docs/architecture/features/corpus.toml"}, wantDone: true},
		{name: "the same file outside a docs path owes a test run", written: []file{{"config/corpus.toml", ".toml"}},
			wantMissing: []string{"/test_run_not_green"}},
		{name: "Go under a docs path still owes the Go gates", written: []file{{"Docs/examples/main.go", ".go"}},
			underDocs: []string{"Docs/examples/main.go"}, wantMissing: []string{"/build_not_green", "/tests_not_green"}},
		{name: "a docs-path data file beside code owes what the code owes", written: []file{{"Docs/corpus.toml", ".toml"}, {"app/main.py", ".py"}},
			underDocs: []string{"Docs/corpus.toml"}, wantMissing: []string{"/test_run_not_green"}},
		{name: "a green test run does not stand in for the Go gates", written: []file{{"internal/a/a.go", ".go"}},
			gates: []gate{{"/test_run", passing}}, wantMissing: []string{"/build_not_green", "/tests_not_green"}},
		{name: "a gate that recorded both verdicts is red", written: []file{{"app/main.py", ".py"}},
			gates: []gate{{"/test_run", passing}, {"/test_run", failing}}, wantMissing: []string{"/test_run_not_green"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, err := NewRealKernel()
			if err != nil {
				t.Fatalf("the shipped corpus must load: %v", err)
			}
			turn := types.MangleAtom("/turn_write_class")
			facts := []types.Fact{{Predicate: "turn_evidence", Args: []any{
				turn, types.MangleAtom("/fix"), 2, len(tc.written), 0, types.MangleAtom("/false"), types.MangleAtom("/false"),
			}}}
			for _, f := range tc.written {
				facts = append(facts, types.Fact{Predicate: "turn_written", Args: []any{turn, types.MangleString(f.path), types.MangleString(f.ext)}})
			}
			for _, p := range tc.underDocs {
				facts = append(facts, types.Fact{Predicate: "turn_doc_write", Args: []any{turn, types.MangleString(p)}})
			}
			for _, g := range tc.gates {
				facts = append(facts, types.Fact{Predicate: "turn_gate", Args: []any{turn, types.MangleAtom(g.name), g.verdict}})
			}
			for _, f := range facts {
				if aerr := k.Assert(f); aerr != nil {
					t.Fatalf("assert %s%v: %v", f.Predicate, f.Args, aerr)
				}
			}
			assertTurnVerdict(t, k, tc.wantDone, tc.wantMissing)
		})
	}
}

// The dream-mode arm: a write-oriented turn with no recorded write has no
// extension to read, so it owes both Go gates, as every write did before.
func TestCorpus_AWriteOrientedTurnWithNoRecordedWriteOwesTheGoGates(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	turn := types.MangleAtom("/turn_dream_write")
	if aerr := k.Assert(types.Fact{Predicate: "turn_evidence", Args: []any{
		turn, types.MangleAtom("/create"), 1, 0, 0, types.MangleAtom("/false"), types.MangleAtom("/true"),
	}}); aerr != nil {
		t.Fatalf("assert turn_evidence: %v", aerr)
	}
	assertTurnVerdict(t, k, false, []string{"/build_not_green", "/tests_not_green"})
}

func assertTurnVerdict(t *testing.T, k *RealKernel, wantDone bool, wantMissing []string) {
	t.Helper()
	done, err := k.Query("turn_done")
	if err != nil {
		t.Fatalf("query turn_done: %v", err)
	}
	if got := len(done) == 1; got != wantDone {
		t.Fatalf("turn_done = %t, want %t", got, wantDone)
	}
	rows, err := k.Query("turn_missing_evidence")
	if err != nil {
		t.Fatalf("query turn_missing_evidence: %v", err)
	}
	var missing []string
	for _, f := range rows {
		missing = append(missing, types.ExtractString(f.Args[1]))
	}
	slices.Sort(missing)
	missing = slices.Compact(missing)
	if !slices.Equal(missing, wantMissing) {
		t.Fatalf("turn_missing_evidence = %v, want %v", missing, wantMissing)
	}
}
