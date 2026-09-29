package research

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"codenerd/internal/browser"
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/types"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// researchPageUserAgent is the identity a research page read sends. It is
// not a tunable: the browser, not the caller, owns the header.
const researchPageUserAgent = "Mozilla/5.0 (compatible; codeNERD/1.0; +https://github.com/codenerd)"

// The stable-wait windows are the browser_wait mode "stable" schema defaults
// (BrowserWaitTool: poll 200ms, network idle 500ms, DOM idle 200ms). They are
// not tunables. readBrowserPage calls waitForStableBrowser with them instead
// of keeping a second copy of the quiescence loop.
const (
	researchPageStablePoll  = 200 * time.Millisecond
	researchPageNetworkIdle = 500 * time.Millisecond
	researchPageDOMIdle     = 200 * time.Millisecond
	// researchPageStatusGrace bounds the wait for the document response event
	// that can land just after WaitLoad. The request context still caps it.
	researchPageStatusGrace = time.Second
)

// browserPage is one rendered document. HTML is documentElement.outerHTML.
// Text is the body, or the <pre> text when Chrome wrapped a plain response.
type browserPage struct {
	URL         string
	HTML        string
	Text        string
	Status      int
	StatusText  string
	ContentType string
}

// chromeLaunchError is a research-page Chrome launch failure. The message
// names browser.launch because that is the config key for the binary; the
// bound manager's launch list is unexported, so this process cannot copy it.
type chromeLaunchError struct {
	err error
}

func (e *chromeLaunchError) Error() string {
	return fmt.Sprintf("failed to launch Chrome for a research page read (set browser.launch to the Chrome binary): %v", e.err)
}

func (e *chromeLaunchError) Unwrap() error { return e.err }

// researchKernelSink writes the headless browser's facts into whichever
// kernel is bound right now. The sink outlives a single SetBrowserRuntime,
// so it reads the kernel at fact time rather than capturing it.
type researchKernelSink struct{}

func (researchKernelSink) AddFacts(facts []mangle.Fact) error {
	kernel := getBrowserKernel()
	if kernel == nil || len(facts) == 0 {
		return nil
	}
	converted := make([]types.Fact, 0, len(facts))
	for _, fact := range facts {
		converted = append(converted, types.Fact{
			Predicate: fact.Predicate,
			Args:      append([]any(nil), fact.Args...),
		})
	}
	return kernel.AssertBatch(converted)
}

// The boot-bound SessionManager is often headed (browser.headless defaults
// false). LaunchAdditional uses that same flag, so a research read cannot
// ask the pool for a headless process. When research.browser_headless is
// true, page reads use this separate process.
var (
	researchHeadlessMu sync.Mutex
	researchHeadless   *browser.SessionManager
)

func researchHeadlessConfig() browser.Config {
	cfg := browser.DefaultConfig()
	cfg.Headless = true
	cfg.EnableDOMIngestion = true
	cfg.EventLoggingLevel = "normal"
	cfg.EventThrottleMs = 50
	cfg.SessionStore = ""
	cfg.WorkspaceRoot = ""
	off := false
	cfg.EvidenceEnabled = &off
	// Stability only needs net_* and dom_updated. Header facts are a
	// credential surface this read does not use.
	cfg.HeaderIngestionMode = browser.HeaderIngestionOff
	if bin := discoverChromeBinary(); bin != "" {
		// --disable-gpu keeps the read off a wedged Vulkan device. The
		// sandbox flags are the ones in-repo Chrome launches already need.
		cfg.Launch = []string{bin, "--disable-gpu", "--no-sandbox", "--disable-dev-shm-usage"}
	}
	return cfg
}

// discoverChromeBinary finds a Chrome for the headless research process.
// A custom browser.launch is honored only when research.browser_headless is
// false and the read uses the boot-bound manager. NERD_TEST_CHROME_BIN is an
// environment probe for tests, not a config default.
func discoverChromeBinary() string {
	if explicit := os.Getenv("NERD_TEST_CHROME_BIN"); explicit != "" {
		if st, err := os.Stat(explicit); err == nil && !st.IsDir() {
			return explicit
		}
	}
	if path, ok := launcher.LookPath(); ok && path != "" {
		return path
	}
	for _, path := range []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
	} {
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path
		}
	}
	return ""
}

func researchHeadlessManager(ctx context.Context) (*browser.SessionManager, error) {
	researchHeadlessMu.Lock()
	defer researchHeadlessMu.Unlock()
	if researchHeadless != nil {
		return researchHeadless, nil
	}
	mgr := browser.NewSessionManagerWithSink(researchHeadlessConfig(), researchKernelSink{})
	if err := mgr.Start(ctx); err != nil {
		return nil, &chromeLaunchError{err: err}
	}
	researchHeadless = mgr
	return mgr, nil
}

func shutdownResearchBrowser() {
	researchHeadlessMu.Lock()
	mgr := researchHeadless
	researchHeadless = nil
	researchHeadlessMu.Unlock()
	if mgr != nil {
		_ = mgr.Shutdown(context.Background())
	}
}

// researchPageBrowser is the process the current policy would read with,
// if one has been started. Tests use it to count tabs. It does not launch.
func researchPageBrowser() *browser.SessionManager {
	if config.ResolvedResearchPolicy().BrowserHeadless {
		researchHeadlessMu.Lock()
		defer researchHeadlessMu.Unlock()
		return researchHeadless
	}
	return getBrowserManager()
}

func requireResearchPageRuntime() error {
	if getBrowserManager() == nil {
		return fmt.Errorf("browser runtime is not bound; boot must call research.SetBrowserRuntime before a research page read (set browser.launch to the Chrome binary)")
	}
	if getBrowserKernel() == nil {
		return fmt.Errorf("browser runtime was bound without a kernel; research page reads need DOM-stability facts from that kernel")
	}
	return nil
}

func researchPageManager(ctx context.Context) (*browser.SessionManager, error) {
	if err := requireResearchPageRuntime(); err != nil {
		return nil, err
	}
	if !config.ResolvedResearchPolicy().BrowserHeadless {
		return getBrowserManager(), nil
	}
	return researchHeadlessManager(ctx)
}

type documentStatus struct {
	status int
	text   string
	mime   string
	header string
}

// readBrowserPage opens an isolated tab, waits for load and DOM stability,
// and reads the rendered document. The tab is closed before return. A failed
// wait is an error: the result is not a blank frame. There is no net/http
// fallback.
func readBrowserPage(ctx context.Context, rawURL string) (browserPage, error) {
	mgr, err := researchPageManager(ctx)
	if err != nil {
		return browserPage{}, err
	}
	safeURL := mgr.SanitizeForEvidence(rawURL)

	session, err := mgr.CreateTab(ctx, "", "about:blank", true)
	if err != nil {
		if ctx.Err() != nil {
			return browserPage{}, ctx.Err()
		}
		return browserPage{}, fmt.Errorf("open research tab for %s: %w", safeURL, err)
	}
	defer func() {
		_ = mgr.CloseSession(context.Background(), session.ID)
	}()

	page, ok := mgr.Page(session.ID)
	if !ok || page == nil {
		return browserPage{}, fmt.Errorf("research tab %s has no page", session.ID)
	}
	if err := (proto.NetworkEnable{}).Call(page); err != nil {
		logging.ResearcherDebug("research page read: network domain: %v", err)
	}
	if err := (proto.NetworkSetUserAgentOverride{UserAgent: researchPageUserAgent}).Call(page); err != nil {
		// The read still works with Chrome's own identity. The override is
		// so servers can tell a codeNERD read from a generic headless client.
		logging.ResearcherDebug("research page read: user-agent override: %v", err)
	}

	var statusMu sync.Mutex
	var doc documentStatus
	listen, stopListen := page.WithCancel()
	defer stopListen()
	waitEvents := listen.EachEvent(func(e *proto.NetworkResponseReceived) {
		if e.Type != proto.NetworkResourceTypeDocument || e.Response == nil {
			return
		}
		// CreateTab opens about:blank before this subscription. Its document
		// response can still arrive here and must not count as the page we
		// navigated to, or a later non-200 would be reported as that 200.
		if e.Response.URL == "" || strings.HasPrefix(e.Response.URL, "about:") {
			return
		}
		header := headerValue(e.Response.Headers, "content-type")
		statusMu.Lock()
		doc = documentStatus{
			status: e.Response.Status,
			text:   e.Response.StatusText,
			mime:   e.Response.MIMEType,
			header: header,
		}
		statusMu.Unlock()
	})
	go waitEvents()

	p := page.Context(ctx)
	if err := p.Navigate(rawURL); err != nil {
		if ctx.Err() != nil {
			return browserPage{}, ctx.Err()
		}
		// Chrome rejects the navigation for a non-2xx document and the status
		// is only on the response event. Prefer that over the net error so a
		// 404 is an HTTP result, not a transport failure.
		if strings.Contains(err.Error(), "ERR_HTTP_RESPONSE_CODE_FAILURE") {
			if observed, statusErr := waitDocumentStatus(ctx, func() documentStatus {
				statusMu.Lock()
				defer statusMu.Unlock()
				return doc
			}); statusErr == nil && observed.status != 0 {
				return browserPage{}, fmt.Errorf("HTTP %d: %s", observed.status, observed.text)
			}
		}
		return browserPage{}, fmt.Errorf("navigate %s: %w", safeURL, err)
	}
	if err := p.WaitLoad(); err != nil {
		if ctx.Err() != nil {
			return browserPage{}, ctx.Err()
		}
		return browserPage{}, fmt.Errorf("page load wait failed for %s: %w", safeURL, err)
	}

	observed, err := waitDocumentStatus(ctx, func() documentStatus {
		statusMu.Lock()
		defer statusMu.Unlock()
		return doc
	})
	if err != nil {
		return browserPage{}, fmt.Errorf("%s: %w", safeURL, err)
	}
	// A non-200 is already a failed read. Waiting out DOM quiescence would
	// only delay the error; it would not change the status.
	if observed.status != 200 {
		return browserPage{}, fmt.Errorf("HTTP %d: %s", observed.status, observed.text)
	}

	kernel := getBrowserKernel()
	if kernel == nil {
		return browserPage{}, fmt.Errorf("browser runtime was bound without a kernel; research page reads need DOM-stability facts from that kernel")
	}
	// The session stream installs its mutation observer on the about:blank
	// tab, and that observer does not survive this navigation.
	// DOM.documentUpdated is not a per-edit signal. waitForStableBrowser only
	// reads dom_updated, so the edits have to be asserted or a page that is
	// still mutating looks quiet.
	stopDOM := watchResearchDOM(p, session.ID, kernel)
	defer stopDOM()
	stableFor, err := researchPageStableTimeout(ctx)
	if err != nil {
		return browserPage{}, err
	}
	stable, err := waitForStableBrowser(ctx, kernel, session.ID, time.Now().UnixMilli(), stableFor, researchPageStablePoll, researchPageNetworkIdle, researchPageDOMIdle)
	if err != nil {
		if ctx.Err() != nil {
			return browserPage{}, ctx.Err()
		}
		return browserPage{}, fmt.Errorf("page did not reach DOM stability for %s: %w", safeURL, err)
	}
	if stable == nil || stable["success"] != true {
		status := any(nil)
		if stable != nil {
			status = stable["status"]
		}
		return browserPage{}, fmt.Errorf("page did not reach DOM stability (%v); refusing a blank-frame extract", status)
	}

	html, text, err := evalRenderedPage(p)
	if err != nil {
		if ctx.Err() != nil {
			return browserPage{}, ctx.Err()
		}
		return browserPage{}, fmt.Errorf("read rendered page %s: %w", safeURL, err)
	}
	return browserPage{
		URL:         rawURL,
		HTML:        html,
		Text:        text,
		Status:      observed.status,
		StatusText:  observed.text,
		ContentType: contentTypeOf(observed),
	}, nil
}

// watchResearchDOM asserts dom_updated while the document keeps changing.
// The session stream's observer is installed on about:blank and does not
// survive navigation, and a second EachEvent on DOM.documentUpdated did not
// see script edits (a page that appended a node every 80ms still went quiet).
// A binding installed on the live document reports those edits into the same
// predicate waitForStableBrowser already reads. The 50ms throttle stays
// inside the 200ms DOM-idle window without flooding the kernel. Cancel stops
// the listener; it does not decide stability.
func watchResearchDOM(page *rod.Page, sessionID string, kernel types.Kernel) func() {
	if page == nil || kernel == nil {
		return func() {}
	}
	const binding = "codenerdResearchDOM"
	listen, cancel := page.WithCancel()
	var last atomic.Int64
	note := func() {
		now := time.Now().UnixMilli()
		prev := last.Load()
		if prev != 0 && now-prev < 50 {
			return
		}
		if !last.CompareAndSwap(prev, now) {
			return
		}
		_ = kernel.Assert(types.Fact{
			Predicate: "dom_updated",
			Args:      []any{sessionID, now},
		})
	}
	wait := listen.EachEvent(
		func(e *proto.RuntimeBindingCalled) {
			if e != nil && e.Name == binding {
				note()
			}
		},
		func(*proto.DOMChildNodeInserted) { note() },
		func(*proto.DOMCharacterDataModified) { note() },
	)
	go wait()
	if err := (proto.RuntimeAddBinding{Name: binding}).Call(page); err != nil {
		logging.ResearcherDebug("research page read: dom binding: %v", err)
	}
	// Depth -1 arms mutation events for the whole document. A depth of 0
	// left script-inserted nodes unreported.
	depth := -1
	_, _ = (proto.DOMGetDocument{Depth: &depth}).Call(page)
	_, _ = page.Eval(`() => {
		const w = window;
		if (w.__codenerdResearchDOM) return true;
		w.__codenerdResearchDOM = true;
		let last = 0;
		const ping = () => {
			const now = Date.now();
			if (now - last < 50) return;
			last = now;
			if (typeof codenerdResearchDOM === "function") codenerdResearchDOM("1");
		};
		new MutationObserver(ping).observe(document.documentElement, {
			subtree: true, childList: true, attributes: true, characterData: true
		});
		return true;
	}`)
	return cancel
}

func headerValue(headers proto.NetworkHeaders, name string) string {
	for key, value := range headers {
		if strings.EqualFold(key, name) {
			return value.Str()
		}
	}
	return ""
}

func contentTypeOf(doc documentStatus) string {
	if strings.Contains(strings.ToLower(doc.header), "markdown") {
		return doc.header
	}
	if doc.mime != "" {
		return doc.mime
	}
	return doc.header
}

func waitDocumentStatus(ctx context.Context, current func() documentStatus) (documentStatus, error) {
	deadline := time.Now().Add(researchPageStatusGrace)
	for {
		if cur := current(); cur.status != 0 {
			return cur, nil
		}
		if err := ctx.Err(); err != nil {
			return documentStatus{}, err
		}
		if time.Now().After(deadline) {
			return documentStatus{}, fmt.Errorf("no HTTP status observed for the document")
		}
		timer := time.NewTimer(20 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return documentStatus{}, ctx.Err()
		case <-timer.C:
		}
	}
}

func researchPageStableTimeout(ctx context.Context) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0, ctx.Err()
		}
		// A quiet-timeout and the caller's deadline land on the same instant
		// otherwise, and the deadline wins. Leave a margin so the result is
		// the stability refusal, which is what the caller has to act on.
		const margin = 500 * time.Millisecond
		if remaining > margin {
			remaining -= margin
		}
		return remaining, nil
	}
	// The three research tools set their own per-request deadline before
	// calling. A bare read still has to stop: web_fetch's bound is that cap.
	return config.ResolvedResearchPolicy().WebFetchTimeout, nil
}

func evalRenderedPage(p *rod.Page) (string, string, error) {
	// body > pre is how Chrome presents a text/plain or text/markdown
	// response. textContent matches the source bytes more closely than
	// innerText, which collapses whitespace.
	obj, err := p.Eval(`() => {
		const pre = document.querySelector("body > pre");
		const text = pre ? (pre.textContent || "") : (document.body ? (document.body.innerText || "") : "");
		const html = document.documentElement ? document.documentElement.outerHTML : "";
		return {html: html, text: text};
	}`)
	if err != nil {
		return "", "", err
	}
	if obj == nil {
		return "", "", fmt.Errorf("page DOM eval returned no value")
	}
	fields := obj.Value.Map()
	return fields["html"].Str(), fields["text"].Str(), nil
}

// cutGuardBytes bounds one response body. The limit is the caller's process
// guard, not a paging window: the caller marks the trip. The cut is on a
// rune boundary so a later HTML parse does not see a split code point.
func cutGuardBytes(s string, limit int) (string, bool) {
	if limit < 0 {
		limit = 0
	}
	if len(s) <= limit {
		return s, false
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], true
}

// plainResearchBody is the document text for a non-HTML response. HTML
// callers keep the rendered markup and convert it themselves.
func plainResearchBody(page browserPage) bool {
	ct := strings.ToLower(page.ContentType)
	return strings.Contains(ct, "text/plain") || strings.Contains(ct, "text/markdown")
}
