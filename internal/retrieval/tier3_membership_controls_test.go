package retrieval

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

func TestTier3Membership_GitIgnoreAndExplicitTrackedExclusions(t *testing.T) {
	root, seed, _, _ := tier3MemberWorkspace(t, "go")
	for _, args := range [][]string{{"init", "--quiet", root}, {"-C", root, "add", "-f", "peer/a_excluded.go"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("local Git fixture: %v / %s", err, out)
		}
	}
	writeFile(t, root, ".gitignore", []byte("peer/z1.go\n"))
	k := newDecisionKernel(t)
	assertIntent(t, k, "/git_member_task", "/mutation", "/fix", seed)
	tr := NewTaskRetriever(k, TaskRetrieverConfig{WorkDir: root, Params: config.RetrievalConfig{BriefMinRelevance: 50}.Params()})
	brief, release := tr.Retrieve(t.Context(), "/git_member_task", "Fix "+seed)
	t.Cleanup(release)
	rows, err := k.Query("tiered_context_file")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, row := range rows {
		if types.ExtractString(row.Args[2]) == "/tier3" {
			got = append(got, types.ExtractString(row.Args[1]))
		}
	}
	sort.Strings(got)
	want := []string{"peer/z2.go", "peer/z3.go", "peer/z4.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Git/explicit exclusions or pre-cap admission failed: %v, want %v", got, want)
	}
	for _, p := range want {
		if !strings.Contains(brief, p+" (tier3, 50)") {
			t.Errorf("eligible neighbor absent from rendered brief: %s", brief)
		}
	}
	for _, p := range []string{"peer/a_excluded.go", "peer/z1.go"} {
		if strings.Contains(brief, p+" (") {
			t.Errorf("tracked explicit exclusion or Git ignore reached brief: %s", brief)
		}
	}
}

func TestTier3Membership_RejectsNonMemberSeedsAndUnavailableAuthority(t *testing.T) {
	for _, mode := range []string{"excluded_seed", "outside_seed", "missing_workspace"} {
		t.Run(mode, func(t *testing.T) {
			root, seed, _, _ := tier3MemberWorkspace(t, "python")
			workDir := root
			switch mode {
			case "excluded_seed":
				seed = "hidden.py"
				writeFile(t, root, seed, []byte("import admitted\n"))
			case "outside_seed":
				workDir = filepath.Join(root, "scoped")
				if err := os.Mkdir(workDir, 0o755); err != nil {
					t.Fatal(err)
				}
			case "missing_workspace":
				workDir = filepath.Join(root, "missing")
			}
			b := NewTieredContextBuilder(DefaultTieredContextConfig(workDir))
			added := map[string]bool{}
			got := b.expandImportGraph(t.Context(), []ContextFile{{FilePath: filepath.Join(root, seed)}}, added)
			if len(got) != 0 || len(added) != 0 {
				t.Fatalf("unadmitted seed/authority produced context or spent capacity: %v / %v", got, added)
			}
		})
	}
}
