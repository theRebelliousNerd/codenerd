package research

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/browser"
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/types"
)

// BrowserRuntimeOptions is what a research read inherits from the operator's
// browser configuration. Research narrows everything else on purpose:
// headless, isolated tabs, no header ingestion, no bulk evidence recording,
// no personal profile or debugger attachment.
type BrowserRuntimeOptions struct {
	// Launch is browser.launch: the Chrome binary and its flags. Profile and
	// remote-debugging flags are dropped, because research must never run in
	// the operator's own profile or attach to a session it does not own.
	Launch []string
	// Reaper is browser.reaper: the bounds on process inspection and cleanup.
	Reaper config.BrowserReaperConfig
	// ExtraSensitiveKeys are browser.extra_sensitive_keys; research redacts
	// whatever the interactive browser redacts.
	ExtraSensitiveKeys []string
	// MaxTabs and MaxBrowsers are browser.max_tabs / browser.max_browsers.
	// Zero keeps the browser defaults.
	MaxTabs     int
	MaxBrowsers int
}

// BrowserRuntime is the one owner of browser resources for research: the
// boot-bound interactive manager, the live kernel, and the lazily started
// headless manager that page reads use by default. Cortex boot creates it,
// and Cortex.Close (which boot rollback also runs) closes it, so the headless
// Chrome cannot outlive the runtime that started it.
//
// A read leases the runtime for its whole lifetime. The lease pins the
// manager, kernel, and cancellation, so a rebind of the process-wide binding
// cannot move an in-flight read onto another kernel.
type BrowserRuntime struct {
	mgr    *browser.SessionManager
	kernel types.Kernel
	opts   BrowserRuntimeOptions

	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	closed   bool
	reads    sync.WaitGroup
	headless *browser.SessionManager

	// startMu serializes the lazy headless launch. A read that finds the
	// runtime closed after the launch returns shuts the new process down.
	startMu sync.Mutex
}

var (
	boundRuntime   *BrowserRuntime
	boundRuntimeMu sync.RWMutex
)

// SetBrowserRuntime binds browser control and browser reasoning to one Cortex
// and returns the runtime that owns the research browser resources. The kernel
// is the same live authority used for planning and policy. The caller owns the
// returned runtime and must Close it with the Cortex. Binding a nil manager
// unbinds and returns nil.
func SetBrowserRuntime(mgr *browser.SessionManager, kernel types.Kernel, opts ...BrowserRuntimeOptions) *BrowserRuntime {
	if mgr == nil {
		bindRuntime(nil)
		return nil
	}
	var o BrowserRuntimeOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	rt := newBrowserRuntime(mgr, kernel, o)
	bindRuntime(rt)
	return rt
}

func newBrowserRuntime(mgr *browser.SessionManager, kernel types.Kernel, opts BrowserRuntimeOptions) *BrowserRuntime {
	rt := &BrowserRuntime{mgr: mgr, kernel: kernel, opts: opts}
	rt.ctx, rt.cancel = context.WithCancel(context.Background())
	return rt
}

func bindRuntime(rt *BrowserRuntime) {
	boundRuntimeMu.Lock()
	boundRuntime = rt
	boundRuntimeMu.Unlock()
}

func currentRuntime() *BrowserRuntime {
	boundRuntimeMu.RLock()
	defer boundRuntimeMu.RUnlock()
	return boundRuntime
}

// getBrowserManager returns the boot-bound interactive manager. A nil manager
// means the runtime is not bound: this does not construct a fallback. A hidden
// nil-kernel manager would let a research read run before the kernel can see
// DOM facts.
func getBrowserManager() *browser.SessionManager {
	if rt := currentRuntime(); rt != nil {
		return rt.mgr
	}
	return nil
}

func getBrowserKernel() types.Kernel {
	if rt := currentRuntime(); rt != nil {
		return rt.kernel
	}
	return nil
}

func requireBoundBrowser() (*browser.SessionManager, error) {
	mgr := getBrowserManager()
	if mgr == nil {
		return nil, fmt.Errorf("browser runtime is not bound; boot must call research.SetBrowserRuntime before browser tools run")
	}
	return mgr, nil
}

// ClearBrowserManager unbinds the runtime owning mgr, only if it is still the
// process binding. This prevents an older Cortex shutdown from detaching a
// newer Cortex runtime. It does not close anything: BrowserRuntime.Close does.
func ClearBrowserManager(mgr *browser.SessionManager) {
	boundRuntimeMu.Lock()
	if boundRuntime != nil && boundRuntime.mgr == mgr {
		boundRuntime = nil
	}
	boundRuntimeMu.Unlock()
}

// researchLease is one admitted read's immutable view of its runtime.
type researchLease struct {
	rt     *BrowserRuntime
	mgr    *browser.SessionManager
	kernel types.Kernel
	ctx    context.Context
	stop   context.CancelFunc
	stopAf func() bool
}

// release ends the lease. Close waits for every unreleased lease.
func (l *researchLease) release() {
	l.stopAf()
	l.stop()
	l.rt.reads.Done()
}

// admitResearchRead leases the bound runtime for one page read.
func admitResearchRead(ctx context.Context) (*researchLease, error) {
	rt := currentRuntime()
	if rt == nil {
		return nil, fmt.Errorf("browser runtime is not bound; boot must call research.SetBrowserRuntime before a research page read (set browser.launch to the Chrome binary)")
	}
	return rt.admit(ctx)
}

func (rt *BrowserRuntime) admit(ctx context.Context) (*researchLease, error) {
	if rt.kernel == nil {
		return nil, fmt.Errorf("browser runtime was bound without a kernel; research page reads need DOM-stability facts from that kernel")
	}
	rt.mu.Lock()
	if rt.closed {
		rt.mu.Unlock()
		return nil, fmt.Errorf("research browser runtime is closed; no new page reads are admitted")
	}
	rt.reads.Add(1)
	rt.mu.Unlock()

	leaseCtx, stop := context.WithCancel(ctx)
	lease := &researchLease{rt: rt, kernel: rt.kernel, ctx: leaseCtx, stop: stop}
	// Runtime close cancels the read; the lease's own cancel ends the watch.
	lease.stopAf = context.AfterFunc(rt.ctx, stop)

	mgr, err := rt.pageManager(leaseCtx)
	if err != nil {
		lease.release()
		return nil, err
	}
	lease.mgr = mgr
	return lease, nil
}

// pageManager picks the process a read uses: the bound manager when
// research.browser_headless is false, otherwise the runtime's headless one.
func (rt *BrowserRuntime) pageManager(ctx context.Context) (*browser.SessionManager, error) {
	if !config.ResolvedResearchPolicy().BrowserHeadless {
		return rt.mgr, nil
	}
	return rt.headlessManager(ctx)
}

func (rt *BrowserRuntime) headlessManager(ctx context.Context) (*browser.SessionManager, error) {
	rt.startMu.Lock()
	defer rt.startMu.Unlock()
	rt.mu.Lock()
	existing := rt.headless
	rt.mu.Unlock()
	if existing != nil {
		return existing, nil
	}
	mgr := browser.NewSessionManagerWithSink(researchHeadlessConfig(rt.opts), runtimeKernelSink{kernel: rt.kernel})
	// The retractor is what lets a closed tab take its observations with it.
	mgr.SetFactRetractor(browser.NewKernelFactRetractor(rt.kernel))
	if err := mgr.Start(ctx); err != nil {
		return nil, &chromeLaunchError{err: err}
	}
	rt.mu.Lock()
	if rt.closed {
		rt.mu.Unlock()
		// Close ran while Chrome was launching; do not leave it behind.
		return nil, errors.Join(fmt.Errorf("research browser runtime closed while its headless browser was launching"),
			mgr.Shutdown(context.Background()))
	}
	rt.headless = mgr
	rt.mu.Unlock()
	return mgr, nil
}

// Close ends the research side of the runtime: it stops admitting reads,
// cancels the admitted ones and waits for them (so their sensors are joined
// and their tabs retired), then shuts down the headless browser this runtime
// started. The interactive manager belongs to Cortex and is shut down by it
// after this returns. Close is idempotent; a replacement runtime's binding and
// resources are never touched. A bounded ctx that expires before the reads
// drain still shuts the headless browser down and reports the stragglers.
func (rt *BrowserRuntime) Close(ctx context.Context) error {
	if rt == nil {
		return nil
	}
	rt.mu.Lock()
	rt.closed = true
	rt.mu.Unlock()
	rt.cancel()
	ClearBrowserManager(rt.mgr)

	drained := make(chan struct{})
	go func() {
		rt.reads.Wait()
		close(drained)
	}()
	var errs []error
	select {
	case <-drained:
	case <-ctx.Done():
		errs = append(errs, fmt.Errorf("research reads still draining at close: %w", ctx.Err()))
	}

	// A launch in flight holds startMu and notices the closed flag itself.
	rt.startMu.Lock()
	rt.mu.Lock()
	headless := rt.headless
	rt.headless = nil
	rt.mu.Unlock()
	rt.startMu.Unlock()
	if headless != nil {
		shutdownCtx := ctx
		if shutdownCtx.Err() != nil {
			shutdownCtx = context.Background()
		}
		if err := headless.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("shut down research headless browser: %w", err))
		}
	}
	return errors.Join(errs...)
}

// runtimeKernelSink writes the headless browser's facts into the kernel its
// runtime was built with. The kernel is fixed at construction, so a rebind of
// the process-wide binding cannot redirect an in-flight read's facts.
type runtimeKernelSink struct {
	kernel types.Kernel
}

func (s runtimeKernelSink) AddFacts(facts []mangle.Fact) error {
	if s.kernel == nil || len(facts) == 0 {
		return nil
	}
	converted := make([]types.Fact, 0, len(facts))
	for _, fact := range facts {
		converted = append(converted, types.Fact{
			Predicate: fact.Predicate,
			Args:      append([]any(nil), fact.Args...),
		})
	}
	return s.kernel.AssertBatch(converted)
}

// researchLaunchArgv is the Chrome argv a research read launches with. An
// operator's browser.launch wins: its binary and flags are used, minus the
// flags that would point research at a profile or debugger it does not own.
// With no browser.launch, the discovered binary gets the flags in-repo Chrome
// launches already need. The second result names every dropped flag.
func researchLaunchArgv(configured []string) ([]string, []string) {
	if len(configured) == 0 {
		bin := discoverChromeBinary()
		if bin == "" {
			return nil, nil
		}
		// --disable-gpu keeps the read off a wedged Vulkan device. The
		// sandbox flags are the ones in-repo Chrome launches already need.
		return []string{bin, "--disable-gpu", "--no-sandbox", "--disable-dev-shm-usage"}, nil
	}
	argv := []string{configured[0]}
	var dropped []string
	for _, flag := range configured[1:] {
		name, _, _ := strings.Cut(strings.TrimLeft(flag, "-"), "=")
		switch name {
		case "user-data-dir", "remote-debugging-port", "remote-debugging-address", "remote-debugging-pipe":
			dropped = append(dropped, flag)
		default:
			argv = append(argv, flag)
		}
	}
	return argv, dropped
}

func researchHeadlessConfig(opts BrowserRuntimeOptions) browser.Config {
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
	// Inherited from the operator: the browser binary, cleanup bounds, extra
	// redaction keys, and resource limits.
	cfg.Reaper = opts.Reaper.WithDefaults()
	cfg.ExtraSensitiveKeys = append([]string(nil), opts.ExtraSensitiveKeys...)
	if opts.MaxTabs > 0 {
		cfg.MaxTabs = opts.MaxTabs
	}
	if opts.MaxBrowsers > 0 {
		cfg.MaxBrowsers = opts.MaxBrowsers
	}
	argv, dropped := researchLaunchArgv(opts.Launch)
	if len(dropped) > 0 {
		logging.ResearcherDebug("research browser launch: ignoring %v from browser.launch (research never uses an operator profile or debugger)", dropped)
	}
	cfg.Launch = argv
	return cfg
}
