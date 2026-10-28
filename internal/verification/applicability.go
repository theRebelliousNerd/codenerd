package verification

import (
	"context"
	"fmt"

	"codenerd/internal/evidence"
)

// Applicability (Docs/audits/verification-applicability-and-progressive-
// evidence.md, V1). A judge call can take minutes, and the workspace is not
// frozen while it runs. Grounding checked freshness only for the files a
// judgment cited, and a pass cites nothing, so a pass over a revision another
// writer had since replaced could become an accepted delegation -- shown to
// the user as Accepted for a tree the evidence no longer described. After
// every judge round the host now measures whether the packet still describes
// the workspace, whatever the judge said, and asserts that measurement
// (judge_evidence_applicability); policy/delegation.mg accepts only evidence
// that is current, and holds superseded or unknown evidence as historical:
// never a pass, never a worker retry.

// Applicability is whether an attempt's evidence still describes the
// workspace a disposition would be applied to. The host measures it; no model
// field can assert it.
type Applicability string

const (
	// ApplicabilityCurrent: the workspace is the one the evidence was bound
	// to, and every changed file still holds what the attempt left.
	ApplicabilityCurrent Applicability = "current"
	// ApplicabilitySuperseded: the workspace or a changed file has changed
	// since; the evidence describes the attempt as it was.
	ApplicabilitySuperseded Applicability = "superseded"
	// ApplicabilityUnknown: no identity was bound to the evidence, or the
	// workspace could not be measured.
	ApplicabilityUnknown Applicability = "unknown"
)

// Atom is the applicability as judge_evidence_applicability carries it.
func (a Applicability) Atom() string { return "/" + string(a) }

// measureApplicability measures, now, whether the packet still describes the
// workspace, and records it on the packet. A changed file is checked by its
// own identity; the rest of the tree -- unchanged dependencies,
// configuration, tests the attempt did not write -- by the workspace
// snapshot the evidence was bound to.
func (p *EvidencePacket) measureApplicability(ctx context.Context) Applicability {
	p.Applicability, p.ApplicabilityWhy = p.applicability(ctx)
	return p.Applicability
}

func (p *EvidencePacket) applicability(ctx context.Context) (Applicability, string) {
	var unreadable string
	for i := range p.Items {
		it := &p.Items[i]
		if it.path == "" {
			continue
		}
		if it.after == "unknown" {
			unreadable = it.path
			continue
		}
		if now := fileIdentity(it.path); now != it.after {
			return ApplicabilitySuperseded, fmt.Sprintf("%s changed after the attempt (left %s, now %s)", it.path, it.after, now)
		}
	}
	if unreadable != "" {
		return ApplicabilityUnknown, fmt.Sprintf("what the attempt left in %s could not be read", unreadable)
	}
	switch {
	case p.workspace == "":
		return ApplicabilityUnknown, "the attempt named no workspace to measure"
	case p.baseline == "":
		return ApplicabilityUnknown, "no workspace snapshot was bound to the evidence"
	}
	now, err := evidence.Snapshot(ctx, p.workspace)
	if err != nil {
		return ApplicabilityUnknown, fmt.Sprintf("the workspace could not be measured: %v", err)
	}
	if now != p.baseline {
		return ApplicabilitySuperseded, fmt.Sprintf("the workspace changed since the evidence was bound (snapshot %s, now %s)", p.baseline, now)
	}
	return ApplicabilityCurrent, fmt.Sprintf("the workspace still matches snapshot %s", p.baseline)
}
