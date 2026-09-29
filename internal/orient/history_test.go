package orient

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

func TestFactsFromGitLog_RenameCopyAndGap(t *testing.T) {
	t.Parallel()
	// Newest first. a.md is renamed to b.md, then (in an older commit) added.
	// February has no commit, so the month row is still emitted.
	log := strings.Join([]string{
		"---ORIENT---new|1584230400",
		"R100\ta.md\tb.md",
		"",
		"---ORIENT---old|1579046400",
		"A\ta.md",
		"A\treadme.md",
		"",
	}, "\n")
	facts, n, err := factsFromGitLog(strings.NewReader(log), false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("commits %d", n)
	}
	h := indexHist(facts)
	if _, ok := h["a.md"]; ok {
		t.Fatal("rename left history on a.md")
	}
	b := h["b.md"]
	if b.commits != 2 || b.days != 2 || b.first != 1579046400 || b.last != 1584230400 {
		t.Fatalf("b.md %+v", b)
	}
	if h["readme.md"].commits != 1 || h["readme.md"].first != 1579046400 {
		t.Fatalf("readme %+v", h["readme.md"])
	}
	months := indexMonths(facts)
	if len(months) != 3 {
		t.Fatalf("months %d, want Jan Feb Mar", len(months))
	}
	if months[1].label != "2020-02" || months[1].commits != 0 {
		t.Fatalf("february %+v", months[1])
	}
	if months[0].docs != 2 || months[2].docs != 0 {
		t.Fatalf("docs added %+v %+v", months[0], months[2])
	}
	spans := indexSpans(facts)
	if len(spans) != 3 {
		t.Fatalf("spans %d", len(spans))
	}
	for i := int64(0); i < int64(len(spans))-1; i++ {
		if spans[i].end+1 != spans[i+1].start {
			t.Fatalf("spans overlap or gap: %+v %+v", spans[i], spans[i+1])
		}
	}
	if got := findArg(facts, "repo_span", 3); got != "/no" {
		t.Fatalf("shallow %s", got)
	}
}

func TestFactsFromGitLog_CopyDoesNotStitch(t *testing.T) {
	t.Parallel()
	log := strings.Join([]string{
		"---ORIENT---new|1584230400",
		"C100\told.md\tcopy.md",
		"",
		"---ORIENT---old|1579046400",
		"A\told.md",
		"",
	}, "\n")
	facts, _, err := factsFromGitLog(strings.NewReader(log), false)
	if err != nil {
		t.Fatal(err)
	}
	h := indexHist(facts)
	if h["old.md"].commits != 1 || h["old.md"].first != 1579046400 {
		t.Fatalf("old %+v", h["old.md"])
	}
	if h["copy.md"].commits != 1 || h["copy.md"].first != 1584230400 {
		t.Fatalf("copy %+v", h["copy.md"])
	}
}

func TestFactsFromGitLog_RenameChainAndBack(t *testing.T) {
	t.Parallel()
	chain := strings.Join([]string{
		"---ORIENT---3|300",
		"R100\tb.md\tc.md",
		"",
		"---ORIENT---2|200",
		"R100\ta.md\tb.md",
		"",
		"---ORIENT---1|100",
		"A\ta.md",
		"",
	}, "\n")
	facts, _, err := factsFromGitLog(strings.NewReader(chain), false)
	if err != nil {
		t.Fatal(err)
	}
	h := indexHist(facts)
	if _, ok := h["a.md"]; ok {
		t.Fatal("chain left a.md")
	}
	if _, ok := h["b.md"]; ok {
		t.Fatal("chain left b.md")
	}
	if h["c.md"].commits != 3 || h["c.md"].first != 100 || h["c.md"].last != 300 {
		t.Fatalf("c.md %+v", h["c.md"])
	}

	// Renamed away and back: history lives on the HEAD name, once.
	back := strings.Join([]string{
		"---ORIENT---2|200",
		"R100\tb.md\ta.md",
		"",
		"---ORIENT---1|100",
		"R100\ta.md\tb.md",
		"",
		"---ORIENT---0|50",
		"A\ta.md",
		"",
	}, "\n")
	facts, _, err = factsFromGitLog(strings.NewReader(back), false)
	if err != nil {
		t.Fatal(err)
	}
	h = indexHist(facts)
	if _, ok := h["b.md"]; ok {
		t.Fatal("rename-back left b.md")
	}
	if h["a.md"].commits != 3 || h["a.md"].first != 50 || h["a.md"].last != 200 {
		t.Fatalf("a.md %+v", h["a.md"])
	}
}

func TestFactsFromGitLog_QuotedPathAndEmpty(t *testing.T) {
	t.Parallel()
	log := "---ORIENT---h|100\nA\t\"docs/my file.md\"\n"
	facts, n, err := factsFromGitLog(strings.NewReader(log), true)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal(n)
	}
	h := indexHist(facts)
	if _, ok := h["docs/my file.md"]; !ok {
		t.Fatalf("paths %v", h)
	}
	if got := findArg(facts, "repo_span", 3); got != "/yes" {
		t.Fatalf("shallow %s", got)
	}
	facts, n, err = factsFromGitLog(strings.NewReader(""), false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || findArg(facts, "repo_span", 2) != "0" {
		t.Fatalf("empty log %+v", facts)
	}
	if _, _, err := factsFromGitLog(strings.NewReader("---ORIENT---nope\n"), false); err == nil {
		t.Fatal("bad marker")
	}
}

func TestScanHistory_Repo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	git(t, root, time.Time{}, "init")
	h, err := ScanHistory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !h.Empty || h.Shallow || h.Commits != 0 {
		t.Fatalf("empty repo %+v", h)
	}

	jan := time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
	mar := time.Date(2020, 3, 15, 0, 0, 0, 0, time.UTC)
	writeFile(t, root, "readme.md", "# readme\n")
	writeFile(t, root, "a.md", "alpha body\n")
	for i := 0; i < 8; i++ {
		writeFile(t, root, fmt.Sprintf("c%d.md", i), "cohort\n")
	}
	git(t, root, jan, "add", ".")
	git(t, root, jan, "commit", "-m", "birth")
	git(t, root, mar, "mv", "a.md", "b.md")
	git(t, root, mar, "commit", "-m", "rename")
	for i := 0; i < 4; i++ {
		when := mar.Add(time.Duration(i) * time.Hour)
		writeFile(t, root, "day.md", fmt.Sprintf("day %d\n", i))
		git(t, root, when, "add", "day.md")
		git(t, root, when, "commit", "-m", fmt.Sprintf("day %d", i))
	}

	h, err = ScanHistory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if h.Shallow || h.Empty || h.Commits != 6 {
		t.Fatalf("scan commits=%d shallow=%v empty=%v", h.Commits, h.Shallow, h.Empty)
	}
	hist := indexHist(h.Facts)
	if _, ok := hist["a.md"]; ok {
		t.Fatal("git rename left a.md in history")
	}
	if hist["b.md"].commits < 2 || hist["b.md"].first != jan.Unix() {
		t.Fatalf("b.md %+v", hist["b.md"])
	}
	if hist["day.md"].commits != 4 || hist["day.md"].days != 1 {
		t.Fatalf("day.md %+v", hist["day.md"])
	}
	months := indexMonths(h.Facts)
	if len(months) != 3 || months[1].label != "2020-02" || months[1].commits != 0 {
		t.Fatalf("months %+v", months)
	}
	born := 0
	for p, rec := range hist {
		if rec.first == jan.Unix() && isDocPath(p) {
			born++
		}
	}
	if born != 10 {
		t.Fatalf("january births %d, want 10 (readme, renamed b, c0-c7)", born)
	}

	if _, err := ScanHistory(context.Background(), t.TempDir()); err == nil {
		t.Fatal("non-repo")
	}

	dst := t.TempDir()
	// A local path clone hardlinks the object store and ignores --depth.
	// --no-local forces the transport, which is what records a shallow clone.
	git(t, root, time.Time{}, "-c", "safe.directory=*", "clone", "--no-local", "--depth", "1", root, dst)
	shallow, err := ScanHistory(context.Background(), dst)
	if err != nil {
		t.Fatal(err)
	}
	if !shallow.Shallow {
		t.Fatal("depth-1 clone was not shallow")
	}
	if got := findArg(shallow.Facts, "repo_span", 3); got != "/yes" {
		t.Fatalf("span shallow %s", got)
	}
}

type histView struct {
	first, last int64
	commits     int64
	days        int64
}

func indexHist(facts []types.Fact) map[string]histView {
	out := map[string]histView{}
	for _, f := range facts {
		if f.Predicate != "repo_file_history" || len(f.Args) != 5 {
			continue
		}
		first, _ := types.ExtractInt64(f.Args[1])
		last, _ := types.ExtractInt64(f.Args[2])
		commits, _ := types.ExtractInt64(f.Args[3])
		days, _ := types.ExtractInt64(f.Args[4])
		out[types.ExtractString(f.Args[0])] = histView{first, last, commits, days}
	}
	return out
}

type monthView struct {
	label   string
	commits int64
	files   int64
	docs    int64
}

func indexMonths(facts []types.Fact) map[int64]monthView {
	out := map[int64]monthView{}
	for _, f := range facts {
		if f.Predicate != "repo_month" || len(f.Args) != 5 {
			continue
		}
		i, _ := types.ExtractInt64(f.Args[0])
		commits, _ := types.ExtractInt64(f.Args[2])
		files, _ := types.ExtractInt64(f.Args[3])
		docs, _ := types.ExtractInt64(f.Args[4])
		out[i] = monthView{types.ExtractString(f.Args[1]), commits, files, docs}
	}
	return out
}

type spanView struct{ start, end int64 }

func indexSpans(facts []types.Fact) map[int64]spanView {
	out := map[int64]spanView{}
	for _, f := range facts {
		if f.Predicate != "repo_month_span" || len(f.Args) != 3 {
			continue
		}
		i, _ := types.ExtractInt64(f.Args[0])
		s, _ := types.ExtractInt64(f.Args[1])
		e, _ := types.ExtractInt64(f.Args[2])
		out[i] = spanView{s, e}
	}
	return out
}

func findArg(facts []types.Fact, pred string, arg int) string {
	for _, f := range facts {
		if f.Predicate == pred && arg < len(f.Args) {
			return types.ExtractString(f.Args[arg])
		}
	}
	return ""
}

func git(t *testing.T, dir string, when time.Time, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv(when)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitEnv(when time.Time) []string {
	kv := map[string]string{
		"GIT_AUTHOR_NAME":     "Orient Test",
		"GIT_AUTHOR_EMAIL":    "orient@example.com",
		"GIT_COMMITTER_NAME":  "Orient Test",
		"GIT_COMMITTER_EMAIL": "orient@example.com",
		"GIT_OPTIONAL_LOCKS":  "0",
	}
	if !when.IsZero() {
		stamp := when.UTC().Format(time.RFC3339)
		kv["GIT_AUTHOR_DATE"] = stamp
		kv["GIT_COMMITTER_DATE"] = stamp
	}
	return withEnv(os.Environ(), kv)
}

func writeFile(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestChunkAndHeadings(t *testing.T) {
	t.Parallel()
	s := strings.Repeat("a", 3) + "émore"
	parts := chunkBytes(s, 4)
	if len(parts) < 2 || parts[0] != "aaa" {
		t.Fatalf("chunks %#v", parts)
	}
	if !strings.HasPrefix(parts[1], "é") {
		t.Fatalf("split a rune: %#v", parts)
	}
	body := "# Title\n\n```\n# not\n```\n\n## Real\n"
	if got := headingCount(body); got != 2 {
		t.Fatalf("headings %d", got)
	}
	known := map[string]struct{}{
		"docs/b.md": {},
		"readme.md": {},
	}
	text := "See [b](b.md#section) and [up](../readme.md) and [web](https://example.com).\n"
	got := linksIn("docs/a.md", text, known)
	if strings.Join(got, ",") != "docs/b.md,readme.md" {
		t.Fatalf("links %v", got)
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s", "")
}
