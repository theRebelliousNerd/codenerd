package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func liveReaperConfig(t *testing.T) Config {
	t.Helper()
	bin := os.Getenv("CODENERD_BROWSER_LIVE_CHROME")
	if bin == "" {
		t.Skip("set CODENERD_BROWSER_LIVE_CHROME to the installed Chrome executable to enable local live browser tests")
	}
	if !filepath.IsAbs(bin) {
		t.Fatal("CODENERD_BROWSER_LIVE_CHROME must name an absolute executable path")
	}
	if info, err := os.Stat(bin); err != nil || info.IsDir() {
		t.Skipf("Chrome executable unavailable: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Headless = true
	cfg.Launch = []string{bin, "--no-sandbox", "--disable-dev-shm-usage"}
	cfg.NavigationTimeoutMs = 10000
	cfg.Reaper.TimeoutMs = 10000
	return cfg
}

func assertNoChromeProfile(t *testing.T, dir string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	processes, err := DefaultProcessQuerier().QueryChromeProcesses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, process := range processes {
		if strings.Contains(process.CommandLine, dir) || profileIdentity(extractUserDataDir(process.CommandLine)) == profileIdentity(dir) {
			t.Fatalf("Chrome PID %d still carries this run's temporary profile", process.PID)
		}
	}
}

type onlyProfileQuerier struct {
	ProcessQuerier
	dir string
}

func (q onlyProfileQuerier) QueryChromeProcesses(ctx context.Context) ([]ProcessInfo, error) {
	processes, err := q.ProcessQuerier.QueryChromeProcesses(ctx)
	if err != nil {
		return nil, err
	}
	var owned []ProcessInfo
	for _, process := range processes {
		dir := extractUserDataDir(process.CommandLine)
		if q.dir != "" && profileIdentity(dir) == profileIdentity(q.dir) {
			owned = append(owned, process)
			continue
		}
		if !isTemporaryUserDataDir(dir) {
			continue
		}
		if data, err := os.ReadFile(filepath.Join(dir, ".codenerd-owner.json")); err == nil {
			var owner temporaryProfileOwner
			if json.Unmarshal(data, &owner) == nil && owner.LauncherPID == os.Getpid() {
				owned = append(owned, process)
			}
		}
	}
	return owned, nil
}

func TestBrowserProcessLifecycle_LiveShutdown(t *testing.T) {
	cfg := liveReaperConfig(t)
	m := NewSessionManagerWithSink(cfg, nil)
	// Exercise Start without scavenging temporary browsers from another run.
	m.SetProcessQuerier(onlyProfileQuerier{ProcessQuerier: DefaultProcessQuerier()})
	t.Cleanup(func() {
		if err := m.Shutdown(context.Background()); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Skipf("Chrome cannot launch on this host: %v", err)
	}
	m.mu.RLock()
	record := m.browsers[m.defaultID]
	pid, dir, temporary := record.pid, record.userDataDir, record.tempDir
	m.mu.RUnlock()
	if pid <= 1 || dir == "" || !temporary {
		t.Fatalf("launch ownership missing: pid=%d temporary=%v", pid, temporary)
	}
	processes, err := DefaultProcessQuerier().QueryChromeProcesses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, process := range processes {
		if process.PID == pid && strings.Contains(process.CommandLine, dir) {
			found = true
		}
	}
	if !found {
		t.Fatal("anti-vacuity: launched Chrome/profile absent from process inspection")
	}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertNoChromeProfile(t, dir)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("temporary profile retained: %v", err)
	}
}

func TestNavigate_SameURL_Reloads(t *testing.T) {
	cfg := liveReaperConfig(t)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/page" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprintf(w, "<!doctype html><title>request-%d</title><body>parsed</body>", requests.Add(1))
	}))
	defer server.Close()
	m := NewSessionManagerWithSink(cfg, nil)
	m.SetProcessQuerier(onlyProfileQuerier{ProcessQuerier: DefaultProcessQuerier()})
	t.Cleanup(func() {
		if err := m.Shutdown(context.Background()); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Skipf("Chrome cannot launch: %v", err)
	}
	session, err := m.CreateTab(ctx, "", server.URL+"/page", false)
	if err != nil {
		t.Fatal(err)
	}
	before := requests.Load()
	if before < 1 {
		t.Fatal("initial document was not fetched")
	}
	if err := m.Navigate(ctx, session.ID, server.URL+"/page"); err != nil {
		t.Fatal(err)
	}
	if requests.Load() <= before {
		t.Fatal("same-URL navigation did not reload the document")
	}
	page, _ := m.Page(session.ID)
	result, err := page.Context(ctx).Eval(`() => document.readyState`)
	if err != nil {
		t.Fatal(err)
	}
	if state := result.Value.Str(); state != "interactive" && state != "complete" {
		t.Fatalf("document not parsed: %s", state)
	}
}

type orphanHandoff struct {
	PID int
	Dir string
}

func TestCreateTab_LiveReturnsBeforeBackgroundResourceLoads(t *testing.T) {
	cfg := liveReaperConfig(t)
	cfg.NavigationTimeoutMs = 5000
	requested, release := make(chan struct{}, 1), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow.png" {
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			select {
			case requested <- struct{}{}:
			default:
			}
			select {
			case <-release:
			case <-r.Context().Done():
			}
			return
		}
		fmt.Fprint(w, `<!doctype html><title>parsed</title><body><img src="/slow.png">ready</body>`)
	}))
	defer server.Close()
	defer close(release)
	m := NewSessionManagerWithSink(cfg, nil)
	m.SetProcessQuerier(onlyProfileQuerier{ProcessQuerier: DefaultProcessQuerier()})
	t.Cleanup(func() {
		if err := m.Shutdown(context.Background()); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Skipf("Chrome cannot launch: %v", err)
	}
	type creation struct {
		session *Session
		err     error
	}
	created := make(chan creation, 1)
	go func() { session, err := m.CreateTab(ctx, "", server.URL, false); created <- creation{session, err} }()
	select {
	case <-requested:
	case <-ctx.Done():
		t.Fatal("background resource was not requested")
	}
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case result := <-created:
		if result.err != nil {
			t.Fatal(result.err)
		}
		page, _ := m.Page(result.session.ID)
		state, err := page.Context(ctx).Eval(`() => document.readyState`)
		if err != nil || state.Value.Str() != "interactive" {
			t.Fatalf("expected parsed document with pending load: %v, %v", state, err)
		}
	case <-timer.C:
		t.Fatal("CreateTab waited for the deliberately blocked background load")
	}
}

// The helper launches one owned temporary browser and deliberately exits before
// cleanup. The parent test never terminates an unrelated launcher or profile.
func TestReaperOrphanHelper(t *testing.T) {
	if os.Getenv("CODENERD_BROWSER_REAPER_HELPER") != "1" {
		return
	}
	cfg := liveReaperConfig(t)
	m := NewSessionManagerWithSink(cfg, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_, pid, dir, temporary, err := m.launchControlURL(ctx, false)
	if err != nil || !temporary || pid <= 1 {
		t.Fatalf("helper Chrome launch failed: %v", err)
	}
	data, err := json.Marshal(orphanHandoff{PID: pid, Dir: dir})
	if err == nil {
		err = os.WriteFile(os.Getenv("CODENERD_BROWSER_REAPER_HANDOFF"), data, 0o600)
	}
	if err != nil {
		_ = m.cleanupBrowserProcess(pid, dir, temporary)
		t.Fatal(err)
	}
	os.Exit(0)
}

func TestBrowserProcessLifecycle_LiveStartupReapsPlantedOrphan(t *testing.T) {
	cfg := liveReaperConfig(t)
	handoffPath := filepath.Join(t.TempDir(), "orphan.json")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestReaperOrphanHelper$")
	cmd.Env = append(os.Environ(), "CODENERD_BROWSER_REAPER_HELPER=1", "CODENERD_BROWSER_REAPER_HANDOFF="+handoffPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("Chrome helper cannot launch: %v (%s)", err, out)
	}
	data, err := os.ReadFile(handoffPath)
	if err != nil {
		t.Fatal(err)
	}
	var orphan orphanHandoff
	if err := json.Unmarshal(data, &orphan); err != nil {
		t.Fatal(err)
	}
	if _, err := temporaryProfilePath(orphan.Dir); err != nil || orphan.PID <= 1 {
		t.Fatalf("invalid planted orphan: %v", err)
	}
	q := DefaultProcessQuerier()
	t.Cleanup(func() {
		m := NewSessionManagerWithSink(cfg, nil)
		if err := m.cleanupBrowserProcess(orphan.PID, orphan.Dir, true); err != nil {
			t.Errorf("planted orphan cleanup: %v", err)
		}
	})
	if !q.IsProcessAlive(orphan.PID) {
		t.Fatal("anti-vacuity: planted orphan is not alive")
	}
	if q.IsProcessAlive(cmd.Process.Pid) {
		t.Fatal("helper launcher still alive")
	}
	m := NewSessionManagerWithSink(cfg, nil)
	m.SetProcessQuerier(onlyProfileQuerier{ProcessQuerier: q, dir: orphan.Dir})
	t.Cleanup(func() {
		if err := m.Shutdown(context.Background()); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	if err := m.Start(ctx); err != nil {
		t.Skipf("Chrome cannot launch: %v", err)
	}
	if m.ReapedOrphans() != 1 {
		t.Fatalf("startup reaped %d trees, want this run's single orphan", m.ReapedOrphans())
	}
	assertNoChromeProfile(t, orphan.Dir)
	if _, err := os.Stat(orphan.Dir); !os.IsNotExist(err) {
		t.Fatalf("orphan profile retained: %v", err)
	}
}
