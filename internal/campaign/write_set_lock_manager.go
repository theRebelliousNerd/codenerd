package campaign

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const defaultWriteSetLockPollInterval = 10 * time.Millisecond

var ErrWriteSetLockTimeout = errors.New("write_set lock acquisition timed out")

// writeSetLockManager coordinates deterministic file-level write locks.
// Paths are normalized to absolute canonical form and sorted before acquisition.
type writeSetLockManager struct {
	workspace string

	mu     sync.Mutex
	owners map[string]string // normalized absolute path -> taskID
}

func newWriteSetLockManager(workspace string) *writeSetLockManager {
	return &writeSetLockManager{
		workspace: workspace,
		owners:    make(map[string]string),
	}
}

// writeSetLockLease represents an acquired lock set.
// Release is idempotent.
type writeSetLockLease struct {
	manager *writeSetLockManager
	taskID  string
	paths   []string
	once    sync.Once
}

func (l *writeSetLockLease) release() {
	if l == nil || l.manager == nil {
		return
	}
	l.once.Do(func() {
		l.manager.releasePaths(l.taskID, l.paths)
	})
}

func (m *writeSetLockManager) acquire(
	ctx context.Context,
	taskID string,
	writeSet []string,
	pollInterval time.Duration,
) (*writeSetLockLease, error) {
	if m == nil {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if taskID == "" {
		return nil, fmt.Errorf("write_set lock acquisition requires non-empty task id")
	}

	paths := normalizeWriteSetPaths(m.workspace, writeSet)
	if len(paths) == 0 {
		return nil, nil
	}

	if pollInterval <= 0 {
		pollInterval = defaultWriteSetLockPollInterval
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		if ok := m.tryAcquirePaths(taskID, paths); ok {
			return &writeSetLockLease{
				manager: m,
				taskID:  taskID,
				paths:   paths,
			}, nil
		}

		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf("%w: task=%s", ErrWriteSetLockTimeout, taskID)
			}
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *writeSetLockManager) tryAcquirePaths(taskID string, paths []string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range paths {
		owner, held := m.owners[p]
		if held && owner != taskID {
			return false
		}
	}

	for _, p := range paths {
		m.owners[p] = taskID
	}
	return true
}

func (m *writeSetLockManager) releasePaths(taskID string, paths []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range paths {
		if owner, held := m.owners[p]; held && owner == taskID {
			delete(m.owners, p)
		}
	}
}

func normalizeWriteSetPaths(workspace string, writeSet []string) []string {
	if len(writeSet) == 0 {
		return nil
	}

	normalized := make(map[string]struct{}, len(writeSet))
	for _, raw := range writeSet {
		path := normalizeAbsolutePath(workspace, raw)
		if path == "" {
			continue
		}
		normalized[path] = struct{}{}
	}

	if len(normalized) == 0 {
		return nil
	}

	out := make([]string, 0, len(normalized))
	for p := range normalized {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func normalizeAbsolutePath(workspace, rawPath string) string {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return ""
	}

	if !filepath.IsAbs(path) && workspace != "" {
		path = filepath.Join(workspace, path)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}

	normalized := filepath.ToSlash(filepath.Clean(abs))
	if runtime.GOOS == "windows" {
		normalized = strings.ToLower(normalized)
	}
	return normalized
}
