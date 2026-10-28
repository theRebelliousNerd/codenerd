package sessionlog

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"codenerd/internal/atomicfile"
)

// LeaseWindow is how recent a session's lease heartbeat must be for the
// session to count as running.
const LeaseWindow = 5 * time.Minute

type lease struct {
	PID     int       `json:"pid"`
	Updated time.Time `json:"updated"`
}

func writeLease(dir string, now time.Time) error {
	data, err := MarshalFile(lease{PID: os.Getpid(), Updated: now})
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(filepath.Join(dir, LeaseFile), data, 0o644)
}

// leaseFresh reports whether dir holds a lease heartbeat inside LeaseWindow.
func leaseFresh(dir string, now time.Time) bool {
	data, err := os.ReadFile(filepath.Join(dir, LeaseFile))
	if err != nil {
		return false
	}
	var l lease
	if json.Unmarshal(data, &l) != nil {
		return false
	}
	return now.Sub(l.Updated) < LeaseWindow
}

// PruneReport says what a prune removed and what it spared.
type PruneReport struct {
	Removed []string
	Bytes   int64
	// Running are sessions spared because their lease is fresh.
	Running []string
	Failed  []string
	Kept    int
}

// Prune deletes session folders whose last activity is older than ttl. A ttl
// of zero or less keeps everything. A session in keep (the one being booted),
// and any session whose lease heartbeat is fresh (running in another
// process), is never deleted.
func Prune(workspace string, ttl time.Duration, keep map[string]bool, now time.Time) (PruneReport, error) {
	var rep PruneReport
	if ttl <= 0 {
		return rep, nil
	}
	root := SessionsDir(workspace)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return rep, nil
		}
		return rep, err
	}
	for _, e := range entries {
		if !e.IsDir() || ValidateID(e.Name()) != nil {
			continue
		}
		id := e.Name()
		dir := filepath.Join(root, id)
		if keep[id] {
			rep.Kept++
			continue
		}
		if leaseFresh(dir, now) {
			rep.Running = append(rep.Running, id)
			rep.Kept++
			continue
		}
		activity, size := folderActivity(dir)
		if now.Sub(activity) <= ttl {
			rep.Kept++
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			rep.Failed = append(rep.Failed, id)
			continue
		}
		rep.Removed = append(rep.Removed, id)
		rep.Bytes += size
	}
	sort.Strings(rep.Removed)
	return rep, nil
}

// folderActivity returns the newest modification time of any file under dir
// (the folder's own time when it holds none) and the total size of its files.
// Directories do not count: creating a sub-folder is not activity.
func folderActivity(dir string) (time.Time, int64) {
	var newest time.Time
	var size int64
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		size += info.Size()
		return nil
	})
	if newest.IsZero() {
		if info, err := os.Stat(dir); err == nil {
			newest = info.ModTime()
		}
	}
	return newest, size
}
