package autopoiesis

import (
	"testing"
)

// --- toCamelCase ---

func TestToCamelCase_AllCases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello_world", "helloWorld"},
		{"simple", "simple"},
		{"a_b_c", "aBC"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toCamelCase(tt.input)
			if got != tt.want {
				t.Errorf("toCamelCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- toPascalCase ---

func TestToPascalCase_AllCases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello_world", "HelloWorld"},
		{"simple", "Simple"},
		{"a_b_c", "ABC"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toPascalCase(tt.input)
			if got != tt.want {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- extractCodeBlock ---

func TestExtractCodeBlock_WhenGoBlock_ShouldExtract(t *testing.T) {
	input := "Some text\n```go\nfunc main() {}\n```\nMore text"
	got := extractCodeBlock(input, "go")
	if got != "func main() {}" {
		t.Errorf("extractCodeBlock() = %q, want 'func main() {}'", got)
	}
}

func TestExtractCodeBlock_WhenNoBlock_ShouldReturnTrimmedInput(t *testing.T) {
	input := "  just raw code  "
	got := extractCodeBlock(input, "go")
	if got != "just raw code" {
		t.Errorf("extractCodeBlock() = %q, want 'just raw code'", got)
	}
}

// --- extractDescription ---

func TestExtractDescription_WhenConstant_ShouldExtract(t *testing.T) {
	code := `package tools
const MyToolDescription = "does something cool"
func MyTool() {}`
	got := extractDescription(code)
	if got != "does something cool" {
		t.Errorf("extractDescription() = %q", got)
	}
}

func TestExtractDescription_WhenComment_ShouldExtract(t *testing.T) {
	code := "// This is the package comment\npackage tools"
	got := extractDescription(code)
	if got != "This is the package comment" {
		t.Errorf("extractDescription() = %q", got)
	}
}

func TestExtractDescription_WhenNone_ShouldReturnDefault(t *testing.T) {
	code := "package tools\nfunc main() {}"
	got := extractDescription(code)
	if got != "No description available" {
		t.Errorf("extractDescription() = %q", got)
	}
}

// --- getZeroValue ---

func TestGetZeroValue_AllTypes(t *testing.T) {
	tests := []struct {
		typeName string
		want     string
	}{
		{"string", `""`},
		{"[]byte", "nil"},
		{"int", "0"},
		{"float64", "0.0"},
		{"bool", "false"},
		{"error", "nil"},
		{"[]string", "nil"},
		{"map[string]int", "nil"},
		{"*int", "nil"},
		{"MyStruct", "MyStruct{}"},
	}
	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			got := getZeroValue(tt.typeName)
			if got != tt.want {
				t.Errorf("getZeroValue(%q) = %q, want %q", tt.typeName, got, tt.want)
			}
		})
	}
}

// --- getTestValue ---

func TestGetTestValue_WhenStringValid_ShouldReturnTestString(t *testing.T) {
	got := getTestValue("string", true)
	if got != `"test input"` {
		t.Errorf("getTestValue(string, true) = %q", got)
	}
}

func TestGetTestValue_WhenStringInvalid_ShouldReturnEmpty(t *testing.T) {
	got := getTestValue("string", false)
	if got != `""` {
		t.Errorf("getTestValue(string, false) = %q", got)
	}
}

func TestGetTestValue_WhenInt_ShouldReturn42(t *testing.T) {
	got := getTestValue("int", true)
	if got != "42" {
		t.Errorf("getTestValue(int, true) = %q", got)
	}
}

// --- extractJSONFromTemplate ---

func TestExtractJSONFromTemplate_WhenWrapped_ShouldExtract(t *testing.T) {
	got := extractJSONFromTemplate(`prefix {"key": "val"} suffix`)
	if got != `{"key": "val"}` {
		t.Errorf("extractJSONFromTemplate() = %q", got)
	}
}

func TestExtractJSONFromTemplate_WhenNoJSON_ShouldReturnEmpty(t *testing.T) {
	got := extractJSONFromTemplate("no json")
	if got != "{}" {
		t.Errorf("extractJSONFromTemplate() = %q, want '{}'", got)
	}
}

// --- extractFunctionSignatures ---

func TestExtractFunctionSignatures_WhenValidGo_ShouldExtract(t *testing.T) {
	code := `package main

func HelloWorld() {}
func internalFunc() {}
`
	sigs := extractFunctionSignatures(code)
	if len(sigs) != 2 {
		t.Fatalf("expected 2 sigs, got %d", len(sigs))
	}
	if sigs[0].Name != "HelloWorld" {
		t.Errorf("expected HelloWorld, got %s", sigs[0].Name)
	}
	if !sigs[0].IsExported {
		t.Error("HelloWorld should be exported")
	}
	if sigs[1].IsExported {
		t.Error("internalFunc should not be exported")
	}
}

func TestExtractFunctionSignatures_WhenInvalidCode_ShouldReturnEmpty(t *testing.T) {
	sigs := extractFunctionSignatures("not valid go code !@#$")
	if len(sigs) != 0 {
		t.Errorf("expected 0 sigs for invalid code, got %d", len(sigs))
	}
}

// --- containsIssueType ---

func TestContainsIssueType_WhenPresent_ShouldReturnTrue(t *testing.T) {
	types := []IssueType{IssueIncomplete, IssueSlow}
	if !containsIssueType(types, IssueSlow) {
		t.Error("expected true for existing issue type")
	}
}

func TestContainsIssueType_WhenAbsent_ShouldReturnFalse(t *testing.T) {
	types := []IssueType{IssueIncomplete}
	if containsIssueType(types, IssueSlow) {
		t.Error("expected false for missing issue type")
	}
}

// --- contains ---

func TestContains_WhenPresent_ShouldReturnTrue(t *testing.T) {
	if !contains([]string{"a", "b"}, "a") {
		t.Error("expected true")
	}
}

func TestContains_WhenAbsent_ShouldReturnFalse(t *testing.T) {
	if contains([]string{"a", "b"}, "c") {
		t.Error("expected false")
	}
}

// --- getComplexTestValue ---

func TestGetComplexTestValue_WhenSlice_ShouldReturnSlice(t *testing.T) {
	got := getComplexTestValue("[]MyType", true)
	if got == "" {
		t.Error("expected non-empty result for slice type")
	}
}

func TestGetComplexTestValue_WhenPointer_ShouldReturnPointer(t *testing.T) {
	got := getComplexTestValue("*string", true)
	if got == "" {
		t.Error("expected non-empty result for pointer type")
	}
}

func TestGetComplexTestValue_WhenChannel_ShouldReturnChannel(t *testing.T) {
	got := getComplexTestValue("chan int", true)
	if got == "" {
		t.Error("expected non-empty result for channel type")
	}
}

func TestGetComplexTestValue_WhenFunc_ShouldReturnFunc(t *testing.T) {
	got := getComplexTestValue("func()", true)
	if got == "" {
		t.Error("expected non-empty result for func type")
	}
}
