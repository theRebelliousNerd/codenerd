package retrieval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/transparency"
	"codenerd/internal/types"
)

// =============================================================================
// EDB TRANSDUCTION
// =============================================================================
//
// Retrieval used to end at a Go struct. Every candidate the scanner ranked, every
// tier the builder assembled, died inside the caller's stack frame, so no Mangle
// rule could ever reason about what was retrieved — the kernel is the executive
// here, and it was being handed nothing. This file is the transducer: it turns a
// completed retrieval pass into the EDB surface declared in section 52 of
// internal/core/defaults/schemas_knowledge.mg.
//
// Every argument below is typed against its Decl bound rather than against what
// looks natural in Go, because the mismatch is silent in both directions:
//
//   - A /number slot is int64. types.Fact.ToAtom maps a Go float64 to
//     ast.Float64, and RealKernel.coerceAtomToDeclLocked REJECTS a fractional
//     one outright (a whole one it narrows). That is why the pre-existing seed
//     path asserted issue_keyword weights of 0.9/0.85/0.7/0.5 and every one of
//     them was dropped on the floor: only the 1.0 weights on mentioned files
//     ever reached the store. Ratios go through types.PercentFromRatio.
//   - A /name slot must receive types.MangleAtom. A Go string only becomes a
//     name if isValidMangleNameConstant happens to say yes, which it does for
//     "/tier1" and does not for most values.
//   - A /string slot that holds a name-shaped value ("/issue_17…") must be
//     wrapped in types.MangleString, otherwise the same heuristic silently
//     stores it as a name constant instead.

// FactSink is the write side of the Mangle kernel, narrowed to what this package
// needs. *core.RealKernel and types.Kernel both satisfy it; taking the interface
// keeps internal/retrieval free of an import edge to internal/core.
type FactSink interface {
	LoadFacts(facts []types.Fact) error
}

// FactSource is the read side, used to learn which workspace files changed since
// the last pass so their cached keyword hits can be dropped.
type FactSource interface {
	Query(predicate string) ([]types.Fact, error)
}

// DefaultSeedTimeout bounds one issue-seeding pass.
//
// It is deliberately independent of the LLM timeouts: this budget covers
// filesystem work on the caller's turn, and a repository large enough to blow
// through it is exactly the case where the turn must proceed with whatever tiers
// completed rather than stall. The scan is cache-backed, so the second seed in a
// session is typically far cheaper than the first.
const DefaultSeedTimeout = 5 * time.Second

// SeedRequest describes one issue-driven retrieval + assertion pass.
type SeedRequest struct {
	// IssueID is the EDB key every asserted fact is joined on. Generated when
	// empty.
	IssueID string

	// IssueText is the raw problem statement.
	IssueText string

	// WorkDir is the workspace root. Paths are asserted relative to it.
	WorkDir string

	// Retriever is reused when non-nil so the session keeps its warm keyword
	// cache; otherwise one is constructed for WorkDir.
	Retriever *SparseRetriever

	// Timeout bounds the whole pass. DefaultSeedTimeout when zero.
	Timeout time.Duration

	// MaxFiles caps the tiered context. The builder default applies when zero.
	MaxFiles int

	// GlassBox receives the liveness event. Callers that own a bus should pass
	// it: nothing in production calls transparency.SetProcessBus, so the
	// process-wide fallback below is empty in a real session and an event sent
	// only there would be invisible — the exact "wired but unreachable" shape
	// this pass exists to remove.
	GlassBox *transparency.GlassBoxEventBus

	// TurnID tags the glass-box event.
	TurnID int
}

// SeedReport records what the pass actually did. It exists so callers (and
// tests) can prove liveness instead of trusting that the search ran.
type SeedReport struct {
	IssueID     string
	Facts       int
	TierCounts  [4]int
	Candidates  int
	KeywordHits int
	TotalTokens int64
	Duration    time.Duration
	// TimedOut is true when the budget expired mid-pass. Facts from the tiers
	// that did complete are still asserted.
	TimedOut bool
	// Metrics is the retriever's cumulative counter snapshot after the pass.
	Metrics RetrieverMetrics
}

// Summary renders the report as the one line the glass box and the context log
// both show.
func (r *SeedReport) Summary() string {
	if r == nil {
		return "sparse retrieval: no pass"
	}
	s := fmt.Sprintf("sparse retrieval %s: %d facts, tiers %d/%d/%d/%d, %d candidates, %d keyword hits",
		r.IssueID, r.Facts, r.TierCounts[0], r.TierCounts[1], r.TierCounts[2], r.TierCounts[3],
		r.Candidates, r.KeywordHits)
	if r.TimedOut {
		s += " (budget expired)"
	}
	return s
}

// NewIssueID mints an EDB key for an issue-driven turn.
func NewIssueID() string {
	return fmt.Sprintf("/issue_%d", time.Now().UnixNano())
}

// SeedIssueFacts runs a bounded sparse-retrieval pass over the workspace and
// asserts the full section-52 fact set into the kernel.
//
// This is the wire the corpus called for and never had: Model.Retriever was
// constructed at boot and no method was ever called on it, so candidate_file,
// keyword_hit, issue_context and tiers 2-4 of tiered_context_file simply did not
// exist in any session. The keyword-only facts are asserted even when the search
// half fails or times out, because losing the whole seed to a slow filesystem is
// worse than losing the disk-ranked half of it.
func SeedIssueFacts(ctx context.Context, sink FactSink, req SeedRequest) (*SeedReport, error) {
	if sink == nil {
		return nil, fmt.Errorf("retrieval: nil fact sink")
	}
	issueText := strings.TrimSpace(req.IssueText)
	if issueText == "" {
		return nil, nil
	}

	started := time.Now()
	issueID := req.IssueID
	if issueID == "" {
		issueID = NewIssueID()
	}
	workDir := req.WorkDir
	if workDir == "" {
		workDir = "."
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = DefaultSeedTimeout
	}

	keywords := ExtractKeywords(issueText)
	report := &SeedReport{IssueID: issueID}
	facts := IssueKeywordFacts(issueID, issueText, keywords)

	retriever := req.Retriever
	if retriever == nil {
		retriever = NewSparseRetriever(DefaultSparseRetrieverConfig(workDir))
	}

	// A file edited earlier in the session must not be answered from the hit
	// cache. The kernel already records every write as file_written, so the
	// invalidation signal is read out of Mangle rather than plumbed through a
	// second Go callback chain.
	if src, ok := sink.(FactSource); ok {
		retriever.InvalidateFromKernel(src)
	}

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cfg := DefaultTieredContextConfig(workDir)
	cfg.Retriever = retriever
	if req.MaxFiles > 0 {
		cfg.MaxTotal = req.MaxFiles
	}
	builder := NewTieredContextBuilder(cfg)

	tc, err := builder.BuildContext(cctx, issueText)
	if cctx.Err() != nil {
		report.TimedOut = true
	}
	if err != nil {
		logging.Context("SeedIssueFacts: tiered build failed for %s: %v", issueID, err)
	}

	if tc != nil {
		facts = append(facts, TieredContextFacts(issueID, tc, workDir)...)
		report.TierCounts = [4]int{tc.Tier1Count, tc.Tier2Count, tc.Tier3Count, tc.Tier4Count}
		report.Candidates = len(tc.Candidates)
		for _, c := range tc.Candidates {
			report.KeywordHits += len(c.Hits)
		}
		report.TotalTokens = totalTokens(tc, workDir)
	} else {
		// No tiered context at all: the mentions still deserve to be resolved
		// and asserted, otherwise a timeout costs the caller even the free half.
		facts = append(facts, mentionFacts(issueID, workDir, keywords.MentionedFiles, nil)...)
	}

	report.Facts = len(facts)
	report.Duration = time.Since(started)

	if err := sink.LoadFacts(facts); err != nil {
		return report, fmt.Errorf("retrieval: loading %d issue facts: %w", len(facts), err)
	}

	reportLiveness(report, req.GlassBox, req.TurnID)
	report.Metrics = retriever.Metrics()
	return report, nil
}

// reportLiveness writes the pass to the context log and mirrors it onto the
// glass box. Without this a live sparse search is indistinguishable from the
// dormant retriever it replaced — the corpus explicitly asked for a signal that
// proves the search ran.
func reportLiveness(report *SeedReport, bus *transparency.GlassBoxEventBus, turnID int) {
	logging.Context("%s in %dms", report.Summary(), report.Duration.Milliseconds())
	if bus == nil {
		bus = transparency.ProcessBus()
	}
	if bus != nil {
		bus.EmitImmediate(transparency.GlassBoxEvent{
			Timestamp: time.Now(),
			Category:  transparency.CategoryKernel,
			Summary:   report.Summary(),
			Details: fmt.Sprintf("issue=%s facts=%d tier1=%d tier2=%d tier3=%d tier4=%d tokens=%d timed_out=%v",
				report.IssueID, report.Facts, report.TierCounts[0], report.TierCounts[1],
				report.TierCounts[2], report.TierCounts[3], report.TotalTokens, report.TimedOut),
			TurnID:   turnID,
			Duration: report.Duration,
			Source:   "sparse_retriever",
		})
	}
}

// =============================================================================
// FACT BUILDERS
// =============================================================================

// IssueKeywordFacts builds the extraction half of the EDB surface:
// issue_text/2, issue_keyword/3 and keyword_weight/2. It touches no filesystem,
// so it is safe on the caller's turn even when the search budget is zero.
func IssueKeywordFacts(issueID, issueText string, keywords *IssueKeywords) []types.Fact {
	id := types.MangleString(issueID)
	facts := []types.Fact{{
		Predicate: "issue_text",
		Args:      []any{id, issueText},
	}}
	if keywords == nil {
		return facts
	}

	// Map order is random; a stable fact order keeps EDB snapshots and test
	// output diffable.
	words := make([]string, 0, len(keywords.Weights))
	for kw := range keywords.Weights {
		if strings.TrimSpace(kw) != "" {
			words = append(words, kw)
		}
	}
	sort.Strings(words)

	for _, kw := range words {
		facts = append(facts, types.Fact{
			Predicate: "issue_keyword",
			// Weight is declared /number: a 0..1 ratio has to be scaled to the
			// integer percent scale or the kernel drops the fact silently.
			Args: []any{id, types.MangleString(kw), types.PercentFromRatio(keywords.Weights[kw])},
		})
	}

	for _, kw := range keywords.Primary {
		facts = append(facts, keywordCategoryFact(kw, "/primary"))
	}
	for _, kw := range keywords.Secondary {
		facts = append(facts, keywordCategoryFact(kw, "/secondary"))
	}
	for _, kw := range keywords.Tertiary {
		facts = append(facts, keywordCategoryFact(kw, "/tertiary"))
	}

	return facts
}

func keywordCategoryFact(keyword, category string) types.Fact {
	return types.Fact{
		Predicate: "keyword_weight",
		Args:      []any{types.MangleString(keyword), types.MangleAtom(category)},
	}
}

// TieredContextFacts builds the search half: file_mentioned/2, context_tier/2,
// tiered_context_file/5, candidate_file/2, keyword_hit/3 and issue_context/3.
//
// Paths are resolved and made workspace-relative first. Asserting the raw
// mention ("kernel.go") produced facts no other predicate could join against,
// since every other file-keyed predicate in the corpus carries a real path.
func TieredContextFacts(issueID string, tc *TieredContext, workDir string) []types.Fact {
	if tc == nil {
		return nil
	}
	id := types.MangleString(issueID)
	facts := make([]types.Fact, 0, len(tc.Files)*2+len(tc.Candidates)*2+8)

	var mentioned []string
	if tc.Keywords != nil {
		mentioned = tc.Keywords.MentionedFiles
	}
	facts = append(facts, mentionFacts(issueID, workDir, mentioned, tc.ResolvedMentions)...)

	var totalTok int64
	seenTier := make(map[string]bool, len(tc.Files))
	for _, f := range tc.Files {
		path := workspacePath(workDir, f.FilePath)
		if path == "" {
			continue
		}
		tier := tierAtom(f.Tier)
		if tier == "" {
			continue
		}
		tokens := estimateTokens(f.FilePath)
		totalTok += tokens

		if !seenTier[path] {
			seenTier[path] = true
			facts = append(facts, types.Fact{
				Predicate: "context_tier",
				Args:      []any{types.MangleString(path), tier},
			})
		}
		facts = append(facts, types.Fact{
			Predicate: "tiered_context_file",
			Args: []any{
				id,
				types.MangleString(path),
				tier,
				types.PercentFromRatio(f.RelevanceScore),
				tokens,
			},
		})
	}

	for _, c := range tc.Candidates {
		path := workspacePath(workDir, c.FilePath)
		if path == "" {
			continue
		}
		facts = append(facts, types.Fact{
			Predicate: "candidate_file",
			Args:      []any{types.MangleString(path), types.PercentFromRatio(c.RelevanceScore)},
		})
		for keyword, count := range keywordCounts(c) {
			facts = append(facts, types.Fact{
				Predicate: "keyword_hit",
				Args:      []any{types.MangleString(path), types.MangleString(keyword), count},
			})
		}
	}

	facts = append(facts, types.Fact{
		Predicate: "issue_context",
		Args:      []any{id, int64(len(tc.Files)), totalTok},
	})

	return facts
}

// mentionFacts asserts file_mentioned/2 against resolved paths. A mention that
// does not resolve is still asserted under its normalized spelling: dropping it
// would lose the only signal the issue text gave about that file.
func mentionFacts(issueID, workDir string, mentions []string, resolved map[string]string) []types.Fact {
	id := types.MangleString(issueID)
	facts := make([]types.Fact, 0, len(mentions))
	seen := make(map[string]bool, len(mentions))
	for _, mention := range mentions {
		if strings.TrimSpace(mention) == "" {
			continue
		}
		path := normalizePathSeparators(mention)
		if full, ok := resolved[mention]; ok && full != "" {
			if rel := workspacePath(workDir, full); rel != "" {
				path = rel
			}
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		facts = append(facts, types.Fact{
			Predicate: "file_mentioned",
			Args:      []any{types.MangleString(path), id},
		})
	}
	return facts
}

// keywordCounts collapses a candidate's individual hits into per-keyword totals.
func keywordCounts(c CandidateFile) map[string]int64 {
	counts := make(map[string]int64, len(c.Keywords))
	for _, h := range c.Hits {
		if strings.TrimSpace(h.Keyword) == "" {
			continue
		}
		counts[h.Keyword]++
	}
	return counts
}

// tierAtom maps a builder tier number onto the /tierN name constant that
// context_tier and tiered_context_file declare. Anything out of range yields ""
// so the caller skips the fact rather than asserting an atom no rule expects.
func tierAtom(tier int) types.MangleAtom {
	switch tier {
	case 1:
		return "/tier1"
	case 2:
		return "/tier2"
	case 3:
		return "/tier3"
	case 4:
		return "/tier4"
	}
	return ""
}

// workspacePath renders a path for the EDB: forward-slashed and relative to the
// workspace root when it lies inside it. Absolute machine-specific paths in the
// store make facts unjoinable with everything the world scanner asserts and
// unportable across sessions.
func workspacePath(workDir, path string) string {
	if path == "" {
		return ""
	}
	clean := filepath.Clean(path)
	if workDir != "" {
		if rel, err := filepath.Rel(filepath.Clean(workDir), clean); err == nil {
			if rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				clean = rel
			}
		}
	}
	return normalizePathSeparators(clean)
}

// estimateTokens approximates a file's token cost for the context budget.
// Four bytes per token is the usual rule of thumb for source text; the value
// only has to rank files against each other and fill issue_context's total.
func estimateTokens(path string) int64 {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0
	}
	return info.Size() / 4
}

func totalTokens(tc *TieredContext, workDir string) int64 {
	var total int64
	for _, f := range tc.Files {
		if workspacePath(workDir, f.FilePath) == "" {
			continue
		}
		total += estimateTokens(f.FilePath)
	}
	return total
}
