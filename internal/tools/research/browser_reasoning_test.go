package research

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

func TestQueryScopedBrowserFactsReportsScanTruncation(t *testing.T) {
	facts := make([]types.Fact, maxBrowserKernelScan+1)
	for index := range facts {
		facts[index] = types.Fact{Predicate: "console_event", Args: []any{"other-session", "error", index, int64(index)}}
	}
	kernel := &browserReasoningKernel{facts: facts}
	if _, err := queryScopedBrowserFacts(context.Background(), kernel, "console_event", "console_event", "session-a"); !errors.Is(err, errBrowserKernelScanLimit) {
		t.Fatalf("expected explicit scan limit, got %v", err)
	}
}

func TestWaitForBrowserConditionsRequiresFreshFact(t *testing.T) {
	// A successful wait redacts fact args through the bound manager. There is
	// no fallback manager when the runtime is unbound.
	mgr := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	SetBrowserRuntime(mgr, nil)
	t.Cleanup(func() { ClearBrowserManager(mgr) })

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

func TestBrowserManglePagesTheWholeResult(t *testing.T) {
	window := pinResearchDefaults(t).BrowserReasonItems
	const total = 30
	facts := make([]types.Fact, total)
	for i := range facts {
		facts[i] = types.Fact{
			Predicate: "console_event",
			Args:      []any{"session-a", "info", fmt.Sprintf("m-%02d", i), int64(i + 1)},
		}
	}
	kernel := &browserReasoningKernel{facts: facts}
	mgr := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	SetBrowserRuntime(mgr, kernel)
	defer ClearBrowserManager(mgr)

	firstRaw, err := executeBrowserMangle(context.Background(), map[string]any{
		"operation": "query", "session_id": "session-a",
		"query": "console_event(S, Level, Message, T)", "view": "full",
	})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	first, decoded := browserMangleMessages(t, firstRaw)
	if decoded["count"] != float64(total) || decoded["truncated"] != true {
		t.Fatalf("count/truncated = %v/%v, want %d/true", decoded["count"], decoded["truncated"], total)
	}
	wantHint := fmt.Sprintf("%d more facts, page with offset=%d", total-window, window)
	if decoded["facts_hint"] != wantHint {
		t.Fatalf("facts_hint = %v, want %q", decoded["facts_hint"], wantHint)
	}
	if len(first) != window || first[0] != fmt.Sprintf("m-%02d", total-1) {
		t.Fatalf("first page = %v, want %d newest facts starting at m-%02d", first, window, total-1)
	}

	secondRaw, err := executeBrowserMangle(context.Background(), map[string]any{
		"operation": "query", "session_id": "session-a",
		"query": "console_event(S, Level, Message, T)", "view": "full",
		"offset": window,
	})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	second, decoded := browserMangleMessages(t, secondRaw)
	if decoded["truncated"] != false {
		t.Fatalf("second page still truncated: %s", secondRaw)
	}
	if _, ok := decoded["facts_hint"]; ok {
		t.Fatalf("second page named a remainder: %v", decoded["facts_hint"])
	}
	got := append(append([]string{}, first...), second...)
	if len(got) != total {
		t.Fatalf("paged %d messages, want %d", len(got), total)
	}
	for i, message := range got {
		want := fmt.Sprintf("m-%02d", total-1-i)
		if message != want {
			t.Fatalf("paged[%d] = %q, want %q", i, message, want)
		}
	}
}

func TestBrowserMangleHonorsMaxItemsAboveTheOldCap(t *testing.T) {
	const total = 120 // the deleted maxBrowserReasonItems cap was 100
	facts := make([]types.Fact, total)
	for i := range facts {
		facts[i] = types.Fact{
			Predicate: "console_event",
			Args:      []any{"session-a", "info", fmt.Sprintf("c-%03d", i), int64(i + 1)},
		}
	}
	kernel := &browserReasoningKernel{facts: facts}
	mgr := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	SetBrowserRuntime(mgr, kernel)
	defer ClearBrowserManager(mgr)

	raw, err := executeBrowserMangle(context.Background(), map[string]any{
		"operation": "query", "session_id": "session-a",
		"query": "console_event(S, Level, Message, T)", "view": "full",
		"max_items": total,
	})
	if err != nil {
		t.Fatalf("executeBrowserMangle: %v", err)
	}
	messages, decoded := browserMangleMessages(t, raw)
	if decoded["truncated"] != false {
		t.Fatalf("explicit window covering every fact was truncated: %s", raw)
	}
	if _, ok := decoded["facts_hint"]; ok {
		t.Fatalf("whole result carried a paging hint: %v", decoded["facts_hint"])
	}
	if len(messages) != total {
		t.Fatalf("returned %d facts, want all %d", len(messages), total)
	}
}

func TestBrowserMangleReadKeepsFactsFromLaterPredicates(t *testing.T) {
	// click_event sorts before console_event. The old read stopped once it
	// had max_items+1 facts, so a later predicate's newer facts were dropped
	// before the newest-first sort. The read is whole now; paging happens after.
	facts := make([]types.Fact, 0, 33)
	for i := 0; i < 30; i++ {
		facts = append(facts, types.Fact{
			Predicate: "click_event",
			Args:      []any{"session-a", fmt.Sprintf("click-%02d", i), int64(i + 1)},
		})
	}
	for i := 0; i < 3; i++ {
		facts = append(facts, types.Fact{
			Predicate: "console_event",
			Args:      []any{"session-a", "info", fmt.Sprintf("console-%d", i), int64(1000 + i)},
		})
	}
	kernel := &browserReasoningKernel{facts: facts}
	mgr := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	SetBrowserRuntime(mgr, kernel)
	defer ClearBrowserManager(mgr)

	raw, err := executeBrowserMangle(context.Background(), map[string]any{
		"operation": "read", "session_id": "session-a", "view": "full", "max_items": 5,
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	_, decoded := browserMangleMessages(t, raw)
	if decoded["count"] != float64(33) || decoded["truncated"] != true {
		t.Fatalf("count/truncated = %v/%v, want 33/true (%s)", decoded["count"], decoded["truncated"], raw)
	}
	rows, _ := decoded["facts"].([]any)
	if len(rows) != 5 {
		t.Fatalf("window = %d, want 5", len(rows))
	}
	consoles := 0
	for _, item := range rows {
		row, _ := item.(map[string]any)
		if row["predicate"] == "console_event" {
			consoles++
		}
	}
	if consoles != 3 {
		t.Fatalf("first page kept %d console facts, want all 3 newer ones: %s", consoles, raw)
	}
}

func TestBrowserMangleScanLimitIsAnError(t *testing.T) {
	facts := make([]types.Fact, maxBrowserKernelScan+1)
	for index := range facts {
		facts[index] = types.Fact{Predicate: "console_event", Args: []any{"session-a", "info", index, int64(index)}}
	}
	kernel := &browserReasoningKernel{facts: facts}
	mgr := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	SetBrowserRuntime(mgr, kernel)
	defer ClearBrowserManager(mgr)

	output, err := executeBrowserMangle(context.Background(), map[string]any{
		"operation": "query", "session_id": "session-a",
		"query": "console_event(S, Level, Message, T)", "view": "full", "max_items": 10,
	})
	if !errors.Is(err, errBrowserKernelScanLimit) {
		t.Fatalf("expected scan-limit error, got %v output %q", err, output)
	}
	if output != "" {
		t.Fatalf("scan limit returned a payload: %s", output)
	}
}

func TestPageBrowserFactsHugeWindowDoesNotWrap(t *testing.T) {
	facts := make([]types.Fact, 8)
	got, end := pageBrowserFacts(facts, 5, math.MaxInt)
	if end != 8 || len(got) != 3 {
		t.Fatalf("huge window = len %d end %d, want 3 facts ending at 8", len(got), end)
	}
}

func TestPageReasonSectionsNamesTheRemainder(t *testing.T) {
	policy := pinResearchDefaults(t)
	const n = 30
	rows := make([]map[string]any, n)
	for i := range rows {
		rows[i] = map[string]any{"id": i}
	}
	data := map[string]any{"failed_requests": rows, "note": "kept-whole"}

	if got := reasonWindow(map[string]any{}, "compact", policy.BrowserReasonItems); got != policy.BrowserReasonCompactItems {
		t.Fatalf("compact default window = %d, want %d", got, policy.BrowserReasonCompactItems)
	}
	page := pageReasonSections(data, 0, policy.BrowserReasonCompactItems)
	if page["note"] != "kept-whole" {
		t.Fatalf("non-row section = %v", page["note"])
	}
	window, _ := page["failed_requests"].([]map[string]any)
	if len(window) != policy.BrowserReasonCompactItems || page["failed_requests_truncated"] != true || page["failed_requests_total"] != n {
		t.Fatalf("first page markers = len %d truncated %v total %v", len(window), page["failed_requests_truncated"], page["failed_requests_total"])
	}
	wantHint := fmt.Sprintf("%d more rows, page with offset=%d", n-policy.BrowserReasonCompactItems, policy.BrowserReasonCompactItems)
	if page["failed_requests_hint"] != wantHint {
		t.Fatalf("hint = %v, want %q", page["failed_requests_hint"], wantHint)
	}

	// An explicit max_items is the model's window in both views. The old
	// compact path used min(max, 10) and every path capped at 100.
	compactArgs := map[string]any{"max_items": 25}
	view, maxItems, offset, err := normalizeReasonView(compactArgs)
	if err != nil || view != "compact" || maxItems != 25 || offset != 0 {
		t.Fatalf("normalize compact = %s %d %d %v", view, maxItems, offset, err)
	}
	if got := reasonWindow(compactArgs, view, maxItems); got != 25 {
		t.Fatalf("explicit compact window = %d, want 25", got)
	}
	fullArgs := map[string]any{"view": "full", "max_items": 150}
	view, maxItems, _, err = normalizeReasonView(fullArgs)
	if err != nil || view != "full" || maxItems != 150 {
		t.Fatalf("normalize full = %s %d %v", view, maxItems, err)
	}
	if got := reasonWindow(fullArgs, view, maxItems); got != 150 {
		t.Fatalf("explicit full window = %d, want 150 (old cap was 100)", got)
	}
	whole := pageReasonSections(data, 0, 150)
	wholeRows, _ := whole["failed_requests"].([]map[string]any)
	if len(wholeRows) != n {
		t.Fatalf("window covering the section returned %d rows", len(wholeRows))
	}
	if _, ok := whole["failed_requests_truncated"]; ok {
		t.Fatalf("whole section was marked truncated: %v", whole["failed_requests_hint"])
	}

	var ids []int
	for off := 0; off < n; {
		paged := pageReasonSections(data, off, policy.BrowserReasonCompactItems)
		part, _ := paged["failed_requests"].([]map[string]any)
		for _, row := range part {
			ids = append(ids, row["id"].(int))
		}
		if len(part) == 0 {
			t.Fatal("empty page before the rows were exhausted")
		}
		off += len(part)
	}
	if len(ids) != n {
		t.Fatalf("paged %d ids, want %d", len(ids), n)
	}
	for i, id := range ids {
		if id != i {
			t.Fatalf("paged[%d] = %d", i, id)
		}
	}

	if _, _, offset, err = normalizeReasonView(map[string]any{"offset": -4}); err != nil || offset != 0 {
		t.Fatalf("negative offset = %d, %v", offset, err)
	}
}

func browserMangleMessages(t *testing.T, raw string) ([]string, map[string]any) {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, raw)
	}
	facts, _ := decoded["facts"].([]any)
	messages := make([]string, 0, len(facts))
	for _, item := range facts {
		row, _ := item.(map[string]any)
		args, _ := row["args"].([]any)
		if len(args) >= 3 {
			messages = append(messages, fmt.Sprint(args[2]))
		}
	}
	return messages, decoded
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
