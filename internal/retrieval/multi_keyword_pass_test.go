package retrieval

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func multiKeywordFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	root := t.TempDir()
	files := map[string]string{
		"a.go":        "package a\n\n// Kernel evaluates rules.\nfunc Kernel() {}\nfunc kernelHelper() { Kernel() }\n",
		"b/b.go":      "package b\n\n// Dreamer simulates actions against the kernel.\ntype Dreamer struct{}\n",
		"b/notes.md":  "The dreamer and the kernel talk.\nKernelized text is not a word match.\n",
		"c/c.txt":     "nothing relevant here\n",
		"c/c_test.go": "package c\n// kernel kernel kernel\n",
	}
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "--quiet"}, {"add", "."}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return root
}

func hitKeys(hits []KeywordHit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, strings.Join([]string{filepath.Base(h.FilePath), h.Keyword, h.Context}, "|")+"|"+itoa(h.Line)+":"+itoa(h.Column))
	}
	sort.Strings(out)
	return out
}

// One walk for many keywords records, per keyword, exactly what a walk for
// that keyword alone records.
func TestMultiKeywordPassMatchesSingleKeywordWalks(t *testing.T) {
	root := multiKeywordFixture(t)
	keywords := []string{"kernel", "dreamer", "absent", "rules"}
	r := NewSparseRetriever(&SparseRetrieverConfig{WorkDir: root, SearchTimeout: 10 * time.Second, Parallelism: 4})

	multi, err := r.searchKeywordsNative(context.Background(), keywords)
	if err != nil {
		t.Fatalf("multi pass: %v", err)
	}
	for _, kw := range keywords {
		singleRes, err := r.searchSingleKeyword(context.Background(), kw)
		single := singleRes.Hits
		if err != nil {
			t.Fatalf("single %q: %v", kw, err)
		}
		got, want := hitKeys(multi[kw]), hitKeys(single)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("keyword %q: multi pass\n%s\nsingle walk\n%s", kw, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
	if len(multi["kernel"]) == 0 || len(multi["absent"]) != 0 {
		t.Fatalf("fixture sanity: kernel=%d absent=%d", len(multi["kernel"]), len(multi["absent"]))
	}
}

// Timing on this repository: NERD_RETRIEVAL_BENCH=1 go test -run TestMultiKeywordPassTiming -v ./internal/retrieval/
func TestMultiKeywordPassTiming(t *testing.T) {
	if os.Getenv("NERD_RETRIEVAL_BENCH") == "" {
		t.Skip("set NERD_RETRIEVAL_BENCH=1 to time against this repository")
	}
	root, _ := filepath.Abs("../..")
	keywords := strings.Fields("northstar wizard agentic interview explore codebase questions suggest navigation session kernel dreamer campaign orient embedding shard retrieval budget context atom prompt policy mangle config verify test coverage repair escalation transducer")
	r := NewSparseRetriever(&SparseRetrieverConfig{WorkDir: root, SearchTimeout: 2 * time.Minute})
	start := time.Now()
	if _, _, _, err := r.searchEachKeyword(context.Background(), keywords); err != nil {
		t.Logf("per-keyword walks: %v", err)
	}
	perKeyword := time.Since(start)
	start = time.Now()
	if _, err := r.searchKeywordsNative(context.Background(), keywords); err != nil {
		t.Logf("one pass: %v", err)
	}
	t.Logf("%d keywords over this repository: per-keyword walks %v, one pass %v", len(keywords), perKeyword, time.Since(start))
}
