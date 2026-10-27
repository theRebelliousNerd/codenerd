package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/core"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// CompleteWithSchema implements the capability consumed by the scheduler,
// tracing client, broker and perception transducers. Schema constraints not
// supported by Anthropic's grammar are still enforced locally before returning
// data to codeNERD. An unsupported/open-ended schema never falls back silently
// to unvalidated prompt-only JSON inside this adapter.
func (c *AnthropicClient) CompleteWithSchema(ctx context.Context, system, user, schemaText string) (string, error) {
	if !c.SchemaCapable() {
		return "", core.ErrSchemaNotSupported
	}
	if err := c.admitInput(system, userTurn(user), nil); err != nil {
		return "", err
	}
	parsed, parseErr := jsonschema.UnmarshalJSON(strings.NewReader(schemaText))
	schema, ok := parsed.(map[string]any)
	if parseErr != nil || !ok {
		return "", fmt.Errorf("invalid Anthropic JSON schema: expected a schema object")
	}
	wire, validator, err := prepareAnthropicSchema(schema)
	if err != nil {
		return "", err
	}
	req, err := c.request(ctx, system, []AnthropicMessage{{Role: "user", Content: user}}, nil)
	if err != nil {
		return "", err
	}
	if req.OutputConfig == nil {
		req.OutputConfig = &AnthropicOutputConfig{}
	}
	req.OutputConfig.Format = &AnthropicOutputFormat{Type: "json_schema", Schema: wire}
	result, err := c.completeText(ctx, req, "CompleteWithSchema", usageOpChat)
	if err != nil {
		return "", err
	}
	instance, err := jsonschema.UnmarshalJSON(strings.NewReader(result))
	if err != nil {
		return "", fmt.Errorf("Anthropic structured output is not valid JSON")
	}
	if err := validator.Validate(instance); err != nil {
		return "", fmt.Errorf("Anthropic structured output does not match the requested JSON schema")
	}
	return result, nil
}

func compileAnthropicSchema(schema map[string]any) (*jsonschema.Schema, error) {
	// Tool definitions are built in Go and commonly contain []string or int
	// values. Canonicalize to JSON types before handing them to the validator.
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("encode JSON schema: %w", err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("decode JSON schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	// Schema input is data. Do not let $ref/$id cause file or network reads.
	// Standard metaschemas are embedded by the validator; other resources must
	// be within this document.
	compiler.UseLoader(jsonschema.SchemeURLLoader{})
	const location = "https://codenerd.invalid/anthropic-schema.json"
	if err := compiler.AddResource(location, document); err != nil {
		return nil, fmt.Errorf("invalid JSON schema: %w", err)
	}
	compiled, err := compiler.Compile(location)
	if err != nil {
		return nil, fmt.Errorf("invalid or externally referenced JSON schema: %w", err)
	}
	return compiled, nil
}

func prepareAnthropicSchema(schema map[string]any) (map[string]any, *jsonschema.Schema, error) {
	if schema == nil {
		return nil, nil, fmt.Errorf("JSON schema is missing")
	}
	validator, err := compileAnthropicSchema(schema)
	if err != nil {
		return nil, nil, err
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, nil, fmt.Errorf("encode JSON schema: %w", err)
	}
	parsed, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}
	wire := parsed.(map[string]any)
	if err := normalizeAnthropicSchema(wire); err != nil {
		return nil, nil, err
	}
	return wire, validator, nil
}

// Normalize only schema nodes, never property names, defaults or enum values.
// Anthropic requires closed objects. An object with no declared shape could
// carry arbitrary tool arguments/control data; closing that object would erase
// its contract, so reject it explicitly instead.
func normalizeAnthropicSchema(node map[string]any) error {
	if typ, _ := node["type"].(string); typ == "object" || node["properties"] != nil {
		props, _ := node["properties"].(map[string]any)
		additionalAllowed, _ := node["additionalProperties"].(bool)
		// false is the only permitted value. Distinguish absent from false.
		_, hasAdditional := node["additionalProperties"]
		if len(props) == 0 && (!hasAdditional || additionalAllowed) {
			return fmt.Errorf("Anthropic structured output cannot represent an open object without declared properties; use native tool calls or a closed schema")
		}
		if extra, ok := node["additionalProperties"]; ok && extra != false {
			return fmt.Errorf("Anthropic structured output requires additionalProperties=false")
		}
		node["additionalProperties"] = false
	}
	for _, key := range []string{"minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf", "minLength", "maxLength", "maxItems", "uniqueItems"} {
		if value, exists := node[key]; exists {
			// Keep removed constraints visible to the model as guidance; the
			// original schema validator above remains the acceptance gate.
			desc, _ := node["description"].(string)
			encoded, _ := json.Marshal(value)
			node["description"] = strings.TrimSpace(desc + " " + key + "=" + string(encoded) + ".")
			delete(node, key)
		}
	}
	minimum, _ := node["minItems"].(json.Number)
	if count, _ := minimum.Float64(); count > 1 {
		desc, _ := node["description"].(string)
		node["description"] = fmt.Sprintf("%s minItems=%s.", desc, minimum)
		delete(node, "minItems")
	}
	delete(node, "$schema")
	for _, key := range []string{"properties", "$defs", "definitions", "patternProperties"} {
		if children, ok := node[key].(map[string]any); ok {
			for _, child := range children {
				if m, ok := child.(map[string]any); ok {
					if err := normalizeAnthropicSchema(m); err != nil {
						return err
					}
				}
			}
		}
	}
	for _, key := range []string{"items", "not", "if", "then", "else", "additionalProperties"} {
		if child, ok := node[key].(map[string]any); ok {
			if err := normalizeAnthropicSchema(child); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"anyOf", "allOf", "oneOf", "prefixItems"} {
		if children, ok := node[key].([]any); ok {
			for _, child := range children {
				if m, ok := child.(map[string]any); ok {
					if err := normalizeAnthropicSchema(m); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func validateAnthropicToolCall(call AnthropicContentBlock, defs []ToolDefinition) error {
	for _, def := range defs {
		if def.Name != call.Name {
			continue
		}
		validator, err := compileAnthropicSchema(def.InputSchema)
		if err != nil {
			return err
		}
		if err := validator.Validate(call.Input); err != nil {
			return fmt.Errorf("Anthropic tool %q arguments do not match the requested JSON schema", call.Name)
		}
		return nil
	}
	return fmt.Errorf("Anthropic requested an undeclared tool %q", call.Name)
}
