package research

// Browser reasoning is adapted from BrowserNERD's Apache-2.0 progressive
// reasoning contracts. codeNERD deliberately queries the Cortex-owned kernel
// instead of creating a browser-private Mangle engine.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"codenerd/internal/browser"
	"codenerd/internal/config"
	"codenerd/internal/mangle"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

const (
	// The reason paging windows (research.browser_reason_items and
	// research.browser_reason_compact_items) are read from the installed
	// research policy at the call sites below, not kept here: every fact
	// stays reachable via offset, so the window is paging rather than a
	// cut, and there is deliberately no hard cap — an explicit max_items
	// is the model's own choice and is honored as given, exactly like
	// browser_act's window (browser_progressive.go).
	defaultBrowserTimeout = 10 * time.Second
	maxBrowserTimeout     = 30 * time.Second
	defaultBrowserPoll    = 200 * time.Millisecond
	minBrowserPoll        = 50 * time.Millisecond
	maxBrowserPoll        = time.Second
	maxBrowserConditions  = 10
	// maxBrowserKernelScan bounds ONE kernel predicate scan. It protects
	// the process: a pathological kernel (tens of thousands of facts under
	// one predicate, or a callback kernel that yields unboundedly) turns
	// every browser_mangle query — and every wait poll, which re-queries —
	// into an unbounded memory and CPU sink. A trip returns
	// errBrowserKernelScanLimit. Every caller in this file propagates that
	// error, so the model gets a failure instead of a shortened fact list.
	// browser_audit.go does not swallow it: on this error its collectors keep
	// the partial rows the scan already returned and attach a note naming
	// the predicate (LC5b). Any other kernel error still fails the audit.
	maxBrowserKernelScan = 2000
	defaultReasonWindow  = 5 * time.Minute
	maxReasonWindow      = 24 * time.Hour
)

type browserPredicateSpec struct {
	timestampIndex int
}

// All exposed predicates are session-scoped at argument zero. Keeping this an
// explicit allowlist prevents browser_mangle from becoming a general kernel
// exfiltration surface.
var browserPredicateSpecs = map[string]browserPredicateSpec{
	"navigation_event":       {timestampIndex: 2},
	"current_url":            {timestampIndex: -1},
	"console_event":          {timestampIndex: 3},
	"click_event":            {timestampIndex: 2},
	"input_event":            {timestampIndex: 3},
	"state_change":           {timestampIndex: 3},
	"dom_updated":            {timestampIndex: 1},
	"toast_notification":     {timestampIndex: 4},
	"browser_page_state":     {timestampIndex: 4},
	"net_request":            {timestampIndex: 5},
	"net_response":           {timestampIndex: -1},
	"net_header":             {timestampIndex: -1},
	"request_initiator":      {timestampIndex: -1},
	"net_failure":            {timestampIndex: 4},
	"failed_request":         {timestampIndex: -1},
	"failed_request_at":      {timestampIndex: 4},
	"slow_api":               {timestampIndex: -1},
	"slow_api_at":            {timestampIndex: 4},
	"root_cause":             {timestampIndex: -1},
	"root_cause_at":          {timestampIndex: 4},
	"user_visible_error":     {timestampIndex: 3},
	"interaction_blocked":    {timestampIndex: -1},
	"interaction_blocked_at": {timestampIndex: 2},
}

type browserFactCondition struct {
	Predicate string   `json:"predicate"`
	MatchArgs []string `json:"match_args,omitempty"`
}

type browserKernelCallback interface {
	QueryCallback(string, func(types.Fact) error) error
}

var errBrowserKernelScanLimit = errors.New("browser kernel scan limit reached")

// BrowserMangleTool returns bounded, read-only access to browser facts in the
// live Cortex kernel.
func BrowserMangleTool() *tools.Tool {
	reasonDefault := config.ResolvedResearchPolicy().BrowserReasonItems
	return &tools.Tool{
		Name:        "browser_mangle",
		Description: `Read and wait on session-scoped browser facts in the live Cortex kernel. Operations: query, read, temporal, evaluate, await_fact, await_conditions. Results page through a max_items window starting at offset; when facts continue past the window the result names the remainder and the offset that reaches it. Waits are cancelable and fresh-only by default. Rule submission and fact mutation are intentionally unavailable because they could change constitutional reality.`,
		Category:    tools.CategoryResearch,
		Priority:    72,
		Execute:     executeBrowserMangle,
		Schema: tools.ToolSchema{
			Required: []string{"operation", "session_id"},
			Properties: map[string]tools.Property{
				"operation":        {Type: "string", Enum: []any{"query", "read", "temporal", "evaluate", "await_fact", "await_conditions"}},
				"session_id":       {Type: "string", Description: "Session scope enforced on every result"},
				"query":            {Type: "string", Description: "Single allowed browser atom, for example failed_request_at(S, R, U, Status, T)"},
				"predicate":        {Type: "string", Description: "Allowed browser predicate for read/evaluate/temporal/await_fact"},
				"match_args":       {Type: "array", Description: "Arguments after session_id; use _ as a wildcard", Items: &tools.PropertyItems{Type: "string"}},
				"conditions":       {Type: "array", Description: "Up to 10 predicate/match_args objects", Items: &tools.PropertyItems{Type: "object"}},
				"after_ms":         {Type: "integer", Description: "Inclusive epoch-ms lower bound"},
				"before_ms":        {Type: "integer", Description: "Inclusive epoch-ms upper bound"},
				"fresh_only":       {Type: "boolean", Default: true, Description: "Wait only for facts at/after the call watermark"},
				"since_ms":         {Type: "integer", Description: "Optional freshness watermark, typically browser_act.started_ms"},
				"timeout_ms":       {Type: "integer", Default: 10000, Description: "Hard-capped at 30000"},
				"poll_interval_ms": {Type: "integer", Default: 200, Description: "Clamped to 50..1000"},
				"view":             {Type: "string", Default: "compact", Enum: []any{"summary", "compact", "full"}},
				"max_items":        {Type: "integer", Default: reasonDefault, Description: "Results window size; every fact stays reachable via offset"},
				"offset":           {Type: "integer", Default: 0, Description: "Fact offset where the max_items window starts (default: 0)"},
			},
		},
	}
}

// BrowserWaitTool consolidates stable, fact, and multi-condition waits.
func BrowserWaitTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_wait",
		Description: `Wait for fresh live-kernel browser evidence. Modes: stable (network and DOM quiescence), fact, conditions (AND). All modes require session_id, honor cancellation, cap timeout at 30s, and return bounded evidence rather than polling in the model.`,
		Category:    tools.CategoryResearch,
		Priority:    71,
		Execute:     executeBrowserWait,
		Schema: tools.ToolSchema{
			Required: []string{"session_id", "mode"},
			Properties: map[string]tools.Property{
				"session_id":       {Type: "string"},
				"mode":             {Type: "string", Enum: []any{"stable", "fact", "conditions"}},
				"predicate":        {Type: "string"},
				"match_args":       {Type: "array", Description: "Arguments after session_id; use _ as a wildcard", Items: &tools.PropertyItems{Type: "string"}},
				"conditions":       {Type: "array", Items: &tools.PropertyItems{Type: "object"}},
				"fresh_only":       {Type: "boolean", Default: true},
				"since_ms":         {Type: "integer", Description: "Optional freshness watermark, typically browser_act.started_ms"},
				"timeout_ms":       {Type: "integer", Default: 10000},
				"poll_interval_ms": {Type: "integer", Default: 200},
				"network_idle_ms":  {Type: "integer", Default: 500},
				"dom_idle_ms":      {Type: "integer", Default: 200},
			},
		},
	}
}

// BrowserReasonTool returns bounded session diagnosis from the live kernel.
func BrowserReasonTool() *tools.Tool {
	reasonDefault := config.ResolvedResearchPolicy().BrowserReasonItems
	return &tools.Tool{
		Name:        "browser_reason",
		Description: `Diagnose one browser session from fresh live Cortex facts. Topics: health, next_best_action, blocking_issue, why_failed, what_changed_since. Summary is cheapest; compact pages key evidence through a small window and full through a larger one, and every row stays reachable via offset. Current-route scoping is enabled by default.`,
		Category:    tools.CategoryResearch,
		Priority:    73,
		Execute:     executeBrowserReason,
		Schema: tools.ToolSchema{
			Required: []string{"session_id"},
			Properties: map[string]tools.Property{
				"session_id":       {Type: "string"},
				"topic":            {Type: "string", Default: "health", Enum: []any{"health", "next_best_action", "blocking_issue", "why_failed", "what_changed_since"}},
				"view":             {Type: "string", Default: "compact", Enum: []any{"summary", "compact", "full"}},
				"max_items":        {Type: "integer", Default: reasonDefault, Description: "Per-section window size; every row stays reachable via offset"},
				"offset":           {Type: "integer", Default: 0, Description: "Row offset where the max_items window starts (default: 0)"},
				"time_window_ms":   {Type: "integer", Default: 300000, Description: "Clamped to 0..86400000"},
				"since_navigation": {Type: "boolean", Default: true},
			},
		},
	}
}

func executeBrowserMangle(ctx context.Context, args map[string]any) (string, error) {
	kernel := getBrowserKernel()
	if kernel == nil {
		return "", fmt.Errorf("browser mangle: live Cortex kernel is not bound")
	}
	sessionID := strings.TrimSpace(stringArg(args, "session_id"))
	if sessionID == "" {
		return "", fmt.Errorf("browser mangle: session_id is required")
	}
	operation := strings.ToLower(strings.TrimSpace(stringArg(args, "operation")))
	view, maxItems, offset, err := normalizeReasonView(args)
	if err != nil {
		return "", fmt.Errorf("browser mangle: %w", err)
	}

	var facts []types.Fact
	switch operation {
	case "query":
		query := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(stringArg(args, "query")), "."))
		predicate, queryErr := validateBrowserQuery(query)
		if queryErr != nil {
			return "", fmt.Errorf("browser mangle: %w", queryErr)
		}
		facts, err = queryScopedBrowserFacts(ctx, kernel, query, predicate, sessionID)
	case "read", "evaluate":
		predicate := strings.TrimSpace(stringArg(args, "predicate"))
		if predicate == "" && operation == "read" {
			facts, err = readBrowserFacts(ctx, kernel, sessionID)
		} else {
			if _, ok := browserPredicateSpecs[predicate]; !ok {
				return "", fmt.Errorf("browser mangle: predicate %q is not exposed", predicate)
			}
			facts, err = queryScopedBrowserFacts(ctx, kernel, predicate, predicate, sessionID)
		}
	case "temporal":
		predicate := strings.TrimSpace(stringArg(args, "predicate"))
		spec, ok := browserPredicateSpecs[predicate]
		if !ok || spec.timestampIndex < 0 {
			return "", fmt.Errorf("browser mangle: predicate %q has no temporal contract", predicate)
		}
		facts, err = queryScopedBrowserFacts(ctx, kernel, predicate, predicate, sessionID)
		if err == nil {
			after := int64Arg(args, "after_ms", 0)
			before := int64Arg(args, "before_ms", time.Now().UnixMilli())
			if after < 0 || before < after || before-after > maxReasonWindow.Milliseconds() {
				return "", fmt.Errorf("browser mangle: temporal window must be ordered and at most 24h")
			}
			facts = filterFactsByTime(facts, spec, after, before)
		}
	case "await_fact", "await_conditions":
		conditions, conditionErr := conditionsFromArgs(operation, args)
		if conditionErr != nil {
			return "", fmt.Errorf("browser mangle: %w", conditionErr)
		}
		waited, waitErr := waitForBrowserConditions(ctx, kernel, sessionID, conditions, boolArg(args, "fresh_only", true), int64Arg(args, "since_ms", 0), boundedTimeout(args), boundedPoll(args))
		if waitErr != nil {
			return "", fmt.Errorf("browser mangle: %w", waitErr)
		}
		recordBrowserToolEvidence(sessionID, "wait", map[string]any{
			"tool": "browser_mangle", "operation": operation, "result": waited,
		})
		return marshalProgressiveResult(waited)
	default:
		return "", fmt.Errorf("browser mangle: unsupported operation %q", operation)
	}
	if err != nil {
		return "", fmt.Errorf("browser mangle: %w", err)
	}

	sortFactsNewest(facts)
	total := len(facts)
	facts, end := pageBrowserFacts(facts, offset, maxItems)
	truncated := end < total
	output := map[string]any{
		"success": true, "operation": operation, "session_id": sessionID, "view": view,
		"count": total, "truncated": truncated,
		"summary":          fmt.Sprintf("%s returned %d browser fact(s)", operation, total),
		"evidence_handles": []string{fmt.Sprintf("browser:%s:mangle:%s", sessionID, operation)},
	}
	if truncated {
		output["facts_hint"] = fmt.Sprintf("%d more facts, page with offset=%d", total-end, end)
	}
	if view != "summary" {
		output["facts"] = publicBrowserFacts(getBrowserManager(), facts, view == "full")
	}
	recordBrowserToolEvidence(sessionID, "mangle", map[string]any{
		"operation": operation, "view": view, "count": total, "truncated": truncated,
	})
	return marshalProgressiveResult(output)
}

func executeBrowserWait(ctx context.Context, args map[string]any) (string, error) {
	kernel := getBrowserKernel()
	if kernel == nil {
		return "", fmt.Errorf("browser wait: live Cortex kernel is not bound")
	}
	sessionID := strings.TrimSpace(stringArg(args, "session_id"))
	if sessionID == "" {
		return "", fmt.Errorf("browser wait: session_id is required")
	}
	if _, ok := getBrowserManager().GetSession(sessionID); !ok {
		return "", fmt.Errorf("browser wait: unknown session %s", sessionID)
	}
	mode := strings.ToLower(strings.TrimSpace(stringArg(args, "mode")))
	if mode == "stable" {
		result, err := waitForStableBrowser(ctx, kernel, sessionID, int64Arg(args, "since_ms", 0), boundedTimeout(args), boundedPoll(args), boundedIdle(args, "network_idle_ms", 500), boundedIdle(args, "dom_idle_ms", 200))
		if err != nil {
			return "", fmt.Errorf("browser wait: %w", err)
		}
		recordBrowserToolEvidence(sessionID, "wait", map[string]any{"mode": mode, "result": result})
		return marshalProgressiveResult(result)
	}
	operation := "await_fact"
	if mode == "conditions" {
		operation = "await_conditions"
	} else if mode != "fact" {
		return "", fmt.Errorf("browser wait: unsupported mode %q", mode)
	}
	conditions, err := conditionsFromArgs(operation, args)
	if err != nil {
		return "", fmt.Errorf("browser wait: %w", err)
	}
	result, err := waitForBrowserConditions(ctx, kernel, sessionID, conditions, boolArg(args, "fresh_only", true), int64Arg(args, "since_ms", 0), boundedTimeout(args), boundedPoll(args))
	if err != nil {
		return "", fmt.Errorf("browser wait: %w", err)
	}
	recordBrowserToolEvidence(sessionID, "wait", map[string]any{"mode": mode, "result": result})
	return marshalProgressiveResult(result)
}

func executeBrowserReason(ctx context.Context, args map[string]any) (string, error) {
	kernel := getBrowserKernel()
	if kernel == nil {
		return "", fmt.Errorf("browser reason: live Cortex kernel is not bound")
	}
	sessionID := strings.TrimSpace(stringArg(args, "session_id"))
	if sessionID == "" {
		return "", fmt.Errorf("browser reason: session_id is required")
	}
	view, maxItems, offset, err := normalizeReasonView(args)
	if err != nil {
		return "", fmt.Errorf("browser reason: %w", err)
	}
	topic := strings.ToLower(strings.TrimSpace(stringArg(args, "topic")))
	if topic == "" {
		topic = "health"
	}
	switch topic {
	case "health", "next_best_action", "blocking_issue", "why_failed", "what_changed_since":
	default:
		return "", fmt.Errorf("browser reason: unsupported topic %q", topic)
	}

	// Refresh current page state and assert it into the same live kernel before
	// reading diagnosis. This prevents a cached health result from masquerading
	// as current page evidence.
	stateObservation, err := getBrowserManager().Observe(ctx, sessionID, browser.ObserveOptions{Mode: "state", View: "summary", MaxItems: 1, VisibleOnly: true})
	if err != nil {
		return "", fmt.Errorf("browser reason: refresh page state: %w", err)
	}

	windowMs := int64Arg(args, "time_window_ms", defaultReasonWindow.Milliseconds())
	if windowMs < 0 {
		windowMs = 0
	}
	if windowMs > maxReasonWindow.Milliseconds() {
		windowMs = maxReasonWindow.Milliseconds()
	}
	since := int64(0)
	if windowMs > 0 {
		since = time.Now().UnixMilli() - windowMs
	}
	sinceNavigation := boolArg(args, "since_navigation", true)
	navigationSince := int64(0)
	if sinceNavigation {
		watermark, watermarkErr := latestBrowserTimestamp(ctx, kernel, "navigation_event", sessionID)
		if watermarkErr != nil {
			return "", fmt.Errorf("browser reason: navigation watermark: %w", watermarkErr)
		}
		navigationSince = watermark
		if navigationSince > since {
			since = navigationSince
		}
	}

	sections := map[string][]types.Fact{}
	for _, predicate := range []string{"root_cause_at", "failed_request_at", "slow_api_at", "user_visible_error", "net_failure", "interaction_blocked_at", "console_event", "toast_notification"} {
		facts, queryErr := queryScopedBrowserFacts(ctx, kernel, predicate, predicate, sessionID)
		if queryErr != nil {
			return "", fmt.Errorf("browser reason: query %s: %w", predicate, queryErr)
		}
		spec := browserPredicateSpecs[predicate]
		if since > 0 && spec.timestampIndex >= 0 {
			facts = filterFactsByTime(facts, spec, since, time.Now().UnixMilli())
		}
		sortFactsNewest(facts)
		sections[predicate] = facts
	}

	errorConsole, warningConsole := splitConsoleLevels(sections["console_event"])
	errorToasts, warningToasts := splitToastLevels(sections["toast_notification"])
	failed := sections["failed_request_at"]
	netFailures := sections["net_failure"]
	rootCauses := sections["root_cause_at"]
	slow := sections["slow_api_at"]
	blocked := sections["interaction_blocked_at"]
	if state, ok := stateObservation.Data["state"].(browser.PageState); !ok || !state.HasDialog {
		blocked = nil
	}
	visibleErrors := sections["user_visible_error"]

	status := "ok"
	if len(rootCauses)+len(failed)+len(netFailures)+len(errorConsole)+len(errorToasts) > 0 {
		status = "error"
	} else if len(slow)+len(blocked)+len(warningConsole)+len(warningToasts) > 0 {
		status = "warning"
	}
	correlations := correlateBrowserFailures(failed, visibleErrors, 5*time.Second)
	contradictions := detectBrowserContradictions(failed, sections["toast_notification"])
	recommendations := browserRecommendations(status, len(failed), len(blocked), len(slow), len(netFailures))
	containerEvents := adaptRuntimeErrorEvents(failed, visibleErrors)
	containerResult := getBrowserManager().CorrelateContainerErrors(ctx, containerEvents, 5*time.Second)
	counts := map[string]int{
		"root_causes": len(rootCauses), "failed_requests": len(failed), "network_failures": len(netFailures),
		"slow_apis": len(slow), "blocking_issues": len(blocked), "console_errors": len(errorConsole),
		"console_warnings": len(warningConsole), "error_toasts": len(errorToasts), "warning_toasts": len(warningToasts),
		"correlations": len(correlations), "contradictions": len(contradictions),
	}
	if len(containerResult.Correlations) > 0 || len(containerResult.Notes) > 0 {
		counts["container_correlations"] = len(containerResult.Correlations)
	}
	data := map[string]any{
		"root_causes":         publicBrowserFacts(getBrowserManager(), rootCauses, true),
		"failed_requests":     publicBrowserFacts(getBrowserManager(), failed, true),
		"network_failures":    publicBrowserFacts(getBrowserManager(), netFailures, true),
		"slow_apis":           publicBrowserFacts(getBrowserManager(), slow, true),
		"blocking_issues":     publicBrowserFacts(getBrowserManager(), blocked, true),
		"user_visible_errors": publicBrowserFacts(getBrowserManager(), visibleErrors, true),
		"console_errors":      publicBrowserFacts(getBrowserManager(), errorConsole, true),
		"console_warnings":    publicBrowserFacts(getBrowserManager(), warningConsole, true),
		"error_toasts":        publicBrowserFacts(getBrowserManager(), errorToasts, true),
		"warning_toasts":      publicBrowserFacts(getBrowserManager(), warningToasts, true),
		"correlations":        correlations, "contradictions": contradictions, "recommendations": recommendations,
	}
	if len(containerResult.Correlations) > 0 || len(containerResult.Notes) > 0 {
		data["container_correlations"] = containerResult.Correlations
		if len(containerResult.Notes) > 0 {
			data["container_correlation_notes"] = containerResult.Notes
		}
	}
	if topic == "what_changed_since" {
		data["changes"] = mergeReasonChanges(rootCauses, failed, netFailures, slow, visibleErrors)
	}
	if view == "compact" || view == "full" {
		data = pageReasonSections(data, offset, reasonWindow(args, view, maxItems))
	}

	evidenceHandles := []string{
		"reason:" + sessionID + ":root_causes", "reason:" + sessionID + ":failed_requests",
		"reason:" + sessionID + ":network_failures", "reason:" + sessionID + ":slow_apis",
		"reason:" + sessionID + ":blocking_issues", "reason:" + sessionID + ":user_visible_errors",
		"reason:" + sessionID + ":correlations", "reason:" + sessionID + ":recommendations",
	}
	if len(containerResult.Correlations) > 0 || len(containerResult.Notes) > 0 {
		evidenceHandles = append(evidenceHandles, "reason:"+sessionID+":container_correlations")
	}
	output := map[string]any{
		"success": true, "session_id": sessionID, "topic": topic, "view": view, "status": status,
		"evidence_scope": "bounded_live_kernel", "counts": counts,
		"summary":    fmt.Sprintf("status=%s root_causes=%d failed_requests=%d network_failures=%d slow_apis=%d blocking_issues=%d", status, len(rootCauses), len(failed), len(netFailures), len(slow), len(blocked)),
		"page_state": stateObservation.Data["state"], "time_window_ms": windowMs,
		"since_navigation": sinceNavigation, "navigation_since_ms": navigationSince, "effective_since_ms": since,
		"evidence_handles": evidenceHandles,
	}
	if view != "summary" {
		output["data"] = data
	}
	recordBrowserToolEvidence(sessionID, "reason", map[string]any{
		"topic": topic, "view": view, "status": status, "counts": counts,
		"summary": output["summary"], "effective_since_ms": since,
	})
	return marshalProgressiveResult(output)
}

func validateBrowserQuery(query string) (string, error) {
	query = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(query), "."))
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	if len(query) > 512 || strings.ContainsAny(query, "\r\n;") || strings.Contains(query, ":-") {
		return "", fmt.Errorf("query must be one bounded atom")
	}
	atom, err := mangle.ParseAtom(query)
	if err != nil {
		return "", fmt.Errorf("invalid Mangle atom: %w", err)
	}
	predicate := atom.Predicate.Symbol
	if _, ok := browserPredicateSpecs[predicate]; !ok {
		return "", fmt.Errorf("predicate %q is not exposed", predicate)
	}
	return predicate, nil
}

func queryScopedBrowserFacts(ctx context.Context, kernel types.Kernel, query, predicate, sessionID string) ([]types.Fact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if callbackKernel, ok := kernel.(browserKernelCallback); ok {
		result := make([]types.Fact, 0)
		scanned := 0
		err := callbackKernel.QueryCallback(query, func(fact types.Fact) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			scanned++
			if scanned > maxBrowserKernelScan {
				return errBrowserKernelScanLimit
			}
			if fact.Predicate == predicate && len(fact.Args) > 0 && fmt.Sprint(fact.Args[0]) == sessionID {
				result = append(result, types.Fact{Predicate: fact.Predicate, Args: append([]any(nil), fact.Args...)})
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, errBrowserKernelScanLimit) {
				return result, errBrowserKernelScanLimit
			}
			return nil, err
		}
		return result, nil
	}
	facts, err := kernel.Query(query)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make([]types.Fact, 0, len(facts))
	for index, fact := range facts {
		if index >= maxBrowserKernelScan {
			return result, errBrowserKernelScanLimit
		}
		if fact.Predicate != predicate || len(fact.Args) == 0 || fmt.Sprint(fact.Args[0]) != sessionID {
			continue
		}
		result = append(result, types.Fact{Predicate: fact.Predicate, Args: append([]any(nil), fact.Args...)})
	}
	return result, nil
}

// readBrowserFacts collects every session-scoped fact across the exposed
// predicates. The old maxItems early-stop cut the collection mid-sweep in
// predicate-alphabetical order — before the caller's newest-first sort — so
// the window could hold stale facts while newer ones from later predicates
// were silently dropped. Collection is whole now; the caller sorts and
// pages (limits cleanup 2026-09-29).
func readBrowserFacts(ctx context.Context, kernel types.Kernel, sessionID string) ([]types.Fact, error) {
	predicates := make([]string, 0, len(browserPredicateSpecs))
	for predicate := range browserPredicateSpecs {
		predicates = append(predicates, predicate)
	}
	sort.Strings(predicates)
	var result []types.Fact
	for _, predicate := range predicates {
		facts, err := queryScopedBrowserFacts(ctx, kernel, predicate, predicate, sessionID)
		if err != nil {
			return nil, err
		}
		result = append(result, facts...)
	}
	return result, nil
}

func conditionsFromArgs(operation string, args map[string]any) ([]browserFactCondition, error) {
	if operation == "await_fact" {
		predicate := strings.TrimSpace(stringArg(args, "predicate"))
		if predicate == "" {
			return nil, fmt.Errorf("predicate is required")
		}
		return []browserFactCondition{{Predicate: predicate, MatchArgs: stringSliceArg(args["match_args"])}}, nil
	}
	data, err := json.Marshal(args["conditions"])
	if err != nil {
		return nil, fmt.Errorf("encode conditions: %w", err)
	}
	var conditions []browserFactCondition
	if err := json.Unmarshal(data, &conditions); err != nil {
		return nil, fmt.Errorf("conditions must be objects: %w", err)
	}
	if len(conditions) == 0 || len(conditions) > maxBrowserConditions {
		return nil, fmt.Errorf("conditions must contain 1..%d entries", maxBrowserConditions)
	}
	return conditions, nil
}

func waitForBrowserConditions(ctx context.Context, kernel types.Kernel, sessionID string, conditions []browserFactCondition, freshOnly bool, sinceMS int64, timeout, poll time.Duration) (map[string]any, error) {
	start := time.Now()
	watermark := sinceMS
	if watermark <= 0 || watermark > start.UnixMilli() {
		watermark = start.UnixMilli()
	}
	for _, condition := range conditions {
		spec, ok := browserPredicateSpecs[condition.Predicate]
		if !ok {
			return nil, fmt.Errorf("predicate %q is not exposed", condition.Predicate)
		}
		if freshOnly && spec.timestampIndex < 0 {
			return nil, fmt.Errorf("predicate %q has no freshness timestamp; set fresh_only=false explicitly", condition.Predicate)
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for {
		matches := make([]types.Fact, 0, len(conditions))
		allMatched := true
		for _, condition := range conditions {
			facts, err := queryScopedBrowserFacts(ctx, kernel, condition.Predicate, condition.Predicate, sessionID)
			if err != nil {
				return nil, err
			}
			spec := browserPredicateSpecs[condition.Predicate]
			matched := firstMatchingBrowserFact(facts, spec, condition.MatchArgs, freshOnly, watermark)
			if matched == nil {
				allMatched = false
				break
			}
			matches = append(matches, *matched)
		}
		if allMatched {
			return map[string]any{
				"success": true, "status": "matched", "matched": true, "session_id": sessionID,
				"fresh_only": freshOnly, "watermark_ms": watermark, "waited_ms": time.Since(start).Milliseconds(),
				"facts":            publicBrowserFacts(getBrowserManager(), matches, true),
				"evidence_handles": []string{fmt.Sprintf("browser:%s:wait:conditions:%d", sessionID, watermark)},
			}, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return map[string]any{
				"success": false, "status": "timeout", "matched": false, "session_id": sessionID,
				"fresh_only": freshOnly, "watermark_ms": watermark, "waited_ms": time.Since(start).Milliseconds(),
				"evidence_handles": []string{fmt.Sprintf("browser:%s:wait:conditions:%d", sessionID, watermark)},
			}, nil
		case <-ticker.C:
		}
	}
}

func waitForStableBrowser(ctx context.Context, kernel types.Kernel, sessionID string, sinceMS int64, timeout, poll, networkIdle, domIdle time.Duration) (map[string]any, error) {
	start := time.Now()
	watermark := sinceMS
	if watermark <= 0 || watermark > start.UnixMilli() {
		watermark = start.UnixMilli()
	}
	minimumQuiet := networkIdle
	if domIdle > minimumQuiet {
		minimumQuiet = domIdle
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for {
		now := time.Now()
		requests, err := queryScopedBrowserFacts(ctx, kernel, "net_request", "net_request", sessionID)
		if err != nil {
			return nil, err
		}
		responses, err := queryScopedBrowserFacts(ctx, kernel, "net_response", "net_response", sessionID)
		if err != nil {
			return nil, err
		}
		failures, err := queryScopedBrowserFacts(ctx, kernel, "net_failure", "net_failure", sessionID)
		if err != nil {
			return nil, err
		}
		domUpdates, err := queryScopedBrowserFacts(ctx, kernel, "dom_updated", "dom_updated", sessionID)
		if err != nil {
			return nil, err
		}
		recentRequests := filterFactsByTime(requests, browserPredicateSpecs["net_request"], now.Add(-networkIdle).UnixMilli(), now.UnixMilli())
		recentDOM := filterFactsByTime(domUpdates, browserPredicateSpecs["dom_updated"], now.Add(-domIdle).UnixMilli(), now.UnixMilli())
		active := activeRequestCount(requests, watermark, responses, failures)
		if time.Since(start) >= minimumQuiet && len(recentRequests) == 0 && len(recentDOM) == 0 && active == 0 {
			return map[string]any{
				"success": true, "status": "stable", "session_id": sessionID,
				"duration_ms": time.Since(start).Milliseconds(), "network_idle_ms": networkIdle.Milliseconds(), "dom_idle_ms": domIdle.Milliseconds(),
				"evidence_handles": []string{fmt.Sprintf("browser:%s:wait:stable:%d", sessionID, watermark)},
			}, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return map[string]any{
				"success": false, "status": "timeout", "session_id": sessionID,
				"duration_ms": time.Since(start).Milliseconds(), "active_requests": active,
				"evidence_handles": []string{fmt.Sprintf("browser:%s:wait:stable:%d", sessionID, watermark)},
			}, nil
		case <-ticker.C:
		}
	}
}

func firstMatchingBrowserFact(facts []types.Fact, spec browserPredicateSpec, expected []string, freshOnly bool, watermark int64) *types.Fact {
	for i := len(facts) - 1; i >= 0; i-- {
		fact := facts[i]
		if freshOnly && factTimestamp(fact, spec) < watermark {
			continue
		}
		matches := true
		for argIndex, value := range expected {
			if value == "_" {
				continue
			}
			factIndex := argIndex + 1 // session_id is enforced separately
			if factIndex >= len(fact.Args) || fmt.Sprint(fact.Args[factIndex]) != value {
				matches = false
				break
			}
		}
		if matches {
			copy := types.Fact{Predicate: fact.Predicate, Args: append([]any(nil), fact.Args...)}
			return &copy
		}
	}
	return nil
}

func activeRequestCount(requests []types.Fact, watermark int64, completionGroups ...[]types.Fact) int {
	completed := make(map[string]struct{})
	for _, group := range completionGroups {
		for _, fact := range group {
			if len(fact.Args) > 1 {
				completed[fmt.Sprint(fact.Args[1])] = struct{}{}
			}
		}
	}
	active := 0
	for _, fact := range requests {
		if len(fact.Args) <= 1 || factTimestamp(fact, browserPredicateSpecs["net_request"]) < watermark {
			continue
		}
		if _, ok := completed[fmt.Sprint(fact.Args[1])]; !ok {
			active++
		}
	}
	return active
}

// latestBrowserTimestamp reports the newest timestamp for one predicate,
// or the query error. A scan-guard trip used to collapse to zero here,
// which silently widened the diagnosis window and reported a bogus
// navigation_since_ms of 0; the error now propagates so the caller fails
// loudly instead (limits cleanup 2026-09-29).
func latestBrowserTimestamp(ctx context.Context, kernel types.Kernel, predicate, sessionID string) (int64, error) {
	facts, err := queryScopedBrowserFacts(ctx, kernel, predicate, predicate, sessionID)
	if err != nil {
		return 0, err
	}
	spec := browserPredicateSpecs[predicate]
	latest := int64(0)
	for _, fact := range facts {
		if ts := factTimestamp(fact, spec); ts > latest {
			latest = ts
		}
	}
	return latest, nil
}

func filterFactsByTime(facts []types.Fact, spec browserPredicateSpec, after, before int64) []types.Fact {
	result := make([]types.Fact, 0, len(facts))
	for _, fact := range facts {
		ts := factTimestamp(fact, spec)
		if ts >= after && (before == 0 || ts <= before) {
			result = append(result, fact)
		}
	}
	return result
}

func factTimestamp(fact types.Fact, spec browserPredicateSpec) int64 {
	if spec.timestampIndex < 0 || spec.timestampIndex >= len(fact.Args) {
		return 0
	}
	switch value := fact.Args[spec.timestampIndex].(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	case float32:
		return int64(value)
	default:
		return 0
	}
}

func sortFactsNewest(facts []types.Fact) {
	sort.SliceStable(facts, func(i, j int) bool {
		return factTimestamp(facts[i], browserPredicateSpecs[facts[i].Predicate]) > factTimestamp(facts[j], browserPredicateSpecs[facts[j].Predicate])
	})
}

func publicBrowserFacts(mgr *browser.SessionManager, facts []types.Fact, includeArgs bool) []map[string]any {
	rows := make([]map[string]any, 0, len(facts))
	for _, fact := range facts {
		row := map[string]any{"predicate": fact.Predicate}
		if includeArgs {
			args := make([]any, len(fact.Args))
			for i, arg := range fact.Args {
				switch value := arg.(type) {
				case string:
					args[i] = mgr.SanitizeForEvidence(value)
				case types.MangleAtom:
					args[i] = mgr.SanitizeForEvidence(string(value))
				default:
					args[i] = value
				}
			}
			row["args"] = args
		}
		if spec, ok := browserPredicateSpecs[fact.Predicate]; ok && spec.timestampIndex >= 0 {
			row["timestamp_ms"] = factTimestamp(fact, spec)
		}
		rows = append(rows, row)
	}
	return rows
}

func splitConsoleLevels(facts []types.Fact) (errors, warnings []types.Fact) {
	for _, fact := range facts {
		if len(fact.Args) < 2 {
			continue
		}
		switch strings.ToLower(fmt.Sprint(fact.Args[1])) {
		case "error", "assert":
			errors = append(errors, fact)
		case "warning", "warn":
			warnings = append(warnings, fact)
		}
	}
	return errors, warnings
}

func splitToastLevels(facts []types.Fact) (errors, warnings []types.Fact) {
	for _, fact := range facts {
		if len(fact.Args) < 3 {
			continue
		}
		switch strings.ToLower(fmt.Sprint(fact.Args[2])) {
		case "error", "danger":
			errors = append(errors, fact)
		case "warning", "warn":
			warnings = append(warnings, fact)
		}
	}
	return errors, warnings
}

func correlateBrowserFailures(failed, visible []types.Fact, window time.Duration) []map[string]any {
	result := make([]map[string]any, 0)
	for _, event := range visible {
		if len(event.Args) < 4 {
			continue
		}
		eventTS := factTimestamp(event, browserPredicateSpecs["user_visible_error"])
		bestDelta := window.Milliseconds() + 1
		var best *types.Fact
		for i := range failed {
			delta := int64(math.Abs(float64(eventTS - factTimestamp(failed[i], browserPredicateSpecs["failed_request_at"]))))
			if delta <= window.Milliseconds() && delta < bestDelta {
				bestDelta = delta
				best = &failed[i]
			}
		}
		if best != nil && len(best.Args) >= 4 {
			result = append(result, map[string]any{
				"source": event.Args[1], "message": event.Args[2], "request_id": best.Args[1],
				"url": best.Args[2], "status": best.Args[3], "delta_ms": bestDelta,
			})
		}
	}
	return result
}

// maxAdaptedContainerEvents caps how many browser facts are adapted into
// RuntimeErrorEvents for container correlation. A storm of failures must not
// turn one diagnosis into a huge correlation pass. The most recent events are
// kept.
const maxAdaptedContainerEvents = 32

// adaptRuntimeErrorEvents converts browser facts into RuntimeErrorEvents for
// container log correlation. It builds events from the same slices the
// diagnosis already has — failed (Kind "failed_request") and visibleErrors
// (Kind "console_error") — reusing factTimestamp with the same predicate specs
// correlateBrowserFailures uses so the two correlations agree about when a
// fact happened. Facts with undeterminable timestamps (factTimestamp == 0)
// are skipped rather than producing a zero time that would correlate against
// the epoch. The most recent events are kept up to maxAdaptedContainerEvents.
func adaptRuntimeErrorEvents(failed, visible []types.Fact) []browser.RuntimeErrorEvent {
	type candidate struct {
		ts     int64
		kind   string
		detail string
	}
	candidates := make([]candidate, 0, len(failed)+len(visible))
	for _, f := range failed {
		ts := factTimestamp(f, browserPredicateSpecs[f.Predicate])
		if ts == 0 {
			continue
		}
		detail := ""
		if len(f.Args) >= 4 {
			// failed_request_at(SessionID, ReqID, URL, Status, Timestamp)
			url := fmt.Sprint(f.Args[2])
			status := fmt.Sprint(f.Args[3])
			if url != "" && status != "" {
				detail = fmt.Sprintf("%s status=%s", url, status)
			} else if url != "" {
				detail = url
			} else {
				detail = status
			}
		} else if len(f.Args) >= 3 {
			detail = fmt.Sprint(f.Args[2])
		} else {
			detail = f.Predicate
		}
		candidates = append(candidates, candidate{ts: ts, kind: "failed_request", detail: detail})
	}
	for _, f := range visible {
		ts := factTimestamp(f, browserPredicateSpecs[f.Predicate])
		if ts == 0 {
			continue
		}
		detail := ""
		if len(f.Args) >= 3 {
			// user_visible_error(SessionID, Source, Message, Timestamp)
			msg := fmt.Sprint(f.Args[2])
			src := ""
			if len(f.Args) >= 2 {
				src = fmt.Sprint(f.Args[1])
			}
			if src != "" && msg != "" {
				detail = fmt.Sprintf("%s: %s", src, msg)
			} else if msg != "" {
				detail = msg
			} else {
				detail = src
			}
		} else if len(f.Args) >= 2 {
			detail = fmt.Sprint(f.Args[1])
		} else {
			detail = f.Predicate
		}
		candidates = append(candidates, candidate{ts: ts, kind: "console_error", detail: detail})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ts > candidates[j].ts })
	if len(candidates) > maxAdaptedContainerEvents {
		candidates = candidates[:maxAdaptedContainerEvents]
	}
	events := make([]browser.RuntimeErrorEvent, 0, len(candidates))
	for _, c := range candidates {
		events = append(events, browser.RuntimeErrorEvent{
			Kind:      c.kind,
			Detail:    c.detail,
			Timestamp: time.UnixMilli(c.ts),
		})
	}
	return events
}

func detectBrowserContradictions(failed, toasts []types.Fact) []map[string]any {
	if len(failed) == 0 {
		return nil
	}
	result := make([]map[string]any, 0)
	for _, toast := range toasts {
		if len(toast.Args) >= 3 && strings.EqualFold(fmt.Sprint(toast.Args[2]), "info") {
			text := strings.ToLower(fmt.Sprint(toast.Args[1]))
			if strings.Contains(text, "success") || strings.Contains(text, "saved") || strings.Contains(text, "complete") {
				result = append(result, map[string]any{"type": "success_toast_with_failed_request", "toast": toast.Args[1]})
			}
		}
	}
	return result
}

func browserRecommendations(status string, failed, blocked, slow, networkFailures int) []map[string]any {
	result := make([]map[string]any, 0, 4)
	if failed+networkFailures > 0 {
		result = append(result, map[string]any{"tool": "browser_mangle", "operation": "query", "reason": "inspect the newest failed request and correlated evidence"})
	}
	if blocked > 0 {
		result = append(result, map[string]any{"tool": "browser_observe", "mode": "hidden", "reason": "inspect the blocking dialog or expandable content"})
	}
	if slow > 0 {
		result = append(result, map[string]any{"tool": "browser_wait", "mode": "stable", "reason": "wait for bounded network and DOM quiescence"})
	}
	if status == "ok" {
		result = append(result, map[string]any{"tool": "browser_observe", "mode": "interactive", "reason": "continue with current opaque element refs"})
	}
	return result
}

func mergeReasonChanges(groups ...[]types.Fact) []map[string]any {
	merged := make([]types.Fact, 0)
	for _, group := range groups {
		merged = append(merged, group...)
	}
	sortFactsNewest(merged)
	return publicBrowserFacts(getBrowserManager(), merged, true)
}

// pageBrowserFacts windows facts to [offset, offset+maxItems), newest-first
// as sorted by the caller. It returns the window and the end index for the
// paging hint; every fact stays reachable by paging with offset. The window
// is compared to the remaining length so a huge max_items cannot wrap
// offset+maxItems and panic the slice.
func pageBrowserFacts(facts []types.Fact, offset, maxItems int) ([]types.Fact, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(facts) {
		offset = len(facts)
	}
	end := len(facts)
	if maxItems > 0 && maxItems < end-offset {
		end = offset + maxItems
	}
	return facts[offset:end], end
}

// pageReasonSections windows every []map section to
// [offset, offset+window). The old truncateReasonSections cut each section
// at the window with a bare boolean and no way to reach the rest; now a
// windowed section carries its total and a hint naming the remainder and
// the offset that reaches it. Sections fitting the window are stored whole
// with no marker. One offset applies uniformly: short sections simply
// exhaust first (limits cleanup 2026-09-29).
func pageReasonSections(data map[string]any, offset, window int) map[string]any {
	if offset < 0 {
		offset = 0
	}
	result := make(map[string]any, len(data))
	for key, value := range data {
		rows, ok := value.([]map[string]any)
		if !ok {
			result[key] = value
			continue
		}
		start := offset
		if start > len(rows) {
			start = len(rows)
		}
		end := len(rows)
		// Compare to the remainder so a huge window cannot wrap start+window.
		if window > 0 && window < end-start {
			end = start + window
		}
		result[key] = rows[start:end]
		if end < len(rows) {
			result[key+"_truncated"] = true
			result[key+"_total"] = len(rows)
			result[key+"_hint"] = fmt.Sprintf("%d more rows, page with offset=%d", len(rows)-end, end)
		}
	}
	return result
}

// reasonWindow resolves the per-section paging window. An explicit positive
// max_items is the model's own choice and is honored as given in both
// views; when max_items is absent, compact defaults to the cheap rung and
// full to the default window. The old compact min(max,10) silently
// tightened an explicit request; it is gone (limits cleanup 2026-09-29).
func reasonWindow(args map[string]any, view string, maxItems int) int {
	if value, ok := argInt(args, "max_items"); ok && value > 0 {
		return value
	}
	if view == "compact" {
		return config.ResolvedResearchPolicy().BrowserReasonCompactItems
	}
	return maxItems
}

func normalizeReasonView(args map[string]any) (string, int, int, error) {
	view := strings.ToLower(strings.TrimSpace(stringArg(args, "view")))
	if view == "" {
		view = "compact"
	}
	if view != "summary" && view != "compact" && view != "full" {
		return "", 0, 0, fmt.Errorf("unsupported view %q", view)
	}
	// No hard cap: an explicit max_items is the model's own paging window
	// and is honored as given. Non-positive values select the configured
	// default (research.browser_reason_items).
	reasonDefault := config.ResolvedResearchPolicy().BrowserReasonItems
	maxItems := intArg(args, "max_items", reasonDefault)
	if maxItems <= 0 {
		maxItems = reasonDefault
	}
	offset := intArg(args, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	return view, maxItems, offset, nil
}

func boundedTimeout(args map[string]any) time.Duration {
	value := time.Duration(intArg(args, "timeout_ms", int(defaultBrowserTimeout.Milliseconds()))) * time.Millisecond
	if value <= 0 {
		return defaultBrowserTimeout
	}
	if value > maxBrowserTimeout {
		return maxBrowserTimeout
	}
	return value
}

func boundedPoll(args map[string]any) time.Duration {
	value := time.Duration(intArg(args, "poll_interval_ms", int(defaultBrowserPoll.Milliseconds()))) * time.Millisecond
	if value < minBrowserPoll {
		return minBrowserPoll
	}
	if value > maxBrowserPoll {
		return maxBrowserPoll
	}
	return value
}

func boundedIdle(args map[string]any, key string, fallback int) time.Duration {
	value := time.Duration(intArg(args, key, fallback)) * time.Millisecond
	if value < minBrowserPoll {
		return minBrowserPoll
	}
	if value > 5*time.Second {
		return 5 * time.Second
	}
	return value
}

func int64Arg(args map[string]any, key string, fallback int64) int64 {
	// Canonical 64-bit coercion (tools.CoerceInt64): millisecond timestamps
	// exceed int32 and thread through int64 APIs, so they get their own
	// canonical helper rather than a ninth divergent switch.
	if value, ok := tools.CoerceInt64(args[key]); ok {
		return value
	}
	return fallback
}

func stringSliceArg(value any) []string {
	switch items := value.(type) {
	case []string:
		return append([]string(nil), items...)
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			result = append(result, fmt.Sprint(item))
		}
		return result
	default:
		return nil
	}
}
