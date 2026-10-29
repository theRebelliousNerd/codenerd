package browser

// Reaper regressions are adapted from BrowserNERD's Apache-2.0 reaper tests.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/mangle"

	"github.com/google/uuid"
)

type fakeProcessQuerier struct {
	mu        sync.Mutex
	processes []ProcessInfo
	alive     map[int]bool
	killed    []int
	queryErr  error
	killErr   error
	keepAlive bool
}

func (q *fakeProcessQuerier) QueryChromeProcesses(context.Context) ([]ProcessInfo, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]ProcessInfo(nil), q.processes...), q.queryErr
}

func (q *fakeProcessQuerier) IsProcessAlive(pid int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.alive[pid]
}

func (q *fakeProcessQuerier) KillProcessTree(pid int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.killed = append(q.killed, pid)
	if q.killErr != nil {
		return q.killErr
	}
	if !q.keepAlive {
		for _, child := range descendantPIDs(pid, q.processes) {
			delete(q.alive, child)
		}
	}
	return nil
}

func reaperProfile(t *testing.T, owner string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	container := filepath.Join(root, owner, "user-data")
	profile := filepath.Join(container, "b3-"+uuid.NewString())
	if _, err := temporaryProfilePath(profile); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(profile) })
	return profile
}

func TestExtractUserDataDir_Formats(t *testing.T) {
	for _, tc := range []struct{ name, command, want string }{
		{"double quote", `chrome.exe --user-data-dir="C:\Temp\rod\user-data\a b" --flag`, `C:\Temp\rod\user-data\a b`},
		{"single quote", `chrome --user-data-dir='/tmp/rod/user-data/abc' --flag`, `/tmp/rod/user-data/abc`},
		{"space separated", `chrome --user-data-dir "/tmp/rod/user-data/a b" --flag`, `/tmp/rod/user-data/a b`},
		{"unquoted", `chrome --user-data-dir=/tmp/rod/user-data/abc`, `/tmp/rod/user-data/abc`},
		{"quoted argument", `chrome "--user-data-dir=/tmp/rod/user-data/abc"`, `/tmp/rod/user-data/abc`},
		{"tabs", "chrome\t--user-data-dir\t/tmp/rod/user-data/abc", `/tmp/rod/user-data/abc`},
		{"missing", `chrome --user-data-path=/tmp/rod/user-data/abc`, ""},
		{"embedded", `chrome --other="--user-data-dir=/tmp/rod/user-data/abc"`, ""},
		{"duplicate", `chrome --user-data-dir=/tmp/rod/user-data/abc --user-data-dir=/tmp/rod/user-data/def`, ""},
		{"unterminated", `chrome --user-data-dir="/tmp/rod/user-data/abc`, ""},
		{"empty", `chrome --user-data-dir=`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractUserDataDir(tc.command); got != tc.want {
				t.Fatalf("extractUserDataDir() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRemoveUserDataDir_SafetyRefusal(t *testing.T) {
	m := NewSessionManagerWithSink(DefaultConfig(), nil)
	profile := reaperProfile(t, "rod")
	personal := t.TempDir()
	marker := filepath.Join(personal, "Cookies")
	if err := os.WriteFile(marker, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{personal, filepath.Dir(profile), filepath.Dir(filepath.Dir(profile)),
		filepath.Join(personal, "rod", "user-data", "misleading"), filepath.Join(profile, "Default"), ".", filepath.VolumeName(profile) + string(filepath.Separator)} {
		err := m.cleanupBrowserProcess(0, dir, true)
		if err == nil || !strings.Contains(err.Error(), "safety refusal") {
			t.Fatalf("cleanup accepted unsafe path %q: %v", dir, err)
		}
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "preserve" {
		t.Fatalf("personal profile changed: %q, %v", data, err)
	}
	if err := m.cleanupBrowserProcess(0, "", true); err != nil {
		t.Fatal(err)
	}
	if err := m.cleanupBrowserProcess(0, profile, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(profile); !os.IsNotExist(err) {
		t.Fatalf("temporary profile still exists: %v", err)
	}
}

func TestRemoveUserDataDir_RefusesSymlinkEscape(t *testing.T) {
	profile := reaperProfile(t, "rod")
	if err := os.Remove(profile); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, profile); err != nil {
		t.Skipf("host cannot create directory symlinks: %v", err)
	}
	m := NewSessionManagerWithSink(DefaultConfig(), nil)
	if err := m.cleanupBrowserProcess(0, profile, true); err == nil {
		t.Fatal("symlink escape accepted")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("symlink target changed: %v", err)
	}
}

func TestReapOrphans_ReapsOnlyDeadParentTrees(t *testing.T) {
	orphan := reaperProfile(t, "rod")
	live := reaperProfile(t, "rod")
	leaf := reaperProfile(t, "browsernerd")
	personal := t.TempDir()
	command := func(dir string) string { return `chrome --user-data-dir="` + dir + `"` }
	q := &fakeProcessQuerier{
		alive: map[int]bool{100: true, 101: true, 102: true, 8888: true, 200: true, 201: true, 300: true, 401: true},
		processes: []ProcessInfo{
			{PID: 100, PPID: 9999, CommandLine: command(orphan)},
			{PID: 101, PPID: 100, CommandLine: command(orphan)},
			{PID: 102, PPID: 100, CommandLine: command(orphan)},
			{PID: 200, PPID: 8888, CommandLine: command(live)},
			{PID: 201, PPID: 200, CommandLine: command(live)},
			{PID: 300, PPID: 1, CommandLine: command(personal)},
			{PID: 401, PPID: 400, CommandLine: command(leaf)},
		},
	}
	m := NewSessionManagerWithSink(DefaultConfig(), nil)
	m.SetProcessQuerier(q)
	count, err := m.ReapOrphans(context.Background())
	if err != nil || count != 2 || m.ReapedOrphans() != 2 {
		t.Fatalf("ReapOrphans() = %d, %v, cumulative=%d", count, err, m.ReapedOrphans())
	}
	for _, pid := range []int{100, 101, 102, 401} {
		if q.IsProcessAlive(pid) {
			t.Fatalf("orphan PID %d survived", pid)
		}
	}
	for _, pid := range []int{200, 201, 300} {
		if !q.IsProcessAlive(pid) {
			t.Fatalf("protected PID %d was killed", pid)
		}
	}
	for _, dir := range []string{orphan, leaf} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("orphan profile retained: %v", err)
		}
	}
	if _, err := os.Stat(live); err != nil {
		t.Fatal("live profile removed", err)
	}
	q.alive[9999] = true
	q.alive[100] = true
	q.processes = []ProcessInfo{{PID: 100, PPID: 9999, CommandLine: command(orphan)}}
	if count, err := m.ReapOrphans(context.Background()); err != nil || count != 0 {
		t.Fatalf("live-parent mutation reaped: %d, %v", count, err)
	}
}

func TestReapOrphans_RefusesAmbiguousOwnership(t *testing.T) {
	for _, tc := range []struct {
		name   string
		owner  string
		parent int
		marker string
	}{
		{"unmarked native profile", "codenerd", 9999, ""},
		{"live native owner after reparent", "codenerd", 1, `{"launcher_pid":8888,"temporary":true}`},
		{"invalid owner", "codenerd", 9999, `{bad`},
		{"explicit profile under legacy temp root", "rod", 9999, `{"launcher_pid":9999,"temporary":false}`},
		{"current launcher", "rod", os.Getpid(), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile := reaperProfile(t, tc.owner)
			if tc.marker != "" {
				if err := os.WriteFile(filepath.Join(profile, ".codenerd-owner.json"), []byte(tc.marker), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			q := &fakeProcessQuerier{alive: map[int]bool{100: true, 8888: true}, processes: []ProcessInfo{{PID: 100, PPID: tc.parent, CommandLine: `chrome --user-data-dir="` + profile + `"`}}}
			m := NewSessionManagerWithSink(DefaultConfig(), nil)
			m.SetProcessQuerier(q)
			if count, err := m.ReapOrphans(context.Background()); count != 0 || err != nil {
				t.Fatalf("unexpected reap %d, %v", count, err)
			}
			if len(q.killed) != 0 {
				t.Fatal("protected process was killed")
			}
		})
	}
}

func TestReapOrphans_FailuresPreserveProfileAndCount(t *testing.T) {
	for _, tc := range []struct {
		name              string
		queryErr, killErr error
		keepAlive         bool
	}{
		{"query failed", errors.New("query failed"), nil, false},
		{"kill failed", nil, errors.New("kill denied"), false},
		{"kill did not exit", nil, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile := reaperProfile(t, "rod")
			cfg := DefaultConfig()
			cfg.Reaper.TimeoutMs, cfg.Reaper.PollIntervalMs = 20, 1
			q := &fakeProcessQuerier{alive: map[int]bool{100: true}, processes: []ProcessInfo{{PID: 100, PPID: 9999, CommandLine: `chrome --user-data-dir="` + profile + `"`}}, queryErr: tc.queryErr, killErr: tc.killErr, keepAlive: tc.keepAlive}
			m := NewSessionManagerWithSink(cfg, nil)
			m.SetProcessQuerier(q)
			if count, err := m.ReapOrphans(context.Background()); count != 0 || err == nil {
				t.Fatalf("failure suppressed: %d, %v", count, err)
			}
			if m.ReapedOrphans() != 0 {
				t.Fatal("failed cleanup counted as success")
			}
			if _, err := os.Stat(profile); err != nil {
				t.Fatal("failed cleanup deleted profile", err)
			}
		})
	}
}

func TestBrowserLifecycle_KillsOwnedTreeAndPreservesExplicitProfile(t *testing.T) {
	for _, operation := range []string{"close", "shutdown"} {
		for _, temporary := range []bool{true, false} {
			t.Run(operation+"/"+map[bool]string{true: "temporary", false: "explicit"}[temporary], func(t *testing.T) {
				profile := reaperProfile(t, "rod")
				q := &fakeProcessQuerier{alive: map[int]bool{500: true}, processes: []ProcessInfo{{PID: 500, PPID: os.Getpid(), CommandLine: `chrome --user-data-dir="` + profile + `"`}}}
				m := NewSessionManagerWithSink(DefaultConfig(), nil)
				m.SetProcessQuerier(q)
				cancelled := false
				m.browsers["test"] = &browserRecord{meta: BrowserInstance{ID: "test"}, pid: 500, userDataDir: profile, tempDir: temporary, cancel: func() { cancelled = true }}
				var err error
				if operation == "close" {
					err = m.CloseBrowser(context.Background(), "test")
				} else {
					err = m.Shutdown(context.Background())
				}
				if err != nil {
					t.Fatal(err)
				}
				if !cancelled || !reflect.DeepEqual(q.killed, []int{500}) {
					t.Fatalf("cleanup not wired: cancelled=%v killed=%v", cancelled, q.killed)
				}
				_, statErr := os.Stat(profile)
				if temporary && !os.IsNotExist(statErr) {
					t.Fatalf("temporary profile retained: %v", statErr)
				}
				if !temporary && statErr != nil {
					t.Fatalf("explicit profile removed: %v", statErr)
				}
			})
		}
	}
}

func TestDescendantPIDs_LeavesOtherTreesAlone(t *testing.T) {
	got := descendantPIDs(10, []ProcessInfo{{PID: 11, PPID: 10}, {PID: 12, PPID: 11}, {PID: 20, PPID: 1}, {PID: 21, PPID: 20}, {PID: 10, PPID: 12}})
	if !reflect.DeepEqual(got, []int{10, 11, 12}) {
		t.Fatalf("descendants = %v", got)
	}
}

func TestCleanupBrowserProcess_RefusesReusedPID(t *testing.T) {
	profile := reaperProfile(t, "rod")
	q := &fakeProcessQuerier{alive: map[int]bool{500: true}, processes: []ProcessInfo{{PID: 500, PPID: 99, CommandLine: `chrome --user-data-dir="` + t.TempDir() + `"`}}}
	m := NewSessionManagerWithSink(DefaultConfig(), nil)
	m.SetProcessQuerier(q)
	if err := m.cleanupBrowserProcess(500, profile, true); err == nil || !strings.Contains(err.Error(), "safety refusal") {
		t.Fatalf("reused PID accepted: %v", err)
	}
	if len(q.killed) != 0 {
		t.Fatal("reused PID was killed")
	}
	if _, err := os.Stat(profile); err != nil {
		t.Fatal("profile removed on ownership refusal", err)
	}
}

type attendedNavigationSink struct {
	facts []mangle.Fact
	err   error
}

func (s *attendedNavigationSink) AddFacts(facts []mangle.Fact) error {
	if s.err != nil {
		return s.err
	}
	s.facts = append(s.facts, facts...)
	return nil
}

func TestMarkAttendedNavigation_RecordsSensorFactAndPropagatesFailure(t *testing.T) {
	for _, failure := range []bool{false, true} {
		sink := &attendedNavigationSink{}
		if failure {
			sink.err = errors.New("kernel unavailable")
		}
		m := NewSessionManagerWithSink(DefaultConfig(), sink)
		before := time.Now().UnixMilli()
		err := m.markAttendedNavigation("session")
		if failure {
			if !errors.Is(err, sink.err) {
				t.Fatalf("kernel error lost: %v", err)
			}
			continue
		}
		if err != nil || len(sink.facts) != 1 || sink.facts[0].Predicate != "attended" || sink.facts[0].Args[0] != "session" {
			t.Fatalf("attended fact missing: %v, %v", sink.facts, err)
		}
		stamp, ok := sink.facts[0].Args[1].(int64)
		if !ok || stamp < before || stamp > time.Now().UnixMilli() {
			t.Fatalf("invalid timestamp: %v", stamp)
		}
	}
}

func TestReapOrphans_UsesDeadNativeOwnerAfterReparent(t *testing.T) {
	profile := reaperProfile(t, "codenerd")
	data := []byte(`{"launcher_pid":9999,"chrome_pid":100,"temporary":true}`)
	if err := os.WriteFile(filepath.Join(profile, ".codenerd-owner.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	q := &fakeProcessQuerier{alive: map[int]bool{100: true}, processes: []ProcessInfo{{PID: 100, PPID: 1, CommandLine: `chrome --user-data-dir="` + profile + `"`}}}
	m := NewSessionManagerWithSink(DefaultConfig(), nil)
	m.SetProcessQuerier(q)
	if count, err := m.ReapOrphans(context.Background()); count != 1 || err != nil {
		t.Fatalf("marked native orphan not reaped: %d, %v", count, err)
	}
	if _, err := os.Stat(profile); !os.IsNotExist(err) {
		t.Fatalf("native orphan profile retained: %v", err)
	}
}
