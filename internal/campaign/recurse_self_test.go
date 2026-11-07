package campaign

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The codeNERD module in miniature: the module path and main package that mark
// a workspace as the tree its own binary is built from, an embedded file, and
// the fixture's red store test.
var selfModule = map[string]string{
	"go.mod":              "module codenerd\n\ngo 1.21\n",
	".gitignore":          ".nerd/\n",
	"store/store.go":      "package store\n\nfunc Get() int { return 1 }\n",
	"store/store_test.go": recurseModule["store/store_test.go"],
	"web/web.go":          "package web\n\nimport \"codenerd/store\"\n\nfunc Page() int { return store.Get() }\n",
	"rules/rules.go":      "package rules\n\nimport _ \"embed\"\n\n//go:embed policy.txt\nvar Policy string\n",
	"rules/policy.txt":    "allow\n",
	"cmd/nerd/main.go":    "package main\n\nimport (\n\t_ \"codenerd/rules\"\n\t_ \"codenerd/web\"\n)\n\nfunc main() {}\n",
}

func selfFixture(t *testing.T) string {
	t.Helper()
	return recurseFixture(t, selfModule)
}

// fakeBuilds stands for the go build and the self-check: it records the calls
// and writes a marker where the binary would be.
type fakeBuilds struct {
	mu          sync.Mutex
	builds      []string
	checks      []string
	buildErrors []error // consumed one per build; nil entry or exhausted = success
	checkErr    error
}

func (f *fakeBuilds) hooks() *RecurseSelfUpdate {
	return &RecurseSelfUpdate{
		Build: func(ctx context.Context, root, out string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.builds = append(f.builds, out)
			if len(f.buildErrors) > 0 {
				err := f.buildErrors[0]
				f.buildErrors = f.buildErrors[1:]
				if err != nil {
					return err
				}
			}
			return os.WriteFile(out, []byte("binary"), 0o755)
		},
		Check: func(ctx context.Context, root, binary string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.checks = append(f.checks, binary)
			return f.checkErr
		},
	}
}

func (f *fakeBuilds) built() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.builds)
}

func withSelf(f *fakeBuilds) func(*RecurseCycleConfig) {
	return func(c *RecurseCycleConfig) { c.SelfUpdate = f.hooks() }
}

// What the binary is built from decides the rebuild: a Go change, go.mod and
// an embedded file do, docs and tests do not, and a path nothing builds from
// does not either.
func TestBinaryTouched_ReadsWhatTheBinaryIsBuiltFrom(t *testing.T) {
	root := selfFixture(t)
	writeTree(t, root, map[string]string{"docs/notes.txt": "not built\n", "unreferenced/blob.yaml": "x: 1\n"})
	for _, tc := range []struct {
		name string
		path string
		want bool
	}{
		{"a Go source file", "store/store.go", true},
		{"a Go file that was deleted", "store/gone.go", true},
		{"go.mod", "go.mod", true},
		{"an embedded file", "rules/policy.txt", true},
		{"a test file", "store/store_test.go", false},
		{"markdown", "README.md", false},
		{"a text file no package embeds", "docs/notes.txt", false},
		{"a yaml file no package embeds", "unreferenced/blob.yaml", false},
	} {
		got, err := binaryTouched(context.Background(), root, []string{tc.path})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s (%s): touched = %v, want %v", tc.name, tc.path, got, tc.want)
		}
	}
	if got, err := binaryTouched(context.Background(), root, []string{"README.md", "docs/notes.txt", "rules/policy.txt"}); err != nil || !got {
		t.Fatalf("one embedded file among docs is enough: %v, %v", got, err)
	}
}

// A kept commit that changes a Go file builds the binary, checks it, and ends
// the run with the restart to continue on. The next run resumes from the
// ledger without redoing the fix.
func TestRecurseCycles_AKeptGoChangeRebuildsAndAsksToRestart(t *testing.T) {
	root := selfFixture(t)
	g := recurseGit{root: root}
	fake := &fakeBuilds{}
	progress := &lineHook{}
	res, err := runCycles(t, context.Background(), root, 0, progress, func(ctx context.Context, a RecurseAttempt) error {
		if a.Angle == "" {
			write(t, root, "store/store.go", fixedStore)
		}
		return nil
	}, withSelf(fake))
	t.Logf("progress:\n%s", progress)
	if !errors.Is(err, ErrRecurseRestart) || res == nil || res.Restart == nil {
		t.Fatalf("RunRecurseCycles = %+v, %v; want ErrRecurseRestart with a Restart", res, err)
	}
	head := headOf(t, g)
	if res.Restart.Commit != head {
		t.Fatalf("restart commit = %s, want HEAD %s", res.Restart.Commit, head)
	}
	if want := filepath.Join(recurseBinDir(root), "nerd-"+head[:12]+exeSuffix()); res.Restart.Binary != want {
		t.Fatalf("restart binary = %s, want %s in the run's state directory", res.Restart.Binary, want)
	}
	if data, err := os.ReadFile(res.Restart.Binary); err != nil || string(data) != "binary" {
		t.Fatalf("the built binary is not at its path: %q, %v", data, err)
	}
	if fake.built() != 1 || len(fake.checks) != 1 {
		t.Fatalf("builds %d, checks %d; want one of each", fake.built(), len(fake.checks))
	}
	if strings.HasSuffix(fake.builds[0], filepath.Base(res.Restart.Binary)) {
		t.Fatalf("the build wrote straight to the final path %s; only a binary that passed its check belongs there", fake.builds[0])
	}
	if _, err := os.Stat(fake.builds[0]); err == nil {
		t.Fatal("the side file the build wrote is left behind")
	}
	recs := journalOf(t, root)
	rebuilds, reexecs := stepsOf(recs, stepRebuild), stepsOf(recs, stepReexec)
	if len(rebuilds) != 1 || rebuilds[0].Commit != head || rebuilds[0].Detail != res.Restart.Binary || len(reexecs) != 1 {
		t.Fatalf("ledger: rebuild %+v, reexec %+v", rebuilds, reexecs)
	}
	if indexOfStep(recs, stepRebuild) < indexOfStep(recs, stepRatchet) {
		t.Fatal("the rebuild came before the attempt that changed the code was judged")
	}
	if !strings.Contains(progress.String(), "INFO self-update: built") {
		t.Fatalf("the rebuild is an INFO line:\n%s", progress)
	}
	if st, _ := ReadRecurseStatus(root); len(st.Base.Generations) != 1 || !strings.Contains(st.String(), "built from "+head[:9]) {
		t.Fatalf("status shows no generation:\n%s", st)
	}

	// Resume on the "new binary": the ledger knows the fix was kept, so the
	// run carries on from there and, with nothing newly changed, does not
	// rebuild again.
	again := &fakeBuilds{}
	res2, err := runCycles(t, context.Background(), root, 1, nil, func(ctx context.Context, a RecurseAttempt) error {
		if a.Angle == "" {
			t.Errorf("the resumed run attempts %s again; the ledger says it was kept", a.Finding.ID)
		}
		return nil
	}, withSelf(again))
	if err != nil {
		t.Fatalf("resumed run: %v", err)
	}
	if res2.Restart != nil || again.built() != 0 {
		t.Fatalf("a resumed run with no new change rebuilt (%d builds) or asked to restart", again.built())
	}
}

// A base change that is only docs does not rebuild; one that changes a Go file
// does. Not the loop's own commit: the base's.
func TestRecurseCycles_OnlyBaseChangesToTheBinaryRebuild(t *testing.T) {
	for _, tc := range []struct {
		name        string
		files       map[string]string
		wantRebuild bool
	}{
		{"docs only", map[string]string{"docs/notes.md": "# notes\n", "README.md": "readme\n"}, false},
		{"a Go file", map[string]string{"web/extra.go": "package web\n\nfunc Extra() int { return 1 }\n"}, true},
		{"an embedded file", map[string]string{"rules/policy.txt": "deny\n"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := selfFixture(t)
			fake := &fakeBuilds{}
			landed := false
			progress := &lineHook{}
			res, err := runCycles(t, context.Background(), root, 1, progress, func(ctx context.Context, a RecurseAttempt) error {
				if !landed {
					landed = true
					landOnBase(t, recurseGit{root: root}, "main", tc.files, "base change")
				}
				return nil
			}, withSelf(fake))
			t.Logf("progress:\n%s", progress)
			if tc.wantRebuild {
				if !errors.Is(err, ErrRecurseRestart) || res.Restart == nil || fake.built() != 1 {
					t.Fatalf("err %v, restart %+v, builds %d; a change to the binary's inputs rebuilds", err, res, fake.built())
				}
				return
			}
			if err != nil || res.Restart != nil || fake.built() != 0 {
				t.Fatalf("err %v, builds %d: a docs-only change must not rebuild or restart", err, fake.built())
			}
			if len(stepsOf(journalOf(t, root), stepAbsorb)) != 1 || !strings.Contains(progress.String(), "no rebuild") {
				t.Fatalf("the docs-only absorb should be recorded and its no-rebuild said:\n%s", progress)
			}
		})
	}
}

// A failed build keeps the old binary running and is recorded; it is not
// retried on the same tree, only once the tree has moved (here the next kept
// commit). A build that compiles but fails its check never reaches the restart.
func TestRecurseCycles_AFailedBuildKeepsTheOldBinaryAndRetriesAfterTheNextChange(t *testing.T) {
	root := selfFixture(t)
	g := recurseGit{root: root}
	fake := &fakeBuilds{buildErrors: []error{errors.New("compile error: undefined thing")}}
	landed := false
	progress := &lineHook{}
	progress.onLine = func(line string) {
		// The first fix attempt starts after the failed build and several
		// boundaries on the unchanged tree.
		if strings.HasPrefix(line, "recurse cycle 3:") && fake.built() != 1 {
			t.Errorf("builds before the next change = %d: a failed build is not retried on the same tree", fake.built())
		}
	}
	res, err := runCycles(t, context.Background(), root, 0, progress, func(ctx context.Context, a RecurseAttempt) error {
		if a.Angle == "" {
			write(t, root, "store/store.go", fixedStore)
		}
		if !landed {
			landed = true
			landOnBase(t, g, "main", map[string]string{"web/extra.go": "package web\n\nfunc Extra() int { return 1 }\n"}, "base changes Go")
		}
		return nil
	}, withSelf(fake))
	t.Logf("progress:\n%s", progress)
	if !errors.Is(err, ErrRecurseRestart) || res.Restart == nil {
		t.Fatalf("err %v: the retry after the next kept commit should have built and asked to restart", err)
	}
	if res.Kept != 1 {
		t.Fatalf("kept %d: the run must have gone on and kept the fix on the old binary before the retry", res.Kept)
	}
	recs := journalOf(t, root)
	failed := stepsOf(recs, stepRebuildFailed)
	if len(failed) != 1 || !strings.Contains(failed[0].Detail, "undefined thing") {
		t.Fatalf("rebuild_failed records = %+v", failed)
	}
	if len(stepsOf(recs, stepRebuild)) != 1 || fake.built() != 2 {
		t.Fatalf("rebuilds %d, builds %d; want the failure then one success", len(stepsOf(recs, stepRebuild)), fake.built())
	}
	if !strings.Contains(progress.String(), "WARN self-update: build from") {
		t.Fatalf("a build failure is a WARN:\n%s", progress)
	}

	// A failing self-check is the same: no restart, nothing at the final path.
	root2 := selfFixture(t)
	bad := &fakeBuilds{checkErr: errors.New("exit status 2: plan failed")}
	landed = false
	res2, err := runCycles(t, context.Background(), root2, 1, nil, func(ctx context.Context, a RecurseAttempt) error {
		if !landed {
			landed = true
			landOnBase(t, recurseGit{root: root2}, "main", map[string]string{"web/extra.go": "package web\n\nfunc Extra() int { return 1 }\n"}, "base changes Go")
		}
		return nil
	}, withSelf(bad))
	if err != nil || res2.Restart != nil {
		t.Fatalf("err %v, restart %+v: a binary that fails its self-check must not be restarted onto", err, res2.Restart)
	}
	if entries, _ := os.ReadDir(recurseBinDir(root2)); len(entries) != 0 {
		t.Fatalf("binaries left behind after a failed check: %v", entries)
	}
	if f := stepsOf(journalOf(t, root2), stepRebuildFailed); len(f) != 1 || !strings.Contains(f[0].Detail, "plan failed") {
		t.Fatalf("rebuild_failed = %+v", f)
	}
}

// Self-update is off unless the workspace is the codeNERD module itself.
func TestRecurseCycles_SelfUpdateIsOffOutsideTheCodeNERDModule(t *testing.T) {
	root := recurseFixture(t, nil) // module example.com/m
	fake := &fakeBuilds{}
	landed := false
	progress := &lineHook{}
	res, err := runCycles(t, context.Background(), root, 1, progress, func(ctx context.Context, a RecurseAttempt) error {
		if !landed {
			landed = true
			landOnBase(t, recurseGit{root: root}, "main", map[string]string{"web/extra.go": "package web\n\nfunc Extra() int { return 1 }\n"}, "base changes Go")
		}
		return nil
	}, withSelf(fake))
	if err != nil || res.Restart != nil || fake.built() != 0 {
		t.Fatalf("err %v, builds %d: a workspace that is not codeNERD never rebuilds itself", err, fake.built())
	}
	if !strings.Contains(progress.String(), "is not the codeNERD module") {
		t.Fatalf("the reason it is off is not said:\n%s", progress)
	}
}

func TestPruneBinaries_KeepsTheNewest(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	// nerd-a is the oldest, nerd-d the newest.
	for i, name := range []string{"nerd-a", "nerd-b", "nerd-c", "nerd-d"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, nil, 0o755); err != nil {
			t.Fatal(err)
		}
		when := now.Add(time.Duration(i-4) * time.Hour)
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
	}
	if removed := pruneBinaries(dir, 3); len(removed) != 1 || filepath.Base(removed[0]) != "nerd-a" {
		t.Fatalf("removed %v, want only the oldest", removed)
	}
}
