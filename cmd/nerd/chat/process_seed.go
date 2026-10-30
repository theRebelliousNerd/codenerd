package chat

import (
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/retrieval"
	"codenerd/internal/transparency"
	"context"
	"strings"
)

// chatIssueID is the one EDB key the chat seeds under, for the reason it
// asserts one /current_intent: a kernel that keeps every past turn's
// candidates answers the current turn with all of them, and the compressor
// picked "the last" issue_context row out of an unordered query result.
const chatIssueID = "/chat_issue"

// seedIssueFacts runs a bounded sparse-retrieval pass over the workspace and
// asserts the issue_* / candidate_file / keyword_hit / tiered_context_file /
// issue_context EDB surface. This drives issue-aware spreading activation and
// prompt atom selection.
//
// Whether it runs is the kernel's decision, issue_retrieval_wanted over
// /current_intent (schemas_knowledge.mg 52.5), not a Go switch on the verb:
// until 2026-09-25 this method carried that switch and no other entry path
// consulted it, so a shard's task turns never retrieved. The pass replaces
// the previous turn's, so the kernel holds one live chat issue.
//
// It used to extract keywords and stop there, so the SparseRetriever that boot
// builds into SystemComponents.Retriever was never called — it was not even
// copied onto the model — and the kernel only ever learned about files the user
// had named by hand. Two of the facts it did assert were
// silently dropped by the kernel besides: issue_keyword's weight and
// tiered_context_file's relevance are declared /number, and a fractional float
// in a /number slot is rejected outright, which took out every keyword weight
// except the 1.0 on mentioned files. Both scales now go through
// types.PercentFromRatio inside the retrieval transducer.
func (m *Model) seedIssueFacts(ctx context.Context, intent perception.Intent, rawInput string) {
	if m.kernel == nil {
		return
	}

	wanted, err := retrieval.Wanted(m.kernel, "/current_intent")
	if err != nil {
		logging.Context("[seedIssueFacts] kernel not asked whether %s retrieves: %v", intent.Verb, err)
		return
	}
	if !wanted {
		return
	}

	issueText := strings.TrimSpace(rawInput)
	if issueText == "" {
		return
	}

	// Keep the stored issue text bounded to avoid EDB bloat.
	const maxIssueChars = 4000
	if len(issueText) > maxIssueChars {
		issueText = issueText[:maxIssueChars]
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if err := retrieval.SupersedeIssue(m.kernel, chatIssueID); err != nil {
		logging.Context("[seedIssueFacts] previous chat issue not retracted: %v", err)
	}

	// The budget is deliberately separate from the LLM timeouts: this is
	// filesystem work on the user's turn, and a repository too large to finish
	// inside it must yield the tiers that completed rather than stall the loop.
	var bus *transparency.GlassBoxEventBus
	if m.glassBoxEnabled {
		bus = m.glassBoxEventBus
	}

	relevance := m.relevanceUse()
	if _, err := retrieval.SeedIssueFacts(m.jevContext(ctx), m.kernel, retrieval.SeedRequest{
		IssueID:   chatIssueID,
		IssueText: issueText,
		WorkDir:   m.workspace,
		Retriever: m.retriever,
		// cortex.SemanticSearcher: nil when no embedding engine came up at
		// boot, in which case the seed keeps the heuristic Tier 4 fallback.
		SemanticSearcher: m.semanticSearcher,
		// Jev, when configured, judges Tier 2's keyword candidates for
		// relevance (turnjudge.RelevanceFor); off, the keyword ranking
		// stands.
		Relevance:           relevance.Judge,
		RelevanceDrop:       relevance.Drop,
		RelevancePoolFactor: relevance.PoolFactor,
		RelevanceHeadBytes:  relevance.HeadBytes,
		Timeout:             retrieval.DefaultSeedTimeout,
		GlassBox:            bus,
		TurnID:              m.turnCount,
	}); err != nil {
		logging.Context("[seedIssueFacts] retrieval seed failed: %v", err)
	}
}
