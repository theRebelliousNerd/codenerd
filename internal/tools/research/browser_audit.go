package research

// Browser audit is the passive half of the contract audit system: discover,
// report and resume. It reads page facts and searches a bounded, confined
// repository scan. This tool navigates nothing, presses nothing and changes
// nothing; the execute phase, which would, is not available.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/browser"
	browsersecurity "codenerd/internal/browser/security"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// BrowserAuditTool returns the passive phases of contract audits.
func BrowserAuditTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_audit",
		Description: `Passive phases of the contract audit. discover reads page facts (request URLs, form field descriptors, current route) and searches a bounded, confined repository scan. report synthesizes that evidence into bounded, redacted sections with evidence handles. resume reopens only the sections named by handles from a report. It navigates nothing, presses nothing and changes nothing. Mutating controls are reported as requiring approval rather than exercised; the execute phase is not available.`,
		Category:    tools.CategoryResearch,
		Priority:    70,
		Execute:     executeBrowserAudit,
		Schema: tools.ToolSchema{
			Required: []string{"operation", "session_id"},
			Properties: map[string]tools.Property{
				"operation":      {Type: "string", Enum: []any{"discover", "report", "resume"}, Description: "discover lists findings; report synthesizes them into sections with evidence handles; resume reopens the sections named by handles. The execute phase is not available"},
				"handles":        {Type: "array", Description: "resume only: evidence handles from a report (audit:<session>:<section>)", Items: &tools.PropertyItems{Type: "string"}},
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
	// Every browser tool needs the manager boot bound; without it the
	// helpers below would dereference nil. Refuse here, before any of them run.
	if _, err := requireBoundBrowser(); err != nil {
		return "", err
	}
	kernel := getBrowserKernel()
	if kernel == nil {
		return "", fmt.Errorf("browser audit: live Cortex kernel is not bound")
	}
	sessionID := strings.TrimSpace(stringArg(args, "session_id"))
	if sessionID == "" {
		return "", fmt.Errorf("browser audit: session_id is required")
	}
	operation := parseAuditOperation(args)
	if operation != "discover" && operation != "report" && operation != "resume" {
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
	input, auditNotes, err := buildAuditInput(ctx, kernel, sessionID, repoRoot, limits)
	if err != nil {
		return "", err
	}
	discovery, err := browser.DiscoverContract(ctx, input)
	if err != nil {
		return "", fmt.Errorf("browser audit: %w", err)
	}
	allNotes := mergeAuditNotes(discovery.Notes, auditNotes)
	var output map[string]any
	switch operation {
	case "discover":
		output = buildAuditOutput(sessionID, operation, view, discovery, allNotes)
	default:
		// A report is pure over the evidence, and discovery over the same page
		// facts and repository is deterministic, so resume rebuilds the report
		// rather than keeping one between calls.
		report := browser.BuildAuditReport(browser.AuditReportInput{SessionID: sessionID, Discovery: discovery})
		report.Notes = mergeAuditNotes(report.Notes, auditNotes)
		if operation == "report" {
			output = buildAuditReportOutput(sessionID, view, report)
		} else {
			evidence, resumeNotes := browser.ResumeAuditEvidence(report, stringSliceArg(args["handles"]))
			output = map[string]any{
				"success": true, "session_id": sessionID, "operation": operation,
				"repo_root_confined": true, "evidence": evidence, "notes": resumeNotes,
			}
		}
	}
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

// buildAuditInput reads the page facts the audit reasons over. Collector
// notes (a scan that stopped at the kernel bound) travel into the audit
// JSON notes; a kernel error fails the audit, because auditing empty
// evidence as if the page had none would certify a blank page.
func buildAuditInput(ctx context.Context, kernel types.Kernel, sessionID, repoRoot string, limits browser.RepoTraceLimits) (browser.ContractAuditInput, []string, error) {
	requestURLs, urlsNote, err := collectAuditRequestURLs(ctx, kernel, sessionID)
	if err != nil {
		return browser.ContractAuditInput{}, nil, fmt.Errorf("browser audit: collect request URLs: %w", err)
	}
	formFields, fieldsNote, err := collectAuditFormFields(ctx, kernel, sessionID)
	if err != nil {
		return browser.ContractAuditInput{}, nil, fmt.Errorf("browser audit: collect form fields: %w", err)
	}
	routes, routeNotes, err := collectAuditRoutes(ctx, kernel, sessionID)
	if err != nil {
		return browser.ContractAuditInput{}, nil, fmt.Errorf("browser audit: collect routes: %w", err)
	}
	mutatingNote := "mutating-control detection is not yet wired"
	notes := []string{mutatingNote}
	if urlsNote != "" {
		notes = append(notes, urlsNote)
	}
	if fieldsNote != "" {
		notes = append(notes, fieldsNote)
	}
	notes = append(notes, routeNotes...)
	in := browser.ContractAuditInput{
		RepoRoot:         repoRoot,
		Routes:           routes,
		FormFields:       formFields,
		RequestURLs:      requestURLs,
		MutatingControls: []string{},
		Limits:           limits,
	}
	return in, notes, nil
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

// buildAuditReportOutput shapes a report for the view: summary carries counts
// and handles, compact adds the finding sections, full adds every section.
func buildAuditReportOutput(sessionID, view string, report browser.AuditReport) map[string]any {
	out := map[string]any{
		"success":            true,
		"session_id":         sessionID,
		"operation":          "report",
		"repo_root_confined": true,
		"view":               view,
		"counts":             report.Counts,
		"handles":            report.Handles,
		"notes":              report.Notes,
		"truncated":          report.Truncated,
	}
	switch view {
	case "compact":
		sections := make(map[string][]string)
		for name, lines := range report.Sections {
			if browser.IsAuditFindingSection(name) {
				sections[name] = lines
			}
		}
		out["sections"] = sections
	case "full":
		out["sections"] = report.Sections
	}
	return out
}

// browserScanLimitNote names the predicate whose kernel scan passed
// maxBrowserKernelScan: the facts past the scan were not read, so the rows
// the audit keeps are partial. The note travels with the rows into the
// audit JSON notes rather than failing the audit, because a bounded scan
// is a known-degraded read, not a broken kernel (LC5b).
func browserScanLimitNote(predicate string) string {
	return fmt.Sprintf("browser audit: %s facts past the kernel scan limit were not read; results are partial", predicate)
}

func collectAuditRequestURLs(ctx context.Context, kernel types.Kernel, sessionID string) ([]string, string, error) {
	facts, err := queryScopedBrowserFacts(ctx, kernel, "net_request", "net_request", sessionID)
	if err != nil && !errors.Is(err, errBrowserKernelScanLimit) {
		return nil, "", err
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
	}
	if errors.Is(err, errBrowserKernelScanLimit) {
		return urls, browserScanLimitNote("net_request"), nil
	}
	return urls, "", nil
}

func collectAuditFormFields(ctx context.Context, kernel types.Kernel, sessionID string) ([]string, string, error) {
	facts, err := queryScopedBrowserFacts(ctx, kernel, "input_event", "input_event", sessionID)
	if err != nil && !errors.Is(err, errBrowserKernelScanLimit) {
		return nil, "", err
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
	}
	if errors.Is(err, errBrowserKernelScanLimit) {
		return fields, browserScanLimitNote("input_event"), nil
	}
	return fields, "", nil
}

func collectAuditRoutes(ctx context.Context, kernel types.Kernel, sessionID string) ([]string, []string, error) {
	var notes []string
	routes, note, err := routesFromPredicate(ctx, kernel, sessionID, "navigation_event")
	if err != nil {
		return nil, nil, err
	}
	if note != "" {
		notes = append(notes, note)
	}
	if len(routes) > 0 {
		return routes, notes, nil
	}
	if _, ok := browserPredicateSpecs["current_url"]; ok {
		routes, note, err := routesFromPredicate(ctx, kernel, sessionID, "current_url")
		if err != nil {
			return nil, nil, err
		}
		if note != "" {
			notes = append(notes, note)
		}
		if len(routes) > 0 {
			return routes, notes, nil
		}
	}
	notes = append(notes, "route facts were unavailable; using URL path segments only")
	return nil, notes, nil
}

func routesFromPredicate(ctx context.Context, kernel types.Kernel, sessionID, predicate string) ([]string, string, error) {
	facts, err := queryScopedBrowserFacts(ctx, kernel, predicate, predicate, sessionID)
	if err != nil && !errors.Is(err, errBrowserKernelScanLimit) {
		return nil, "", err
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
	}
	if errors.Is(err, errBrowserKernelScanLimit) {
		return routes, browserScanLimitNote(predicate), nil
	}
	return routes, "", nil
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
		// Whole detail: the old compact view cut details at 300 bytes
		// with "...", silently dropping audit evidence. Compact still
		// differs from full (no sources, no match list); it just no
		// longer cuts. The ledger sizes the result for the window
		// (limits cleanup 2026-09-29).
		row := map[string]any{
			"kind":    string(f.Kind),
			"subject": f.Subject,
			"detail":  f.Detail,
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

func marshalBrowserAuditResult(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal browser audit result: %w", err)
	}
	return string(data), nil
}

var _ = json.Number("")
