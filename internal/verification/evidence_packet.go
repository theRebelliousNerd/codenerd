package verification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"unicode/utf8"

	"codenerd/internal/diff"
	"codenerd/internal/evidence"
	"codenerd/internal/observation"
	"codenerd/internal/session"
)

// The evidence a judge is shown (Docs/audits/verification-evidence-and-judge-
// contracts.md, Gap B). The judge used to read the task and the attempt's
// prose -- with the host's one-line evidence summary glued onto it -- while the
// execution boundary already carried the kernel's verdict, the change stage,
// the acceptance report, the observed checks, the changed paths with their
// before and after content, the critic's findings and the untested paths, all
// typed and all dropped. The packet hands them over labeled by provenance, so
// "the worker says its tests pass" and "the host watched them pass" cannot be
// read as the same claim, and every item has an ID a judgment must cite.
//
// delegation.judge_evidence_bytes bounds the packet as the judge reads it --
// headers, labels, omission markers and bodies alike -- not only its bodies
// (Docs/audits/verification-applicability-and-progressive-evidence.md, V3).
// Labels are never cut: a packet whose labels alone overflow the budget is
// refused, and the judge is unavailable. Bodies share what the labels leave.

// Evidence sources. Everything but SourceReported is host-issued.
const (
	sourceKernel   = "kernel"
	sourceObserved = observation.SourceObserved
	sourceReported = observation.SourceReported
)

// EvidenceItem is one labeled piece of the packet.
type EvidenceItem struct {
	ID     string
	Kind   string
	Source string
	Label  string
	Body   string
	// Omitted is how many bytes of the item the default packet left out.
	Omitted int

	// A change item is backed by a file: path is where it lives, after its
	// identity when the turn ended.
	path  string
	after string
	// full is what an expansion discloses, range by range, when the packet
	// shows less: a change's post-change content (once the file is shown to
	// still hold it), or the whole of a cut body. rest names it to the judge.
	full string
	rest string
	// disclosed is how many bytes of full the judge has already been shown:
	// a cut body's prefix, then each expansion's range. An expansion starts
	// here, so no range is shown twice.
	disclosed int
}

// EvidencePacket is one attempt's frozen evidence.
type EvidencePacket struct {
	Attempt int64
	Rubric  string
	// Snapshot is the workspace identity the attempt's evidence was bound
	// to, or empty when the producer had none.
	Snapshot string
	Items    []EvidenceItem
	// Budget is the packet's byte budget (delegation.judge_evidence_bytes):
	// the rendered packet, and each round's disclosure.
	Budget int

	// workspace is the tree the attempt worked in; baseline the identity
	// its evidence is held against: Snapshot, or -- when no closing evidence
	// was bound to one revision -- the tree as it was when the packet was
	// frozen. Empty when no identity could be had (applicability.go).
	workspace string
	baseline  string

	// Applicability is the host's last measurement of whether this evidence
	// still describes the workspace, and ApplicabilityWhy what it found
	// (applicability.go). Empty before the first judge round.
	Applicability    Applicability
	ApplicabilityWhy string
}

func (p *EvidencePacket) item(id string) (*EvidenceItem, bool) {
	if p == nil {
		return nil, false
	}
	for i := range p.Items {
		if p.Items[i].ID == id {
			return &p.Items[i], true
		}
	}
	return nil, false
}

// usable reports whether an item can ground a judgment now: it is in the
// packet, and a file-backed item's file still holds what the attempt left. A
// citation of a superseded revision describes a workspace that no longer
// exists; it is stale, not a defect.
func (p *EvidencePacket) usable(id string) bool {
	it, ok := p.item(id)
	if !ok {
		return false
	}
	return it.current()
}

// current reports whether a file-backed item's file still holds what the
// attempt left; an item backed by no file is always current.
func (it *EvidenceItem) current() bool {
	if it.path == "" {
		return true
	}
	return it.after != "unknown" && fileIdentity(it.path) == it.after
}

// header is the packet's framing between the task and the first item. It is
// charged to the budget; the task is not (it is the request itself).
func (p *EvidencePacket) header() string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n\n## Evidence packet (attempt %d, rubric %s, reply contract %s)\n", p.Attempt, p.Rubric, JudgeContract)
	b.WriteString("Each item is host-issued unless its source is \"reported\": a reported item is the worker's own claim, not evidence. Cite items by ID.\n")
	if p.Snapshot != "" {
		fmt.Fprintf(&b, "Workspace snapshot: %s\n", p.Snapshot)
	} else {
		b.WriteString("Workspace snapshot: unavailable (no closing evidence was bound to one revision)\n")
	}
	return b.String()
}

// head is an item's identity line, never cut.
func (it *EvidenceItem) head() string {
	return fmt.Sprintf("\n[%s] source=%s kind=%s: %s\n", it.ID, it.Source, it.Kind, it.Label)
}

// marker says how much of the item the packet left out, and how to get it.
func (it *EvidenceItem) marker(omitted int, expandable bool) string {
	if expandable {
		return fmt.Sprintf("(%d bytes omitted; request %s for %s)\n", omitted, it.ID, it.rest)
	}
	return fmt.Sprintf("(%d bytes omitted)\n", omitted)
}

// render is the packet as the judge's user prompt carries it.
func (p *EvidencePacket) render(task string) string {
	var b strings.Builder
	b.WriteString("## Task\n")
	b.WriteString(task)
	b.WriteString(p.header())
	for i := range p.Items {
		it := &p.Items[i]
		b.WriteString(it.head())
		if it.Body != "" {
			b.WriteString(it.Body)
			b.WriteString("\n")
		}
		if it.Omitted > 0 {
			b.WriteString(it.marker(it.Omitted, it.full != ""))
		}
	}
	return b.String()
}

// buildEvidencePacket freezes one attempt's evidence from its observed
// return. Nothing here is read out of the prose: the prose is one item, and
// it is labeled reported. When the attempt bound no snapshot, the workspace is
// measured now, so what the judge reads can still be held against the tree
// its judgment is applied to.
func buildEvidencePacket(ctx context.Context, attempt int64, rubric string, ret observation.Return, budget int) (*EvidencePacket, error) {
	p := &EvidencePacket{Attempt: attempt, Rubric: rubric, Snapshot: ret.Snapshot, Budget: budget, workspace: ret.Workspace, baseline: ret.Snapshot}
	if p.baseline == "" && p.workspace != "" {
		if snap, err := evidence.Snapshot(ctx, p.workspace); err == nil {
			p.baseline = snap
		}
	}
	add := func(it EvidenceItem) {
		it.ID = fmt.Sprintf("E%d", len(p.Items)+1)
		if it.rest == "" {
			it.rest = "the rest"
		}
		p.Items = append(p.Items, it)
	}

	verdict := ret.Outcome
	if verdict == "" {
		verdict = "none"
	}
	add(EvidenceItem{Kind: "verdict", Source: sourceKernel, Label: "turn verdict " + verdict})
	if len(ret.Missing) > 0 {
		add(EvidenceItem{Kind: "missing", Source: sourceKernel, Label: "missing evidence: " + session.DescribeMissingEvidence(ret.Missing)})
	}
	if ret.Stage != "" {
		add(EvidenceItem{Kind: "stage", Source: sourceObserved, Label: "change stage " + ret.Stage})
	}
	if a := ret.Acceptance; a != nil {
		add(EvidenceItem{Kind: "acceptance", Source: sourceObserved, Label: fmt.Sprintf("acceptance contract %s: %s", a.Contract, a.Status), Body: a.Summary})
	}
	for _, v := range []*observation.Verification{ret.Build, ret.Tests} {
		if v == nil {
			continue
		}
		label := fmt.Sprintf("%s check: ran=%t ok=%t outcome=%s", v.Kind, v.Ran, v.OK, v.Outcome)
		if v.Passed > 0 || v.Failed > 0 {
			label += fmt.Sprintf(" passed=%d failed=%d", v.Passed, v.Failed)
		}
		body := v.Detail
		if len(v.FailedNames) > 0 {
			body = strings.TrimSpace(body + "\nfailing: " + strings.Join(v.FailedNames, ", "))
		}
		add(EvidenceItem{Kind: "check", Source: v.Source, Label: label, Body: body})
	}

	// Each changed file, as a diff of what the attempt did to it; when the
	// diff is cut, the post-change content is retained for expansion.
	for i, w := range ret.Writes {
		label := w.Path
		if len(ret.Changed) == len(ret.Writes) {
			label = ret.Changed[i]
		}
		before, after := fileStateLabel(w.Before), fileStateLabel(w.After)
		body, full := "", ""
		switch {
		case !w.After.Known:
			body = "(what the attempt left could not be read)"
		case !w.After.Exists:
			body = "(the attempt deleted the file)"
		default:
			full = w.After.Content
			body = renderDiff(label, w.Before.Content, w.After.Content)
		}
		it := EvidenceItem{
			Kind: "change", Source: sourceObserved,
			Label: fmt.Sprintf("changed %s (before %s, after %s)", label, before, after),
			Body:  body,
			path:  w.Path, after: after, full: full,
		}
		if full != "" {
			it.rest = "the full post-change content"
		}
		add(it)
	}
	if len(ret.Writes) == 0 && len(ret.Changed) > 0 {
		add(EvidenceItem{Kind: "change", Source: sourceObserved, Label: fmt.Sprintf("changed paths, content not retained (%d)", len(ret.Changed)), Body: strings.Join(ret.Changed, "\n")})
	}
	for _, f := range ret.Findings {
		label := "finding"
		if f.File != "" {
			label = fmt.Sprintf("%s:%d [%s]", f.File, f.Line, f.Severity)
		}
		add(EvidenceItem{Kind: "finding", Source: sourceObserved, Label: label, Body: f.Message})
	}
	if len(ret.Untested) > 0 {
		add(EvidenceItem{Kind: "untested", Source: sourceObserved, Label: fmt.Sprintf("written with no test beside them (%d)", len(ret.Untested)), Body: strings.Join(ret.Untested, "\n")})
	}
	for _, n := range ret.Notes {
		add(EvidenceItem{Kind: "note", Source: sourceObserved, Label: "note", Body: n})
	}
	add(EvidenceItem{Kind: "narrative", Source: sourceReported, Label: "the worker's own account of the attempt", Body: ret.Output})
	if err := p.admit(); err != nil {
		return p, err
	}
	return p, nil
}

// markerReserve is the most an omission marker can cost an item: the widest
// count, the expansion offer.
func (it *EvidenceItem) markerReserve() int {
	return len(it.marker(1<<62, true))
}

// admit fits the rendered packet into the budget. Every header, label and
// omission marker is charged first and is never cut; the bodies share what
// remains, the narrative at most a quarter of it, each body taking no more
// than it needs so a short one leaves its share to the others. A cut body
// keeps what it left out for expansion.
func (p *EvidencePacket) admit() error {
	fixed := len(p.header())
	var bodies []int
	for i := range p.Items {
		it := &p.Items[i]
		fixed += len(it.head())
		if it.Body != "" {
			fixed += it.markerReserve()
			bodies = append(bodies, i)
		}
	}
	if fixed > p.Budget {
		return fmt.Errorf("the evidence packet's %d items need %d bytes of labels and markers before any body, over delegation.judge_evidence_bytes (%d); no label is cut", len(p.Items), fixed, p.Budget)
	}
	room := p.Budget - fixed
	demand := make([]int, len(bodies))
	for k, i := range bodies {
		demand[k] = len(p.Items[i].Body) + 1 // the body's newline
		if p.Items[i].Kind == "narrative" {
			demand[k] = min(demand[k], room/4)
		}
	}
	alloc := shareFairly(demand, room)
	for k, i := range bodies {
		it := &p.Items[i]
		whole := it.Body
		shown := cutUTF8(whole, alloc[k]-1)
		it.Body = shown
		it.Omitted = len(whole) - len(shown)
		switch {
		case it.Omitted == 0:
			// The whole body is shown: a change's content adds nothing to
			// its whole diff.
			it.full = ""
		case it.full == "":
			it.full, it.disclosed = whole, len(shown)
		}
	}
	return nil
}

// shareFairly divides room among demands: each gets at most what it asks,
// and no more than an equal share of what the smaller demands left.
func shareFairly(demand []int, room int) []int {
	alloc := make([]int, len(demand))
	order := make([]int, len(demand))
	for i := range order {
		order[i] = i
	}
	// Smallest demand first, so what a short body leaves goes to the rest.
	for i := 1; i < len(order); i++ {
		for j := i; j > 0 && demand[order[j]] < demand[order[j-1]]; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	for k, i := range order {
		share := room / (len(order) - k)
		alloc[i] = min(demand[i], share)
		room -= alloc[i]
	}
	return alloc
}

// cutUTF8 is s cut to at most n bytes, on a rune boundary.
func cutUTF8(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// renderDiff is a unified diff of one file's change.
func renderDiff(path, before, after string) string {
	fd := diff.ComputeDiff(path, path, before, after)
	if fd == nil {
		return ""
	}
	if fd.IsBinary {
		return "(binary content)"
	}
	var b strings.Builder
	for _, h := range fd.Hunks {
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
		for _, l := range h.Lines {
			switch l.Type {
			case diff.LineAdded:
				b.WriteString("+")
			case diff.LineRemoved:
				b.WriteString("-")
			default:
				b.WriteString(" ")
			}
			b.WriteString(strings.TrimSuffix(l.Content, "\n"))
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// fileStateLabel is a file state's identity: its digest, absent, or
// unknown. The same spelling fileIdentity reads off disk.
func fileStateLabel(s observation.FileState) string {
	switch {
	case !s.Known:
		return "unknown"
	case !s.Exists:
		return "absent"
	default:
		sum := sha256.Sum256([]byte(s.Content))
		return "sha256:" + hex.EncodeToString(sum[:])
	}
}

// fileIdentity is a path's identity on disk now.
func fileIdentity(path string) string {
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		sum := sha256.Sum256(data)
		return "sha256:" + hex.EncodeToString(sum[:])
	case errors.Is(err, fs.ErrNotExist):
		return "absent"
	default:
		return "unknown"
	}
}
