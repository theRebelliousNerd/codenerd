package research

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/browser"
	"codenerd/internal/config"
	"codenerd/internal/mangle"
	"codenerd/internal/types"
)

// retiringKernel is a kernel whose retraction really removes facts, so a test
// can see what a read left behind in it. failRetract makes retraction fail.
type retiringKernel struct {
	browserReasoningKernel
	failRetract bool
}

func factKey(f types.Fact) string { return fmt.Sprintf("%s|%v", f.Predicate, f.Args) }

func (k *retiringKernel) RetractExactFactsBatch(facts []types.Fact) error {
	if k.failRetract {
		return errors.New("kernel retraction unavailable")
	}
	drop := make(map[string]struct{}, len(facts))
	for _, f := range facts {
		drop[factKey(f)] = struct{}{}
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	kept := k.facts[:0]
	for _, f := range k.facts {
		if _, gone := drop[factKey(f)]; !gone {
			kept = append(kept, f)
		}
	}
	k.facts = kept
	return nil
}

// browserFactCount counts the transient browser observations a read leaves.
func (k *retiringKernel) browserFactCount() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	n := 0
	for _, f := range k.facts {
		if strings.HasPrefix(f.Predicate, "net_") || f.Predicate == "dom_updated" ||
			f.Predicate == "browser_epoch" || f.Predicate == "browser_stream_saturated" ||
			f.Predicate == "console_event" || f.Predicate == "dom_node" {
			n++
		}
	}
	return n
}

func (k *retiringKernel) factsOf(predicate string) int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	n := 0
	for _, f := range k.facts {
		if f.Predicate == predicate {
			n++
		}
	}
	return n
}

// liveRuntime builds a runtime of its own (sentinel interactive manager, the
// given kernel), binds it, and launches its headless Chrome. A launch failure
// skips: the suite must not call a missing browser a passed read.
func liveRuntime(t *testing.T, kernel types.Kernel) *BrowserRuntime {
	t.Helper()
	if !config.ResolvedResearchPolicy().BrowserHeadless {
		t.Fatal("live runtime tests need research.browser_headless true")
	}
	sentinel := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	rt := newBrowserRuntime(sentinel, kernel, BrowserRuntimeOptions{})
	bindForTest(t, rt)
	t.Cleanup(func() { _ = rt.Close(context.Background()) })
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := rt.pageManager(ctx); err != nil {
		var launch *chromeLaunchError
		if errors.As(err, &launch) {
			t.Skipf("Chrome cannot be launched: %v", err)
		}
		t.Fatalf("research browser: %v", err)
	}
	return rt
}

func staticPage(t *testing.T, body string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(ts.Close)
	return ts
}

func chromeAcceptsConnections(controlURL string) bool {
	u, err := url.Parse(controlURL)
	if err != nil || u.Host == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", u.Host, time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func nonHeadlessRuntime(t *testing.T, kernel types.Kernel) *BrowserRuntime {
	t.Helper()
	installResearchPolicy(t, func(p *config.ResearchPolicy) { p.BrowserHeadless = false })
	sentinel := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	rt := newBrowserRuntime(sentinel, kernel, BrowserRuntimeOptions{})
	bindForTest(t, rt)
	return rt
}

// Close is the single edge that ends the research browser: it refuses new
// reads, and a read that raced the close cannot relaunch the old runtime's
// headless process.
func TestBrowserRuntime_Close_RefusesNewReadsAndNeverRelaunches(t *testing.T) {
	rt := newBrowserRuntime(browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil), &browserReasoningKernel{}, BrowserRuntimeOptions{})
	bindForTest(t, rt)
	if err := rt.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := rt.Close(context.Background()); err != nil {
		t.Fatalf("second Close must be a no-op: %v", err)
	}
	lease, err := rt.admit(context.Background())
	if err == nil || !strings.Contains(err.Error(), "closed") {
		if lease != nil {
			lease.release()
		}
		t.Fatalf("admit after close = %v, want a closed-runtime refusal", err)
	}
	if rt.headlessIfStarted() != nil {
		t.Fatal("a closed runtime launched a headless browser")
	}
	if currentRuntime() == rt {
		t.Fatal("Close left the closed runtime bound")
	}
}

// Closing waits for admitted reads, and tells them to stop first, so their
// sensors are joined and their tabs retired before the browser is shut down.
func TestBrowserRuntime_Close_CancelsThenWaitsForAdmittedReads(t *testing.T) {
	rt := nonHeadlessRuntime(t, &browserReasoningKernel{})
	lease, err := rt.admit(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- rt.Close(context.Background()) }()

	select {
	case <-lease.ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not cancel the admitted read")
	}
	select {
	case err := <-closed:
		t.Fatalf("Close returned (%v) while a read was still draining", err)
	case <-time.After(150 * time.Millisecond):
	}
	lease.release()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return after the read drained")
	}
}

func TestBrowserRuntime_Close_BoundedByContextReportsStragglers(t *testing.T) {
	rt := nonHeadlessRuntime(t, &browserReasoningKernel{})
	lease, err := rt.admit(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.release()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = rt.Close(ctx)
	if err == nil || !strings.Contains(err.Error(), "still draining") {
		t.Fatalf("Close = %v, want the straggler report", err)
	}
}

// Close of an older runtime must neither unbind nor disable its replacement.
func TestBrowserRuntime_Close_LeavesReplacementUsable(t *testing.T) {
	installResearchPolicy(t, func(p *config.ResearchPolicy) { p.BrowserHeadless = false })
	old := newBrowserRuntime(browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil), &browserReasoningKernel{}, BrowserRuntimeOptions{})
	replacement := newBrowserRuntime(browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil), &browserReasoningKernel{}, BrowserRuntimeOptions{})
	bindForTest(t, old)
	bindRuntime(replacement)
	t.Cleanup(func() { _ = replacement.Close(context.Background()) })

	if err := old.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if currentRuntime() != replacement {
		t.Fatal("closing the old runtime unbound its replacement")
	}
	lease, err := admitResearchRead(context.Background())
	if err != nil {
		t.Fatalf("replacement refused a read: %v", err)
	}
	lease.release()
}

// A lease is the read's immutable view. Rebinding the process-wide runtime
// mid-read must not move the read's manager or kernel.
func TestResearchLease_PinsManagerAndKernelAcrossRebind(t *testing.T) {
	installResearchPolicy(t, func(p *config.ResearchPolicy) { p.BrowserHeadless = false })
	k1, k2 := &browserReasoningKernel{}, &browserReasoningKernel{}
	first := newBrowserRuntime(browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil), k1, BrowserRuntimeOptions{})
	second := newBrowserRuntime(browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil), k2, BrowserRuntimeOptions{})
	bindForTest(t, first)

	lease, err := admitResearchRead(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.release()
	bindRuntime(second)
	if lease.kernel != types.Kernel(k1) || lease.mgr != first.mgr {
		t.Fatal("a rebind moved an admitted read onto the replacement runtime")
	}
	if getBrowserKernel() != types.Kernel(k2) {
		t.Fatal("the replacement is not the process binding")
	}
}

// The headless manager's sink is fixed to its runtime's kernel. The old sink
// resolved whichever kernel was bound when each batch arrived, so a rebind
// redirected an in-flight read's facts to another workspace.
func TestRuntimeKernelSink_WritesToItsOwnKernelAfterRebind(t *testing.T) {
	k1, k2 := &browserReasoningKernel{}, &browserReasoningKernel{}
	first := newBrowserRuntime(browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil), k1, BrowserRuntimeOptions{})
	second := newBrowserRuntime(browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil), k2, BrowserRuntimeOptions{})
	bindForTest(t, first)
	sink := runtimeKernelSink{kernel: first.kernel}
	bindRuntime(second)

	if err := sink.AddFacts([]mangle.Fact{{Predicate: "dom_updated", Args: []any{"s", int64(1)}}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := k1.Query("dom_updated"); len(got) != 1 {
		t.Fatalf("owning kernel has %d facts, want 1", len(got))
	}
	if got, _ := k2.Query("dom_updated"); len(got) != 0 {
		t.Fatalf("the replacement's kernel received %d of another runtime's facts", len(got))
	}
}

func TestResearchLaunchArgv_ConfiguredLaunchWinsMinusOwnedProfileFlags(t *testing.T) {
	argv, dropped := researchLaunchArgv([]string{"/opt/chrome/chrome", "--lang=en", "--user-data-dir=/home/op/profile", "--remote-debugging-port=9222", "--disable-extensions"})
	want := []string{"/opt/chrome/chrome", "--lang=en", "--disable-extensions"}
	if !slices.Equal(argv, want) {
		t.Fatalf("argv = %v, want %v", argv, want)
	}
	if len(dropped) != 2 {
		t.Fatalf("dropped = %v, want the profile and debugger flags named", dropped)
	}
}

func TestResearchLaunchArgv_UnsetLaunchDiscoversBinary(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "chrome")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NERD_TEST_CHROME_BIN", fake)
	argv, dropped := researchLaunchArgv(nil)
	if len(argv) == 0 || argv[0] != fake || len(dropped) != 0 {
		t.Fatalf("argv = %v dropped = %v, want the discovered binary", argv, dropped)
	}
}

func TestResearchHeadlessConfig_InheritsOperatorSettingsAndKeepsNarrowing(t *testing.T) {
	reaper := config.BrowserReaperConfig{TimeoutMs: 7000, PollIntervalMs: 90}
	cfg := researchHeadlessConfig(BrowserRuntimeOptions{
		Launch:             []string{"/opt/chrome/chrome", "--lang=en"},
		Reaper:             reaper,
		ExtraSensitiveKeys: []string{"x-internal-token"},
		MaxTabs:            3,
		MaxBrowsers:        2,
	})
	if cfg.Reaper != reaper {
		t.Errorf("reaper = %+v, want the operator's %+v", cfg.Reaper, reaper)
	}
	if !slices.Equal(cfg.Launch, []string{"/opt/chrome/chrome", "--lang=en"}) {
		t.Errorf("launch = %v, want the operator's binary and flags", cfg.Launch)
	}
	if !slices.Equal(cfg.ExtraSensitiveKeys, []string{"x-internal-token"}) || cfg.MaxTabs != 3 || cfg.MaxBrowsers != 2 {
		t.Errorf("redaction keys / limits not inherited: %v %d %d", cfg.ExtraSensitiveKeys, cfg.MaxTabs, cfg.MaxBrowsers)
	}
	if !cfg.Headless || cfg.DebuggerURL != "" || cfg.SessionStore != "" || cfg.WorkspaceRoot != "" ||
		cfg.IsEvidenceEnabled() || cfg.ShouldIngestHeaders() {
		t.Errorf("research narrowing was widened: %+v", cfg)
	}
}

// A sensor that could not install, or a stream that dropped facts, cannot
// certify a page as stable: absence of activity is not evidence of quiet.
func TestResearchDOMWatch_IncompleteObservationIsNotStable(t *testing.T) {
	w := &researchDOMWatch{}
	if err := w.incomplete(browser.SessionFactStats{}); err != nil {
		t.Fatalf("clean watch reported %v", err)
	}
	if err := w.incomplete(browser.SessionFactStats{Epoch: 1, Dropped: 4}); err == nil || !strings.Contains(err.Error(), "saturated") {
		t.Fatalf("dropped stream facts = %v, want a saturation refusal", err)
	}
	w.fail(errors.New("install DOM binding: target closed"))
	if err := w.incomplete(browser.SessionFactStats{}); err == nil || !strings.Contains(err.Error(), "install DOM binding") {
		t.Fatalf("sensor failure = %v, want it reported", err)
	}
}

// The default headless path: a read leaves nothing of its own behind in the
// kernel. DOM sensor facts, network facts and lifecycle markers retire with
// the tab.
func TestReadBrowserPage_RetiresEveryTransientFactWithTheTab(t *testing.T) {
	kernel := &retiringKernel{}
	liveRuntime(t, kernel)
	ts := staticPage(t, `<html><body><p id="a">x</p><script>
let n = 0; const t = setInterval(function(){ n++; const s = document.createElement("span"); s.textContent = "t" + n; document.body.appendChild(s); if (n >= 4) clearInterval(t); }, 60);
</script></body></html>`)

	page, err := readBrowserPage(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("readBrowserPage: %v", err)
	}
	if page.CleanupErr != nil {
		t.Fatalf("cleanup: %v", page.CleanupErr)
	}
	if n := kernel.browserFactCount(); n != 0 {
		t.Fatalf("%d transient browser facts survive the closed tab", n)
	}
}

// A cleanup failure after a good read keeps the content and says cleanup is
// pending, in the structured field and in the model-bound tool result.
func TestReadBrowserPage_CleanupFailureKeepsContentAndReportsPending(t *testing.T) {
	kernel := &retiringKernel{failRetract: true}
	liveRuntime(t, kernel)
	ts := staticPage(t, `<html><body><h1>kept content</h1></body></html>`)

	page, err := readBrowserPage(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("a cleanup failure must not fail the read: %v", err)
	}
	if page.CleanupErr == nil || !strings.Contains(page.Text, "kept content") {
		t.Fatalf("page = %+v, want the content and a cleanup error", page)
	}
	out, err := executeWebFetch(context.Background(), map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("executeWebFetch: %v", err)
	}
	if !strings.Contains(out, "kept content") || !strings.Contains(out, "cleanup pending") {
		t.Fatalf("tool result = %q, want the content plus the pending-cleanup notice", out)
	}
}

// Closing the runtime while a read is blocked on the network ends the read,
// joins it, and takes the headless Chrome down with the runtime.
func TestBrowserRuntime_Close_EndsInFlightReadAndChrome(t *testing.T) {
	kernel := &retiringKernel{}
	rt := liveRuntime(t, kernel)
	headless := rt.headlessIfStarted()
	controlURL := headless.ControlURL()
	if !chromeAcceptsConnections(controlURL) {
		t.Skipf("cannot probe Chrome at %q", controlURL)
	}

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
	}))
	t.Cleanup(func() { close(release); ts.Close() })

	var wg sync.WaitGroup
	var readErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, readErr = readBrowserPage(context.Background(), ts.URL)
	}()
	select {
	case <-started:
	case <-time.After(20 * time.Second):
		t.Fatal("the read never reached the server")
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := rt.Close(closeCtx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	wg.Wait()
	if readErr == nil {
		t.Fatal("the in-flight read survived runtime close")
	}
	if chromeAcceptsConnections(controlURL) {
		t.Fatal("headless Chrome is still serving after the runtime closed")
	}
	if n := kernel.browserFactCount(); n != 0 {
		t.Fatalf("%d transient facts left behind by the cancelled read", n)
	}
}

// Rebinding the process-wide runtime while a read is in flight leaves the
// read on its own kernel: every fact it publishes and queries stays there.
func TestReadBrowserPage_StaysOnItsKernelWhenRuntimeRebinds(t *testing.T) {
	first := &retiringKernel{}
	liveRuntime(t, first)
	other := &retiringKernel{}
	replacement := newBrowserRuntime(browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil), other, BrowserRuntimeOptions{})
	t.Cleanup(func() { _ = replacement.Close(context.Background()) })

	ts := staticPage(t, `<html><body><script>
let n = 0; const t = setInterval(function(){ n++; const s = document.createElement("i"); document.body.appendChild(s); if (n >= 10) clearInterval(t); }, 80);
</script>pinned</body></html>`)

	type result struct {
		page browserPage
		err  error
	}
	done := make(chan result, 1)
	go func() {
		page, err := readBrowserPage(context.Background(), ts.URL)
		done <- result{page, err}
	}()
	deadline := time.Now().Add(20 * time.Second)
	for first.factsOf("dom_updated")+first.factsOf("net_request") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the read published no facts to its kernel")
		}
		time.Sleep(10 * time.Millisecond)
	}
	bindRuntime(replacement)

	res := <-done
	if res.err != nil {
		t.Fatalf("a rebind broke the in-flight read: %v", res.err)
	}
	if n := other.browserFactCount() + other.factsOf("dom_updated"); n != 0 {
		t.Fatalf("the replacement kernel received %d facts from another runtime's read", n)
	}
}
