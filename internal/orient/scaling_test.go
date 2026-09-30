package orient

import (
	"fmt"
	"testing"
	"time"

	"codenerd/internal/types"
)

// Orientation runs on whole repositories: thousands of documents and agent
// sources. A rule that pairs every row with every other (a join through a
// comparison instead of a shared key) passes every fixture test and then
// does not finish on a large repository — on 2026-09-29 two such rules held
// init in Evaluate for over an hour. This guard evaluates the real policy at
// n and 4n and fails when the time grows like n^2 (16x) instead of n (4x).

// scalingFacts holds n documents born on n distinct days and n agent sources:
// the same skill mirrored once per tool (four copies, different bodies), which
// is the duplicate shape a repository with several agent CLIs really has. The
// group size stays constant as n grows, so correct policy work is linear.
func scalingFacts(n int) []types.Fact {
	facts := []types.Fact{spanFact(0, 86400*int64(4*n+100), int64(4*n), "/no")}
	tools := []string{"/claude", "/codex", "/agents", "/gemini"}
	for i := 0; i < n; i++ {
		dir := fmt.Sprintf("docs/d%d", i%50)
		path := fmt.Sprintf("%s/p%05d.md", dir, i)
		day := int64(10 + i)
		facts = append(facts, docFact(path, dir), histFact(path, day*86400, day*86400, 1, 1),
			F("doc_subtree", path, dir), F("repo_file_day", path, day), tieFact(path, i))
		id := fmt.Sprintf("s%05d", i)
		name := fmt.Sprintf("skill-%05d", i/4)
		facts = append(facts,
			F("agent_source", id, N(tools[i%4]), N("/skill"), name, "."+tools[i%4][1:]+"/"+name+"/SKILL.md", N("/yes")),
			F("agent_source_ord", id, int64(i)),
			F("agent_source_norm", id, name),
			F("agent_source_digest", id, fmt.Sprintf("dig%05d", i)),
			F("agent_source_bytes", id, int64(100+i)),
			F("agent_source_topic", id, name))
	}
	return facts
}

func evalSeconds(t *testing.T, n int) float64 {
	t.Helper()
	start := time.Now()
	evalFacts(t, nil, scalingFacts(n))
	return time.Since(start).Seconds()
}

func TestPolicy_EvaluationScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("scaling guard")
	}
	const n = 300
	evalSeconds(t, n/4) // warm the engine's one-time costs
	small := evalSeconds(t, n)
	large := evalSeconds(t, 4*n)
	ratio := large / small
	t.Logf("n=%d %.2fs, n=%d %.2fs, ratio %.1f", n, small, 4*n, large, ratio)
	if ratio > 10 {
		t.Fatalf("evaluation grew %.1fx for 4x input (linear ~4x, quadratic ~16x): a rule joins two relations through a comparison instead of a shared key", ratio)
	}
}
