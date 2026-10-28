package learnjudge

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/typesafe"
)

func TestDuplicateRequest_ShapeAndLimits(t *testing.T) {
	req := ToolRequest{Name: "json_validator2", Purpose: "Check a JSON file is valid.", Input: "path"}
	state, qs, err := DuplicateRequest(req, []ExistingTool{{Name: "json_validator", Description: "Reports whether JSON is well-formed."}, {Name: "bare"}})
	if err != nil {
		t.Fatal(err)
	}
	r := state["request"].(map[string]string)
	if _, has := r["tool_name"]; has {
		t.Fatal("the proposed name must stay out of the duplicate check")
	}
	if r["purpose"] == "" || r["input"] == "" {
		t.Fatalf("request fields = %v", r)
	}
	choice := qs[QWhichTool].(typesafe.Choice)
	if choice.Criteria["json_validator"] == nil || choice.Criteria["bare"] != nil {
		t.Fatalf("options = %v: a described tool carries its description, a bare one only its name", choice.Criteria)
	}
	if _, ok := qs[QAnyToolExists].(typesafe.Noul); !ok {
		t.Fatal("the companion noul is missing")
	}
	if _, _, err := DuplicateRequest(req, nil); err == nil {
		t.Fatal("no tools must be an error, not an empty choice")
	}
	many := make([]ExistingTool, MaxToolOptions+1)
	for i := range many {
		many[i] = ExistingTool{Name: fmt.Sprintf("t%d", i)}
	}
	if _, _, err := DuplicateRequest(req, many); err == nil {
		t.Fatal("more options than the API takes must be an error")
	}
}

func TestCodeRequest_BoundsLongCodeAndSaysSo(t *testing.T) {
	code := strings.Repeat("x", codeStateLimit+500)
	state, qs := CodeRequest(ToolRequest{Name: "t", Purpose: "p"}, code)
	if len(qs) != 2 {
		t.Fatalf("questions = %d, want 2", len(qs))
	}
	if n, _ := state["code_omitted_bytes"].(int); n <= 0 {
		t.Fatalf("long code must say how much was left out: %v", state["code_omitted_bytes"])
	}
	state, _ = CodeRequest(ToolRequest{Purpose: "p"}, "package main")
	if _, has := state["code_omitted_bytes"]; has || state["code"] != "package main" {
		t.Fatalf("short code goes whole: %v", state)
	}
}
