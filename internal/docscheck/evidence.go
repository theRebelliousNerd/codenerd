package docscheck

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PolicyVersion names the evidence rules in this file set. It is part of
// every scan's generation, so a change to a role cue or a resolution grade
// can never be mistaken for the same scan under older rules.
const PolicyVersion = "docs-evidence/1"

// This file is the documentation evidence view: what a document's citations
// establish, kept in the kind of evidence each one is. The structural grade
// in docscheck.go answers "is this package shaped to the R6 bar"; this view
// answers a different question and never replaces that grade.
//
// Evidence kinds produced here:
//
//   - Reference: the document names a target, in a stated role.
//   - Resolution: the target resolves, or fails to, in the observed
//     domain, at a stated grade.
//   - Manifest: what the scan covered and what it could not see.
//
// Evidence kinds this package deliberately does NOT produce, and says so in
// every manifest (Manifest.NotObserved): a relation claim (someone asserts
// that code realizes a requirement), a relation disposition (an accountable
// reviewer accepts or rejects that claim), an acceptance obligation (what
// would count as proof), and an execution receipt (a check actually ran).
// A reference that resolves is therefore never an implementation claim, and
// nothing here can be read as one.

// Role is why a document cites a target. It only ever restricts what a
// citation can later be used for; no role promotes a reference into a claim
// that the target realizes anything.
type Role string

const (
	// RoleWitness is an ADR's **Witness:** line: the author names the
	// artifact that demonstrates the decision.
	RoleWitness Role = "witness"
	// RolePlannedWitness is the witness of an accepted-not-implemented ADR.
	// The decision is accepted before its witness exists, so the target is
	// not required to resolve.
	RolePlannedWitness Role = "planned_witness"
	// RolePlannedSeam is a citation in a planned/target-state/north-star
	// document: the existing code where a future capability would attach.
	// It must resolve (R6 rule 2) and it says nothing about realization.
	RolePlannedSeam Role = "planned_seam"
	// RoleNegative is a citation in a sentence or section describing a gap,
	// a missing behavior, or an unwired path.
	RoleNegative Role = "negative"
	// RoleHistorical is a citation of something removed, replaced,
	// rejected, or superseded.
	RoleHistorical Role = "historical"
	// RoleCited is any other citation: the document refers to the target.
	RoleCited Role = "cited"
)

// exemptFromResolution reports roles whose target may legitimately not
// resolve today.
func (r Role) exemptFromResolution() bool {
	return r == RolePlannedWitness || r == RoleNegative || r == RoleHistorical
}

// Grade is how far a reference resolved in the observed domain. Grades are
// not ordered by strength across kinds; only the set below satisfies a
// required resolution (see Grade.satisfies).
type Grade string

const (
	// GradeDeclared: exactly one declaration, read from the parse tree
	// (Go) or a Decl statement (Mangle), not from text.
	GradeDeclared Grade = "declared"
	// GradeTestDeclared: exactly one test identity — a _test.go function
	// with a signature `go test` discovers. It was not executed.
	GradeTestDeclared Grade = "test_declared"
	// GradeFilePresent: the cited path exists. Cited lines are not read.
	GradeFilePresent Grade = "file_present"
	// GradeAmbiguous: more than one candidate; none is chosen.
	GradeAmbiguous Grade = "ambiguous"
	// GradeOccurrenceOnly: the spelling occurs as text (comment, string,
	// use site, rule body) but no qualifying declaration exists.
	GradeOccurrenceOnly Grade = "occurrence_only"
	// GradeUnresolved: the domain was fully observed and nothing matched.
	GradeUnresolved Grade = "unresolved"
	// GradeUnobserved: the domain that would answer was not fully
	// observed, so absence cannot be concluded.
	GradeUnobserved Grade = "unobserved"
	// GradeUnsupported: the target kind has no resolver.
	GradeUnsupported Grade = "unsupported"
)

func (g Grade) satisfies() bool {
	return g == GradeDeclared || g == GradeTestDeclared || g == GradeFilePresent
}

// Reference is one place a document names a target.
type Reference struct {
	Record  string `json:"record"` // always "reference"
	ID      string `json:"id"`
	Package string `json:"package"`
	// File is the workspace-relative document, slash separated.
	File string `json:"file"`
	Line int    `json:"line"`
	// Section is the nearest preceding heading, empty before the first.
	Section string `json:"section,omitempty"`
	Role    Role   `json:"role"`
	// Spelling is the target exactly as the document wrote it.
	Spelling string `json:"spelling"`
	// TargetKind is file, symbol, test, predicate, path, or path_symbol.
	TargetKind string `json:"target_kind"`
}

// Candidate is one entity a spelling could mean.
type Candidate struct {
	// Package is the slash-relative directory (Go) or "" (Mangle).
	Package string `json:"package,omitempty"`
	// Name is the declared spelling, without any package qualifier.
	Name string `json:"name,omitempty"`
	File string `json:"file"`
	Line int    `json:"line,omitempty"`
	Kind string `json:"kind,omitempty"`
}

// Resolution is the outcome of resolving one Reference.
type Resolution struct {
	Record string `json:"record"` // always "resolution"
	RefID  string `json:"ref_id"`
	Grade  Grade  `json:"grade"`
	// Entity is the canonical identity when exactly one candidate matched:
	// <dir>.<symbol> for Go, the predicate name for Mangle, the path for a
	// file.
	Entity string `json:"entity,omitempty"`
	// Candidates lists every match for ambiguous results and the single
	// match otherwise. It is never shortened.
	Candidates []Candidate `json:"candidates,omitempty"`
	// Constrained: the declaration sits in a file under a build
	// constraint, so it exists only in some build configurations.
	Constrained bool `json:"constrained,omitempty"`
	// Source names the resolver: go/parser, mangle-decl, stat, text.
	Source string `json:"source"`
	Detail string `json:"detail,omitempty"`
}

// Deficit is something the scan could not observe.
type Deficit struct {
	Code   string `json:"code"`
	File   string `json:"file,omitempty"`
	Detail string `json:"detail"`
}

// Terminal states of a scan.
const (
	// TerminalComplete: every requested document was read and every
	// resolver domain it needed was fully observed.
	TerminalComplete = "complete"
	// TerminalIncomplete: some part of the domain could not be observed.
	// Absence of findings proves nothing.
	TerminalIncomplete = "incomplete"
	// TerminalEmpty: the requested domain held no documents. This is
	// unknown, not clean.
	TerminalEmpty = "empty"
)

// Scope states whether a result covers the whole architecture tree.
const (
	ScopeWorkspace = "workspace"
	ScopeSubset    = "subset"
)

// Manifest is what a scan covered. Every consumer of the records must carry
// it with them: a result without its manifest does not say what it is about.
type Manifest struct {
	Record string `json:"record"` // always "manifest"
	Policy string `json:"policy"`
	// Generation identifies this scan's inputs: policy, requested domain,
	// and a digest of every document and every source file read. It changes
	// with any byte of any input, so a dirty worktree and a commit are
	// distinguished by content, not by a revision name.
	Generation string `json:"generation"`
	DocDigest  string `json:"doc_digest"`
	// CodeDigest is empty when no code or policy file was read.
	CodeDigest string `json:"code_digest"`
	Scope      string `json:"scope"`
	// Requested is the package list asked for, or every package when none
	// was named; Observed is the packages whose documents were read.
	Requested []string `json:"requested"`
	Observed  []string `json:"observed"`
	Documents int      `json:"documents"`
	// CodeFilesParsed counts tracked Go and Mangle files read to resolve
	// references; CodeFilesFailed counts the Go files that did not parse.
	CodeFilesParsed int           `json:"code_files_parsed"`
	CodeFilesFailed int           `json:"code_files_failed"`
	References      int           `json:"references"`
	ByGrade         map[Grade]int `json:"by_grade"`
	// Failing counts references that needed a satisfying grade and did not
	// reach one; exempt roles are not counted.
	Failing  int       `json:"failing"`
	Deficits []Deficit `json:"deficits"`
	Terminal string    `json:"terminal"`
	// NotObserved lists the evidence families this scan never produces.
	NotObserved []string `json:"not_observed"`
}

// notObserved is the fixed list of evidence families outside this view.
var notObserved = []string{
	"relation_claim", "relation_disposition", "acceptance_obligation", "execution_receipt",
}

// Evidence is one scan: records in document order, then the manifest.
type Evidence struct {
	References  []Reference
	Resolutions []Resolution
	Manifest    Manifest
}

// Clean reports whether the scan may be read as "nothing is wrong in this
// domain": it completed and nothing needed a grade it lacks. A scan in any
// other terminal state is never clean, however few findings it holds.
func (e Evidence) Clean() bool {
	return e.Manifest.Terminal == TerminalComplete && e.Manifest.Failing == 0
}

// Evidence reads the named packages' documents (every package when none is
// named) and returns the evidence view. It reads documents and tracked
// source; it runs nothing. A package that does not exist is an error, as in
// Check: a request that cannot be read must not print a clean bill.
func (c *Checker) Evidence(pkgs []string) (Evidence, error) {
	scope := ScopeSubset
	all, err := c.ListPackages()
	if err != nil {
		return Evidence{}, err
	}
	requested := pkgs
	if len(requested) == 0 {
		requested = all
		scope = ScopeWorkspace
	} else {
		requested = append([]string(nil), requested...)
	}

	var ev Evidence
	m := Manifest{
		Record: "manifest", Policy: PolicyVersion, Scope: scope,
		Requested: requested, Observed: []string{},
		ByGrade: map[Grade]int{}, Deficits: []Deficit{}, NotObserved: notObserved,
	}
	docHash := sha256.New()
	res := newResolver(c)

	for _, pkg := range requested {
		rels, err := c.packageDocs(pkg)
		if err != nil {
			return Evidence{}, err
		}
		m.Observed = append(m.Observed, pkg)
		for _, rel := range rels {
			file := archRel(pkg, rel)
			data, rerr := os.ReadFile(filepath.Join(c.ArchDir(), pkg, filepath.FromSlash(rel)))
			if rerr != nil {
				m.Deficits = append(m.Deficits, Deficit{Code: "document_unreadable", File: file, Detail: rerr.Error()})
				continue
			}
			m.Documents++
			fmt.Fprintf(docHash, "%s\x00%x\x00", file, sha256.Sum256(data))
			refs := extractReferences(pkg, file, normalizeText(string(data)))
			for _, r := range refs {
				rs := res.resolve(r)
				ev.References = append(ev.References, r)
				ev.Resolutions = append(ev.Resolutions, rs)
				m.ByGrade[rs.Grade]++
				if !r.Role.exemptFromResolution() && !rs.Grade.satisfies() {
					m.Failing++
				}
			}
		}
	}

	m.References = len(ev.References)
	m.DocDigest = hex.EncodeToString(docHash.Sum(nil))
	code := res.report()
	m.CodeDigest, m.CodeFilesParsed, m.CodeFilesFailed = code.digest, code.parsed, code.failed
	m.Deficits = append(m.Deficits, code.deficits...)
	sort.SliceStable(m.Deficits, func(i, j int) bool {
		if m.Deficits[i].Code != m.Deficits[j].Code {
			return m.Deficits[i].Code < m.Deficits[j].Code
		}
		return m.Deficits[i].File < m.Deficits[j].File
	})
	switch {
	case m.Documents == 0:
		m.Terminal = TerminalEmpty
	case len(m.Deficits) > 0:
		m.Terminal = TerminalIncomplete
	default:
		m.Terminal = TerminalComplete
	}
	gen := sha256.New()
	fmt.Fprintf(gen, "%s\x00%s\x00%s\x00%s\x00%s", PolicyVersion, scope, strings.Join(requested, ","), m.DocDigest, m.CodeDigest)
	m.Generation = hex.EncodeToString(gen.Sum(nil))
	ev.Manifest = m
	return ev, nil
}
