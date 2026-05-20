package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"codenerd/internal/mangle"

	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/gson"
)

// --- Config Tests ---

func TestDefaultConfig_WhenCalled_ShouldReturnSensibleDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Headless != false {
		t.Errorf("Expected Headless=false, got %v", cfg.Headless)
	}
	if cfg.ViewportWidth != 1920 {
		t.Errorf("Expected ViewportWidth=1920, got %d", cfg.ViewportWidth)
	}
	if cfg.ViewportHeight != 1080 {
		t.Errorf("Expected ViewportHeight=1080, got %d", cfg.ViewportHeight)
	}
	if cfg.NavigationTimeoutMs != 30000 {
		t.Errorf("Expected NavigationTimeoutMs=30000, got %d", cfg.NavigationTimeoutMs)
	}
	if cfg.EventLoggingLevel != "normal" {
		t.Errorf("Expected EventLoggingLevel='normal', got %q", cfg.EventLoggingLevel)
	}
	if cfg.EnableDOMIngestion != true {
		t.Errorf("Expected EnableDOMIngestion=true, got %v", cfg.EnableDOMIngestion)
	}
	if cfg.EventThrottleMs != 100 {
		t.Errorf("Expected EventThrottleMs=100, got %d", cfg.EventThrottleMs)
	}
}

func TestConfig_IsHeadless_WhenTrue_ShouldReturnTrue(t *testing.T) {
	cfg := Config{Headless: true}
	if !cfg.IsHeadless() {
		t.Error("Expected IsHeadless() to return true")
	}
}

func TestConfig_IsHeadless_WhenFalse_ShouldReturnFalse(t *testing.T) {
	cfg := Config{Headless: false}
	if cfg.IsHeadless() {
		t.Error("Expected IsHeadless() to return false")
	}
}

func TestConfig_GetViewportWidth_WhenSet_ShouldReturnValue(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		expected int
	}{
		{"Zero returns default", 0, 1920},
		{"Custom value", 1280, 1280},
		{"Small value", 320, 320},
		{"Large value", 3840, 3840},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{ViewportWidth: tc.width}
			got := cfg.GetViewportWidth()
			if got != tc.expected {
				t.Errorf("GetViewportWidth()=%d, want %d", got, tc.expected)
			}
		})
	}
}

func TestConfig_GetViewportHeight_WhenSet_ShouldReturnValue(t *testing.T) {
	tests := []struct {
		name     string
		height   int
		expected int
	}{
		{"Zero returns default", 0, 1080},
		{"Custom value", 720, 720},
		{"Small value", 240, 240},
		{"Large value", 2160, 2160},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{ViewportHeight: tc.height}
			got := cfg.GetViewportHeight()
			if got != tc.expected {
				t.Errorf("GetViewportHeight()=%d, want %d", got, tc.expected)
			}
		})
	}
}

func TestConfig_NavigationTimeout_WhenSet_ShouldReturnDuration(t *testing.T) {
	tests := []struct {
		name      string
		timeoutMs int
		expected  time.Duration
	}{
		{"Zero returns default 30s", 0, 30 * time.Second},
		{"5000ms returns 5s", 5000, 5 * time.Second},
		{"1ms returns 1ms", 1, 1 * time.Millisecond},
		{"60000ms returns 60s", 60000, 60 * time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{NavigationTimeoutMs: tc.timeoutMs}
			got := cfg.NavigationTimeout()
			if got != tc.expected {
				t.Errorf("NavigationTimeout()=%v, want %v", got, tc.expected)
			}
		})
	}
}

// --- Event Throttler Tests ---

func TestNewEventThrottler_WhenZeroOrNegative_ShouldReturnNil(t *testing.T) {
	tests := []struct {
		name string
		ms   int
	}{
		{"Zero", 0},
		{"Negative", -1},
		{"Large negative", -1000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			throttler := newEventThrottler(tc.ms)
			if throttler != nil {
				t.Error("Expected nil throttler for non-positive ms")
			}
		})
	}
}

func TestNewEventThrottler_WhenPositive_ShouldReturnThrottler(t *testing.T) {
	throttler := newEventThrottler(100)
	if throttler == nil {
		t.Fatal("Expected non-nil throttler")
	}
	if throttler.interval != 100*time.Millisecond {
		t.Errorf("Expected interval=100ms, got %v", throttler.interval)
	}
	if throttler.last == nil {
		t.Error("Expected non-nil last map")
	}
}

func TestEventThrottler_Allow_WhenNil_ShouldAlwaysAllow(t *testing.T) {
	var throttler *eventThrottler
	if !throttler.Allow("any_key") {
		t.Error("Nil throttler should always allow")
	}
}

func TestEventThrottler_Allow_WhenFirstCall_ShouldAllow(t *testing.T) {
	throttler := newEventThrottler(1000) // 1s interval
	if !throttler.Allow("test_key") {
		t.Error("First call should always be allowed")
	}
}

func TestEventThrottler_Allow_WhenCalledWithinInterval_ShouldDeny(t *testing.T) {
	throttler := newEventThrottler(1000) // 1s interval

	if !throttler.Allow("test_key") {
		t.Fatal("First call should be allowed")
	}
	// Second call immediately should be denied
	if throttler.Allow("test_key") {
		t.Error("Second call within interval should be denied")
	}
}

func TestEventThrottler_Allow_WhenDifferentKeys_ShouldAllowBoth(t *testing.T) {
	throttler := newEventThrottler(1000)

	if !throttler.Allow("key1") {
		t.Error("First call for key1 should be allowed")
	}
	if !throttler.Allow("key2") {
		t.Error("First call for key2 should be allowed (different key)")
	}
	// key1 still throttled
	if throttler.Allow("key1") {
		t.Error("key1 should still be throttled")
	}
}

func TestEventThrottler_Allow_WhenConcurrent_ShouldBeSafe(t *testing.T) {
	throttler := newEventThrottler(1)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			throttler.Allow("concurrent_key")
		}()
	}
	wg.Wait()
	// No race condition = pass
}

// --- Session Manager Construction Tests ---

func TestNewSessionManager_WhenNilEngine_ShouldCreateWithNilSink(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManager(cfg, nil)
	if sm == nil {
		t.Fatal("Expected non-nil SessionManager")
	}
	if sm.engine != nil {
		t.Error("Expected nil engine sink when engine is nil")
	}
	if sm.sessions == nil {
		t.Error("Expected non-nil sessions map")
	}
}

func TestNewSessionManager_WhenEngineProvided_ShouldWrapInAdapter(t *testing.T) {
	cfg := DefaultConfig()
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	sm := NewSessionManager(cfg, engine)
	if sm == nil {
		t.Fatal("Expected non-nil SessionManager")
	}
	if sm.engine == nil {
		t.Error("Expected non-nil engine sink when engine provided")
	}
}

// mockEngineSink implements EngineSink for testing.
type mockEngineSink struct {
	mu       sync.Mutex
	facts    []mangle.Fact
	addErr   error
	addCount int
}

func (m *mockEngineSink) AddFacts(facts []mangle.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addCount++
	if m.addErr != nil {
		return m.addErr
	}
	m.facts = append(m.facts, facts...)
	return nil
}

func (m *mockEngineSink) getFacts() []mangle.Fact {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]mangle.Fact, len(m.facts))
	copy(copied, m.facts)
	return copied
}

func TestNewSessionManagerWithSink_WhenCustomSink_ShouldUseIt(t *testing.T) {
	cfg := DefaultConfig()
	sink := &mockEngineSink{}
	sm := NewSessionManagerWithSink(cfg, sink)
	if sm == nil {
		t.Fatal("Expected non-nil SessionManager")
	}
	if sm.engine != sink {
		t.Error("Expected engine to be the provided sink")
	}
}

func TestNewSessionManagerWithSink_WhenNilSink_ShouldAcceptNil(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManagerWithSink(cfg, nil)
	if sm == nil {
		t.Fatal("Expected non-nil SessionManager")
	}
	if sm.engine != nil {
		t.Error("Expected nil engine when nil sink provided")
	}
}

// --- Session Manager State Tests ---

func TestSessionManager_ControlURL_WhenNotStarted_ShouldReturnEmpty(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManagerWithSink(cfg, nil)
	url := sm.ControlURL()
	if url != "" {
		t.Errorf("Expected empty control URL, got %q", url)
	}
}

func TestSessionManager_IsConnected_WhenNotStarted_ShouldReturnFalse(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManagerWithSink(cfg, nil)
	if sm.IsConnected() {
		t.Error("Expected IsConnected()=false when not started")
	}
}

func TestSessionManager_List_WhenEmpty_ShouldReturnEmptySlice(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManagerWithSink(cfg, nil)
	sessions := sm.List()
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions, got %d", len(sessions))
	}
}

func TestSessionManager_Page_WhenUnknownSession_ShouldReturnFalse(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManagerWithSink(cfg, nil)
	page, ok := sm.Page("nonexistent-id")
	if ok {
		t.Error("Expected ok=false for unknown session")
	}
	if page != nil {
		t.Error("Expected nil page for unknown session")
	}
}

func TestSessionManager_GetSession_WhenUnknownSession_ShouldReturnFalse(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManagerWithSink(cfg, nil)
	session, ok := sm.GetSession("nonexistent-id")
	if ok {
		t.Error("Expected ok=false for unknown session")
	}
	if session.ID != "" {
		t.Errorf("Expected empty session ID, got %q", session.ID)
	}
}

func TestSessionManager_GetSession_WhenSessionExists_ShouldReturnMeta(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManagerWithSink(cfg, nil)

	// Manually inject a session
	now := time.Now()
	sm.sessions["test-id"] = &sessionRecord{
		meta: Session{
			ID:         "test-id",
			TargetID:   "target-1",
			URL:        "https://example.com",
			Title:      "Example",
			Status:     "active",
			CreatedAt:  now,
			LastActive: now,
		},
	}

	session, ok := sm.GetSession("test-id")
	if !ok {
		t.Fatal("Expected ok=true for existing session")
	}
	if session.ID != "test-id" {
		t.Errorf("Expected ID='test-id', got %q", session.ID)
	}
	if session.URL != "https://example.com" {
		t.Errorf("Expected URL='https://example.com', got %q", session.URL)
	}
	if session.Status != "active" {
		t.Errorf("Expected Status='active', got %q", session.Status)
	}
}

func TestSessionManager_List_WhenSessionsExist_ShouldReturnAll(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManagerWithSink(cfg, nil)

	now := time.Now()
	sm.sessions["id-1"] = &sessionRecord{
		meta: Session{ID: "id-1", URL: "https://a.com", CreatedAt: now, LastActive: now},
	}
	sm.sessions["id-2"] = &sessionRecord{
		meta: Session{ID: "id-2", URL: "https://b.com", CreatedAt: now, LastActive: now},
	}

	sessions := sm.List()
	if len(sessions) != 2 {
		t.Fatalf("Expected 2 sessions, got %d", len(sessions))
	}

	ids := map[string]bool{}
	for _, s := range sessions {
		ids[s.ID] = true
	}
	if !ids["id-1"] {
		t.Error("Expected session id-1 in list")
	}
	if !ids["id-2"] {
		t.Error("Expected session id-2 in list")
	}
}

func TestSessionManager_UpdateMetadata_WhenSessionExists_ShouldUpdateMeta(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManagerWithSink(cfg, nil)

	sm.sessions["test-id"] = &sessionRecord{
		meta: Session{
			ID:     "test-id",
			Status: "active",
			URL:    "https://original.com",
		},
	}

	sm.UpdateMetadata("test-id", func(s Session) Session {
		s.Status = "navigated"
		s.URL = "https://updated.com"
		return s
	})

	session, ok := sm.GetSession("test-id")
	if !ok {
		t.Fatal("Session should still exist after update")
	}
	if session.Status != "navigated" {
		t.Errorf("Expected Status='navigated', got %q", session.Status)
	}
	if session.URL != "https://updated.com" {
		t.Errorf("Expected URL='https://updated.com', got %q", session.URL)
	}
}

func TestSessionManager_UpdateMetadata_WhenUnknownSession_ShouldNoOp(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManagerWithSink(cfg, nil)

	// Should not panic
	sm.UpdateMetadata("nonexistent", func(s Session) Session {
		s.Status = "should-not-happen"
		return s
	})

	_, ok := sm.GetSession("nonexistent")
	if ok {
		t.Error("Should not create session for unknown ID")
	}
}

// --- stringifyConsoleArgs Tests ---

func TestStringifyConsoleArgs_WhenNilSlice_ShouldReturnEmpty(t *testing.T) {
	result := stringifyConsoleArgs(nil)
	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

func TestStringifyConsoleArgs_WhenEmptySlice_ShouldReturnEmpty(t *testing.T) {
	result := stringifyConsoleArgs([]*proto.RuntimeRemoteObject{})
	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

func TestStringifyConsoleArgs_WhenNilEntries_ShouldSkipThem(t *testing.T) {
	result := stringifyConsoleArgs([]*proto.RuntimeRemoteObject{nil, nil})
	if result != "" {
		t.Errorf("Expected empty string for all-nil entries, got %q", result)
	}
}

func TestStringifyConsoleArgs_WhenValuesPresent_ShouldJoinWithSpace(t *testing.T) {
	args := []*proto.RuntimeRemoteObject{
		{Value: gson.New("hello")},
		{Value: gson.New("world")},
	}
	result := stringifyConsoleArgs(args)
	if result != "hello world" {
		t.Errorf("Expected 'hello world', got %q", result)
	}
}

func TestStringifyConsoleArgs_WhenDescriptionFallback_ShouldUseDescription(t *testing.T) {
	args := []*proto.RuntimeRemoteObject{
		{Value: gson.New(nil), Description: "Error: something failed"},
	}
	result := stringifyConsoleArgs(args)
	if result != "Error: something failed" {
		t.Errorf("Expected 'Error: something failed', got %q", result)
	}
}

func TestStringifyConsoleArgs_WhenMixed_ShouldConcatenate(t *testing.T) {
	args := []*proto.RuntimeRemoteObject{
		{Value: gson.New("log:")},
		nil,
		{Value: gson.New(nil), Description: "Object"},
		{Value: gson.New(42)},
	}
	result := stringifyConsoleArgs(args)
	if result != "log: Object 42" {
		t.Errorf("Expected 'log: Object 42', got %q", result)
	}
}

// --- coalesceNonEmpty Tests ---

func TestCoalesceNonEmpty_WhenNoArgs_ShouldReturnEmpty(t *testing.T) {
	result := coalesceNonEmpty()
	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

func TestCoalesceNonEmpty_WhenFirstNonEmpty_ShouldReturnFirst(t *testing.T) {
	result := coalesceNonEmpty("hello", "world")
	if result != "hello" {
		t.Errorf("Expected 'hello', got %q", result)
	}
}

func TestCoalesceNonEmpty_WhenFirstEmpty_ShouldReturnSecond(t *testing.T) {
	result := coalesceNonEmpty("", "world")
	if result != "world" {
		t.Errorf("Expected 'world', got %q", result)
	}
}

func TestCoalesceNonEmpty_WhenAllEmpty_ShouldReturnEmpty(t *testing.T) {
	result := coalesceNonEmpty("", "", "   ", "\t")
	if result != "" {
		t.Errorf("Expected empty string for all-whitespace args, got %q", result)
	}
}

func TestCoalesceNonEmpty_WhenWhitespaceOnly_ShouldSkip(t *testing.T) {
	result := coalesceNonEmpty("   ", "\t", "valid")
	if result != "valid" {
		t.Errorf("Expected 'valid', got %q", result)
	}
}

func TestCoalesceNonEmpty_WhenSingleValue_ShouldReturnIt(t *testing.T) {
	result := coalesceNonEmpty("only")
	if result != "only" {
		t.Errorf("Expected 'only', got %q", result)
	}
}

// --- isInternalScript Tests ---

func TestIsInternalScript_WhenInternalPrefixes_ShouldReturnTrue(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"Chrome protocol", "chrome://extensions/"},
		{"Chrome extension", "chrome-extension://abc123/popup.html"},
		{"Devtools", "devtools://devtools/bundled/inspector.js"},
		{"About page", "about:blank"},
		{"Data URI", "data:text/html,<h1>Hello</h1>"},
		{"Blob URL", "blob:https://example.com/123-456"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !isInternalScript(tc.url) {
				t.Errorf("Expected isInternalScript(%q)=true", tc.url)
			}
		})
	}
}

func TestIsInternalScript_WhenExternalURLs_ShouldReturnFalse(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"HTTPS URL", "https://example.com/app.js"},
		{"HTTP URL", "http://localhost:3000/main.js"},
		{"File protocol", "file:///C:/code/script.js"},
		{"Relative path", "/scripts/main.js"},
		{"Empty string", ""},
		{"Just a name", "script.js"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if isInternalScript(tc.url) {
				t.Errorf("Expected isInternalScript(%q)=false", tc.url)
			}
		})
	}
}

// --- persistSessions / loadSessionsLocked Tests ---

func TestPersistSessions_WhenNoSessionStore_ShouldReturnNil(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SessionStore = ""
	sm := NewSessionManagerWithSink(cfg, nil)
	err := sm.persistSessions()
	if err != nil {
		t.Errorf("Expected nil error when no session store, got %v", err)
	}
}

func TestPersistAndLoadSessions_WhenValidPath_ShouldRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "sessions.json")

	cfg := DefaultConfig()
	cfg.SessionStore = storePath
	sm := NewSessionManagerWithSink(cfg, nil)

	now := time.Now().Truncate(time.Millisecond)
	sm.sessions["ses-1"] = &sessionRecord{
		meta: Session{
			ID:         "ses-1",
			TargetID:   "target-1",
			URL:        "https://example.com",
			Title:      "Example",
			Status:     "active",
			CreatedAt:  now,
			LastActive: now,
		},
	}
	sm.sessions["ses-2"] = &sessionRecord{
		meta: Session{
			ID:         "ses-2",
			URL:        "https://other.com",
			Status:     "attached",
			CreatedAt:  now,
			LastActive: now,
		},
	}

	// Persist
	err := sm.persistSessions()
	if err != nil {
		t.Fatalf("persistSessions failed: %v", err)
	}

	// Verify file exists
	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("Failed to read persisted file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Persisted file is empty")
	}

	// Verify JSON is valid
	var sessions []Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		t.Fatalf("Persisted JSON is invalid: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("Expected 2 persisted sessions, got %d", len(sessions))
	}

	// Load into new manager
	sm2 := NewSessionManagerWithSink(cfg, nil)
	err = sm2.loadSessionsLocked()
	if err != nil {
		t.Fatalf("loadSessionsLocked failed: %v", err)
	}

	if len(sm2.sessions) != 2 {
		t.Fatalf("Expected 2 loaded sessions, got %d", len(sm2.sessions))
	}

	// Loaded sessions should have status "detached"
	for id, rec := range sm2.sessions {
		if rec.meta.Status != "detached" {
			t.Errorf("Session %s expected status='detached', got %q", id, rec.meta.Status)
		}
		if rec.page != nil {
			t.Errorf("Session %s expected nil page after load", id)
		}
	}
}

func TestLoadSessions_WhenFileNotExist_ShouldReturnNil(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SessionStore = filepath.Join(t.TempDir(), "nonexistent.json")
	sm := NewSessionManagerWithSink(cfg, nil)

	err := sm.loadSessionsLocked()
	if err != nil {
		t.Errorf("Expected nil error for missing file, got %v", err)
	}
	if len(sm.sessions) != 0 {
		t.Errorf("Expected 0 sessions, got %d", len(sm.sessions))
	}
}

func TestLoadSessions_WhenInvalidJSON_ShouldReturnError(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(storePath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("Failed to write bad JSON: %v", err)
	}

	cfg := DefaultConfig()
	cfg.SessionStore = storePath
	sm := NewSessionManagerWithSink(cfg, nil)

	err := sm.loadSessionsLocked()
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestLoadSessions_WhenNoSessionStore_ShouldReturnNil(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SessionStore = ""
	sm := NewSessionManagerWithSink(cfg, nil)

	err := sm.loadSessionsLocked()
	if err != nil {
		t.Errorf("Expected nil error when no session store, got %v", err)
	}
}

func TestPersistSessions_WhenNestedDir_ShouldCreateParentDirs(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "deep", "nested", "sessions.json")

	cfg := DefaultConfig()
	cfg.SessionStore = storePath
	sm := NewSessionManagerWithSink(cfg, nil)

	err := sm.persistSessions()
	if err != nil {
		t.Fatalf("persistSessions failed for nested dir: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(storePath); os.IsNotExist(err) {
		t.Error("Expected session file to be created in nested dir")
	}
}

// --- Session struct JSON Tests ---

func TestSession_JSONMarshal_ShouldIncludeAllFields(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	s := Session{
		ID:         "session-123",
		TargetID:   "target-456",
		URL:        "https://example.com",
		Title:      "Test Page",
		Status:     "active",
		CreatedAt:  now,
		LastActive: now,
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Failed to marshal session: %v", err)
	}

	var decoded Session
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal session: %v", err)
	}

	if decoded.ID != s.ID {
		t.Errorf("ID mismatch: got %q, want %q", decoded.ID, s.ID)
	}
	if decoded.TargetID != s.TargetID {
		t.Errorf("TargetID mismatch: got %q, want %q", decoded.TargetID, s.TargetID)
	}
	if decoded.URL != s.URL {
		t.Errorf("URL mismatch: got %q, want %q", decoded.URL, s.URL)
	}
	if decoded.Title != s.Title {
		t.Errorf("Title mismatch: got %q, want %q", decoded.Title, s.Title)
	}
	if decoded.Status != s.Status {
		t.Errorf("Status mismatch: got %q, want %q", decoded.Status, s.Status)
	}
}

// --- DetectionResult struct Tests ---

func TestDetectionResult_JSONMarshal_ShouldRoundTrip(t *testing.T) {
	dr := DetectionResult{
		ElementID:  "elem_0",
		Selector:   "#hidden-link",
		Reasons:    []string{"Hidden via display:none", "Zero size"},
		Confidence: 0.8,
		TagName:    "a",
		Href:       "https://honeypot.example.com",
	}

	data, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("Failed to marshal DetectionResult: %v", err)
	}

	var decoded DetectionResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal DetectionResult: %v", err)
	}

	if decoded.ElementID != dr.ElementID {
		t.Errorf("ElementID mismatch: got %q, want %q", decoded.ElementID, dr.ElementID)
	}
	if decoded.Confidence != dr.Confidence {
		t.Errorf("Confidence mismatch: got %f, want %f", decoded.Confidence, dr.Confidence)
	}
	if len(decoded.Reasons) != 2 {
		t.Errorf("Expected 2 reasons, got %d", len(decoded.Reasons))
	}
}

func TestDetectionResult_WhenEmptyHref_ShouldOmitInJSON(t *testing.T) {
	dr := DetectionResult{
		ElementID: "elem_0",
		Href:      "",
	}
	data, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	// href has `omitempty` tag
	jsonStr := string(data)
	if json.Valid([]byte(jsonStr)) {
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err == nil {
			if _, exists := raw["href"]; exists {
				t.Error("Expected href to be omitted when empty")
			}
		}
	}
}

// --- Link struct Tests ---

func TestLink_JSONMarshal_ShouldRoundTrip(t *testing.T) {
	link := Link{
		Selector:        "a[href='/about']",
		Href:            "/about",
		Text:            "About Us",
		IsHoneypot:      true,
		HoneypotReasons: []string{"Hidden via visibility:hidden"},
	}

	data, err := json.Marshal(link)
	if err != nil {
		t.Fatalf("Failed to marshal Link: %v", err)
	}

	var decoded Link
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal Link: %v", err)
	}

	if decoded.Href != link.Href {
		t.Errorf("Href mismatch: got %q, want %q", decoded.Href, link.Href)
	}
	if decoded.IsHoneypot != link.IsHoneypot {
		t.Errorf("IsHoneypot mismatch: got %v, want %v", decoded.IsHoneypot, link.IsHoneypot)
	}
	if len(decoded.HoneypotReasons) != 1 {
		t.Errorf("Expected 1 honeypot reason, got %d", len(decoded.HoneypotReasons))
	}
}

func TestLink_WhenNotHoneypot_ShouldOmitReasons(t *testing.T) {
	link := Link{
		Selector:   "a[href='/']",
		Href:       "/",
		Text:       "Home",
		IsHoneypot: false,
	}

	data, err := json.Marshal(link)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to unmarshal to map: %v", err)
	}
	if _, exists := raw["honeypot_reasons"]; exists {
		t.Error("Expected honeypot_reasons to be omitted when empty")
	}
}

// --- NewHoneypotDetector Tests ---

func TestNewHoneypotDetector_WhenEngineProvided_ShouldSetEngine(t *testing.T) {
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	detector := NewHoneypotDetector(engine)
	if detector == nil {
		t.Fatal("Expected non-nil detector")
	}
	if detector.engine != engine {
		t.Error("Expected detector engine to match provided engine")
	}
}

func TestNewHoneypotDetector_WhenNilEngine_ShouldStillCreate(t *testing.T) {
	detector := NewHoneypotDetector(nil)
	if detector == nil {
		t.Fatal("Expected non-nil detector even with nil engine")
	}
	if detector.engine != nil {
		t.Error("Expected nil engine in detector")
	}
}

// --- Shutdown Tests ---

func TestSessionManager_Shutdown_WhenNotStarted_ShouldNotPanic(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManagerWithSink(cfg, nil)

	// Should not panic, browser is nil
	err := sm.Shutdown(nil)
	if err != nil {
		t.Errorf("Expected nil error shutting down unstarted manager, got %v", err)
	}
}

func TestSessionManager_Shutdown_WhenSessionsExist_ShouldClearSessions(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManagerWithSink(cfg, nil)

	// Inject mock sessions (without real pages)
	sm.sessions["id-1"] = &sessionRecord{
		meta: Session{ID: "id-1"},
		page: nil, // no real page
	}
	sm.sessions["id-2"] = &sessionRecord{
		meta: Session{ID: "id-2"},
		page: nil,
	}

	err := sm.Shutdown(nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(sm.sessions) != 0 {
		t.Errorf("Expected 0 sessions after shutdown, got %d", len(sm.sessions))
	}
	if sm.controlURL != "" {
		t.Errorf("Expected empty controlURL after shutdown, got %q", sm.controlURL)
	}
}

// --- engineAdapter Tests ---

func TestEngineAdapter_AddFacts_WhenEngineNotNil_ShouldDelegate(t *testing.T) {
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	adapter := &engineAdapter{engine: engine}

	// AddFacts should delegate to engine.AddFacts
	// Since no schemas are loaded, it should return an error about no schemas
	facts := []mangle.Fact{{Predicate: "test", Args: []interface{}{"a"}}}
	addErr := adapter.AddFacts(facts)
	if addErr == nil {
		t.Error("Expected error when no schemas loaded, got nil")
	}
}

// --- Concurrent access tests ---

func TestSessionManager_ConcurrentAccess_ShouldBeSafe(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManagerWithSink(cfg, nil)

	now := time.Now()
	sm.sessions["test-id"] = &sessionRecord{
		meta: Session{
			ID:         "test-id",
			URL:        "https://example.com",
			Status:     "active",
			CreatedAt:  now,
			LastActive: now,
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(4)
		go func() {
			defer wg.Done()
			sm.List()
		}()
		go func() {
			defer wg.Done()
			sm.IsConnected()
		}()
		go func() {
			defer wg.Done()
			sm.GetSession("test-id")
		}()
		go func() {
			defer wg.Done()
			sm.Page("test-id")
		}()
	}
	wg.Wait()
	// No data race = pass
}

// --- Edge case: Config with all zero values ---

func TestConfig_AllZeroValues_ShouldReturnDefaults(t *testing.T) {
	cfg := Config{} // all zero values

	if cfg.IsHeadless() {
		t.Error("Zero Config.Headless should be false")
	}
	if cfg.GetViewportWidth() != 1920 {
		t.Errorf("Zero ViewportWidth should default to 1920, got %d", cfg.GetViewportWidth())
	}
	if cfg.GetViewportHeight() != 1080 {
		t.Errorf("Zero ViewportHeight should default to 1080, got %d", cfg.GetViewportHeight())
	}
	if cfg.NavigationTimeout() != 30*time.Second {
		t.Errorf("Zero NavigationTimeoutMs should default to 30s, got %v", cfg.NavigationTimeout())
	}
}

// --- Config JSON serialization ---

func TestConfig_JSONRoundTrip_ShouldPreserveAllFields(t *testing.T) {
	cfg := Config{
		DebuggerURL:           "ws://127.0.0.1:9222",
		Launch:                []string{"chrome", "--no-sandbox"},
		Headless:              true,
		ViewportWidth:         1280,
		ViewportHeight:        720,
		NavigationTimeoutMs:   15000,
		SessionStore:          "/tmp/sessions.json",
		EventLoggingLevel:     "verbose",
		EnableDOMIngestion:    true,
		EnableHeaderIngestion: true,
		EventThrottleMs:       50,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	if decoded.DebuggerURL != cfg.DebuggerURL {
		t.Errorf("DebuggerURL mismatch: got %q, want %q", decoded.DebuggerURL, cfg.DebuggerURL)
	}
	if len(decoded.Launch) != 2 {
		t.Errorf("Launch length mismatch: got %d, want 2", len(decoded.Launch))
	}
	if decoded.Headless != cfg.Headless {
		t.Errorf("Headless mismatch: got %v, want %v", decoded.Headless, cfg.Headless)
	}
	if decoded.ViewportWidth != cfg.ViewportWidth {
		t.Errorf("ViewportWidth mismatch: got %d, want %d", decoded.ViewportWidth, cfg.ViewportWidth)
	}
	if decoded.EventThrottleMs != cfg.EventThrottleMs {
		t.Errorf("EventThrottleMs mismatch: got %d, want %d", decoded.EventThrottleMs, cfg.EventThrottleMs)
	}
	if decoded.EnableHeaderIngestion != cfg.EnableHeaderIngestion {
		t.Errorf("EnableHeaderIngestion mismatch: got %v, want %v", decoded.EnableHeaderIngestion, cfg.EnableHeaderIngestion)
	}
}

// --- startEventStream nil engine guard ---

func TestSessionManager_StartEventStream_WhenNilEngine_ShouldNotPanic(t *testing.T) {
	cfg := DefaultConfig()
	sm := NewSessionManagerWithSink(cfg, nil)

	// startEventStream should return early without panic when engine is nil
	// We can't directly call it with a nil page, but the nil engine guard is the first check
	sm.startEventStream(nil, "test-session", nil)
	// If we reach here without panic, the nil guard works
}
