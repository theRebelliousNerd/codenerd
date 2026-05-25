package synth

import (
	"encoding/json"
	"testing"
)

func TestSchemaV1JSON(t *testing.T) {
	jsonStr := SchemaV1JSON()
	if jsonStr == "" {
		t.Fatal("SchemaV1JSON returned empty string")
	}

	var schema map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &schema)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify sync.Once works by calling it again
	jsonStr2 := SchemaV1JSON()
	if jsonStr != jsonStr2 {
		t.Fatal("SchemaV1JSON did not return the exact same string on second call")
	}
}

func TestSchemaV1SingleClauseJSON(t *testing.T) {
	jsonStr := SchemaV1SingleClauseJSON()
	if jsonStr == "" {
		t.Fatal("SchemaV1SingleClauseJSON returned empty string")
	}

	var schema map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &schema)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify sync.Once works by calling it again
	jsonStr2 := SchemaV1SingleClauseJSON()
	if jsonStr != jsonStr2 {
		t.Fatal("SchemaV1SingleClauseJSON did not return the exact same string on second call")
	}
}

func TestBuildSchemaV1(t *testing.T) {
	schema := BuildSchemaV1()
	if schema == nil {
		t.Fatal("BuildSchemaV1 returned nil")
	}

	// verify format
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema properties not found")
	}

	formatProp, ok := props["format"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema format not found")
	}

	enumList, ok := formatProp["enum"].([]string)
	if !ok || len(enumList) == 0 || enumList[0] != FormatV1 {
		t.Errorf("Expected enum to contain %s", FormatV1)
	}

	// verify clauses property
	programProp, ok := props["program"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema program not found")
	}

	progProps, ok := programProp["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema program properties not found")
	}

	clausesProp, ok := progProps["clauses"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema program clauses not found")
	}

	if _, hasMin := clausesProp["minItems"]; hasMin {
		t.Errorf("BuildSchemaV1 should not have minItems for clauses")
	}
	if _, hasMax := clausesProp["maxItems"]; hasMax {
		t.Errorf("BuildSchemaV1 should not have maxItems for clauses")
	}
}

func TestBuildSchemaV1SingleClause(t *testing.T) {
	schema := BuildSchemaV1SingleClause()
	if schema == nil {
		t.Fatal("BuildSchemaV1SingleClause returned nil")
	}

	// verify clauses property has minItems and maxItems
	props := schema["properties"].(map[string]interface{})
	programProp := props["program"].(map[string]interface{})
	progProps := programProp["properties"].(map[string]interface{})
	clausesProp := progProps["clauses"].(map[string]interface{})

	minItems, ok := clausesProp["minItems"].(int)
	if !ok || minItems != 1 {
		t.Errorf("Expected minItems=1, got %v", clausesProp["minItems"])
	}

	maxItems, ok := clausesProp["maxItems"].(int)
	if !ok || maxItems != 1 {
		t.Errorf("Expected maxItems=1, got %v", clausesProp["maxItems"])
	}
}

func TestMarshalSchema(t *testing.T) {
	// marshalSchema handles nil correctly
	str := marshalSchema(nil)
	if str != "" {
		t.Errorf("marshalSchema(nil) expected empty string, got %q", str)
	}

	// A map that fails to marshal (e.g. contains functions)
	badMap := map[string]interface{}{
		"fn": func() {},
	}
	str2 := marshalSchema(badMap)
	if str2 != "" {
		t.Errorf("marshalSchema(badMap) expected empty string, got %q", str2)
	}
}
