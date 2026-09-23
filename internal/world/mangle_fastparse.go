package world

import (
	"codenerd/internal/core"
	"codenerd/internal/world/codemodel"
	"fmt"
)

func extractMangleSymbolFacts(path string, content string) []core.Fact {
	statements := codemodel.SplitMangleStatements(content)
	seen := make(map[string]struct{}, len(statements))

	facts := make([]core.Fact, 0, len(statements))
	for _, st := range statements {
		head, _ := codemodel.MangleHead(st.Text)
		pred, arity := codemodel.ManglePredicate(head)
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
			Args:      []any{symbolID, "/predicate", "/public", path, sig},
		})
	}
	return facts
}
