// Package browser provides browser automation with DOM/React reification into Mangle facts.
// Adapted from BrowserNERD for the Cortex 1.5.0 Browser Physics Engine (Section 9.0).
package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	browsersecurity "codenerd/internal/browser/security"
	browserspec "codenerd/internal/browser/specs"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Session describes the public metadata for a tracked browser context.
type Session struct {
	ID         string    `json:"id"`
	BrowserID  string    `json:"browser_id,omitempty"`
	TargetID   string    `json:"target_id,omitempty"`
	URL        string    `json:"url,omitempty"`
	Title      string    `json:"title,omitempty"`
	Status     string    `json:"status,omitempty"`
	Isolated   bool      `json:"isolated,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
}

type sessionRecord struct {
	meta         Session
	page         *rod.Page
	isolated     *rod.Browser
	streamCancel context.CancelFunc
	registry     *ElementRegistry
}

// BrowserInstance describes a managed Chrome connection.
type BrowserInstance struct {
	ID         string    `json:"id"`
	ControlURL string    `json:"control_url,omitempty"`
	Default    bool      `json:"default"`
	CreatedAt  time.Time `json:"created_at"`
	TabCount   int       `json:"tab_count"`
}

type browserRecord struct {
	meta    BrowserInstance
	browser *rod.Browser
	cancel  context.CancelFunc
}

type eventThrottler struct {
	interval time.Duration
	mu       sync.Mutex
	last     map[string]time.Time
}

func newEventThrottler(ms int) *eventThrottler {
	if ms <= 0 {
		return nil
	}
	return &eventThrottler{
		interval: time.Duration(ms) * time.Millisecond,
		last:     make(map[string]time.Time),
	}
}

func (t *eventThrottler) Allow(key string) bool {
	if t == nil {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if last, ok := t.last[key]; ok {
		if now.Sub(last) < t.interval {
			return false
		}
	}
	t.last[key] = now
	return true
}

// Config holds browser configuration.
type Config struct {
	DebuggerURL           string             `json:"debugger_url"`
	Launch                []string           `json:"launch"`
	Headless              bool               `json:"headless"`
	ViewportWidth         int                `json:"viewport_width"`
	ViewportHeight        int                `json:"viewport_height"`
	NavigationTimeoutMs   int                `json:"navigation_timeout_ms"`
	SessionStore          string             `json:"session_store"`
	EventLoggingLevel     string             `json:"event_logging_level"` // minimal, normal, verbose
	EnableDOMIngestion    bool               `json:"enable_dom_ingestion"`
	EnableHeaderIngestion bool               `json:"enable_header_ingestion"`
	EventThrottleMs       int                `json:"event_throttle_ms"`
	MultiTabDefault       *bool              `json:"multi_tab_default,omitempty"`
	MaxTabs               int                `json:"max_tabs,omitempty"`
	MaxBrowsers           int                `json:"max_browsers,omitempty"`
	IdleTabTimeoutMs      int                `json:"idle_tab_timeout_ms,omitempty"`
	ExtraSensitiveKeys    []string           `json:"extra_sensitive_keys,omitempty"`
	WorkspaceRoot         string             `json:"workspace_root,omitempty"`
	WritableRoots         []string           `json:"writable_roots,omitempty"`
	EvidenceEnabled       *bool              `json:"evidence_enabled,omitempty"`
	EvidenceDir           string             `json:"evidence_dir,omitempty"`
	MaxEvidenceFiles      int                `json:"max_evidence_files,omitempty"`
	MaxEvidenceFileBytes  int64              `json:"max_evidence_file_bytes,omitempty"`
	Specs                 browserspec.Config `json:"specs,omitempty"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	sharedTabs := true
	return Config{
		Headless:             false,
		ViewportWidth:        1920,
		ViewportHeight:       1080,
		NavigationTimeoutMs:  30000,
		EventLoggingLevel:    "normal",
		EnableDOMIngestion:   true,
		EventThrottleMs:      100,
		MultiTabDefault:      &sharedTabs,
		MaxTabs:              32,
		MaxBrowsers:          4,
		EvidenceEnabled:      boolPointer(true),
		MaxEvidenceFiles:     16,
		MaxEvidenceFileBytes: 4 << 20,
		Specs:                browserspec.DefaultConfig(),
	}
}

func boolPointer(value bool) *bool { return &value }

// IsEvidenceEnabled reports whether the bounded flight recorder is enabled.
func (c Config) IsEvidenceEnabled() bool {
	return c.EvidenceEnabled == nil || *c.EvidenceEnabled
}

// GetMaxEvidenceFiles returns the global rotated trace-file ceiling.
func (c Config) GetMaxEvidenceFiles() int {
	if c.MaxEvidenceFiles <= 0 {
		return 16
	}
	if c.MaxEvidenceFiles > 256 {
		return 256
	}
	return c.MaxEvidenceFiles
}

// GetMaxEvidenceFileBytes returns the per-trace rotation threshold.
func (c Config) GetMaxEvidenceFileBytes() int64 {
	if c.MaxEvidenceFileBytes <= 0 {
		return 4 << 20
	}
	if c.MaxEvidenceFileBytes > 64<<20 {
		return 64 << 20
	}
	return c.MaxEvidenceFileBytes
}

// IsHeadless returns the headless setting.
func (c Config) IsHeadless() bool {
	return c.Headless
}

// GetViewportWidth returns viewport width.
func (c Config) GetViewportWidth() int {
	if c.ViewportWidth == 0 {
		return 1920
	}
	return c.ViewportWidth
}

// GetViewportHeight returns viewport height.
func (c Config) GetViewportHeight() int {
	if c.ViewportHeight == 0 {
		return 1080
	}
	return c.ViewportHeight
}

// NavigationTimeout returns the navigation timeout.
func (c Config) NavigationTimeout() time.Duration {
	if c.NavigationTimeoutMs == 0 {
		return 30 * time.Second
	}
	return time.Duration(c.NavigationTimeoutMs) * time.Millisecond
}

// IsMultiTabDefault reports whether ordinary tabs share the browser profile.
func (c Config) IsMultiTabDefault() bool {
	return c.MultiTabDefault == nil || *c.MultiTabDefault
}

// GetMaxTabs returns the manager-wide tab limit.
func (c Config) GetMaxTabs() int {
	if c.MaxTabs <= 0 {
		return 32
	}
	return c.MaxTabs
}

// GetMaxBrowsers returns the managed browser limit.
func (c Config) GetMaxBrowsers() int {
	if c.MaxBrowsers <= 0 {
		return 4
	}
	return c.MaxBrowsers
}

// GetIdleTabTimeout returns zero when idle tab reaping is disabled.
func (c Config) GetIdleTabTimeout() time.Duration {
	if c.IdleTabTimeoutMs <= 0 {
		return 0
	}
	return time.Duration(c.IdleTabTimeoutMs) * time.Millisecond
}

// EngineSink defines the minimal interface for the Mangle logic layer.
type EngineSink interface {
	AddFacts(facts []mangle.Fact) error
}

// engineAdapter wraps a mangle.Engine to satisfy EngineSink.
type engineAdapter struct {
	engine *mangle.Engine
}

func (a *engineAdapter) AddFacts(facts []mangle.Fact) error {
	return a.engine.AddFacts(facts)
}

// SessionManager owns the detached Chrome instance and tracks active sessions.
type SessionManager struct {
	cfg          Config
	engine       EngineSink
	startMu      sync.Mutex
	mu           sync.RWMutex
	browser      *rod.Browser
	sessions     map[string]*sessionRecord
	controlURL   string // WebSocket URL for DevTools
	browsers     map[string]*browserRecord
	defaultID    string
	reaperCancel context.CancelFunc
	redactor     *browsersecurity.Redactor
	pathPolicy   *browsersecurity.PathPolicy
	recorder     *FlightRecorder
	specCatalog  *browserspec.Catalog
	pendingTabs  int
}

// NewSessionManager creates a new session manager.
func NewSessionManager(cfg Config, engine *mangle.Engine) *SessionManager {
	var sink EngineSink
	if engine != nil {
		sink = &engineAdapter{engine: engine}
	}
	return newSessionManager(cfg, sink)
}

// NewSessionManagerWithSink creates a session manager with a custom sink.
func NewSessionManagerWithSink(cfg Config, sink EngineSink) *SessionManager {
	return newSessionManager(cfg, sink)
}

func newSessionManager(cfg Config, sink EngineSink) *SessionManager {
	policy, err := browsersecurity.NewPathPolicy(cfg.WorkspaceRoot, cfg.WritableRoots)
	if err != nil {
		logging.BrowserWarn("Browser output path policy unavailable: %v", err)
	}
	manager := &SessionManager{
		cfg:        cfg,
		engine:     sink,
		sessions:   make(map[string]*sessionRecord),
		browsers:   make(map[string]*browserRecord),
		redactor:   browsersecurity.NewRedactor(cfg.ExtraSensitiveKeys),
		pathPolicy: policy,
	}
	if cfg.IsEvidenceEnabled() && strings.TrimSpace(cfg.WorkspaceRoot) != "" && policy != nil {
		recorder, recorderErr := NewFlightRecorder(cfg, policy, manager.redactor)
		if recorderErr != nil {
			logging.BrowserWarn("Browser flight recorder unavailable: %v", recorderErr)
		} else {
			manager.recorder = recorder
		}
	}
	if cfg.Specs.IsEnabled() && strings.TrimSpace(cfg.WorkspaceRoot) != "" {
		catalog, catalogErr := browserspec.NewCatalog(cfg.WorkspaceRoot, cfg.Specs)
		if catalogErr != nil {
			logging.BrowserWarn("Browser spec catalog unavailable: %v", catalogErr)
		} else {
			manager.specCatalog = catalog
		}
	}
	return manager
}

// LoadSpecs loads the bounded workspace browser specification catalog.
func (m *SessionManager) LoadSpecs(ctx context.Context) (browserspec.LoadResult, error) {
	if m == nil || m.specCatalog == nil {
		return browserspec.LoadResult{}, fmt.Errorf("browser specs are disabled or unavailable")
	}
	return m.specCatalog.Load(ctx)
}

// SpecsConfig returns the normalized catalog delivery limits.
func (m *SessionManager) SpecsConfig() browserspec.Config {
	if m == nil || m.specCatalog == nil {
		return browserspec.Config{}
	}
	return m.specCatalog.Config()
}

// SpecsEnabled reports whether the manager has a usable workspace catalog.
func (m *SessionManager) SpecsEnabled() bool { return m != nil && m.specCatalog != nil }

// ResolveOutputPath confines a browser artifact to configured writable roots.
func (m *SessionManager) ResolveOutputPath(requested, defaultRoot, defaultName string) (string, error) {
	return m.pathPolicy.ResolveForWrite(requested, defaultRoot, defaultName)
}

// SanitizeForEvidence redacts a string before it is logged, returned, or persisted.
func (m *SessionManager) SanitizeForEvidence(value string) string {
	return m.redactor.SanitizeString(value)
}

// Start connects to an existing Chrome or launches a new one.
func (m *SessionManager) Start(ctx context.Context) error {
	timer := logging.StartTimer(logging.CategoryBrowser, "Browser session start")
	defer timer.Stop()

	logging.Browser("Starting browser session manager")
	_, err := m.startDefault(ctx)
	return err
}

func (m *SessionManager) ensureStarted(ctx context.Context) error {
	m.mu.RLock()
	if m.browser != nil {
		m.mu.RUnlock()
		return nil
	}
	m.mu.RUnlock()
	return m.Start(ctx)
}

// ControlURL returns the WebSocket debugger URL.
func (m *SessionManager) ControlURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.controlURL
}

// IsConnected returns whether the browser is connected.
func (m *SessionManager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.browser != nil
}

// Shutdown closes tracked pages and the browser.
func (m *SessionManager) Shutdown(ctx context.Context) error {
	return m.shutdown(ctx)
}

// List returns metadata for all known sessions.
func (m *SessionManager) List() []Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]Session, 0, len(m.sessions))
	for _, record := range m.sessions {
		results = append(results, record.meta)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].CreatedAt.Equal(results[j].CreatedAt) {
			return results[i].ID < results[j].ID
		}
		return results[i].CreatedAt.Before(results[j].CreatedAt)
	})
	return results
}

// CreateSession opens a new page and tracks it.
func (m *SessionManager) CreateSession(ctx context.Context, url string) (*Session, error) {
	return m.CreateTab(ctx, "", url, !m.cfg.IsMultiTabDefault())
}

// Attach binds to an existing target by TargetID.
func (m *SessionManager) Attach(ctx context.Context, targetID string) (*Session, error) {
	return m.AttachToBrowser(ctx, "", targetID)
}

// Page returns the underlying Rod page for a session.
func (m *SessionManager) Page(sessionID string) (*rod.Page, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.sessions[sessionID]
	if !ok {
		return nil, false
	}
	rec.meta.LastActive = time.Now()
	return rec.page, true
}

// UpdateMetadata updates session metadata.
func (m *SessionManager) UpdateMetadata(sessionID string, updater func(Session) Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.sessions[sessionID]
	if !ok {
		return
	}
	rec.meta = updater(rec.meta)
	rec.meta.LastActive = time.Now()
}

// GetSession returns session metadata.
func (m *SessionManager) GetSession(sessionID string) (Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.sessions[sessionID]
	if !ok {
		return Session{}, false
	}
	return rec.meta, true
}

// Registry returns the session-scoped element registry. Existing records from
// older persisted metadata and focused tests receive one lazily.
func (m *SessionManager) Registry(sessionID string) *ElementRegistry {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.sessions[sessionID]
	if !ok {
		return nil
	}
	if record.registry == nil {
		record.registry = NewElementRegistry()
	}
	return record.registry
}

func (m *SessionManager) invalidateElementReferences(sessionID string) {
	if registry := m.Registry(sessionID); registry != nil {
		registry.Clear()
	}
}

// ReifyReact walks the React Fiber tree and emits facts for components, props, and state.
func (m *SessionManager) ReifyReact(ctx context.Context, sessionID string) ([]mangle.Fact, error) {
	if m.engine == nil {
		return nil, errors.New("mangle engine not configured")
	}
	page, ok := m.Page(sessionID)
	if !ok {
		return nil, fmt.Errorf("unknown session: %s", sessionID)
	}

	res, err := page.Context(ctx).Evaluate(&rod.EvalOptions{
		JS: `
		() => {
			const root = document.querySelector('[data-reactroot]') || document.getElementById('root') || document.body;
			if (!root) return [];
			const fiberKey = Object.keys(root).find(k => k.startsWith('__reactFiber'));
			if (!fiberKey) return [];

			const sanitize = (v) => {
				if (v === null) return null;
				const t = typeof v;
				if (t === 'string' || t === 'number' || t === 'boolean') return v;
				return undefined;
			};

			const rootFiber = root[fiberKey];
			const stack = [{ fiber: rootFiber, parent: null }];
			const seen = new Set();
			const results = [];
			let counter = 0;

			while (stack.length) {
				const { fiber, parent } = stack.pop();
				if (!fiber || seen.has(fiber)) continue;
				seen.add(fiber);

				const id = fiber._debugID || ('fiber_' + (counter++));
				const name = (fiber.type && (fiber.type.displayName || fiber.type.name)) ||
							 (fiber.elementType && fiber.elementType.name) ||
							 'Anonymous';

				const props = {};
				if (fiber.memoizedProps && typeof fiber.memoizedProps === 'object') {
					for (const [k, v] of Object.entries(fiber.memoizedProps)) {
						const s = sanitize(v);
						if (s !== undefined) props[k] = s;
					}
				}

				const state = [];
				if (fiber.memoizedState !== undefined) {
					const ms = fiber.memoizedState;
					if (Array.isArray(ms)) {
						ms.forEach((v, i) => {
							const s = sanitize(v);
							if (s !== undefined) state.push([i, s]);
						});
					} else if (ms && typeof ms === 'object' && 'baseState' in ms) {
						const s = sanitize(ms.baseState);
						if (s !== undefined) state.push([0, s]);
					}
				}

				const domNodeId = fiber.stateNode && fiber.stateNode.id ? fiber.stateNode.id : null;
				results.push({ id, name, parent, props, state, domNodeId });

				if (fiber.child) stack.push({ fiber: fiber.child, parent: id });
				if (fiber.sibling) stack.push({ fiber: fiber.sibling, parent });
			}
			return results;
		}
		`,
		ByValue:      true,
		AwaitPromise: true,
	})
	if err != nil || res == nil {
		return nil, fmt.Errorf("react reification failed: %w", err)
	}

	raw, err := res.Value.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal reified tree: %w", err)
	}

	var nodes []struct {
		ID        string         `json:"id"`
		Name      string         `json:"name"`
		Parent    *string        `json:"parent"`
		Props     map[string]any `json:"props"`
		State     [][]any        `json:"state"`
		DomNodeID *string        `json:"domNodeId"`
	}
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil, fmt.Errorf("decode reified tree: %w", err)
	}

	facts := make([]mangle.Fact, 0, len(nodes)*4)
	now := time.Now()

	for _, n := range nodes {
		parent := ""
		if n.Parent != nil {
			parent = *n.Parent
		}
		facts = append(facts, mangle.Fact{
			Predicate: "react_component",
			Args:      []any{n.ID, n.Name, parent},
			Timestamp: now,
		})

		for k, v := range n.Props {
			facts = append(facts, mangle.Fact{
				Predicate: "react_prop",
				Args:      []any{n.ID, k, fmt.Sprintf("%v", v)},
				Timestamp: now,
			})
		}

		for _, entry := range n.State {
			if len(entry) != 2 {
				continue
			}
			// react_state(FiberID, HookIndex, Value) bound
			// [/string, /number, /string]. JSON-decoded numbers arrive as
			// float64 from json.Unmarshal; coerce to int64 so the slot
			// satisfies /number instead of being rejected for a /float64
			// type mismatch.
			var hookIndex int64
			switch v := entry[0].(type) {
			case float64:
				hookIndex = int64(v)
			case int:
				hookIndex = int64(v)
			case int64:
				hookIndex = v
			default:
				// Best-effort parse; skip malformed entries rather than
				// poisoning the kernel with a non-numeric hook index.
				continue
			}
			facts = append(facts, mangle.Fact{
				Predicate: "react_state",
				Args:      []any{n.ID, hookIndex, fmt.Sprintf("%v", entry[1])},
				Timestamp: now,
			})
		}

		if n.DomNodeID != nil && *n.DomNodeID != "" {
			facts = append(facts, mangle.Fact{
				Predicate: "dom_mapping",
				Args:      []any{n.ID, *n.DomNodeID},
				Timestamp: now,
			})
		}
	}

	if err := m.addFacts(facts); err != nil {
		return nil, err
	}
	return facts, nil
}

// ForkSession clones cookies + storage from an existing session into a new incognito context.
func (m *SessionManager) ForkSession(ctx context.Context, sessionID, url string) (*Session, error) {
	if err := m.ensureStarted(ctx); err != nil {
		return nil, err
	}
	srcPage, ok := m.Page(sessionID)
	if !ok {
		return nil, fmt.Errorf("unknown session: %s", sessionID)
	}

	srcMeta, _ := m.GetSession(sessionID)

	// Snapshot cookies
	cookiesRes, err := proto.NetworkGetCookies{}.Call(srcPage)
	if err != nil {
		return nil, fmt.Errorf("get cookies: %w", err)
	}

	// Snapshot storage
	localJSON := snapshotStorage(srcPage, "localStorage")
	sessionJSON := snapshotStorage(srcPage, "sessionStorage")

	targetURL := url
	if targetURL == "" {
		targetURL = srcMeta.URL
		if targetURL == "" {
			targetURL = "about:blank"
		}
	}

	dest, err := m.CreateTab(ctx, srcMeta.BrowserID, targetURL, true)
	if err != nil {
		return nil, fmt.Errorf("create forked session: %w", err)
	}

	destPage, ok := m.Page(dest.ID)
	if !ok {
		return dest, nil
	}

	// Restore cookies
	params := make([]*proto.NetworkCookieParam, 0, len(cookiesRes.Cookies))
	for _, c := range cookiesRes.Cookies {
		params = append(params, &proto.NetworkCookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: c.SameSite,
			Priority: c.Priority,
		})
	}
	if len(params) > 0 {
		_ = destPage.SetCookies(params)
	}

	// Restore storage
	restoreStorage(destPage, localJSON, sessionJSON)
	m.UpdateMetadata(dest.ID, func(s Session) Session {
		s.Status = "forked"
		return s
	})

	_ = m.persistSessions()
	return dest, nil
}

// Navigate navigates to a URL.
func (m *SessionManager) Navigate(ctx context.Context, sessionID, url string) error {
	timer := logging.StartTimer(logging.CategoryBrowser, "Page navigation")
	defer timer.Stop()

	logging.Browser("Navigating session %s to URL: %s", sessionID, m.SanitizeForEvidence(url))
	if err := m.ensureStarted(ctx); err != nil {
		logging.BrowserError("Failed to ensure browser started for navigation: %v", err)
		return err
	}
	page, ok := m.Page(sessionID)
	if !ok {
		logging.BrowserError("Unknown session for navigation: %s", sessionID)
		return fmt.Errorf("unknown session: %s", sessionID)
	}
	// Navigation may partially succeed even when Rod reports an error. Fail
	// closed by invalidating element refs before issuing it.
	m.invalidateElementReferences(sessionID)
	logging.BrowserDebug("Navigating with timeout: %s", m.cfg.NavigationTimeout())
	err := page.Context(ctx).Timeout(m.cfg.NavigationTimeout()).Navigate(url)
	if err != nil {
		logging.BrowserError("Navigation failed for session %s: %v", sessionID, err)
	} else {
		actualURL := url
		title := ""
		if info, infoErr := page.Context(ctx).Info(); infoErr == nil && info != nil {
			if info.URL != "" {
				actualURL = info.URL
			}
			title = info.Title
		}
		m.UpdateMetadata(sessionID, func(session Session) Session {
			session.URL = m.redactor.SanitizeString(actualURL)
			session.Title = m.redactor.SanitizeString(title)
			return session
		})
		logging.BrowserDebug("Navigation completed for session %s", sessionID)
	}
	return err
}

// Click clicks an element.
func (m *SessionManager) Click(ctx context.Context, sessionID, selector string) error {
	logging.Browser("Clicking element: %s (session=%s)", selector, sessionID)
	if err := m.ensureStarted(ctx); err != nil {
		logging.BrowserError("Failed to ensure browser started for click: %v", err)
		return err
	}
	page, ok := m.Page(sessionID)
	if !ok {
		logging.BrowserError("Unknown session for click: %s", sessionID)
		return fmt.Errorf("unknown session: %s", sessionID)
	}
	logging.BrowserDebug("Finding element: %s", selector)
	el, err := page.Context(ctx).Element(selector)
	if err != nil {
		logging.BrowserError("Element not found for click: %s - %v", selector, err)
		return fmt.Errorf("element not found: %w", err)
	}
	logging.BrowserDebug("Element found, performing click")
	err = el.Click(proto.InputMouseButtonLeft, 1)
	if err != nil {
		logging.BrowserError("Click failed on element %s: %v", selector, err)
	} else {
		logging.BrowserDebug("Click completed on element: %s", selector)
	}
	return err
}

// Type types text into an element.
func (m *SessionManager) Type(ctx context.Context, sessionID, selector, text string) error {
	logging.Browser("Typing into element: %s (session=%s, len=%d)", selector, sessionID, len(text))
	if err := m.ensureStarted(ctx); err != nil {
		logging.BrowserError("Failed to ensure browser started for type: %v", err)
		return err
	}
	page, ok := m.Page(sessionID)
	if !ok {
		logging.BrowserError("Unknown session for type: %s", sessionID)
		return fmt.Errorf("unknown session: %s", sessionID)
	}
	logging.BrowserDebug("Finding element for input: %s", selector)
	el, err := page.Context(ctx).Element(selector)
	if err != nil {
		logging.BrowserError("Element not found for type: %s - %v", selector, err)
		return fmt.Errorf("element not found: %w", err)
	}
	logging.BrowserDebug("Element found, typing %d characters", len(text))
	err = el.Input(text)
	if err != nil {
		logging.BrowserError("Type failed on element %s: %v", selector, err)
	} else {
		logging.BrowserDebug("Type completed on element: %s", selector)
	}
	return err
}

// Screenshot captures a screenshot.
func (m *SessionManager) Screenshot(ctx context.Context, sessionID string, fullPage bool) ([]byte, error) {
	timer := logging.StartTimer(logging.CategoryBrowser, "Screenshot capture")
	defer timer.Stop()

	logging.Browser("Capturing screenshot (session=%s, fullPage=%v)", sessionID, fullPage)
	if err := m.ensureStarted(ctx); err != nil {
		logging.BrowserError("Failed to ensure browser started for screenshot: %v", err)
		return nil, err
	}
	page, ok := m.Page(sessionID)
	if !ok {
		logging.BrowserError("Unknown session for screenshot: %s", sessionID)
		return nil, fmt.Errorf("unknown session: %s", sessionID)
	}
	var data []byte
	var err error
	if fullPage {
		logging.BrowserDebug("Capturing full page screenshot")
		data, err = page.Context(ctx).Screenshot(true, nil)
	} else {
		logging.BrowserDebug("Capturing viewport screenshot")
		data, err = page.Context(ctx).Screenshot(false, nil)
	}
	if err != nil {
		logging.BrowserError("Screenshot capture failed: %v", err)
		return nil, err
	}
	logging.Browser("Screenshot captured successfully (%d bytes)", len(data))
	return data, nil
}
