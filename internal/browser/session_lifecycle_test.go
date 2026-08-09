package browser

import (
	"context"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

func TestBrowserLifecycleConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.IsMultiTabDefault() {
		t.Fatal("ordinary tabs should share the default browser profile")
	}
	if got := cfg.GetMaxTabs(); got != 32 {
		t.Fatalf("GetMaxTabs() = %d, want 32", got)
	}
	if got := cfg.GetMaxBrowsers(); got != 4 {
		t.Fatalf("GetMaxBrowsers() = %d, want 4", got)
	}
	if got := cfg.GetIdleTabTimeout(); got != 0 {
		t.Fatalf("GetIdleTabTimeout() = %v, want disabled", got)
	}

	isolated := false
	cfg.MultiTabDefault = &isolated
	if cfg.IsMultiTabDefault() {
		t.Fatal("explicit multi_tab_default=false should be preserved")
	}
}

func TestSessionManagerCloseSessionCancelsAndIsIdempotent(t *testing.T) {
	sm := NewSessionManagerWithSink(DefaultConfig(), nil)
	_, cancel := context.WithCancel(context.Background())
	cancelled := false
	sm.sessions["session-1"] = &sessionRecord{
		meta: Session{ID: "session-1", LastActive: time.Now()},
		streamCancel: func() {
			cancelled = true
			cancel()
		},
	}

	if err := sm.CloseSession(context.Background(), "session-1"); err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}
	if !cancelled {
		t.Fatal("CloseSession() did not cancel the event stream")
	}
	if _, ok := sm.GetSession("session-1"); ok {
		t.Fatal("closed session remains registered")
	}
	if err := sm.CloseSession(context.Background(), "session-1"); err != nil {
		t.Fatalf("repeated CloseSession() error = %v", err)
	}
}

func TestSessionManagerBrowserInventoryAndPromotion(t *testing.T) {
	sm := NewSessionManagerWithSink(DefaultConfig(), nil)
	now := time.Now()
	sm.defaultID = "browser-1"
	sm.browsers["browser-1"] = &browserRecord{meta: BrowserInstance{ID: "browser-1", Default: true, CreatedAt: now}}
	sm.browsers["browser-2"] = &browserRecord{meta: BrowserInstance{ID: "browser-2", CreatedAt: now.Add(time.Second)}}
	sm.sessions["tab-1"] = &sessionRecord{meta: Session{ID: "tab-1", BrowserID: "browser-1"}, page: &rod.Page{}}
	sm.sessions["tab-2"] = &sessionRecord{meta: Session{ID: "tab-2", BrowserID: "browser-2"}, page: &rod.Page{}}

	browsers := sm.ListBrowsers()
	if len(browsers) != 2 || browsers[0].ID != "browser-1" || browsers[0].TabCount != 1 {
		t.Fatalf("unexpected browser inventory: %+v", browsers)
	}
	sm.sessions["tab-1"].page = nil
	sm.sessions["tab-2"].page = nil
	if err := sm.CloseBrowser(context.Background(), "browser-1"); err != nil {
		t.Fatalf("CloseBrowser() error = %v", err)
	}
	if sm.defaultID != "browser-2" {
		t.Fatalf("defaultID = %q, want promoted browser-2", sm.defaultID)
	}
	if _, ok := sm.GetSession("tab-1"); ok {
		t.Fatal("closing a browser did not remove its tabs")
	}
	if _, ok := sm.GetSession("tab-2"); !ok {
		t.Fatal("closing a browser removed another browser's tab")
	}
}

func TestSessionManagerReapsOnlyIdleTabs(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IdleTabTimeoutMs = 1000
	sm := NewSessionManagerWithSink(cfg, nil)
	now := time.Now()
	sm.sessions["idle"] = &sessionRecord{meta: Session{ID: "idle", LastActive: now.Add(-2 * time.Second)}}
	sm.sessions["active"] = &sessionRecord{meta: Session{ID: "active", LastActive: now}}

	sm.reapIdleTabs(now)
	if _, ok := sm.GetSession("idle"); ok {
		t.Fatal("idle tab was not reaped")
	}
	if _, ok := sm.GetSession("active"); !ok {
		t.Fatal("active tab was reaped")
	}
}

func TestSessionManagerTabLimitIgnoresDetachedMetadataAndReservesAtomically(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxTabs = 1
	sm := NewSessionManagerWithSink(cfg, nil)
	sm.sessions["detached"] = &sessionRecord{meta: Session{ID: "detached", Status: "detached"}}
	if err := sm.reserveTab(); err != nil {
		t.Fatalf("detached metadata consumed live tab capacity: %v", err)
	}
	if err := sm.reserveTab(); err == nil {
		t.Fatal("concurrent tab reservation exceeded configured capacity")
	}
	sm.releaseTabReservation()
	sm.sessions["live"] = &sessionRecord{meta: Session{ID: "live"}, page: &rod.Page{}}
	if err := sm.reserveTab(); err == nil {
		t.Fatal("live tab did not consume configured capacity")
	}
}
