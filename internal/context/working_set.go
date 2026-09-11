package context

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

//go:embed working_set.mg
var workingSetPolicy string

type WorkingWorld interface {
	Query(string) ([]core.Fact, error)
}

// WorkingSet connects the existing context compiler to a task-private policy
// scope and durable observations. It never writes facts to the action kernel.
type WorkingSet struct {
	mu       sync.Mutex
	world    WorkingWorld
	engine   *mangle.Engine
	store    *WorkingStore
	root     string
	selector *Compressor
}

func NewWorkingSet(world WorkingWorld, root, scope string) (*WorkingSet, error) {
	root, err := tools.CanonicalWorkspaceRoot(root)
	if err != nil {
		return nil, err
	}
	schemas, _, err := core.DefaultCorpusText()
	if err != nil {
		return nil, err
	}
	policy, err := core.GetDefaultContent("policy/context_compilation.mg")
	if err != nil {
		return nil, err
	}
	cfg := mangle.DefaultConfig()
	cfg.AutoEval = true
	engine, err := mangle.NewEngine(cfg, nil)
	if err != nil {
		return nil, err
	}
	if err = engine.LoadSchemaString(schemas + "\n" + policy + "\n" + workingSetPolicy); err != nil {
		_ = engine.Close()
		return nil, err
	}
	// Facts written in the policy (the spans, the transcript window) reach
	// the fact store only when an evaluation runs; before the first Select or
	// Continue nothing could read them back.
	if err = engine.Evaluate(); err != nil {
		_ = engine.Close()
		return nil, err
	}
	storage, err := OpenWorkingStore(root, scope)
	if err != nil {
		_ = engine.Close()
		return nil, err
	}
	return &WorkingSet{world: world, engine: engine, store: storage, root: root, selector: NewCompressor(nil, nil, nil)}, nil
}

func (w *WorkingSet) Close() error                                    { _ = w.engine.Close(); return w.store.Close() }
func (w *WorkingSet) Save(ctx context.Context, r WorkingRecord) error { return w.store.Save(ctx, r) }
func (w *WorkingSet) Search(ctx context.Context, query string, offset, limit int) (string, error) {
	return w.store.Search(ctx, query, offset, limit)
}

// Recall returns a page of an archived observation from a character offset;
// a limit of zero or less returns the rest of the body.
func (w *WorkingSet) Recall(ctx context.Context, id string, offset, limit int) (string, error) {
	r, total, err := w.store.Read(ctx, id, offset, limit)
	if err != nil {
		return "", err
	}
	current := w.Revision(r.Entity)
	data, err := json.Marshal(struct {
		Record          WorkingRecord `json:"record"`
		CurrentRevision string        `json:"current_revision"`
		Stale           bool          `json:"stale"`
		Offset          int           `json:"offset"`
		Total           int           `json:"total_chars"`
		Next            int           `json:"next_offset"`
	}{r, current, current != r.Revision, offset, total, min(total, offset+len([]rune(r.Body)))})
	return string(data), err
}

// Revision is content identity, not HEAD: uncommitted edits invalidate views.
func (w *WorkingSet) Revision(entity string) string {
	if entity == "" {
		return "nonfile"
	}
	path, err := tools.ResolveWorkspacePath(context.Background(), w.root, entity)
	if err != nil {
		return "unavailable"
	}
	f, err := os.Open(path)
	if err != nil {
		return "unavailable"
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(h.Sum(nil))
}

type WorkingSelection struct {
	Text       string
	Selected   []string
	Omitted    []string
	Candidates int
}

// WorkingProgress is the loop's report to policy at a round boundary. The
// loop counts; the policy (working_set.mg) decides what the counts mean.
type WorkingProgress struct {
	Cycle        bool   // the tail of the tool trace repeats deterministically
	FailedRounds int    // consecutive rounds in which every tool failed
	WriteIntent  bool   // the turn's verb is write-oriented
	Rounds       int    // rounds completed this turn
	Writes       int    // durable writes so far
	SinceWrite   int    // rounds since the last durable write (Rounds when none)
	SinceVerify  int    // rounds since the last focused verification (Rounds when none)
	Regime       string // the regime the round just ran under ("" open, "commit")
}

// WorkingDecision is policy's answer. A Stop means the task is unresolved. A
// Finalize means exploration is over and the harness collects the conclusion
// and runs verification. A Nudge is steering text for the model; it never
// ends the loop by itself.
type WorkingDecision struct {
	Continue bool
	Stop     string // working_stop reason, without the leading slash
	Finalize string // working_finalize reason, without the leading slash
	Nudge    string // working_nudge kind, without the leading slash
	Regime   string // working_regime for the next round, without the leading slash
}

// Continue asks policy whether observed execution should continue. A stop is
// an unresolved task, never a successful completion witness.
func (w *WorkingSet) Continue(ctx context.Context, p WorkingProgress) (WorkingDecision, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Plain "/"-prefixed strings: the engine encodes those as name atoms. A
	// types.MangleAtom is a fmt.Stringer to the encoder and became a string
	// constant, so working_control(/yes, _) never matched and the repeated
	// cycle stop had never fired.
	flag := "/no"
	if p.Cycle {
		flag = "/yes"
	}
	intent := "/read"
	if p.WriteIntent {
		intent = "/write"
	}
	regime := "/open"
	if strings.TrimPrefix(strings.TrimSpace(p.Regime), "/") == "commit" {
		regime = "/commit"
	}
	facts := []mangle.Fact{
		{Predicate: "working_control", Args: []any{flag, int64(p.FailedRounds)}},
		{Predicate: "working_progress", Args: []any{intent, int64(p.Rounds), int64(p.Writes), int64(p.SinceWrite), int64(p.SinceVerify)}},
		{Predicate: "working_regime_now", Args: []any{regime}},
	}
	// Control facts are not file-keyed and their derivations must not outlive
	// them; ReplaceFactsForFile did neither (see Engine.ReplaceControlFacts).
	if err := w.engine.ReplaceControlFacts(facts, "working_control", "working_progress", "working_regime_now"); err != nil {
		return WorkingDecision{}, err
	}
	first := func(query, variable string) (string, error) {
		rows, err := w.engine.Query(ctx, query)
		if err != nil {
			return "", err
		}
		if len(rows.Bindings) == 0 {
			return "", nil
		}
		return strings.TrimPrefix(fmt.Sprint(rows.Bindings[0][variable]), "/"), nil
	}
	var decision WorkingDecision
	var err error
	if decision.Stop, err = first("working_stop(Reason)", "Reason"); err != nil {
		return WorkingDecision{}, err
	}
	if decision.Stop != "" {
		return decision, nil
	}
	if decision.Finalize, err = first("working_finalize(Reason)", "Reason"); err != nil {
		return WorkingDecision{}, err
	}
	if decision.Nudge, err = first("working_nudge(Kind)", "Kind"); err != nil {
		return WorkingDecision{}, err
	}
	if decision.Regime, err = first("working_regime(Regime)", "Regime"); err != nil {
		return WorkingDecision{}, err
	}
	rows, err := w.engine.Query(ctx, "working_continue()")
	if err != nil {
		return WorkingDecision{}, err
	}
	decision.Continue = len(rows.Bindings) > 0
	return decision, nil
}

// Select refreshes a bounded dependency slice, then asks the canonical Mangle
// context rules which entities matter. Only the selected record bodies load.
// TranscriptRounds is the policy's span of native call/result rounds the
// request keeps in the provider transcript (working_transcript_rounds).
//
// Read from the fact store rather than through Query: a constant written in
// the policy is a base fact, and the rule-evaluating query path answers
// nothing for one (working_nudge_rounds(N) likewise), while the rules that
// join on it evaluate fine.
func (w *WorkingSet) TranscriptRounds(context.Context) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	facts := w.engine.QueryFacts("working_transcript_rounds")
	if len(facts) == 0 || len(facts[0].Args) != 1 {
		return 0, fmt.Errorf("working policy declares no working_transcript_rounds")
	}
	n, err := strconv.Atoi(fmt.Sprint(facts[0].Args[0]))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("working_transcript_rounds must be a positive count, got %v", facts[0].Args[0])
	}
	return n, nil
}

// Select chooses the observations for this round's system prompt. shown
// names the observations whose native call/result pair the request already
// carries in the transcript; they are neither selected nor reported omitted.
func (w *WorkingSet) Select(ctx context.Context, focus string, recent, shown []string, charBudget int) (WorkingSelection, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return WorkingSelection{}, err
	}
	inTranscript := make(map[string]bool, len(shown))
	for _, id := range shown {
		inTranscript[id] = true
	}
	var facts []mangle.Fact
	var worldFacts []core.Fact
	entities := []string{focus}
	seen := map[string]bool{focus: true}
	add := func(p string, args ...any) { facts = append(facts, mangle.Fact{Predicate: p, Args: args}) }
	add("user_intent", types.MangleAtom("/current_intent"), types.MangleAtom("/query"), types.MangleAtom("/read"), focus, "")
	add("focus_resolution", focus, focus, "", int64(100))
	// Two hops match C4's explicit reachability policy. Cap traversal input,
	// never enumerate the entire world merely to choose a small working set.
	frontier := []string{focus}
	for hop := 0; hop < 2 && w.world != nil; hop++ {
		var next []string
		for _, entity := range frontier {
			for _, query := range []string{fmt.Sprintf("dependency_link(%q, Dep, Kind)", entity), fmt.Sprintf("dependency_link(Dep, %q, Kind)", entity)} {
				rows, err := w.world.Query(query)
				if err != nil {
					return WorkingSelection{}, fmt.Errorf("working dependency query: %w", err)
				}
				for _, f := range rows {
					if len(f.Args) != 3 {
						continue
					}
					a, _ := f.Args[0].(string)
					b, _ := f.Args[1].(string)
					other := a
					if a == entity {
						other = b
					}
					if !seen[other] && len(entities) >= 64 {
						continue
					}
					facts = append(facts, mangle.Fact{Predicate: f.Predicate, Args: f.Args})
					if !seen[other] {
						seen[other] = true
						entities = append(entities, other)
						next = append(next, other)
					}
				}
			}
		}
		frontier = next
	}
	for _, entity := range entities {
		add("working_revision", entity, w.Revision(entity))
		if w.world == nil {
			continue
		}
		for _, query := range []string{fmt.Sprintf("code_defines(%q, Symbol, Kind, Start, End)", entity), fmt.Sprintf("code_element(Ref, Kind, %q, Start, End)", entity)} {
			rows, err := w.world.Query(query)
			if err != nil {
				return WorkingSelection{}, fmt.Errorf("working code query: %w", err)
			}
			worldFacts = append(worldFacts, rows...)
		}
	}
	records, err := w.store.Candidates(ctx, entities, 256)
	if err != nil {
		return WorkingSelection{}, err
	}
	for _, r := range records {
		add("working_observation", r.ID, r.Entity, r.Revision, r.Kind, r.Step)
		add("working_digest", r.ID, r.Digest)
	}
	for _, id := range recent {
		add("working_recent", id)
	}
	for _, id := range shown {
		add("working_in_transcript", id)
	}
	// Replace, not accumulate. ReplaceFactsForFile keys removal by a fact's
	// first string argument, and none of these facts is keyed by the label,
	// so nothing asserted here was ever removed: every revision a file had
	// ever had stayed asserted as working_revision, and evaluation is
	// monotone, so once the first edit landed every later observation of that
	// file derived working_stale against the old revision and none was ever
	// selected again. Seen live 2026-09-11: after its one edit the model
	// re-read the edited file eight times and concluded that no edit had been
	// needed.
	if err := w.engine.ReplaceControlFacts(facts, "user_intent", "focus_resolution", "dependency_link", "working_revision", "working_observation", "working_digest", "working_recent", "working_in_transcript"); err != nil {
		return WorkingSelection{}, err
	}
	result, err := w.engine.Query(ctx, "working_selected(ID, Priority)")
	if err != nil {
		return WorkingSelection{}, err
	}
	priorities := map[string]int{}
	for _, row := range result.Bindings {
		id, _ := row["ID"].(string)
		p := strings.TrimPrefix(fmt.Sprint(row["Priority"]), "/p")
		n, _ := strconv.Atoi(p)
		if n > priorities[id] {
			priorities[id] = n
		}
	}
	sort.SliceStable(records, func(i, j int) bool {
		if priorities[records[i].ID] != priorities[records[j].ID] {
			return priorities[records[i].ID] > priorities[records[j].ID]
		}
		return records[i].Step > records[j].Step
	})
	selection := WorkingSelection{Candidates: len(records)}
	var text strings.Builder
	// Facts use the existing serializer and canonical context inclusion priorities.
	included, err := w.engine.Query(ctx, "should_include_context(Entity, Priority)")
	if err != nil {
		return selection, err
	}
	var decisions []core.Fact
	for _, row := range included.Bindings {
		decisions = append(decisions, core.Fact{Predicate: "should_include_context", Args: []any{row["Entity"], fmt.Sprint(row["Priority"])}})
	}
	// World facts annotate the evidence; they are not the evidence. They used
	// to be allowed half the section, which at a 16 KB section was 8 KB and
	// at the window-derived section is enough for the symbol table of every
	// file within two dependency hops: measured 2026-09-11, 762 code_defines
	// lines (78 KB, about 20k tokens) on every round of a one-line edit, most
	// for files the task never touched. The share is one eighth; the policy's
	// priority order decides which facts fill it, so the focus entity's own
	// definitions come first and the far neighbourhood is what gets cut.
	factShare := charBudget / 8
	w.selector.config.AtomReserve = max(1, factShare/4)
	selectedFacts := w.selector.buildKernelDerivedContext(decisions, worldFacts)
	for _, f := range selectedFacts {
		line := f.Fact.String() + "\n"
		if text.Len()+len(line) <= factShare {
			text.WriteString(line)
		}
	}
	for _, r := range records {
		if inTranscript[r.ID] {
			continue
		}
		if priorities[r.ID] == 0 {
			selection.Omitted = append(selection.Omitted, r.ID)
			continue
		}
		// The whole body, so the budget alone decides whether it is shown. A
		// 16000-character page here meant a longer observation was pointed at
		// and never shown, however much budget the request had.
		body, _, err := w.store.Read(ctx, r.ID, 0, 0)
		if err != nil {
			return selection, err
		}
		header := fmt.Sprintf("\n[observation id=%q entity=%q revision=%q failed=%t]\n", r.ID, r.Entity, r.Revision, r.Failed)
		if text.Len()+len(header)+len(body.Body)+1 > charBudget {
			selection.Omitted = append(selection.Omitted, r.ID)
			ref := header + "[body outside active budget; recover with recall_context]\n"
			if text.Len()+len(ref) <= charBudget {
				text.WriteString(ref)
			}
			continue
		}
		text.WriteString(header)
		text.WriteString(body.Body)
		text.WriteByte('\n')
		selection.Selected = append(selection.Selected, r.ID)
	}
	selection.Text = text.String()
	logging.Context("Working context: candidates=%d selected=%d omitted=%d in_transcript=%d chars=%d focus=%q", len(records), len(selection.Selected), len(selection.Omitted), len(shown), text.Len(), focus)
	return selection, nil
}
