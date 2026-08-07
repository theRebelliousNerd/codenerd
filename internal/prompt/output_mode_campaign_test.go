package prompt

import (
	"strings"
	"testing"
)

// Every campaign role's LLM output is json.Unmarshal'd by Go, so none of them
// may be told the Piggyback envelope is mandatory. Two mandatory output
// contracts in one prompt is a silent failure, not a style problem: the model
// picks one, the parser expects the other, and the fallback path makes the
// result look like success.
//
// The shard-type strings here must match campaign.GetShardTypeForRole. They are
// literals rather than an import because internal/campaign imports this package.
func TestIsStructuredOutputOnly_CoversEveryCampaignRole(t *testing.T) {
	// role -> shard type, from campaign.GetShardTypeForRole.
	roles := map[string]string{
		"librarian": "librarian",
		"extractor": "extractor",
		"analysis":  "analyzer",
		"assault":   "analyzer",
		"planner":   "planner",
		"taxonomy":  "planner",
		"replanner": "planner",
	}

	for role, shardType := range roles {
		t.Run(role, func(t *testing.T) {
			if !IsStructuredOutputOnly(shardType) {
				t.Errorf("campaign role %q compiles as shard %q, whose output is json.Unmarshal'd, "+
					"but it is not structured-output-only — its prompt will carry "+
					"\"OUTPUT PROTOCOL: PIGGYBACK ENVELOPE (MANDATORY)\" alongside the schema it "+
					"actually needs, and the model will obey the wrong one", role, shardType)
			}
		})
	}
}

// The slash-prefixed form is what CompilationContext.ShardType carries.
func TestIsStructuredOutputOnly_AcceptsSlashPrefixedShardTypes(t *testing.T) {
	for _, shardType := range []string{"/planner", "/librarian", "/extractor", "/analyzer", "/mangle_repair", "/legislator"} {
		if !IsStructuredOutputOnly(shardType) {
			t.Errorf("IsStructuredOutputOnly(%q) = false; the compilation context stores shard types slash-prefixed", shardType)
		}
	}
}

// Conversational shards still need the envelope — it is how they request tools.
// Over-applying this would strip the tool protocol from the coding path.
func TestIsStructuredOutputOnly_LeavesConversationalShardsAlone(t *testing.T) {
	for _, shardType := range []string{"coder", "reviewer", "tester", "researcher", "nemesis", ""} {
		if IsStructuredOutputOnly(shardType) {
			t.Errorf("IsStructuredOutputOnly(%q) = true; that shard needs the Piggyback envelope to request tools", shardType)
		}
	}
}

// filterAtomsForStructuredOutput is the mechanism the flag drives. If it stops
// removing protocol atoms, the flag becomes decorative.
func TestFilterAtomsForStructuredOutput_RemovesBothProtocolFamilies(t *testing.T) {
	atoms := []*PromptAtom{
		{ID: "protocol/piggyback/envelope"},
		{ID: "protocol/reasoning/trace"},
		{ID: "safety/constitutional/default_deny"},
		{ID: "identity/planner/mission"},
	}

	cc := NewCompilationContext()
	cc.ShardType = "/planner"

	kept := filterAtomsForStructuredOutput(atoms, cc)

	var ids []string
	for _, a := range kept {
		ids = append(ids, a.ID)
	}
	joined := strings.Join(ids, ",")

	if strings.Contains(joined, "protocol/piggyback/") {
		t.Error("a piggyback protocol atom survived into a structured-output compile")
	}
	if strings.Contains(joined, "protocol/reasoning/") {
		t.Error("a reasoning protocol atom survived into a structured-output compile")
	}
	if !strings.Contains(joined, "safety/constitutional/default_deny") {
		t.Error("safety atoms must survive; structured output does not mean unconstrained")
	}
	if !strings.Contains(joined, "identity/planner/mission") {
		t.Error("the shard's own identity must survive")
	}
}

func TestFilterAtomsForStructuredOutput_LeavesOtherShardsUntouched(t *testing.T) {
	atoms := []*PromptAtom{
		{ID: "protocol/piggyback/envelope"},
		{ID: "protocol/reasoning/trace"},
	}
	cc := NewCompilationContext()
	cc.ShardType = "/coder"

	if got := filterAtomsForStructuredOutput(atoms, cc); len(got) != len(atoms) {
		t.Errorf("a conversational compile kept %d of %d protocol atoms; it needs all of them", len(got), len(atoms))
	}
}
