package context_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"codenerd/internal/articulation"
)

// Every memory operation the protocol admits must be handled where memory
// operations are handled.
//
// The two ends of this had drifted apart with nothing to notice. The schema in
// internal/articulation validates four operations, the prompt atom
// protocol/piggyback/memory_ops describes all four to the model, and
// Compressor.processMemoryOperation had cases for three. A "note" was
// therefore legal on the way in, produced on request, and then fell out of a
// switch with no default -- accepted, validated, and dropped.
//
// The check reads the schema constant rather than a copy of it, so adding an
// operation to the enum fails here until something handles it. That is the
// point: the enum is a promise to the model, and this is where the promise is
// kept.
func TestEveryProtocolMemoryOpIsHandled(t *testing.T) {
	want := memoryOpEnum(t)
	if len(want) == 0 {
		t.Fatal("found no memory-operation enum in the piggyback schema; if it " +
			"moved, this test needs to follow it rather than be deleted")
	}

	got := switchCases(t, "compressor_turns.go", "processMemoryOperation")
	if len(got) == 0 {
		t.Fatal("found no cases in processMemoryOperation; this test is asserting nothing")
	}

	for _, op := range want {
		if !got[op] {
			t.Errorf("memory operation %q is admitted by the protocol schema and "+
				"described to the model, but processMemoryOperation has no case for it: "+
				"it is validated on the way in and then silently dropped", op)
		}
	}

	// The reverse is a smaller problem but still a drift: a case for something
	// the model can never send is dead code that reads as live capability.
	for op := range got {
		found := false
		for _, w := range want {
			if w == op {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("processMemoryOperation handles %q, which the protocol schema "+
				"does not admit; the model cannot send it", op)
		}
	}
}

// memoryOpEnum pulls the operation enum out of the live schema constant by
// walking the parsed JSON, so it cannot fall out of date with it.
func memoryOpEnum(t *testing.T) []string {
	t.Helper()
	var schema map[string]any
	if err := json.Unmarshal([]byte(articulation.PiggybackEnvelopeSchema), &schema); err != nil {
		t.Fatalf("the piggyback schema is not valid JSON: %v", err)
	}
	node := dig(schema,
		"properties", "control_packet",
		"properties", "memory_operations",
		"items", "properties", "op")
	obj, ok := node.(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := obj["enum"].([]any)
	if !ok {
		return nil
	}
	ops := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			ops = append(ops, s)
		}
	}
	return ops
}

func dig(node any, path ...string) any {
	for _, key := range path {
		obj, ok := node.(map[string]any)
		if !ok {
			return nil
		}
		node = obj[key]
	}
	return node
}

// switchCases returns the string case values of every switch in the named
// function.
func switchCases(t *testing.T, file, fn string) map[string]bool {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	cases := make(map[string]bool)
	for _, decl := range parsed.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if !ok || f.Name.Name != fn {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			clause, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, expr := range clause.List {
				lit, ok := expr.(*ast.BasicLit)
				if ok && lit.Kind == token.STRING {
					cases[strip(lit.Value)] = true
				}
			}
			return true
		})
	}
	return cases
}

func strip(quoted string) string {
	if len(quoted) >= 2 {
		return quoted[1 : len(quoted)-1]
	}
	return quoted
}
