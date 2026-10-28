package retrieval

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/types"
)

// =============================================================================
// RETRIEVAL PROVENANCE
// =============================================================================
//
// A pass that failed halfway used to look exactly like one that finished:
// SearchKeywords logged a keyword's error and returned the hits it had with a
// nil error, and the seed report said "embeddings" whenever an engine was
// configured, whatever backend actually furnished Tier 4. A short brief could
// mean "nothing else matches" or "the scan timed out"; nobody downstream could
// tell. Each tier now reports what it searched and how that went, the pass
// asserts it as section-52.6 facts, and policy decides what the model is told
// (retrieval_brief_caveat, policy/retrieval.mg).

// RetrievalStatus says whether a tier searched its whole domain. An empty
// result is a count, not a status.
type RetrievalStatus string

const (
	// StatusComplete: the tier searched everything it is defined over.
	StatusComplete RetrievalStatus = "complete"
	// StatusPartial: its results are true, but part of the domain went
	// unsearched (or was searched by a fallback).
	StatusPartial RetrievalStatus = "partial"
	// StatusUnavailable: it searched nothing.
	StatusUnavailable RetrievalStatus = "unavailable"
)

// Retrieval tiers that report an outcome.
const (
	TierKeyword  = "keyword"
	TierSemantic = "semantic"
)

// Reasons a tier gives for its status. "none" is a complete tier's.
const (
	ReasonNone            = "none"
	ReasonTimeout         = "timeout"
	ReasonCanceled        = "canceled"
	ReasonBackendError    = "backend_error"
	ReasonMembership      = "membership_error"
	ReasonOverflow        = "overflow"
	ReasonBoundedCorpus   = "bounded_corpus"
	ReasonEmbedIncomplete = "embed_incomplete"
	ReasonNoSemanticHits  = "no_semantic_hits"
)

// Backends a tier can attempt or use.
const (
	BackendNative         = "native"
	BackendRipgrep        = "ripgrep"
	BackendEmbeddings     = "embeddings"
	BackendDefinitionScan = "definition_scan"
)

// TierOutcome is the provenance of one tier's part of a pass.
type TierOutcome struct {
	Tier   string
	Status RetrievalStatus
	Reason string
	// Attempted is the backend the tier was configured to use; Used is the
	// one whose results it kept (a fallback when they differ).
	Attempted string
	Used      string
	// Eligible and Covered are the files the tier's domain holds and the
	// ones it actually searched; both zero when the tier does not measure
	// them (the keyword walk visits every member).
	Eligible int
	Covered  int
	// Queries counts the keywords asked; CacheHits those answered from the
	// keyword cache; Incomplete names the ones whose result is not whole
	// (failed or overflowed) -- the bounded next operation is to search them
	// again or with a narrower term.
	Queries    int
	CacheHits  int
	Incomplete []string
	// Detail is the error text, for the log and `nerd retrieve`.
	Detail string
}

// String renders the outcome as one log/CLI phrase.
func (o TierOutcome) String() string {
	s := fmt.Sprintf("%s %s", o.Tier, o.Status)
	if o.Reason != "" && o.Reason != ReasonNone {
		s += " (" + o.Reason + ")"
	}
	if o.Used != "" && o.Used != o.Attempted {
		s += fmt.Sprintf(" via %s, %s attempted", o.Used, o.Attempted)
	} else if o.Used != "" {
		s += " via " + o.Used
	}
	if o.Eligible > 0 {
		s += fmt.Sprintf(", %d of %d files covered", o.Covered, o.Eligible)
	}
	if len(o.Incomplete) > 0 {
		s += fmt.Sprintf(", %d of %d keywords incomplete", len(o.Incomplete), o.Queries)
	}
	return s
}

// errorReason classifies a search error as a provenance reason.
func errorReason(err error) string {
	switch {
	case err == nil:
		return ReasonNone
	case errors.Is(err, context.DeadlineExceeded):
		return ReasonTimeout
	case errors.Is(err, context.Canceled):
		return ReasonCanceled
	case errors.Is(err, errMembership):
		return ReasonMembership
	default:
		return ReasonBackendError
	}
}

// OutcomeFacts asserts each tier's provenance (schemas_knowledge.mg 52.6):
// retrieval_outcome/4, retrieval_backend/4 and, where the tier measured its
// domain, retrieval_coverage/4. All are keyed by the issue, so a pass that
// runs beside another retracts only its own.
func OutcomeFacts(issueID string, outcomes []TierOutcome) []types.Fact {
	id := types.MangleString(issueID)
	var facts []types.Fact
	for _, o := range outcomes {
		if o.Tier == "" || o.Status == "" {
			continue
		}
		tier := types.MangleAtom("/" + o.Tier)
		reason := o.Reason
		if reason == "" {
			reason = ReasonNone
		}
		facts = append(facts, types.Fact{
			Predicate: "retrieval_outcome",
			Args:      []any{id, tier, types.MangleAtom("/" + string(o.Status)), types.MangleAtom("/" + reason)},
		})
		if o.Attempted != "" && o.Used != "" {
			facts = append(facts, types.Fact{
				Predicate: "retrieval_backend",
				Args:      []any{id, tier, types.MangleAtom("/" + o.Attempted), types.MangleAtom("/" + o.Used)},
			})
		}
		if o.Eligible > 0 {
			facts = append(facts, types.Fact{
				Predicate: "retrieval_coverage",
				Args:      []any{id, tier, int64(o.Eligible), int64(o.Covered)},
			})
		}
	}
	return facts
}

// BriefCaveat is one retrieval_brief_caveat row: a tier whose shortfall
// policy decided the model must be told about.
type BriefCaveat struct {
	Tier   string
	Status string
	Reason string
}

// Caveats reads the retrieval_brief_caveat rows the kernel derived for
// issueID, in a stable order.
func Caveats(src FactSource, issueID string) ([]BriefCaveat, error) {
	if src == nil || issueID == "" {
		return nil, nil
	}
	rows, err := src.Query("retrieval_brief_caveat")
	if err != nil {
		return nil, fmt.Errorf("query retrieval_brief_caveat: %w", err)
	}
	var out []BriefCaveat
	seen := map[BriefCaveat]bool{}
	for _, row := range rows {
		if len(row.Args) < 4 || types.ExtractString(row.Args[0]) != issueID {
			continue
		}
		c := BriefCaveat{
			Tier:   strings.TrimPrefix(types.ExtractString(row.Args[1]), "/"),
			Status: strings.TrimPrefix(types.ExtractString(row.Args[2]), "/"),
			Reason: strings.TrimPrefix(types.ExtractString(row.Args[3]), "/"),
		}
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tier != out[j].Tier {
			return out[i].Tier < out[j].Tier
		}
		if out[i].Status != out[j].Status {
			return out[i].Status < out[j].Status
		}
		return out[i].Reason < out[j].Reason
	})
	return out, nil
}

// caveatLine renders one caveat as a brief line.
func caveatLine(c BriefCaveat) string {
	return fmt.Sprintf("- %s search %s (%s): what it did not reach is not listed, so a file missing here is not evidence that none matches.", c.Tier, c.Status, strings.ReplaceAll(c.Reason, "_", " "))
}
