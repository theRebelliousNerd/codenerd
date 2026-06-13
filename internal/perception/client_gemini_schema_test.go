package perception

import "testing"

func TestShallowSchema_NilDefaultsToObject(t *testing.T) {
	got := shallowSchema(nil)
	if got["type"] != "object" {
		t.Errorf("shallowSchema(nil)=%v, want type=object", got)
	}
	if _, hasProps := got["properties"]; hasProps {
		t.Error("a nil schema should not carry properties")
	}
}

func TestShallowSchema_FlattensProperties(t *testing.T) {
	in := map[string]any{
		"type":     "object",
		"required": []any{"name"},
		"properties": map[string]any{
			"name":   map[string]any{"type": "string"},
			"count":  map[string]any{"type": "integer"},
			"nested": map[string]any{"type": "object", "properties": map[string]any{"deep": map[string]any{"type": "string"}}},
			"mode":   map[string]any{"enum": []any{"a", "b"}},
		},
	}
	got := shallowSchema(in)
	if got["type"] != "object" {
		t.Fatalf("type=%v, want object", got["type"])
	}
	if got["required"] == nil {
		t.Error("required should be preserved")
	}
	props, ok := got["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties missing or wrong type: %T", got["properties"])
	}
	// A nested object is flattened to just {"type":"object"} (no deep props).
	nested, _ := props["nested"].(map[string]any)
	if nested["type"] != "object" {
		t.Errorf("nested property type=%v, want object", nested["type"])
	}
	if _, hasDeep := nested["properties"]; hasDeep {
		t.Error("shallowSchema must drop nested sub-properties")
	}
	// An enum property keeps its enum and becomes type=string.
	mode, _ := props["mode"].(map[string]any)
	if mode["type"] != "string" || mode["enum"] == nil {
		t.Errorf("enum property=%v, want type=string with enum preserved", mode)
	}
}

func TestShallowSchemaProperty_Variants(t *testing.T) {
	// Enum wins and forces type=string.
	enum := shallowSchemaProperty(map[string]any{"enum": []any{"x"}, "type": "number"})
	if enum["type"] != "string" || enum["enum"] == nil {
		t.Errorf("enum variant=%v, want type=string with enum", enum)
	}
	// A typed property keeps its type.
	typed := shallowSchemaProperty(map[string]any{"type": "boolean"})
	if typed["type"] != "boolean" {
		t.Errorf("typed variant=%v, want type=boolean", typed)
	}
	// A non-map / untyped value falls back to type=string.
	fallback := shallowSchemaProperty("not a map")
	if fallback["type"] != "string" {
		t.Errorf("fallback variant=%v, want type=string", fallback)
	}
}
