package research

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/browser"
	"codenerd/internal/tools"
)

// BrowserObserveTool returns the bounded, ref-producing observation surface.
func BrowserObserveTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_observe",
		Description: `Observe a browser session with progressive disclosure. Modes: state, nav, interactive, grids, hidden, composite, sessions, screenshot, react, dom_snapshot. Use compact or summary first. Interactive outputs contain opaque refs for browser_act; refs become stale after navigation.`,
		Category:    tools.CategoryResearch,
		Priority:    70,
		Execute:     executeBrowserObserve,
		Schema: tools.ToolSchema{
			Properties: map[string]tools.Property{
				"session_id":    {Type: "string", Description: "Target session; optional only for sessions mode"},
				"mode":          {Type: "string", Description: "Observation slice", Default: "composite", Enum: []any{"state", "nav", "interactive", "grids", "hidden", "composite", "sessions", "screenshot", "react", "dom_snapshot"}},
				"view":          {Type: "string", Description: "Disclosure depth", Default: "compact", Enum: []any{"summary", "compact", "full"}},
				"max_items":     {Type: "integer", Description: "Maximum items per list; hard-capped at 100", Default: 20},
				"filter":        {Type: "string", Description: "Interactive filter", Default: "all", Enum: []any{"all", "buttons", "inputs", "links", "selects"}},
				"visible_only":  {Type: "boolean", Description: "Return only visible interactive elements", Default: true},
				"internal_only": {Type: "boolean", Description: "For nav mode, omit external origins", Default: false},
				"full_page":     {Type: "boolean", Description: "For screenshot mode, capture the full page", Default: false},
				"save_path":     {Type: "string", Description: "For screenshot mode, optional path under configured writable roots"},
				"include_specs": {Type: "boolean", Description: "Attach bounded route/term-matched workspace spec context", Default: false},
				"spec_terms":    {Type: "array", Description: "Optional bounded spec relevance terms", Items: &tools.PropertyItems{Type: "string"}},
			},
		},
	}
}

func executeBrowserObserve(ctx context.Context, args map[string]any) (string, error) {
	visibleOnly := true
	if value, ok := args["visible_only"].(bool); ok {
		visibleOnly = value
	}
	observation, err := getBrowserManager().Observe(ctx, stringArg(args, "session_id"), browser.ObserveOptions{
		Mode: stringArg(args, "mode"), View: stringArg(args, "view"), MaxItems: intArg(args, "max_items", 20),
		Filter: stringArg(args, "filter"), VisibleOnly: visibleOnly, InternalOnly: boolArg(args, "internal_only", false),
		FullPage: boolArg(args, "full_page", false), SavePath: stringArg(args, "save_path"),
	})
	if err != nil {
		return "", fmt.Errorf("browser observe: %w", err)
	}
	if boolArg(args, "include_specs", false) && observation.SessionID != "" {
		matches, matchErr := browserSpecContext(ctx, getBrowserManager(), observation.SessionID, stringSliceArg(args["spec_terms"]))
		if matchErr != nil {
			observation.Data["spec_context_error"] = matchErr.Error()
		} else {
			observation.Data["spec_context"] = matches
		}
	}
	recordBrowserToolEvidence(observation.SessionID, "observe", map[string]any{
		"mode": observation.Mode, "view": observation.View, "summary": observation.Summary,
		"generation": observation.Generation, "truncated": observation.Truncated,
		"evidence_handles": observation.EvidenceHandles,
	})
	return marshalProgressiveResult(observation)
}

// BrowserActTool returns the closed, sequential browser action surface.
func BrowserActTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_act",
		Description: `Execute up to 25 browser operations in sequence. Interactive calls normally consume fresh opaque refs from browser_observe; declarative replay may instead use a bounded semantic target that resolves uniquely to a fresh ref. Operation types: navigate{url}; interact{ref|target,action,value,submit} where action is click|type|select|toggle|clear; fill{fields:[{ref|target,value}],submit,submit_button|submit_target}; key{key}; history{action:back|forward|reload}; sleep{duration_ms}; session_create/session_attach/session_fork/session_focus/session_close; browser_launch/browser_close. Stops on the first failure by default. Arbitrary JavaScript and fact waits are intentionally unavailable here.`,
		Category:    tools.CategoryResearch,
		Priority:    65,
		Execute:     executeBrowserAct,
		Schema: tools.ToolSchema{
			Required: []string{"operations"},
			Properties: map[string]tools.Property{
				"session_id":    {Type: "string", Description: "Default target session for operations"},
				"operations":    {Type: "array", Description: "Ordered operation objects; each requires type", Items: &tools.PropertyItems{Type: "object"}},
				"stop_on_error": {Type: "boolean", Description: "Stop after the first failed operation", Default: true},
				"view":          {Type: "string", Description: "Result disclosure depth", Default: "compact", Enum: []any{"summary", "compact", "full"}},
				"max_items":     {Type: "integer", Description: "Maximum per-operation results returned", Default: 20},
				"include_specs": {Type: "boolean", Description: "Attach bounded route/term-matched workspace spec context after the plan", Default: false},
				"spec_terms":    {Type: "array", Description: "Optional bounded spec relevance terms", Items: &tools.PropertyItems{Type: "string"}},
			},
		},
	}
}

func executeBrowserAct(ctx context.Context, args map[string]any) (string, error) {
	operations, err := decodeActionOperations(args["operations"])
	if err != nil {
		return "", err
	}
	stopOnError := boolArg(args, "stop_on_error", true)
	execution, err := getBrowserManager().ExecuteActions(ctx, stringArg(args, "session_id"), operations, stopOnError)
	if err != nil {
		return "", fmt.Errorf("browser act: %w", err)
	}

	view := strings.ToLower(strings.TrimSpace(stringArg(args, "view")))
	if view == "" {
		view = "compact"
	}
	if view != "summary" && view != "compact" && view != "full" {
		return "", fmt.Errorf("browser act: unsupported view %q", view)
	}
	maxItems := intArg(args, "max_items", 20)
	if maxItems <= 0 {
		maxItems = 20
	}
	if maxItems > 25 {
		maxItems = 25
	}

	results := execution.Results
	truncated := len(results) > maxItems
	if truncated {
		results = results[:maxItems]
	}
	if view == "summary" {
		results = nil
	} else if view == "compact" {
		compact := make([]browser.ActionStepResult, len(results))
		for i := range results {
			compact[i] = results[i]
			compact[i].Result = nil
		}
		results = compact
	}
	output := map[string]any{
		"success": execution.Success, "status": execution.Status, "session_id": execution.SessionID,
		"started_ms": execution.StartedMS, "finished_ms": execution.FinishedMS,
		"summary": execution.Summary, "counts": execution.Counts, "view": view,
		"evidence_handles": execution.EvidenceHandles, "truncated": truncated,
	}
	if results != nil {
		output["results"] = results
	}
	if boolArg(args, "include_specs", false) && execution.SessionID != "" {
		matches, matchErr := browserSpecContext(ctx, getBrowserManager(), execution.SessionID, stringSliceArg(args["spec_terms"]))
		if matchErr != nil {
			output["spec_context_error"] = matchErr.Error()
		} else {
			output["spec_context"] = matches
		}
	}
	recordBrowserToolEvidence(execution.SessionID, "act", map[string]any{
		"success": execution.Success, "status": execution.Status, "started_ms": execution.StartedMS,
		"finished_ms": execution.FinishedMS, "summary": execution.Summary, "counts": execution.Counts,
		"results": execution.Results, "evidence_handles": execution.EvidenceHandles,
	})
	return marshalProgressiveResult(output)
}

func decodeActionOperations(value any) ([]browser.ActionOperation, error) {
	if value == nil {
		return nil, fmt.Errorf("operations must be a non-empty array")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode operations: %w", err)
	}
	var operations []browser.ActionOperation
	if err := json.Unmarshal(data, &operations); err != nil {
		return nil, fmt.Errorf("operations must be an array of objects: %w", err)
	}
	if len(operations) == 0 {
		return nil, fmt.Errorf("operations must be a non-empty array")
	}
	return operations, nil
}

func marshalProgressiveResult(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal browser result: %w", err)
	}
	return string(data), nil
}

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return value
}

func boolArg(args map[string]any, key string, fallback bool) bool {
	value, ok := args[key].(bool)
	if !ok {
		return fallback
	}
	return value
}

func intArg(args map[string]any, key string, fallback int) int {
	switch value := args[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, err := value.Int64()
		if err == nil {
			return int(parsed)
		}
	}
	return fallback
}
