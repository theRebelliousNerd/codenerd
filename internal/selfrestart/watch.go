package selfrestart

import (
	"context"
	"debug/buildinfo"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// settleInterval is how often a changed executable is re-read until it holds
// still: a build writing in place passes through partial files, and only one
// that stays the same across two reads and parses as a Go binary is a build.
const settleInterval = 250 * time.Millisecond

// Watch reports, through changed, each settled rebuild of the executable at
// path (a rename over it, or a write in place). It watches the directory,
// since a rename over the file replaces the inode a file watch would hold,
// and runs until ctx ends.
func Watch(ctx context.Context, path string, changed func(Identity)) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := w.Add(filepath.Dir(path)); err != nil {
		_ = w.Close()
		return err
	}
	go func() {
		defer w.Close()
		var settle <-chan time.Time
		var last Identity
		stable := 0
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if filepath.Clean(ev.Name) != filepath.Clean(path) {
					continue
				}
				stable, last = 0, Identity{}
				settle = time.After(settleInterval)
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			case <-settle:
				settle = nil
				id, err := IdentityOf(path)
				if err != nil {
					settle = time.After(settleInterval) // mid-rename: try again
					continue
				}
				if last.Path != "" && !last.Changed(id) {
					stable++
				} else {
					stable = 0
				}
				last = id
				_, parseErr := buildinfo.ReadFile(path)
				if stable == 0 || parseErr != nil {
					settle = time.After(settleInterval)
					continue
				}
				changed(id)
			}
		}
	}()
	return nil
}
