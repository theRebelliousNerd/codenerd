package world

import (
	"codenerd/internal/core"
	"fmt"
)

func extractMangleSymbolFacts(path string, content string) []core.Fact {
	statements := splitMangleStatements(content)
	seen := make(map[string]struct{}, len(statements))

	facts := make([]core.Fact, 0, len(statements))
	for _, st := range statements {
		head, _ := splitMangleHead(st.Text)
		pred, arity := parseManglePredicateAndArity(head)
		if pred == "" {
			continue
		}
		sig := fmt.Sprintf("%s/%d", pred, arity)
		symbolID := "pred:" + sig
		if _, ok := seen[symbolID]; ok {
			continue
		}
		seen[symbolID] = struct{}{}
		facts = append(facts, core.Fact{
			Predicate: "symbol_graph",
			Args:      []interface{}{symbolID, "/predicate", "/public", path, sig},
		})
	}
	return facts
}

