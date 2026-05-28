package synth

import (
	"encoding/json"
	"sync"
)

var (
	schemaV1Once             sync.Once
	schemaV1JSON             string
	schemaV1SingleClauseOnce sync.Once
	schemaV1SingleClauseJSON string
)

// SchemaV1JSON returns a JSON schema string for MangleSynth (multi-clause).
func SchemaV1JSON() string {
	schemaV1Once.Do(func() {
		schemaV1JSON = marshalSchema(buildSchema(false))
	})
	return schemaV1JSON
}

// SchemaV1SingleClauseJSON returns a JSON schema string enforcing a single clause.
func SchemaV1SingleClauseJSON() string {
	schemaV1SingleClauseOnce.Do(func() {
		schemaV1SingleClauseJSON = marshalSchema(buildSchema(true))
	})
	return schemaV1SingleClauseJSON
}

// BuildSchemaV1 exposes the schema map for provider-specific clients.
func BuildSchemaV1() map[string]any {
	return buildSchema(false)
}

// BuildSchemaV1SingleClause exposes the schema map for a single-clause schema.
func BuildSchemaV1SingleClause() map[string]any {
	return buildSchema(true)
}

func marshalSchema(schema map[string]any) string {
	if schema == nil {
		return ""
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return ""
	}
	return string(data)
}

func buildSchema(singleClause bool) map[string]any {
	exprArg := exprSchema(nil)
	expr := exprSchema(exprArg)
	atom := atomSchema(expr)
	term := termSchema(atom, expr)
	transformStmt := transformStmtSchema(expr)
	transform := transformSchema(transformStmt)
	clause := clauseSchema(atom, term, transform)

	clauses := schemaArray(clause)
	if singleClause {
		clauses["minItems"] = 1
		clauses["maxItems"] = 1
	}

	decl := declSchema(atom, expr)
	pkg := packageSchema(atom)
	use := useSchema(atom)

	program := schemaObject(map[string]any{
		"package": pkg,
		"use":     schemaArray(use),
		"decls":   schemaArray(decl),
		"clauses": clauses,
	}, "clauses")

	return schemaObject(map[string]any{
		"format":  schemaEnum(FormatV1),
		"program": program,
	}, "format", "program")
}

func exprSchema(argItems map[string]any) map[string]any {
	props := map[string]any{
		"kind":     schemaString(),
		"value":    schemaString(),
		"number":   schemaNumber(),
		"float":    schemaNumber(),
		"function": schemaString(),
		"arity":    schemaInteger(),
	}
	if argItems != nil {
		props["args"] = schemaArray(argItems)
	} else {
		props["args"] = schemaArray(schemaObject(nil))
	}
	return schemaObject(props, "kind")
}

func atomSchema(expr map[string]any) map[string]any {
	return schemaObject(map[string]any{
		"pred": schemaString(),
		"args": schemaArray(expr),
	}, "pred")
}

func termSchema(atom, expr map[string]any) map[string]any {
	return schemaObject(map[string]any{
		"kind":  schemaString(),
		"atom":  atom,
		"left":  expr,
		"right": expr,
		"op":    schemaString(),
	}, "kind")
}

func transformStmtSchema(expr map[string]any) map[string]any {
	return schemaObject(map[string]any{
		"kind": schemaString(),
		"var":  schemaString(),
		"fn":   expr,
	}, "kind", "fn")
}

func transformSchema(stmt map[string]any) map[string]any {
	return schemaObject(map[string]any{
		"statements": schemaArray(stmt),
	}, "statements")
}

func clauseSchema(atom, term, transform map[string]any) map[string]any {
	return schemaObject(map[string]any{
		"head":      atom,
		"body":      schemaArray(term),
		"transform": transform,
	}, "head")
}

func declSchema(atom, expr map[string]any) map[string]any {
	return schemaObject(map[string]any{
		"atom":      atom,
		"descr":     schemaArray(atom),
		"bounds":    schemaArray(boundSchema(expr)),
		"inclusion": schemaArray(atom),
	}, "atom")
}

func boundSchema(expr map[string]any) map[string]any {
	return schemaObject(map[string]any{
		"terms": schemaArray(expr),
	}, "terms")
}

func packageSchema(atom map[string]any) map[string]any {
	return schemaObject(map[string]any{
		"name":  schemaString(),
		"atoms": schemaArray(atom),
	}, "name")
}

func useSchema(atom map[string]any) map[string]any {
	return schemaObject(map[string]any{
		"name":  schemaString(),
		"atoms": schemaArray(atom),
	}, "name")
}

func schemaObject(props map[string]any, required ...string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	obj := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		obj["required"] = required
	}
	return obj
}

func schemaArray(items map[string]any) map[string]any {
	return map[string]any{
		"type":  "array",
		"items": items,
	}
}

func schemaString() map[string]any {
	return map[string]any{
		"type": "string",
	}
}

func schemaNumber() map[string]any {
	return map[string]any{
		"type": "number",
	}
}

func schemaInteger() map[string]any {
	return map[string]any{
		"type": "integer",
	}
}

func schemaEnum(values ...string) map[string]any {
	return map[string]any{
		"type": "string",
		"enum": values,
	}
}
