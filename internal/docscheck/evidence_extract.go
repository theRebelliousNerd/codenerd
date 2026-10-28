package docscheck

import (
	"fmt"
	"regexp"
	"strings"
)

// Citation grammar the evidence view reads. It is deliberately smaller than
// cmd/tools/audit_doc_citations: that tool grades whether every citation in
// prose is broken (paths, line ranges, bound symbols); this view records what
// a few unambiguous citation forms establish and at what grade.
//
//   - an ADR's **Witness:** line, in the existing kind:value form;
//   - a backticked repository path under internal/, cmd/ or pkg/ ending in
//     .go or .mg, with an optional :line or :line-line suffix and an
//     optional #Symbol suffix.
//
// Fenced code blocks are skipped: an example is not a citation.
var pathCiteRe = regexp.MustCompile("`((?:internal|cmd|pkg)/[A-Za-z0-9_./-]+\\.(?:go|mg))(:[0-9]+(?:-[0-9]+)?)?(#[A-Za-z_][A-Za-z0-9_.]*)?`")

// Role cues. They are text cues on the citing line or the nearest heading,
// they are deterministic, and they can only move a reference into a more
// restricted role: an unrecognised sentence stays "cited" or "planned_seam"
// and no cue ever makes a reference mean more.
var (
	negativeCue = regexp.MustCompile(`(?i)\b(?:not\s+(?:yet\s+)?(?:implemented|wired|built|shipped|exist(?:s|ing)?)|unimplemented|unwired|absent|missing|does\s+not\s+exist|no\s+production\s+caller|known\s+gaps?|has\s+no\s+caller|dark\s+field|not\s+reachable)\b`)
	historyCue  = regexp.MustCompile(`(?i)\b(?:removed|deleted|deprecated|superseded|rejected|formerly|previously|obsolete|retired|historical|legacy|no\s+longer|was\s+replaced|replaced\s+by)\b`)
)

// extractReferences returns the references in one document, in line order.
func extractReferences(pkg, file, text string) []Reference {
	fm, hasFM := frontMatter(text)
	status, class := fm["implementation-status"], fm["doc-class"]
	planned := hasFM && (planStatuses[status] || class == "north-star")
	isADR := strings.Contains(file, "/adr/")

	lines := strings.Split(text, "\n")
	start := 0
	if hasFM {
		// frontMatter accepts "---" ... "\n---"; skip through the closing line.
		for i := 1; i < len(lines); i++ {
			if strings.HasPrefix(lines[i], "---") {
				start = i + 1
				break
			}
		}
	}

	var refs []Reference
	section := ""
	fenced := false
	seen := map[string]bool{}
	add := func(line int, spelling, kind string, role Role) {
		key := fmt.Sprintf("%d\x00%s", line, spelling)
		if seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, Reference{
			Record: "reference", ID: fmt.Sprintf("%s:%d:%d", file, line, len(refs)+1),
			Package: pkg, File: file, Line: line, Section: section,
			Role: role, Spelling: spelling, TargetKind: kind,
		})
	}
	for i := start; i < len(lines); i++ {
		line := lines[i]
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if strings.HasPrefix(trim, "#") {
			section = strings.TrimSpace(strings.TrimLeft(trim, "#"))
			continue
		}
		if isADR {
			if m := witnessLineRe.FindStringSubmatch(line); m != nil {
				role := RoleWitness
				if status == "accepted-not-implemented" {
					role = RolePlannedWitness
				}
				for _, w := range witnessTargets(m[1]) {
					add(i+1, w.spelling, w.kind, role)
				}
				continue
			}
		}
		for _, m := range pathCiteRe.FindAllStringSubmatch(line, -1) {
			spelling := m[1] + m[2] + m[3]
			kind := "path"
			if m[3] != "" {
				kind = "path_symbol"
			}
			add(i+1, spelling, kind, bodyRole(line, section, planned))
		}
	}
	return refs
}

// bodyRole classifies a body citation. Line cues outrank heading cues,
// which outrank the document's own layer: a planned document's sentence that
// says a path is missing is a negative citation, not a seam.
func bodyRole(line, section string, planned bool) Role {
	switch {
	case negativeCue.MatchString(line):
		return RoleNegative
	case historyCue.MatchString(line):
		return RoleHistorical
	case negativeCue.MatchString(section):
		return RoleNegative
	case historyCue.MatchString(section):
		return RoleHistorical
	case planned:
		return RolePlannedSeam
	}
	return RoleCited
}

// witnessForm finds each kind:value witness in an ADR witness line. ADRs
// name several (`test:A` and `symbol:b.C`), sometimes in backticks.
var witnessForm = regexp.MustCompile("(?i)\\b(test|symbol|predicate|file):([^\\s`,;]+)")

type witnessRef struct{ spelling, kind string }

// witnessTargets splits a witness value into its witnesses. A value with no
// recognised kind:value form is one unsupported witness holding the whole
// text, so it is reported rather than dropped.
func witnessTargets(v string) []witnessRef {
	var out []witnessRef
	for _, m := range witnessForm.FindAllStringSubmatch(v, -1) {
		out = append(out, witnessRef{spelling: strings.TrimRight(m[2], ".)"), kind: strings.ToLower(m[1])})
	}
	if len(out) == 0 {
		out = append(out, witnessRef{spelling: strings.TrimSpace(v), kind: "unsupported"})
	}
	return out
}
