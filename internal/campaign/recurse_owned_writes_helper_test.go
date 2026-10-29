package campaign

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/observation"
)

// attemptFn is a test attempt that edits the tree and reports nothing.
type attemptFn func(ctx context.Context, a RecurseAttempt) error

// ownWrites makes a RecurseExecutor of fn that reports, as the attempt's own
// writes, every file fn changed under root (.git and .nerd aside) -- what a
// real executor's observed return carries. It attributes by difference, so a
// test with an outside writer reports its writes itself instead.
func ownWrites(root string, fn attemptFn) RecurseExecutor {
	return func(ctx context.Context, a RecurseAttempt) ([]observation.FileWrite, error) {
		before := treeContents(root)
		err := fn(ctx, a)
		after := treeContents(root)
		paths := map[string]bool{}
		for p := range before {
			paths[p] = true
		}
		for p := range after {
			paths[p] = true
		}
		var sorted []string
		for p := range paths {
			sorted = append(sorted, p)
		}
		sort.Strings(sorted)
		var writes []observation.FileWrite
		for _, p := range sorted {
			b, hadB := before[p]
			a, hasA := after[p]
			if hadB == hasA && b == a {
				continue
			}
			writes = append(writes, observation.FileWrite{
				Path:   p,
				Before: observation.FileState{Known: true, Exists: hadB, Content: b},
				After:  observation.FileState{Known: true, Exists: hasA, Content: a},
			})
		}
		return writes, err
	}
}

func treeContents(root string) map[string]string {
	out := map[string]string{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if name := d.Name(); path != root && (name == ".git" || name == ".nerd") {
				return filepath.SkipDir
			}
			return nil
		}
		if data, rerr := os.ReadFile(path); rerr == nil && !strings.Contains(path, string(filepath.Separator)+".git"+string(filepath.Separator)) {
			out[path] = string(data)
		}
		return nil
	})
	return out
}

// OwnWrites is ownWrites for the package's external tests.
func OwnWrites(root string, fn func(context.Context, RecurseAttempt) error) RecurseExecutor {
	return ownWrites(root, fn)
}
