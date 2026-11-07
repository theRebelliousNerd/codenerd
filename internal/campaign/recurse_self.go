package campaign

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/types"
)

// Running its own improvements. When recurse works on codeNERD itself, its
// kept commits (and the base commits it absorbs) change the code of the very
// binary it is running, and a process keeps the code it started with: a
// failure the new code already fixes keeps happening until a human restarts
// it. At the safe point the run rebuilds the binary from its tree into
// .nerd/recurse/bin, proves the build runs, and hands over to it with the same
// arguments; the new process resumes from the ledger like any restart.
//
// Rulings.
//   - Detection is go.mod's module path (build.IsCodeNERDModule), never a
//     nerd.md declaration: nerd.md is the workspace's own text and must not
//     choose a build for the run to exec.
//   - A change triggers a rebuild when it touches a file the binary is built
//     from: the exact set `go list` reports for the main package (Go, cgo and
//     embedded files), plus any non-test .go file and go.mod/go.sum so a
//     deleted file is not missed. Docs and tests do not.
//   - A failed build or self-check is a warning in the ledger and the log. The
//     old binary keeps running, and the build is tried again once the tree has
//     moved (the next kept commit or absorb).
//   - The hand-over is the cmd layer's: the loop returns ErrRecurseRestart
//     through its normal exit, so the journal, the lock and the kernel close in
//     order before the process image is replaced.

// ErrRecurseRestart ends a run that has built a binary to continue on; the
// result's Restart names it.
var ErrRecurseRestart = errors.New("recurse: continuing on a rebuilt binary")

// RecurseRestart is the binary a run asks its caller to continue on.
type RecurseRestart struct {
	// Binary is the built, self-checked executable.
	Binary string
	// Commit is the tree commit it was built from.
	Commit string
}

// RecurseSelfUpdate switches a run's self-update on and carries how it builds
// and checks. Nil in a run's config means off. A nil hook is the production
// one.
type RecurseSelfUpdate struct {
	// Build compiles the module in root into the file out.
	Build func(ctx context.Context, root, out string) error
	// Check proves the built binary runs against root.
	Check func(ctx context.Context, root, binary string) error
}

// recurseSelfState is a run's self-update bookkeeping.
type recurseSelfState struct {
	hooks RecurseSelfUpdate
	// pending are paths changed since they were last classified.
	pending map[string]bool
	// stale: some change touched what the binary is built from, and no build
	// has replaced the running binary since.
	stale bool
	// lastTry is the tree commit a build was last attempted on.
	lastTry string
}

// startSelf switches self-update on when the config asks and the workspace is
// the codeNERD module. A mission is not self-updated (see startBase).
func (r *recurseRun) startSelf() {
	if r.cfg.SelfUpdate == nil || r.cfg.Mission != nil {
		return
	}
	if !build.IsCodeNERDModule(r.root) {
		r.info("self-update: off, %s is not the codeNERD module", r.root)
		return
	}
	s := &recurseSelfState{hooks: *r.cfg.SelfUpdate, pending: map[string]bool{}}
	if s.hooks.Build == nil {
		s.hooks.Build = defaultSelfBuild
	}
	if s.hooks.Check == nil {
		s.hooks.Check = defaultSelfCheck
	}
	r.self = s
	r.info("self-update: on, this run rebuilds and restarts onto its own commits when they change the binary's inputs")
}

// noteTreeChange records files a kept commit or an absorb changed.
func (r *recurseRun) noteTreeChange(paths []string) {
	if r.self == nil {
		return
	}
	for _, p := range paths {
		r.self.pending[p] = true
	}
}

// selfUpdate rebuilds and hands over when a change touched the binary's
// inputs. It returns ErrRecurseRestart when the new binary is ready.
func (r *recurseRun) selfUpdate(ctx context.Context, pass int) error {
	s := r.self
	if s == nil {
		return nil
	}
	if len(s.pending) > 0 {
		paths := sortedSet(s.pending)
		s.pending = map[string]bool{}
		touches, err := binaryTouched(ctx, r.root, paths)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			r.warn("self-update: cannot tell whether %d changed file(s) are built into the binary (%v); rebuilding to be safe", len(paths), err)
			touches = true
		}
		if touches {
			s.stale = true
		} else {
			r.info("self-update: %d changed file(s) are not built into the binary; no rebuild", len(paths))
		}
	}
	if !s.stale {
		return nil
	}
	head, err := r.git.commitOf(ctx, "HEAD")
	if err != nil {
		return err
	}
	if head == s.lastTry {
		return nil // a build of this very tree failed; wait for the tree to move
	}
	s.lastTry = head
	bin, err := r.buildSelf(ctx, head)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		detail := types.ClampText(err.Error(), 4000, "build failure")
		r.warn("self-update: build from %.9s failed, the old binary keeps running; trying again after the next change: %s", head, oneLine(detail))
		return r.journal.append(recurseRecord{Step: stepRebuildFailed, Pass: pass, Commit: head, Detail: detail})
	}
	if err := r.journal.append(recurseRecord{Step: stepRebuild, Pass: pass, Commit: head, Detail: bin}); err != nil {
		return err
	}
	if recurseStopRequested(r.root) {
		r.info("self-update: built %s from %.9s; a stop was requested, so the run ends on the old binary", bin, head)
		return nil
	}
	r.info("self-update: built %s from %.9s and checked it; continuing on it, resuming from the ledger", bin, head)
	if err := r.journal.append(recurseRecord{Step: stepReexec, Pass: pass, Commit: head, Detail: bin}); err != nil {
		return err
	}
	r.saveWorkspaceState(ctx, pass)
	r.result.Restart = &RecurseRestart{Binary: bin, Commit: head}
	return ErrRecurseRestart
}

// recurseBinDir holds the run's rebuilt binaries.
func recurseBinDir(root string) string { return filepath.Join(recurseDir(root), "bin") }

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// buildSelf builds the tree's binary into the state directory and returns its
// path once it has also passed the self-check. The build goes to a side file
// that only a passing check renames into place: a binary that did not build
// and run is never at the path a restart would exec.
func (r *recurseRun) buildSelf(ctx context.Context, commit string) (string, error) {
	dir := recurseBinDir(r.root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	final := filepath.Join(dir, "nerd-"+commit[:min(12, len(commit))]+exeSuffix())
	side := strings.TrimSuffix(final, exeSuffix()) + "-building" + exeSuffix()
	defer os.Remove(side)
	if err := r.self.hooks.Build(ctx, r.root, side); err != nil {
		return "", fmt.Errorf("go build: %w", err)
	}
	if err := r.self.hooks.Check(ctx, r.root, side); err != nil {
		return "", fmt.Errorf("self-check of the built binary: %w", err)
	}
	if err := os.Rename(side, final); err != nil {
		return "", err
	}
	if pruned := pruneBinaries(dir, 3); len(pruned) > 0 {
		r.info("self-update: removed %d older binary generation(s)", len(pruned))
	}
	return final, nil
}

// pruneBinaries removes all but the keep newest binaries in dir (the running
// generation is among them) and returns what it removed.
func pruneBinaries(dir string, keep int) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type bin struct {
		path string
		mod  time.Time
	}
	var bins []bin
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !e.IsDir() && strings.HasPrefix(e.Name(), "nerd-") {
			bins = append(bins, bin{filepath.Join(dir, e.Name()), info.ModTime()})
		}
	}
	sort.Slice(bins, func(i, j int) bool { return bins[i].mod.After(bins[j].mod) })
	var removed []string
	for i := keep; i < len(bins); i++ {
		if os.Remove(bins[i].path) == nil {
			removed = append(removed, bins[i].path)
		}
	}
	return removed
}

func defaultSelfBuild(ctx context.Context, root, out string) error {
	output, err := build.BuildBinary(ctx, root, out, build.SelfBinaryPackage)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// defaultSelfCheck runs the built binary's own recurse planner over the tree:
// it loads the embedded policy, derives the sweep graph and detects the gates,
// and exits 0, without a model.
func defaultSelfCheck(ctx context.Context, root, binary string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "campaign", "recurse", "--plan")
	cmd.Dir = root
	cmd.Env = build.AllowedEnv(root)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("`campaign recurse --plan` failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// inertChange is a path no binary is built from, whatever the build lists:
// tests and prose.
func inertChange(p string) bool {
	return strings.HasSuffix(p, "_test.go") || strings.HasSuffix(p, ".md")
}

// binaryTouched reports whether any of paths (root-relative, slash-separated)
// is built into the module's binary.
func binaryTouched(ctx context.Context, root string, paths []string) (bool, error) {
	live := paths[:0:0]
	for _, p := range paths {
		if !inertChange(p) {
			live = append(live, p)
		}
	}
	if len(live) == 0 {
		return false, nil
	}
	for _, p := range live {
		if base := path.Base(p); base == "go.mod" || base == "go.sum" || base == "go.work" ||
			(strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go")) {
			return true, nil
		}
	}
	inputs, err := binaryInputs(ctx, root)
	if err != nil {
		return false, err
	}
	for _, p := range live {
		if inputs[p] {
			return true, nil
		}
	}
	return false, nil
}

// binaryInputs is the set of root-relative files the main package's build
// reads in the module: what `go list` reports as Go, cgo, C-family and
// embedded files of every package the binary depends on.
func binaryInputs(ctx context.Context, root string) (map[string]bool, error) {
	fields := []string{"GoFiles", "CgoFiles", "CFiles", "CXXFiles", "MFiles", "HFiles", "SFiles", "SysoFiles", "EmbedFiles"}
	var tmpl strings.Builder
	tmpl.WriteString("{{if .Module}}{{if .Module.Main}}{{$d := .Dir}}")
	for _, f := range fields {
		fmt.Fprintf(&tmpl, "{{range .%s}}{{$d}}/{{.}}\n{{end}}", f)
	}
	tmpl.WriteString("{{end}}{{end}}")
	args := []string{"list"}
	args = append(args, build.TestTagsForWorkspace(root)...)
	args = append(args, "-deps", "-e", "-f", tmpl.String(), build.SelfBinaryPackage)
	env, argv := build.GoInvocation(root, root, args)
	cmd := exec.CommandContext(ctx, "go", argv...)
	cmd.Dir = root
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		return nil, fmt.Errorf("go list: %w: %s", err, stderr)
	}
	set := map[string]bool{}
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		rel, err := filepath.Rel(root, line)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		set[filepath.ToSlash(rel)] = true
	}
	return set, nil
}
