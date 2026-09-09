package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToolSignature_ShouldPutRequiredArgumentsFirst(t *testing.T) {
	t.Parallel()

	tool := &MCPTool{
		Name: "search_issues",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"required":["query"],
			"properties":{
				"query":{"type":"string"},
				"limit":{"type":"integer"},
				"sort":{"type":"string","enum":["created","updated","relevance"]}
			}
		}`),
	}

	got := ToolSignature(tool)
	// Required first and unadorned, optional in brackets: an agent reading left
	// to right sees what it must supply before what it may.
	if !strings.HasPrefix(got, "search_issues(query: str, [") {
		t.Errorf("signature = %q; want required args first, optional bracketed", got)
	}
	// An enum converts a guess into a choice, which is worth the characters.
	if !strings.Contains(got, "created|updated|relevance") {
		t.Errorf("signature = %q; want the enum inlined", got)
	}
}

func TestToolSignature_ShouldSummariseALongParameterList(t *testing.T) {
	t.Parallel()

	props := map[string]any{}
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"} {
		props[n] = map[string]any{"type": "string"}
	}
	raw, _ := json.Marshal(map[string]any{"type": "object", "properties": props})

	got := ToolSignature(&MCPTool{Name: "wide", InputSchema: raw})
	if !strings.Contains(got, "more)") {
		t.Errorf("signature = %q; a tool with 11 parameters must summarise the tail", got)
	}
}

func TestToolSignature_WhenNoSchema_ShouldStillRender(t *testing.T) {
	t.Parallel()

	if got := ToolSignature(&MCPTool{Name: "ping"}); got != "ping()" {
		t.Errorf("signature = %q, want ping()", got)
	}
	if got := ToolSignature(nil); got != "" {
		t.Errorf("nil tool signature = %q, want empty", got)
	}
}

func TestToolLine_ShouldSurfaceRiskButNotForSafeTools(t *testing.T) {
	t.Parallel()

	// Risk belongs on the browsing line because it changes whether the agent
	// should call the tool at all. Annotating every safe tool would be noise.
	safe := ToolLine(&MCPTool{Name: "get_x", Condensed: "fetch x", Risk: RiskSafe})
	if strings.Contains(safe, "[") {
		t.Errorf("safe tool line = %q; want no risk annotation", safe)
	}
	risky := ToolLine(&MCPTool{Name: "rm_x", Condensed: "remove x", Risk: RiskDestructive})
	if !strings.Contains(risky, "[destructive]") {
		t.Errorf("destructive tool line = %q; want the risk called out", risky)
	}
}

func TestValidateArgs_ShouldCatchMissingRequiredAndWrongType(t *testing.T) {
	t.Parallel()

	tool := &MCPTool{
		ToolID: "srv/create",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"required":["name","count"],
			"properties":{
				"name":{"type":"string"},
				"count":{"type":"integer"},
				"tags":{"type":"array"}
			}
		}`),
	}

	err := ValidateArgs(tool, map[string]any{"count": "seven"})
	if err == nil {
		t.Fatal("invalid arguments were accepted")
	}
	msg := err.Error()
	if !strings.Contains(msg, "missing required \"name\"") {
		t.Errorf("error = %q; want the missing argument named", msg)
	}
	if !strings.Contains(msg, "\"count\" expects int") {
		t.Errorf("error = %q; want the type mismatch named", msg)
	}

	// The schema rides along, because the moment an agent gets an argument
	// wrong is the moment it has proven it needs the schema.
	var argErr *ArgumentError
	if !asArgumentError(err, &argErr) || argErr.Schema == "" {
		t.Error("ArgumentError carried no schema")
	}
}

func TestValidateArgs_ShouldNotBeStricterThanTheServer(t *testing.T) {
	t.Parallel()

	tool := &MCPTool{
		ToolID: "srv/get",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"required":["id"],
			"properties":{"id":{"type":"string"}}
		}`),
	}

	// additionalProperties defaults to true in JSON Schema and servers routinely
	// accept extras they never documented. A client-side gate stricter than the
	// server blocks calls that would have worked.
	if err := ValidateArgs(tool, map[string]any{"id": "x", "undocumented": 1}); err != nil {
		t.Errorf("an undeclared extra argument was rejected: %v", err)
	}
	// An explicit null is the server's business; JSON Schema treats null as its
	// own type and many servers read it as "unset".
	if err := ValidateArgs(tool, map[string]any{"id": nil}); err != nil {
		t.Errorf("an explicit null was rejected: %v", err)
	}
	// A tool with no declared schema constrains nothing.
	if err := ValidateArgs(&MCPTool{ToolID: "srv/free"}, map[string]any{"anything": 1}); err != nil {
		t.Errorf("a schemaless tool rejected arguments: %v", err)
	}
}

func TestFullSchema_ShouldStayBounded(t *testing.T) {
	t.Parallel()

	props := map[string]any{}
	for i := 0; i < 400; i++ {
		props[string(rune('a'+i%26))+strings.Repeat("x", i%40)] = map[string]any{
			"type": "string", "description": strings.Repeat("long ", 40),
		}
	}
	raw, _ := json.Marshal(map[string]any{"type": "object", "properties": props})

	// "Full" is a disclosure tier, not an escape from the budget.
	got := FullSchema(&MCPTool{Name: "huge", InputSchema: raw}, 2000)
	if len(got) > 2000 {
		t.Errorf("full schema rendered %d bytes against a 2000 cap", len(got))
	}
}

// asArgumentError is a local errors.As shim kept in the test so the assertion
// reads plainly.
func asArgumentError(err error, target **ArgumentError) bool {
	if ae, ok := err.(*ArgumentError); ok {
		*target = ae
		return true
	}
	return false
}
