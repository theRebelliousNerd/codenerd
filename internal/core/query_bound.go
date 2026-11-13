package core

import (
	"strconv"
	"strings"

	"codenerd/internal/types"
)

// BoundQuery spells the pattern query for pred's rows whose arguments at the
// given positions equal the given values -- a string, or a MangleAtom for a
// name constant: every other position is a fresh variable. The kernel answers a pattern from the store's argument
// index, so a caller that wants one file's rows of a world-sized relation
// does not copy the relation into Go to find them.
//
// Before this, per-turn and per-file code asked for whole relations
// (dependency_link: 251k rows, code_calls: 136k on this repository) and
// filtered them in a loop, once per file the model opened.
func BoundQuery(pred string, arity int, bound map[int]any) string {
	var b strings.Builder
	b.WriteString(pred)
	b.WriteByte('(')
	for i := range arity {
		if i > 0 {
			b.WriteString(", ")
		}
		switch v := bound[i].(type) {
		case MangleAtom:
			b.WriteString(string(v))
		case string:
			b.WriteString(strconv.Quote(v))
		default:
			b.WriteString("V")
			b.WriteString(strconv.Itoa(i))
		}
	}
	b.WriteByte(')')
	return b.String()
}

// QueryBound answers BoundQuery(pred, arity, bound) from the store's argument
// index.
func (k *RealKernel) QueryBound(pred string, arity int, bound map[int]any) ([]Fact, error) {
	return k.Query(BoundQuery(pred, arity, bound))
}

// QueryBound routes the bound query as Query routes its predicate.
func (c *CortexKernel) QueryBound(pred string, arity int, bound map[int]any) ([]types.Fact, error) {
	return c.Query(BoundQuery(pred, arity, bound))
}

// BoundRows returns pred's rows whose arguments at the bound positions equal
// the bound values (a string, or a MangleAtom for a name constant). A querier
// with an argument index (QueryBound: the kernels) answers from it; any other
// querier -- a narrow interface, a test's fake that answers bare predicates --
// is asked for the predicate and its rows are filtered here, so the answer is
// the same either way and no caller depends on a Query implementation
// understanding pattern strings.
func BoundRows(q interface {
	Query(predicate string) ([]Fact, error)
}, pred string, arity int, bound map[int]any) ([]Fact, error) {
	if indexed, ok := q.(interface {
		QueryBound(pred string, arity int, bound map[int]any) ([]Fact, error)
	}); ok {
		return indexed.QueryBound(pred, arity, bound)
	}
	rows, err := q.Query(pred)
	if err != nil {
		return nil, err
	}
	out := make([]Fact, 0, len(rows))
rows:
	for _, f := range rows {
		if len(f.Args) != arity {
			continue
		}
		for pos, want := range bound {
			if pos < 0 || pos >= arity {
				continue rows
			}
			var w string
			switch v := want.(type) {
			case MangleAtom:
				w = string(v)
			case string:
				w = v
			default:
				continue
			}
			if types.ExtractString(f.Args[pos]) != w {
				continue rows
			}
		}
		out = append(out, f)
	}
	return out, nil
}
