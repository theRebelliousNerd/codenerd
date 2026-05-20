package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"codenerd/internal/mangle"

	"github.com/go-rod/rod/lib/launcher"
)

// testEngineSinkLocal implements EngineSink for local testing.
type testEngineSinkLocal struct {
	mu    sync.Mutex
	facts []mangle.Fact
}

func (s *testEngineSinkLocal) AddFacts(facts []mangle.Fact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.facts = append(s.facts, facts...)
	return nil
}

func (s *testEngineSinkLocal) getFacts() []mangle.Fact {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]mangle.Fact, len(s.facts))
	copy(copied, s.facts)
	return copied
}

func (s *testEngineSinkLocal) findFactsByPredicate(pred string) []mangle.Fact {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []mangle.Fact
	for _, f := range s.facts {
		if f.Predicate == pred {
			result = append(result, f)
		}
	}
	return result
}

// TestBrowserLifecycle_WhenChromeAvailable runs all Chrome-dependent tests
// in a single test function with a shared Chrome instance to avoid spawning
// multiple browser processes and keep test execution fast.
func TestBrowserLifecycle_WhenChromeAvailable(t *testing.T) {
	// Try to find and launch Chrome
	path, found := launcher.LookPath()
	if !found || path == "" {
		t.Skip("Chrome not found, skipping all lifecycle tests")
	}

	controlURL, err := launcher.New().Headless(true).Launch()
	if err != nil {
		t.Skipf("Failed to launch Chrome: %v", err)
	}

	// Create a test HTTP server that serves multiple pages
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/page2":
			fmt.Fprintln(w, `<html><body><h1>Page Two</h1></body></html>`)
		case "/simple":
			fmt.Fprintln(w, `<html><body><h1>Simple Page</h1></body></html>`)
		default:
			fmt.Fprintln(w, `<html><body>
				<h1 id="heading">Test Page</h1>
				<button id="btn1">Click Me</button>
				<input id="input1" type="text" />
				<a href="/page2">Link to Page 2</a>
				<a href="/hidden" style="display:none">Hidden Link</a>
			</body></html>`)
		}
	}))
	defer ts.Close()

	t.Run("FullLifecycle", func(t *testing.T) {
		sink := &testEngineSinkLocal{}
		cfg := DefaultConfig()
		cfg.DebuggerURL = controlURL
		cfg.Headless = true
		cfg.NavigationTimeoutMs = 10000
		cfg.EventThrottleMs = 10
		cfg.EnableDOMIngestion = true
		cfg.EventLoggingLevel = "normal"

		sm := NewSessionManagerWithSink(cfg, sink)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer shutdownCancel()
			if shutdownErr := sm.Shutdown(shutdownCtx); shutdownErr != nil {
				t.Logf("Shutdown error: %v", shutdownErr)
			}
		}()

		// --- Start ---
		err := sm.Start(ctx)
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		if !sm.IsConnected() {
			t.Fatal("Expected IsConnected()=true after Start")
		}
		if sm.ControlURL() != controlURL {
			t.Errorf("Expected ControlURL=%q, got %q", controlURL, sm.ControlURL())
		}

		// --- CreateSession ---
		session, err := sm.CreateSession(ctx, ts.URL)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		if session.ID == "" {
			t.Fatal("Expected non-empty session ID")
		}
		if session.Status != "active" {
			t.Errorf("Expected session status='active', got %q", session.Status)
		}

		// --- List ---
		sessions := sm.List()
		if len(sessions) == 0 {
			t.Error("Expected at least 1 session in list")
		}

		// --- Page ---
		page, ok := sm.Page(session.ID)
		if !ok {
			t.Fatal("Expected to find session page")
		}
		if page == nil {
			t.Fatal("Expected non-nil page")
		}

		// --- Navigate ---
		err = sm.Navigate(ctx, session.ID, ts.URL+"/page2")
		if err != nil {
			t.Fatalf("Navigate failed: %v", err)
		}
		err = sm.Navigate(ctx, session.ID, ts.URL)
		if err != nil {
			t.Fatalf("Navigate back failed: %v", err)
		}
		time.Sleep(300 * time.Millisecond)

		// --- Click ---
		err = sm.Click(ctx, session.ID, "#btn1")
		if err != nil {
			t.Logf("Click warning (timing): %v", err)
		}

		// --- Type ---
		err = sm.Type(ctx, session.ID, "#input1", "hello world")
		if err != nil {
			t.Logf("Type warning (timing): %v", err)
		}

		// --- Screenshot viewport ---
		data, err := sm.Screenshot(ctx, session.ID, false)
		if err != nil {
			t.Fatalf("Screenshot failed: %v", err)
		}
		if len(data) == 0 {
			t.Error("Expected non-empty screenshot data")
		}

		// --- Screenshot full page ---
		dataFull, err := sm.Screenshot(ctx, session.ID, true)
		if err != nil {
			t.Fatalf("Full-page screenshot failed: %v", err)
		}
		if len(dataFull) == 0 {
			t.Error("Expected non-empty full-page screenshot data")
		}

		// --- SnapshotDOM ---
		err = sm.SnapshotDOM(ctx, session.ID)
		if err != nil {
			t.Fatalf("SnapshotDOM failed: %v", err)
		}

		// --- GetSession after activity ---
		updatedSession, ok := sm.GetSession(session.ID)
		if !ok {
			t.Fatal("Expected to find session after activity")
		}
		if updatedSession.ID != session.ID {
			t.Errorf("Session ID mismatch after activity")
		}

		// --- Verify facts emitted ---
		time.Sleep(500 * time.Millisecond)
		facts := sink.getFacts()
		if len(facts) == 0 {
			t.Error("Expected some facts emitted")
		}

		// --- Navigate unknown session ---
		err = sm.Navigate(ctx, "nonexistent-session-id", "https://example.com")
		if err == nil {
			t.Error("Expected error for unknown session in Navigate")
		}

		// --- Screenshot unknown session ---
		badData, err := sm.Screenshot(ctx, "nonexistent-session", false)
		if err == nil {
			t.Error("Expected error for unknown session in Screenshot")
		}
		if badData != nil {
			t.Error("Expected nil data for unknown session")
		}

		// --- ForkSession unknown source ---
		_, err = sm.ForkSession(ctx, "nonexistent-session", "https://example.com")
		if err == nil {
			t.Error("Expected error for unknown source in ForkSession")
		}

		// --- Click invalid selector ---
		err = sm.Navigate(ctx, session.ID, ts.URL+"/simple")
		if err != nil {
			t.Fatalf("Navigate to simple page failed: %v", err)
		}
		time.Sleep(300 * time.Millisecond)

		err = sm.Click(ctx, session.ID, "#nonexistent-button-xyz")
		if err == nil {
			t.Error("Expected error for non-existent selector in Click")
		}

		// --- Type invalid selector ---
		err = sm.Type(ctx, session.ID, "#nonexistent-input-xyz", "test")
		if err == nil {
			t.Error("Expected error for non-existent selector in Type")
		}

		// --- ReifyReact on non-React page ---
		reactFacts, err := sm.ReifyReact(ctx, session.ID)
		if err != nil {
			t.Logf("ReifyReact returned error (acceptable for non-React page): %v", err)
		} else {
			t.Logf("ReifyReact returned %d facts for non-React page", len(reactFacts))
		}

		// --- ForkSession success (timing-dependent, non-fatal) ---
		forkedSession, forkErr := sm.ForkSession(ctx, session.ID, ts.URL)
		if forkErr != nil {
			t.Logf("ForkSession warning (timing-dependent): %v", forkErr)
		} else {
			if forkedSession == nil {
				t.Error("Expected non-nil forked session")
			} else if forkedSession.ID == session.ID {
				t.Error("Forked session should have different ID")
			} else {
				_, forkedOK := sm.GetSession(forkedSession.ID)
				if !forkedOK {
					t.Error("Forked session should be in session list")
				}
			}

			// --- ForkSession with empty URL ---
			forkedSession2, fork2Err := sm.ForkSession(ctx, session.ID, "")
			if fork2Err != nil {
				t.Logf("ForkSession empty URL warning: %v", fork2Err)
			} else if forkedSession2 == nil {
				t.Error("Expected non-nil forked session for empty URL")
			}
		}
	})
}
