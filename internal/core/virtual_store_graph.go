package core

import (
	"fmt"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/world"

	"github.com/google/mangle/ast"
)

// SetGraphQuery sets the graph query interface for world model access.
func (v *VirtualStore) SetGraphQuery(gq world.GraphQuery) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.graphQuery = gq
	logging.VirtualStoreDebug("GraphQuery interface attached for Mangle-World bridge")
}

// getQueryGraphAtoms handles query_graph(Type, Params, Result).
func (v *VirtualStore) getQueryGraphAtoms(query ast.Atom) ([]ast.Atom, error) {
	if len(query.Args) != 3 {
		return nil, nil
	}

	v.mu.RLock()
	gq := v.graphQuery
	v.mu.RUnlock()

	if gq == nil {
		return nil, nil
	}

	// 1. Extract QueryType
	qTypeTerm, ok := query.Args[0].(ast.Constant)
	if !ok {
		return nil, nil
	}
	qType := cleanMangleString(qTypeTerm.String())

	// 2. Extract Params (Map or List or String)
	// For simplicity, we assume Params is passed as a Map-like structure or just raw args
	// But Mangle AST might be complex. Let'sക്കു support a simple key-value map if passed as a Map.
	// Or maybe Params is just a string/list.
	// Let's assume Params is a Map for now.
	params := make(map[string]interface{})
	if mapTerm, ok := query.Args[1].(ast.Map); ok {
		for k, val := range mapTerm.Values {
			params[cleanMangleString(k.String())] = cleanMangleString(val.String())
		}
	} else {
		// Fallback: treat as single "arg" param
		params["arg"] = cleanMangleString(query.Args[1].String())
	}

	// 3. Execute Query
	result, err := gq.QueryGraph(qType, params)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Warn("Graph query failed: %v", err)
		return nil, nil
	}

	// 4. Bind Result
	// Result should be converted to Mangle AST
	resTerm, err := goToMangleTerm(result)
	if err != nil {
		return nil, err
	}

	// Return fact: query_graph(Type, Params, ResultVal)
	return []ast.Atom{ast.NewAtom("query_graph", query.Args[0], query.Args[1], resTerm)}, nil
}

// Helper to clean Mangle strings (remove quotes, leading slashes)
func cleanMangleString(s string) string {
	s = strings.TrimPrefix(s, "/")
	s = strings.Trim(s, """)
	return s
}

// Helper to convert Go types to Mangle AST terms
func goToMangleTerm(val interface{}) (ast.BaseTerm, error) {
	switch v := val.(type) {
	case string:
		return ast.String(v), nil
	case int:
		return ast.Number(int64(v)), nil
	case int64:
		return ast.Number(v), nil
	case bool:
		if v {
			return ast.TrueConstant, nil
		}
		return ast.FalseConstant, nil
	case []string:
		list := make([]ast.BaseTerm, len(v))
		for i, s := range v {
			list[i] = ast.String(s)
		}
		return ast.List(list), nil
	default:
		return ast.String(fmt.Sprintf("%v", v)), nil
	}
}