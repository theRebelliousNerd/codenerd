package research

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"codenerd/internal/browser"
	"codenerd/internal/config"
)

func TestMain(m *testing.M) {
	code := m.Run()
	shutdownResearchBrowser()
	os.Exit(code)
}

// withBoundResearchRuntime binds a sentinel manager and kernel, then starts
// the headless research Chrome. A launch failure skips: the suite must not
// pretend a missing browser is a passed read. The sentinel is not a page
// reader; research.browser_headless (default true) uses the separate process.
func withBoundResearchRuntime(t *testing.T) {
	t.Helper()
	if !config.ResolvedResearchPolicy().BrowserHeadless {
		t.Fatal("withBoundResearchRuntime requires research.browser_headless true; pin the default policy first")
	}
	browserMgrMu.Lock()
	prevMgr := browserMgr
	prevKernel := browserKernel
	prevOwner := browserKernelOwner
	browserMgrMu.Unlock()

	sentinel := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	SetBrowserRuntime(sentinel, &browserReasoningKernel{})
	t.Cleanup(func() {
		browserMgrMu.Lock()
		defer browserMgrMu.Unlock()
		if browserMgr == sentinel {
			browserMgr = prevMgr
			browserKernel = prevKernel
			browserKernelOwner = prevOwner
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := researchPageManager(ctx); err != nil {
		var launch *chromeLaunchError
		if errors.As(err, &launch) {
			t.Skipf("Chrome cannot be launched: %v. Set browser.launch to the Chrome binary.", err)
		}
		t.Fatalf("research browser: %v", err)
	}
}

// pinLocalGitHub points raw GitHub file URLs at a local server so a
// context7 read does not leave the machine.
func pinLocalGitHub(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	withBoundResearchRuntime(t)
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	prev := rawGitHubFileURL
	rawGitHubFileURL = func(owner, repo, ref, path string) string {
		return ts.URL + "/" + owner + "/" + repo + "/" + ref + "/" + strings.TrimPrefix(path, "/")
	}
	t.Cleanup(func() { rawGitHubFileURL = prev })
}

func pinLocalGitHubFiles(t *testing.T, files map[string]string) {
	t.Helper()
	pinLocalGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, body)
	})
}

func researchTabCount(t *testing.T) int {
	t.Helper()
	mgr := researchPageBrowser()
	if mgr == nil {
		t.Fatal("research page browser is not started")
	}
	return len(mgr.List())
}

func TestReadBrowserPage_StaticHTMLAndTabBaseline(t *testing.T) {
	withBoundResearchRuntime(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><body><h1>Hello World</h1><p>Test content.</p></body></html>`)
	}))
	t.Cleanup(ts.Close)

	before := researchTabCount(t)
	page, err := readBrowserPage(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("readBrowserPage: %v", err)
	}
	if got := researchTabCount(t); got != before {
		t.Fatalf("tabs after read = %d, baseline %d", got, before)
	}
	if page.Status != 200 || !strings.Contains(page.HTML, "Hello World") || !strings.Contains(page.Text, "Test content") {
		t.Fatalf("page = %+v", page)
	}

	markdown, err := executeWebFetch(context.Background(), map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("executeWebFetch: %v", err)
	}
	if !strings.Contains(markdown, "# Hello World") || !strings.Contains(markdown, "Test content") {
		t.Fatalf("markdown = %q", markdown)
	}
	if got := researchTabCount(t); got != before {
		t.Fatalf("tabs after web_fetch = %d, baseline %d", got, before)
	}
}

func TestReadBrowserPage_RendersScriptedDOM(t *testing.T) {
	withBoundResearchRuntime(t)
	// The slot stays "loading" until eight DOM mutations have landed, which
	// is past the stable-wait minimum quiet. A reader that returns on the
	// quiet timer without watching dom_updated still sees "loading".
	const doc = `<!doctype html><html><body>
<p id="slot">loading</p>
<script>
let n = 0;
const timer = setInterval(function() {
  n++;
  const s = document.createElement("span");
  s.textContent = "tick-" + n;
  document.body.appendChild(s);
  if (n >= 8) {
    clearInterval(timer);
    document.getElementById("slot").textContent = "rendered-by-js";
  }
}, 80);
</script>
</body></html>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, doc)
	}))
	t.Cleanup(ts.Close)

	before := researchTabCount(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	start := time.Now()
	page, err := readBrowserPage(ctx, ts.URL)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("readBrowserPage: %v", err)
	}
	if researchTabCount(t) != before {
		t.Fatalf("tabs after read = %d, baseline %d", researchTabCount(t), before)
	}
	if strings.Contains(page.Text, "loading") || !strings.Contains(page.Text, "rendered-by-js") {
		t.Fatalf("rendered text = %q after %s", page.Text, elapsed)
	}
	if elapsed < researchPageNetworkIdle {
		t.Fatalf("read returned in %s, before DOM stability could be observed", elapsed)
	}
}

func TestReadBrowserPage_PlainAndMarkdownText(t *testing.T) {
	withBoundResearchRuntime(t)
	const plain = "response body"
	const markdown = "# Heading\n\nParagraph content"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/plain":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(w, plain)
		case "/md":
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			fmt.Fprint(w, markdown)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)

	before := researchTabCount(t)
	page, err := readBrowserPage(context.Background(), ts.URL+"/plain")
	if err != nil {
		t.Fatalf("plain: %v", err)
	}
	if page.Text != plain {
		t.Fatalf("plain text = %q, want %q", page.Text, plain)
	}
	got, err := executeWebFetch(context.Background(), map[string]any{"url": ts.URL + "/md"})
	if err != nil {
		t.Fatalf("markdown fetch: %v", err)
	}
	if got != markdown {
		t.Fatalf("markdown fetch = %q, want %q", got, markdown)
	}
	if researchTabCount(t) != before {
		t.Fatalf("tabs = %d, baseline %d", researchTabCount(t), before)
	}
}

func TestReadBrowserPage_UnstableDOMIsAnError(t *testing.T) {
	withBoundResearchRuntime(t)
	const doc = `<!doctype html><html><body><p id="slot">loading</p><script>
setInterval(function() {
  const s = document.createElement("span");
  s.textContent = "churn";
  document.body.appendChild(s);
}, 80);
</script></body></html>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, doc)
	}))
	t.Cleanup(ts.Close)

	before := researchTabCount(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	page, err := readBrowserPage(ctx, ts.URL)
	if err == nil {
		t.Fatalf("unstable page returned text %q", page.Text)
	}
	if !strings.Contains(err.Error(), "DOM stability") {
		t.Fatalf("error = %v, want a DOM-stability refusal", err)
	}
	if page.HTML != "" || page.Text != "" {
		t.Fatalf("stability failure still extracted a frame: %#v", page)
	}
	if researchTabCount(t) != before {
		t.Fatalf("tabs = %d, baseline %d", researchTabCount(t), before)
	}
}

func TestReadBrowserPage_UnboundRuntimeIsAnError(t *testing.T) {
	browserMgrMu.Lock()
	prevMgr, prevKernel, prevOwner := browserMgr, browserKernel, browserKernelOwner
	browserMgr, browserKernel, browserKernelOwner = nil, nil, nil
	browserMgrMu.Unlock()
	t.Cleanup(func() {
		browserMgrMu.Lock()
		defer browserMgrMu.Unlock()
		if browserMgr == nil && browserKernel == nil {
			browserMgr = prevMgr
			browserKernel = prevKernel
			browserKernelOwner = prevOwner
		}
	})

	_, err := readBrowserPage(context.Background(), "http://127.0.0.1/")
	if err == nil || !strings.Contains(err.Error(), "SetBrowserRuntime") {
		t.Fatalf("error = %v, want the unbound-runtime error", err)
	}
	if getBrowserManager() != nil {
		t.Fatal("unbound read constructed a fallback manager")
	}
}

func TestReadBrowserPage_BoundWithoutKernelIsAnError(t *testing.T) {
	browserMgrMu.Lock()
	prevMgr, prevKernel, prevOwner := browserMgr, browserKernel, browserKernelOwner
	browserMgrMu.Unlock()
	sentinel := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	SetBrowserRuntime(sentinel, nil)
	t.Cleanup(func() {
		browserMgrMu.Lock()
		defer browserMgrMu.Unlock()
		if browserMgr == sentinel {
			browserMgr = prevMgr
			browserKernel = prevKernel
			browserKernelOwner = prevOwner
		}
	})

	_, err := readBrowserPage(context.Background(), "http://127.0.0.1/")
	if err == nil || !strings.Contains(err.Error(), "without a kernel") {
		t.Fatalf("error = %v, want the missing-kernel error", err)
	}
}

func TestResearchPageManager_HeadlessFalseUsesBoundManager(t *testing.T) {
	browserMgrMu.Lock()
	prevMgr, prevKernel, prevOwner := browserMgr, browserKernel, browserKernelOwner
	browserMgrMu.Unlock()
	sentinel := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	SetBrowserRuntime(sentinel, &browserReasoningKernel{})
	t.Cleanup(func() {
		browserMgrMu.Lock()
		defer browserMgrMu.Unlock()
		if browserMgr == sentinel {
			browserMgr = prevMgr
			browserKernel = prevKernel
			browserKernelOwner = prevOwner
		}
	})
	installResearchPolicy(t, func(p *config.ResearchPolicy) { p.BrowserHeadless = false })

	mgr, err := researchPageManager(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if mgr != sentinel {
		t.Fatal("research.browser_headless false did not use the bound manager")
	}
}

func TestResearchPageReadsDoNotUseNetHTTP(t *testing.T) {
	forbidden := []string{"http.DefaultClient", "http.Get(", "http.NewRequest"}
	for _, name := range []string{"web_fetch.go", "web_search.go", "context7.go", "browser_read.go"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, needle := range forbidden {
			if strings.Contains(text, needle) {
				t.Errorf("%s still contains %s", name, needle)
			}
		}
	}
}

func TestReadBrowserPage_DoesNotUseDefaultClient(t *testing.T) {
	withBoundResearchRuntime(t)
	prev := http.DefaultClient.Transport
	http.DefaultClient.Transport = roundTripFailer{t: t}
	t.Cleanup(func() { http.DefaultClient.Transport = prev })

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("page read sent Authorization %q", auth)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "via-browser")
	}))
	t.Cleanup(ts.Close)

	page, err := readBrowserPage(context.Background(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if page.Text != "via-browser" {
		t.Fatalf("text = %q", page.Text)
	}
}

type roundTripFailer struct{ t *testing.T }

func (f roundTripFailer) RoundTrip(*http.Request) (*http.Response, error) {
	f.t.Fatal("research page read used http.DefaultClient")
	return nil, errors.New("http.DefaultClient")
}
