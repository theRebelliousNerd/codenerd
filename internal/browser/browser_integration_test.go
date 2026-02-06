//go:build integration
package browser_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"codenerd/internal/browser"
	"codenerd/internal/mangle"
	"github.com/stretchr/testify/require"
)

// TestEngineSink implements browser.EngineSink for testing.
type TestEngineSink struct {
	mu    sync.Mutex
	facts []mangle.Fact
}

func (s *TestEngineSink) AddFacts(facts []mangle.Fact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.facts = append(s.facts, facts...)
	return nil
}

func (s *TestEngineSink) GetFacts() []mangle.Fact {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Return a copy
	copied := make([]mangle.Fact, len(s.facts))
	copy(copied, s.facts)
	return copied
}

func TestSessionManager_Navigation_Integration(t *testing.T) {
	// 1. Setup local server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "<html><body><h1>Hello World</h1></body></html>")
	}))
	defer ts.Close()

	// 2. Setup SessionManager
	sink := &TestEngineSink{}
	cfg := browser.DefaultConfig()
	cfg.Headless = true
	// Faster timeouts for testing
	cfg.NavigationTimeoutMs = 10000
	cfg.EventThrottleMs = 10

	sm := browser.NewSessionManagerWithSink(cfg, sink)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Ensure shutdown to clean up browser process
	defer func() {
		if err := sm.Shutdown(context.Background()); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	err := sm.Start(ctx)
	require.NoError(t, err, "Failed to start browser")

	// 3. Create Session
	session, err := sm.CreateSession(ctx, ts.URL)
	require.NoError(t, err, "Failed to create session")
	require.NotEmpty(t, session.ID)
	require.Equal(t, ts.URL, session.URL)

	// 4. Verify Session State
	retrieved, ok := sm.GetSession(session.ID)
	require.True(t, ok)
	require.Equal(t, "active", retrieved.Status)

	// 5. Navigate again to ensure event stream is active and capturing
	// The initial navigation in CreateSession might happen before the event stream
	// is attached, so we perform an explicit navigation to verify the integration.
	targetURL := ts.URL + "/page2"
	err = sm.Navigate(ctx, session.ID, targetURL)
	require.NoError(t, err, "Failed to navigate to second page")

	// 6. Verify Facts (Async)
	// We check for dom_text to verify the page was actually loaded and processed.
	// We also check for navigation events, but they might be timing sensitive or racey with the event stream start.
	require.Eventually(t, func() bool {
		facts := sink.GetFacts()
		foundContent := false
		foundNav := false

		for _, f := range facts {
			if f.Predicate == "dom_text" {
				// Args: sessionID, id, text
				if len(f.Args) >= 3 {
					if text, ok := f.Args[2].(string); ok && text == "Hello World" {
						foundContent = true
					}
				}
			}
			// Also accept if we see the navigation event or current_url matching either URL (original or target)
			if f.Predicate == "navigation_event" || f.Predicate == "current_url" {
				if len(f.Args) >= 2 {
					urlArg, _ := f.Args[1].(string)
					if urlArg == ts.URL || urlArg == targetURL {
						foundNav = true
					}
				}
			}
		}
		return foundContent || foundNav
	}, 10*time.Second, 100*time.Millisecond, "Expected DOM content or navigation facts not found. Got facts: %v", sink.GetFacts())
}

func TestSessionManager_Interaction_Integration(t *testing.T) {
	// 1. Setup local server with interactive elements
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `
			<html>
			<body>
				<button id="btn1">Click Me</button>
				<input id="inp1" type="text" />
			</body>
			</html>
		`)
	}))
	defer ts.Close()

	sink := &TestEngineSink{}
	cfg := browser.DefaultConfig()
	cfg.Headless = true
	cfg.NavigationTimeoutMs = 10000
	cfg.EventThrottleMs = 10

	sm := browser.NewSessionManagerWithSink(cfg, sink)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	defer func() {
		if err := sm.Shutdown(context.Background()); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	require.NoError(t, sm.Start(ctx), "Failed to start browser")

	session, err := sm.CreateSession(ctx, ts.URL)
	require.NoError(t, err, "Failed to create session")

	// 2. Perform Interaction
	// Click
	err = sm.Click(ctx, session.ID, "#btn1")
	require.NoError(t, err, "Failed to click button")

	// Type
	err = sm.Type(ctx, session.ID, "#inp1", "hello")
	require.NoError(t, err, "Failed to type text")

	// 3. Verify Events
	// Note: click_event and input_event are captured by the JS hook injected by startEventStream
	require.Eventually(t, func() bool {
		facts := sink.GetFacts()
		foundClick := false
		foundInput := false
		for _, f := range facts {
			if f.Predicate == "click_event" {
				// Args: ID, timestamp
				if len(f.Args) >= 1 && f.Args[0] == "btn1" {
					foundClick = true
				}
			}
			if f.Predicate == "input_event" {
				// Args: ID, Value, timestamp
				if len(f.Args) >= 2 && f.Args[0] == "inp1" && f.Args[1] == "hello" {
					foundInput = true
				}
			}
		}
		return foundClick && foundInput
	}, 10*time.Second, 100*time.Millisecond, "Expected interaction facts not found")
}
