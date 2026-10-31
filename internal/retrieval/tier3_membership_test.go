package retrieval

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

func tier3MemberWorkspace(t *testing.T, language string) (root, seed string, want, excluded []string) {
	t.Helper()
	root = t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	files := map[string]string{}
	switch language {
	case "go":
		seed = "entry.go"
		files["go.mod"] = "module example.test/work\n"
		files[seed] = "package entry\nimport (\n_ \"example.test/work/peer\"\n_ \"example.test/work/blocked\"\n_ \"example.test/work/.nerd\"\n)\n"
		for _, name := range []string{"a_excluded.go", "b_excluded.go", "c_excluded.go"} {
			p := "peer/" + name
			files[p] = "package peer\n"
			excluded = append(excluded, p)
		}
		for _, name := range []string{"z1.go", "z2.go", "z3.go", "z4.go"} {
			p := "peer/" + name
			files[p] = "package peer\n// A larger admitted declaration file.\n"
			if name != "z4.go" {
				want = append(want, p)
			}
		}
		files["blocked/other.go"] = "package blocked\n"
		files[".nerd/other.go"] = "package local\n"
		excluded = append(excluded, "blocked/other.go", ".nerd/other.go")
	case "python":
		seed = "entry.py"
		files[seed] = "import hidden\nimport admitted\nimport blocked.part\n"
		files["hidden.py"], files["admitted.py"], files["blocked/part.py"] = "VALUE = 1\n", "VALUE = 2\n", "VALUE = 3\n"
		want, excluded = []string{"admitted.py"}, []string{"hidden.py", "blocked/part.py"}
	case "typescript":
		seed = "entry.ts"
		files[seed] = "import './hidden';\nimport './admitted';\nimport './blocked/part';\nimport './.nerd/local';\n"
		for _, p := range []string{"hidden.ts", "admitted.ts", "blocked/part.ts", ".nerd/local.ts"} {
			files[p] = "export const value = 1;\n"
		}
		want, excluded = []string{"admitted.ts"}, []string{"hidden.ts", "blocked/part.ts", ".nerd/local.ts"}
	case "rust":
		seed = "src/lib.rs"
		files[seed] = "mod hidden;\nmod admitted;\nmod blocked;\n"
		for _, p := range []string{"src/hidden.rs", "src/admitted.rs", "src/blocked.rs"} {
			files[p] = "pub const VALUE: u8 = 1;\n"
		}
		want, excluded = []string{"src/admitted.rs"}, []string{"src/hidden.rs", "src/blocked.rs"}
	default:
		t.Fatal("unsupported fixture language")
	}
	for p, body := range files {
		writeFile(t, root, p, []byte(body))
	}
	patterns := append([]string{}, excluded...)
	patterns = append(patterns, "blocked/**")
	data, err := json.Marshal(map[string]any{"world": map[string]any{"ignore_patterns": patterns}})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, ".nerd/config.json", data)
	return root, seed, want, excluded
}

// Exercise the production task -> seed -> real-kernel facts -> rendered brief
// boundary. The default floor hides score-50 neighbors from the brief, but
// must not hide a non-member that still reached the kernel's tier facts.
func TestTier3Membership_TaskFactsAndBriefRespectMembership(t *testing.T) {
	for _, language := range []string{"go", "python", "typescript", "rust"} {
		for _, floor := range []int{config.DefaultRetrievalConfig().BriefMinRelevance, 50} {
			t.Run(language+"/"+itoa(floor), func(t *testing.T) {
				root, seed, want, excluded := tier3MemberWorkspace(t, language)
				k := newDecisionKernel(t)
				assertIntent(t, k, "/tier3_task", "/mutation", "/fix", seed)
				tr := NewTaskRetriever(k, TaskRetrieverConfig{WorkDir: root, Params: config.RetrievalConfig{BriefMinRelevance: floor}.Params()})
				brief, release := tr.Retrieve(t.Context(), "/tier3_task", "Fix "+seed)
				t.Cleanup(release)
				if !strings.Contains(brief, seed+" (tier1,") {
					t.Fatalf("production pass did not reach its named-file brief: %s", brief)
				}
				rows, err := k.Query("tiered_context_file")
				if err != nil {
					t.Fatal(err)
				}
				var got []string
				for _, row := range rows {
					p := types.ExtractString(row.Args[1])
					for _, denied := range excluded {
						if p == denied {
							t.Errorf("excluded import target reached kernel facts: %s", p)
						}
					}
					if types.ExtractString(row.Args[2]) == "/tier3" {
						got = append(got, p)
						if row.Args[3] != int64(50) {
							t.Errorf("neighbor relevance changed: %v", row)
						}
					}
				}
				sort.Strings(got) // Kernel fact iteration is not selection order.
				if !reflect.DeepEqual(got, want) {
					t.Errorf("Tier 3 facts = %v, want admitted neighbors %v", got, want)
				}
				for _, p := range want {
					if present := strings.Contains(brief, p+" (tier3, 50)"); present != (floor <= 50) {
						t.Errorf("neighbor %s in rendered brief = %v at floor %d: %s", p, present, floor, brief)
					}
				}
				for _, p := range excluded {
					if strings.Contains(brief, p+" (") {
						t.Errorf("excluded target reached rendered brief: %s", brief)
					}
				}
			})
		}
	}
}

func TestTier3Membership_AppliesBeforeBothGoCaps(t *testing.T) {
	root, seed, want, _ := tier3MemberWorkspace(t, "go")
	cfg := DefaultTieredContextConfig(root)
	cfg.Tier1Budget, cfg.Tier2Budget, cfg.Tier3Budget, cfg.Tier4Budget, cfg.MaxTotal = .5, 0, .5, 0, 2
	b := NewTieredContextBuilder(cfg)
	pack, err := b.BuildContext(t.Context(), "Fix "+seed)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Tier1Count != 1 || pack.Tier3Count != 1 || len(pack.Files) != 2 || pack.Files[1].FilePath != filepath.Join(root, want[0]) {
		t.Fatalf("excluded sources spent package or tier capacity: %+v", pack)
	}
}

func TestTier3Membership_CancellationAndZeroBudget(t *testing.T) {
	root, seed, _, _ := tier3MemberWorkspace(t, "python")
	b := NewTieredContextBuilder(DefaultTieredContextConfig(root))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	seeds := []ContextFile{{FilePath: filepath.Join(root, seed)}}
	if got := b.expandImportGraph(ctx, seeds, map[string]bool{}); len(got) != 0 {
		t.Fatalf("canceled expansion returned %v", got)
	}
	b.maxTier3 = 0
	if got := b.expandImportGraph(t.Context(), seeds, map[string]bool{}); len(got) != 0 {
		t.Fatalf("zero budget expansion returned %v", got)
	}
}
