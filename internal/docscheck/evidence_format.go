package docscheck

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// maxCandidatesShown bounds how many candidates the text report names on one
// line, to keep a line readable. The count is printed with the remainder and
// the JSON form lists every candidate.
const maxCandidatesShown = 4

// EvidenceText renders the evidence view for a person: one line per
// reference with its role and grade, the deficits, and the manifest that
// scopes the whole report. The manifest is printed even when there are no
// references, because an empty list is only meaningful with its scope.
func EvidenceText(ev Evidence) string {
	var b strings.Builder
	for i, ref := range ev.References {
		rs := ev.Resolutions[i]
		fmt.Fprintf(&b, "%s:%d  %-15s %-12s %s -> %s", ref.File, ref.Line, ref.Role, ref.TargetKind, ref.Spelling, rs.Grade)
		if rs.Entity != "" {
			fmt.Fprintf(&b, " %s", rs.Entity)
		}
		if rs.Constrained {
			b.WriteString(" [build-constrained]")
		}
		if rs.Grade == GradeAmbiguous || rs.Grade == GradeUnresolved || rs.Grade == GradeOccurrenceOnly || rs.Grade == GradeUnobserved || rs.Grade == GradeUnsupported {
			if rs.Detail != "" {
				fmt.Fprintf(&b, " (%s)", rs.Detail)
			}
		}
		if rs.Grade == GradeAmbiguous {
			b.WriteString("\n")
			shown := rs.Candidates
			if len(shown) > maxCandidatesShown {
				shown = shown[:maxCandidatesShown]
			}
			for _, c := range shown {
				fmt.Fprintf(&b, "    candidate %s:%d\n", c.File, c.Line)
			}
			if extra := len(rs.Candidates) - len(shown); extra > 0 {
				fmt.Fprintf(&b, "    ... %d more candidates (use --json for all)\n", extra)
			}
			continue
		}
		b.WriteString("\n")
	}
	m := ev.Manifest
	for _, d := range m.Deficits {
		fmt.Fprintf(&b, "deficit %s %s %s\n", d.Code, d.File, d.Detail)
	}
	grades := make([]string, 0, len(m.ByGrade))
	for g, n := range m.ByGrade {
		grades = append(grades, fmt.Sprintf("%s=%d", g, n))
	}
	sort.Strings(grades)
	fmt.Fprintf(&b, "manifest: status=%s scope=%s policy=%s generation=%s\n", m.Terminal, m.Scope, m.Policy, m.Generation)
	fmt.Fprintf(&b, "  domain: Docs/architecture packages=%s observed=%d documents=%d code_files=%d (parse failures %d)\n",
		strings.Join(m.Requested, ","), len(m.Observed), m.Documents, m.CodeFilesParsed, m.CodeFilesFailed)
	fmt.Fprintf(&b, "  references=%d failing=%d grades: %s\n", m.References, m.Failing, strings.Join(grades, " "))
	fmt.Fprintf(&b, "  not observed by this view: %s\n", strings.Join(m.NotObserved, " "))
	switch m.Terminal {
	case TerminalEmpty:
		b.WriteString("  result: UNKNOWN - the requested domain held no documents; nothing here is a clean bill\n")
	case TerminalIncomplete:
		b.WriteString("  result: INCOMPLETE - some of the domain was not observed; absence of findings proves nothing\n")
	default:
		if m.Failing == 0 {
			fmt.Fprintf(&b, "  result: every required reference resolved within this %s scope\n", m.Scope)
		} else {
			fmt.Fprintf(&b, "  result: %d required references did not reach a satisfying grade\n", m.Failing)
		}
	}
	return b.String()
}

// WriteEvidenceJSON writes one JSON object per line: each reference
// immediately followed by its resolution, then the manifest. Every object
// carries a "record" key naming its kind.
func WriteEvidenceJSON(w io.Writer, ev Evidence) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for i := range ev.References {
		if err := enc.Encode(ev.References[i]); err != nil {
			return fmt.Errorf("encode doc reference: %w", err)
		}
		if err := enc.Encode(ev.Resolutions[i]); err != nil {
			return fmt.Errorf("encode doc resolution: %w", err)
		}
	}
	if err := enc.Encode(ev.Manifest); err != nil {
		return fmt.Errorf("encode doc manifest: %w", err)
	}
	return nil
}
