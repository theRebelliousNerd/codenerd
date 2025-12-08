// Package browser provides browser automation with DOM/React reification into Mangle facts.
// Adapted from BrowserNERD for the Cortex 1.5.0 Browser Physics Engine (Section 9.0).
package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	m.mu.Lock()
	defer m.mu.Unlock()

	// If we already have a browser, verify it's still alive
	if m.browser != nil {
		_, err := m.browser.Version()
		if err == nil {
			return nil // Browser is healthy
		}
		log.Printf("Stale browser connection detected, reconnecting...")
		_ = m.browser.Close()
		m.browser = nil
		m.controlURL = ""
		m.sessions = make(map[string]*sessionRecord)
	}

	if err := m.loadSessionsLocked(); err != nil {
		return fmt.Errorf("load sessions: %w", err)
	}

	controlURL := m.cfg.DebuggerURL
	if controlURL == "" && len(m.cfg.Launch) > 0 {
		bin := m.cfg.Launch[0]
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
			// Fallback
			fallback := launcher.New().Bin(bin).Headless(m.cfg.IsHeadless())
			if alt, altErr := fallback.Launch(); altErr == nil {
				controlURL = alt
			} else {
				return fmt.Errorf("launch chrome: %w (fallback: %v)", err, altErr)
			}
		} else {
			controlURL = url
		}
	}

	if controlURL == "" {
		// Try default launcher
		url, err := launcher.New().Headless(m.cfg.IsHeadless()).Launch()
		if err != nil {
			return fmt.Errorf("no debugger_url and failed to launch: %w", err)
		}
		controlURL = url
	}

	browser := rod.New().ControlURL(controlURL).Context(ctx)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("connect to chrome: %w", err)
	}

	m.browser = browser
	m.controlURL = controlURL
	// Note: Browser connected - no log output to avoid TUI interference
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
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, record := range m.sessions {
		if record.page != nil {
			_ = record.page.Close()
		}
		delete(m.sessions, id)
	}

	var err error
	if m.browser != nil {
		err = m.browser.Close()
		m.browser = nil
	}
	m.controlURL = ""
	// Note: Browser shutdown complete - no log output to avoid TUI interference
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
	if err := m.ensureStarted(ctx); err != nil {
		return nil, err
	}
	if m.browser == nil {
		return nil, errors.New("browser not connected")
	}

	incognito, err := m.browser.Incognito()
	if err != nil {
		return nil, fmt.Errorf("incognito context: %w", err)
	}

	page, err := incognito.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, fmt.Errorf("create page: %w", err)
	}

	// Set viewport dimensions
	if err := (proto.EmulationSetDeviceMetricsOverride{
		Width:             m.cfg.GetViewportWidth(),
		Height:            m.cfg.GetViewportHeight(),
		DeviceScaleFactor: 1.0,
		Mobile:            false,
	}).Call(page); err != nil {
		log.Printf("warning: failed to set viewport: %v", err)
	}

	// Navigate
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

	m.startEventStream(ctx, meta.ID, page)
	_ = m.persistSessions()

	return &meta, nil
}

// Attach binds to an existing target by TargetID.
func (m *SessionManager) Attach(ctx context.Context, targetID string) (*Session, error) {
	if err := m.ensureStarted(ctx); err != nil {
		return nil, err
	}
	if m.browser == nil {
		return nil, errors.New("browser not connected")
	}

	page, err := m.browser.PageFromTarget(proto.TargetTargetID(targetID))
	if err != nil {
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

	m.startEventStream(ctx, meta.ID, page)
	_ = m.persistSessions()
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
		ID        string                 `json:"id"`
		Name      string                 `json:"name"`
		Parent    *string                `json:"parent"`
		Props     map[string]interface{} `json:"props"`
		State     [][]interface{}        `json:"state"`
		DomNodeID *string                `json:"domNodeId"`
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
			Args:      []interface{}{n.ID, n.Name, parent},
			Timestamp: now,
		})

		for k, v := range n.Props {
			facts = append(facts, mangle.Fact{
				Predicate: "react_prop",
				Args:      []interface{}{n.ID, k, fmt.Sprintf("%v", v)},
				Timestamp: now,
			})
		}

		for _, entry := range n.State {
			if len(entry) != 2 {
				continue
			}
			facts = append(facts, mangle.Fact{
				Predicate: "react_state",
				Args:      []interface{}{n.ID, entry[0], fmt.Sprintf("%v", entry[1])},
				Timestamp: now,
			})
		}

		if n.DomNodeID != nil && *n.DomNodeID != "" {
			facts = append(facts, mangle.Fact{
				Predicate: "dom_mapping",
				Args:      []interface{}{n.ID, *n.DomNodeID},
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
	if err := m.ensureStarted(ctx); err != nil {
		return err
	}
	page, ok := m.Page(sessionID)
	if !ok {
		return fmt.Errorf("unknown session: %s", sessionID)
	}
	return page.Context(ctx).Timeout(m.cfg.NavigationTimeout()).Navigate(url)
}

// Click clicks an element.
func (m *SessionManager) Click(ctx context.Context, sessionID, selector string) error {
	if err := m.ensureStarted(ctx); err != nil {
		return err
	}
	page, ok := m.Page(sessionID)
	if !ok {
		return fmt.Errorf("unknown session: %s", sessionID)
	}
	el, err := page.Context(ctx).Element(selector)
	if err != nil {
		return fmt.Errorf("element not found: %w", err)
	}
	return el.Click(proto.InputMouseButtonLeft, 1)
}

// Type types text into an element.
func (m *SessionManager) Type(ctx context.Context, sessionID, selector, text string) error {
	if err := m.ensureStarted(ctx); err != nil {
		return err
	}
	page, ok := m.Page(sessionID)
	if !ok {
		return fmt.Errorf("unknown session: %s", sessionID)
	}
	el, err := page.Context(ctx).Element(selector)
	if err != nil {
		return fmt.Errorf("element not found: %w", err)
	}
	return el.Input(text)
}

// Screenshot captures a screenshot.
func (m *SessionManager) Screenshot(ctx context.Context, sessionID string, fullPage bool) ([]byte, error) {
	if err := m.ensureStarted(ctx); err != nil {
		return nil, err
	}
	page, ok := m.Page(sessionID)
	if !ok {
		return nil, fmt.Errorf("unknown session: %s", sessionID)
	}
	if fullPage {
		return page.Context(ctx).Screenshot(true, nil)
	}
	return page.Context(ctx).Screenshot(false, nil)
}

// startEventStream wires Rod CDP events into the fact sink.
func (m *SessionManager) startEventStream(ctx context.Context, sessionID string, page *rod.Page) {
	if m.engine == nil {
		return
	}

	go func() {
		var wg sync.WaitGroup

		level := strings.ToLower(m.cfg.EventLoggingLevel)
		captureDOM := m.cfg.EnableDOMIngestion && level != "minimal"
		captureHeaders := m.cfg.EnableHeaderIngestion && level != "minimal"
		consoleErrorsOnly := level == "minimal"
		throttler := newEventThrottler(m.cfg.EventThrottleMs)

		// Optionally capture initial DOM snapshot
		if captureDOM {
			_ = proto.DOMEnable{}.Call(page)
			_ = m.captureDOMFacts(ctx, sessionID, page)
		}

		// Install lightweight click/input/state trackers
		_, _ = page.Context(ctx).Evaluate(&rod.EvalOptions{
			JS: `
			() => {
				const w = window;
				if (w.__browsernerdHooked) return true;
				w.__browsernerdHooked = true;
				w.__browsernerdEvents = [];

				document.addEventListener('click', (ev) => {
					try {
						const target = ev.target || {};
						const id = target.id || '';
						w.__browsernerdEvents.push({ type: 'click', id, ts: Date.now() });
					} catch (e) {}
				}, true);

				document.addEventListener('input', (ev) => {
					try {
						const target = ev.target || {};
						const id = target.id || target.name || '';
						const value = target.value || '';
						w.__browsernerdEvents.push({ type: 'input', id, value, ts: Date.now() });
					} catch (e) {}
				}, true);

				document.addEventListener('change', (ev) => {
					try {
						const target = ev.target || {};
						const id = target.id || target.name || '';
						const value = target.value || '';
						w.__browsernerdEvents.push({ type: 'input', id, value, ts: Date.now() });
					} catch (e) {}
				}, true);

				const obs = new MutationObserver((mutations) => {
					mutations.forEach((m) => {
						if (m.type === 'attributes' && m.attributeName && m.attributeName.startsWith('data-state')) {
							const val = (m.target && m.target.getAttribute) ? (m.target.getAttribute(m.attributeName) || '') : '';
							w.__browsernerdEvents.push({ type: 'state', name: m.attributeName, value: val, ts: Date.now() });
						}
					});
				});
				obs.observe(document.documentElement || document.body, { attributes: true, subtree: true });
				return true;
			}
			`,
			ByValue:      true,
			AwaitPromise: true,
		})

		// Navigation events
		waitNav := page.Context(ctx).EachEvent(func(ev *proto.PageFrameNavigated) {
			now := time.Now()
			facts := []mangle.Fact{
				{
					Predicate: "navigation_event",
					Args:      []interface{}{sessionID, ev.Frame.URL, now.UnixMilli()},
					Timestamp: now,
				},
				{
					Predicate: "current_url",
					Args:      []interface{}{sessionID, ev.Frame.URL},
					Timestamp: now,
				},
			}
			if err := m.engine.AddFacts(facts); err != nil {
				log.Printf("[session:%s] navigation fact error: %v", sessionID, err)
			}
			m.UpdateMetadata(sessionID, func(s Session) Session {
				s.URL = ev.Frame.URL
				s.LastActive = now
				return s
			})
		})

		// Console, network, and DOM streams
		waitRest := page.Context(ctx).EachEvent(
			func(ev *proto.RuntimeConsoleAPICalled) {
				if consoleErrorsOnly && ev.Type != proto.RuntimeConsoleAPICalledTypeError && ev.Type != proto.RuntimeConsoleAPICalledTypeWarning {
					return
				}
				if !throttler.Allow("console") {
					return
				}
				now := time.Now()
				msg := stringifyConsoleArgs(ev.Args)
				if err := m.engine.AddFacts([]mangle.Fact{{
					Predicate: "console_event",
					Args:      []interface{}{string(ev.Type), msg, now.UnixMilli()},
					Timestamp: now,
				}}); err != nil {
					log.Printf("[session:%s] console fact error: %v", sessionID, err)
				}
			},
			func(ev *proto.NetworkRequestWillBeSent) {
				if !throttler.Allow("net_request") {
					return
				}
				now := time.Now()
				initiatorType := ""
				initiatorID := ""
				initiatorScript := ""
				initiatorLineNo := 0

				if ev.Initiator != nil {
					initiatorType = string(ev.Initiator.Type)
					if ev.Initiator.RequestID != "" {
						initiatorID = string(ev.Initiator.RequestID)
					}
					if initiatorID == "" && ev.Initiator.URL != "" {
						initiatorID = ev.Initiator.URL
					}
					if ev.Initiator.Stack != nil && len(ev.Initiator.Stack.CallFrames) > 0 {
						frame := ev.Initiator.Stack.CallFrames[0]
						initiatorScript = frame.URL
						if initiatorScript == "" {
							initiatorScript = string(frame.ScriptID)
						}
						initiatorLineNo = frame.LineNumber
						for _, f := range ev.Initiator.Stack.CallFrames {
							if f.URL != "" && !isInternalScript(f.URL) {
								initiatorScript = f.URL
								initiatorLineNo = f.LineNumber
								break
							}
						}
					}
				}

				facts := []mangle.Fact{{
					Predicate: "net_request",
					Args:      []interface{}{string(ev.RequestID), ev.Request.Method, ev.Request.URL, initiatorType, now.UnixMilli()},
					Timestamp: now,
				}}

				if initiatorType != "" || initiatorID != "" || initiatorScript != "" {
					parentRef := coalesceNonEmpty(initiatorID, initiatorScript)
					if initiatorLineNo > 0 && initiatorScript != "" {
						parentRef = fmt.Sprintf("%s:%d", initiatorScript, initiatorLineNo)
					}
					facts = append(facts, mangle.Fact{
						Predicate: "request_initiator",
						Args:      []interface{}{string(ev.RequestID), initiatorType, parentRef},
						Timestamp: now,
					})
				}

				if err := m.engine.AddFacts(facts); err != nil {
					log.Printf("[session:%s] net_request fact error: %v", sessionID, err)
				}

				if captureHeaders && ev.Request != nil {
					for k, v := range ev.Request.Headers {
						if err := m.engine.AddFacts([]mangle.Fact{{
							Predicate: "net_header",
							Args:      []interface{}{string(ev.RequestID), "req", strings.ToLower(k), fmt.Sprintf("%v", v)},
							Timestamp: now,
						}}); err != nil {
							log.Printf("[session:%s] net_header fact error: %v", sessionID, err)
						}
					}
				}
			},
			func(ev *proto.NetworkResponseReceived) {
				if !throttler.Allow("net_response") {
					return
				}
				now := time.Now()
				var latency, duration int64
				if ev.Response != nil && ev.Response.Timing != nil {
					latency = int64(ev.Response.Timing.ReceiveHeadersEnd)
					duration = int64(ev.Response.Timing.ConnectEnd)
				}
				if err := m.engine.AddFacts([]mangle.Fact{{
					Predicate: "net_response",
					Args:      []interface{}{string(ev.RequestID), ev.Response.Status, latency, duration},
					Timestamp: now,
				}}); err != nil {
					log.Printf("[session:%s] net_response fact error: %v", sessionID, err)
				}

				if captureHeaders && ev.Response != nil {
					for k, v := range ev.Response.Headers {
						if err := m.engine.AddFacts([]mangle.Fact{{
							Predicate: "net_header",
							Args:      []interface{}{string(ev.RequestID), "res", strings.ToLower(k), fmt.Sprintf("%v", v)},
							Timestamp: now,
						}}); err != nil {
							log.Printf("[session:%s] res net_header fact error: %v", sessionID, err)
						}
					}
				}
			},
			func(ev *proto.DOMDocumentUpdated) {
				if !captureDOM {
					return
				}
				if !throttler.Allow("dom_update") {
					return
				}
				if err := m.captureDOMFacts(ctx, sessionID, page); err != nil {
					log.Printf("[session:%s] DOM capture error: %v", sessionID, err)
				}
			},
		)

		wg.Add(3)
		go func() {
			defer wg.Done()
			waitNav()
		}()
		go func() {
			defer wg.Done()
			waitRest()
		}()
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					res, err := page.Context(ctx).Evaluate(&rod.EvalOptions{
						JS: `
						() => {
							const buf = Array.isArray(window.__browsernerdEvents) ? window.__browsernerdEvents : [];
							window.__browsernerdEvents = [];
							return buf;
						}
						`,
						ByValue:      true,
						AwaitPromise: true,
					})
					if err != nil || res == nil {
						continue
					}
					if res.Value.Nil() {
						continue
					}
					raw, err := res.Value.MarshalJSON()
					if err != nil {
						continue
					}
					var events []struct {
						Type  string  `json:"type"`
						ID    string  `json:"id"`
						Name  string  `json:"name"`
						Value string  `json:"value"`
						TS    float64 `json:"ts"`
					}
					if err := json.Unmarshal(raw, &events); err != nil {
						continue
					}

					facts := make([]mangle.Fact, 0, len(events))
					for _, ev := range events {
						ts := time.UnixMilli(int64(ev.TS))
						switch ev.Type {
						case "click":
							facts = append(facts, mangle.Fact{
								Predicate: "click_event",
								Args:      []interface{}{ev.ID, ts.UnixMilli()},
								Timestamp: ts,
							})
						case "input":
							facts = append(facts, mangle.Fact{
								Predicate: "input_event",
								Args:      []interface{}{ev.ID, ev.Value, ts.UnixMilli()},
								Timestamp: ts,
							})
						case "state":
							facts = append(facts, mangle.Fact{
								Predicate: "state_change",
								Args:      []interface{}{ev.Name, ev.Value, ts.UnixMilli()},
								Timestamp: ts,
							})
						}
					}
					if len(facts) > 0 {
						if err := m.engine.AddFacts(facts); err != nil {
							log.Printf("[session:%s] click/state fact error: %v", sessionID, err)
						}
					}
				}
			}
		}()
		wg.Wait()
	}()
}

func stringifyConsoleArgs(args []*proto.RuntimeRemoteObject) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		if a == nil {
			continue
		}
		if !a.Value.Nil() {
			parts = append(parts, a.Value.String())
			continue
		}
		if a.Description != "" {
			parts = append(parts, a.Description)
		}
	}
	return strings.Join(parts, " ")
}

// captureDOMFacts snapshots a limited DOM view into facts.
func (m *SessionManager) captureDOMFacts(ctx context.Context, sessionID string, page *rod.Page) error {
	const maxNodes = 200
	script := fmt.Sprintf(`
	() => {
		const nodes = Array.from(document.querySelectorAll('*')).slice(0, %d);
		return nodes.map((el, idx) => {
			const attrs = {};
			for (const { name, value } of Array.from(el.attributes || [])) {
				attrs[name] = value;
			}
			const rect = el.getBoundingClientRect();
			const style = window.getComputedStyle(el);
			const isVisible = style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0' && rect.width > 0 && rect.height > 0;

			return {
				id: el.id || ('node_' + idx),
				tag: el.tagName,
				text: (el.innerText || '').slice(0, 256),
				parent: el.parentElement && (el.parentElement.id || el.parentElement.tagName || 'root'),
				attrs,
				layout: {
					x: rect.x,
					y: rect.y,
					width: rect.width,
					height: rect.height,
					visible: isVisible
				}
			};
		});
	}
	`, maxNodes)

	res, err := page.Context(ctx).Evaluate(&rod.EvalOptions{
		JS:           script,
		ByValue:      true,
		AwaitPromise: true,
	})
	if err != nil || res == nil {
		return err
	}

	raw, err := res.Value.MarshalJSON()
	if err != nil {
		return err
	}

	var nodes []struct {
		ID     string            `json:"id"`
		Tag    string            `json:"tag"`
		Text   string            `json:"text"`
		Parent string            `json:"parent"`
		Attrs  map[string]string `json:"attrs"`
		Layout struct {
			X       float64 `json:"x"`
			Y       float64 `json:"y"`
			Width   float64 `json:"width"`
			Height  float64 `json:"height"`
			Visible bool    `json:"visible"`
		} `json:"layout"`
	}
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return err
	}

	now := time.Now()
	facts := make([]mangle.Fact, 0, len(nodes)*3)
	for _, n := range nodes {
		// Include sessionID in all facts to associate DOM with specific browser session
		facts = append(facts, mangle.Fact{
			Predicate: "dom_node",
			Args:      []interface{}{sessionID, n.ID, n.Tag, n.Text, n.Parent},
			Timestamp: now,
		})
		if n.Text != "" {
			facts = append(facts, mangle.Fact{
				Predicate: "dom_text",
				Args:      []interface{}{sessionID, n.ID, n.Text},
				Timestamp: now,
			})
		}
		for k, v := range n.Attrs {
			facts = append(facts, mangle.Fact{
				Predicate: "dom_attr",
				Args:      []interface{}{sessionID, n.ID, k, v},
				Timestamp: now,
			})
		}
		facts = append(facts, mangle.Fact{
			Predicate: "dom_layout",
			Args:      []interface{}{sessionID, n.ID, n.Layout.X, n.Layout.Y, n.Layout.Width, n.Layout.Height, fmt.Sprintf("%v", n.Layout.Visible)},
			Timestamp: now,
		})
	}
	return m.engine.AddFacts(facts)
}

// SnapshotDOM triggers a one-off DOM capture for the given session.
func (m *SessionManager) SnapshotDOM(ctx context.Context, sessionID string) error {
	if err := m.ensureStarted(ctx); err != nil {
		return err
	}
	page, ok := m.Page(sessionID)
	if !ok {
		return fmt.Errorf("unknown session: %s", sessionID)
	}
	return m.captureDOMFacts(ctx, sessionID, page)
}

func snapshotStorage(page *rod.Page, store string) string {
	jsFunc := fmt.Sprintf(`() => {
		try {
			const out = {};
			for (const key of Object.keys(%s)) {
				out[key] = %s.getItem(key);
			}
			return JSON.stringify(out);
		} catch (e) {
			return "{}";
		}
	}`, store, store)

	res, err := page.Evaluate(&rod.EvalOptions{
		JS:           jsFunc,
		ByValue:      true,
		AwaitPromise: true,
	})
	if err != nil || res == nil || res.Value.Nil() {
		return "{}"
	}
	return res.Value.String()
}

func restoreStorage(page *rod.Page, localJSON, sessionJSON string) {
	_, _ = page.Evaluate(&rod.EvalOptions{
		JS: `
		(local, session) => {
			try {
				const l = JSON.parse(local || "{}");
				Object.entries(l).forEach(([k, v]) => localStorage.setItem(k, v));
			} catch (e) {}
			try {
				const s = JSON.parse(session || "{}");
				Object.entries(s).forEach(([k, v]) => sessionStorage.setItem(k, v));
			} catch (e) {}
		}
		`,
		JSArgs:       []interface{}{localJSON, sessionJSON},
		ByValue:      true,
		AwaitPromise: true,
		UserGesture:  true,
	})
}

// persistSessions writes session metadata to disk.
func (m *SessionManager) persistSessions() error {
	if m.cfg.SessionStore == "" {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]Session, 0, len(m.sessions))
	for _, rec := range m.sessions {
		sessions = append(sessions, rec.meta)
	}

	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(m.cfg.SessionStore), 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.cfg.SessionStore, data, 0o644)
}

// loadSessionsLocked loads persisted metadata. Caller must hold lock.
func (m *SessionManager) loadSessionsLocked() error {
	if m.cfg.SessionStore == "" {
		return nil
	}

	data, err := os.ReadFile(m.cfg.SessionStore)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var sessions []Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return err
	}

	for _, s := range sessions {
		s.Status = "detached"
		m.sessions[s.ID] = &sessionRecord{meta: s, page: nil}
	}
	return nil
}

func coalesceNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func isInternalScript(url string) bool {
	internalPrefixes := []string{
		"chrome://",
		"chrome-extension://",
		"devtools://",
		"about:",
		"data:",
		"blob:",
	}
	for _, prefix := range internalPrefixes {
		if strings.HasPrefix(url, prefix) {
			return true
		}
	}
	return false
}
