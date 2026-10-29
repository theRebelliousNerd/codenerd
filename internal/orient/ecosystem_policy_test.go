package orient

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

func evalPolicy(t *testing.T, facts []types.Fact) *Outcome {
	t.Helper()
	out, err := Run(context.Background(), facts)
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	return out
}

func factText(facts []types.Fact) string {
	var b strings.Builder
	for _, fact := range facts {
		parts := make([]string, len(fact.Args))
		for i, arg := range fact.Args {
			parts[i] = types.ExtractString(arg)
		}
		b.WriteString(fact.Predicate)
		b.WriteString("(")
		b.WriteString(strings.Join(parts, ", "))
		b.WriteString(")\n")
	}
	if b.Len() == 0 {
		return "(none)\n"
	}
	return b.String()
}

func requireRow(t *testing.T, facts []types.Fact, args ...string) {
	t.Helper()
	for _, fact := range facts {
		if len(fact.Args) != len(args) {
			continue
		}
		match := true
		for i := range args {
			if types.ExtractString(fact.Args[i]) != args[i] {
				match = false
				break
			}
		}
		if match {
			return
		}
	}
	t.Fatalf("missing (%s) in\n%s", strings.Join(args, ", "), factText(facts))
}

func forbidArg0(t *testing.T, facts []types.Fact, name string) {
	t.Helper()
	for _, fact := range facts {
		if len(fact.Args) > 0 && types.ExtractString(fact.Args[0]) == name {
			t.Fatalf("unexpected %s in\n%s", name, factText(facts))
		}
	}
}

func namesOf(facts []types.Fact) map[string]bool {
	out := map[string]bool{}
	for _, fact := range facts {
		if len(fact.Args) > 0 {
			out[types.ExtractString(fact.Args[0])] = true
		}
	}
	return out
}

func atom(name string) types.MangleAtom { return types.MangleAtom(name) }

func sourceFacts(id, tool, kind, name, path, tracked, digest, norm string, bytes int64, ord int64, topics ...string) []types.Fact {
	facts := []types.Fact{
		{Predicate: "agent_source", Args: []any{id, atom("/" + tool), atom("/" + kind), name, path, atom("/" + tracked)}},
		{Predicate: "agent_source_digest", Args: []any{id, digest}},
		{Predicate: "agent_source_norm", Args: []any{id, norm}},
		{Predicate: "agent_source_bytes", Args: []any{id, bytes}},
		{Predicate: "agent_source_ord", Args: []any{id, ord}},
	}
	for _, topic := range topics {
		facts = append(facts, types.Fact{Predicate: "agent_source_topic", Args: []any{id, topic}})
		facts = append(facts, types.Fact{Predicate: "agent_source_tag", Args: []any{id, topic}})
	}
	return facts
}

func history(path string, last int64) types.Fact {
	return types.Fact{Predicate: "repo_file_history", Args: []any{path, int64(1), last, int64(2), int64(1)}}
}

func overlapMin(n int64) types.Fact {
	return types.Fact{Predicate: "config_param", Args: []any{atom("/orient_topic_overlap_min"), n}}
}

func clusterMin(n int64) types.Fact {
	return types.Fact{Predicate: "config_param", Args: []any{atom("/orient_skill_cluster_min"), n}}
}

func signal(kind, key string) types.Fact {
	return types.Fact{Predicate: "profile_signal", Args: []any{atom(kind), key}}
}

func TestPolicy_NewerCopyWinsOnRecency(t *testing.T) {
	facts := sourceFacts("a-old", "claude", "skill", "Lint", "a.md", "yes", "d1", "lint", 10, 0)
	facts = append(facts, sourceFacts("b-new", "codex", "skill", "lint", "b.md", "yes", "d2", "lint", 10, 1)...)
	// The newer file has the worse id rank, so only recency can select it.
	facts = append(facts, history("a.md", 100), history("b.md", 200))
	out := evalPolicy(t, facts)
	requireRow(t, out.AgentSourceWinner, "b-new", "/more_recent")
	requireRow(t, out.AgentSourceLoser, "a-old", "b-new", "/more_recent")
	forbidArg0(t, out.AgentSourceWinner, "a-old")
}

func TestPolicy_TrackedBeatsUntrackedWhenHistoryMissing(t *testing.T) {
	facts := sourceFacts("local", "claude", "skill", "Lint", "local.md", "no", "d1", "lint", 10, 0)
	facts = append(facts, sourceFacts("tracked", "codex", "skill", "Lint", "tracked.md", "yes", "d2", "lint", 10, 1)...)
	out := evalPolicy(t, facts)
	requireRow(t, out.AgentSourceWinner, "tracked", "/tracked")
	requireRow(t, out.AgentSourceLoser, "local", "tracked", "/tracked")
}

func TestPolicy_TrackedBeatsWhenTimestampsEqual(t *testing.T) {
	facts := sourceFacts("local", "claude", "skill", "Lint", "local.md", "no", "d1", "lint", 40, 0)
	facts = append(facts, sourceFacts("tracked", "codex", "skill", "Lint", "tracked.md", "yes", "d2", "lint", 10, 1)...)
	facts = append(facts, history("local.md", 50), history("tracked.md", 50))
	out := evalPolicy(t, facts)
	requireRow(t, out.AgentSourceWinner, "tracked", "/tracked")
}

func TestPolicy_LargerBodyWinsWhenTrackedAndTimeTie(t *testing.T) {
	facts := sourceFacts("small", "claude", "skill", "Lint", "small.md", "yes", "d1", "lint", 10, 0)
	facts = append(facts, sourceFacts("large", "codex", "skill", "Lint", "large.md", "yes", "d2", "lint", 80, 1)...)
	facts = append(facts, history("small.md", 50), history("large.md", 50))
	out := evalPolicy(t, facts)
	requireRow(t, out.AgentSourceWinner, "large", "/larger_body")
	requireRow(t, out.AgentSourceLoser, "small", "large", "/larger_body")
}

func TestPolicy_SameDigestIsNotADuplicate(t *testing.T) {
	facts := sourceFacts("a", "claude", "skill", "Lint", "a.md", "yes", "same", "lint", 10, 0)
	facts = append(facts, sourceFacts("b", "codex", "skill", "Lint", "b.md", "no", "same", "lint", 10, 1)...)
	out := evalPolicy(t, facts)
	if len(out.AgentSourceDuplicate) != 0 {
		t.Fatalf("duplicates:\n%s", factText(out.AgentSourceDuplicate))
	}
	requireRow(t, out.AgentSourceWinner, "a", "/unique")
	requireRow(t, out.AgentSourceWinner, "b", "/unique")
}

func TestPolicy_SameToolIsNotADuplicate(t *testing.T) {
	facts := sourceFacts("a", "claude", "skill", "Lint", "a.md", "yes", "d1", "lint", 10, 0)
	facts = append(facts, sourceFacts("b", "claude", "skill", "Lint", "b.md", "yes", "d2", "lint", 99, 1)...)
	out := evalPolicy(t, facts)
	if len(out.AgentSourceDuplicate) != 0 {
		t.Fatalf("duplicates:\n%s", factText(out.AgentSourceDuplicate))
	}
	requireRow(t, out.AgentSourceWinner, "a", "/unique")
	requireRow(t, out.AgentSourceWinner, "b", "/unique")
}

func TestPolicy_SharedTopicsAcrossToolsAreDuplicates(t *testing.T) {
	facts := sourceFacts("a", "claude", "skill", "Alpha", "a.md", "yes", "d1", "alpha", 10, 0, "browser", "selectors")
	facts = append(facts, sourceFacts("b", "codex", "rule", "Beta", "b.md", "yes", "d2", "beta", 10, 1, "browser", "selectors")...)
	facts = append(facts, overlapMin(2))
	out := evalPolicy(t, facts)
	requireRow(t, out.AgentSourceDuplicate, "a", "b")
	requireRow(t, out.AgentSourceWinner, "a", "/stable_id")
	requireRow(t, out.AgentSourceLoser, "b", "a", "/stable_id")
}

func TestPolicy_OneSharedTopicIsNotEnough(t *testing.T) {
	facts := sourceFacts("a", "claude", "skill", "Alpha", "a.md", "yes", "d1", "alpha", 10, 0, "browser")
	facts = append(facts, sourceFacts("b", "codex", "skill", "Beta", "b.md", "yes", "d2", "beta", 10, 1, "browser")...)
	facts = append(facts, overlapMin(2))
	out := evalPolicy(t, facts)
	if len(out.AgentSourceDuplicate) != 0 {
		t.Fatalf("one shared topic duplicated:\n%s", factText(out.AgentSourceDuplicate))
	}
}

func TestPolicy_SkillClusterFromWinningSkills(t *testing.T) {
	facts := sourceFacts("a", "claude", "skill", "Alpha", "a.md", "yes", "d1", "alpha", 10, 0, "browser")
	facts = append(facts, sourceFacts("b", "claude", "skill", "Beta", "b.md", "yes", "d2", "beta", 10, 1, "browser")...)
	facts = append(facts, clusterMin(2))
	out := evalPolicy(t, facts)
	requireRow(t, out.OrientAgent, "browser", "skill cluster")
	requireRow(t, out.AgentSourceWinner, "a", "/unique")
	requireRow(t, out.AgentSourceWinner, "b", "/unique")
}

func TestPolicy_LosingDuplicateDoesNotFormACluster(t *testing.T) {
	facts := sourceFacts("winner", "claude", "skill", "Lint", "w.md", "yes", "d1", "lint", 50, 0, "browser")
	facts = append(facts, sourceFacts("loser", "codex", "skill", "Lint", "l.md", "yes", "d2", "lint", 10, 1, "browser")...)
	facts = append(facts, clusterMin(2), overlapMin(2))
	out := evalPolicy(t, facts)
	requireRow(t, out.AgentSourceWinner, "winner", "/larger_body")
	forbidArg0(t, out.OrientAgent, "browser")
}

func TestPolicy_TransitiveComponentHasOneWinner(t *testing.T) {
	// a-b share a name. b-c share two topics and do not share a name.
	// a-c share neither, so the component exists only through b.
	// Topic counts match so specificity cannot decide; only the id rank can,
	// and a reaches c only through b.
	facts := sourceFacts("a", "claude", "skill", "Lint", "a.md", "yes", "d1", "lint", 10, 0, "solo-a", "pad")
	facts = append(facts, sourceFacts("b", "codex", "skill", "Lint", "b.md", "yes", "d2", "lint", 10, 1, "left", "right")...)
	facts = append(facts, sourceFacts("c", "grok", "skill", "Other", "c.md", "yes", "d3", "other", 10, 2, "left", "right")...)
	facts = append(facts, overlapMin(2))
	out := evalPolicy(t, facts)
	requireRow(t, out.AgentSourceWinner, "a", "/stable_id")
	requireRow(t, out.AgentSourceLoser, "b", "a", "/stable_id")
	requireRow(t, out.AgentSourceLoser, "c", "a", "/stable_id")
	if got := namesOf(out.AgentSourceWinner); got["b"] || got["c"] {
		t.Fatalf("winners:\n%s", factText(out.AgentSourceWinner))
	}
}

func TestPolicy_MoreSpecificAndMoreReferenced(t *testing.T) {
	facts := sourceFacts("plain", "claude", "skill", "Lint", "p.md", "yes", "d1", "lint", 10, 0)
	facts = append(facts, sourceFacts("tagged", "codex", "skill", "Lint", "t.md", "yes", "d2", "lint", 10, 1, "browser")...)
	out := evalPolicy(t, facts)
	requireRow(t, out.AgentSourceWinner, "tagged", "/more_specific")

	facts = sourceFacts("cited", "claude", "skill", "Lint", "c.md", "yes", "d1", "lint", 10, 0, "browser")
	facts = append(facts, sourceFacts("quiet", "codex", "skill", "Lint", "q.md", "yes", "d2", "lint", 10, 1, "browser")...)
	facts = append(facts, types.Fact{Predicate: "agent_source_refers", Args: []any{"other", "cited"}})
	out = evalPolicy(t, facts)
	requireRow(t, out.AgentSourceWinner, "cited", "/more_referenced")
}

func TestPolicy_CatalogMatchesTheOldSwitch(t *testing.T) {
	cases := []struct {
		name    string
		facts   []types.Fact
		want    []string
		missing []string
	}{
		{name: "go", facts: []types.Fact{signal("/language", "go")}, want: []string{"GoExpert"}, missing: []string{"SecurityAuditor", "TestArchitect"}},
		{name: "golang", facts: []types.Fact{signal("/language", "golang")}, want: []string{"GoExpert"}},
		{name: "python", facts: []types.Fact{signal("/language", "python")}, want: []string{"PythonExpert"}},
		{name: "typescript", facts: []types.Fact{signal("/language", "typescript")}, want: []string{"TSExpert"}},
		{name: "javascript", facts: []types.Fact{signal("/language", "javascript")}, want: []string{"TSExpert"}},
		{name: "kotlin", facts: []types.Fact{signal("/language", "kotlin")}, want: []string{"AndroidExpert"}},
		{name: "haskell", facts: []types.Fact{signal("/language", "haskell")}, missing: []string{"GoExpert", "SecurityAuditor", "TestArchitect"}},
		{name: "haskell with tests", facts: []types.Fact{signal("/language", "haskell"), signal("/tests", "present")}, want: []string{"TestArchitect"}, missing: []string{"SecurityAuditor"}},
		{name: "empty"},
		{name: "go with tests keeps the stack expert and the test architect", facts: []types.Fact{signal("/language", "go"), signal("/tests", "present")}, want: []string{"GoExpert", "TestArchitect"}},
		{name: "go+gin", facts: []types.Fact{signal("/language", "go"), signal("/framework", "gin")}, want: []string{"GoExpert", "WebAPIExpert"}},
		{name: "go+rod", facts: []types.Fact{signal("/language", "go"), signal("/dependency", "rod")}, want: []string{"GoExpert", "RodExpert"}},
		{name: "chromedp is a security surface", facts: []types.Fact{signal("/dependency", "chromedp")}, want: []string{"BrowserAutomationExpert", "SecurityAuditor"}, missing: []string{"RodExpert", "TestArchitect"}},
		{name: "react", facts: []types.Fact{signal("/language", "typescript"), signal("/framework", "react")}, want: []string{"TSExpert", "FrontendExpert"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := evalPolicy(t, tc.facts)
			got := namesOf(out.OrientAgent)
			for _, name := range tc.want {
				if !got[name] {
					t.Fatalf("missing %s in\n%s", name, factText(out.OrientAgent))
				}
			}
			for _, name := range tc.missing {
				if got[name] {
					t.Fatalf("unexpected %s in\n%s", name, factText(out.OrientAgent))
				}
			}
			if tc.name == "go" {
				requireRow(t, out.OrientAgentDescription, "GoExpert", "Expert in Go idioms, concurrency patterns, and standard library")
				requireRow(t, out.OrientAgentTopic, "GoExpert", "go concurrency")
				requireRow(t, out.OrientAgentPermission, "GoExpert", "read_file")
				requireRow(t, out.OrientAgentPriority, "GoExpert", "100")
				requireRow(t, out.OrientResearchTopic, "GoExpert", "go testing")
			}
			if tc.name == "empty" && len(got) != 0 {
				t.Fatalf("nothing measured, nothing supported, yet agents were derived:\n%s", factText(out.OrientAgent))
			}
		})
	}
}

func TestPolicy_ImportedSubagentAndCoveredResearch(t *testing.T) {
	facts := sourceFacts("rev", "claude", "subagent", "reviewer", ".claude/agents/reviewer.md", "yes", "d1", "reviewer", 20, 0, "reviewer", "Reviews changes")
	facts = append(facts, types.Fact{Predicate: "agent_source_description", Args: []any{"rev", "Reviews changes"}})
	facts = append(facts, types.Fact{Predicate: "agent_source_declared_tool", Args: []any{"rev", "read_file"}})
	facts = append(facts, sourceFacts("mem", "claude", "memory", "reviewer", "memory/reviewer/note.md", "no", "d2", "reviewer", 5, 1, "reviewer")...)
	out := evalPolicy(t, facts)
	requireRow(t, out.OrientAgent, "reviewer", "imported subagent")
	requireRow(t, out.OrientAgentKnowledge, "reviewer", "rev")
	requireRow(t, out.OrientAgentKnowledge, "reviewer", "mem")
	requireRow(t, out.OrientAgentPermission, "reviewer", "read_file")
	requireRow(t, out.OrientAgentPriority, "reviewer", "90")
	for _, fact := range out.OrientResearchTopic {
		if types.ExtractString(fact.Args[0]) == "reviewer" {
			t.Fatalf("covered topic still researched:\n%s", factText(out.OrientResearchTopic))
		}
	}
}

func TestPolicy_TopicOverlapWaitsForItsThreshold(t *testing.T) {
	facts := sourceFacts("a", "claude", "skill", "Alpha", "a.md", "yes", "d1", "alpha", 10, 0, "left", "right")
	facts = append(facts, sourceFacts("b", "codex", "skill", "Beta", "b.md", "yes", "d2", "beta", 10, 1, "left", "right")...)
	out := evalPolicy(t, append(append([]types.Fact{}, facts...), overlapMin(3)))
	if len(out.AgentSourceDuplicate) != 0 {
		t.Fatalf("overlap below the configured threshold duplicated:\n%s", factText(out.AgentSourceDuplicate))
	}
	// Name duplicates do not need the threshold.
	named := sourceFacts("c", "claude", "skill", "Lint", "c.md", "yes", "d3", "lint", 10, 2)
	named = append(named, sourceFacts("d", "grok", "skill", "lint", "d.md", "yes", "d4", "lint", 10, 3)...)
	out = evalPolicy(t, named)
	requireRow(t, out.AgentSourceDuplicate, "c", "d")

	facts = append(facts, overlapMin(2))
	out = evalPolicy(t, facts)
	requireRow(t, out.AgentSourceDuplicate, "a", "b")
}

func TestNewEngineLoadsEcosystemRules(t *testing.T) {
	cfg := config.DefaultOrientConfig()
	eng, err := NewEngine(&cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()
	if err := eng.Assert([]types.Fact{signal("/language", "go")}); err != nil {
		t.Fatalf("assert: %v", err)
	}
	if err := eng.Evaluate(context.Background()); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	rows, err := eng.Query("orient_agent")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	requireRow(t, rows, "GoExpert", "Go project detected - expert knowledge improves code quality")
	forbidArg0(t, rows, "SecurityAuditor")
}

func visionFacts(name, why, description string, topics, questions []string) []types.Fact {
	facts := []types.Fact{
		{Predicate: "vision_agent", Args: []any{name, why}},
		{Predicate: "vision_agent_description", Args: []any{name, description}},
	}
	for _, topic := range topics {
		facts = append(facts, types.Fact{Predicate: "vision_agent_topic", Args: []any{name, topic}})
	}
	for _, question := range questions {
		facts = append(facts, types.Fact{Predicate: "vision_agent_question", Args: []any{name, question}})
	}
	return facts
}

// Vision experts join the roster through the same predicates as stack
// experts; their topics are researched from their own questions, not by a
// documentation fetch; an expert named like a stack expert merges with it.
func TestPolicy_VisionExpertsJoinTheRoster(t *testing.T) {
	facts := []types.Fact{signal("/language", "go")}
	facts = append(facts, visionFacts("AgentFailureModesExpert", "north star: the product is a coding agent",
		"Expert in how coding agents fail", []string{"coding agent failure modes"}, []string{"How do coding agents fail to wire new code?"})...)
	facts = append(facts, visionFacts("GoExpert", "north star: the agent is written in Go",
		"Go for agent runtimes", []string{"goroutine leaks in long-running agents"}, nil)...)
	out := evalPolicy(t, facts)

	requireRow(t, out.OrientAgent, "AgentFailureModesExpert", "north star: the product is a coding agent")
	requireRow(t, out.OrientAgentTopic, "AgentFailureModesExpert", "coding agent failure modes")
	requireRow(t, out.OrientAgentDescription, "AgentFailureModesExpert", "Expert in how coding agents fail")
	requireRow(t, out.OrientAgentPriority, "AgentFailureModesExpert", "95")
	requireRow(t, out.OrientAgentPermission, "AgentFailureModesExpert", "read_file")
	requireRow(t, out.OrientAgentTopic, "GoExpert", "go concurrency")
	requireRow(t, out.OrientAgentTopic, "GoExpert", "goroutine leaks in long-running agents")
	for _, fact := range out.OrientResearchTopic {
		if topic := types.ExtractString(fact.Args[1]); topic == "coding agent failure modes" || topic == "goroutine leaks in long-running agents" {
			t.Fatalf("a vision topic went to the documentation fetch:\n%s", factText(out.OrientResearchTopic))
		}
	}
	requireRow(t, out.OrientResearchTopic, "GoExpert", "go concurrency")
}

// The north star keeps a core expert the scan alone would not.
func TestPolicy_VisionSupportsCoreExpert(t *testing.T) {
	facts := []types.Fact{
		signal("/language", "haskell"),
		{Predicate: "vision_core_support", Args: []any{"SecurityAuditor", "north star: agents execute tools on the user's machine"}},
	}
	out := evalPolicy(t, facts)
	got := namesOf(out.OrientAgent)
	if !got["SecurityAuditor"] || got["TestArchitect"] {
		t.Fatalf("roster:\n%s", factText(out.OrientAgent))
	}
	requireRow(t, out.OrientAgentTopic, "SecurityAuditor", "OWASP top 10")
}

// Imported subagents are bounded and ranked by recency, then source order,
// so a large corpus cannot crowd the roster.
func TestPolicy_ImportedAgentsAreBounded(t *testing.T) {
	var facts []types.Fact
	for index, name := range []string{"alpha", "bravo", "charlie", "delta"} {
		path := ".claude/agents/" + name + ".md"
		facts = append(facts, sourceFacts(name, "claude", "subagent", name, path, "yes", "d-"+name, name, 10, int64(index))...)
	}
	facts = append(facts, history(".claude/agents/delta.md", 500), history(".claude/agents/alpha.md", 100))
	facts = append(facts, types.Fact{Predicate: "config_param", Args: []any{atom("/orient_imported_agent_max"), int64(2)}})
	out := evalPolicy(t, facts)
	got := namesOf(out.OrientAgent)
	if !got["delta"] || !got["alpha"] || got["bravo"] || got["charlie"] || len(got) != 2 {
		t.Fatalf("want the two most recently maintained subagents, got:\n%s", factText(out.OrientAgent))
	}
}

// Skill clusters count against the same import bound.
func TestPolicy_SkillClustersShareTheImportBound(t *testing.T) {
	var facts []types.Fact
	for index, tag := range []string{"alpha", "alpha", "bravo", "bravo"} {
		id := fmt.Sprintf("skill-%d", index)
		facts = append(facts, sourceFacts(id, "claude", "skill", id, ".claude/skills/"+id+"/SKILL.md", "yes", "d-"+id, id, 10, int64(index), tag)...)
	}
	facts = append(facts, clusterMin(2))
	facts = append(facts, types.Fact{Predicate: "config_param", Args: []any{atom("/orient_imported_agent_max"), int64(1)}})
	out := evalPolicy(t, facts)
	got := namesOf(out.OrientAgent)
	if len(got) != 1 || !got["alpha"] {
		t.Fatalf("want one skill-cluster agent under a bound of 1, got:\n%s", factText(out.OrientAgent))
	}
}
