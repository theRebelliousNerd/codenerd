package synth

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSchemaV1JSON(t *testing.T) {
	schemaStr := SchemaV1JSON()
	if schemaStr == "" {
		t.Fatal("SchemaV1JSON() returned empty string")
	}

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
		t.Fatalf("Failed to parse SchemaV1JSON() as JSON: %v", err)
	}

	// Validate presence of basic structure
	if schema["type"] != "object" {
		t.Errorf("Expected root type 'object', got %v", schema["type"])
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing properties object in root schema")
	}

	if _, ok := props["program"]; !ok {
		t.Error("Missing 'program' in root properties")
	}
}

func TestSchemaV1SingleClauseJSON(t *testing.T) {
	schemaStr := SchemaV1SingleClauseJSON()
	if schemaStr == "" {
		t.Fatal("SchemaV1SingleClauseJSON() returned empty string")
	}

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
		t.Fatalf("Failed to parse SchemaV1SingleClauseJSON() as JSON: %v", err)
	}

	// Validate minItems/maxItems on clauses
	props, _ := schema["properties"].(map[string]interface{})
	program, _ := props["program"].(map[string]interface{})
	progProps, _ := program["properties"].(map[string]interface{})
	clauses, _ := progProps["clauses"].(map[string]interface{})

	if min, ok := clauses["minItems"].(float64); !ok || min != 1 {
		t.Errorf("Expected single clause schema to have minItems=1, got %v", clauses["minItems"])
	}
	if max, ok := clauses["maxItems"].(float64); !ok || max != 1 {
		t.Errorf("Expected single clause schema to have maxItems=1, got %v", clauses["maxItems"])
	}
}

func TestBuildSchemaV1(t *testing.T) {
	schema := BuildSchemaV1()
	if schema == nil {
		t.Fatal("BuildSchemaV1() returned nil")
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

	props, _ := schema["properties"].(map[string]interface{})
	program, _ := props["program"].(map[string]interface{})
	progProps, _ := program["properties"].(map[string]interface{})
	clauses, _ := progProps["clauses"].(map[string]interface{})

	if clauses["minItems"] != 1 || clauses["maxItems"] != 1 {
		t.Errorf("BuildSchemaV1SingleClause did not set correct items constraints: min=%v, max=%v", clauses["minItems"], clauses["maxItems"])
	}
}

func TestMarshalSchema(t *testing.T) {
	// Test nil
	if marshalSchema(nil) != "" {
		t.Error("marshalSchema(nil) should return empty string")
	}

	// Test valid
	valid := map[string]interface{}{"type": "string"}
	res := marshalSchema(valid)
	if !strings.Contains(res, `"type":"string"`) {
		t.Errorf("marshalSchema(valid) unexpected result: %v", res)
	}

	// Test invalid (types that cannot be marshaled to JSON, like function or channel)
	invalid := map[string]interface{}{"func": func() {}}
	if marshalSchema(invalid) != "" {
		t.Error("marshalSchema(invalid) should return empty string on marshal error")
	}
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
	obj := schemaObject(map[string]interface{}{"test": "prop"}, "test")
	if obj["type"] != "object" {
		t.Errorf("Expected type=object, got %v", obj["type"])
	}
	props := obj["properties"].(map[string]interface{})
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
	items := map[string]interface{}{"type": "string"}
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
