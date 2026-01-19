package core

import (
	"testing"
)

// MangleWatcher tests are skipped because fsnotify spawns Windows-specific
// goroutines that goleak cannot reliably track or ignore.
// The watcher functionality is tested at integration level.

func TestMangleWatcher_New(t *testing.T) {
	t.Skip("Skipping: fsnotify Windows goroutines cause goleak failures")
}

func TestMangleWatcher_StartStop(t *testing.T) {
	t.Skip("Skipping: fsnotify Windows goroutines cause goleak failures")
}

func TestMangleWatcher_GetStats(t *testing.T) {
	t.Skip("Skipping: fsnotify Windows goroutines cause goleak failures")
}

func TestMangleWatcher_ResetStats(t *testing.T) {
	t.Skip("Skipping: fsnotify Windows goroutines cause goleak failures")
}

func TestMangleWatcher_GetWatchedDirs(t *testing.T) {
	t.Skip("Skipping: fsnotify Windows goroutines cause goleak failures")
}
