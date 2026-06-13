// Package browser provides browser automation with DOM/React reification into Mangle facts.
// Adapted from BrowserNERD for the Cortex 1.5.0 Browser Physics Engine (Section 9.0).
package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/mangle"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
	"github.com/google/uuid"
)

// Session describes the public metadata for a tracked browser context.
type Session struct {
	ID         string    `json:"id"`
	TargetID   string    `json:"target_id,omitempty"`
	URL        string    `json:"url,omitempty"`
	Title      string    `json:"title,omitempty"`
	Status     string    `json:"status,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
}

type sessionRecord struct {
	meta Session
	page *rod.Page
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
	DebuggerURL           string   `json:"debugger_url"`
	Launch                []string `json:"launch"`
	Headless              bool     `json:"headless"`
	ViewportWidth         int      `json:"viewport_width"`
	ViewportHeight        int      `json:"viewport_height"`
	NavigationTimeoutMs   int      `json:"navigation_timeout_ms"`
	SessionStore          string   `json:"session_store"`
	EventLoggingLevel     string   `json:"event_logging_level"` // minimal, normal, verbose
	EnableDOMIngestion    bool     `json:"enable_dom_ingestion"`
	EnableHeaderIngestion bool     `json:"enable_header_ingestion"`
	EventThrottleMs       int      `json:"event_throttle_ms"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Headless:            false,
		ViewportWidth:       1920,
		ViewportHeight:      1080,
		NavigationTimeoutMs: 30000,
		EventLoggingLevel:   "normal",
		EnableDOMIngestion:  true,
		EventThrottleMs:     100,
	}
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
	cfg        Config
	engine     EngineSink
	mu         sync.RWMutex
	browser    *rod.Browser
	sessions   map[string]*sessionRecord
	controlURL string // WebSocket URL for DevTools
}

// NewSessionManager creates a new session manager.
func NewSessionManager(cfg Config, engine *mangle.Engine) *SessionManager {
	var sink EngineSink
	if engine != nil {
		sink = &engineAdapter{engine: engine}
	}
	return &SessionManager{
		cfg:      cfg,
		engine:   sink,
		sessions: make(map[string]*sessionRecord),
	}
}

// NewSessionManagerWithSink creates a session manager with a custom sink.
func NewSessionManagerWithSink(cfg Config, sink EngineSink) *SessionManager {
	return &SessionManager{
		cfg:      cfg,
		engine:   sink,
		sessions: make(map[string]*sessionRecord),
	}
}

// Start connects to an existing Chrome or launches a new one.
func (m *SessionManager) Start(ctx context.Context) error {
	timer := logging.StartTimer(logging.CategoryBrowser, "Browser session start")
	defer timer.Stop()

	logging.Browser("Starting browser session manager")
	m.mu.Lock()
	defer m.mu.Unlock()

	// If we already have a browser, verify it's still alive
	if m.browser != nil {
		logging.BrowserDebug("Checking existing browser connection health")
		_, err := m.browser.Version()
		if err == nil {
			logging.BrowserDebug("Browser connection healthy, reusing existing session")
			return nil // Browser is healthy
		}
		logging.BrowserWarn("Stale browser connection detected, reconnecting...")
		_ = m.browser.Close()
		m.browser = nil
		m.controlURL = ""
		m.sessions = make(map[string]*sessionRecord)
	}

	logging.BrowserDebug("Loading persisted sessions")
	if err := m.loadSessionsLocked(); err != nil {
		logging.BrowserError("Failed to load sessions: %v", err)
		return fmt.Errorf("load sessions: %w", err)
	}

	controlURL := m.cfg.DebuggerURL
	if controlURL == "" && len(m.cfg.Launch) > 0 {
		bin := m.cfg.Launch[0]
		logging.Browser("Launching Chrome from binary: %s (headless=%v)", bin, m.cfg.IsHeadless())
		launch := launcher.New().Bin(bin).Headless(m.cfg.IsHeadless())
		if len(m.cfg.Launch) > 1 {
			for _, rawFlag := range m.cfg.Launch[1:] {
				flagStr := strings.TrimLeft(rawFlag, "-")
				name, val, hasVal := strings.Cut(flagStr, "=")
				if hasVal {
					launch = launch.Set(flags.Flag(name), val)
				} else {
					launch = launch.Set(flags.Flag(name))
				}
			}
		}
		url, err := launch.Launch()
		if err != nil {
			logging.BrowserWarn("Chrome launch failed, trying fallback: %v", err)
			// Fallback
			fallback := launcher.New().Bin(bin).Headless(m.cfg.IsHeadless())
			if alt, altErr := fallback.Launch(); altErr == nil {
				controlURL = alt
				logging.Browser("Chrome launched via fallback, control URL: %s", controlURL)
			} else {
				logging.BrowserError("Chrome launch failed (primary: %v, fallback: %v)", err, altErr)
				return fmt.Errorf("launch chrome: %w (fallback: %v)", err, altErr)
			}
		} else {
			controlURL = url
			logging.Browser("Chrome launched successfully, control URL: %s", controlURL)
		}
	}

	if controlURL == "" {
		// Try default launcher
		logging.Browser("No debugger URL configured, using default launcher")
		url, err := launcher.New().Headless(m.cfg.IsHeadless()).Launch()
		if err != nil {
			logging.BrowserError("Default launcher failed: %v", err)
			return fmt.Errorf("no debugger_url and failed to launch: %w", err)
		}
		controlURL = url
		logging.Browser("Default launcher succeeded, control URL: %s", controlURL)
	}

	logging.BrowserDebug("Connecting to Chrome at: %s", controlURL)
	browser := rod.New().ControlURL(controlURL).Context(ctx)
	if err := browser.Connect(); err != nil {
		logging.BrowserError("Failed to connect to Chrome: %v", err)
		return fmt.Errorf("connect to chrome: %w", err)
	}

	m.browser = browser
	m.controlURL = controlURL
	logging.Browser("Browser session manager started successfully")
	return nil
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
	logging.Browser("Shutting down browser session manager")
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionCount := len(m.sessions)
	logging.BrowserDebug("Closing %d active sessions", sessionCount)
	for id, record := range m.sessions {
		if record.page != nil {
			logging.BrowserDebug("Closing session page: %s", id)
			_ = record.page.Close()
		}
		delete(m.sessions, id)
	}

	var err error
	if m.browser != nil {
		logging.BrowserDebug("Closing browser connection")
		err = m.browser.Close()
		if err != nil {
			logging.BrowserError("Error closing browser: %v", err)
		}
		m.browser = nil
	}
	m.controlURL = ""
	logging.Browser("Browser session manager shutdown complete")
	return err
}

// List returns metadata for all known sessions.
func (m *SessionManager) List() []Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]Session, 0, len(m.sessions))
	for _, record := range m.sessions {
		results = append(results, record.meta)
	}
	return results
}

// CreateSession opens a new page and tracks it.
func (m *SessionManager) CreateSession(ctx context.Context, url string) (*Session, error) {
	timer := logging.StartTimer(logging.CategoryBrowser, "Create browser session")
	defer timer.Stop()

	logging.Browser("Creating new browser session for URL: %s", url)
	if err := m.ensureStarted(ctx); err != nil {
		logging.BrowserError("Failed to ensure browser started: %v", err)
		return nil, err
	}
	if m.browser == nil {
		logging.BrowserError("Browser not connected when creating session")
		return nil, errors.New("browser not connected")
	}

	logging.BrowserDebug("Creating incognito context")
	incognito, err := m.browser.Incognito()
	if err != nil {
		logging.BrowserError("Failed to create incognito context: %v", err)
		return nil, fmt.Errorf("incognito context: %w", err)
	}

	logging.BrowserDebug("Creating new page")
	page, err := incognito.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		logging.BrowserError("Failed to create page: %v", err)
		return nil, fmt.Errorf("create page: %w", err)
	}

	// Set viewport dimensions
	logging.BrowserDebug("Setting viewport: %dx%d", m.cfg.GetViewportWidth(), m.cfg.GetViewportHeight())
	if err := (proto.EmulationSetDeviceMetricsOverride{
		Width:             m.cfg.GetViewportWidth(),
		Height:            m.cfg.GetViewportHeight(),
		DeviceScaleFactor: 1.0,
		Mobile:            false,
	}).Call(page); err != nil {
		logging.BrowserWarn("Failed to set viewport: %v", err)
	}

	// Navigate
	logging.Browser("Navigating to URL: %s (timeout=%s)", url, m.cfg.NavigationTimeout())
	_ = page.Timeout(m.cfg.NavigationTimeout()).Navigate(url)

	meta := Session{
		ID:         uuid.NewString(),
		TargetID:   string(page.TargetID),
		URL:        url,
		Status:     "active",
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}

	m.mu.Lock()
	m.sessions[meta.ID] = &sessionRecord{meta: meta, page: page}
	m.mu.Unlock()

	logging.BrowserDebug("Starting event stream for session: %s", meta.ID)
	m.startEventStream(ctx, meta.ID, page)
	_ = m.persistSessions()

	logging.Browser("Session created successfully: %s (target=%s)", meta.ID, meta.TargetID)
	return &meta, nil
}

// Attach binds to an existing target by TargetID.
func (m *SessionManager) Attach(ctx context.Context, targetID string) (*Session, error) {
	logging.Browser("Attaching to existing browser target: %s", targetID)
	if err := m.ensureStarted(ctx); err != nil {
		logging.BrowserError("Failed to ensure browser started for attach: %v", err)
		return nil, err
	}
	if m.browser == nil {
		logging.BrowserError("Browser not connected when attaching")
		return nil, errors.New("browser not connected")
	}

	logging.BrowserDebug("Getting page from target: %s", targetID)
	page, err := m.browser.PageFromTarget(proto.TargetTargetID(targetID))
	if err != nil {
		logging.BrowserError("Failed to attach to target %s: %v", targetID, err)
		return nil, fmt.Errorf("attach to target %s: %w", targetID, err)
	}

	meta := Session{
		ID:         uuid.NewString(),
		TargetID:   targetID,
		Status:     "attached",
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
	}

	m.mu.Lock()
	m.sessions[meta.ID] = &sessionRecord{meta: meta, page: page}
	m.mu.Unlock()

	logging.BrowserDebug("Starting event stream for attached session: %s", meta.ID)
	m.startEventStream(ctx, meta.ID, page)
	_ = m.persistSessions()
	logging.Browser("Successfully attached to target %s as session %s", targetID, meta.ID)
	return &meta, nil
}

// Page returns the underlying Rod page for a session.
func (m *SessionManager) Page(sessionID string) (*rod.Page, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.sessions[sessionID]
	if !ok {
		return nil, false
	}
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

	if err := m.engine.AddFacts(facts); err != nil {
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

	dest, err := m.CreateSession(ctx, targetURL)
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

	logging.Browser("Navigating session %s to URL: %s", sessionID, url)
	if err := m.ensureStarted(ctx); err != nil {
		logging.BrowserError("Failed to ensure browser started for navigation: %v", err)
		return err
	}
	page, ok := m.Page(sessionID)
	if !ok {
		logging.BrowserError("Unknown session for navigation: %s", sessionID)
		return fmt.Errorf("unknown session: %s", sessionID)
	}
	logging.BrowserDebug("Navigating with timeout: %s", m.cfg.NavigationTimeout())
	err := page.Context(ctx).Timeout(m.cfg.NavigationTimeout()).Navigate(url)
	if err != nil {
		logging.BrowserError("Navigation failed for session %s: %v", sessionID, err)
	} else {
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
