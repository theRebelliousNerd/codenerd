package autopoiesis

import "testing"

func TestCoerceToolIOType_EnforcesStringContract(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "int coerced", raw: "int", want: "string"},
		{name: "bool coerced", raw: "bool", want: "string"},
		{name: "slice coerced", raw: "[]byte", want: "string"},
		{name: "map coerced", raw: "map[string]any", want: "string"},
		{name: "string stays", raw: "string", want: "string"},
		{name: "empty coerced", raw: "", want: "string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coerceToolIOType(tt.raw, "input_type")
			if got != tt.want {
				t.Fatalf("coerceToolIOType(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestCoerceToolNeedTypes_EnforcesStringContract(t *testing.T) {
	// The bug this prevents: the LLM picks int for a counting tool ("count the
	// number of Mangle Decl statements"), the detector copies it through, and
	// tool_compiler.go:isExactEntryPoint then rejects it because it only
	// accepts func(context.Context, string) (string, error) and writeWrapper
	// can only carry string I/O.
	need := &ToolNeed{
		Name:       "count_decl",
		Purpose:    "count the number of Mangle Decl statements in a .mg file given its path",
		InputType:  "int",
		OutputType: "int",
	}
	coerceToolNeedTypes(need)
	if need.InputType != toolIOType {
		t.Fatalf("InputType = %q, want %q (toolIOType); coercion must force string", need.InputType, toolIOType)
	}
	if need.OutputType != toolIOType {
		t.Fatalf("OutputType = %q, want %q (toolIOType); coercion must force string", need.OutputType, toolIOType)
	}

	// Also verify mixed pair is fully coerced.
	need2 := &ToolNeed{InputType: "string", OutputType: "bool"}
	coerceToolNeedTypes(need2)
	if need2.InputType != "string" || need2.OutputType != "string" {
		t.Fatalf("mixed pair not fully coerced: got InputType=%q OutputType=%q, want both %q", need2.InputType, need2.OutputType, toolIOType)
	}

	// Nil-safe: should not panic.
	coerceToolNeedTypes(nil)

	// Realistic non-string pair from LLM: map input, slice output
	need3 := &ToolNeed{InputType: "map[string]any", OutputType: "[]string"}
	coerceToolNeedTypes(need3)
	if need3.InputType != "string" || need3.OutputType != "string" {
		t.Fatalf("complex types not coerced: got %q / %q, want string/string", need3.InputType, need3.OutputType)
	}
}

func TestToolIOType_IsString(t *testing.T) {
	if toolIOType != "string" {
		t.Fatalf("toolIOType = %q, want %q: the compiler contract is func(context.Context, string) (string, error)", toolIOType, "string")
	}
}
