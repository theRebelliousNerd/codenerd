package prompt_evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"codenerd/internal/learnjudge"
	"codenerd/internal/mangle"
	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// evidenceSelectionVersion names the deterministic alternating newest/oldest
// selection. Both ends remain reachable as a backlog grows; timestamp ties
// compare task ID and verdict version, never database row or map order.
const evidenceSelectionVersion = "ends-v1"

type evidenceCohort struct {
	id        string
	policy    *mangle.Engine
	state     map[string]any
	questions map[string]typesafe.Question
	json      string
	verdicts  []*JudgeVerdict
	evidence  []EvidenceRef
	omitted   int
}

func selectEvidenceCohort(ctx context.Context, group failureGroup, cfg EvolverConfig) (*evidenceCohort, error) {
	var eligible []*ExecutionRecord
	for _, f := range agentMistakes(group.failures) {
		if f != nil && f.Verdict != nil && f.VerdictVersion != "" {
			eligible = append(eligible, f)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		a, b := eligible[i], eligible[j]
		if !a.Timestamp.Equal(b.Timestamp) {
			return a.Timestamp.After(b.Timestamp)
		}
		if a.TaskID != b.TaskID {
			return a.TaskID < b.TaskID
		}
		return a.VerdictVersion < b.VerdictVersion
	})
	policy, err := newEvolutionPolicy(cfg)
	if err != nil {
		return nil, err
	}
	c := &evidenceCohort{policy: policy}
	groupID := string(cfg.AtomPinScope) + "|" + group.key.String()
	var ranked []*ExecutionRecord
	var rankFacts []mangle.Fact
	lo, hi := 0, len(eligible)-1
	for lo <= hi {
		idx := lo
		if len(ranked)%2 == 0 {
			lo++
		} else {
			idx = hi
			hi--
		}
		f := eligible[idx]
		rankFacts = append(rankFacts, mangle.Fact{Predicate: "evolution_evidence_rank", Args: []any{groupID, f.TaskID, f.VerdictVersion, len(ranked)}})
		ranked = append(ranked, f)
	}
	if err := policy.AddFactsContext(ctx, rankFacts); err != nil {
		return nil, err
	}
	rows, err := policy.Query(ctx, "evolution_cohort_member(Group, Task, Version)")
	if err != nil {
		return nil, err
	}
	members := make(map[EvidenceRef]bool, len(rows.Bindings))
	for _, row := range rows.Bindings {
		members[EvidenceRef{types.ExtractString(row["Task"]), types.ExtractString(row["Version"])}] = true
	}
	var input []learnjudge.GroupFailure
	for _, f := range ranked {
		if !members[EvidenceRef{f.TaskID, f.VerdictVersion}] {
			continue
		}
		// Execution identity is authoritative, even for historical verdicts
		// whose TaskID was absent or inconsistent.
		v := *f.Verdict
		v.TaskID = f.TaskID
		c.verdicts = append(c.verdicts, &v)
		c.evidence = append(c.evidence, EvidenceRef{f.TaskID, f.VerdictVersion})
		input = append(input, learnjudge.GroupFailure{
			TaskID: f.TaskID, VerdictVersion: f.VerdictVersion,
			Request: f.TaskRequest, Explanation: v.Explanation,
			Category: string(v.Category), SuggestedRule: v.ImprovementRule,
		})
	}
	c.omitted = len(eligible) - len(input)
	c.state, c.questions = learnjudge.SharedCauseRequest(input)
	c.state["selection_policy"] = evidenceSelectionVersion
	c.state["observed_on"] = servingPinFor(group.failures).String()
	c.state["group"] = string(cfg.AtomPinScope) + "|" + group.key.String()
	// Hash unselected identities rather than copying an unbounded backlog
	// into either prompt. None of these identities will be consumed.
	selected := make(map[EvidenceRef]bool, len(c.evidence))
	for _, ref := range c.evidence {
		selected[ref] = true
	}
	var omitted []EvidenceRef
	for _, f := range eligible {
		ref := EvidenceRef{f.TaskID, f.VerdictVersion}
		if !selected[ref] {
			omitted = append(omitted, ref)
		}
	}
	b, err := json.Marshal(omitted)
	if err != nil {
		return nil, fmt.Errorf("cohort omissions: %w", err)
	}
	c.state["unselected_failures"] = c.omitted
	c.state["unselected_identity_digest"] = versionOf(b)
	b, err = json.Marshal(c.state)
	if err != nil {
		return nil, fmt.Errorf("cohort state: %w", err)
	}
	c.json = string(b)
	c.id = versionOf(b)
	return c, nil
}

// saveCohort preserves exact task/verdict identity and the actual field
// projection, including when screening is disabled. Stored source records are
// never shortened; omissions apply only to the explicitly bounded projection.
func (l *EvidenceLedger) saveCohort(c *evidenceCohort) error {
	b, err := json.Marshal(c.evidence)
	if err != nil {
		return err
	}
	_, err = l.db.Exec(`INSERT OR IGNORE INTO evolution_cohorts
	 (cohort_id, state_json, evidence_json, omitted_count) VALUES (?, ?, ?, ?)`, c.id, c.json, string(b), c.omitted)
	return err
}
