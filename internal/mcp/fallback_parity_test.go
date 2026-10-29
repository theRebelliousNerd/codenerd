package mcp

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// The Go selection path is the policy's fallback mirror: it runs when the
// kernel query fails, and it must rank a tool the way policy_mcp.mg would.
// Selection is a kernel decision, so the policy is the source; this test reads
// it and fails when the two drift.
func TestFallbackSelection_ShouldMirrorPolicyWeightsAndTiers(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(mcpSchemaDir, mcpPolicyRel))
	if err != nil {
		t.Fatalf("read policy: %v", err)
	}
	policy := string(data)

	weight := func(variable string) int {
		t.Helper()
		m := regexp.MustCompile(`fn:div\(fn:mult\(` + variable + `, (\d+)\), 10\)`).FindStringSubmatch(policy)
		if m == nil {
			t.Fatalf("policy_mcp.mg has no %s weight in tenths; the relevance rule changed shape", variable)
		}
		n, _ := strconv.Atoi(m[1])
		return n
	}
	if got := weight("LogicScore"); got != policyLogicWeightTenths {
		t.Errorf("policy logic weight = %d/10, fallback uses %d/10", got, policyLogicWeightTenths)
	}
	if got := weight("VectorScore"); got != policyVectorWeightTenths {
		t.Errorf("policy vector weight = %d/10, fallback uses %d/10", got, policyVectorWeightTenths)
	}

	tier := func(mode string) int {
		t.Helper()
		m := regexp.MustCompile(`mcp_tool_selected\(ShardType, ToolID, /` + mode + `\) :-\s*\n\s*mcp_tool_relevance\(ShardType, ToolID, Score\),\s*\n\s*Score >= (\d+)`).FindStringSubmatch(policy)
		if m == nil {
			t.Fatalf("policy_mcp.mg has no scored /%s tier; the selection rule changed shape", mode)
		}
		n, _ := strconv.Atoi(m[1])
		return n
	}
	cfg := DefaultToolSelectionConfig()
	for mode, fallback := range map[string]int{
		"full":      cfg.FullThreshold,
		"condensed": cfg.CondensedThreshold,
		"minimal":   cfg.MinimalThreshold,
	} {
		if got := tier(mode); got != fallback {
			t.Errorf("policy /%s tier starts at %d, fallback at %d", mode, got, fallback)
		}
	}
}

// The two relevance rules, in the policy's arithmetic. With no vector score a
// tool's relevance is its logic score; the fallback used to weight it by 7/10
// anyway, which put an affinity-50 tool in /minimal where the policy has it in
// /condensed. With a vector score each weighted term is floored before the sum.
func TestFallbackSelection_ShouldScoreLikeThePolicyRelevanceRules(t *testing.T) {
	c := NewJITToolCompiler(nil, nil, nil) // nil kernel: the fallback path

	noVector := c.selectTools(context.Background(), ToolCompilationContext{ShardType: "coder"},
		[]*MCPTool{{ToolID: "mid", ShardAffinities: map[string]int{"coder": 50}}}, nil, nil)
	if len(noVector) != 1 || noVector[0].FinalScore != 50 || noVector[0].RenderMode != RenderModeCondensed {
		t.Fatalf("affinity 50, no vector score: got %+v, want final 50 at /condensed (the policy's logic-only rule)", noVector)
	}

	// 19*7/10 + 23*3/10 = 13 + 6 = 19 in the policy: below the minimal tier.
	// Flooring the sum instead, (133 + 69) / 10 = 20, selected it.
	withVector := c.selectTools(context.Background(), ToolCompilationContext{ShardType: "coder"},
		[]*MCPTool{{ToolID: "v", ShardAffinities: map[string]int{"coder": 19}}}, map[string]float64{"v": 0.235}, nil)
	if len(withVector) != 0 {
		t.Fatalf("combined relevance 19 is below the minimal tier; got %+v", withVector)
	}
	edge := c.selectTools(context.Background(), ToolCompilationContext{ShardType: "coder"},
		[]*MCPTool{{ToolID: "e", ShardAffinities: map[string]int{"coder": 25}}}, map[string]float64{"e": 0.10}, nil)
	// 25*7/10 + 10*3/10 = 17 + 3 = 20: exactly the minimal tier.
	if len(edge) != 1 || edge[0].FinalScore != 20 {
		t.Fatalf("got %+v, want final 20 (17 + 3)", edge)
	}
}
