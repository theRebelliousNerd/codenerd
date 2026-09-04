package browser

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	browsersecurity "codenerd/internal/browser/security"
)

// AuditReportInput is what a report synthesizes. Every field is evidence
// already gathered; report performs no new discovery.
type AuditReportInput struct {
	SessionID    string
	Discovery    ContractAuditDiscovery
	Correlations ContainerCorrelationResult // backend-log evidence; zero value when unconfigured
	KernelFacts  int                        // count of audit facts asserted, 0 when not asserted
}

// AuditReport is a bounded synthesis of one audit.
type AuditReport struct {
	SessionID string
	Counts    map[string]int      // per AuditFindingKind, plus "needles", "matches", "correlations"
	Sections  map[string][]string // human-readable lines per section, already redacted
	Handles   []string            // evidence handles this report can reopen
	Notes     []string
	Truncated bool
}

// maxAuditReportLines caps total lines across all sections.
const maxAuditReportLines = 400

// maxResumeHandles caps the number of handles served per resume call.
const maxResumeHandles = 20

var orderedReportSections = []string{
	"observations",
	"inferences",
	"skipped",
	"approval_required",
	"contract_mismatches",
	"execution_failures",
	"sources",
	"backend_logs",
}

var findingSectionSet = map[string]struct{}{
	"observations":       {},
	"inferences":         {},
	"skipped":            {},
	"approval_required":  {},
	"contract_mismatches": {},
	"execution_failures": {},
}

func isFindingSection(name string) bool {
	_, ok := findingSectionSet[name]
	return ok
}

// looksRooted reports whether p is rooted under EITHER convention.
// filepath.IsAbs is platform-specific - on Windows a leading slash is not
// absolute - but a path that is rooted under any convention must still be
// made relative before it enters a report, because the report is read on a
// machine we do not control.
func looksRooted(p string) bool {
	if p == "" {
		return false
	}
	if filepath.IsAbs(p) {
		return true
	}
	if p[0] == '/' || p[0] == '\\' {
		return true
	}
	if len(p) >= 2 && ((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z')) && p[1] == ':' {
		return true
	}
	if len(p) >= 2 && ((p[0] == '/' && p[1] == '/') || (p[0] == '\\' && p[1] == '\\')) {
		return true
	}
	return false
}

func auditReportRelativePath(p string) string {
	clean := filepath.ToSlash(p)
	clean = strings.ReplaceAll(clean, "\\", "/")
	if !looksRooted(clean) {
		return clean
	}
	if len(clean) >= 2 && ((clean[0] >= 'A' && clean[0] <= 'Z') || (clean[0] >= 'a' && clean[0] <= 'z')) && clean[1] == ':' {
		clean = clean[2:]
		clean = strings.TrimLeft(clean, "/")
	} else {
		clean = strings.TrimLeft(clean, "/")
		if len(clean) >= 2 && ((clean[0] >= 'A' && clean[0] <= 'Z') || (clean[0] >= 'a' && clean[0] <= 'z')) && clean[1] == ':' {
			clean = clean[2:]
			clean = strings.TrimLeft(clean, "/")
		}
	}
	clean = strings.TrimLeft(clean, "/")
	if clean == "" {
		base := filepath.Base(filepath.ToSlash(p))
		base = strings.ReplaceAll(base, "\\", "/")
		base = filepath.ToSlash(base)
		clean = base
	}
	if looksRooted(clean) || clean == "" {
		base := filepath.Base(clean)
		base = strings.ReplaceAll(base, "\\", "/")
		base = filepath.ToSlash(base)
		clean = base
	}
	if looksRooted(clean) {
		clean = strings.TrimLeft(clean, "/")
		clean = strings.TrimLeft(clean, "\\")
		if len(clean) >= 2 && ((clean[0] >= 'A' && clean[0] <= 'Z') || (clean[0] >= 'a' && clean[0] <= 'z')) && clean[1] == ':' {
			clean = clean[2:]
			clean = strings.TrimLeft(clean, "/")
			clean = strings.TrimLeft(clean, "\\")
		}
		clean = strings.TrimLeft(clean, "/")
		clean = strings.TrimLeft(clean, "\\")
		if clean == "" {
			if p == "" {
				return ""
			}
			return "."
		}
	}
	clean = strings.ReplaceAll(clean, "\\", "/")
	clean = filepath.ToSlash(clean)
	clean = strings.TrimLeft(clean, "/")
	if clean == "" {
		if p == "" {
			return ""
		}
		return "."
	}
	if len(clean) >= 2 && ((clean[0] >= 'A' && clean[0] <= 'Z') || (clean[0] >= 'a' && clean[0] <= 'z')) && clean[1] == ':' {
		tmp := strings.TrimLeft(clean[2:], "/\\")
		tmp = strings.ReplaceAll(tmp, "\\", "/")
		tmp = filepath.ToSlash(tmp)
		if tmp != "" {
			clean = tmp
		} else {
			if p == "" {
				return ""
			}
			clean = "."
		}
	}
	return clean
}

func buildAuditCounts(in AuditReportInput) map[string]int {
	counts := map[string]int{
		string(AuditObservation):      0,
		string(AuditInference):        0,
		string(AuditSkipped):          0,
		string(AuditApprovalRequired): 0,
		string(AuditExecutionFailure): 0,
		string(AuditContractMismatch): 0,
	}
	for _, f := range in.Discovery.Findings {
		key := string(f.Kind)
		if _, ok := counts[key]; !ok {
			counts[key] = 0
		}
		counts[key]++
	}
	needles := len(in.Discovery.Needles)
	matches := len(in.Discovery.Matches)
	correlations := len(in.Correlations.Correlations)
	counts["needles"] = needles
	counts["matches"] = matches
	counts["correlations"] = correlations
	return counts
}

func buildAuditSections(in AuditReportInput) map[string][]string {
	sections := make(map[string][]string, 8)
	for k := range findingSectionSet {
		sections[k] = []string{}
	}
	for _, f := range in.Discovery.Findings {
		var sec string
		switch f.Kind {
		case AuditObservation:
			sec = "observations"
		case AuditInference:
			sec = "inferences"
		case AuditSkipped:
			sec = "skipped"
		case AuditApprovalRequired:
			sec = "approval_required"
		case AuditContractMismatch:
			sec = "contract_mismatches"
		case AuditExecutionFailure:
			sec = "execution_failures"
		default:
			continue
		}
		line := fmt.Sprintf("%s: %s", f.Subject, f.Detail)
		sections[sec] = append(sections[sec], line)
	}
	if len(in.Discovery.Matches) > 0 {
		var srcLines []string
		for _, m := range in.Discovery.Matches {
			rel := auditReportRelativePath(m.Path)
			line := fmt.Sprintf("%s:%d", rel, m.Line)
			srcLines = append(srcLines, line)
		}
		sections["sources"] = srcLines
	}
	if len(in.Correlations.Correlations) > 0 {
		var logLines []string
		for _, c := range in.Correlations.Correlations {
			line := fmt.Sprintf("%s: %s (delta %dms)", c.Container, c.LogMessage, c.DeltaMs)
			logLines = append(logLines, line)
		}
		sections["backend_logs"] = logLines
	}
	return sections
}

func truncateAuditSections(sections map[string][]string, alreadyTruncated bool) (bool, []string) {
	total := 0
	for _, k := range orderedReportSections {
		if sec, ok := sections[k]; ok {
			total += len(sec)
		}
	}
	if total <= maxAuditReportLines {
		return alreadyTruncated, nil
	}
	remaining := maxAuditReportLines
	cutSection := ""
	for idx, k := range orderedReportSections {
		sec, ok := sections[k]
		if !ok {
			continue
		}
		if len(sec) <= remaining {
			remaining -= len(sec)
			continue
		}
		cutSection = k
		if remaining < len(sec) {
			if isFindingSection(k) {
				if remaining == 0 {
					sections[k] = []string{}
				} else {
					cp := make([]string, remaining)
					copy(cp, sec[:remaining])
					sections[k] = cp
				}
			} else {
				if remaining == 0 {
					delete(sections, k)
				} else {
					cp := make([]string, remaining)
					copy(cp, sec[:remaining])
					sections[k] = cp
				}
			}
		}
		for j := idx + 1; j < len(orderedReportSections); j++ {
			later := orderedReportSections[j]
			if _, exists := sections[later]; !exists {
				continue
			}
			if isFindingSection(later) {
				sections[later] = []string{}
			} else {
				delete(sections, later)
			}
		}
		break
	}
	redactor := browsersecurity.NewRedactor(nil)
	var note string
	if cutSection != "" {
		note = fmt.Sprintf("truncated section %q: total lines capped at %d", cutSection, maxAuditReportLines)
	} else {
		note = fmt.Sprintf("truncated report: total lines capped at %d", maxAuditReportLines)
	}
	note = redactor.SanitizeString(note)
	return true, []string{note}
}

func buildAuditNotes(in AuditReportInput, truncNotes []string) []string {
	var notes []string
	if len(in.Discovery.Notes) > 0 {
		cp := make([]string, len(in.Discovery.Notes))
		copy(cp, in.Discovery.Notes)
		notes = append(notes, cp...)
	}
	if len(in.Correlations.Notes) > 0 {
		cp := make([]string, len(in.Correlations.Notes))
		copy(cp, in.Correlations.Notes)
		notes = append(notes, cp...)
	}
	if len(truncNotes) > 0 {
		notes = append(notes, truncNotes...)
	}
	if notes == nil {
		notes = []string{}
	}
	return notes
}

func buildAuditHandles(sessionID string, sections map[string][]string) []string {
	var handles []string
	for _, k := range orderedReportSections {
		sec, ok := sections[k]
		if !ok || len(sec) == 0 {
			continue
		}
		h := fmt.Sprintf("audit:%s:%s", sessionID, k)
		handles = append(handles, h)
	}
	sort.Strings(handles)
	if handles == nil {
		handles = []string{}
	}
	return handles
}

// BuildAuditReport synthesizes gathered evidence into a bounded report. It
// is pure: it performs no scanning, no navigation and no kernel access, so
// a report can be rebuilt from stored evidence without side effects.
func BuildAuditReport(in AuditReportInput) AuditReport {
	counts := buildAuditCounts(in)
	sections := buildAuditSections(in)
	alreadyTruncated := in.Discovery.Truncated || in.Correlations.Truncated
	truncated, truncNotes := truncateAuditSections(sections, alreadyTruncated)
	for k := range findingSectionSet {
		if _, ok := sections[k]; !ok {
			sections[k] = []string{}
		}
		if sections[k] == nil {
			sections[k] = []string{}
		}
	}
	notes := buildAuditNotes(in, truncNotes)
	handles := buildAuditHandles(in.SessionID, sections)
	if sections == nil {
		sections = map[string][]string{}
	}
	return AuditReport{
		SessionID: in.SessionID,
		Counts:    counts,
		Sections:  sections,
		Handles:   handles,
		Notes:     notes,
		Truncated: truncated,
	}
}

// ResumeAuditEvidence reopens ONLY the requested handles. An unknown or
// unauthorized handle is reported as such rather than silently ignored,
// and never causes another handle's evidence to be withheld.
func ResumeAuditEvidence(report AuditReport, handles []string) (map[string][]string, []string) {
	if len(handles) == 0 {
		return map[string][]string{}, []string{"no handles requested: empty handle list returns no evidence"}
	}
	var notes []string
	if len(handles) > maxResumeHandles {
		excess := len(handles) - maxResumeHandles
		redactor := browsersecurity.NewRedactor(nil)
		note := fmt.Sprintf("too many handles requested: capped at %d, %d excess handles ignored", maxResumeHandles, excess)
		note = redactor.SanitizeString(note)
		notes = append(notes, note)
		handles = handles[:maxResumeHandles]
	}
	result := make(map[string][]string)
	for _, h := range handles {
		parts := strings.Split(h, ":")
		if len(parts) != 3 || parts[0] != "audit" {
			notes = append(notes, fmt.Sprintf("unknown handle %q: invalid format", h))
			continue
		}
		sessPart := parts[1]
		secPart := parts[2]
		if sessPart != report.SessionID {
			notes = append(notes, fmt.Sprintf("handle %q: session mismatch: expected session %q", h, report.SessionID))
			continue
		}
		content, ok := report.Sections[secPart]
		if !ok {
			notes = append(notes, fmt.Sprintf("unknown handle %q: unknown section %q", h, secPart))
			continue
		}
		cp := make([]string, len(content))
		copy(cp, content)
		result[secPart] = cp
	}
	if notes == nil {
		notes = []string{}
	}
	if result == nil {
		result = map[string][]string{}
	}
	return result, notes
}
