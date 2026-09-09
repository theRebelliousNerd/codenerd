package mcp

import (
	"encoding/json"
	"testing"
)

func boolPtr(v bool) *bool { return &v }

func TestClassifyTool_WhenServerDeclaresReadOnly_ShouldOverrideNameSignal(t *testing.T) {
	t.Parallel()

	// "run_report" reads as an execution surface by name alone, and the server
	// says it has no side effects. The declaration has to win, or a control
	// plane gates a harmless reporting tool behind a confirmation prompt and
	// teaches the operator that the prompts are noise.
	got := ClassifyTool(MCPToolSchema{
		Name:        "run_report",
		Description: "Render a saved report",
		Annotations: MCPToolAnnotations{ReadOnlyHint: boolPtr(true)},
	}, nil)

	if got.Risk != RiskSafe {
		t.Errorf("risk = %q, want %q", got.Risk, RiskSafe)
	}
	if got.RiskSource != SourceAnnotation {
		t.Errorf("risk source = %q, want %q", got.RiskSource, SourceAnnotation)
	}
	if got.Facet != FacetAnalyze {
		t.Errorf("facet = %q, want %q (a read-only 'run' is an analysis)", got.Facet, FacetAnalyze)
	}
}

func TestClassifyTool_WhenServerDeclaresDestructive_ShouldClassifyDestructive(t *testing.T) {
	t.Parallel()

	got := ClassifyTool(MCPToolSchema{
		Name:        "archive_record",
		Description: "Archive a record",
		Annotations: MCPToolAnnotations{DestructiveHint: boolPtr(true)},
	}, nil)

	if got.Risk != RiskDestructive {
		t.Errorf("risk = %q, want %q", got.Risk, RiskDestructive)
	}
	if got.RiskSource != SourceAnnotation {
		t.Errorf("risk source = %q, want %q", got.RiskSource, SourceAnnotation)
	}
}

func TestClassifyTool_WhenNameContradictsCapability_ShouldTrustName(t *testing.T) {
	t.Parallel()

	// The analyzer's capability inference is substring matching over the
	// description, so a read tool whose description says "updated" gets tagged
	// /write. The name is the stronger signal and must win.
	got := ClassifyTool(MCPToolSchema{
		Name:        "get_issue",
		Description: "Fetch an issue, including when it was last updated",
	}, &ToolAnalysis{Capabilities: []string{"/write", "/read"}})

	if got.Facet != FacetRead {
		t.Errorf("facet = %q, want %q", got.Facet, FacetRead)
	}
	if got.Risk != RiskSafe {
		t.Errorf("risk = %q, want %q", got.Risk, RiskSafe)
	}
}

func TestClassifyTool_WhenCamelCaseName_ShouldTokenizeLikeSnakeCase(t *testing.T) {
	t.Parallel()

	camel := ClassifyTool(MCPToolSchema{Name: "listPullRequests"}, nil)
	snake := ClassifyTool(MCPToolSchema{Name: "list_pull_requests"}, nil)

	if camel.Facet != snake.Facet || camel.Risk != snake.Risk {
		t.Errorf("camelCase %v/%v != snake_case %v/%v — naming convention must not change classification",
			camel.Facet, camel.Risk, snake.Facet, snake.Risk)
	}
	if camel.Facet != FacetRead {
		t.Errorf("facet = %q, want %q", camel.Facet, FacetRead)
	}
}

func TestClassifyTool_WhenCodeParameterPresent_ShouldClassifyArbitrary(t *testing.T) {
	t.Parallel()

	// The name says nothing alarming; the parameter does. This is the case a
	// name-only classifier misses, and it is the highest-consequence one.
	got := ClassifyTool(MCPToolSchema{
		Name:        "notebook_cell",
		Description: "Add a cell to the notebook",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"code":{"type":"string"}}}`),
	}, nil)

	if got.Risk != RiskArbitrary {
		t.Errorf("risk = %q, want %q", got.Risk, RiskArbitrary)
	}
	if got.RiskSource != SourceSchema {
		t.Errorf("risk source = %q, want %q", got.RiskSource, SourceSchema)
	}
}

func TestClassifyTool_WhenNothingMatches_ShouldNotDefaultToSafe(t *testing.T) {
	t.Parallel()

	// Default-deny is the house rule. An unclassifiable tool must not arrive
	// classified as harmless: the classifier ran out of evidence, which is not
	// the same as finding none.
	got := ClassifyTool(MCPToolSchema{Name: "xyzzy", Description: "plugh"}, nil)

	if got.Risk == RiskSafe {
		t.Errorf("risk = %q; an unclassifiable tool must not be treated as safe", got.Risk)
	}
	if got.RiskSource != SourceDefault {
		t.Errorf("risk source = %q, want %q", got.RiskSource, SourceDefault)
	}
}

func TestClassifyTool_WhenDeleteVerb_ShouldClassifyDestructiveWrite(t *testing.T) {
	t.Parallel()

	got := ClassifyTool(MCPToolSchema{Name: "delete_branch"}, nil)
	if got.Facet != FacetWrite {
		t.Errorf("facet = %q, want %q", got.Facet, FacetWrite)
	}
	if got.Risk != RiskDestructive {
		t.Errorf("risk = %q, want %q", got.Risk, RiskDestructive)
	}
}

func TestClassifyTool_WhenQueryParameter_ShouldClassifySearch(t *testing.T) {
	t.Parallel()

	got := ClassifyTool(MCPToolSchema{
		Name:        "issues",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`),
	}, nil)

	if got.Facet != FacetSearch {
		t.Errorf("facet = %q, want %q", got.Facet, FacetSearch)
	}
}

func TestNameTokens_ShouldSplitEveryConvention(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"get_file":        {"get", "file"},
		"get-file":        {"get", "file"},
		"getFile":         {"get", "file"},
		"HTTPRequest":     {"http", "request"},
		"repo.search":     {"repo", "search"},
		"github/list_prs": {"github", "list", "prs"},
	}
	for name, want := range cases {
		got := nameTokens(name)
		if len(got) != len(want) {
			t.Errorf("nameTokens(%q) = %v, want %v", name, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("nameTokens(%q) = %v, want %v", name, got, want)
				break
			}
		}
	}
}

func TestCensusFor_ShouldGroupByFacetAndReportMaxRisk(t *testing.T) {
	t.Parallel()

	tools := []*MCPTool{
		{Name: "get_a", Facet: FacetRead, Risk: RiskSafe},
		{Name: "get_b", Facet: FacetRead, Risk: RiskSafe},
		{Name: "rm_c", Facet: FacetWrite, Risk: RiskDestructive},
		{Name: "put_d", Facet: FacetWrite, Risk: RiskMutating},
	}
	census := CensusFor(tools)

	if len(census) != 2 {
		t.Fatalf("census rows = %d, want 2 (empty facets must be omitted)", len(census))
	}
	if census[0].Facet != FacetRead || census[0].Count != 2 || census[0].Risk != RiskSafe {
		t.Errorf("read row = %+v", census[0])
	}
	// The row's risk is the maximum in the bucket: a facet containing one
	// destructive tool is not a safe facet.
	if census[1].Facet != FacetWrite || census[1].Count != 2 || census[1].Risk != RiskDestructive {
		t.Errorf("write row = %+v", census[1])
	}
}

func TestCensusFor_ShouldBoundSampleSize(t *testing.T) {
	t.Parallel()

	var tools []*MCPTool
	for _, n := range []string{"get_a", "get_b", "get_c", "get_d", "get_e", "get_f"} {
		tools = append(tools, &MCPTool{Name: n, Facet: FacetRead, Risk: RiskSafe})
	}
	census := CensusFor(tools)

	if census[0].Count != 6 {
		t.Errorf("count = %d, want 6", census[0].Count)
	}
	// The count grows with the server; the atlas row does not. That is what
	// keeps a 200-tool server costing the same as a 6-tool one.
	if len(census[0].Sample) != atlasSampleSize {
		t.Errorf("sample size = %d, want %d", len(census[0].Sample), atlasSampleSize)
	}
}

func TestToolSchemaHash_WhenAnnotationsChange_ShouldChange(t *testing.T) {
	t.Parallel()

	base := MCPToolSchema{Name: "t", Description: "d", InputSchema: json.RawMessage(`{}`)}
	flipped := base
	flipped.Annotations = MCPToolAnnotations{ReadOnlyHint: boolPtr(true)}

	// Annotations decide the risk class, so a server flipping readOnlyHint has
	// changed what the control plane may do with the tool. If the fingerprint
	// misses it, the old and more permissive classification stays cached.
	if ToolSchemaHash(base) == ToolSchemaHash(flipped) {
		t.Error("schema hash ignored an annotation change; cached classification would go stale")
	}
}
