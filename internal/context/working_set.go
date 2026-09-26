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

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/mangle"
	"codenerd/internal/tools"
	"codenerd/internal/world/codemodel"
)

//go:embed working_set.mg
var workingSetPolicy string

// WorkingSet is a tool loop's task-private policy scope and its durable
// observations: the working policy (working_set.mg) decides the loop's regime,
// steering and stops, and what its context ledger carries. It never writes
// facts to the action kernel.
type WorkingSet struct {
	mu     sync.Mutex
	engine *mangle.Engine
	store  *WorkingStore
	root   string
}

// NewWorkingSet builds a task's working set. spans is the working section of
// .nerd/config.json: the policy reads every span it decides with from it
// (config_param rows), and the set refuses to build while one it requires is
// missing, rather than run a loop whose stop and finalize rules can never
// fire.
func NewWorkingSet(root, scope string, spans config.WorkingConfig) (*WorkingSet, error) {
	root, err := tools.CanonicalWorkspaceRoot(root)
	if err != nil {
		return nil, err
	}
	schemas, _, err := core.DefaultCorpusText()
	if err != nil {
		return nil, err
	}
	params, err := core.GetDefaultContent("policy/config_params.mg")
	if err != nil {
		return nil, err
	}
	cfg := mangle.DefaultConfig()
	cfg.AutoEval = true
	engine, err := mangle.NewEngine(cfg, nil)
	if err != nil {
		return nil, err
	}
	if err = engine.LoadSchemaString(schemas + "\n" + params + "\n" + workingSetPolicy); err != nil {
		_ = engine.Close()
		return nil, err
	}
	var rows []mangle.Fact
	for _, p := range spans.Params() {
		rows = append(rows, mangle.Fact{Predicate: config.ConfigParamPredicate, Args: []any{p.Key, p.Value}})
	}
	if err = engine.AddFacts(rows); err != nil {
		_ = engine.Close()
		return nil, fmt.Errorf("assert the working spans: %w", err)
	}
	// Facts derived from the spans reach the fact store only when an
	// evaluation runs; before the first Ledger or Continue nothing could read
	// them back.
	if err = engine.Evaluate(); err != nil {
		_ = engine.Close()
		return nil, err
	}
	if missing := engine.QueryFacts("config_param_missing"); len(missing) > 0 {
		_ = engine.Close()
		var keys []string
		for _, f := range missing {
			if len(f.Args) == 2 {
				keys = append(keys, strings.TrimPrefix(fmt.Sprint(f.Args[1]), "/"))
			}
		}
		sort.Strings(keys)
		return nil, fmt.Errorf("the working policy needs %s from the working section of config.json", strings.Join(keys, ", "))
	}
	storage, err := OpenWorkingStore(root, scope)
	if err != nil {
		_ = engine.Close()
		return nil, err
	}
	return &WorkingSet{engine: engine, store: storage, root: root}, nil
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

// Entity names the file an archived observation was recorded under, or ""
// when the store holds no record with that id.
func (w *WorkingSet) Entity(ctx context.Context, id string) (string, error) {
	records, err := w.store.Records(ctx, []string{id})
	if err != nil || len(records) == 0 {
		return "", err
	}
	return records[0].Entity, nil
}

// elementSeparator joins a file and an element key in an element entity. A key
// may hold "#" ("init#2") and ":" ("rule:pred/2@3fa2c1"); a workspace-relative
// path holds neither "::" nor a key.
const elementSeparator = "::"

// ElementEntity is the working-set entity of one element of a file: its
// observations are dated by the element's own bytes, not the file's.
func ElementEntity(file, key string) string { return file + elementSeparator + key }

// EntityFile is the file an entity is in: the entity itself for a file, the
// file part of an element entity.
func EntityFile(entity string) string {
	file, _, _ := strings.Cut(entity, elementSeparator)
	return file
}

// Revision is content identity, not HEAD: uncommitted edits invalidate views.
//
// A file entity is the hash of the whole file. An element entity
// (ElementEntity) is the element's own revision -- its bytes, doc comment
// included (codemodel.Revision) -- so an edit elsewhere in the file leaves the
// observations of the element current; "absent" once the file no longer has
// it. With whole-file revisions every edit staled every observation of every
// function in the file: measured on one campaign (2026-09-22), one file read
// 34 times and 21 of 45 Go reads re-reads.
func (w *WorkingSet) Revision(entity string) string {
	if entity == "" {
		return "nonfile"
	}
	file, key, element := strings.Cut(entity, elementSeparator)
	path, err := tools.ResolveWorkspacePath(context.Background(), w.root, file)
	if err != nil {
		return "unavailable"
	}
	if element {
		data, err := os.ReadFile(path)
		if err != nil {
			return "unavailable"
		}
		model, ok := codemodel.Parse(file, string(data))
		if !ok {
			return "unavailable"
		}
		for i := range model.Elements {
			if model.Elements[i].Key == key {
				return model.Elements[i].Revision
			}
		}
		return "absent"
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

// LedgerEntry is one tool result a working request carries whole.
type LedgerEntry struct {
	Call  string // the tool call it answers
	ID    string // the observation it holds
	Bytes int    // its rendered size, with any harness text appended to it
	Round int    // the round it entered
}

// LedgerDecision is the working policy's answer for the round being prepared.
type LedgerDecision struct {
	// Evict names the calls whose results a compaction moves out behind their
	// recall handles (working_evict); empty when the ledger is under its
	// ceiling (no working_compact).
	Evict []string
	// Restate names the observations still carried whose file changed after
	// they were made, and that have not been restated at its current revision
	// (working_restate).
	Restate []string
}

// Ledger asks the working policy what this round's request does with its
// ledger: whether it compacts and what goes, and which observations the
// harness restates. round is the latest round in the ledger; restated maps an
// observation to the revision it was last restated at. Go measures (sizes,
// rounds, file revisions); the policy decides.
//
// hot are the files an observation of which is pinned past the age cut
// (working_pinned): the loop's focus and the files it has written.
func (w *WorkingSet) Ledger(ctx context.Context, entries []LedgerEntry, round int, restated map[string]string, hot []string) (LedgerDecision, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return LedgerDecision{}, err
	}
	var facts []mangle.Fact
	add := func(p string, args ...any) { facts = append(facts, mangle.Fact{Predicate: p, Args: args}) }
	add("working_round_now", int64(round))
	var ids []string
	seenID := map[string]bool{}
	for _, e := range entries {
		add("working_ledger", e.Call, e.ID, int64(e.Bytes), int64(e.Round))
		if !seenID[e.ID] {
			seenID[e.ID] = true
			ids = append(ids, e.ID)
		}
	}
	records, err := w.store.Records(ctx, ids)
	if err != nil {
		return LedgerDecision{}, err
	}
	seenEntity := map[string]bool{}
	for _, r := range records {
		add("working_observation", r.ID, r.Entity, r.Revision, r.Kind, r.Step)
		add("working_digest", r.ID, r.Digest)
		if r.End > 0 {
			add("working_span", r.ID, r.Start, r.End)
		}
		if !seenEntity[r.Entity] {
			seenEntity[r.Entity] = true
			add("working_revision", r.Entity, w.Revision(r.Entity))
			add("working_entity_file", r.Entity, EntityFile(r.Entity))
		}
	}
	for id, revision := range restated {
		add("working_restated", id, revision)
	}
	seenHot := map[string]bool{}
	for _, file := range hot {
		if file != "" && !seenHot[file] {
			seenHot[file] = true
			add("working_hot", file)
		}
	}
	// Replace, not accumulate: every one of these describes this round only.
	if err := w.engine.ReplaceControlFacts(facts, "working_round_now", "working_ledger", "working_observation", "working_digest", "working_span", "working_revision", "working_restated", "working_entity_file", "working_hot"); err != nil {
		return LedgerDecision{}, err
	}
	var decision LedgerDecision
	evict, err := w.engine.Query(ctx, "working_evict(Call)")
	if err != nil {
		return LedgerDecision{}, err
	}
	for _, row := range evict.Bindings {
		if call, _ := row["Call"].(string); call != "" {
			decision.Evict = append(decision.Evict, call)
		}
	}
	restate, err := w.engine.Query(ctx, "working_restate(ID)")
	if err != nil {
		return LedgerDecision{}, err
	}
	for _, row := range restate.Bindings {
		if id, _ := row["ID"].(string); id != "" {
			decision.Restate = append(decision.Restate, id)
		}
	}
	sort.Strings(decision.Evict)
	sort.Strings(decision.Restate)
	return decision, nil
}

// Observations returns the archived records with these ids, in no particular
// order; an id the store does not hold is left out.
func (w *WorkingSet) Observations(ctx context.Context, ids []string) ([]WorkingRecord, error) {
	return w.store.Records(ctx, ids)
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
	Regime       string // the regime the round just ran under ("" open, "commit", "repair")

	StructuralAttempts int // structural queries that ran (find_symbol, callers_of, ...)
	StructuralMisses   int // of those, the ones that errored or returned no rows
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
	// SearchOpen is working_search_open: the raw search tools are offered.
	SearchOpen bool
}

// Continue asks policy whether observed execution should continue. A stop is
// an unresolved task, never a successful completion witness.
func (w *WorkingSet) Continue(ctx context.Context, p WorkingProgress) (WorkingDecision, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Plain "/"-prefixed strings: the engine encodes those as name atoms,
	// exactly like the types.MangleAtom form used for user_intent below
	// (convertValueToTypedTerm honours both; the old "MangleAtom became a
	// string constant" failure is pinned fixed in the mangle package).
	flag := "/no"
	if p.Cycle {
		flag = "/yes"
	}
	intent := "/read"
	if p.WriteIntent {
		intent = "/write"
	}
	regime := "/open"
	switch strings.TrimPrefix(strings.TrimSpace(p.Regime), "/") {
	case "commit":
		regime = "/commit"
	case "repair":
		regime = "/repair"
	}
	facts := []mangle.Fact{
		{Predicate: "working_control", Args: []any{flag, int64(p.FailedRounds)}},
		{Predicate: "working_progress", Args: []any{intent, int64(p.Rounds), int64(p.Writes), int64(p.SinceWrite), int64(p.SinceVerify)}},
		{Predicate: "working_regime_now", Args: []any{regime}},
		{Predicate: "working_structural", Args: []any{int64(p.StructuralAttempts), int64(p.StructuralMisses)}},
	}
	// Control facts are not file-keyed and their derivations must not outlive
	// them; ReplaceFactsForFile did neither (see Engine.ReplaceControlFacts).
	if err := w.engine.ReplaceControlFacts(facts, "working_control", "working_progress", "working_regime_now", "working_structural"); err != nil {
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
	open, err := w.engine.Query(ctx, "working_search_open()")
	if err != nil {
		return WorkingDecision{}, err
	}
	decision.SearchOpen = len(open.Bindings) > 0
	rows, err := w.engine.Query(ctx, "working_continue()")
	if err != nil {
		return WorkingDecision{}, err
	}
	decision.Continue = len(rows.Bindings) > 0
	return decision, nil
}

// RepeatThreshold is the policy's span for a deterministic trace cycle
// (working_repeat_threshold). The loop is the only side that can see the tool
// trace, so it does the measuring and reports the verdict as working_control/2;
// the span it measures against is policy's to set, like the nudge, commit,
// finalize and stall spans beside it. Read through QueryFacts: the query path
// answers nothing for a fact derived from a base fact alone, while the rules
// that join on it evaluate fine.
func (w *WorkingSet) RepeatThreshold(context.Context) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	facts := w.engine.QueryFacts("working_repeat_threshold")
	if len(facts) == 0 || len(facts[0].Args) != 1 {
		return 0, fmt.Errorf("working policy declares no working_repeat_threshold")
	}
	n, err := strconv.Atoi(fmt.Sprint(facts[0].Args[0]))
	if err != nil || n < 2 {
		return 0, fmt.Errorf("working_repeat_threshold must be at least 2, got %v", facts[0].Args[0])
	}
	return n, nil
}
