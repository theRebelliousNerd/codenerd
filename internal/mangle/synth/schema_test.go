package synth

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildSchemaV1(t *testing.T) {
	schema := BuildSchemaV1()

	if schema == nil {
		t.Fatal("BuildSchemaV1() returned nil")
	}

	rootProps, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected root properties to be a map, got %v", schema["properties"])
	}

	formatObj, ok := rootProps["format"].(map[string]interface{})
	if !ok || formatObj["type"] != "string" {
		t.Errorf("Expected format to be a string schema, got %v", formatObj)
	}

	programObj, ok := rootProps["program"].(map[string]interface{})
	if !ok || programObj["type"] != "object" {
		t.Fatalf("Expected program to be an object schema, got %v", programObj)
	}

	props, ok := programObj["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected program properties, got %v", programObj["properties"])
	}

	clauses, ok := props["clauses"].(map[string]interface{})
	if !ok || clauses["type"] != "array" {
		t.Fatalf("Expected clauses to be an array schema, got %v", clauses)
	}

	if _, hasMin := clauses["minItems"]; hasMin {
		t.Error("Expected no minItems for multi-clause schema")
	}
	if _, hasMax := clauses["maxItems"]; hasMax {
		t.Error("Expected no maxItems for multi-clause schema")
	}
}

func TestBuildSchemaV1SingleClause(t *testing.T) {
	schema := BuildSchemaV1SingleClause()

	if schema == nil {
		t.Fatal("BuildSchemaV1SingleClause() returned nil")
	}

	rootProps, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected root properties to be a map, got %v", schema["properties"])
	}

	programObj, ok := rootProps["program"].(map[string]interface{})
	if !ok || programObj["type"] != "object" {
		t.Fatalf("Expected program to be an object schema, got %v", programObj)
	}

	props, ok := programObj["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected program properties, got %v", programObj["properties"])
	}

	clauses, ok := props["clauses"].(map[string]interface{})
	if !ok || clauses["type"] != "array" {
		t.Fatalf("Expected clauses to be an array schema, got %v", clauses)
	}

	if min, ok := clauses["minItems"].(int); !ok || min != 1 {
		t.Errorf("Expected minItems = 1, got %v", clauses["minItems"])
	}
	if max, ok := clauses["maxItems"].(int); !ok || max != 1 {
		t.Errorf("Expected maxItems = 1, got %v", clauses["maxItems"])
	}
}

func TestMarshalSchema(t *testing.T) {
	// 1. nil map
	if got := marshalSchema(nil); got != "" {
		t.Errorf("marshalSchema(nil) = %q, want \"\"", got)
	}

	// 2. valid map
	validMap := map[string]interface{}{"type": "string"}
	gotValid := marshalSchema(validMap)
	if !strings.Contains(gotValid, `"type":"string"`) {
		t.Errorf("marshalSchema(validMap) = %q, want containing \"type\":\"string\"", gotValid)
	}

	// 3. invalid map (contains unmarshalable value, e.g., chan or func)
	invalidMap := map[string]interface{}{"type": make(chan int)}
	if got := marshalSchema(invalidMap); got != "" {
		t.Errorf("marshalSchema(invalidMap) = %q, want \"\"", got)
	}
}

func TestSchemaJSONSingletons(t *testing.T) {
	json1 := SchemaV1JSON()
	if json1 == "" {
		t.Fatal("SchemaV1JSON() returned empty string")
	}

	var parsed1 map[string]interface{}
	if err := json.Unmarshal([]byte(json1), &parsed1); err != nil {
		t.Errorf("SchemaV1JSON() returned invalid JSON: %v", err)
	}

	json2 := SchemaV1SingleClauseJSON()
	if json2 == "" {
		t.Fatal("SchemaV1SingleClauseJSON() returned empty string")
	}

	var parsed2 map[string]interface{}
	if err := json.Unmarshal([]byte(json2), &parsed2); err != nil {
		t.Errorf("SchemaV1SingleClauseJSON() returned invalid JSON: %v", err)
	}

	// Make sure calling them again returns the same JSON string (tests sync.Once)
	if got := SchemaV1JSON(); got != json1 {
		t.Error("SchemaV1JSON() returned a different string on second call")
	}
	if got := SchemaV1SingleClauseJSON(); got != json2 {
		t.Error("SchemaV1SingleClauseJSON() returned a different string on second call")
	}
}
