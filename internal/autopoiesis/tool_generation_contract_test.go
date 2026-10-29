package autopoiesis

import "testing"

func TestEnforceToolNeedContract_EmptyTypesCoercedToString(t *testing.T) {
	// This is the buildCLIToolNeed case from cmd/nerd/cmd_advanced.go:814:
	// that constructor sets Name, Purpose, Reasoning, Confidence and Priority
	// but leaves InputType and OutputType as empty strings. The previous
	// detection-site-only coercion left those empty, so the LLM prompt
	// contained "Input Type: " / "Output Type: " and the model invented
	// e.g. CountTheNumberOfMangleDeclStatementsInAInput struct and int return,
	// which tool_compiler.go:isExactEntryPoint rejects (only
	// func(context.Context, string) (string, error) via writeWrapper is supported).
	// The choke-point at GenerateTool must coerce empty to "string".
	need := &ToolNeed{
		Name:       "count_the_number_of_mangle_decl_statements_in_a_mg_file_given_its_path",
		Purpose:    "count the number of Mangle Decl statements in a .mg file given its path",
		InputType:  "",
		OutputType: "",
	}
	if err := enforceToolNeedContract(need); err != nil {
		t.Fatalf("enforceToolNeedContract returned unexpected error: %v", err)
	}
	if need.InputType != toolIOType {
		t.Fatalf("InputType = %q, want %q (toolIOType); empty must be coerced to string at choke point", need.InputType, toolIOType)
	}
	if need.OutputType != toolIOType {
		t.Fatalf("OutputType = %q, want %q (toolIOType); empty must be coerced to string at choke point", need.OutputType, toolIOType)
	}
}

func TestEnforceToolNeedContract_NilGuard(t *testing.T) {
	err := enforceToolNeedContract(nil)
	if err == nil {
		t.Fatal("enforceToolNeedContract(nil) = nil, want error \"tool need is nil\"")
	}
	if err.Error() != "tool need is nil" {
		t.Fatalf("enforceToolNeedContract(nil) error = %q, want %q", err.Error(), "tool need is nil")
	}
}

func TestEnforceToolNeedContract_IdempotentString(t *testing.T) {
	need := &ToolNeed{InputType: "string", OutputType: "string"}
	if err := enforceToolNeedContract(need); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if need.InputType != "string" || need.OutputType != "string" {
		t.Fatalf("string/string not preserved: got %q/%q", need.InputType, need.OutputType)
	}
}
