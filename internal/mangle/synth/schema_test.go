package synth

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestSchemaV1JSON(t *testing.T) {
	jsonStr := SchemaV1JSON()
	if jsonStr == "" {
		t.Fatal("SchemaV1JSON() returned empty string")
	}
	if !strings.Contains(jsonStr, "format") {
		t.Error("SchemaV1JSON() expected to contain 'format'")
	}

	var schema map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &schema)
	if err != nil {
		t.Fatalf("SchemaV1JSON() returned invalid JSON: %v", err)
	}

	if schema["type"] != "object" {
		t.Errorf("Expected type object, got %v", schema["type"])
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected properties to be an object")
	}

	format, ok := props["format"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected format to be an object")
	}

	if format["type"] != "string" {
		t.Errorf("Expected format type string, got %v", format["type"])
	}
}

func TestSchemaV1SingleClauseJSON(t *testing.T) {
	jsonStr := SchemaV1SingleClauseJSON()
	if jsonStr == "" {
		t.Fatal("SchemaV1SingleClauseJSON() returned empty string")
	}

	var schema map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &schema)
	if err != nil {
		t.Fatalf("SchemaV1SingleClauseJSON() returned invalid JSON: %v", err)
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok || props["format"] == nil {
		t.Fatal("SchemaV1SingleClauseJSON() missing 'format' property")
	}

	program, _ := props["program"].(map[string]interface{})
	progProps, _ := program["properties"].(map[string]interface{})
	clauses, ok := progProps["clauses"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected clauses to be defined in schema")
	}

	minItems, ok := clauses["minItems"].(float64)
	if !ok || minItems != 1 {
		t.Errorf("Expected clauses minItems 1, got %v", clauses["minItems"])
	}

	maxItems, ok := clauses["maxItems"].(float64)
	if !ok || maxItems != 1 {
		t.Errorf("Expected clauses maxItems 1, got %v", clauses["maxItems"])
	}
}

func TestBuildSchemaV1(t *testing.T) {
	schema := BuildSchemaV1()
	if schema == nil {
		t.Fatal("BuildSchemaV1() returned nil")
	}

	if schema["type"] != "object" {
		t.Errorf("Expected schema type object, got %v", schema["type"])
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing 'properties' map")
	}

	if _, ok := props["format"]; !ok {
		t.Error("Expected format in properties")
	}

	program, ok := props["program"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing 'program' property")
	}

	programProps, ok := program["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema program missing 'properties' map")
	}

	clauses, ok := programProps["clauses"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema program missing 'clauses' property")
	}

	if _, hasMin := clauses["minItems"]; hasMin {
		t.Error("BuildSchemaV1() should not have minItems constraint on clauses")
	}
	if _, hasMax := clauses["maxItems"]; hasMax {
		t.Error("BuildSchemaV1() should not have maxItems constraint on clauses")
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("Expected required to be a string array")
	}

	hasFormat := false
	hasProgram := false
	for _, req := range required {
		if req == "format" {
			hasFormat = true
		}
		if req == "program" {
			hasProgram = true
		}
	}

	if !hasFormat || !hasProgram {
		t.Errorf("Expected format and program in required, got %v", required)
	}
}

func TestBuildSchemaV1SingleClause(t *testing.T) {
	schema := BuildSchemaV1SingleClause()
	if schema == nil {
		t.Fatal("BuildSchemaV1SingleClause() returned nil")
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing 'properties' map")
	}
	program, ok := props["program"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema missing 'program' property")
	}
	programProps, ok := program["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema program missing 'properties' map")
	}
	clauses, ok := programProps["clauses"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected clauses to be defined in schema")
	}

	minItems, ok := clauses["minItems"].(int)
	if !ok || minItems != 1 {
		t.Errorf("Expected clauses minItems 1, got %v", clauses["minItems"])
	}

	maxItems, ok := clauses["maxItems"].(int)
	if !ok || maxItems != 1 {
		t.Errorf("Expected clauses maxItems 1, got %v", clauses["maxItems"])
	}
}

func TestMarshalSchema(t *testing.T) {
	t.Run("nil schema", func(t *testing.T) {
		res := marshalSchema(nil)
		if res != "" {
			t.Errorf("marshalSchema(nil) expected empty string, got %q", res)
		}
	})

	t.Run("valid schema", func(t *testing.T) {
		schema := map[string]interface{}{"foo": "bar"}
		res := marshalSchema(schema)
		expected := `{"foo":"bar"}`
		if res != expected {
			t.Errorf("marshalSchema() expected %s, got %s", expected, res)
		}
	})

	t.Run("unmarshalable schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"bad": math.NaN(),
		}
		res := marshalSchema(schema)
		if res != "" {
			t.Errorf("Expected empty string for unmarshalable schema, got %q", res)
		}
	})
}
