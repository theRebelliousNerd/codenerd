package browser

import (
	"context"
	"errors"
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
				<input id="password1" name="password" type="password" />
				<a href="/page2">Link to Page 2</a>
				<a href="/hidden" style="display:none">Hidden Link</a>
				<details><summary id="more">More</summary><p>Hidden details</p></details>
				<table aria-label="Rows"><tr><th>Name</th></tr><tr data-row-id="one"><td>One</td></tr></table>
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
		startRequestCtx, cancelStartRequest := context.WithCancel(ctx)
		err := sm.Start(startRequestCtx)
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		cancelStartRequest()
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

		// Shared tabs preserve browser-profile state and survive the request
		// context that created them. Explicitly isolated tabs do neither.
		if _, err := page.Context(ctx).Eval(`() => { localStorage.setItem('codenerd-shared', 'yes'); return true }`); err != nil {
			t.Fatalf("seed shared storage: %v", err)
		}
		requestCtx, requestCancel := context.WithCancel(ctx)
		shared, err := sm.CreateTab(requestCtx, session.BrowserID, ts.URL, false)
		if err != nil {
			t.Fatalf("CreateTab(shared) failed: %v", err)
		}
		requestCancel()
		sharedPage, ok := sm.Page(shared.ID)
		if !ok {
			t.Fatal("shared tab was not tracked")
		}
		sharedValue, err := sharedPage.Context(ctx).Eval(`() => localStorage.getItem('codenerd-shared') === 'yes'`)
		if err != nil || !sharedValue.Value.Bool() {
			t.Fatalf("shared tab lost profile storage: value=%v err=%v", sharedValue, err)
		}
		if err := sm.FocusSession(ctx, shared.ID); err != nil {
			t.Fatalf("FocusSession(shared) failed: %v", err)
		}

		isolated, err := sm.CreateTab(ctx, session.BrowserID, ts.URL, true)
		if err != nil {
			t.Fatalf("CreateTab(isolated) failed: %v", err)
		}
		isolatedPage, ok := sm.Page(isolated.ID)
		if !ok {
			t.Fatal("isolated tab was not tracked")
		}
		isolatedValue, err := isolatedPage.Context(ctx).Eval(`() => localStorage.getItem('codenerd-shared') === null`)
		if err != nil || !isolatedValue.Value.Bool() {
			t.Fatalf("isolated tab inherited shared storage: value=%v err=%v", isolatedValue, err)
		}
		if err := sm.CloseSession(ctx, shared.ID); err != nil {
			t.Fatalf("CloseSession(shared) failed: %v", err)
		}
		if err := sm.CloseSession(ctx, isolated.ID); err != nil {
			t.Fatalf("CloseSession(isolated) failed: %v", err)
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

		// --- Progressive observe/act parity route ---
		observation, err := sm.Observe(ctx, session.ID, ObserveOptions{
			Mode: "composite", View: "full", MaxItems: 20, VisibleOnly: true,
		})
		if err != nil {
			t.Fatalf("Observe(composite) failed: %v", err)
		}
		interactive, ok := observation.Data["interactive"].([]InteractiveElement)
		if !ok || len(interactive) == 0 {
			t.Fatalf("Observe returned no interactive elements: %#v", observation.Data["interactive"])
		}
		var buttonRef, inputRef string
		for _, element := range interactive {
			if element.Fingerprint == nil {
				continue
			}
			switch element.Fingerprint.ID {
			case "btn1":
				buttonRef = element.Ref
			case "input1":
				inputRef = element.Ref
			}
		}
		if buttonRef == "" || inputRef == "" {
			t.Fatalf("Observe did not issue refs for fixture controls: button=%q input=%q", buttonRef, inputRef)
		}
		if counts, ok := observation.Data["counts"].(map[string]int); !ok || counts["grids"] == 0 || counts["hidden"] == 0 {
			t.Fatalf("Observe did not discover grid/hidden surfaces: %#v", observation.Data["counts"])
		}
		repeated, err := sm.Observe(ctx, session.ID, ObserveOptions{Mode: "interactive", View: "full", MaxItems: 20, VisibleOnly: true})
		if err != nil {
			t.Fatalf("Observe(interactive) failed: %v", err)
		}
		stable := false
		for _, element := range repeated.Data["interactive"].([]InteractiveElement) {
			if element.Fingerprint != nil && element.Fingerprint.ID == "btn1" && element.Ref == buttonRef {
				stable = true
			}
		}
		if !stable || repeated.Generation != observation.Generation {
			t.Fatalf("ref was not stable within generation: first=%d second=%d", observation.Generation, repeated.Generation)
		}
		execution, err := sm.ExecuteActions(ctx, session.ID, []ActionOperation{
			{Type: "interact", Ref: inputRef, Action: "type", Value: "progressive text"},
			{Type: "key", Key: "End"},
			{Type: "interact", Ref: buttonRef, Action: "click"},
		}, true)
		if err != nil || !execution.Success || execution.Counts["succeeded"] != 3 {
			t.Fatalf("ExecuteActions failed: execution=%+v err=%v", execution, err)
		}
		oldGeneration := observation.Generation
		if err := sm.Navigate(ctx, session.ID, ts.URL+"/page2"); err != nil {
			t.Fatalf("Navigate for stale-ref proof failed: %v", err)
		}
		if _, staleErr := sm.InteractRef(ctx, session.ID, buttonRef, "click", "", false); staleErr == nil {
			t.Fatal("expected pre-navigation ref to fail closed")
		}
		if registry := sm.Registry(session.ID); registry.Generation() <= oldGeneration {
			t.Fatalf("navigation did not advance ref generation: before=%d after=%d", oldGeneration, registry.Generation())
		}
		if err := sm.Navigate(ctx, session.ID, ts.URL); err != nil {
			t.Fatalf("Navigate after stale-ref proof failed: %v", err)
		}
		time.Sleep(200 * time.Millisecond)

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
		const browserSecretFixture = "live-browser-secret-7419"
		if err := sm.Type(ctx, session.ID, "#password1", browserSecretFixture); err != nil {
			t.Logf("Password type warning (timing): %v", err)
		}
		time.Sleep(600 * time.Millisecond)
		for _, fact := range sink.getFacts() {
			for _, arg := range fact.Args {
				if value, ok := arg.(string); ok && value == browserSecretFixture {
					t.Fatalf("live browser secret reached fact sink in %s", fact.Predicate)
				}
			}
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

		clickCtx, cancelClick := context.WithTimeout(ctx, time.Second)
		err = sm.Click(clickCtx, session.ID, "#nonexistent-button-xyz")
		cancelClick()
		if err == nil {
			t.Error("Expected error for non-existent selector in Click")
		}

		// --- Type invalid selector ---
		typeCtx, cancelType := context.WithTimeout(ctx, time.Second)
		err = sm.Type(typeCtx, session.ID, "#nonexistent-input-xyz", "test")
		cancelType()
		if err == nil {
			t.Error("Expected error for non-existent selector in Type")
		}

		// --- ReifyReact on non-React page ---
		reactCtx, cancelReact := context.WithTimeout(ctx, 3*time.Second)
		reactFacts, err := sm.ReifyReact(reactCtx, session.ID)
		cancelReact()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("ReifyReact stalled on a non-React page: %v", err)
			}
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
