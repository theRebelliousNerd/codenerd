package synth

import (
	"encoding/json"
	"math"
	"testing"
)

func TestSchemaV1JSON(t *testing.T) {
	schemaStr := SchemaV1JSON()
	if schemaStr == "" {
		t.Fatal("SchemaV1JSON() returned empty string")
	}

	var schema map[string]interface{}
	err := json.Unmarshal([]byte(schemaStr), &schema)
	if err != nil {
		t.Fatalf("SchemaV1JSON() returned invalid JSON: %v", err)
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok || props["format"] == nil {
		t.Error("SchemaV1JSON() missing 'format' property")
	}
}

func TestSchemaV1SingleClauseJSON(t *testing.T) {
	schemaStr := SchemaV1SingleClauseJSON()
	if schemaStr == "" {
		t.Fatal("SchemaV1SingleClauseJSON() returned empty string")
	}

	var schema map[string]interface{}
	err := json.Unmarshal([]byte(schemaStr), &schema)
	if err != nil {
		t.Fatalf("SchemaV1SingleClauseJSON() returned invalid JSON: %v", err)
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok || props["format"] == nil {
		t.Error("SchemaV1SingleClauseJSON() missing 'format' property")
	}
}

func TestBuildSchemaV1(t *testing.T) {
	schema := BuildSchemaV1()
	if schema == nil {
		t.Fatal("BuildSchemaV1() returned nil")
	}

	// Verify it's a multi-clause schema
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
		t.Fatal("Schema program missing 'clauses' property")
	}

	if _, hasMin := clauses["minItems"]; hasMin {
		t.Error("BuildSchemaV1() should not have minItems constraint on clauses")
	}
	if _, hasMax := clauses["maxItems"]; hasMax {
		t.Error("BuildSchemaV1() should not have maxItems constraint on clauses")
	}
}

func TestBuildSchemaV1SingleClause(t *testing.T) {
	schema := BuildSchemaV1SingleClause()
	if schema == nil {
		t.Fatal("BuildSchemaV1SingleClause() returned nil")
	}

	// Verify it's a single-clause schema
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
		t.Fatal("Schema program missing 'clauses' property")
	}

	if clauses["minItems"] != 1 {
		t.Errorf("BuildSchemaV1SingleClause() expected minItems=1, got %v", clauses["minItems"])
	}
	if clauses["maxItems"] != 1 {
		t.Errorf("BuildSchemaV1SingleClause() expected maxItems=1, got %v", clauses["maxItems"])
	}
}

func TestMarshalSchema(t *testing.T) {
	t.Run("nil schema", func(t *testing.T) {
		res := marshalSchema(nil)
		if res != "" {
			t.Errorf("marshalSchema(nil) expected empty string, got %v", res)
		}
	})

	t.Run("valid schema", func(t *testing.T) {
		schema := map[string]interface{}{"foo": "bar"}
		res := marshalSchema(schema)
		expected := `{"foo":"bar"}`
		if res != expected {
			t.Errorf("marshalSchema() expected %v, got %v", expected, res)
		}
	})

	t.Run("invalid schema", func(t *testing.T) {
		// math.NaN() cannot be marshaled to JSON by encoding/json
		schema := map[string]interface{}{"invalid": math.NaN()}
		res := marshalSchema(schema)
		if res != "" {
			t.Errorf("marshalSchema(invalid) expected empty string, got %v", res)
		}
	})
}
