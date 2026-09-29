package orient

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"codenerd/internal/types"
)

const commitMarker = "---ORIENT---"

// History is one streaming pass of git log for the whole work tree.
// Committer time is the clock. Filesystem mtime is not an identity.
type History struct {
	Facts   []types.Fact
	Shallow bool
	Empty   bool
	Commits int
}

// ScanHistory asserts repo_file_history, repo_month, repo_month_span and
// repo_span for root. One git log process, newest commit first, renames
// followed so a moved document keeps its history on the path it has now.
// A shallow clone is still measured; repo_span carries /yes and the policy
// then refuses to draw eras from the truncated span.
func ScanHistory(ctx context.Context, root string) (*History, error) {
	root, err := absPath(root)
	if err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("orient history: git is not on PATH")
	}
	if err := ensureWorkTree(ctx, root); err != nil {
		return nil, err
	}
	shallow, err := isShallow(ctx, root)
	if err != nil {
		return nil, err
	}
	empty, err := noHEAD(ctx, root)
	if err != nil {
		return nil, err
	}
	if empty {
		return &History{
			Facts:   spanFacts(0, 0, 0, shallow),
			Shallow: shallow,
			Empty:   true,
		}, nil
	}
	cmd := gitCmd(ctx, root,
		"-c", "core.quotePath=false",
		"-c", "safe.directory=*",
		"--no-pager", "log",
		"--name-status", "--find-renames",
		"--format=format:"+commitMarker+"%H|%ct",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("orient history: git log: %w", err)
	}
	facts, n, err := factsFromGitLog(stdout, shallow)
	if err != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if err != nil {
		return nil, err
	}
	if waitErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = waitErr.Error()
		}
		return nil, fmt.Errorf("orient history: git log: %s", msg)
	}
	return &History{Facts: facts, Shallow: shallow, Commits: n}, nil
}

func ensureWorkTree(ctx context.Context, root string) error {
	cmd := gitCmd(ctx, root, "-c", "safe.directory=*", "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("orient history: %s is not a git work tree", root)
	}
	return nil
}

func isShallow(ctx context.Context, root string) (bool, error) {
	cmd := gitCmd(ctx, root, "-c", "safe.directory=*", "rev-parse", "--is-shallow-repository")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("orient history: shallow probe: %w", err)
	}
	switch strings.TrimSpace(string(out)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("orient history: shallow probe returned %q", strings.TrimSpace(string(out)))
	}
}

func noHEAD(ctx context.Context, root string) (bool, error) {
	cmd := gitCmd(ctx, root, "-c", "safe.directory=*", "rev-parse", "--verify", "--quiet", "HEAD")
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return true, nil
	}
	return false, nil
}

func gitCmd(ctx context.Context, root string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	cmd.Env = withEnv(os.Environ(), map[string]string{"GIT_OPTIONAL_LOCKS": "0"})
	return cmd
}

func withEnv(base []string, kv map[string]string) []string {
	skip := make(map[string]bool, len(kv))
	for k := range kv {
		skip[k] = true
	}
	out := make([]string, 0, len(base)+len(kv))
	for _, e := range base {
		k, _, ok := strings.Cut(e, "=")
		if ok && skip[k] {
			continue
		}
		out = append(out, e)
	}
	for k, v := range kv {
		out = append(out, k+"="+v)
	}
	return out
}

type statusChange struct {
	code  string
	paths []string
}

type pendingCommit struct {
	hash    string
	unix    int64
	changes []statusChange
}

type fileRec struct {
	first, last int64
	commits     map[string]struct{}
	days        map[int64]struct{}
}

// factsFromGitLog parses `git log --name-status --find-renames` output in
// the marker format ScanHistory requests. The walk is newest-first, which
// is what lets a rename chain collapse onto the path the file has at HEAD:
// each rename records "this historical name is that newer path", and older
// commits then resolve through the map. Copies are not chained; a copy is
// a new file. A rename back to an earlier name does not close a cycle.
func factsFromGitLog(r io.Reader, shallow bool) ([]types.Fact, int, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	canon := map[string]string{}
	files := map[string]*fileRec{}
	monthCommits := map[monthKey]int{}
	var total int
	var firstUnix, lastUnix int64
	var have bool
	var cur *pendingCommit

	resolve := func(p string) string {
		seen := map[string]bool{}
		for {
			if seen[p] {
				return p
			}
			seen[p] = true
			n, ok := canon[p]
			if !ok || n == "" || n == p {
				return p
			}
			p = n
		}
	}
	link := func(old, neu string) {
		if old == "" || neu == "" || old == neu {
			return
		}
		head := resolve(neu)
		if head == old {
			return
		}
		canon[old] = head
	}
	flush := func() error {
		if cur == nil {
			return nil
		}
		for _, ch := range cur.changes {
			if strings.HasPrefix(ch.code, "R") && len(ch.paths) >= 2 {
				link(ch.paths[0], ch.paths[1])
			}
		}
		touched := map[string]struct{}{}
		for _, ch := range cur.changes {
			var p string
			switch {
			case (strings.HasPrefix(ch.code, "R") || strings.HasPrefix(ch.code, "C")) && len(ch.paths) >= 2:
				p = resolve(ch.paths[1])
			case len(ch.paths) >= 1:
				p = resolve(ch.paths[0])
			}
			if p != "" {
				touched[p] = struct{}{}
			}
		}
		for p := range touched {
			rec := files[p]
			if rec == nil {
				rec = &fileRec{commits: map[string]struct{}{}, days: map[int64]struct{}{}}
				files[p] = rec
			}
			rec.commits[cur.hash] = struct{}{}
			day := cur.unix / 86400
			rec.days[day] = struct{}{}
			if len(rec.commits) == 1 {
				rec.first, rec.last = cur.unix, cur.unix
			} else {
				if cur.unix < rec.first {
					rec.first = cur.unix
				}
				if cur.unix > rec.last {
					rec.last = cur.unix
				}
			}
		}
		k := monthOf(cur.unix)
		monthCommits[k]++
		total++
		if !have || cur.unix < firstUnix {
			firstUnix = cur.unix
		}
		if !have || cur.unix > lastUnix {
			lastUnix = cur.unix
		}
		have = true
		cur = nil
		return nil
	}

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, commitMarker) {
			if err := flush(); err != nil {
				return nil, 0, err
			}
			hash, unix, err := parseMarker(line)
			if err != nil {
				return nil, 0, err
			}
			cur = &pendingCommit{hash: hash, unix: unix}
			continue
		}
		if cur == nil {
			return nil, 0, fmt.Errorf("orient history: status line before any commit: %s", line)
		}
		ch, err := parseStatus(line)
		if err != nil {
			return nil, 0, err
		}
		cur.changes = append(cur.changes, ch)
	}
	if err := sc.Err(); err != nil {
		return nil, 0, fmt.Errorf("orient history: read git log: %w", err)
	}
	if err := flush(); err != nil {
		return nil, 0, err
	}
	if !have {
		return spanFacts(0, 0, 0, shallow), 0, nil
	}
	facts := make([]types.Fact, 0, len(files)+8)
	facts = append(facts, spanFacts(firstUnix, lastUnix, total, shallow)...)
	for p, rec := range files {
		for day := range rec.days {
			facts = append(facts, types.Fact{Predicate: "repo_file_day", Args: []any{p, day}})
		}
		facts = append(facts, types.Fact{
			Predicate: "repo_file_history",
			Args: []any{
				p,
				rec.first,
				rec.last,
				int64(len(rec.commits)),
				int64(len(rec.days)),
			},
		})
	}
	facts = append(facts, monthFacts(firstUnix, lastUnix, monthCommits, files)...)
	return facts, total, nil
}

func parseMarker(line string) (string, int64, error) {
	rest := strings.TrimPrefix(line, commitMarker)
	hash, ts, ok := strings.Cut(rest, "|")
	if !ok || hash == "" || ts == "" {
		return "", 0, fmt.Errorf("orient history: bad commit marker %q", line)
	}
	unix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("orient history: commit time %q: %w", ts, err)
	}
	return hash, unix, nil
}

func parseStatus(line string) (statusChange, error) {
	tab := strings.IndexByte(line, '\t')
	if tab <= 0 {
		return statusChange{}, fmt.Errorf("orient history: bad status line %q", line)
	}
	code := line[:tab]
	parts := strings.Split(line[tab+1:], "\t")
	paths := make([]string, 0, len(parts))
	for _, p := range parts {
		p = cleanGitPath(p)
		if p == "" {
			continue
		}
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		return statusChange{}, fmt.Errorf("orient history: status line names no path: %q", line)
	}
	return statusChange{code: code, paths: paths}, nil
}

func cleanGitPath(p string) string {
	p = strings.TrimSpace(p)
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		if u, err := strconv.Unquote(p); err == nil {
			p = u
		} else {
			p = p[1 : len(p)-1]
		}
	}
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	for strings.HasPrefix(p, "/") {
		p = p[1:]
	}
	if p == "." {
		return ""
	}
	return p
}

type monthKey struct {
	y int
	m time.Month
}

func monthOf(unix int64) monthKey {
	t := time.Unix(unix, 0).UTC()
	return monthKey{t.Year(), t.Month()}
}

func monthFacts(first, last int64, commits map[monthKey]int, files map[string]*fileRec) []types.Fact {
	start := time.Unix(first, 0).UTC()
	end := time.Unix(last, 0).UTC()
	cur := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastM := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)
	added := map[monthKey][2]int{}
	for p, rec := range files {
		k := monthOf(rec.first)
		n := added[k]
		n[0]++
		if isDocPath(p) {
			n[1]++
		}
		added[k] = n
	}
	var out []types.Fact
	for i := 0; !cur.After(lastM); i++ {
		next := cur.AddDate(0, 1, 0)
		k := monthKey{cur.Year(), cur.Month()}
		add := added[k]
		label := cur.Format("2006-01")
		out = append(out, types.Fact{
			Predicate: "repo_month",
			Args:      []any{int64(i), label, int64(commits[k]), int64(add[0]), int64(add[1])},
		})
		// End is the second before the next month starts, so the spans
		// abut and a commit unix falls in exactly one of them.
		out = append(out, types.Fact{
			Predicate: "repo_month_span",
			Args:      []any{int64(i), cur.Unix(), next.Unix() - 1},
		})
		cur = next
	}
	return out
}

func spanFacts(first, last int64, commits int, shallow bool) []types.Fact {
	flag := types.MangleAtom("/no")
	if shallow {
		flag = types.MangleAtom("/yes")
	}
	return []types.Fact{{
		Predicate: "repo_span",
		Args:      []any{first, last, int64(commits), flag},
	}}
}

func isDocPath(p string) bool {
	e := strings.ToLower(pathExt(p))
	switch e {
	case ".md", ".markdown", ".rst", ".txt":
		return true
	default:
		return false
	}
}

func pathExt(p string) string {
	i := strings.LastIndex(p, "/")
	base := p
	if i >= 0 {
		base = p[i+1:]
	}
	j := strings.LastIndex(base, ".")
	if j <= 0 {
		return ""
	}
	return base[j:]
}
