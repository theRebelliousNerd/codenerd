package synth

import (
	"encoding/json"
	"sync"
)

var (
	schemaV1Once            sync.Once
	schemaV1JSON            string
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
func BuildSchemaV1() map[string]interface{} {
	return buildSchema(false)
}

// BuildSchemaV1SingleClause exposes the schema map for a single-clause schema.
func BuildSchemaV1SingleClause() map[string]interface{} {
	return buildSchema(true)
}

func marshalSchema(schema map[string]interface{}) string {
	if schema == nil {
		return ""
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return ""
	}
	return string(data)
}

func buildSchema(singleClause bool) map[string]interface{} {
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

	program := schemaObject(map[string]interface{}{
		"package": pkg,
		"use":     schemaArray(use),
		"decls":   schemaArray(decl),
		"clauses": clauses,
	}, "clauses")

	return schemaObject(map[string]interface{}{
		"format": schemaEnum(FormatV1),
		"program": program,
	}, "format", "program")
}

func exprSchema(argItems map[string]interface{}) map[string]interface{} {
	props := map[string]interface{}{
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

func atomSchema(expr map[string]interface{}) map[string]interface{} {
	return schemaObject(map[string]interface{}{
		"pred": schemaString(),
		"args": schemaArray(expr),
	}, "pred")
}

func termSchema(atom, expr map[string]interface{}) map[string]interface{} {
	return schemaObject(map[string]interface{}{
		"kind":  schemaString(),
		"atom":  atom,
		"left":  expr,
		"right": expr,
		"op":    schemaString(),
	}, "kind")
}

func transformStmtSchema(expr map[string]interface{}) map[string]interface{} {
	return schemaObject(map[string]interface{}{
		"kind": schemaString(),
		"var":  schemaString(),
		"fn":   expr,
	}, "kind", "fn")
}

func transformSchema(stmt map[string]interface{}) map[string]interface{} {
	return schemaObject(map[string]interface{}{
		"statements": schemaArray(stmt),
	}, "statements")
}

func clauseSchema(atom, term, transform map[string]interface{}) map[string]interface{} {
	return schemaObject(map[string]interface{}{
		"head":      atom,
		"body":      schemaArray(term),
		"transform": transform,
	}, "head")
}

func declSchema(atom, expr map[string]interface{}) map[string]interface{} {
	return schemaObject(map[string]interface{}{
		"atom":      atom,
		"descr":     schemaArray(atom),
		"bounds":    schemaArray(boundSchema(expr)),
		"inclusion": schemaArray(atom),
	}, "atom")
}

func boundSchema(expr map[string]interface{}) map[string]interface{} {
	return schemaObject(map[string]interface{}{
		"terms": schemaArray(expr),
	}, "terms")
}

func packageSchema(atom map[string]interface{}) map[string]interface{} {
	return schemaObject(map[string]interface{}{
		"name":  schemaString(),
		"atoms": schemaArray(atom),
	}, "name")
}

func useSchema(atom map[string]interface{}) map[string]interface{} {
	return schemaObject(map[string]interface{}{
		"name":  schemaString(),
		"atoms": schemaArray(atom),
	}, "name")
}

func schemaObject(props map[string]interface{}, required ...string) map[string]interface{} {
	if props == nil {
		props = map[string]interface{}{}
	}
	obj := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		obj["required"] = required
	}
	return obj
}

func schemaArray(items map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":  "array",
		"items": items,
	}
}

func schemaString() map[string]interface{} {
	return map[string]interface{}{
		"type": "string",
	}
}

func schemaNumber() map[string]interface{} {
	return map[string]interface{}{
		"type": "number",
	}
}

func schemaInteger() map[string]interface{} {
	return map[string]interface{}{
		"type": "integer",
	}
}

func schemaEnum(values ...string) map[string]interface{} {
	return map[string]interface{}{
		"type": "string",
		"enum": values,
	}
}
