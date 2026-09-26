package campaign

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// recurseGit is the ratchet's hands: it names what an attempt changed, commits
// a kept attempt, and puts back a reverted one. It touches only the paths the
// attempt changed. There is no reset --hard, no stash, no clean: the loop runs
// in the owner's checkout, and anything it did not write is not its to undo.
type recurseGit struct {
	root string // workspace root; paths are relative to it, slash-separated
}

// DefaultRecurseBranch is where recurse commits unless configured otherwise.
const DefaultRecurseBranch = "nerd/recurse"

// Commit trailers the resume logic reads back from the log.
const (
	recurseCycleTrailer   = "Recurse-Cycle"
	recurseFindingTrailer = "Recurse-Finding"
)

var errNotARepo = errors.New("recurse: the workspace is not a git repository; the ratchet needs git to keep and revert attempts")

func (g recurseGit) run(ctx context.Context, stdin []byte, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.root
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// prepare checks the repository is one the loop can own and puts it on
// branch. A tracked change the loop did not make would be committed or
// reverted along with an attempt, so the loop refuses to start over one.
func (g recurseGit) prepare(ctx context.Context, branch string) error {
	if _, err := g.run(ctx, nil, "rev-parse", "--show-toplevel"); err != nil {
		return errNotARepo
	}
	dirty, err := g.run(ctx, nil, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if strings.TrimSpace(dirty) != "" {
		return fmt.Errorf("recurse: the checkout has uncommitted changes; commit or stash them first so no attempt can take them with it:\n%s", dirty)
	}
	current, err := g.run(ctx, nil, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(current) == branch {
		return nil
	}
	if _, err := g.run(ctx, nil, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		_, err = g.run(ctx, nil, "switch", "--quiet", branch)
		return err
	}
	_, err = g.run(ctx, nil, "switch", "--quiet", "--create", branch)
	return err
}

// untracked lists the untracked, not-ignored files under root.
func (g recurseGit) untracked(ctx context.Context) (map[string]bool, error) {
	out, err := g.run(ctx, nil, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			set[p] = true
		}
	}
	return set, nil
}

// changed lists what the working tree changed since HEAD under root: tracked
// files modified, added or deleted, plus untracked files that were not there
// before the attempt. Sorted, slash-separated, relative to root.
func (g recurseGit) changed(ctx context.Context, untrackedBefore map[string]bool) ([]string, error) {
	out, err := g.run(ctx, nil, "diff", "--name-only", "--relative", "-z", "HEAD", "--", ".")
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			set[p] = true
		}
	}
	now, err := g.untracked(ctx)
	if err != nil {
		return nil, err
	}
	for p := range now {
		if !untrackedBefore[p] {
			set[p] = true
		}
	}
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths, nil
}

// commit records paths as one commit whose trailers name the cycle and the
// finding, so a resumed run can tell a kept attempt from an unfinished one.
func (g recurseGit) commit(ctx context.Context, paths []string, subject string, cycle int, findingID string) (string, error) {
	if len(paths) == 0 {
		return "", errors.New("recurse: nothing to commit")
	}
	spec := []byte(strings.Join(paths, "\x00"))
	if _, err := g.run(ctx, spec, "add", "--all", "--pathspec-from-file=-", "--pathspec-file-nul"); err != nil {
		return "", err
	}
	msg := fmt.Sprintf("%s\n\n%s: %d\n%s: %s\n", subject, recurseCycleTrailer, cycle, recurseFindingTrailer, findingID)
	args := []string{"commit", "--quiet", "--file=-"}
	if name, _ := g.run(ctx, nil, "config", "user.email"); strings.TrimSpace(name) == "" {
		// No identity configured: commit as the loop rather than fail every
		// kept attempt. A configured identity always wins.
		args = append([]string{"-c", "user.name=codeNERD recurse", "-c", "user.email=recurse@codenerd.invalid"}, args...)
	}
	if _, err := g.run(ctx, []byte(msg), args...); err != nil {
		return "", err
	}
	head, err := g.run(ctx, nil, "rev-parse", "HEAD")
	return strings.TrimSpace(head), err
}

// attemptDiff is what an attempt changed, for the next attempt at the same
// work to read: the patch against HEAD for tracked paths and the head of each
// new file, bounded to limit bytes. It is read before the revert erases it,
// and a failure to read it costs only the note, never the revert.
func (g recurseGit) attemptDiff(ctx context.Context, paths []string, limit int) string {
	if len(paths) == 0 {
		return ""
	}
	var b strings.Builder
	patch, err := g.run(ctx, nil, append([]string{"diff", "--no-color", "HEAD", "--"}, paths...)...)
	if err == nil {
		b.WriteString(patch)
	}
	inHead, _ := g.run(ctx, nil, "ls-tree", "-r", "--name-only", "-z", "HEAD")
	tracked := map[string]bool{}
	for _, p := range strings.Split(inHead, "\x00") {
		tracked[p] = true
	}
	for _, p := range paths {
		if tracked[p] || b.Len() >= limit {
			continue
		}
		data, err := os.ReadFile(filepath.Join(g.root, filepath.FromSlash(p)))
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "new file %s:\n%s\n", p, data)
	}
	out := b.String()
	if len(out) > limit {
		out = out[:limit] + "\n... (truncated)"
	}
	return out
}

// revert puts paths back as HEAD has them: a tracked path is checked out, a
// path HEAD does not have is removed. Nothing outside paths is touched.
func (g recurseGit) revert(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	spec := []byte(strings.Join(paths, "\x00"))
	// Unstage first: an attempt that ran `git add` left the index ahead of
	// HEAD, and a checkout restores from the index unless told otherwise.
	if _, err := g.run(ctx, spec, "reset", "--quiet", "--pathspec-from-file=-", "--pathspec-file-nul"); err != nil {
		return err
	}
	// HEAD's files under root, as the membership test for "restore or
	// remove". ls-tree run from root lists root's subtree with root-relative
	// paths, the same spelling diff and ls-files used.
	inHead, err := g.run(ctx, nil, "ls-tree", "-r", "--name-only", "-z", "HEAD")
	if err != nil {
		return err
	}
	tracked := map[string]bool{}
	for _, p := range strings.Split(inHead, "\x00") {
		if p != "" {
			tracked[p] = true
		}
	}
	var restore []string
	for _, p := range paths {
		if tracked[p] {
			restore = append(restore, p)
			continue
		}
		if err := os.Remove(filepath.Join(g.root, filepath.FromSlash(p))); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("recurse: remove %s: %w", p, err)
		}
	}
	if len(restore) > 0 {
		if _, err := g.run(ctx, []byte(strings.Join(restore, "\x00")), "checkout", "HEAD", "--pathspec-from-file=-", "--pathspec-file-nul"); err != nil {
			return err
		}
	}
	return nil
}

// keptCycles reads the cycles the log records as kept, newest first, up to
// limit commits back.
func (g recurseGit) keptCycles(ctx context.Context, limit int) (map[int]string, error) {
	out, err := g.run(ctx, nil, "log", fmt.Sprintf("-%d", limit), "--format=%H%x00%(trailers:key="+recurseCycleTrailer+",valueonly)%x01")
	if err != nil {
		return nil, err
	}
	kept := map[int]string{}
	for _, rec := range strings.Split(out, "\x01") {
		hash, value, ok := strings.Cut(strings.TrimSpace(rec), "\x00")
		if !ok {
			continue
		}
		var cycle int
		if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &cycle); err == nil {
			kept[cycle] = hash
		}
	}
	return kept, nil
}
