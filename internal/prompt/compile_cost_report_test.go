package prompt

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// compileCostTurn is one representative turn: a role, a verb, a target file
// and its language, compiled the way the session executor compiles it.
type compileCostTurn struct {
	Name     string `json:"name"`
	Shard    string `json:"shard"`
	Verb     string `json:"verb"`
	Target   string `json:"target,omitempty"`
	Language string `json:"language,omitempty"`
}

// compileCostTurns span the roles and languages the corpus serves most.
var compileCostTurns = []compileCostTurn{
	{"coder-go-fix", "/coder", "/fix", "internal/session/executor.go", "/go"},
	{"coder-go-create", "/coder", "/create", "internal/session/work_steps.go", "/go"},
	{"coder-python-fix", "/coder", "/fix", "scripts/report.py", "/python"},
	{"coder-typescript-refactor", "/coder", "/refactor", "web/src/app.ts", "/typescript"},
	{"coder-rust-fix", "/coder", "/fix", "src/lib.rs", "/rust"},
	{"coder-mangle-fix", "/coder", "/fix", "internal/core/defaults/policy/jit_needs.mg", "/mangle"},
	{"coder-markdown-create", "/coder", "/create", "Docs/notes.md", "/markdown"},
	{"tester-go-test", "/tester", "/test", "internal/session/executor_test.go", "/go"},
	{"tester-python-test", "/tester", "/test", "tests/test_report.py", "/python"},
	{"reviewer-go-review", "/reviewer", "/review", "internal/session/executor.go", "/go"},
	{"reviewer-java-review", "/reviewer", "/review", "src/main/java/App.java", "/java"},
	{"researcher-research", "/researcher", "/research", "", ""},
}

type compileCostRow struct {
	compileCostTurn
	TotalTokens int            `json:"total_tokens"`
	Atoms       int            `json:"atoms"`
	AtomTokens  map[string]int `json:"atom_tokens"`
}

// TestCompileCostReport writes the token cost of every representative turn's
// compile to the path in CODENERD_COMPILE_COST_REPORT: every token here is
// paid on every such call. The compile is the executor's: the verb's derived
// tool envelope, the target's kernel-derived needs, the serving vendor.
// CODENERD_COMPILE_COST_ATOMS names a directory of atom YAML to compile
// instead of the embedded corpus (an earlier revision, for a before/after).
// Rule selection only: vector recall and Jev's per-turn relevance need live
// services and are measured by dogfood turns.
func TestCompileCostReport(t *testing.T) {
	out := os.Getenv("CODENERD_COMPILE_COST_REPORT")
	if out == "" {
		t.Skip("set CODENERD_COMPILE_COST_REPORT=<path> to write the compile cost report")
	}
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	corpus := compileCostCorpus(t, os.Getenv("CODENERD_COMPILE_COST_ATOMS"))
	compiler, err := NewJITPromptCompiler(WithKernel(&realKernelQuerier{k: k}), WithEmbeddedCorpus(corpus))
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })

	rows := make([]compileCostRow, 0, len(compileCostTurns))
	total := 0
	for _, turn := range compileCostTurns {
		cc := coderTurnContext(t, turn.Verb, turn.Target, turn.Language, kernelTargetNeeds(t, k, turn.Language))
		cc.ShardType = turn.Shard
		cc.ShardID = strings.TrimPrefix(turn.Shard, "/")
		res, err := compiler.Compile(context.Background(), cc)
		if err != nil {
			t.Fatalf("%s: Compile: %v", turn.Name, err)
		}
		row := compileCostRow{compileCostTurn: turn, TotalTokens: res.TotalTokens, Atoms: len(res.IncludedAtoms), AtomTokens: map[string]int{}}
		for _, a := range res.IncludedAtoms {
			row.AtomTokens[a.ID] = a.TokenCount
		}
		total += res.TotalTokens
		rows = append(rows, row)
		t.Logf("%-28s %6d tokens  %3d atoms", turn.Name, res.TotalTokens, len(res.IncludedAtoms))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	report := struct {
		CorpusAtoms  int              `json:"corpus_atoms"`
		CorpusTokens int              `json:"corpus_tokens"`
		MeanTokens   int              `json:"mean_tokens"`
		Turns        []compileCostRow `json:"turns"`
	}{CorpusAtoms: len(corpus.All()), MeanTokens: total / len(rows), Turns: rows}
	for _, a := range corpus.All() {
		report.CorpusTokens += a.TokenCount
	}
	data, err := json.MarshalIndent(report, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("corpus %d atoms / %d tokens, mean %d tokens per compile", report.CorpusAtoms, report.CorpusTokens, report.MeanTokens)
}

// compileCostCorpus is the embedded corpus, or the atoms under dir parsed the
// way the embedded loader parses them, with the shipped importance ledger
// applied to both.
func compileCostCorpus(t *testing.T, dir string) *EmbeddedCorpus {
	t.Helper()
	if dir == "" {
		corpus, err := LoadEmbeddedCorpus()
		if err != nil {
			t.Fatalf("LoadEmbeddedCorpus: %v", err)
		}
		return corpus
	}
	fsys := os.DirFS(dir)
	var atoms []*PromptAtom
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if ext := strings.ToLower(filepath.Ext(p)); ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		parsed, _, err := ParsePromptAtomYAML(data, p, func(source, contentFile string) ([]byte, error) {
			return fs.ReadFile(fsys, pathpkgJoin(source, contentFile))
		})
		if err != nil {
			return err
		}
		for _, r := range parsed {
			atoms = append(atoms, r.Atom)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("load atoms from %s: %v", dir, err)
	}
	ledger, err := EmbeddedImportanceLedger()
	if err != nil {
		t.Fatal(err)
	}
	applyImportance(atoms, ledger)
	return NewEmbeddedCorpus(atoms)
}
