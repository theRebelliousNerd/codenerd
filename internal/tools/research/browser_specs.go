package research

// Native spec delivery is adapted from BrowserNERD's Apache-2.0 configurable
// spec tools. codeNERD restricts every corpus to the active workspace and
// evaluates only bounded browser atoms against the live Cortex kernel.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"codenerd/internal/browser"
	browserspec "codenerd/internal/browser/specs"
	"codenerd/internal/tools"
)

const maxBrowserSpecChecks = 100

// BrowserSpecsTool delivers and checks bounded workspace browser specs.
func BrowserSpecsTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_specs",
		Description: `Discover and check bounded browser specifications from named Markdown corpora confined to the active workspace. Operations: list, get, check. get ranks compact excerpts by file, route, component, selector, and terms. check evaluates only declared single-atom browser invariants against one session in the live Cortex kernel; it cannot submit rules or mutate facts.`,
		Category:    tools.CategoryResearch,
		Priority:    75,
		Execute:     executeBrowserSpecs,
		Schema: tools.ToolSchema{
			Required: []string{"operation"},
			Properties: map[string]tools.Property{
				"operation":           {Type: "string", Enum: []any{"list", "get", "check"}},
				"session_id":          {Type: "string", Description: "Required for check; get may infer the current route from it"},
				"corpus":              {Type: "string", Description: "Optional configured source name"},
				"file":                {Type: "string", Description: "Optional governed source file"},
				"from":                {Type: "integer", Description: "Optional inclusive source line start"},
				"to":                  {Type: "integer", Description: "Optional inclusive source line end"},
				"line":                {Type: "integer", Description: "Single-line shorthand for from/to"},
				"component":           {Type: "string"},
				"route":               {Type: "string"},
				"selector":            {Type: "string"},
				"terms":               {Type: "array", Description: "Up to 20 bounded relevance terms", Items: &tools.PropertyItems{Type: "string"}},
				"view":                {Type: "string", Default: "compact", Enum: []any{"summary", "compact", "full"}},
				"max_items":           {Type: "integer", Default: 20, Description: "Capped by configured max_results and hard-capped at 50"},
				"max_checks":          {Type: "integer", Default: 50, Description: "Hard-capped at 100"},
				"diagnose_on_failure": {Type: "boolean", Default: true},
			},
		},
	}
}

func executeBrowserSpecs(ctx context.Context, args map[string]any) (string, error) {
	manager := getBrowserManager()
	config := manager.SpecsConfig()
	operation := strings.ToLower(strings.TrimSpace(stringArg(args, "operation")))
	if operation != "list" && operation != "get" && operation != "check" {
		return "", fmt.Errorf("browser specs: unsupported operation %q", operation)
	}
	view, err := browserSpecView(args)
	if err != nil {
		return "", fmt.Errorf("browser specs: %w", err)
	}
	input, err := browserSpecInput(manager, args)
	if err != nil {
		return "", fmt.Errorf("browser specs: %w", err)
	}
	loaded, err := manager.LoadSpecs(ctx)
	if err != nil {
		return "", fmt.Errorf("browser specs: %w", err)
	}
	maxItems := intArg(args, "max_items", config.MaxResults)
	if maxItems <= 0 {
		maxItems = config.MaxResults
	}
	if maxItems > config.MaxResults {
		maxItems = config.MaxResults
	}
	input.Max = maxItems
	base := map[string]any{
		"success": true, "operation": operation, "specs_loaded": len(loaded.Specs),
		"files_scanned": loaded.FilesScanned, "entries_scanned": loaded.EntriesScanned, "bytes_loaded": loaded.BytesLoaded,
		"warnings": loaded.Warnings, "catalog_truncated": loaded.Truncated,
	}

	switch operation {
	case "list":
		listInput := browserspec.MatchInput{Corpus: input.Corpus, Max: maxItems}
		documents := browserspec.MatchSpecs(loaded.Specs, listInput, config.MaxExcerptBytes)
		sanitizeBrowserSpecMatches(manager, documents)
		matched := browserspec.CountMatchingSpecs(loaded.Specs, listInput)
		for index := range documents {
			documents[index].Excerpt = ""
		}
		base["documents"] = documents
		base["count"] = len(documents)
		base["truncated"] = loaded.Truncated || len(documents) < matched
	case "get":
		documents := browserspec.MatchSpecs(loaded.Specs, input, config.MaxExcerptBytes)
		sanitizeBrowserSpecMatches(manager, documents)
		matched := browserspec.CountMatchingSpecs(loaded.Specs, input)
		invariants, invariantTruncated := browserspec.SelectInvariants(loaded.Specs, input, maxItems)
		base["documents"] = documents
		base["invariants"] = publicBrowserSpecInvariants(manager, invariants, view, config.MaxExcerptBytes/2)
		base["count"] = len(documents)
		base["truncated"] = loaded.Truncated || invariantTruncated || len(documents) < matched
		if view == "summary" {
			delete(base, "documents")
			delete(base, "invariants")
		} else if view == "compact" {
			for index := range documents {
				documents[index].Excerpt = ""
			}
			base["documents"] = documents
		}
	case "check":
		return executeBrowserSpecCheck(ctx, manager, loaded, input, view, args, base)
	}
	if sessionID := strings.TrimSpace(stringArg(args, "session_id")); sessionID != "" {
		recordBrowserToolEvidence(sessionID, "specs", map[string]any{
			"operation": operation, "specs_loaded": len(loaded.Specs), "count": base["count"],
			"catalog_truncated": loaded.Truncated,
		})
	}
	return marshalProgressiveResult(base)
}

func executeBrowserSpecCheck(ctx context.Context, manager *browser.SessionManager, loaded browserspec.LoadResult, input browserspec.MatchInput, view string, args, output map[string]any) (string, error) {
	kernel := getBrowserKernel()
	if kernel == nil {
		return "", fmt.Errorf("browser specs: live Cortex kernel is not bound")
	}
	sessionID := strings.TrimSpace(stringArg(args, "session_id"))
	if sessionID == "" {
		return "", fmt.Errorf("browser specs: session_id is required for check")
	}
	if _, ok := manager.GetSession(sessionID); !ok {
		return "", fmt.Errorf("browser specs: unknown session %s", sessionID)
	}
	if _, err := manager.Observe(ctx, sessionID, browser.ObserveOptions{Mode: "state", View: "summary", MaxItems: 1}); err != nil {
		return "", fmt.Errorf("browser specs: refresh session state: %w", err)
	}
	maxChecks := intArg(args, "max_checks", 50)
	if maxChecks <= 0 {
		maxChecks = 50
	}
	if maxChecks > maxBrowserSpecChecks {
		maxChecks = maxBrowserSpecChecks
	}
	selected, selectionTruncated := browserspec.SelectInvariants(loaded.Specs, input, maxChecks)
	results := make([]map[string]any, 0, len(selected))
	violations := make([]map[string]any, 0)
	checked, skipped := 0, 0
	for _, selectedInvariant := range selected {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if strings.TrimSpace(selectedInvariant.Query) == "" {
			skipped++
			continue
		}
		checked++
		row := map[string]any{
			"spec": selectedInvariant.Spec, "path": selectedInvariant.Path, "name": selectedInvariant.Name,
			"expect": selectedInvariant.Expect, "query": manager.SanitizeForEvidence(selectedInvariant.Query),
		}
		if selectedInvariant.Expect != "present" && selectedInvariant.Expect != "absent" {
			row["passed"] = false
			row["error"] = fmt.Sprintf("unsupported expect %q", selectedInvariant.Expect)
			violations = append(violations, row)
			results = append(results, row)
			continue
		}
		predicate, queryErr := validateBrowserQuery(selectedInvariant.Query)
		if queryErr != nil {
			row["passed"] = false
			row["error"] = queryErr.Error()
			violations = append(violations, row)
			results = append(results, row)
			continue
		}
		facts, queryErr := queryScopedBrowserFacts(ctx, kernel, selectedInvariant.Query, predicate, sessionID)
		if queryErr != nil {
			row["passed"] = false
			row["error"] = queryErr.Error()
			violations = append(violations, row)
			results = append(results, row)
			continue
		}
		matched := len(facts) > 0
		passed := matched
		if selectedInvariant.Expect == "absent" {
			passed = !matched
		}
		row["passed"] = passed
		row["matches"] = len(facts)
		if !passed {
			violations = append(violations, row)
		}
		results = append(results, row)
	}
	incomplete := loaded.Truncated || selectionTruncated || len(loaded.Warnings) > 0
	passed := checked > 0 && len(violations) == 0 && !incomplete
	status := "passed"
	if len(violations) > 0 {
		status = "failed"
	} else if checked == 0 {
		status = "no_checks"
	} else if incomplete {
		status = "incomplete"
	}
	output["session_id"] = sessionID
	output["checked"] = checked
	output["skipped"] = skipped
	output["passed"] = passed
	output["status"] = status
	output["violations"] = violations
	output["truncated"] = loaded.Truncated || selectionTruncated
	if view == "full" {
		output["results"] = results
	}
	if view == "summary" {
		delete(output, "violations")
	}
	if len(violations) > 0 && boolArg(args, "diagnose_on_failure", true) {
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
	recordBrowserToolEvidence(sessionID, "specs", map[string]any{
		"operation": "check", "checked": checked, "skipped": skipped,
		"passed": passed, "violations": len(violations), "truncated": output["truncated"],
	})
	return marshalProgressiveResult(output)
}

func browserSpecInput(manager *browser.SessionManager, args map[string]any) (browserspec.MatchInput, error) {
	input := browserspec.MatchInput{
		Corpus: strings.TrimSpace(stringArg(args, "corpus")), File: strings.TrimSpace(stringArg(args, "file")),
		From: intArg(args, "from", 0), To: intArg(args, "to", 0),
		Component: strings.TrimSpace(stringArg(args, "component")), Route: strings.TrimSpace(stringArg(args, "route")),
		Selector: strings.TrimSpace(stringArg(args, "selector")), Terms: stringSliceArg(args["terms"]),
	}
	if line := intArg(args, "line", 0); line > 0 && input.From == 0 && input.To == 0 {
		input.From, input.To = line, line
	}
	if input.From < 0 || input.To < 0 || input.From > 0 && input.To > 0 && input.To < input.From {
		return input, fmt.Errorf("from/to must be a positive ordered range")
	}
	if (input.From > 0) != (input.To > 0) {
		return input, fmt.Errorf("from and to must be provided together")
	}
	if len(input.Terms) > 20 {
		return input, fmt.Errorf("terms exceeds limit of 20")
	}
	for _, term := range input.Terms {
		if len(term) > 80 {
			return input, fmt.Errorf("spec term exceeds 80 bytes")
		}
	}
	for name, value := range map[string]string{
		"corpus": input.Corpus, "file": input.File, "component": input.Component, "route": input.Route, "selector": input.Selector,
	} {
		if len(value) > 512 {
			return input, fmt.Errorf("%s exceeds 512 bytes", name)
		}
	}
	if input.Route == "" {
		if sessionID := strings.TrimSpace(stringArg(args, "session_id")); sessionID != "" {
			session, ok := manager.GetSession(sessionID)
			if !ok {
				return input, fmt.Errorf("unknown session %s", sessionID)
			}
			if parsed, parseErr := url.Parse(session.URL); parseErr == nil {
				input.Route = parsed.Path
			}
		}
	}
	return input, nil
}

func browserSpecView(args map[string]any) (string, error) {
	view := strings.ToLower(strings.TrimSpace(stringArg(args, "view")))
	if view == "" {
		view = "compact"
	}
	if view != "summary" && view != "compact" && view != "full" {
		return "", fmt.Errorf("unsupported view %q", view)
	}
	return view, nil
}

func browserSpecContext(ctx context.Context, manager *browser.SessionManager, sessionID string, terms []string) (map[string]any, error) {
	loaded, err := manager.LoadSpecs(ctx)
	if err != nil {
		return nil, err
	}
	input, err := browserSpecInput(manager, map[string]any{"session_id": sessionID, "terms": terms})
	if err != nil {
		return nil, err
	}
	config := manager.SpecsConfig()
	input.Max = config.MaxResults
	matches := browserspec.MatchSpecs(loaded.Specs, input, config.MaxExcerptBytes)
	sanitizeBrowserSpecMatches(manager, matches)
	return map[string]any{
		"documents": matches, "count": len(matches), "specs_loaded": len(loaded.Specs),
		"files_scanned": loaded.FilesScanned, "entries_scanned": loaded.EntriesScanned,
		"bytes_loaded": loaded.BytesLoaded, "warnings": loaded.Warnings,
		"catalog_truncated": loaded.Truncated,
	}, nil
}

func sanitizeBrowserSpecMatches(manager *browser.SessionManager, matches []browserspec.Match) {
	for index := range matches {
		matches[index].Name = manager.SanitizeForEvidence(matches[index].Name)
		matches[index].Title = manager.SanitizeForEvidence(matches[index].Title)
		matches[index].Summary = manager.SanitizeForEvidence(matches[index].Summary)
		matches[index].ReadWhen = manager.SanitizeForEvidence(matches[index].ReadWhen)
		matches[index].Excerpt = manager.SanitizeForEvidence(matches[index].Excerpt)
		for bindingIndex := range matches[index].Bindings {
			matches[index].Bindings[bindingIndex].Target = manager.SanitizeForEvidence(matches[index].Bindings[bindingIndex].Target)
		}
	}
}

func publicBrowserSpecInvariants(manager *browser.SessionManager, invariants []browserspec.SelectedInvariant, view string, maxProseBytes int) []map[string]any {
	result := make([]map[string]any, 0, len(invariants))
	for _, invariant := range invariants {
		row := map[string]any{
			"spec": manager.SanitizeForEvidence(invariant.Spec), "path": invariant.Path,
			"corpus": invariant.Corpus, "name": manager.SanitizeForEvidence(invariant.Name),
			"expect": invariant.Expect,
		}
		if invariant.Query != "" {
			row["query"] = manager.SanitizeForEvidence(invariant.Query)
		}
		if invariant.File != "" {
			row["file"] = invariant.File
		}
		if invariant.From > 0 {
			row["from"], row["to"] = invariant.From, invariant.To
		}
		if view == "full" && invariant.Prose != "" {
			row["prose"] = manager.SanitizeForEvidence(boundBrowserSpecText(invariant.Prose, maxProseBytes))
		}
		result = append(result, row)
	}
	return result
}

func boundBrowserSpecText(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	end := maxBytes
	for end > 0 && end < len(value) && value[end]&0xc0 == 0x80 {
		end--
	}
	return value[:end]
}
