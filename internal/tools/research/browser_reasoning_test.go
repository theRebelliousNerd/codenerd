package research

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/browser"
	"codenerd/internal/types"

	"codeberg.org/TauCeti/mangle-go/analysis"
)

type browserReasoningKernel struct {
	mu    sync.RWMutex
	facts []types.Fact
}

func (k *browserReasoningKernel) LoadFacts(facts []types.Fact) error { return k.AssertBatch(facts) }
func (k *browserReasoningKernel) Query(query string) ([]types.Fact, error) {
	predicate := strings.TrimSpace(strings.TrimSuffix(query, "."))
	if index := strings.IndexByte(predicate, '('); index >= 0 {
		predicate = strings.TrimSpace(predicate[:index])
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	result := make([]types.Fact, 0)
	for _, fact := range k.facts {
		if fact.Predicate == predicate {
			result = append(result, types.Fact{Predicate: fact.Predicate, Args: append([]any(nil), fact.Args...)})
		}
	}
	return result, nil
}
func (k *browserReasoningKernel) QueryAll() (map[string][]types.Fact, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	result := make(map[string][]types.Fact)
	for _, fact := range k.facts {
		result[fact.Predicate] = append(result[fact.Predicate], fact)
	}
	return result, nil
}
func (k *browserReasoningKernel) Assert(fact types.Fact) error {
	k.mu.Lock()
	k.facts = append(k.facts, types.Fact{Predicate: fact.Predicate, Args: append([]any(nil), fact.Args...)})
	k.mu.Unlock()
	return nil
}
func (k *browserReasoningKernel) AssertBatch(facts []types.Fact) error {
	for _, fact := range facts {
		if err := k.Assert(fact); err != nil {
			return err
		}
	}
	return nil
}
func (k *browserReasoningKernel) Retract(string) error                      { return nil }
func (k *browserReasoningKernel) RetractFact(types.Fact) error              { return nil }
func (k *browserReasoningKernel) UpdateSystemFacts() error                  { return nil }
func (k *browserReasoningKernel) GetProgramInfo() *analysis.ProgramInfo     { return nil }
func (k *browserReasoningKernel) Reset()                                    {}
func (k *browserReasoningKernel) AppendPolicy(string)                       {}
func (k *browserReasoningKernel) RetractExactFactsBatch([]types.Fact) error { return nil }
func (k *browserReasoningKernel) RemoveFactsByPredicateSet(map[string]struct{}) error {
	return nil
}

func TestBrowserMangleScopesResultsToBoundSession(t *testing.T) {
	now := time.Now().UnixMilli()
	kernel := &browserReasoningKernel{facts: []types.Fact{
		{Predicate: "console_event", Args: []any{"session-a", "error", "wanted", now}},
		{Predicate: "console_event", Args: []any{"session-b", "error", "foreign", now}},
	}}
	mgr := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	SetBrowserRuntime(mgr, kernel)
	defer ClearBrowserManager(mgr)

	output, err := executeBrowserMangle(context.Background(), map[string]any{
		"operation": "query", "session_id": "session-a", "query": "console_event(S, Level, Message, T)", "view": "full",
	})
	if err != nil {
		t.Fatalf("executeBrowserMangle: %v", err)
	}
	if !strings.Contains(output, "wanted") || strings.Contains(output, "foreign") {
		t.Fatalf("session scope leaked: %s", output)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil || decoded["count"] != float64(1) {
		t.Fatalf("unexpected output: %v, %s", err, output)
	}
}

func TestValidateBrowserQueryRejectsGeneralKernelAndRules(t *testing.T) {
	for _, query := range []string{"user_intent(X)", "console_event(S,L,M,T) :- user_intent(X)", "console_event("} {
		if _, err := validateBrowserQuery(query); err == nil {
			t.Fatalf("validateBrowserQuery(%q) accepted", query)
		}
	}
	if predicate, err := validateBrowserQuery("failed_request_at(S, R, U, Status, T)."); err != nil || predicate != "failed_request_at" {
		t.Fatalf("valid browser query = %q, %v", predicate, err)
	}
}

func TestWaitForBrowserConditionsRequiresFreshFact(t *testing.T) {
	now := time.Now()
	kernel := &browserReasoningKernel{facts: []types.Fact{{
		Predicate: "console_event", Args: []any{"session-a", "error", "old", now.Add(-time.Second).UnixMilli()},
	}}}
	go func() {
		time.Sleep(75 * time.Millisecond)
		_ = kernel.Assert(types.Fact{Predicate: "console_event", Args: []any{"session-a", "error", "new", time.Now().UnixMilli()}})
	}()

	result, err := waitForBrowserConditions(context.Background(), kernel, "session-a", []browserFactCondition{{
		Predicate: "console_event", MatchArgs: []string{"error", "_", "_"},
	}}, true, 0, time.Second, 25*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForBrowserConditions: %v", err)
	}
	rows := result["facts"].([]map[string]any)
	args := rows[0]["args"].([]any)
	if args[2] != "new" {
		t.Fatalf("stale fact satisfied fresh wait: %+v", rows)
	}
}

func TestWaitForBrowserConditionsRejectsUntimestampedFreshPredicate(t *testing.T) {
	_, err := waitForBrowserConditions(context.Background(), &browserReasoningKernel{}, "session-a", []browserFactCondition{{
		Predicate: "current_url",
	}}, true, 0, time.Second, 25*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "no freshness timestamp") {
		t.Fatalf("expected freshness refusal, got %v", err)
	}
}

func TestWaitForStableBrowserTracksFreshActiveRequests(t *testing.T) {
	now := time.Now()
	kernel := &browserReasoningKernel{facts: []types.Fact{{
		Predicate: "net_request", Args: []any{"session-a", "req-1", "GET", "/api", "fetch", now.UnixMilli()},
	}}}
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = kernel.Assert(types.Fact{Predicate: "net_response", Args: []any{"session-a", "req-1", int64(200), int64(5), int64(100)}})
	}()

	result, err := waitForStableBrowser(context.Background(), kernel, "session-a", now.UnixMilli(), time.Second, 25*time.Millisecond, 50*time.Millisecond, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForStableBrowser: %v", err)
	}
	if result["status"] != "stable" || result["duration_ms"].(int64) < 75 {
		t.Fatalf("stable wait returned before request completion: %+v", result)
	}
}

func TestWaitForStableBrowserTreatsNetworkFailureAsCompletion(t *testing.T) {
	now := time.Now()
	kernel := &browserReasoningKernel{facts: []types.Fact{{
		Predicate: "net_request", Args: []any{"session-a", "req-failed", "GET", "/abort", "fetch", now.UnixMilli()},
	}}}
	go func() {
		time.Sleep(75 * time.Millisecond)
		_ = kernel.Assert(types.Fact{Predicate: "net_failure", Args: []any{"session-a", "req-failed", "ERR_ABORTED", "canceled", time.Now().UnixMilli()}})
	}()

	result, err := waitForStableBrowser(context.Background(), kernel, "session-a", now.UnixMilli(), time.Second, 25*time.Millisecond, 50*time.Millisecond, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForStableBrowser: %v", err)
	}
	if result["status"] != "stable" || result["duration_ms"].(int64) < 50 {
		t.Fatalf("failed request did not complete stability tracking: %+v", result)
	}
}

func TestCorrelateBrowserFailuresUsesBoundedTimestampWindow(t *testing.T) {
	failed := []types.Fact{{Predicate: "failed_request_at", Args: []any{"session-a", "req-1", "/api", int64(500), int64(1000)}}}
	visible := []types.Fact{
		{Predicate: "user_visible_error", Args: []any{"session-a", "toast", "save failed", int64(1200)}},
		{Predicate: "user_visible_error", Args: []any{"session-a", "toast", "too late", int64(9000)}},
	}
	correlations := correlateBrowserFailures(failed, visible, time.Second)
	if len(correlations) != 1 || correlations[0]["request_id"] != "req-1" || correlations[0]["delta_ms"] != int64(200) {
		t.Fatalf("unexpected correlations: %+v", correlations)
	}
}
