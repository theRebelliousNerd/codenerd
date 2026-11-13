package context

import (
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// entityView is one context_view row: the facts of pred (of that arity) whose
// argument at pos is a selected entity describe the entity.
type entityView struct {
	pred  string
	arity int
	pos   int
	rank  int64
	focal bool // shown only of the entities the turn is working on
}

// prioritized is an entity the kernel selected for the active context
// (should_include_context) with its priority.
type prioritized struct {
	factStr  string
	priority float64
}

// entityViews reads the kernel's decision about what the window shows of a
// selected entity: the views (context_view), in rank order, and the entities
// the turn is working on (context_focal). Both are asked of the session's
// whole kernel. No view derived means no view applied; the caller still
// resolves entities against the facts that mention them. Caller holds c.mu.
func (c *Compressor) entityViews() ([]entityView, map[string]bool) {
	rows, err := c.asker().Query("context_view")
	if err != nil || len(rows) == 0 {
		if err != nil {
			logging.Get(logging.CategoryContext).Warn("entity views: context_view query failed, no view applied: %v", err)
		}
		return nil, nil
	}
	views := make([]entityView, 0, len(rows))
	for _, r := range rows {
		if len(r.Args) != 5 {
			continue
		}
		pred := strings.TrimPrefix(fmt.Sprint(r.Args[0]), "/")
		arity, okArity := r.Args[1].(int64)
		pos, okPos := r.Args[2].(int64)
		rank, okRank := r.Args[3].(int64)
		if pred == "" || !okArity || !okPos || !okRank || pos < 0 || pos >= arity {
			logging.Get(logging.CategoryContext).Warn("entity views: malformed context_view row %s ignored", r.String())
			continue
		}
		views = append(views, entityView{
			pred: pred, arity: int(arity), pos: int(pos), rank: rank,
			focal: fmt.Sprint(r.Args[4]) == "/focal",
		})
	}
	// Rank first; within a rank the order is the view's own identity, so the
	// same policy always yields the same block.
	sort.SliceStable(views, func(i, j int) bool {
		a, b := views[i], views[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if a.pred != b.pred {
			return a.pred < b.pred
		}
		return a.pos < b.pos
	})

	focal := make(map[string]bool)
	focalRows, err := c.asker().Query("context_focal")
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("entity views: context_focal query failed, focal views not applied: %v", err)
	}
	for _, r := range focalRows {
		if len(r.Args) == 1 {
			if e := fmt.Sprint(r.Args[0]); e != "" {
				focal[e] = true
			}
		}
	}
	return views, focal
}

// viewFacts carries out the kernel's view decision for the selected entities:
// rank by rank, and within a rank entity by entity in priority order, the rows
// of each view that applies to the entity, read from the session's whole
// kernel through the argument index. Each fact carries its entity's priority.
// entities is sorted by priority descending. Caller holds c.mu.
//
// Reading stops once the rows read fill budget (tokens): selection takes them
// in this order, so a row past the budget could not be selected, and a turn
// with a wide import neighbourhood selects hundreds of entities.
//
// Until this existed a selected entity was resolved to "every fact that
// mentions it" among the compressor's own kernel's facts. In a chat session
// that kernel is the Cortex's catch-all shard, which holds none of the world
// model: a file the kernel selected at the top priority brought no symbol,
// import or failure fact into the window.
func (c *Compressor) viewFacts(entities []prioritized, budget int) []ScoredFact {
	views, focal := c.entityViews()
	if len(views) == 0 || len(entities) == 0 {
		return nil
	}
	reader := c.asker()
	var out []ScoredFact
	failed := make(map[string]bool)
	counter := NewTokenCounter()
	used := 0
	for _, v := range views {
		for _, e := range entities {
			if used >= budget {
				return out
			}
			if v.focal && !focal[e.factStr] {
				continue
			}
			if failed[v.pred] {
				break
			}
			rows, err := core.BoundRows(reader, v.pred, v.arity, map[int]any{v.pos: e.factStr})
			if err != nil {
				// A view naming a predicate the kernel cannot answer is a
				// policy defect; say so once and go on with the others.
				logging.Get(logging.CategoryContext).Warn("entity views: view %s/%d could not be read, skipped: %v", v.pred, v.arity, err)
				failed[v.pred] = true
				break
			}
			sortFactsBySpelling(rows)
			for _, f := range rows {
				out = append(out, ScoredFact{Fact: f, Score: e.priority})
				used += counter.CountFact(f)
			}
		}
	}
	return out
}

// sortFactsBySpelling orders one view's rows for one entity by their text, so
// the block does not depend on the order a shard happened to return them in.
func sortFactsBySpelling(rows []core.Fact) {
	if len(rows) < 2 {
		return
	}
	keys := make([]string, len(rows))
	for i, f := range rows {
		keys[i] = f.String()
	}
	idx := make([]int, len(rows))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return keys[idx[a]] < keys[idx[b]] })
	sorted := make([]core.Fact, len(rows))
	for i, j := range idx {
		sorted[i] = rows[j]
	}
	copy(rows, sorted)
}
