package research

// Browser audit is the passive discover phase of the contract audit system.
// It reads page facts and searches a bounded, confined repository scan.
// This tool navigates nothing, presses nothing and changes nothing.

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/browser"
	browsersecurity "codenerd/internal/browser/security"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

const maxAuditDetailBytes = 300

// BrowserAuditTool returns the passive discover phase of contract audits.
func BrowserAuditTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_audit",
		Description: `Passive discover phase of the contract audit. Reads page facts (request URLs, form field descriptors, current route) and searches a bounded, confined repository scan. It navigates nothing, presses nothing and changes nothing. Mutating controls are reported as requiring approval rather than exercised, and execute/report/resume phases are not yet available.`,
		Category:    tools.CategoryResearch,
		Priority:    70,
		Execute:     executeBrowserAudit,
		Schema: tools.ToolSchema{
			Required: []string{"operation", "session_id"},
			Properties: map[string]tools.Property{
				"operation":      {Type: "string", Enum: []any{"discover"}, Description: "Only discover is available; execute/report/resume phases are not yet available"},
				"session_id":     {Type: "string", Description: "Session scope enforced on every result"},
				"repo_root":      {Type: "string", Description: "Optional repository root confined to the workspace; blank defaults to workspace root and is confined before use"},
				"max_files":      {Type: "integer", Description: "Maximum files to open per scan; clamped down to the package ceiling (a caller cannot raise a limit)"},
				"max_file_bytes": {Type: "integer", Description: "Maximum bytes read per file; clamped down to the package ceiling (a caller cannot raise a limit)"},
				"max_matches":    {Type: "integer", Description: "Maximum matches returned; clamped down to the package ceiling (a caller cannot raise a limit)"},
				"max_depth":      {Type: "integer", Description: "Maximum directory depth; clamped down to the package ceiling (a caller cannot raise a limit)"},
				"view":           {Type: "string", Default: "compact", Enum: []any{"summary", "compact", "full"}, Description: "Disclosure depth"},
			},
		},
	}
}

func executeBrowserAudit(ctx context.Context, args map[string]any) (string, error) {
	kernel := getBrowserKernel()
	if kernel == nil {
		return "", fmt.Errorf("browser audit: live Cortex kernel is not bound")
	}
	sessionID := strings.TrimSpace(stringArg(args, "session_id"))
	if sessionID == "" {
		return "", fmt.Errorf("browser audit: session_id is required")
	}
	operation := parseAuditOperation(args)
	if operation != "discover" {
		return "", fmt.Errorf("browser audit: unsupported operation %q", operation)
	}
	view, err := parseAuditView(args)
	if err != nil {
		return "", err
	}
	repoRoot, err := resolveAuditRepoRoot(args)
	if err != nil {
		return "", err
	}
	limits := auditLimitsFromArgs(args)
	input, auditNotes := buildAuditInput(ctx, kernel, sessionID, repoRoot, limits)
	discovery, err := browser.DiscoverContract(ctx, input)
	if err != nil {
		return "", fmt.Errorf("browser audit: %w", err)
	}
	allNotes := mergeAuditNotes(discovery.Notes, auditNotes)
	output := buildAuditOutput(sessionID, operation, view, discovery, allNotes)
	recordBrowserToolEvidence(sessionID, "audit", map[string]any{
		"operation": operation, "view": view, "needles": len(discovery.Needles),
		"matches": len(discovery.Matches), "truncated": discovery.Truncated,
	})
	return marshalBrowserAuditResult(output)
}

func parseAuditOperation(args map[string]any) string {
	op := strings.ToLower(strings.TrimSpace(stringArg(args, "operation")))
	if op == "" {
		op = "discover"
	}
	return op
}

func parseAuditView(args map[string]any) (string, error) {
	view := strings.ToLower(strings.TrimSpace(stringArg(args, "view")))
	if view == "" {
		view = "compact"
	}
	if view != "summary" && view != "compact" && view != "full" {
		return "", fmt.Errorf("browser audit: unsupported view %q", view)
	}
	return view, nil
}

// resolveAuditRepoRoot confines the repository root before use.
// The repository root is the one input that decides what the audit may read,
// so it is confined before use rather than validated after. An unconfined value
// would let a tool call read any path on disk, so the workspace root is the
// trust boundary and every candidate is resolved against it before any scan.
func resolveAuditRepoRoot(args map[string]any) (string, error) {
	mgr := getBrowserManager()
	workspaceRoot := mgr.WorkspaceRoot()
	if strings.TrimSpace(workspaceRoot) == "" {
		return "", fmt.Errorf("browser audit: workspace root is not configured; refusing to audit without a bounded root (would default to process cwd and expose the entire filesystem)")
	}
	repoArg := strings.TrimSpace(stringArg(args, "repo_root"))
	if repoArg == "" {
		return workspaceRoot, nil
	}
	confined, err := browsersecurity.ConfineToRoot(workspaceRoot, repoArg)
	if err != nil {
		return "", fmt.Errorf("browser audit: repo_root escapes workspace: %w", err)
	}
	return confined, nil
}

func auditLimitsFromArgs(args map[string]any) browser.RepoTraceLimits {
	return browser.RepoTraceLimits{
		MaxFiles:     intArg(args, "max_files", 0),
		MaxFileBytes: intArg(args, "max_file_bytes", 0),
		MaxMatches:   intArg(args, "max_matches", 0),
		MaxDepth:     intArg(args, "max_depth", 0),
	}
}

func buildAuditInput(ctx context.Context, kernel types.Kernel, sessionID, repoRoot string, limits browser.RepoTraceLimits) (browser.ContractAuditInput, []string) {
	requestURLs := collectAuditRequestURLs(ctx, kernel, sessionID)
	formFields := collectAuditFormFields(ctx, kernel, sessionID)
	routes, routeNote := collectAuditRoutes(ctx, kernel, sessionID)
	mutatingNote := "mutating-control detection is not yet wired"
	notes := []string{mutatingNote}
	if routeNote != "" {
		notes = append(notes, routeNote)
	}
	in := browser.ContractAuditInput{
		RepoRoot:         repoRoot,
		Routes:           routes,
		FormFields:       formFields,
		RequestURLs:      requestURLs,
		MutatingControls: []string{},
		Limits:           limits,
	}
	return in, notes
}

func mergeAuditNotes(discoveryNotes, inputNotes []string) []string {
	all := make([]string, 0, len(discoveryNotes)+len(inputNotes))
	all = append(all, discoveryNotes...)
	all = append(all, inputNotes...)
	seen := make(map[string]struct{})
	deduped := make([]string, 0, len(all))
	for _, n := range all {
		trim := strings.TrimSpace(n)
		if trim == "" {
			continue
		}
		if _, ok := seen[trim]; ok {
			continue
		}
		seen[trim] = struct{}{}
		deduped = append(deduped, trim)
	}
	return deduped
}

func buildAuditOutput(sessionID, operation, view string, discovery browser.ContractAuditDiscovery, notes []string) map[string]any {
	counts := auditCounts(discovery.Findings)
	out := map[string]any{
		"success":            true,
		"session_id":         sessionID,
		"operation":          operation,
		"repo_root_confined": true,
		"view":               view,
		"needles":            discovery.Needles,
		"needle_count":       len(discovery.Needles),
		"match_count":        len(discovery.Matches),
		"counts":             counts,
	}
	if len(notes) > 0 {
		out["notes"] = notes
	}
	if discovery.Truncated {
		out["truncated"] = true
	}
	switch view {
	case "summary":
	case "compact":
		out["findings"] = compactAuditFindings(discovery.Findings)
	case "full":
		out["findings"] = fullAuditFindings(discovery.Findings)
		matches := make([]map[string]any, 0, len(discovery.Matches))
		for _, m := range discovery.Matches {
			matches = append(matches, map[string]any{
				"path":    m.Path,
				"line":    m.Line,
				"needle":  m.Needle,
				"snippet": m.Snippet,
			})
		}
		out["matches"] = matches
	}
	return out
}

func collectAuditRequestURLs(ctx context.Context, kernel types.Kernel, sessionID string) []string {
	facts, err := queryScopedBrowserFacts(ctx, kernel, "net_request", "net_request", sessionID)
	if err != nil {
		return nil
	}
	var urls []string
	for _, f := range facts {
		if len(f.Args) <= 3 {
			continue
		}
		raw := fmt.Sprint(f.Args[3])
		trim := strings.TrimSpace(raw)
		if trim == "" {
			continue
		}
		urls = append(urls, trim)
		if len(urls) >= maxBrowserKernelScan {
			break
		}
	}
	return urls
}

func collectAuditFormFields(ctx context.Context, kernel types.Kernel, sessionID string) []string {
	facts, err := queryScopedBrowserFacts(ctx, kernel, "input_event", "input_event", sessionID)
	if err != nil {
		return nil
	}
	var fields []string
	for _, f := range facts {
		if len(f.Args) <= 1 {
			continue
		}
		raw := fmt.Sprint(f.Args[1])
		trim := strings.TrimSpace(raw)
		if trim == "" {
			continue
		}
		fields = append(fields, trim)
		if len(fields) >= maxBrowserKernelScan {
			break
		}
	}
	return fields
}

func collectAuditRoutes(ctx context.Context, kernel types.Kernel, sessionID string) ([]string, string) {
	if routes := routesFromPredicate(ctx, kernel, sessionID, "navigation_event"); len(routes) > 0 {
		return routes, ""
	}
	if _, ok := browserPredicateSpecs["current_url"]; ok {
		if routes := routesFromPredicate(ctx, kernel, sessionID, "current_url"); len(routes) > 0 {
			return routes, ""
		}
	}
	return nil, "route facts were unavailable; using URL path segments only"
}

func routesFromPredicate(ctx context.Context, kernel types.Kernel, sessionID, predicate string) []string {
	facts, err := queryScopedBrowserFacts(ctx, kernel, predicate, predicate, sessionID)
	if err != nil || len(facts) == 0 {
		return nil
	}
	var routes []string
	for _, f := range facts {
		if len(f.Args) <= 1 {
			continue
		}
		raw := fmt.Sprint(f.Args[1])
		trim := strings.TrimSpace(raw)
		if trim == "" {
			continue
		}
		routes = append(routes, trim)
		if len(routes) >= maxBrowserKernelScan {
			break
		}
	}
	return routes
}

func auditCounts(findings []browser.AuditFinding) map[string]int {
	counts := make(map[string]int)
	for _, f := range findings {
		counts[string(f.Kind)]++
	}
	for _, kind := range []browser.AuditFindingKind{
		browser.AuditObservation,
		browser.AuditInference,
		browser.AuditSkipped,
		browser.AuditApprovalRequired,
		browser.AuditExecutionFailure,
		browser.AuditContractMismatch,
	} {
		if _, ok := counts[string(kind)]; !ok {
			counts[string(kind)] = 0
		}
	}
	return counts
}

func compactAuditFindings(findings []browser.AuditFinding) []map[string]any {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		return findings[i].Subject < findings[j].Subject
	})
	out := make([]map[string]any, 0, len(findings))
	for _, f := range findings {
		detail := truncateAuditDetail(f.Detail, maxAuditDetailBytes)
		row := map[string]any{
			"kind":    string(f.Kind),
			"subject": f.Subject,
			"detail":  detail,
		}
		out = append(out, row)
	}
	return out
}

func fullAuditFindings(findings []browser.AuditFinding) []map[string]any {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		return findings[i].Subject < findings[j].Subject
	})
	out := make([]map[string]any, 0, len(findings))
	for _, f := range findings {
		row := map[string]any{
			"kind":    string(f.Kind),
			"subject": f.Subject,
			"detail":  f.Detail,
			"sources": f.Sources,
		}
		out = append(out, row)
	}
	return out
}

func truncateAuditDetail(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	end := maxBytes
	for end > 0 && end < len(s) && s[end]&0xc0 == 0x80 {
		end--
	}
	truncated := s[:end]
	if len(s) > maxBytes {
		truncated += "..."
	}
	return truncated
}

func marshalBrowserAuditResult(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal browser audit result: %w", err)
	}
	return string(data), nil
}

var _ = json.Number("")
