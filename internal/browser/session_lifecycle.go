package browser

// Lifecycle semantics in this file are adapted from BrowserNERD's Apache-2.0
// browser instance and session manager. codeNERD keeps one native manager and
// routes its evidence into the Cortex kernel instead of embedding BrowserNERD's
// standalone MCP server or a second logic engine.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
	"github.com/google/uuid"
)

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (m *SessionManager) startDefault(ctx context.Context) (string, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return "", err
	}

	m.startMu.Lock()
	defer m.startMu.Unlock()

	m.mu.RLock()
	current := m.browser
	currentID := m.defaultID
	m.mu.RUnlock()
	if current != nil {
		healthCtx, healthCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, healthErr := current.Context(healthCtx).Version()
		healthCancel()
		if healthErr == nil {
			m.mu.Lock()
			if m.browsers == nil {
				m.browsers = make(map[string]*browserRecord)
			}
			if currentID == "" {
				currentID = uuid.NewString()
				m.defaultID = currentID
			}
			if _, ok := m.browsers[currentID]; !ok {
				m.browsers[currentID] = &browserRecord{
					meta:    BrowserInstance{ID: currentID, ControlURL: m.controlURL, Default: true, CreatedAt: time.Now()},
					browser: current,
				}
			}
			m.startReaperLocked()
			m.mu.Unlock()
			return currentID, nil
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		logging.BrowserWarn("Stale browser connection detected, reconnecting")
		_ = current.Close()
		m.mu.Lock()
		if record := m.browsers[currentID]; record != nil && record.cancel != nil {
			record.cancel()
		}
		for _, session := range m.sessions {
			if session.streamCancel != nil {
				session.streamCancel()
			}
		}
		m.browser = nil
		m.controlURL = ""
		m.defaultID = ""
		m.sessions = make(map[string]*sessionRecord)
		m.browsers = make(map[string]*browserRecord)
		m.mu.Unlock()
	}

	m.mu.Lock()
	if m.sessions == nil {
		m.sessions = make(map[string]*sessionRecord)
	}
	if m.browsers == nil {
		m.browsers = make(map[string]*browserRecord)
	}
	if err := m.loadSessionsLocked(); err != nil {
		m.mu.Unlock()
		return "", fmt.Errorf("load sessions: %w", err)
	}
	m.mu.Unlock()

	controlURL, err := m.launchControlURL(ctx, true)
	if err != nil {
		return "", err
	}
	browser, browserCancel, err := connectBrowser(ctx, controlURL)
	if err != nil {
		return "", fmt.Errorf("connect to chrome: %w", err)
	}

	id := uuid.NewString()
	now := time.Now()
	m.mu.Lock()
	m.browser = browser
	m.controlURL = controlURL
	m.defaultID = id
	m.browsers[id] = &browserRecord{
		meta:    BrowserInstance{ID: id, ControlURL: controlURL, Default: true, CreatedAt: now},
		browser: browser,
		cancel:  browserCancel,
	}
	m.startReaperLocked()
	m.mu.Unlock()
	logging.Browser("Browser session manager started successfully")
	return id, nil
}

func connectBrowser(ctx context.Context, controlURL string) (*rod.Browser, context.CancelFunc, error) {
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	connecting := rod.New().ControlURL(controlURL).Context(lifecycleCtx)
	connected := make(chan error, 1)
	go func() {
		connected <- connecting.Connect()
	}()
	select {
	case err := <-connected:
		if err != nil {
			lifecycleCancel()
			return nil, nil, err
		}
		return connecting, lifecycleCancel, nil
	case <-ctx.Done():
		lifecycleCancel()
		return nil, nil, ctx.Err()
	}
}

func (m *SessionManager) launchControlURL(ctx context.Context, allowDebugger bool) (string, error) {
	if allowDebugger && m.cfg.DebuggerURL != "" {
		return m.cfg.DebuggerURL, nil
	}
	if len(m.cfg.Launch) > 0 {
		bin := m.cfg.Launch[0]
		launch := launcher.New().Context(ctx).Bin(bin).Headless(m.cfg.IsHeadless())
		for _, rawFlag := range m.cfg.Launch[1:] {
			flagText := strings.TrimLeft(rawFlag, "-")
			name, value, hasValue := strings.Cut(flagText, "=")
			if hasValue {
				launch = launch.Set(flags.Flag(name), value)
			} else {
				launch = launch.Set(flags.Flag(name))
			}
		}
		controlURL, err := launch.Launch()
		if err == nil {
			return controlURL, nil
		}
		logging.BrowserWarn("Chrome launch failed, trying configured binary without extra flags: %v", err)
		fallbackURL, fallbackErr := launcher.New().Context(ctx).Bin(bin).Headless(m.cfg.IsHeadless()).Launch()
		if fallbackErr != nil {
			return "", fmt.Errorf("launch chrome: %w (fallback: %v)", err, fallbackErr)
		}
		return fallbackURL, nil
	}
	controlURL, err := launcher.New().Context(ctx).Headless(m.cfg.IsHeadless()).Launch()
	if err != nil {
		return "", fmt.Errorf("no debugger_url and failed to launch: %w", err)
	}
	return controlURL, nil
}

// LaunchAdditional launches and tracks another independent browser process.
func (m *SessionManager) LaunchAdditional(ctx context.Context) (*BrowserInstance, error) {
	ctx = normalizeContext(ctx)
	if _, err := m.startDefault(ctx); err != nil {
		return nil, err
	}

	m.startMu.Lock()
	defer m.startMu.Unlock()
	m.mu.RLock()
	full := len(m.browsers) >= m.cfg.GetMaxBrowsers()
	m.mu.RUnlock()
	if full {
		return nil, fmt.Errorf("browser limit reached: %d", m.cfg.GetMaxBrowsers())
	}

	controlURL, err := m.launchControlURL(ctx, false)
	if err != nil {
		return nil, err
	}
	browser, browserCancel, err := connectBrowser(ctx, controlURL)
	if err != nil {
		return nil, fmt.Errorf("connect additional browser: %w", err)
	}

	record := &browserRecord{
		meta:    BrowserInstance{ID: uuid.NewString(), ControlURL: controlURL, CreatedAt: time.Now()},
		browser: browser,
		cancel:  browserCancel,
	}
	m.mu.Lock()
	if len(m.browsers) >= m.cfg.GetMaxBrowsers() {
		m.mu.Unlock()
		_ = browser.Close()
		browserCancel()
		return nil, fmt.Errorf("browser limit reached: %d", m.cfg.GetMaxBrowsers())
	}
	m.browsers[record.meta.ID] = record
	m.mu.Unlock()
	meta := record.meta
	return &meta, nil
}

// ListBrowsers returns deterministic metadata for all managed browser processes.
func (m *SessionManager) ListBrowsers() []BrowserInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]BrowserInstance, 0, len(m.browsers))
	for _, record := range m.browsers {
		meta := record.meta
		meta.TabCount = 0
		for _, session := range m.sessions {
			if session.meta.BrowserID == meta.ID && session.page != nil {
				meta.TabCount++
			}
		}
		results = append(results, meta)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Default != results[j].Default {
			return results[i].Default
		}
		if results[i].CreatedAt.Equal(results[j].CreatedAt) {
			return results[i].ID < results[j].ID
		}
		return results[i].CreatedAt.Before(results[j].CreatedAt)
	})
	return results
}

func (m *SessionManager) resolveBrowserLocked(browserID string) (string, *rod.Browser, error) {
	if browserID == "" {
		browserID = m.defaultID
	}
	record, ok := m.browsers[browserID]
	if !ok || record.browser == nil {
		return "", nil, fmt.Errorf("unknown browser: %s", browserID)
	}
	return browserID, record.browser, nil
}

func (m *SessionManager) liveTabCountLocked() int {
	count := 0
	for _, session := range m.sessions {
		if session.page != nil {
			count++
		}
	}
	return count
}

func (m *SessionManager) reserveTab() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.liveTabCountLocked()+m.pendingTabs >= m.cfg.GetMaxTabs() {
		return fmt.Errorf("tab limit reached: %d", m.cfg.GetMaxTabs())
	}
	m.pendingTabs++
	return nil
}

func (m *SessionManager) releaseTabReservation() {
	m.mu.Lock()
	if m.pendingTabs > 0 {
		m.pendingTabs--
	}
	m.mu.Unlock()
}

// CreateTab opens a shared-profile tab unless isolated is explicitly requested.
func (m *SessionManager) CreateTab(ctx context.Context, browserID, url string, isolated bool) (*Session, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := m.startDefault(ctx); err != nil {
		return nil, err
	}
	m.startMu.Lock()
	defer m.startMu.Unlock()
	if err := m.reserveTab(); err != nil {
		return nil, err
	}
	defer m.releaseTabReservation()

	m.mu.RLock()
	browserID, base, err := m.resolveBrowserLocked(browserID)
	m.mu.RUnlock()
	if err != nil {
		return nil, err
	}

	pageBrowser := base
	var isolatedBrowser *rod.Browser
	if isolated {
		isolatedBrowser, err = base.Incognito()
		if err != nil {
			return nil, fmt.Errorf("create isolated context: %w", err)
		}
		isolatedBrowser = isolatedBrowser.Context(context.Background())
		pageBrowser = isolatedBrowser
	}
	page, err := pageBrowser.Page(proto.TargetCreateTarget{})
	if err != nil {
		if isolatedBrowser != nil {
			_ = isolatedBrowser.Close()
		}
		return nil, fmt.Errorf("create page: %w", err)
	}
	page = page.Context(context.Background())
	if err := setViewport(page, m.cfg); err != nil {
		logging.BrowserWarn("Failed to set viewport: %v", err)
	}
	if url == "" {
		url = "about:blank"
	}
	if err := page.Context(ctx).Timeout(m.cfg.NavigationTimeout()).Navigate(url); err != nil {
		logging.BrowserWarn("Initial navigation failed for %s: %v", m.SanitizeForEvidence(url), err)
	}
	actualURL := url
	title := ""
	if info, infoErr := page.Context(ctx).Info(); infoErr == nil && info != nil {
		if info.URL != "" {
			actualURL = info.URL
		}
		title = info.Title
	}

	now := time.Now()
	meta := Session{
		ID: uuid.NewString(), BrowserID: browserID, TargetID: string(page.TargetID),
		URL: m.redactor.SanitizeString(actualURL), Title: m.redactor.SanitizeString(title),
		Status: "active", Isolated: isolated, CreatedAt: now, LastActive: now,
	}
	streamCtx, streamCancel := context.WithCancel(context.Background())
	record := &sessionRecord{meta: meta, page: page, isolated: isolatedBrowser, streamCancel: streamCancel, registry: NewElementRegistry()}
	m.mu.Lock()
	m.sessions[meta.ID] = record
	m.mu.Unlock()
	m.startEventStream(streamCtx, meta.ID, page)
	_ = m.persistSessions()
	return &meta, nil
}

func setViewport(page *rod.Page, cfg Config) error {
	return (proto.EmulationSetDeviceMetricsOverride{
		Width: cfg.GetViewportWidth(), Height: cfg.GetViewportHeight(),
		DeviceScaleFactor: 1.0, Mobile: false,
	}).Call(page)
}

// AttachToBrowser tracks an existing target on a selected browser.
func (m *SessionManager) AttachToBrowser(ctx context.Context, browserID, targetID string) (*Session, error) {
	ctx = normalizeContext(ctx)
	if _, err := m.startDefault(ctx); err != nil {
		return nil, err
	}
	m.startMu.Lock()
	defer m.startMu.Unlock()
	if err := m.reserveTab(); err != nil {
		return nil, err
	}
	defer m.releaseTabReservation()
	m.mu.RLock()
	browserID, selected, err := m.resolveBrowserLocked(browserID)
	m.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	page, err := selected.PageFromTarget(proto.TargetTargetID(targetID))
	if err != nil {
		return nil, fmt.Errorf("attach to target %s: %w", targetID, err)
	}
	page = page.Context(context.Background())
	now := time.Now()
	meta := Session{ID: uuid.NewString(), BrowserID: browserID, TargetID: targetID, Status: "attached", CreatedAt: now, LastActive: now}
	if info, infoErr := page.Context(ctx).Info(); infoErr == nil && info != nil {
		meta.URL = m.redactor.SanitizeString(info.URL)
		meta.Title = m.redactor.SanitizeString(info.Title)
	}
	streamCtx, streamCancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.sessions[meta.ID] = &sessionRecord{meta: meta, page: page, streamCancel: streamCancel, registry: NewElementRegistry()}
	m.mu.Unlock()
	m.startEventStream(streamCtx, meta.ID, page)
	_ = m.persistSessions()
	return &meta, nil
}

// FocusSession activates the tab and refreshes its activity timestamp.
func (m *SessionManager) FocusSession(ctx context.Context, sessionID string) error {
	page, ok := m.Page(sessionID)
	if !ok || page == nil {
		return fmt.Errorf("unknown session: %s", sessionID)
	}
	if _, err := page.Context(normalizeContext(ctx)).Activate(); err != nil {
		return fmt.Errorf("focus session %s: %w", sessionID, err)
	}
	return nil
}

// CloseSession closes and forgets a tab. Repeated closes are harmless.
func (m *SessionManager) CloseSession(_ context.Context, sessionID string) error {
	m.mu.Lock()
	record, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}
	if record.streamCancel != nil {
		record.streamCancel()
	}
	closeSessionResources(record)
	_ = m.persistSessions()
	return nil
}

func closeSessionResources(record *sessionRecord) {
	if record == nil {
		return
	}
	if record.page != nil {
		_ = record.page.Close()
	}
	if record.isolated != nil {
		_ = record.isolated.Close()
	}
}

// CloseBrowser closes a managed browser and every tab attached to it.
func (m *SessionManager) CloseBrowser(_ context.Context, browserID string) error {
	m.startMu.Lock()
	defer m.startMu.Unlock()
	m.mu.Lock()
	record, ok := m.browsers[browserID]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	delete(m.browsers, browserID)
	sessions := make([]*sessionRecord, 0)
	for id, session := range m.sessions {
		if session.meta.BrowserID == browserID {
			sessions = append(sessions, session)
			delete(m.sessions, id)
		}
	}
	if browserID == m.defaultID {
		m.promoteDefaultLocked()
	}
	m.mu.Unlock()

	for _, session := range sessions {
		if session.streamCancel != nil {
			session.streamCancel()
		}
		closeSessionResources(session)
	}
	if record.browser != nil {
		err := record.browser.Close()
		if record.cancel != nil {
			record.cancel()
		}
		return err
	}
	if record.cancel != nil {
		record.cancel()
	}
	return nil
}

func (m *SessionManager) promoteDefaultLocked() {
	for _, record := range m.browsers {
		record.meta.Default = false
	}
	m.defaultID = ""
	m.browser = nil
	m.controlURL = ""
	var selected *browserRecord
	for _, candidate := range m.browsers {
		if selected == nil || candidate.meta.CreatedAt.Before(selected.meta.CreatedAt) {
			selected = candidate
		}
	}
	if selected != nil {
		selected.meta.Default = true
		m.defaultID = selected.meta.ID
		m.browser = selected.browser
		m.controlURL = selected.meta.ControlURL
	}
}

func (m *SessionManager) shutdown(_ context.Context) error {
	logging.Browser("Shutting down browser session manager")
	m.startMu.Lock()
	defer m.startMu.Unlock()
	m.mu.Lock()
	if m.reaperCancel != nil {
		m.reaperCancel()
		m.reaperCancel = nil
	}
	sessions := make([]*sessionRecord, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	type managedBrowser struct {
		browser *rod.Browser
		cancel  context.CancelFunc
	}
	browsers := make([]managedBrowser, 0, len(m.browsers)+1)
	seen := make(map[*rod.Browser]struct{})
	for _, record := range m.browsers {
		if record.browser != nil {
			seen[record.browser] = struct{}{}
			browsers = append(browsers, managedBrowser{browser: record.browser, cancel: record.cancel})
		}
	}
	if m.browser != nil {
		if _, ok := seen[m.browser]; !ok {
			browsers = append(browsers, managedBrowser{browser: m.browser})
		}
	}
	m.sessions = make(map[string]*sessionRecord)
	m.browsers = make(map[string]*browserRecord)
	m.pendingTabs = 0
	m.browser = nil
	m.defaultID = ""
	m.controlURL = ""
	m.mu.Unlock()

	for _, session := range sessions {
		if session.streamCancel != nil {
			session.streamCancel()
		}
		closeSessionResources(session)
	}
	var result error
	for _, managed := range browsers {
		if err := managed.browser.Close(); err != nil {
			result = errors.Join(result, err)
		}
		if managed.cancel != nil {
			managed.cancel()
		}
	}
	return result
}

func (m *SessionManager) startReaperLocked() {
	timeout := m.cfg.GetIdleTabTimeout()
	if timeout <= 0 || m.reaperCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.reaperCancel = cancel
	interval := timeout / 2
	if interval < 100*time.Millisecond {
		interval = 100 * time.Millisecond
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				m.reapIdleTabs(now)
			}
		}
	}()
}

func (m *SessionManager) reapIdleTabs(now time.Time) {
	timeout := m.cfg.GetIdleTabTimeout()
	if timeout <= 0 {
		return
	}
	m.mu.RLock()
	ids := make([]string, 0)
	for id, session := range m.sessions {
		if now.Sub(session.meta.LastActive) >= timeout {
			ids = append(ids, id)
		}
	}
	m.mu.RUnlock()
	for _, id := range ids {
		_ = m.CloseSession(context.Background(), id)
	}
}
