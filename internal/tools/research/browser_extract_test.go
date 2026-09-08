package research

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"codenerd/internal/browser"

	"github.com/go-rod/rod/lib/launcher"
)

func findBrowserExtractChromeBinary() (string, bool) {
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

func requireBrowserExtractLiveManager(t *testing.T, ctx context.Context) *browser.SessionManager {
	t.Helper()
	bin, found := findBrowserExtractChromeBinary()
	if !found {
		t.Skip("no Chrome/Chromium binary found; set NERD_TEST_CHROME_BIN or PLAYWRIGHT_BROWSERS_PATH")
	}
	cfg := browser.DefaultConfig()
	cfg.Headless = true
	cfg.Launch = []string{bin, "--no-sandbox", "--disable-dev-shm-usage"}
	cfg.NavigationTimeoutMs = 15000
	cfg.EventThrottleMs = 10
	mgr := browser.NewSessionManagerWithSink(cfg, nil)
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("discovered Chrome failed to start: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Shutdown(context.Background()) })
	return mgr
}

func serveBrowserExtractPage(t *testing.T, html string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	}))
	t.Cleanup(ts.Close)
	return ts
}

func createBrowserExtractSession(t *testing.T, ctx context.Context, mgr *browser.SessionManager, url string) string {
	t.Helper()
	session, err := mgr.CreateSession(ctx, url)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return session.ID
}

func TestBrowserExtractResolveMaxChars(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
		want int
	}{
		{name: "missing selects default", args: map[string]any{}, want: defaultBrowserExtractMaxChars},
		{name: "zero selects default", args: map[string]any{"max_chars": 0}, want: defaultBrowserExtractMaxChars},
		{name: "negative selects default", args: map[string]any{"max_chars": -5}, want: defaultBrowserExtractMaxChars},
		{name: "small honored", args: map[string]any{"max_chars": 100}, want: 100},
		{name: "float64 honored", args: map[string]any{"max_chars": float64(250)}, want: 250},
		{name: "over cap clamped", args: map[string]any{"max_chars": maxBrowserExtractMaxChars + 1000}, want: maxBrowserExtractMaxChars},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveBrowserExtractMaxChars(tc.args); got != tc.want {
				t.Fatalf("max_chars = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestBoundBrowserExtractChars(t *testing.T) {
	t.Run("exact limit not marked", func(t *testing.T) {
		got, truncated, total := boundBrowserExtractChars("abc", 3)
		if truncated || total != 3 || got != "abc" {
			t.Fatalf("exact input must not truncate: got=%q truncated=%v total=%d", got, truncated, total)
		}
	})
	t.Run("longer marks truncation on rune boundary", func(t *testing.T) {
		// Mix ASCII with multi-byte runes so a byte split would be invalid UTF-8.
		input := "a🌟b🌟cdefgh"
		got, truncated, total := boundBrowserExtractChars(input, 4)
		if !truncated {
			t.Fatal("expected truncation")
		}
		if total != len([]rune(input)) {
			t.Fatalf("total runes = %d, want %d", total, len([]rune(input)))
		}
		if !utf8.ValidString(got) {
			t.Fatalf("truncated output split UTF-8: %q", got)
		}
		if got != string([]rune(input)[:4]) {
			t.Fatalf("truncated output = %q, want %q", got, string([]rune(input)[:4]))
		}
	})
}

func TestBrowserExtractRedactionPreservedAfterBounding(t *testing.T) {
	mgr := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	raw := "hello password=live-secret-123 world " + strings.Repeat("x", 9000)
	sanitized := mgr.SanitizeForEvidence(raw)
	if strings.Contains(sanitized, "live-secret-123") {
		t.Fatalf("sanitizer leaked secret: %q", sanitized[:200])
	}
	bounded, truncated, _ := boundBrowserExtractChars(sanitized, 100)
	if !truncated {
		t.Fatal("expected truncation for oversized evidence")
	}
	if !utf8.ValidString(bounded) {
		t.Fatal("bounded evidence split UTF-8")
	}
	if strings.Contains(bounded, "live-secret-123") || !strings.Contains(sanitized, "[REDACTED]") {
		t.Fatalf("redaction not preserved: %q", bounded)
	}
}

func TestBrowserExtractUnknownSession(t *testing.T) {
	mgr := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	SetBrowserManager(mgr)
	defer ClearBrowserManager(mgr)
	if _, err := BrowserExtractTool().Execute(context.Background(), map[string]any{"session_id": "no-such-session"}); err == nil {
		t.Fatal("expected error for unknown session")
	}
}

func TestBrowserExtractLiveSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	mgr := requireBrowserExtractLiveManager(t, ctx)
	ts := serveBrowserExtractPage(t, `<html><body><h1 id="title">Hello Extract</h1><p>Some body text</p></body></html>`)
	sessionID := createBrowserExtractSession(t, ctx, mgr, ts.URL)
	SetBrowserManager(mgr)
	defer ClearBrowserManager(mgr)

	out, err := BrowserExtractTool().Execute(ctx, map[string]any{"session_id": sessionID, "selector": "body"})
	if err != nil {
		t.Fatalf("extract body: %v", err)
	}
	if !strings.Contains(out, "Hello Extract") {
		t.Fatalf("extract missing expected text: %q", out)
	}
	if strings.Contains(out, "[truncated") {
		t.Fatalf("small page must not truncate: %q", out)
	}
	if !utf8.ValidString(out) {
		t.Fatalf("extract output invalid UTF-8: %q", out)
	}
}

func TestBrowserExtractLiveMissingSelectorCancelsPromptly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	mgr := requireBrowserExtractLiveManager(t, ctx)
	ts := serveBrowserExtractPage(t, `<html><body><h1>Present</h1></body></html>`)
	sessionID := createBrowserExtractSession(t, ctx, mgr, ts.URL)
	SetBrowserManager(mgr)
	defer ClearBrowserManager(mgr)

	callCtx, callCancel := context.WithTimeout(ctx, 3*time.Second)
	defer callCancel()
	start := time.Now()
	_, err := BrowserExtractTool().Execute(callCtx, map[string]any{"session_id": sessionID, "selector": "#does-not-exist-xyz"})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error for missing selector")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("missing selector must preserve context cancellation, got: %v", err)
	}
	// Caller gave 3s; our fallback is 10s. Anything near or beyond the fallback
	// means the caller deadline was ignored and the lookup hung.
	if elapsed >= 8*time.Second {
		t.Fatalf("missing selector did not cancel promptly: elapsed=%v err=%v", elapsed, err)
	}
}

func TestBrowserExtractLiveBoundedTruncationAndRedaction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	mgr := requireBrowserExtractLiveManager(t, ctx)
	longText := strings.Repeat("lorem ", 2000)
	html := `<html><body><div id="content">hello 🌟 password=live-secret-123 ` + longText + `</div></body></html>`
	ts := serveBrowserExtractPage(t, html)
	sessionID := createBrowserExtractSession(t, ctx, mgr, ts.URL)
	SetBrowserManager(mgr)
	defer ClearBrowserManager(mgr)

	out, err := BrowserExtractTool().Execute(ctx, map[string]any{"session_id": sessionID, "selector": "#content", "max_chars": 100})
	if err != nil {
		t.Fatalf("bounded extract: %v", err)
	}
	if !strings.Contains(out, "[truncated") {
		t.Fatalf("oversized extract must mark truncation: %q", out)
	}
	if strings.Contains(out, "live-secret-123") {
		t.Fatalf("bounded extract leaked secret: %q", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("bounded extract must preserve redaction: %q", out)
	}
	if !utf8.ValidString(out) {
		t.Fatalf("bounded extract split UTF-8: %q", out)
	}
}

func TestBrowserExtractLiveIncludeHTML(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	mgr := requireBrowserExtractLiveManager(t, ctx)
	ts := serveBrowserExtractPage(t, `<html><body><h1 id="title">HTML Title</h1></body></html>`)
	sessionID := createBrowserExtractSession(t, ctx, mgr, ts.URL)
	SetBrowserManager(mgr)
	defer ClearBrowserManager(mgr)

	out, err := BrowserExtractTool().Execute(ctx, map[string]any{"session_id": sessionID, "selector": "#title", "include_html": true})
	if err != nil {
		t.Fatalf("extract with html: %v", err)
	}
	if !strings.Contains(out, "HTML Title") {
		t.Fatalf("html extract missing text: %q", out)
	}
	if !strings.Contains(out, "--- html ---") {
		t.Fatalf("include_html=true must include html section: %q", out)
	}
	plain, err := BrowserExtractTool().Execute(ctx, map[string]any{"session_id": sessionID, "selector": "#title"})
	if err != nil {
		t.Fatalf("plain extract: %v", err)
	}
	if strings.Contains(plain, "--- html ---") {
		t.Fatalf("include_html=false must not include html section: %q", plain)
	}
	bounded, err := BrowserExtractTool().Execute(ctx, map[string]any{"session_id": sessionID, "selector": "#title", "include_html": true, "max_chars": 24})
	if err != nil {
		t.Fatal(err)
	}
	content, notice, found := strings.Cut(bounded, "\n...[truncated:")
	if !found || notice == "" || utf8.RuneCountInString(content) > 24 {
		t.Fatalf("text and HTML must share one content budget: %q", bounded)
	}
}

func TestBrowserExtractLiveLongParentStillBoundsMissingSelector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	mgr := requireBrowserExtractLiveManager(t, ctx)
	ts := serveBrowserExtractPage(t, `<html><body>Present</body></html>`)
	sessionID := createBrowserExtractSession(t, ctx, mgr, ts.URL)
	SetBrowserManager(mgr)
	defer ClearBrowserManager(mgr)
	start := time.Now()
	_, err := BrowserExtractTool().Execute(ctx, map[string]any{"session_id": sessionID, "selector": "#absent"})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > defaultBrowserExtractTimeout+5*time.Second {
		t.Fatalf("long parent must not extend the extraction deadline: elapsed=%v err=%v", time.Since(start), err)
	}
	if ctx.Err() != nil {
		t.Fatal("extraction timeout canceled the owning session")
	}
	if out, err := BrowserExtractTool().Execute(ctx, map[string]any{"session_id": sessionID}); err != nil || !strings.Contains(out, "Present") {
		t.Fatalf("session unusable after one timed-out extraction: %q, %v", out, err)
	}
}
