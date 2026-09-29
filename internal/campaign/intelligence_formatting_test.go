package campaign

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/autopoiesis"
)

// planContextFor drives the planning prompt the decomposer actually builds.
// buildPlanProposalContext is the function that injects the intelligence
// report; a test of formatIntelligenceContext alone would not show that the
// planner receives it.
func planContextFor(t *testing.T, report *IntelligenceReport) string {
	t.Helper()
	d := &Decomposer{lastIntelligence: report}
	return d.buildPlanProposalContext(context.Background(), "camp", DecomposeRequest{
		Goal:         "plan the change",
		CampaignType: CampaignTypeFeature,
	}, "", nil, nil)
}

func TestBuildPlanProposalContext_HolographicBlock(t *testing.T) {
	sectionA := strings.Repeat("alpha-architecture-line\n", 300) + "SECTION_A_TAIL"
	sectionB := "package surface of b.go\nfunc Beta() error { return nil }\nSECTION_B_TAIL"
	if len(sectionA) <= 4096 {
		t.Fatalf("section A is %d bytes, want longer than the deleted 4096 cap", len(sectionA))
	}
	unread := []string{"unread_alpha.go", "unread_beta.go"}
	report := &IntelligenceReport{
		HolographicSections: []HolographicSection{
			{Path: "a.go", Section: sectionA},
			{Path: "b.go", Section: sectionB},
		},
		HolographicUnread: unread,
	}

	got := planContextFor(t, report)
	if !strings.Contains(got, "## Target Architecture") {
		t.Fatalf("planning context has no Target Architecture block:\n%s", got)
	}
	if !strings.Contains(got, sectionA) {
		t.Fatal("planning context cut holographic section A")
	}
	if !strings.Contains(got, sectionB) {
		t.Fatal("planning context cut holographic section B")
	}
	if !strings.Contains(got, "### Not rendered") {
		t.Fatal("planning context did not name withheld targets")
	}
	for _, path := range unread {
		line := holographicUnreadLine(path)
		if !strings.Contains(got, line) {
			t.Errorf("planning context missing unread line for %s:\n%s", path, got)
		}
		if !strings.Contains(line, "package_outline") || !strings.Contains(line, "get_elements") {
			t.Errorf("unread line for %s does not name the tools: %s", path, line)
		}
	}
}

func TestBuildPlanProposalContext_ChurnAndMCPToolsWhole(t *testing.T) {
	const churnN = 12 // the planning formatter used to stop at 10
	const toolN = 17  // the planning formatter used to stop at 15
	report := &IntelligenceReport{}
	var churnNames []string
	for i := 0; i < churnN; i++ {
		name := fmt.Sprintf("hot_%02d.go", i)
		churnNames = append(churnNames, name)
		report.GitChurnHotspots = append(report.GitChurnHotspots, ChurnHotspot{
			Path:      name,
			ChurnRate: 100 - i,
			Reason:    "touched often " + name,
		})
	}
	var toolNames []string
	for i := 0; i < toolN; i++ {
		name := fmt.Sprintf("mcp_tool_%02d", i)
		toolNames = append(toolNames, name)
		report.MCPToolsAvailable = append(report.MCPToolsAvailable, MCPToolInfo{
			Name:        name,
			Description: "does thing " + name,
		})
	}
	longDesc := strings.Repeat("d", 3000) + "MCP_DESC_TAIL"
	report.MCPToolsAvailable = append(report.MCPToolsAvailable, MCPToolInfo{
		Name:        "long_tool",
		Description: longDesc,
	})

	// Safety warnings and tool gaps had no count cap. Pin that they still
	// arrive whole alongside the lists that used to be cut.
	report.SafetyWarnings = []SafetyWarning{
		{Action: "delete", Path: "secret.go", RuleViolated: "no_delete", Severity: "high"},
		{Action: "exec", Path: "run.go", RuleViolated: "no_shell", Severity: "critical"},
	}
	report.ToolGaps = []autopoiesis.ToolNeed{
		{Name: "gap_alpha", Purpose: "parse the manifest", Confidence: 0.8},
		{Name: "gap_beta", Purpose: "diff two schemas", Confidence: 0.4},
	}

	got := planContextFor(t, report)
	for _, name := range churnNames {
		if !strings.Contains(got, name) {
			t.Errorf("churn file %s missing from planning context", name)
		}
		if !strings.Contains(got, "touched often "+name) {
			t.Errorf("churn reason for %s missing", name)
		}
	}
	if strings.Contains(got, "more high-churn") {
		t.Fatalf("churn list is still capped:\n%s", got)
	}
	for _, name := range toolNames {
		if !strings.Contains(got, name) {
			t.Errorf("MCP tool %s missing from planning context", name)
		}
		if !strings.Contains(got, "does thing "+name) {
			t.Errorf("MCP description for %s missing", name)
		}
	}
	if !strings.Contains(got, longDesc) {
		t.Fatal("MCP description was cut")
	}
	if strings.Contains(got, "more MCP tools") {
		t.Fatalf("MCP tool list is still capped:\n%s", got)
	}
	for _, rule := range []string{"no_delete", "secret.go", "no_shell", "run.go"} {
		if !strings.Contains(got, rule) {
			t.Errorf("safety warning field %q missing:\n%s", rule, got)
		}
	}
	for _, gap := range []string{"gap_alpha", "parse the manifest", "gap_beta", "diff two schemas"} {
		if !strings.Contains(got, gap) {
			t.Errorf("tool gap %q missing:\n%s", gap, got)
		}
	}
}

func TestBuildPlanProposalContext_KeepsLiveSections(t *testing.T) {
	goal := "GOAL_MARKER_" + strings.Repeat("g", 80) + "_GOAL_END"
	advisory := strings.Repeat("advice-line ", 800) + "ADVISORY_TAIL"
	if len(advisory) <= 8192 {
		t.Fatalf("advisory fixture is %d bytes, want longer than the deleted 8192 cap", len(advisory))
	}
	report := &IntelligenceReport{
		FileTopology:      map[string]FileInfo{"a.go": {Path: "a.go", Language: "go"}},
		SymbolGraph:       []SymbolInfo{{Name: "F", Kind: "func", File: "a.go"}},
		LanguageBreakdown: map[string]int{"go": 3, "python": 1},
		HistoricalPatterns: []LearningPattern{
			{ShardType: "coder", Description: "PATTERN_MARKER prefer tables", Confidence: 0.9},
		},
		ShardAdvice: []ConsultationResponse{
			{FromSpec: "reviewer", Confidence: 0.8, Advice: "ADVICE_MARKER keep the lock"},
		},
		AdvisorySummary: advisory,
		TestCoverage:    map[string]float64{"low.go": 0.25, "high.go": 0.9},
		UncoveredPaths:  []string{"UNCOVERED_MARKER.go"},
		CodePatterns: []CodePattern{
			{Name: "singleton", Files: []string{"a.go", "b.go"}},
		},
		ArchitectureHints: []string{"HINT_MARKER layered services"},
		PreviousCampaigns: []CampaignArtifact{
			{CampaignID: "prev-1", Goal: goal, SuccessRate: 0.8},
		},
	}

	got := planContextFor(t, report)
	for _, want := range []string{
		"INTELLIGENCE REPORT (from 12 systems):",
		"## Codebase Overview",
		"Files scanned: 1",
		"Symbols indexed: 1",
		"go (3)",
		"python (1)",
		"## LEARNED PATTERNS",
		"[coder] PATTERN_MARKER prefer tables",
		"## EXPERT RECOMMENDATIONS",
		"ADVICE_MARKER keep the lock",
		"ADVISORY_TAIL",
		"## TEST COVERAGE",
		"low.go: 25%",
		"## Test Coverage Gaps",
		"UNCOVERED_MARKER.go",
		"## DETECTED CODE PATTERNS",
		"singleton in a.go, b.go",
		"## Architecture Hints",
		"HINT_MARKER layered services",
		"## RELEVANT PREVIOUS CAMPAIGNS",
		goal,
		"prev-1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("planning context missing %q", want)
		}
	}
	if strings.Contains(got, "high.go") {
		t.Errorf("adequate coverage was listed as low:\n%s", got)
	}
}

func TestBuildPlanProposalContext_NilIntelligenceOmitsTheReport(t *testing.T) {
	if formatIntelligenceContext(nil) != "" {
		t.Fatal("nil report should render nothing")
	}
	got := planContextFor(t, nil)
	if strings.Contains(got, "INTELLIGENCE REPORT") || strings.Contains(got, "Target Architecture") {
		t.Fatalf("nil intelligence was injected:\n%s", got)
	}
}
