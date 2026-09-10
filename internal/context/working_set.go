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
	}{r, current, current != r.Revision, offset, total, min(total, offset+limit)})
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

// Continue asks policy whether observed execution should continue. A stop is
// an unresolved task, never a successful completion witness.
func (w *WorkingSet) Continue(ctx context.Context, cycle bool, failedRounds int) (bool, string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	flag := types.MangleAtom("/no")
	if cycle {
		flag = types.MangleAtom("/yes")
	}
	if err := w.engine.ReplaceFactsForFile("working-control", []mangle.Fact{{Predicate: "working_control", Args: []any{flag, int64(failedRounds)}}}); err != nil {
		return false, "", err
	}
	stops, err := w.engine.Query(ctx, "working_stop(Reason)")
	if err != nil {
		return false, "", err
	}
	if len(stops.Bindings) > 0 {
		return false, fmt.Sprint(stops.Bindings[0]["Reason"]), nil
	}
	decision, err := w.engine.Query(ctx, "working_continue()")
	if err != nil {
		return false, "", err
	}
	return len(decision.Bindings) > 0, "", nil
}

// Select refreshes a bounded dependency slice, then asks the canonical Mangle
// context rules which entities matter. Only the selected record bodies load.
func (w *WorkingSet) Select(ctx context.Context, focus string, recent []string, charBudget int) (WorkingSelection, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return WorkingSelection{}, err
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
	}
	for _, id := range recent {
		add("working_recent", id)
	}
	if err := w.engine.ReplaceFactsForFile("working-runtime", facts); err != nil {
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
	w.selector.config.AtomReserve = max(1, charBudget/8)
	selectedFacts := w.selector.buildKernelDerivedContext(decisions, worldFacts)
	for _, f := range selectedFacts {
		line := f.Fact.String() + "\n"
		if text.Len()+len(line) <= charBudget/2 {
			text.WriteString(line)
		}
	}
	for _, r := range records {
		if priorities[r.ID] == 0 {
			selection.Omitted = append(selection.Omitted, r.ID)
			continue
		}
		body, length, err := w.store.Read(ctx, r.ID, 0, 16000)
		if err != nil {
			return selection, err
		}
		header := fmt.Sprintf("\n[observation id=%q entity=%q revision=%q failed=%t]\n", r.ID, r.Entity, r.Revision, r.Failed)
		if length > len([]rune(body.Body)) || text.Len()+len(header)+len(body.Body)+1 > charBudget {
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
	logging.Context("Working context: candidates=%d selected=%d omitted=%d chars=%d focus=%q", len(records), len(selection.Selected), len(selection.Omitted), text.Len(), focus)
	return selection, nil
}
