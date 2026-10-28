package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"unicode/utf8"

	"codenerd/internal/diff"
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
	// identity when the turn ended, and full the post-change content an
	// expansion supplies once the file is shown to still hold it.
	path  string
	after string
	full  string
}

// EvidencePacket is one attempt's frozen evidence.
type EvidencePacket struct {
	Attempt int64
	Rubric  string
	// Snapshot is the workspace identity the attempt's evidence was bound
	// to, or empty when the producer had none.
	Snapshot string
	Items    []EvidenceItem
	// Budget is the packet's byte budget (delegation.judge_evidence_bytes).
	Budget int
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
	if it.path == "" {
		return true
	}
	return it.after != "unknown" && fileIdentity(it.path) == it.after
}

// expandable reports whether a requested item has more the host can supply,
// and still describes the file as it is.
func (p *EvidencePacket) expandable(id string) bool {
	it, ok := p.item(id)
	return ok && it.Omitted > 0 && it.full != "" && p.usable(id)
}

// expand renders the requested items in full, within the packet's budget,
// for a further judge round over the same frozen packet.
func (p *EvidencePacket) expand(ids []string) string {
	var b strings.Builder
	b.WriteString("## Requested evidence\n")
	budget := p.Budget
	for _, id := range ids {
		if !p.expandable(id) {
			continue
		}
		it, _ := p.item(id)
		body := cutUTF8(it.full, budget)
		cut := len(it.full) - len(body)
		budget -= len(body)
		fmt.Fprintf(&b, "\n[%s] post-change content of %s\n```\n%s\n```\n", id, it.Label, body)
		if cut > 0 {
			fmt.Fprintf(&b, "(%d bytes beyond the evidence budget omitted)\n", cut)
		}
	}
	return b.String()
}

// render is the packet as the judge's user prompt carries it.
func (p *EvidencePacket) render(task string) string {
	var b strings.Builder
	b.WriteString("## Task\n")
	b.WriteString(task)
	fmt.Fprintf(&b, "\n\n## Evidence packet (attempt %d, rubric %s, reply contract %s)\n", p.Attempt, p.Rubric, JudgeContract)
	b.WriteString("Each item is host-issued unless its source is \"reported\": a reported item is the worker's own claim, not evidence. Cite items by ID.\n")
	if p.Snapshot != "" {
		fmt.Fprintf(&b, "Workspace snapshot: %s\n", p.Snapshot)
	} else {
		b.WriteString("Workspace snapshot: unavailable (no closing evidence was bound to one revision)\n")
	}
	for _, it := range p.Items {
		fmt.Fprintf(&b, "\n[%s] source=%s kind=%s: %s\n", it.ID, it.Source, it.Kind, it.Label)
		if it.Body != "" {
			b.WriteString(it.Body)
			b.WriteString("\n")
		}
		if it.Omitted > 0 {
			if it.full != "" {
				fmt.Fprintf(&b, "(%d bytes omitted; request %s for the full post-change content)\n", it.Omitted, it.ID)
			} else {
				fmt.Fprintf(&b, "(%d bytes omitted)\n", it.Omitted)
			}
		}
	}
	return b.String()
}

// buildEvidencePacket freezes one attempt's evidence from its observed
// return. Nothing here is read out of the prose: the prose is one item, and
// it is labeled reported.
func buildEvidencePacket(attempt int64, rubric string, ret observation.Return, budget int) *EvidencePacket {
	p := &EvidencePacket{Attempt: attempt, Rubric: rubric, Snapshot: ret.Snapshot, Budget: budget}
	add := func(it EvidenceItem) {
		it.ID = fmt.Sprintf("E%d", len(p.Items)+1)
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

	// Each changed file, as a bounded diff of what the attempt did to it.
	// The budget is shared: the narrative takes at most a quarter, and the
	// diffs split what remains.
	remaining := budget
	narrativeCap := budget / 4
	remaining -= narrativeCap
	writes := ret.Writes
	for i, w := range writes {
		label := w.Path
		if len(ret.Changed) == len(writes) {
			label = ret.Changed[i]
		}
		share := remaining
		if left := len(writes) - i; left > 0 {
			share = remaining / left
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
		shown := cutUTF8(body, share)
		omitted := len(body) - len(shown)
		body = shown
		remaining -= len(body)
		add(EvidenceItem{
			Kind: "change", Source: sourceObserved,
			Label: fmt.Sprintf("changed %s (before %s, after %s)", label, before, after),
			Body:  body, Omitted: omitted,
			path: w.Path, after: after, full: full,
		})
	}
	if len(writes) == 0 && len(ret.Changed) > 0 {
		add(EvidenceItem{Kind: "change", Source: sourceObserved, Label: "changed paths (content not retained): " + strings.Join(ret.Changed, ", ")})
	}
	for _, f := range ret.Findings {
		label := f.Message
		if f.File != "" {
			label = fmt.Sprintf("%s:%d [%s] %s", f.File, f.Line, f.Severity, f.Message)
		}
		add(EvidenceItem{Kind: "finding", Source: sourceObserved, Label: label})
	}
	if len(ret.Untested) > 0 {
		add(EvidenceItem{Kind: "untested", Source: sourceObserved, Label: "written with no test beside them: " + strings.Join(ret.Untested, ", ")})
	}
	for _, n := range ret.Notes {
		add(EvidenceItem{Kind: "note", Source: sourceObserved, Label: n})
	}
	narrative := cutUTF8(ret.Output, narrativeCap)
	add(EvidenceItem{Kind: "narrative", Source: sourceReported, Label: "the worker's own account of the attempt", Body: narrative, Omitted: len(ret.Output) - len(narrative)})
	return p
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
