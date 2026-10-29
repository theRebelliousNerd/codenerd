package browser

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

// findChromeBinary locates a browser for live tests.
//
// rod's launcher.LookPath only knows the standard OS install locations. CI
// images and this repo's dev sandbox ship Chromium under
// PLAYWRIGHT_BROWSERS_PATH instead, so the live coverage silently skipped
// everywhere it mattered.
func findChromeBinary() (string, bool) {
	if explicit := os.Getenv("NERD_TEST_CHROME_BIN"); explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit, true
		}
	}
	if path, found := launcher.LookPath(); found && path != "" {
		return path, true
	}
	root := os.Getenv("PLAYWRIGHT_BROWSERS_PATH")
	if root == "" {
		return "", false
	}
	for _, pattern := range []string{
		"chromium-*/chrome-linux/chrome",
		"chromium_headless_shell-*/chrome-linux/headless_shell",
		"chromium-*/chrome-mac/Chromium.app/Contents/MacOS/Chromium",
	} {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			if info, statErr := os.Stat(match); statErr == nil && !info.IsDir() {
				return match, true
			}
		}
	}
	return "", false
}

// liveBrowserConfig returns a headless config bound to a discovered browser, or
// skips the test when none is installed.
func liveBrowserConfig(t *testing.T) Config {
	t.Helper()
	bin, found := findChromeBinary()
	if !found {
		t.Skip("no Chrome/Chromium binary found; set NERD_TEST_CHROME_BIN or PLAYWRIGHT_BROWSERS_PATH")
	}
	cfg := DefaultConfig()
	cfg.Headless = true
	// --no-sandbox is required inside the unprivileged container the tests run
	// in; it only ever applies to this throwaway instance.
	cfg.Launch = []string{bin, "--no-sandbox", "--disable-dev-shm-usage"}
	cfg.NavigationTimeoutMs = 15000
	cfg.EventThrottleMs = 10
	return cfg
}

const honeypotTestPage = `<html><body>
	<a id="real" href="/page2">Real link</a>
	<a id="trap" href="/trap" style="display:none">Hidden trap</a>
	<a id="bait" href="/honeypot/collect">Bait link</a>
	<input id="user" type="text" />
	<input id="honey" type="text" style="position:absolute;left:-9999px;top:0" />
	<button id="go">Go</button>
</body></html>`

// TestHoneypotGuard_WhenElementIsHoneypot_ShouldRefuseInteraction is the live
// proof that detection gates something. Before the guard, Click and Type
// resolved a selector and interacted with whatever it matched; the honeypot
// rules were advisory output nobody consulted.
func TestHoneypotGuard_WhenElementIsHoneypot_ShouldRefuseInteraction(t *testing.T) {
	cfg := liveBrowserConfig(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, honeypotTestPage)
	}))
	defer ts.Close()

	engine := newBrowserTestEngine(t)
	manager := NewSessionManager(cfg, engine)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	defer func() { _ = manager.Shutdown(context.Background()) }()

	if manager.HoneypotDetector() == nil {
		t.Fatal("a manager built over a mangle.Engine must expose a honeypot detector")
	}

	session, err := manager.CreateSession(ctx, ts.URL)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// A hidden anchor is refused before rod ever tries to click it.
	err = manager.Click(ctx, session.ID, "#trap")
	if !errors.Is(err, ErrHoneypotBlocked) {
		t.Errorf("click on hidden trap = %v, want ErrHoneypotBlocked", err)
	}

	// An off-screen input is visible to rod and would otherwise be typed into.
	err = manager.Type(ctx, session.ID, "#honey", "secret")
	if !errors.Is(err, ErrHoneypotBlocked) {
		t.Errorf("type into off-screen input = %v, want ErrHoneypotBlocked", err)
	}

	// A visible, on-screen bait link is caught by URL shape alone.
	err = manager.Click(ctx, session.ID, "#bait")
	if !errors.Is(err, ErrHoneypotBlocked) {
		t.Errorf("click on bait link = %v, want ErrHoneypotBlocked", err)
	}

	// Ordinary elements are untouched.
	if err := manager.Type(ctx, session.ID, "#user", "hello"); err != nil {
		t.Errorf("type into normal input: %v", err)
	}
	if err := manager.Click(ctx, session.ID, "#go"); err != nil {
		t.Errorf("click on normal button: %v", err)
	}

	// The refusal is kernel-visible, not just a Go error string.
	if blocked := engine.QueryFacts("interaction_blocked", session.ID); len(blocked) == 0 {
		t.Error("expected interaction_blocked facts for the refused interactions")
	}
}

// TestHoneypotGuard_WhenDisabled_ShouldAllowInteraction keeps the operator
// escape hatch working.
func TestHoneypotGuard_WhenDisabled_ShouldAllowInteraction(t *testing.T) {
	cfg := liveBrowserConfig(t)
	cfg.HoneypotGuard = HoneypotGuardOff
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, honeypotTestPage)
	}))
	defer ts.Close()

	manager := NewSessionManager(cfg, newBrowserTestEngine(t))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	defer func() { _ = manager.Shutdown(context.Background()) }()

	session, err := manager.CreateSession(ctx, ts.URL)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := manager.Type(ctx, session.ID, "#honey", "secret"); err != nil {
		t.Errorf("guard=off must not refuse: %v", err)
	}
}

// TestHoneypotDetector_WhenAnalyzingLivePage_ShouldFlagOnlyTraps exercises the
// full measure-then-derive path against a real DOM, including the geometry
// facts that were dead while coordinates were asserted as strings.
func TestHoneypotDetector_WhenAnalyzingLivePage_ShouldFlagOnlyTraps(t *testing.T) {
	cfg := liveBrowserConfig(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, honeypotTestPage)
	}))
	defer ts.Close()

	engine := newBrowserTestEngine(t)
	manager := NewSessionManager(cfg, engine)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	defer func() { _ = manager.Shutdown(context.Background()) }()

	session, err := manager.CreateSession(ctx, ts.URL)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	page, ok := manager.Page(session.ID)
	if !ok {
		t.Fatal("no page for session")
	}

	detector := manager.HoneypotDetector()
	links, err := detector.GetAllLinksWithAnalysis(page)
	if err != nil {
		t.Fatalf("analyze links: %v", err)
	}
	if len(links) != 3 {
		t.Fatalf("analyzed %d links, want 3", len(links))
	}
	byHref := make(map[string][]string, len(links))
	flagged := make(map[string]bool, len(links))
	for _, link := range links {
		byHref[link.Href] = link.HoneypotReasons
		flagged[link.Href] = link.IsHoneypot
	}
	if flagged["/page2"] {
		t.Errorf("real link was flagged: %v", byHref["/page2"])
	}
	if !flagged["/trap"] {
		t.Error("display:none link was not flagged")
	}
	if !flagged["/honeypot/collect"] {
		t.Error("bait URL was not flagged")
	}

	safe, err := detector.GetSafeLinks(page)
	if err != nil {
		t.Fatalf("safe links: %v", err)
	}
	if len(safe) != 1 || safe[0].Href != "/page2" {
		t.Errorf("safe links = %+v, want only /page2", safe)
	}
}
