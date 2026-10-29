package research

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
)

// pinResearchDefaults installs the default research policy for the test and
// restores whatever was installed before. Tests that read paging windows or
// timeouts use it so a custom policy installed by another test cannot leak
// in through the process-wide singleton.
func pinResearchDefaults(t *testing.T) config.ResearchPolicy {
	t.Helper()
	prev := config.ResolvedResearchPolicy()
	def, err := config.DefaultResearchConfig().Resolve()
	if err != nil {
		t.Fatalf("the default research config does not resolve: %v", err)
	}
	config.SetResearchPolicy(def)
	t.Cleanup(func() { config.SetResearchPolicy(prev) })
	return def
}

// installResearchPolicy installs a mutated copy of the default policy and
// restores the previous one.
func installResearchPolicy(t *testing.T, mutate func(*config.ResearchPolicy)) config.ResearchPolicy {
	t.Helper()
	prev := config.ResolvedResearchPolicy()
	p, err := config.DefaultResearchConfig().Resolve()
	if err != nil {
		t.Fatalf("the default research config does not resolve: %v", err)
	}
	mutate(&p)
	config.SetResearchPolicy(p)
	t.Cleanup(func() { config.SetResearchPolicy(prev) })
	return p
}

// A hanging server the test releases: the handler blocks until the test
// ends, so Close never waits out a sleep.
func hangingServer(t *testing.T) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	// LIFO: the release runs first so Close never waits out a handler.
	t.Cleanup(ts.Close)
	t.Cleanup(func() { close(release) })
	return ts
}

func TestResolveBrowserExtractMaxChars_ReadsInstalledPolicy(t *testing.T) {
	def := pinResearchDefaults(t)
	if got := resolveBrowserExtractMaxChars(map[string]any{}); got != def.BrowserExtractMaxChars {
		t.Errorf("absent max_chars = %d, want default %d", got, def.BrowserExtractMaxChars)
	}
	if def.BrowserExtractMaxChars != 8000 || def.BrowserExtractMaxCharsCap != 32000 {
		t.Fatalf("default window = %d/%d, want 8000/32000", def.BrowserExtractMaxChars, def.BrowserExtractMaxCharsCap)
	}
	installResearchPolicy(t, func(p *config.ResearchPolicy) {
		p.BrowserExtractMaxChars = 111
		p.BrowserExtractMaxCharsCap = 222
	})
	if got := resolveBrowserExtractMaxChars(map[string]any{}); got != 111 {
		t.Errorf("absent max_chars = %d, want installed 111", got)
	}
	if got := resolveBrowserExtractMaxChars(map[string]any{"max_chars": 100000}); got != 222 {
		t.Errorf("over-cap max_chars = %d, want installed cap 222", got)
	}
	if got := resolveBrowserExtractMaxChars(map[string]any{"max_chars": 150}); got != 150 {
		t.Errorf("max_chars = %d, want honored 150", got)
	}
}

func TestWithBrowserExtractDeadline_ReadsInstalledPolicy(t *testing.T) {
	pinResearchDefaults(t)
	ctx, cancel := withBrowserExtractDeadline(context.Background())
	defer cancel()
	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("no deadline on the extraction context")
	}
	if remaining := time.Until(dl); remaining < 9*time.Second || remaining > 10*time.Second {
		t.Errorf("default extraction deadline in %v, want ~10s", remaining)
	}
	installResearchPolicy(t, func(p *config.ResearchPolicy) {
		p.BrowserExtractTimeout = 150 * time.Millisecond
	})
	ctx2, cancel2 := withBrowserExtractDeadline(context.Background())
	defer cancel2()
	dl2, ok := ctx2.Deadline()
	if !ok {
		t.Fatal("no deadline on the extraction context after install")
	}
	if remaining := time.Until(dl2); remaining <= 0 || remaining > 150*time.Millisecond {
		t.Errorf("installed extraction deadline in %v, want ~150ms", remaining)
	}
}

func TestBrowserExtractTool_SchemaReadsInstalledPolicy(t *testing.T) {
	def := pinResearchDefaults(t)
	tool := BrowserExtractTool()
	prop := tool.Schema.Properties["max_chars"]
	if prop.Default != def.BrowserExtractMaxChars {
		t.Errorf("schema default = %v, want %d", prop.Default, def.BrowserExtractMaxChars)
	}
	if !strings.Contains(prop.Description, "8000") || !strings.Contains(prop.Description, "32000") {
		t.Errorf("schema description names no window: %q", prop.Description)
	}
	installResearchPolicy(t, func(p *config.ResearchPolicy) {
		p.BrowserExtractMaxChars = 111
		p.BrowserExtractMaxCharsCap = 222
		p.BrowserExtractTimeout = 42 * time.Second
	})
	tool = BrowserExtractTool()
	prop = tool.Schema.Properties["max_chars"]
	if prop.Default != 111 {
		t.Errorf("schema default = %v, want installed 111", prop.Default)
	}
	if !strings.Contains(prop.Description, "111") || !strings.Contains(prop.Description, "222") {
		t.Errorf("schema description did not follow the install: %q", prop.Description)
	}
	if !strings.Contains(tool.Description, "42s") {
		t.Errorf("tool description did not follow the install: %q", tool.Description)
	}
}

// The web_fetch bound is the installed policy: a hanging server must fail
// fast, and the failure must be the deadline, not the hang.
func TestExecuteWebFetch_TimeoutReadsInstalledPolicy(t *testing.T) {
	withBoundResearchRuntime(t)
	installResearchPolicy(t, func(p *config.ResearchPolicy) {
		p.WebFetchTimeout = 50 * time.Millisecond
	})
	ts := hangingServer(t)
	start := time.Now()
	_, err := executeWebFetch(context.Background(), map[string]any{"url": ts.URL})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("a hanging fetch succeeded under a 50ms bound")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("error = %v, want the deadline", err)
	}
	if elapsed > 4*time.Second {
		t.Errorf("fetch took %v: the bound did not bite", elapsed)
	}
}

// Same bound, one layer down: context7_fetch reads research.context7_timeout.
func TestFetchURL_TimeoutReadsInstalledPolicy(t *testing.T) {
	withBoundResearchRuntime(t)
	installResearchPolicy(t, func(p *config.ResearchPolicy) {
		p.Context7Timeout = 50 * time.Millisecond
	})
	ts := hangingServer(t)
	start := time.Now()
	_, err := fetchURL(context.Background(), ts.URL)
	if elapsed := time.Since(start); err == nil || elapsed > 4*time.Second {
		t.Errorf("fetchURL err=%v elapsed=%v: want a fast deadline failure", err, elapsed)
	}
}

// Same bound for web_search, pointed at a local server.
func TestSearchDuckDuckGo_TimeoutReadsInstalledPolicy(t *testing.T) {
	withBoundResearchRuntime(t)
	installResearchPolicy(t, func(p *config.ResearchPolicy) {
		p.WebSearchTimeout = 50 * time.Millisecond
	})
	ts := hangingServer(t)
	prevURL := duckDuckGoSearchURL
	duckDuckGoSearchURL = ts.URL + "/html/?q=%s"
	t.Cleanup(func() { duckDuckGoSearchURL = prevURL })
	start := time.Now()
	_, _, err := searchDuckDuckGo(context.Background(), "rods", 10)
	if elapsed := time.Since(start); err == nil || elapsed > 4*time.Second {
		t.Errorf("searchDuckDuckGo err=%v elapsed=%v: want a fast deadline failure", err, elapsed)
	}
}

// The reason paging windows are the installed policy in both views.
func TestReasonWindows_ReadInstalledPolicy(t *testing.T) {
	def := pinResearchDefaults(t)
	if def.BrowserReasonItems != 20 || def.BrowserReasonCompactItems != 10 {
		t.Fatalf("default reason windows = %d/%d, want 20/10", def.BrowserReasonItems, def.BrowserReasonCompactItems)
	}
	if _, maxItems, _, err := normalizeReasonView(map[string]any{}); err != nil || maxItems != 20 {
		t.Errorf("absent max_items = %d, %v; want 20", maxItems, err)
	}
	if got := reasonWindow(map[string]any{}, "compact", 20); got != 10 {
		t.Errorf("absent compact window = %d, want 10", got)
	}
	installResearchPolicy(t, func(p *config.ResearchPolicy) {
		p.BrowserReasonItems = 7
		p.BrowserReasonCompactItems = 5
	})
	if _, maxItems, _, err := normalizeReasonView(map[string]any{}); err != nil || maxItems != 7 {
		t.Errorf("absent max_items = %d, %v; want installed 7", maxItems, err)
	}
	if got := reasonWindow(map[string]any{}, "compact", 7); got != 5 {
		t.Errorf("absent compact window = %d, want installed 5", got)
	}
	if _, maxItems, _, err := normalizeReasonView(map[string]any{"max_items": 25}); err != nil || maxItems != 25 {
		t.Errorf("explicit max_items = %d, %v; want honored 25", maxItems, err)
	}
}

// The mangle and reason schemas advertise the installed window.
func TestReasonToolSchemas_ReadInstalledPolicy(t *testing.T) {
	pinResearchDefaults(t)
	if got := BrowserMangleTool().Schema.Properties["max_items"].Default; got != 20 {
		t.Errorf("browser_mangle schema default = %v, want 20", got)
	}
	if got := BrowserReasonTool().Schema.Properties["max_items"].Default; got != 20 {
		t.Errorf("browser_reason schema default = %v, want 20", got)
	}
	installResearchPolicy(t, func(p *config.ResearchPolicy) {
		p.BrowserReasonItems = 7
	})
	if got := BrowserMangleTool().Schema.Properties["max_items"].Default; got != 7 {
		t.Errorf("browser_mangle schema default = %v, want installed 7", got)
	}
	if got := BrowserReasonTool().Schema.Properties["max_items"].Default; got != 7 {
		t.Errorf("browser_reason schema default = %v, want installed 7", got)
	}
}
