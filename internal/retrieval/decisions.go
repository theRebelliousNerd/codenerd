package retrieval

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// =============================================================================
// KERNEL DECISIONS OVER THE SECTION-52 SURFACE
// =============================================================================
//
// Two decisions sit on either side of a retrieval pass, and both are the
// kernel's (schemas_knowledge.mg, section 52.5):
//
//   - issue_retrieval_wanted(Intent): whether the turn retrieves at all. It
//     replaced a verb switch in the chat's seed that no other entry path
//     consulted, so `nerd fix` -- the path the repo contract tells everyone to
//     use -- never ran a pass.
//   - retrieval_brief_file(IssueID, File, Tier, Relevance): which of the files
//     the pass found the model is handed as places to start reading. Nothing
//     decided that before: the tiered files fed the chat compressor's
//     activation scores and reached no model.
//
// This file is the Go side of both: it asks, it seeds, it renders what the
// kernel derived, and it retracts what it asserted when the issue is over.

// IssueKernel is the slice of a kernel the issue lifecycle needs: seed, read,
// set the thresholds the rules read, and retract.
type IssueKernel interface {
	FactSink
	FactSource
	config.ParamKernel
	Retract(predicate string) error
	RetractExactFactsBatch(facts []types.Fact) error
}

// issueKeyed maps every section-52 predicate that carries the issue ID to the
// argument position holding it. These are the facts one issue can own and
// retract without touching another issue's evidence.
var issueKeyed = map[string]int{
	"issue_text":          0,
	"issue_keyword":       0,
	"file_mentioned":      1,
	"tiered_context_file": 0,
	"issue_context":       0,
}

// issueKeyedKinds is each issue-keyed predicate's Decl, slot by slot: 's'
// /string, 'n' /name, '#' /number. A row read back from a Query has lost the
// Go type it was asserted with -- "/chat_issue" comes back a plain string --
// and a plain string that looks like a name constant is converted to one, so
// retracting the row as read matches nothing (the atom/string trap in
// internal/mangle/agents.md). The row is retyped against its Decl first.
var issueKeyedKinds = map[string]string{
	"issue_text":          "ss",
	"issue_keyword":       "ss#",
	"file_mentioned":      "ss",
	"tiered_context_file": "ssn##",
	"issue_context":       "s##",
}

// retypedRow restores a queried row's argument types from its Decl.
func retypedRow(row types.Fact) types.Fact {
	kinds := issueKeyedKinds[row.Predicate]
	args := make([]any, len(row.Args))
	for i, arg := range row.Args {
		args[i] = arg
		if i >= len(kinds) {
			continue
		}
		switch kinds[i] {
		case 's':
			args[i] = types.MangleString(types.ExtractString(arg))
		case 'n':
			args[i] = types.MangleAtom(types.ExtractString(arg))
		}
	}
	return types.Fact{Predicate: row.Predicate, Args: args}
}

// unscopedPredicates are the section-52 predicates with no issue argument.
// Two concurrent issues cannot tell their rows apart, so a scoped pass never
// asserts them, and the chat's single live issue replaces them wholesale.
var unscopedPredicates = []string{"keyword_weight", "context_tier", "candidate_file", "keyword_hit"}

// Wanted reports whether the kernel derives issue_retrieval_wanted(intentID).
// An error means the kernel could not be asked; callers treat it as "no pass"
// and say so.
func Wanted(src FactSource, intentID string) (bool, error) {
	if src == nil || strings.TrimSpace(intentID) == "" {
		return false, nil
	}
	rows, err := src.Query("issue_retrieval_wanted")
	if err != nil {
		return false, fmt.Errorf("query issue_retrieval_wanted: %w", err)
	}
	for _, row := range rows {
		if len(row.Args) > 0 && types.ExtractString(row.Args[0]) == intentID {
			return true, nil
		}
	}
	return false, nil
}

// scopedFacts keeps only the facts keyed by an issue.
func scopedFacts(facts []types.Fact) []types.Fact {
	out := make([]types.Fact, 0, len(facts))
	for _, f := range facts {
		if _, ok := issueKeyed[f.Predicate]; ok {
			out = append(out, f)
		}
	}
	return out
}

// RetractIssue removes every issue-keyed fact the kernel holds for issueID,
// whoever asserted it. It is how an issue that is over stops being evidence.
func RetractIssue(k IssueKernel, issueID string) error {
	if k == nil || issueID == "" {
		return nil
	}
	var owned []types.Fact
	for predicate, pos := range issueKeyed {
		rows, err := k.Query(predicate)
		if err != nil {
			return fmt.Errorf("query %s: %w", predicate, err)
		}
		for _, row := range rows {
			if len(row.Args) > pos && types.ExtractString(row.Args[pos]) == issueID {
				owned = append(owned, retypedRow(row))
			}
		}
	}
	if len(owned) == 0 {
		return nil
	}
	return k.RetractExactFactsBatch(owned)
}

// SupersedeIssue makes issueID the only live issue of its caller: the facts
// of an earlier pass under the same ID go, and so does the unscoped surface,
// which no later pass can attribute. The chat seeds under one stable ID for
// the same reason it asserts one /current_intent: a kernel that accumulates
// every past turn's candidates answers the current turn with all of them.
func SupersedeIssue(k IssueKernel, issueID string) error {
	if k == nil {
		return nil
	}
	if err := RetractIssue(k, issueID); err != nil {
		return err
	}
	for _, predicate := range unscopedPredicates {
		if err := k.Retract(predicate); err != nil {
			return fmt.Errorf("retract %s: %w", predicate, err)
		}
	}
	return nil
}

// BriefFile is one retrieval_brief_file row.
type BriefFile struct {
	Path      string
	Tier      string
	Relevance int64
}

// Brief reads what the kernel decided to hand the model for issueID, ordered
// the way it is read: the files the issue named first, then by relevance.
func Brief(src FactSource, issueID string) ([]BriefFile, error) {
	if src == nil || issueID == "" {
		return nil, nil
	}
	rows, err := src.Query("retrieval_brief_file")
	if err != nil {
		return nil, fmt.Errorf("query retrieval_brief_file: %w", err)
	}
	seen := make(map[string]bool, len(rows))
	var out []BriefFile
	for _, row := range rows {
		if len(row.Args) < 4 || types.ExtractString(row.Args[0]) != issueID {
			continue
		}
		f := BriefFile{
			Path: types.ExtractString(row.Args[1]),
			Tier: strings.TrimPrefix(types.ExtractString(row.Args[2]), "/"),
		}
		if v, ok := row.Args[3].(int64); ok {
			f.Relevance = v
		}
		if f.Path == "" || seen[f.Path] {
			continue
		}
		seen[f.Path] = true
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tier != out[j].Tier {
			return out[i].Tier < out[j].Tier
		}
		if out[i].Relevance != out[j].Relevance {
			return out[i].Relevance > out[j].Relevance
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

// briefHeader opens the brief where it rides the task. It follows the working
// loop's convention for harness text: it says whose evidence it is and that it
// is not a new request.
const briefHeader = "[harness: files the retrieval pass ranked for this task -- the ones it names, then keyword, import and semantic matches, with tier and relevance percent. Places to start reading, not a new request; the structural queries still answer anything they miss.]"

// RenderBrief renders the brief as harness text, or "" when there is none.
func RenderBrief(files []BriefFile) string {
	if len(files) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(briefHeader)
	for _, f := range files {
		fmt.Fprintf(&b, "\n- %s (%s, %d)", f.Path, f.Tier, f.Relevance)
	}
	return b.String()
}

// taskIssueSeq keeps task issue IDs unique when two tasks start in the same
// nanosecond, which parallel subagents do.
var taskIssueSeq atomic.Uint64

// NewTaskIssueID mints the EDB key for one task's retrieval pass.
func NewTaskIssueID() string {
	return fmt.Sprintf("/task_issue_%d_%d", time.Now().UnixNano(), taskIssueSeq.Add(1))
}

// TaskRetrieverConfig configures a TaskRetriever.
type TaskRetrieverConfig struct {
	// WorkDir is the workspace root the pass searches.
	WorkDir string
	// Retriever is shared with every other pass in the process so the keyword
	// cache stays warm; one is built for WorkDir when nil.
	Retriever *SparseRetriever
	// EmbeddingEngine is the optional Tier 4 backend.
	EmbeddingEngine embedding.EmbeddingEngine
	// Params are the retrieval thresholds the brief rule reads
	// (config.RetrievalConfig.Params).
	Params []config.Param
	// Timeout bounds one pass. DefaultSeedTimeout when zero.
	Timeout time.Duration
}

// TaskRetriever runs the kernel-gated retrieval pass for one task and hands
// back what the kernel decided the model should see.
type TaskRetriever struct {
	kernel IssueKernel
	cfg    TaskRetrieverConfig
}

// NewTaskRetriever builds a TaskRetriever over kernel. Nil when there is no
// kernel to decide with.
func NewTaskRetriever(kernel IssueKernel, cfg TaskRetrieverConfig) *TaskRetriever {
	if kernel == nil {
		return nil
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = "."
	}
	if cfg.Retriever == nil {
		cfg.Retriever = NewSparseRetriever(DefaultSparseRetrieverConfig(cfg.WorkDir))
	}
	return &TaskRetriever{kernel: kernel, cfg: cfg}
}

// Retrieve asks the kernel whether intentID's turn retrieves; if it does, it
// runs one scoped pass over taskText, and returns the brief the kernel derived
// for it ("" when none) and a release that retracts every fact the pass
// asserted. The release is always safe to call.
//
// A failure never fails the turn -- the task still runs, it just starts
// without the brief -- but it is logged where a missing brief can be traced.
func (t *TaskRetriever) Retrieve(ctx context.Context, intentID, taskText string) (string, func()) {
	noop := func() {}
	if t == nil || t.kernel == nil || strings.TrimSpace(taskText) == "" {
		return "", noop
	}
	wanted, err := Wanted(t.kernel, intentID)
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("task retrieval: kernel not asked for %s: %v", intentID, err)
		return "", noop
	}
	if !wanted {
		return "", noop
	}
	if err := config.EnsureParams(t.kernel, t.cfg.Params); err != nil {
		logging.Get(logging.CategoryContext).Warn("task retrieval: thresholds not asserted, brief will hold only named files: %v", err)
	}

	issueID := NewTaskIssueID()
	report, err := SeedIssueFacts(ctx, t.kernel, SeedRequest{
		IssueID:         issueID,
		IssueText:       taskText,
		WorkDir:         t.cfg.WorkDir,
		Retriever:       t.cfg.Retriever,
		EmbeddingEngine: t.cfg.EmbeddingEngine,
		Timeout:         t.cfg.Timeout,
		IssueScopedOnly: true,
	})
	release := func() {
		if rerr := RetractIssue(t.kernel, issueID); rerr != nil {
			logging.Get(logging.CategoryContext).Warn("task retrieval: facts of %s not retracted: %v", issueID, rerr)
		}
	}
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("task retrieval: pass for %s failed: %v", intentID, err)
		return "", release
	}
	if report == nil {
		return "", release
	}

	files, err := Brief(t.kernel, issueID)
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("task retrieval: brief for %s not read: %v", issueID, err)
		return "", release
	}
	logging.Context("task retrieval %s: %d of %d retrieved files handed to the model", issueID, len(files),
		report.TierCounts[0]+report.TierCounts[1]+report.TierCounts[2]+report.TierCounts[3])
	return RenderBrief(files), release
}
