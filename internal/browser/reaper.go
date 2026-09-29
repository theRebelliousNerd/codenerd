package browser

// Process lifecycle is adapted from BrowserNERD's Apache-2.0 process reaper.
// codeNERD restricts cleanup to native lifecycle edges and resolved temporary
// profiles; no process termination capability is exposed to the model.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"codenerd/internal/config"
)

type ProcessInfo struct {
	PID         int    `json:"ProcessId"`
	PPID        int    `json:"ParentProcessId"`
	CommandLine string `json:"CommandLine"`
}

type ProcessQuerier interface {
	QueryChromeProcesses(context.Context) ([]ProcessInfo, error)
	IsProcessAlive(int) bool
	KillProcessTree(int) error
}

func extractUserDataDir(commandLine string) string {
	// Match an argument, not a flag embedded in another argument's value.
	var found string
	for i := 0; i < len(commandLine); {
		for i < len(commandLine) && (commandLine[i] == ' ' || commandLine[i] == '\t') {
			i++
		}
		if i == len(commandLine) {
			break
		}
		var arg strings.Builder
		var quote byte
		for i < len(commandLine) {
			c := commandLine[i]
			if quote != 0 {
				if runtime.GOOS != "windows" && quote == '"' && c == '\\' && i+1 < len(commandLine) &&
					(commandLine[i+1] == '\\' || commandLine[i+1] == '"') {
					i++
					arg.WriteByte(commandLine[i])
				} else if c == quote {
					quote = 0
				} else {
					arg.WriteByte(c)
				}
			} else if c == '"' || (c == '\'' && (arg.Len() == 0 || strings.HasSuffix(arg.String(), "="))) {
				quote = c
			} else if c == ' ' || c == '\t' {
				break
			} else {
				arg.WriteByte(c)
			}
			i++
		}
		if quote != 0 {
			return ""
		}
		value := arg.String()
		if value == "--user-data-dir" {
			// The next argument is parsed with the same quoting rules.
			value = extractUserDataDir("--user-data-dir=" + strings.TrimLeft(commandLine[i:], " \t"))
			if found != "" || value == "" {
				return ""
			}
			return value
		}
		if strings.HasPrefix(value, "--user-data-dir=") {
			if found != "" {
				return ""
			}
			found = strings.TrimPrefix(value, "--user-data-dir=")
			if found == "" {
				return ""
			}
		}
	}
	return found
}

func profileIdentity(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path
}

// resolveProfilePath resolves existing ancestors even after a profile was removed.
func resolveProfilePath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", err
	}
	resolved, err = resolveProfilePath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(path)), nil
}

func temporaryProfilePath(dir string) (string, error) {
	refuse := func() (string, error) {
		return "", fmt.Errorf("safety refusal: path %q is not a resolved codeNERD/Rod temporary profile", dir)
	}
	if dir == "" || !filepath.IsAbs(dir) {
		return refuse()
	}
	root, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return "", fmt.Errorf("resolve system temporary directory: %w", err)
	}
	clean := filepath.Clean(dir)
	resolved, err := resolveProfilePath(clean)
	if err != nil {
		return "", fmt.Errorf("resolve temporary profile: %w", err)
	}
	for _, owner := range []string{"codenerd", "rod", "browsernerd"} {
		container := filepath.Join(root, owner, "user-data")
		// Require a single profile directory. Never delete the container, a
		// nested Chrome Default profile, or a symlink/junction target elsewhere.
		if profileIdentity(filepath.Dir(resolved)) == profileIdentity(container) &&
			profileIdentity(clean) == profileIdentity(resolved) {
			return resolved, nil
		}
	}
	return refuse()
}

func isTemporaryUserDataDir(dir string) bool {
	_, err := temporaryProfilePath(dir)
	return err == nil
}

type temporaryProfileOwner struct {
	LauncherPID int  `json:"launcher_pid"`
	ChromePID   int  `json:"chrome_pid"`
	Temporary   bool `json:"temporary"`
}

func writeTemporaryProfileOwner(dir string, chromePID int, temporary bool) error {
	data, err := json.Marshal(temporaryProfileOwner{LauncherPID: os.Getpid(), ChromePID: chromePID, Temporary: temporary})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".codenerd-owner.json"), data, 0o600)
}

func profileLauncherAlive(dir string, q ProcessQuerier) bool {
	data, err := os.ReadFile(filepath.Join(dir, ".codenerd-owner.json"))
	if err != nil {
		// Legacy Rod/BrowserNERD profiles predate our marker. Native profiles
		// require it; explicit profiles in these containers are marked durable.
		return !os.IsNotExist(err) || strings.EqualFold(filepath.Base(filepath.Dir(filepath.Dir(dir))), "codenerd")
	}
	var owner temporaryProfileOwner
	if json.Unmarshal(data, &owner) != nil || owner.LauncherPID <= 1 || !owner.Temporary {
		return true
	}
	return owner.LauncherPID == os.Getpid() || q.IsProcessAlive(owner.LauncherPID)
}

func removeUserDataDir(ctx context.Context, dir string, interval time.Duration) error {
	if dir == "" {
		return nil
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		resolved, err := temporaryProfilePath(dir)
		if err != nil {
			return err
		}
		if err = os.RemoveAll(resolved); err == nil {
			return nil
		}
		if waitErr := waitProcessPoll(ctx, interval); waitErr != nil {
			return errors.Join(err, waitErr)
		}
	}
}

func waitProcessPoll(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (m *SessionManager) SetProcessQuerier(q ProcessQuerier) {
	m.processMu.Lock()
	defer m.processMu.Unlock()
	m.mu.Lock()
	m.processQuerier = q
	m.mu.Unlock()
}

func (m *SessionManager) ReapedOrphans() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reapedOrphans
}

func (m *SessionManager) processInspector() ProcessQuerier {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.processQuerier == nil {
		m.processQuerier = newProcessQuerier(m.cfg.Reaper.WithDefaults())
	}
	return m.processQuerier
}

// ReapOrphans is a lifecycle driver operation, never a registered model tool.
func (m *SessionManager) ReapOrphans(ctx context.Context) (int, error) {
	m.processMu.Lock()
	defer m.processMu.Unlock()
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	policy := m.cfg.Reaper.WithDefaults()
	if problems := policy.Check("browser.reaper"); len(problems) != 0 {
		return 0, fmt.Errorf("invalid browser reaper configuration: %v", problems)
	}
	q := m.processInspector()
	queryCtx, cancel := context.WithTimeout(ctx, policy.Timeout())
	processes, err := q.QueryChromeProcesses(queryCtx)
	cancel()
	if err != nil {
		return 0, fmt.Errorf("query Chrome processes: %w", err)
	}
	type profileTree struct {
		dir       string
		processes []ProcessInfo
	}
	trees := make(map[string]*profileTree)
	allChrome := make(map[int]ProcessInfo)
	for _, p := range processes {
		allChrome[p.PID] = p
		dir, pathErr := temporaryProfilePath(extractUserDataDir(p.CommandLine))
		if pathErr != nil || p.PID <= 1 || p.PID == os.Getpid() || p.PPID <= 0 {
			continue
		}
		key := profileIdentity(dir)
		if trees[key] == nil {
			trees[key] = &profileTree{dir: dir}
		}
		trees[key].processes = append(trees[key].processes, p)
	}
	keys := make([]string, 0, len(trees))
	for key := range trees {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	count := 0
	var result error
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			result = errors.Join(result, err)
			break
		}
		tree := trees[key]
		if profileLauncherAlive(tree.dir, q) {
			continue
		}
		pids := make(map[int]bool)
		for _, p := range tree.processes {
			pids[p.PID] = true
		}
		liveLauncher, hasRoot := false, false
		for _, p := range tree.processes {
			if pids[p.PPID] {
				continue
			}
			hasRoot = true
			_, chromeParent := allChrome[p.PPID]
			if chromeParent || p.PPID == os.Getpid() || (p.PPID > 1 && q.IsProcessAlive(p.PPID)) {
				liveLauncher = true
			}
		}
		if liveLauncher || !hasRoot {
			continue
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(ctx, policy.Timeout())
		var cleanupErr error
		for _, p := range tree.processes {
			if q.IsProcessAlive(p.PID) {
				cleanupErr = errors.Join(cleanupErr, q.KillProcessTree(p.PID))
			}
		}
		if cleanupErr == nil {
			cleanupErr = waitProcessExit(cleanupCtx, q, pids, policy.PollInterval())
		}
		if cleanupErr == nil {
			cleanupErr = removeUserDataDir(cleanupCtx, tree.dir, policy.PollInterval())
		}
		cleanupCancel()
		if cleanupErr != nil {
			result = errors.Join(result, fmt.Errorf("reap Chrome profile %q: %w", tree.dir, cleanupErr))
			continue
		}
		count++
	}
	m.mu.Lock()
	m.reapedOrphans += count
	m.mu.Unlock()
	return count, result
}

func waitProcessExit(ctx context.Context, q ProcessQuerier, pids map[int]bool, interval time.Duration) error {
	for {
		alive := false
		for pid := range pids {
			alive = alive || q.IsProcessAlive(pid)
		}
		if !alive {
			return nil
		}
		if err := waitProcessPoll(ctx, interval); err != nil {
			return fmt.Errorf("Chrome processes still running: %w", err)
		}
	}
}

func (m *SessionManager) cleanupBrowserProcess(pid int, dir string, temporary bool) error {
	m.processMu.Lock()
	defer m.processMu.Unlock()
	policy := m.cfg.Reaper.WithDefaults()
	if problems := policy.Check("browser.reaper"); len(problems) != 0 {
		return fmt.Errorf("invalid browser reaper configuration: %v", problems)
	}
	ctx, cancel := context.WithTimeout(context.Background(), policy.Timeout())
	defer cancel()
	if temporary && dir != "" {
		if _, err := temporaryProfilePath(dir); err != nil {
			return err
		}
	}
	q := m.processInspector()
	if pid > 1 {
		if pid == os.Getpid() {
			return fmt.Errorf("safety refusal: refusing to terminate current process")
		}
		processes, err := q.QueryChromeProcesses(ctx)
		if err != nil {
			return fmt.Errorf("inspect owned Chrome tree before cleanup: %w", err)
		}
		owned := make(map[int]bool)
		for _, process := range processes {
			if dir != "" && profileIdentity(extractUserDataDir(process.CommandLine)) == profileIdentity(dir) {
				owned[process.PID] = true
			}
		}
		if !owned[pid] && q.IsProcessAlive(pid) {
			return fmt.Errorf("safety refusal: tracked Chrome PID %d no longer carries its recorded profile", pid)
		}
		if owned[pid] && q.IsProcessAlive(pid) {
			if err := q.KillProcessTree(pid); err != nil {
				return fmt.Errorf("terminate Chrome tree %d: %w", pid, err)
			}
		}
		for ownedPID := range owned {
			if ownedPID == os.Getpid() || ownedPID <= 1 {
				return fmt.Errorf("safety refusal: protected PID in recorded Chrome profile")
			}
			if q.IsProcessAlive(ownedPID) {
				if err := q.KillProcessTree(ownedPID); err != nil {
					return fmt.Errorf("terminate Chrome tree %d: %w", ownedPID, err)
				}
			}
		}
		if err := waitProcessExit(ctx, q, owned, policy.PollInterval()); err != nil {
			return err
		}
	}
	if temporary {
		return removeUserDataDir(ctx, dir, policy.PollInterval())
	}
	return nil
}

func DefaultProcessQuerier() ProcessQuerier {
	return newProcessQuerier(config.DefaultBrowserReaperConfig())
}

func descendantPIDs(root int, processes []ProcessInfo) []int {
	children := make(map[int][]int)
	for _, process := range processes {
		children[process.PPID] = append(children[process.PPID], process.PID)
	}
	queue := []int{root}
	seen := map[int]bool{root: true}
	for i := 0; i < len(queue); i++ {
		for _, child := range children[queue[i]] {
			if !seen[child] {
				seen[child] = true
				queue = append(queue, child)
			}
		}
	}
	return queue
}
