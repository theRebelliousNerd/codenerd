package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/mangle"

	"github.com/gorilla/websocket"
)

type telemetryLiveSink struct {
	mu        sync.Mutex
	facts     []mangle.Fact
	retracted []mangle.Fact
}

func (s *telemetryLiveSink) AddFacts(facts []mangle.Fact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, fact := range facts {
		fact.Args = append([]any(nil), fact.Args...)
		s.facts = append(s.facts, fact)
	}
	return nil
}

func (s *telemetryLiveSink) RetractFacts(facts []mangle.Fact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, fact := range facts {
		fact.Args = append([]any(nil), fact.Args...)
		s.retracted = append(s.retracted, fact)
	}
	return nil
}

func (s *telemetryLiveSink) snapshot() []mangle.Fact {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]mangle.Fact(nil), s.facts...)
}

func (s *telemetryLiveSink) retractedSnapshot() []mangle.Fact {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]mangle.Fact(nil), s.retracted...)
}

func TestBrowserEventTelemetry_LiveCDPProvenanceAndCleanup(t *testing.T) {
	cfg := liveBrowserConfig(t)
	cfg.EnableDOMIngestion = false
	cfg.EnableHeaderIngestion = false
	cfg.HeaderIngestionMode = HeaderIngestionOff
	cfg.EventThrottleMs = 0
	cfg.MaxEpochEventFacts = 1000
	evidenceEnabled := false
	cfg.EvidenceEnabled = &evidenceEnabled
	cfg.SessionStore = filepath.Join(t.TempDir(), "sessions.json")
	canceledStarted := make(chan struct{}, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!doctype html><title>telemetry</title><body>
<script>
console.error('access_token=console-secret-marker');
const aborter = new AbortController();
fetch('/cancel', {signal: aborter.signal}).catch(() => {});
setTimeout(() => aborter.abort(), 25);
fetch('/http-error').catch(() => {});
const denied = new WebSocket('ws://' + location.host + '/ws-denied');
denied.onerror = () => {};
const good = new WebSocket('ws://' + location.host + '/ws-ok');
good.onmessage = event => { window.__wsMessage = event.data; };
setTimeout(() => { window.__dialogResult = confirm('access_token=dialog-secret-marker'); }, 50);
</script>
<script src="/asset-error.js"></script>
</body>`)
		case "/http-error":
			http.Error(w, "api unavailable", http.StatusServiceUnavailable)
		case "/asset-error.js":
			http.Error(w, "asset unavailable", http.StatusInternalServerError)
		case "/cancel":
			select {
			case canceledStarted <- struct{}{}:
			default:
			}
			<-r.Context().Done()
		case "/ws-denied":
			http.Error(w, "websocket denied", http.StatusForbidden)
		case "/ws-ok":
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			_ = conn.WriteMessage(websocket.TextMessage, []byte("WS_FRAME_PAYLOAD_MUST_NOT_ENTER_FACTS"))
			time.Sleep(150 * time.Millisecond)
			_ = conn.Close()
		case "/main-fail":
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "hijacking unavailable", http.StatusInternalServerError)
				return
			}
			conn, _, err := hijacker.Hijack()
			if err == nil {
				_ = conn.Close()
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	sink := &telemetryLiveSink{}
	manager := NewSessionManagerWithSink(cfg, sink)
	manager.SetFactRetractor(sink)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	t.Cleanup(func() {
		if err := manager.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown browser manager: %v", err)
		}
	})

	session, err := manager.CreateTab(ctx, "", server.URL+"/start", false)
	if err != nil {
		t.Fatalf("CreateTab: %v", err)
	}
	page, ok := manager.Page(session.ID)
	if !ok || page == nil {
		t.Fatal("created session has no page")
	}

	initial := waitForTelemetryFacts(t, ctx, sink, func(facts []mangle.Fact) bool {
		return hasFactMatching(facts, "net_http_error", 2, "/http-error") &&
			hasFactMatching(facts, "net_http_error", 2, "/asset-error.js") &&
			hasCanceledRequest(facts, "/cancel") &&
			hasFactMatching(facts, "js_dialog", 1, "confirm") &&
			hasFactMatching(facts, "ws_event", 3, "error") &&
			hasFactMatching(facts, "ws_event", 3, "handshake_response") &&
			// console.error arrives via Runtime.consoleAPICalled as a
			// console_event; the Log domain (browser_log) carries
			// network/security entries, not console API calls, in this
			// Chrome build — waiting for it as browser_log never
			// satisfied and timed the test out at 45s.
			hasFactMatching(facts, "console_event", 2, "access_token")
	})
	select {
	case <-canceledStarted:
	case <-ctx.Done():
		t.Fatal("canceled fetch did not reach the local server")
	}

	startRequestID := requestIDForURL(initial, "/start")
	if startRequestID == "" || !hasFactWithRequestStatus(initial, "net_response", startRequestID, 200) {
		t.Fatalf("first navigation response was not captured after listener readiness: facts=%v", summarizeTelemetryFacts(initial))
	}
	if !hasFactWithRequestStatusAndType(initial, "net_http_error", "/http-error", 503, "Fetch") {
		t.Fatalf("real API HTTP error missing status/type: %v", summarizeTelemetryFacts(initial))
	}
	if !hasFactWithRequestStatusAndType(initial, "net_http_error", "/asset-error.js", 500, "Script") {
		t.Fatalf("real asset HTTP error missing status/type: %v", summarizeTelemetryFacts(initial))
	}
	if hasFactMatching(initial, "page_load_failed", 1, "/asset-error.js") || hasFactMatching(initial, "page_load_failed", 1, "/cancel") {
		t.Fatalf("asset or canceled request was misclassified as a page load failure: %v", summarizeTelemetryFacts(initial))
	}
	if !hasSuccessfulWebSocketHandshake(initial, "/ws-ok") || !hasFactMatching(initial, "ws_event", 3, "closed") {
		t.Fatalf("successful WebSocket lifecycle metadata missing: %v", summarizeTelemetryFacts(initial))
	}
	if !hasFactWithDialogAnswer(initial, "false", "default_deny") {
		t.Fatalf("JavaScript dialog was not default-dismissed: %v", summarizeTelemetryFacts(initial))
	}
	if result, evalErr := page.Context(ctx).Eval(`() => window.__dialogResult`); evalErr != nil || result.Value.Bool() {
		t.Fatalf("confirm result = %v, %v; default-denied dialogs must return false", result, evalErr)
	}
	if result, evalErr := page.Context(ctx).Eval(`() => window.__wsMessage`); evalErr != nil || result.Value.Str() != "WS_FRAME_PAYLOAD_MUST_NOT_ENTER_FACTS" {
		t.Fatalf("local WebSocket fixture did not deliver its control payload: %v, %v", result, evalErr)
	}
	for _, fact := range initial {
		for _, arg := range fact.Args {
			text := fmt.Sprint(arg)
			if strings.Contains(text, "console-secret-marker") || strings.Contains(text, "dialog-secret-marker") || strings.Contains(text, "WS_FRAME_PAYLOAD_MUST_NOT_ENTER_FACTS") {
				t.Fatalf("telemetry leaked a secret or WebSocket frame payload in %s: %q", fact.Predicate, text)
			}
		}
	}
	for _, fact := range initial {
		index, timed := map[string]int{"net_http_error": 5, "browser_log": 5, "js_dialog": 5, "console_event": 3}[fact.Predicate]
		if !timed {
			continue
		}
		if len(fact.Args) <= index {
			t.Fatalf("%s missing timestamp slot: %v", fact.Predicate, fact.Args)
		}
		timestamp := factInt64(fact.Args[index])
		if timestamp < time.Now().Add(-2*time.Minute).UnixMilli() || timestamp > time.Now().Add(2*time.Minute).UnixMilli() {
			t.Fatalf("%s timestamp is not epoch milliseconds with CDP/callback provenance: %v", fact.Predicate, fact.Args[index])
		}
	}
	if stats := manager.SessionFactStats(session.ID); stats.Asserted > stats.Budget {
		t.Fatalf("event stream exceeded its per-epoch bound: %+v", stats)
	}

	initialRetractions := len(sink.retractedSnapshot())
	_ = page.Context(ctx).Navigate(server.URL + "/main-fail")
	current := waitForTelemetryFacts(t, ctx, sink, func(facts []mangle.Fact) bool {
		return hasFactMatching(facts, "page_load_failed", 1, "/main-fail")
	})
	if !hasFactForMainFailure(current, "/main-fail") {
		t.Fatalf("non-canceled main-frame Document failure was not classified: %v", summarizeTelemetryFacts(current))
	}
	if !containsRetractedPredicate(sink.retractedSnapshot()[initialRetractions:], "net_http_error", "/http-error") {
		t.Fatalf("navigation did not retract prior epoch telemetry: %v", summarizeTelemetryFacts(sink.retractedSnapshot()[initialRetractions:]))
	}
	if err := manager.CloseSession(ctx, session.ID); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if !containsRetractedPredicate(sink.retractedSnapshot(), "page_load_failed", "/main-fail") {
		t.Fatalf("session close did not retract its final event epoch: %v", summarizeTelemetryFacts(sink.retractedSnapshot()))
	}
}

func waitForTelemetryFacts(t *testing.T, ctx context.Context, sink *telemetryLiveSink, ready func([]mangle.Fact) bool) []mangle.Fact {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		facts := sink.snapshot()
		if ready(facts) {
			return facts
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for browser telemetry: %v", summarizeTelemetryFacts(facts))
		case <-ticker.C:
		}
	}
}

func hasFactMatching(facts []mangle.Fact, predicate string, argIndex int, substring string) bool {
	for _, fact := range facts {
		if fact.Predicate == predicate && len(fact.Args) > argIndex && strings.Contains(fmt.Sprint(fact.Args[argIndex]), substring) {
			return true
		}
	}
	return false
}

func requestIDForURL(facts []mangle.Fact, substring string) string {
	for _, fact := range facts {
		if fact.Predicate == "net_request" && len(fact.Args) >= 4 && strings.Contains(fmt.Sprint(fact.Args[3]), substring) {
			return fmt.Sprint(fact.Args[1])
		}
	}
	return ""
}

func hasFactWithRequestStatus(facts []mangle.Fact, predicate, requestID string, status int64) bool {
	for _, fact := range facts {
		if fact.Predicate == predicate && len(fact.Args) >= 3 && fmt.Sprint(fact.Args[1]) == requestID && factInt64(fact.Args[2]) == status {
			return true
		}
	}
	return false
}

func hasFactWithRequestStatusAndType(facts []mangle.Fact, predicate, urlSubstring string, status int64, resourceType string) bool {
	for _, fact := range facts {
		if fact.Predicate == predicate && len(fact.Args) >= 6 && strings.Contains(fmt.Sprint(fact.Args[2]), urlSubstring) &&
			factInt64(fact.Args[3]) == status && fmt.Sprint(fact.Args[4]) == resourceType {
			return true
		}
	}
	return false
}

func hasCanceledRequest(facts []mangle.Fact, urlSubstring string) bool {
	requestID := requestIDForURL(facts, urlSubstring)
	if requestID == "" {
		return false
	}
	for _, fact := range facts {
		if fact.Predicate == "net_loading_failed" && len(fact.Args) >= 5 && fmt.Sprint(fact.Args[1]) == requestID && fmt.Sprint(fact.Args[3]) == "true" {
			return true
		}
	}
	return false
}

func hasSuccessfulWebSocketHandshake(facts []mangle.Fact, urlSubstring string) bool {
	for _, fact := range facts {
		if fact.Predicate == "ws_event" && len(fact.Args) >= 5 && strings.Contains(fmt.Sprint(fact.Args[2]), urlSubstring) &&
			fmt.Sprint(fact.Args[3]) == "handshake_response" && strings.Contains(fmt.Sprint(fact.Args[4]), "status=101") {
			return true
		}
	}
	return false
}

func hasFactWithDialogAnswer(facts []mangle.Fact, accepted, handledBy string) bool {
	for _, fact := range facts {
		if fact.Predicate == "js_dialog" && len(fact.Args) >= 5 && fmt.Sprint(fact.Args[3]) == accepted && fmt.Sprint(fact.Args[4]) == handledBy {
			return true
		}
	}
	return false
}

func hasFactForMainFailure(facts []mangle.Fact, urlSubstring string) bool {
	for _, fact := range facts {
		if fact.Predicate == "page_load_failed" && len(fact.Args) >= 4 && strings.Contains(fmt.Sprint(fact.Args[1]), urlSubstring) &&
			strings.Contains(fmt.Sprint(fact.Args[2]), "ERR_EMPTY_RESPONSE") {
			return true
		}
	}
	return false
}

func containsRetractedPredicate(facts []mangle.Fact, predicate, substring string) bool {
	for _, fact := range facts {
		if fact.Predicate != predicate {
			continue
		}
		for _, arg := range fact.Args {
			if strings.Contains(fmt.Sprint(arg), substring) {
				return true
			}
		}
	}
	return false
}

func factInt64(value any) int64 {
	switch number := value.(type) {
	case int:
		return int64(number)
	case int64:
		return number
	case float64:
		return int64(number)
	case string:
		var parsed int64
		_, _ = fmt.Sscan(number, &parsed)
		return parsed
	default:
		return 0
	}
}

func summarizeTelemetryFacts(facts []mangle.Fact) string {
	counts := map[string]int{}
	var diagnostic []string
	for _, fact := range facts {
		counts[fact.Predicate]++
		switch fact.Predicate {
		case "net_http_error", "net_loading_failed", "page_load_failed", "ws_event", "js_dialog", "browser_log":
			diagnostic = append(diagnostic, fmt.Sprintf("%s%v", fact.Predicate, fact.Args[1:]))
		}
	}
	return fmt.Sprintf("counts=%v diagnostic=%v", counts, diagnostic)
}

func TestTelemetryRedaction_UsesBuiltInSecretAssignmentNames(t *testing.T) {
	sink := &testEngineSinkLocal{}
	manager := NewSessionManagerWithSink(DefaultConfig(), sink)
	fact := mangle.Fact{Predicate: "browser_log", Args: []any{"s", "console", "error", "access_token=never_publish", "", int64(1)}}
	if err := manager.addFacts([]mangle.Fact{fact}); err != nil {
		t.Fatalf("addFacts: %v", err)
	}
	got := sink.findFactsByPredicate("browser_log")
	if len(got) != 1 || strings.Contains(fmt.Sprint(got[0].Args), "never_publish") {
		t.Fatalf("configured sensitive value leaked: %v", got)
	}
}
