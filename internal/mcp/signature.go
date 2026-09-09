package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// A tool's input schema is the second-largest cost in the whole surface, after
// results. A JSON Schema for a moderately configurable tool runs 200-600 tokens
// once pretty-printed, and the agent needs all of it exactly once — in the turn
// where it actually constructs the call.
//
// So the schema is disclosed in two steps. Everywhere the agent is BROWSING,
// it sees a signature: one line, roughly fifteen tokens, carrying the argument
// names, their types, and which are required. That is enough to choose a tool.
// The full schema arrives only when the agent asks for it, or when it gets an
// argument wrong — see ValidateArgs, which answers a bad call with the schema
// rather than with a complaint.

// signatureMaxParams bounds how many parameters one signature names before it
// summarises the tail. A tool with thirty parameters cannot be called from a
// signature anyway; that is what the full schema is for.
const signatureMaxParams = 8

// jsonSchema is the subset of JSON Schema this package reads. Everything else
// a server may send is preserved verbatim in InputSchema and returned on demand
// — this type exists to render and to validate, never to re-emit.
type jsonSchema struct {
	Type                 string                `json:"type"`
	Description          string                `json:"description"`
	Properties           map[string]jsonSchema `json:"properties"`
	Required             []string              `json:"required"`
	Enum                 []any                 `json:"enum"`
	Items                *jsonSchema           `json:"items"`
	AdditionalProperties *bool                 `json:"additionalProperties"`
	Default              any                   `json:"default"`
}

func parseSchema(raw json.RawMessage) (jsonSchema, bool) {
	if len(raw) == 0 {
		return jsonSchema{}, false
	}
	var s jsonSchema
	if err := json.Unmarshal(raw, &s); err != nil {
		return jsonSchema{}, false
	}
	return s, true
}

// shortType renders a JSON Schema type as a single token.
func (s jsonSchema) shortType() string {
	switch s.Type {
	case "string":
		if len(s.Enum) > 0 {
			// An enum is worth spending characters on: it converts a guess
			// into a choice, which is the difference between one call and
			// three.
			return enumToken(s.Enum)
		}
		return "str"
	case "integer":
		return "int"
	case "number":
		return "num"
	case "boolean":
		return "bool"
	case "array":
		if s.Items != nil {
			return "[]" + s.Items.shortType()
		}
		return "[]"
	case "object":
		return "obj"
	case "":
		if len(s.Enum) > 0 {
			return enumToken(s.Enum)
		}
		return "any"
	default:
		return s.Type
	}
}

// enumMaxValues bounds an inline enum; past that the signature says how many
// there are and the full schema carries the list.
const enumMaxValues = 4

func enumToken(values []any) string {
	parts := make([]string, 0, len(values))
	for i, v := range values {
		if i >= enumMaxValues {
			parts = append(parts, fmt.Sprintf("+%d", len(values)-enumMaxValues))
			break
		}
		parts = append(parts, fmt.Sprintf("%v", v))
	}
	return strings.Join(parts, "|")
}

// ToolSignature renders one tool as a callable one-liner.
//
// Required parameters come first and unadorned; optional ones follow in square
// brackets. That ordering is the point — an agent reading left to right sees
// what it must supply before what it may.
func ToolSignature(tool *MCPTool) string {
	if tool == nil {
		return ""
	}
	return renderSignature(tool.Name, tool.InputSchema)
}

func renderSignature(name string, rawSchema json.RawMessage) string {
	schema, ok := parseSchema(rawSchema)
	if !ok || len(schema.Properties) == 0 {
		return name + "()"
	}

	required := make(map[string]bool, len(schema.Required))
	for _, r := range schema.Required {
		required[r] = true
	}

	names := make([]string, 0, len(schema.Properties))
	for k := range schema.Properties {
		names = append(names, k)
	}
	// Required first, then alphabetical within each group, so the same tool
	// always renders the same line.
	sort.Slice(names, func(i, j int) bool {
		if required[names[i]] != required[names[j]] {
			return required[names[i]]
		}
		return names[i] < names[j]
	})

	var req, opt []string
	shown := 0
	for _, n := range names {
		if shown >= signatureMaxParams {
			break
		}
		token := n + ": " + schema.Properties[n].shortType()
		if required[n] {
			req = append(req, token)
		} else {
			opt = append(opt, token)
		}
		shown++
	}

	var sb strings.Builder
	sb.WriteString(name)
	sb.WriteByte('(')
	sb.WriteString(strings.Join(req, ", "))
	if len(opt) > 0 {
		if len(req) > 0 {
			sb.WriteString(", ")
		}
		sb.WriteByte('[')
		sb.WriteString(strings.Join(opt, ", "))
		sb.WriteByte(']')
	}
	if remaining := len(names) - shown; remaining > 0 {
		sb.WriteString(fmt.Sprintf(", +%d more", remaining))
	}
	sb.WriteByte(')')
	return sb.String()
}

// ToolLine renders the browsing view of one tool: signature, one-line purpose,
// and risk class.
//
// Risk is on the line rather than a level down because it changes whether the
// agent should call the tool at all, and a property that changes the decision
// belongs where the decision is made.
func ToolLine(tool *MCPTool) string {
	if tool == nil {
		return ""
	}
	line := ToolSignature(tool)
	if tool.Condensed != "" {
		line += " — " + tool.Condensed
	}
	if tool.Risk != "" && tool.Risk != RiskSafe {
		line += " [" + string(tool.Risk) + "]"
	}
	return line
}

// FullSchema renders the complete input schema, bounded.
//
// Bounded even here, because "full" is a disclosure tier and not an escape from
// the budget: a server that ships a 40 KB schema should cost one expensive
// lookup, not the rest of the context window.
func FullSchema(tool *MCPTool, maxBytes int) string {
	if tool == nil || len(tool.InputSchema) == 0 {
		return "{}"
	}
	var v any
	if err := json.Unmarshal(tool.InputSchema, &v); err != nil {
		return truncate(string(tool.InputSchema), maxBytes)
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return truncate(string(tool.InputSchema), maxBytes)
	}
	return truncate(string(pretty), maxBytes)
}

// ArgumentError reports a call whose arguments do not satisfy the tool's
// declared schema.
//
// It carries the schema. That is the whole design: the moment an agent gets an
// argument wrong is the moment it has proven it needs the schema, and it is the
// only moment where spending those tokens is certainly worth it. Refusing the
// call and returning a bare "missing required parameter" costs a turn and
// teaches nothing.
type ArgumentError struct {
	ToolID  string
	Reasons []string
	Schema  string
}

func (e *ArgumentError) Error() string {
	return fmt.Sprintf("invalid arguments for %s: %s", e.ToolID, strings.Join(e.Reasons, "; "))
}

// ValidateArgs checks arguments against a tool's declared input schema.
//
// This is intentionally shallow: required-presence and top-level type
// agreement, nothing more. A full JSON Schema validator would reject payloads
// that the target server would have accepted — servers are routinely laxer than
// their own published schema — and a client-side gate that is stricter than the
// server is a gate that blocks working calls. The purpose is to catch the two
// mistakes that actually happen (a forgotten required field, a string where a
// number belongs) early enough to answer them with the schema.
func ValidateArgs(tool *MCPTool, args map[string]any) error {
	if tool == nil {
		return fmt.Errorf("tool is required")
	}
	schema, ok := parseSchema(tool.InputSchema)
	if !ok || len(schema.Properties) == 0 {
		return nil
	}

	var reasons []string

	for _, name := range schema.Required {
		if _, present := args[name]; !present {
			reasons = append(reasons, fmt.Sprintf("missing required %q (%s)",
				name, schema.Properties[name].shortType()))
		}
	}

	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Deterministic reason ordering.

	for _, k := range keys {
		prop, declared := schema.Properties[k]
		if !declared {
			// Not an error. additionalProperties defaults to true in JSON
			// Schema, and many servers accept extras they never documented.
			continue
		}
		if !typeAgrees(prop, args[k]) {
			reasons = append(reasons, fmt.Sprintf("%q expects %s, got %s",
				k, prop.shortType(), goKindName(args[k])))
		}
	}

	if len(reasons) == 0 {
		return nil
	}
	return &ArgumentError{
		ToolID:  tool.ToolID,
		Reasons: reasons,
		Schema:  FullSchema(tool, 4000),
	}
}

// typeAgrees reports whether a Go value could serialize to the declared type.
func typeAgrees(prop jsonSchema, value any) bool {
	if value == nil {
		// A declared-but-null value is the server's business: JSON Schema
		// treats null as its own type and plenty of servers accept it as
		// "unset".
		return true
	}
	switch prop.Type {
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "number", "integer":
		switch value.(type) {
		case int, int8, int16, int32, int64, float32, float64, json.Number:
			return true
		}
		return false
	case "array":
		switch value.(type) {
		case []any, []string, []int, []float64:
			return true
		}
		return false
	case "object":
		_, ok := value.(map[string]any)
		return ok
	default:
		// No declared type, or a type this validator does not model. Accepting
		// is correct: an unmodelled keyword must not become a client-side
		// refusal of a call the server would have served.
		return true
	}
}

func goKindName(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "str"
	case bool:
		return "bool"
	case int, int8, int16, int32, int64, float32, float64, json.Number:
		return "num"
	case []any, []string, []int, []float64:
		return "array"
	case map[string]any:
		return "obj"
	default:
		return fmt.Sprintf("%T", value)
	}
}
