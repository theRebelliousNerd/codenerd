package core

import (
	"strconv"
	"strings"
)

// BoundQuery spells the pattern query for pred's rows whose string arguments
// at the given positions equal the given values: every other position is a
// fresh variable. The kernel answers a pattern from the store's argument
// index, so a caller that wants one file's rows of a world-sized relation
// does not copy the relation into Go to find them.
//
// Before this, per-turn and per-file code asked for whole relations
// (dependency_link: 251k rows, code_calls: 136k on this repository) and
// filtered them in a loop, once per file the model opened.
func BoundQuery(pred string, arity int, bound map[int]string) string {
	var b strings.Builder
	b.WriteString(pred)
	b.WriteByte('(')
	for i := range arity {
		if i > 0 {
			b.WriteString(", ")
		}
		if v, ok := bound[i]; ok {
			b.WriteString(strconv.Quote(v))
			continue
		}
		b.WriteString("V")
		b.WriteString(strconv.Itoa(i))
	}
	b.WriteByte(')')
	return b.String()
}
