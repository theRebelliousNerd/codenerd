package browser

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	browsersecurity "codenerd/internal/browser/security"
)

// AuditFindingKind distinguishes what the audit actually knows. The spec
// requires observation, inference, skipped work, approval-gated work,
// execution failure and confirmed contract mismatch to remain separable,
// so a reader is never left guessing whether a finding was seen or
// deduced.
type AuditFindingKind string

const (
	AuditObservation      AuditFindingKind = "observation"
	AuditInference        AuditFindingKind = "inference"
	AuditSkipped          AuditFindingKind = "skipped"
	AuditApprovalRequired AuditFindingKind = "approval_required"
	AuditExecutionFailure AuditFindingKind = "execution_failure"
	AuditContractMismatch AuditFindingKind = "contract_mismatch"
)

// AuditFinding is a classified observation or plan entry from the audit.
type AuditFinding struct {
	Kind    AuditFindingKind
	Subject string
	Detail  string
	Sources []string
}

const minAuditNeedleLength = 4
const maxAuditNeedles = 24

var auditNoiseSegments = map[string]struct{}{
	"api":   {},
	"v1":    {},
	"v2":    {},
	"www":   {},
	"http":  {},
	"https": {},
}

func isAuditNoise(s string) bool {
	_, ok := auditNoiseSegments[s]
	return ok
}

func isNumericOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func shouldDropAuditTerm(term string) bool {
	if len(term) < minAuditNeedleLength {
		return true
	}
	if isNumericOnly(term) {
		return true
	}
	if isAuditNoise(term) {
		return true
	}
	return false
}

func stripQueryFragment(s string) string {
	if idx := strings.Index(s, "?"); idx != -1 {
		s = s[:idx]
	}
	if idx := strings.Index(s, "#"); idx != -1 {
		s = s[:idx]
	}
	return s
}

func splitPath(p string) []string {
	p = stripQueryFragment(p)
	parts := strings.Split(p, "/")
	var res []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		res = append(res, part)
	}
	return res
}

func extractURLPath(raw string) string {
	u, err := url.Parse(raw)
	if err == nil {
		return u.Path
	}
	p := stripQueryFragment(raw)
	if idx := strings.Index(p, "://"); idx != -1 {
		after := p[idx+3:]
		if slash := strings.Index(after, "/"); slash != -1 {
			return after[slash:]
		}
		return ""
	}
	return p
}

func pathSegments(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.Contains(raw, "://") {
		return splitPath(extractURLPath(raw))
	}
	return splitPath(raw)
}

func urlPathSegments(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	path := extractURLPath(raw)
	return splitPath(path)
}

// auditNeedles derives repository search terms from page facts. Terms are
// deduplicated, lowercased, and filtered: a term shorter than
// minAuditNeedleLength matches almost every file and would turn a bounded
// scan into noise, so it is dropped rather than searched.
func auditNeedles(routes, formFields, requestURLs []string) []string {
	seen := make(map[string]struct{})
	addTerm := func(term string) {
		term = strings.TrimSpace(term)
		if term == "" {
			return
		}
		lower := strings.ToLower(term)
		if shouldDropAuditTerm(lower) {
			return
		}
		seen[lower] = struct{}{}
	}
	for _, r := range routes {
		for _, seg := range pathSegments(r) {
			addTerm(seg)
		}
	}
	for _, f := range formFields {
		addTerm(f)
	}
	for _, u := range requestURLs {
		for _, seg := range urlPathSegments(u) {
			addTerm(seg)
		}
	}
	if len(seen) == 0 {
		return []string{}
	}
	all := make([]string, 0, len(seen))
	for k := range seen {
		all = append(all, k)
	}
	if len(all) > maxAuditNeedles {
		sort.Slice(all, func(i, j int) bool {
			if len(all[i]) != len(all[j]) {
				return len(all[i]) > len(all[j])
			}
			return all[i] < all[j]
		})
		all = all[:maxAuditNeedles]
	}
	sort.Strings(all)
	return all
}

// ContractAuditInput holds the passive facts for discovery.
type ContractAuditInput struct {
	RepoRoot         string
	Routes           []string
	FormFields       []string
	RequestURLs      []string
	MutatingControls []string
	Limits           RepoTraceLimits
}

// ContractAuditDiscovery is the passive inventory and plan.
type ContractAuditDiscovery struct {
	Needles   []string
	Findings  []AuditFinding
	Matches   []RepoMatch
	Notes     []string
	Truncated bool
}

func validateAuditRoot(root string) (string, error) {
	clean := strings.TrimSpace(root)
	if clean == "" {
		return "", fmt.Errorf("root must not be empty")
	}
	info, err := os.Stat(clean)
	if err != nil {
		return "", fmt.Errorf("root %q: %w", clean, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("root %q is not a directory", clean)
	}
	return clean, nil
}

func buildSkippedDiscovery(needles []string, controls []string, redactor *browsersecurity.Redactor) ContractAuditDiscovery {
	detail := redactor.SanitizeString("no searchable term could be derived from routes, form fields, or request URLs")
	findings := []AuditFinding{{
		Kind:    AuditSkipped,
		Subject: "needles",
		Detail:  detail,
		Sources: []string{},
	}}
	for _, ctrl := range controls {
		trim := strings.TrimSpace(ctrl)
		if trim == "" {
			continue
		}
		raw := fmt.Sprintf("control %q requires explicit approval before being exercised; not executed", trim)
		d := redactor.SanitizeString(raw)
		findings = append(findings, AuditFinding{
			Kind:    AuditApprovalRequired,
			Subject: trim,
			Detail:  d,
			Sources: []string{},
		})
	}
	sortAuditFindings(findings)
	return ContractAuditDiscovery{
		Needles:   needles,
		Findings:  findings,
		Matches:   []RepoMatch{},
		Notes:     []string{},
		Truncated: false,
	}
}

func sortAuditFindings(f []AuditFinding) {
	sort.Slice(f, func(i, j int) bool {
		if f[i].Kind != f[j].Kind {
			return f[i].Kind < f[j].Kind
		}
		return f[i].Subject < f[j].Subject
	})
}

func cappedSources(paths []string) ([]string, string) {
	sort.Strings(paths)
	if len(paths) > 10 {
		suffix := fmt.Sprintf(" (showing 10 of %d files)", len(paths))
		return paths[:10], suffix
	}
	return paths, ""
}

func findingForNeedle(needle string, matches []RepoMatch, redactor *browsersecurity.Redactor) AuditFinding {
	if len(matches) == 0 {
		raw := fmt.Sprintf("needle %q appears nowhere in the scanned tree", needle)
		return AuditFinding{
			Kind:    AuditObservation,
			Subject: needle,
			Detail:  redactor.SanitizeString(raw),
			Sources: []string{},
		}
	}
	set := make(map[string]struct{})
	var uniq []string
	for _, m := range matches {
		if _, ok := set[m.Path]; !ok {
			set[m.Path] = struct{}{}
			uniq = append(uniq, m.Path)
		}
	}
	srcs, suffix := cappedSources(uniq)
	raw := fmt.Sprintf("needle %q matched %d file(s); textual match is evidence of a likely handler, not proof%s", needle, len(uniq), suffix)
	return AuditFinding{
		Kind:    AuditInference,
		Subject: needle,
		Detail:  redactor.SanitizeString(raw),
		Sources: srcs,
	}
}

func buildFindings(needles []string, groups map[string][]RepoMatch, controls []string, redactor *browsersecurity.Redactor) []AuditFinding {
	var out []AuditFinding
	for _, n := range needles {
		f := findingForNeedle(n, groups[n], redactor)
		out = append(out, f)
	}
	for _, c := range controls {
		trim := strings.TrimSpace(c)
		if trim == "" {
			continue
		}
		raw := fmt.Sprintf("control %q requires explicit approval before being exercised; not executed", trim)
		out = append(out, AuditFinding{
			Kind:    AuditApprovalRequired,
			Subject: trim,
			Detail:  redactor.SanitizeString(raw),
			Sources: []string{},
		})
	}
	sortAuditFindings(out)
	return out
}

// DiscoverContract performs the passive half of a contract audit: it
// derives search terms from what the page shows, traces them into a
// bounded repository scan, and returns classified findings plus a plan of
// steps that would need approval. It navigates nothing and changes
// nothing.
//
// Only observation, inference, skipped and approval_required findings are
// produced here. Execution failure and contract mismatch belong to later
// phases and are never emitted from passive discovery — emitting a
// mismatch without execution would be exactly the overclaim this
// classification exists to prevent.
func DiscoverContract(ctx context.Context, in ContractAuditInput) (ContractAuditDiscovery, error) {
	clean, err := validateAuditRoot(in.RepoRoot)
	if err != nil {
		return ContractAuditDiscovery{}, err
	}
	needles := auditNeedles(in.Routes, in.FormFields, in.RequestURLs)
	redactor := browsersecurity.NewRedactor(nil)
	if len(needles) == 0 {
		return buildSkippedDiscovery(needles, in.MutatingControls, redactor), nil
	}
	res, err := TraceRepository(ctx, clean, needles, in.Limits)
	if err != nil {
		return ContractAuditDiscovery{}, err
	}
	groups := make(map[string][]RepoMatch)
	for _, m := range res.Matches {
		groups[m.Needle] = append(groups[m.Needle], m)
	}
	findings := buildFindings(needles, groups, in.MutatingControls, redactor)
	matches := res.Matches
	if matches == nil {
		matches = []RepoMatch{}
	}
	notes := res.Notes
	if notes == nil {
		notes = []string{}
	}
	return ContractAuditDiscovery{
		Needles:   needles,
		Findings:  findings,
		Matches:   matches,
		Notes:     notes,
		Truncated: res.Truncated,
	}, nil
}
