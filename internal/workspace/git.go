package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitEnv is the environment of a git read.
// GIT_DIR and GIT_WORK_TREE are dropped so -C names this workspace rather
// than whatever repository the parent process was pointed at. GIT_OPTIONAL_LOCKS=0
// keeps the read off .git/index.lock.
func gitEnv() []string {
	drop := map[string]bool{
		"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_INDEX_FILE": true,
		"GIT_OBJECT_DIRECTORY": true, "GIT_OPTIONAL_LOCKS": true,
		"GIT_NAMESPACE": true,
	}
	env := os.Environ()
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		key, _, ok := strings.Cut(e, "=")
		if ok && drop[key] {
			continue
		}
		out = append(out, e)
	}
	out = append(out, "GIT_OPTIONAL_LOCKS=0")
	return out
}

// gitRun executes git in root. A non-zero exit is returned as code with a nil
// error when git itself ran; err is set only when the process could not start.
// Exit 1 is meaningful for check-ignore (nothing ignored) and is not a failure.
func gitRun(root string, stdin []byte, args ...string) ([]byte, int, error) {
	argv := make([]string, 0, len(args)+4)
	argv = append(argv, "-c", "core.quotepath=false", "-C", root)
	argv = append(argv, args...)
	cmd := exec.Command("git", argv...)
	cmd.Dir = root
	cmd.Env = gitEnv()
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.Bytes(), 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return stdout.Bytes(), ee.ExitCode(), nil
	}
	return nil, -1, err
}

func gitMissing(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}

// workTree reports whether root is inside a real git work tree.
// A missing git binary is reported as an error so the caller can fall back.
// Any other failure (no repository, a fake .git) is "not a work tree".
func workTree(root string) (bool, error) {
	out, code, err := gitRun(root, nil, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false, err
	}
	if code != 0 {
		return false, nil
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

type gitSnapshot struct {
	files      map[string]struct{}
	boundaries map[string]struct{}
	stamps     snapStamps
	paths      gitPaths
	pathsOK    bool
	refName    string
	refPath    string
}

// gitPaths are the files whose bytes decide whether the snapshot is stale.
// rev-parse --git-path is resolved once: Includes calls the freshness check,
// and a walk must not pay a git process per file.
type gitPaths struct {
	index, head, exclude string
}

func loadSnapshot(root string) (gitSnapshot, error) {
	out, code, err := gitRun(root, nil, "ls-files", "-z", "-co", "--exclude-standard")
	if err != nil {
		return gitSnapshot{}, err
	}
	if code != 0 {
		return gitSnapshot{}, fmt.Errorf("git ls-files in %s exited %d", root, code)
	}
	files := make(map[string]struct{})
	bounds := make(map[string]struct{})
	for _, raw := range splitNUL(out) {
		p, dirEntry, ok := acceptGitPath(raw)
		if !ok {
			continue
		}
		if dirEntry {
			// A trailing slash is git's listing of a nested repository: the
			// directory is one untracked entry and its children are not members.
			bounds[p] = struct{}{}
			continue
		}
		files[p] = struct{}{}
	}
	staged, code, err := gitRun(root, nil, "ls-files", "-z", "-s")
	if err != nil {
		return gitSnapshot{}, err
	}
	if code != 0 {
		return gitSnapshot{}, fmt.Errorf("git ls-files -s in %s exited %d", root, code)
	}
	for _, raw := range splitNUL(staged) {
		mode, p, ok := parseStageRecord(raw)
		if !ok {
			continue
		}
		if mode == "160000" {
			bounds[p] = struct{}{}
			delete(files, p)
		}
	}
	for f := range files {
		if underBoundary(f, bounds) {
			delete(files, f)
		}
	}
	gp, gpErr := resolveGitPaths(root)
	var st snapStamps
	var refName, refPath string
	if gpErr == nil {
		st, refName, refPath = stampCached(root, gp, "", "")
	}
	return gitSnapshot{
		files: files, boundaries: bounds, stamps: st,
		paths: gp, pathsOK: gpErr == nil, refName: refName, refPath: refPath,
	}, nil
}

func parseStageRecord(raw string) (mode, rel string, ok bool) {
	tab := strings.IndexByte(raw, '\t')
	if tab < 0 {
		return "", "", false
	}
	meta := strings.Fields(raw[:tab])
	if len(meta) == 0 {
		return "", "", false
	}
	p, _, good := acceptGitPath(raw[tab+1:])
	if !good {
		return "", "", false
	}
	return meta[0], p, true
}

func underBoundary(rel string, bounds map[string]struct{}) bool {
	for g := range bounds {
		if rel == g || strings.HasPrefix(rel, g+"/") {
			return true
		}
	}
	return false
}

// acceptGitPath normalizes one ls-files path. dirEntry is set when git
// reported a trailing slash. Paths that escape the root are refused.
func acceptGitPath(raw string) (rel string, dirEntry bool, ok bool) {
	p := filepath.ToSlash(raw)
	if p == "" {
		return "", false, false
	}
	dirEntry = strings.HasSuffix(p, "/")
	p = strings.TrimSuffix(p, "/")
	// A leading slash or a drive is outside this work tree. pathClean would
	// otherwise turn "/abs.go" into "abs.go".
	if strings.HasPrefix(p, "/") {
		return "", false, false
	}
	if vol := filepath.VolumeName(filepath.FromSlash(p)); vol != "" {
		return "", false, false
	}
	c := pathClean(p)
	if c == "" || c == ".." || strings.HasPrefix(c, "../") || strings.HasPrefix(c, "/") {
		return "", false, false
	}
	return c, dirEntry, true
}

func pathClean(p string) string {
	c := strings.TrimPrefix(p, "./")
	// path.Clean is slash-only and does not look at the OS.
	if c == "" {
		return ""
	}
	out := make([]string, 0, strings.Count(c, "/")+1)
	for _, seg := range strings.Split(c, "/") {
		switch seg {
		case "", ".":
			continue
		case "..":
			if len(out) == 0 {
				return "../" + c
			}
			out = out[:len(out)-1]
		default:
			out = append(out, seg)
		}
	}
	return strings.Join(out, "/")
}

func splitNUL(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	parts := bytes.Split(b, []byte{0})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		out = append(out, string(p))
	}
	return out
}

// checkIgnore asks git which of rels are ignored. asDir also sends the
// trailing-slash form, because a `name/` rule matches the directory path.
// The returned set is keyed by the path without a trailing slash.
// Exit 1 (nothing ignored) is success. Any other failure is an error so the
// caller can fail closed instead of walking a tree git refused to answer.
func checkIgnore(root string, rels []string, asDir bool) (map[string]bool, error) {
	var buf bytes.Buffer
	for _, r := range rels {
		buf.WriteString(r)
		buf.WriteByte(0)
		if asDir {
			buf.WriteString(r)
			buf.WriteByte('/')
			buf.WriteByte(0)
		}
	}
	out, code, err := gitRun(root, buf.Bytes(), "check-ignore", "-z", "--stdin")
	if err != nil {
		return nil, err
	}
	if code == 1 {
		return map[string]bool{}, nil
	}
	if code != 0 {
		return nil, fmt.Errorf("git check-ignore in %s exited %d", root, code)
	}
	ignored := make(map[string]bool, len(rels))
	for _, raw := range splitNUL(out) {
		p, _, ok := acceptGitPath(raw)
		if !ok {
			p = strings.TrimSuffix(filepath.ToSlash(raw), "/")
		}
		if p != "" {
			ignored[p] = true
		}
	}
	return ignored, nil
}

type snapStamps struct {
	index, head, headRef, gitignore, exclude fileStamp
}

type fileStamp struct {
	exists  bool
	size    int64
	modUnix int64
	modNano int64
}

func resolveGitPaths(root string) (gitPaths, error) {
	index, err1 := gitPath(root, "index")
	head, err2 := gitPath(root, "HEAD")
	exclude, err3 := gitPath(root, "info/exclude")
	if err1 != nil && err2 != nil {
		if err1 != nil {
			return gitPaths{}, err1
		}
		return gitPaths{}, err2
	}
	gp := gitPaths{index: index, head: head}
	if err3 == nil {
		gp.exclude = exclude
	}
	return gp, nil
}

// stampCached stats the resolved git files. The ref file behind a symbolic
// HEAD is resolved once and reused while refName still matches; a branch
// switch is a different ref and pays one more git-path lookup.
func stampCached(root string, gp gitPaths, refName, refPath string) (snapStamps, string, string) {
	st := snapStamps{
		index:     statStamp(gp.index),
		head:      statStamp(gp.head),
		exclude:   statStamp(gp.exclude),
		gitignore: statStamp(filepath.Join(root, ".gitignore")),
	}
	if gp.head == "" {
		return st, "", ""
	}
	b, err := os.ReadFile(gp.head)
	if err != nil {
		return st, refName, refPath
	}
	line := strings.TrimSpace(string(b))
	if !strings.HasPrefix(line, "ref: ") {
		return st, "", ""
	}
	ref := strings.TrimSpace(strings.TrimPrefix(line, "ref: "))
	if ref == "" {
		return st, "", ""
	}
	if ref == refName && refPath != "" {
		st.headRef = statStamp(refPath)
		return st, refName, refPath
	}
	p, err := gitPath(root, ref)
	if err != nil {
		return st, ref, ""
	}
	st.headRef = statStamp(p)
	return st, ref, p
}

func gitPath(root, spec string) (string, error) {
	out, code, err := gitRun(root, nil, "rev-parse", "--git-path", spec)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("git rev-parse --git-path %s exited %d", spec, code)
	}
	p := strings.TrimSpace(string(out))
	if p == "" {
		return "", fmt.Errorf("git rev-parse --git-path %s returned nothing", spec)
	}
	p = filepath.FromSlash(p)
	if filepath.IsAbs(p) {
		return p, nil
	}
	return filepath.Join(root, p), nil
}

func statStamp(path string) fileStamp {
	if path == "" {
		return fileStamp{}
	}
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	mt := fi.ModTime()
	return fileStamp{exists: true, size: fi.Size(), modUnix: mt.Unix(), modNano: int64(mt.Nanosecond())}
}
