package research

// Native declarative tests are adapted from BrowserNERD's Apache-2.0
// browser-test/run-test/generate-test contracts. codeNERD replays through its
// own browser_act route and checks only bounded session-scoped atoms against
// the live Cortex kernel.

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/browser"
	"codenerd/internal/browser/testspec"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

const maxGeneratedBrowserActions = testspec.MaxActions

// BrowserTestTool creates, inspects, generates, and runs portable browser
// action/assertion fixtures without a Python harness.
func BrowserTestTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_test",
		Description: `Create, inspect, generate, or run bounded declarative browser tests. Fixtures use portable semantic element targets, the closed browser_act operation vocabulary, and single-atom present/absent assertions over one live Cortex session. Opaque refs and raw selectors are forbidden. Sensitive fields require value_env, resolved only in an execution copy. Generate reads privacy-safe action_intent flight evidence and returns portable YAML.`,
		Category:    tools.CategoryResearch,
		Priority:    76,
		Execute:     executeBrowserTest,
		Schema: tools.ToolSchema{
			Required: []string{"operation"},
			Properties: map[string]tools.Property{
				"operation":           {Type: "string", Enum: []any{"create", "inspect", "generate", "run"}},
				"session_id":          {Type: "string", Description: "Session override; required for generate and run"},
				"name":                {Type: "string", Description: "Generated fixture name"},
				"test":                {Type: "object", Description: "Inline declarative fixture; alternative to test_yaml"},
				"test_yaml":           {Type: "string", Description: "Portable YAML fixture; alternative to test"},
				"since_ms":            {Type: "integer", Description: "Generate only action intents at or after this timestamp"},
				"include_assertions":  {Type: "boolean", Default: true},
				"stop_on_error":       {Type: "boolean", Default: true},
				"diagnose_on_failure": {Type: "boolean", Default: true},
				"settle_timeout_ms":   {Type: "integer", Default: 5000, Description: "Run quiescence wait, hard-capped at 10000"},
				"view":                {Type: "string", Default: "compact", Enum: []any{"summary", "compact", "full"}},
			},
		},
	}
}

func executeBrowserTest(ctx context.Context, args map[string]any) (string, error) {
	operation := strings.ToLower(strings.TrimSpace(stringArg(args, "operation")))
	view, err := browserTestView(args)
	if err != nil {
		return "", fmt.Errorf("browser test: %w", err)
	}
	switch operation {
	case "create":
		if args["test"] == nil && strings.TrimSpace(stringArg(args, "test_yaml")) == "" {
			return generateBrowserTest(ctx, args, "create", view)
		}
		spec, err := parseBrowserTestArgs(args)
		if err != nil {
			return browserTestInvalid("create", err)
		}
		return browserTestFixtureResult("create", spec, view)
	case "inspect":
		spec, err := parseBrowserTestArgs(args)
		if err != nil {
			return browserTestInvalid("inspect", err)
		}
		return browserTestFixtureResult("inspect", spec, view)
	case "generate":
		return generateBrowserTest(ctx, args, operation, view)
	case "run":
		return runBrowserTest(ctx, args, view)
	default:
		return "", fmt.Errorf("browser test: unsupported operation %q", operation)
	}
}

func parseBrowserTestArgs(args map[string]any) (testspec.Spec, error) {
	if raw := strings.TrimSpace(stringArg(args, "test_yaml")); raw != "" {
		return testspec.ParseYAML(raw)
	}
	if value := args["test"]; value != nil {
		return testspec.ParseValue(value)
	}
	return testspec.Spec{}, fmt.Errorf("provide test or test_yaml")
}

func browserTestFixtureResult(operation string, spec testspec.Spec, view string) (string, error) {
	encoded, err := testspec.MarshalYAML(spec)
	if err != nil {
		return browserTestInvalid(operation, err)
	}
	output := map[string]any{
		"success": true, "status": "valid", "operation": operation, "name": spec.Name,
		"session_id": spec.SessionID, "action_count": len(spec.Actions), "assertion_count": len(spec.Assertions),
		"test_yaml": encoded,
	}
	if view == "full" {
		output["test"] = spec
	}
	if view == "summary" {
		delete(output, "test_yaml")
	}
	return marshalProgressiveResult(output)
}

func generateBrowserTest(ctx context.Context, args map[string]any, operation, view string) (string, error) {
	manager := getBrowserManager()
	sessionID := strings.TrimSpace(stringArg(args, "session_id"))
	if sessionID == "" {
		return "", fmt.Errorf("browser test: session_id is required for generate")
	}
	read, err := manager.ReadEvidence(sessionID, browser.FlightReadOptions{
		SinceMS: int64Arg(args, "since_ms", 0), Types: []string{"action_intent"}, MaxItems: 100,
	})
	if err != nil {
		return "", fmt.Errorf("browser test: read action intent: %w", err)
	}
	if read.Truncated {
		return "", fmt.Errorf("browser test: action intent evidence is truncated; narrow since_ms before generating")
	}
	sort.SliceStable(read.Events, func(i, j int) bool { return read.Events[i].TimestampMS < read.Events[j].TimestampMS })
	actions := make([]browser.ActionOperation, 0, len(read.Events))
	for _, event := range read.Events {
		operationValue, decodeErr := decodeRecordedBrowserOperation(event.Data)
		if decodeErr != nil {
			return "", fmt.Errorf("browser test: decode action intent: %w", decodeErr)
		}
		actions = append(actions, operationValue)
	}
	if len(actions) == 0 {
		return "", fmt.Errorf("browser test: no portable action intent evidence found")
	}
	if len(actions) > maxGeneratedBrowserActions {
		return "", fmt.Errorf("browser test: %d recorded actions exceed fixture limit of %d; narrow since_ms", len(actions), maxGeneratedBrowserActions)
	}
	name := strings.TrimSpace(stringArg(args, "name"))
	if name == "" {
		name = "recorded browser test"
	}
	spec := testspec.Spec{Name: name, SessionID: sessionID, Actions: actions}
	if boolArg(args, "include_assertions", true) {
		spec.Assertions = []testspec.Assertion{
			{Name: "no new visible errors", Query: "user_visible_error(S, Kind, Message, Timestamp)", Expect: "absent", Scope: "fresh"},
			{Name: "no new failed requests", Query: "failed_request_at(S, Request, URL, Status, Timestamp)", Expect: "absent", Scope: "fresh"},
		}
	} else {
		spec.Assertions = []testspec.Assertion{
			{Name: "replace with expected outcome", Query: "browser_page_state(S, URL, Loading, Dialog, Timestamp)", Expect: "present", Scope: "fresh"},
		}
	}
	if err := testspec.Normalize(&spec); err != nil {
		return browserTestInvalid(operation, err)
	}
	encoded, err := testspec.MarshalYAML(spec)
	if err != nil {
		return "", err
	}
	output := map[string]any{
		"success": true, "status": "draft", "operation": operation, "name": spec.Name,
		"session_id": sessionID, "action_count": len(spec.Actions), "assertion_count": len(spec.Assertions),
		"test_yaml": encoded, "note": "draft: edit assertions to describe success before running",
		"files_read": read.FilesRead, "scanned_bytes": read.ScannedBytes, "truncated": false,
	}
	if view == "full" {
		output["test"] = spec
	}
	if view == "summary" {
		delete(output, "test_yaml")
	}
	recordBrowserToolEvidence(sessionID, "test", map[string]any{
		"operation": operation, "status": "draft", "action_count": len(spec.Actions), "assertion_count": len(spec.Assertions),
	})
	return marshalProgressiveResult(output)
}

func decodeRecordedBrowserOperation(value any) (browser.ActionOperation, error) {
	var envelope struct {
		Operation browser.ActionOperation `json:"operation"`
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return envelope.Operation, err
	}
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		return envelope.Operation, err
	}
	if strings.TrimSpace(envelope.Operation.Type) == "" {
		return envelope.Operation, fmt.Errorf("missing operation")
	}
	return envelope.Operation, nil
}

func runBrowserTest(ctx context.Context, args map[string]any, view string) (string, error) {
	spec, err := parseBrowserTestArgs(args)
	if err != nil {
		return browserTestInvalid("run", err)
	}
	manager := getBrowserManager()
	kernel := getBrowserKernel()
	if kernel == nil {
		return "", fmt.Errorf("browser test: live Cortex kernel is not bound")
	}
	sessionID := strings.TrimSpace(stringArg(args, "session_id"))
	if sessionID == "" {
		sessionID = spec.SessionID
	}
	if sessionID == "" {
		return browserTestInvalid("run", fmt.Errorf("session_id is required"))
	}
	if _, ok := manager.GetSession(sessionID); !ok {
		return "", fmt.Errorf("browser test: unknown session %s", sessionID)
	}
	if len(spec.Actions) > 0 {
		if err := manager.FocusSession(ctx, sessionID); err != nil {
			return "", fmt.Errorf("browser test: focus replay session: %w", err)
		}
	}
	resolved, err := testspec.ResolveEnvironment(spec)
	if err != nil {
		return browserTestInvalid("run", err)
	}
	if _, err := manager.Observe(ctx, sessionID, browser.ObserveOptions{Mode: "state", View: "summary", MaxItems: 1}); err != nil {
		return "", fmt.Errorf("browser test: refresh session state: %w", err)
	}

	type assertionPlan struct {
		predicate string
		baseline  []types.Fact
	}
	plans := make([]assertionPlan, len(resolved.Assertions))
	for index, assertion := range resolved.Assertions {
		predicate, queryErr := validateBrowserQuery(assertion.Query)
		if queryErr != nil {
			return browserTestInvalid("run", fmt.Errorf("assertions[%d]: %w", index, queryErr))
		}
		plans[index].predicate = predicate
		if assertion.Scope == "fresh" {
			plans[index].baseline, queryErr = queryScopedBrowserFacts(ctx, kernel, assertion.Query, predicate, sessionID)
			if queryErr != nil {
				return "", fmt.Errorf("browser test: establish assertion baseline: %w", queryErr)
			}
		}
	}

	startedMS := time.Now().UnixMilli()
	replayOK := true
	var replay map[string]any
	var settle map[string]any
	if len(resolved.Actions) > 0 {
		operations, err := browserTestOperationsValue(resolved.Actions)
		if err != nil {
			return "", err
		}
		replayRaw, replayErr := executeBrowserAct(ctx, map[string]any{
			"session_id": sessionID, "operations": operations,
			"stop_on_error": boolArg(args, "stop_on_error", true), "view": "full", "max_items": testspec.MaxActions,
		})
		if replayErr != nil {
			replayOK = false
			replay = map[string]any{"success": false, "error": manager.SanitizeForEvidence(replayErr.Error())}
		} else if err := json.Unmarshal([]byte(replayRaw), &replay); err != nil {
			return "", fmt.Errorf("browser test: decode replay: %w", err)
		} else {
			replayOK, _ = replay["success"].(bool)
			if value, ok := replay["started_ms"].(float64); ok {
				startedMS = int64(value)
			}
		}
		if replayOK {
			settleTimeout := time.Duration(intArg(args, "settle_timeout_ms", 5000)) * time.Millisecond
			if settleTimeout <= 0 {
				settleTimeout = 5 * time.Second
			}
			if settleTimeout > 10*time.Second {
				settleTimeout = 10 * time.Second
			}
			settle, err = waitForStableBrowser(ctx, kernel, sessionID, startedMS, settleTimeout, 50*time.Millisecond, 150*time.Millisecond, 150*time.Millisecond)
			if err != nil {
				return "", fmt.Errorf("browser test: settle replay: %w", err)
			}
			if status, _ := settle["status"].(string); status != "stable" {
				replayOK = false
			}
		}
	}

	assertionResults := make([]map[string]any, 0, len(resolved.Assertions))
	violations := 0
	for index, assertion := range resolved.Assertions {
		facts, queryErr := queryScopedBrowserFacts(ctx, kernel, assertion.Query, plans[index].predicate, sessionID)
		row := map[string]any{
			"name": assertion.Name, "query": manager.SanitizeForEvidence(assertion.Query),
			"expect": assertion.Expect, "scope": assertion.Scope,
		}
		if queryErr != nil {
			row["passed"] = false
			row["error"] = manager.SanitizeForEvidence(queryErr.Error())
			violations++
			assertionResults = append(assertionResults, row)
			continue
		}
		if assertion.Scope == "fresh" {
			facts = subtractBrowserFacts(facts, plans[index].baseline)
		}
		matched := len(facts)
		passed := matched > 0
		if assertion.Expect == "absent" {
			passed = matched == 0
		}
		row["matched"] = matched
		row["passed"] = passed
		if !passed {
			violations++
		}
		if view == "full" && matched > 0 {
			sample := facts
			if len(sample) > 3 {
				sample = sample[:3]
			}
			row["sample"] = publicBrowserFacts(manager, sample, true)
		}
		assertionResults = append(assertionResults, row)
	}

	passed := replayOK && violations == 0
	status := "passed"
	if !passed {
		status = "failed"
	}
	output := map[string]any{
		"success": passed, "passed": passed, "status": status, "operation": "run",
		"name": spec.Name, "session_id": sessionID, "started_ms": startedMS, "finished_ms": time.Now().UnixMilli(),
		"action_count": len(resolved.Actions), "assertion_count": len(resolved.Assertions), "violation_count": violations,
		"assertions": assertionResults, "evidence_handles": []string{fmt.Sprintf("browser:%s:test:%d", sessionID, startedMS)},
	}
	if replay != nil {
		output["replay"] = replay
	}
	if settle != nil {
		output["settle"] = settle
	}
	if !passed && boolArg(args, "diagnose_on_failure", true) {
		diagnosisRaw, diagnoseErr := executeBrowserReason(ctx, map[string]any{
			"session_id": sessionID, "topic": "why_failed", "view": "compact", "max_items": 10,
		})
		if diagnoseErr == nil {
			var diagnosis map[string]any
			if json.Unmarshal([]byte(diagnosisRaw), &diagnosis) == nil {
				output["diagnosis"] = diagnosis
			}
		}
	}
	if view == "summary" {
		delete(output, "assertions")
		delete(output, "replay")
		delete(output, "diagnosis")
	} else if view == "compact" {
		delete(output, "replay")
	}
	recordBrowserToolEvidence(sessionID, "test", map[string]any{
		"operation": "run", "status": status, "passed": passed,
		"action_count": len(resolved.Actions), "assertion_count": len(resolved.Assertions), "violation_count": violations,
	})
	return marshalProgressiveResult(output)
}

func browserTestOperationsValue(operations []browser.ActionOperation) ([]any, error) {
	encoded, err := json.Marshal(operations)
	if err != nil {
		return nil, fmt.Errorf("browser test: encode replay operations: %w", err)
	}
	var result []any
	if err := json.Unmarshal(encoded, &result); err != nil {
		return nil, fmt.Errorf("browser test: decode replay operations: %w", err)
	}
	return result, nil
}

func subtractBrowserFacts(facts, baseline []types.Fact) []types.Fact {
	counts := make(map[string]int, len(baseline))
	for _, fact := range baseline {
		counts[browserFactFingerprint(fact)]++
	}
	fresh := make([]types.Fact, 0, len(facts))
	for _, fact := range facts {
		key := browserFactFingerprint(fact)
		if counts[key] > 0 {
			counts[key]--
			continue
		}
		fresh = append(fresh, fact)
	}
	return fresh
}

func browserFactFingerprint(fact types.Fact) string {
	encoded, err := json.Marshal(struct {
		Predicate string `json:"predicate"`
		Args      []any  `json:"args"`
	}{Predicate: fact.Predicate, Args: fact.Args})
	if err != nil {
		return fmt.Sprintf("%s:%v", fact.Predicate, fact.Args)
	}
	return string(encoded)
}

func browserTestView(args map[string]any) (string, error) {
	view := strings.ToLower(strings.TrimSpace(stringArg(args, "view")))
	if view == "" {
		view = "compact"
	}
	if view != "summary" && view != "compact" && view != "full" {
		return "", fmt.Errorf("view must be summary, compact, or full")
	}
	return view, nil
}

func browserTestInvalid(operation string, err error) (string, error) {
	message := "invalid browser test"
	if err != nil {
		message = getBrowserManager().SanitizeForEvidence(err.Error())
	}
	return marshalProgressiveResult(map[string]any{
		"success": false, "passed": false, "status": "invalid", "operation": operation, "error": message,
	})
}
