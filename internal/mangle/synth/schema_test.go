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

	var schema map[string]any
	err := json.Unmarshal([]byte(jsonStr), &schema)
	if err != nil {
		t.Fatalf("SchemaV1JSON() returned invalid JSON: %v", err)
	}

	if schema["type"] != "object" {
		t.Errorf("Expected type object, got %v", schema["type"])
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be an object")
	}

	format, ok := props["format"].(map[string]any)
	if !ok {
		t.Fatal("Expected format to be an object")
	}

	if format["type"] != "string" {
		t.Errorf("Expected format type string, got %v", format["type"])
	}

	if _, ok := props["program"]; !ok {
		t.Error("Missing 'program' in root properties")
	}
}

func TestSchemaV1SingleClauseJSON(t *testing.T) {
	jsonStr := SchemaV1SingleClauseJSON()
	if jsonStr == "" {
		t.Fatal("SchemaV1SingleClauseJSON() returned empty string")
	}

	var schema map[string]any
	err := json.Unmarshal([]byte(jsonStr), &schema)
	if err != nil {
		t.Fatalf("SchemaV1SingleClauseJSON() returned invalid JSON: %v", err)
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok || props["format"] == nil {
		t.Fatal("SchemaV1SingleClauseJSON() missing 'format' property")
	}

	program, _ := props["program"].(map[string]any)
	progProps, _ := program["properties"].(map[string]any)
	clauses, ok := progProps["clauses"].(map[string]any)
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

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("Schema missing 'properties' map")
	}

	if _, ok := props["format"]; !ok {
		t.Error("Expected format in properties")
	}

	program, ok := props["program"].(map[string]any)
	if !ok {
		t.Fatal("Schema missing 'program' property")
	}

	programProps, ok := program["properties"].(map[string]any)
	if !ok {
		t.Fatal("Schema program missing 'properties' map")
	}

	clauses, ok := programProps["clauses"].(map[string]any)
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

	// Check that we can marshal it without errors.
	_, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Failed to marshal map from BuildSchemaV1: %v", err)
	}
}

func TestBuildSchemaV1SingleClause(t *testing.T) {
	schema := BuildSchemaV1SingleClause()
	if schema == nil {
		t.Fatal("BuildSchemaV1SingleClause() returned nil")
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("Schema missing 'properties' map")
	}
	program, ok := props["program"].(map[string]any)
	if !ok {
		t.Fatal("Schema missing 'program' property")
	}
	programProps, ok := program["properties"].(map[string]any)
	if !ok {
		t.Fatal("Schema program missing 'properties' map")
	}
	clauses, ok := programProps["clauses"].(map[string]any)
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
		schema := map[string]any{"foo": "bar"}
		res := marshalSchema(schema)
		expected := `{"foo":"bar"}`
		if res != expected {
			t.Errorf("marshalSchema() expected %s, got %s", expected, res)
		}
	})

	t.Run("unmarshalable schema", func(t *testing.T) {
		schema := map[string]any{
			"bad": math.NaN(),
		}
		res := marshalSchema(schema)
		if res != "" {
			t.Errorf("Expected empty string for unmarshalable schema, got %q", res)
		}
	})

	t.Run("invalid schema types", func(t *testing.T) {
		// Test invalid (types that cannot be marshaled to JSON, like function or channel)
		invalid := map[string]any{"func": func() {}}
		if marshalSchema(invalid) != "" {
			t.Error("marshalSchema(invalid) should return empty string on marshal error")
		}
	})
}

func TestBuildSchema(t *testing.T) {
	// We've already tested singleClause = true/false indirectly,
	// let's do a direct verification of buildSchema.
	s1 := buildSchema(false)
	s2 := buildSchema(true)

	if s1 == nil || s2 == nil {
		t.Fatal("buildSchema returned nil")
	}
}

// Helpers testing
func TestSchemaObject(t *testing.T) {
	obj := schemaObject(map[string]any{"test": "prop"}, "test")
	if obj["type"] != "object" {
		t.Errorf("Expected type=object, got %v", obj["type"])
	}
	props := obj["properties"].(map[string]any)
	if props["test"] != "prop" {
		t.Errorf("Expected property test=prop, got %v", props["test"])
	}
	req := obj["required"].([]string)
	if len(req) != 1 || req[0] != "test" {
		t.Errorf("Expected required=[test], got %v", req)
	}

	// Test with nil props
	objNil := schemaObject(nil)
	if objNil["properties"] == nil {
		t.Error("schemaObject(nil) should initialize an empty properties map")
	}
	if _, ok := objNil["required"]; ok {
		t.Error("schemaObject with no required fields should not have 'required' key")
	}
}

func TestSchemaArray(t *testing.T) {
	items := map[string]any{"type": "string"}
	arr := schemaArray(items)
	if arr["type"] != "array" {
		t.Errorf("Expected type=array, got %v", arr["type"])
	}
	if arr["items"] == nil {
		t.Error("items should not be nil")
	}
}

func TestSchemaString(t *testing.T) {
	if s := schemaString(); s["type"] != "string" {
		t.Errorf("Expected string type, got %v", s["type"])
	}
}

func TestSchemaNumber(t *testing.T) {
	if s := schemaNumber(); s["type"] != "number" {
		t.Errorf("Expected number type, got %v", s["type"])
	}
}

func TestSchemaInteger(t *testing.T) {
	if s := schemaInteger(); s["type"] != "integer" {
		t.Errorf("Expected integer type, got %v", s["type"])
	}
}

func TestSchemaEnum(t *testing.T) {
	s := schemaEnum("a", "b")
	if s["type"] != "string" {
		t.Errorf("Expected enum type string, got %v", s["type"])
	}
	enum := s["enum"].([]string)
	if len(enum) != 2 || enum[0] != "a" || enum[1] != "b" {
		t.Errorf("Expected enum=[a, b], got %v", enum)
	}
}

func TestSubSchemas(t *testing.T) {
	// Test the specific builders just to cover their struct generation
	expr1 := exprSchema(nil)
	if expr1["type"] != "object" {
		t.Error("exprSchema(nil) failed")
	}

	expr2 := exprSchema(expr1)
	if expr2["type"] != "object" {
		t.Error("exprSchema(valid) failed")
	}

	atom := atomSchema(expr2)
	if atom["type"] != "object" {
		t.Error("atomSchema failed")
	}

	term := termSchema(atom, expr2)
	if term["type"] != "object" {
		t.Error("termSchema failed")
	}

	transformStmt := transformStmtSchema(expr2)
	if transformStmt["type"] != "object" {
		t.Error("transformStmtSchema failed")
	}

	transform := transformSchema(transformStmt)
	if transform["type"] != "object" {
		t.Error("transformSchema failed")
	}

	clause := clauseSchema(atom, term, transform)
	if clause["type"] != "object" {
		t.Error("clauseSchema failed")
	}

	decl := declSchema(atom, expr2)
	if decl["type"] != "object" {
		t.Error("declSchema failed")
	}

	bound := boundSchema(expr2)
	if bound["type"] != "object" {
		t.Error("boundSchema failed")
	}

	pkg := packageSchema(atom)
	if pkg["type"] != "object" {
		t.Error("packageSchema failed")
	}

	use := useSchema(atom)
	if use["type"] != "object" {
		t.Error("useSchema failed")
	}
}
