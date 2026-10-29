package orient

import (
	"codenerd/internal/config"
	"codenerd/internal/embedding"
	"codenerd/internal/types"
	"context"
	"encoding/json"
	"fmt"
)

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

// Run evaluates the same embedded program used by init over caller measurements.
func Run(ctx context.Context, facts []types.Fact) (*Outcome, error) {
	params := engineParams(config.DefaultOrientConfig())
	overridden := map[string]bool{}
	for _, fact := range facts {
		if fact.Predicate == "config_param" && len(fact.Args) == 2 {
			overridden[types.ExtractString(fact.Args[0])] = true
		}
	}
	var kept []config.Param
	for _, param := range params {
		if !overridden[types.ExtractString(param.Key)] {
			kept = append(kept, param)
		}
	}
	for _, fact := range facts {
		if fact.Predicate == "config_param" && len(fact.Args) == 2 {
			kept = append(kept, config.Param{Key: types.ExtractString(fact.Args[0]), Value: number(fact.Args[1])})
		}
	}
	engine, err := newEngine(kept)
	if err != nil {
		return nil, err
	}
	defer engine.Close()
	if err := engine.Assert(facts); err != nil {
		return nil, err
	}
	if err := engine.Evaluate(ctx); err != nil {
		return nil, err
	}
	out := &Outcome{}
	var qerr error
	take := func(pred string, dst *[]types.Fact) {
		if qerr != nil {
			return
		}
		rows, err := engine.Query(pred)
		if err != nil {
			qerr = err
			return
		}
		*dst = rows
	}
	take("orient_agent", &out.OrientAgent)
	take("orient_agent_topic", &out.OrientAgentTopic)
	take("orient_agent_description", &out.OrientAgentDescription)
	take("orient_agent_permission", &out.OrientAgentPermission)
	take("orient_agent_priority", &out.OrientAgentPriority)
	take("orient_agent_knowledge", &out.OrientAgentKnowledge)
	take("orient_research_topic", &out.OrientResearchTopic)
	take("agent_source_winner", &out.AgentSourceWinner)
	take("agent_source_loser", &out.AgentSourceLoser)
	take("agent_source_duplicate", &out.AgentSourceDuplicate)
	take("config_param", &out.ConfigParam)
	take("config_param_missing", &out.ConfigParamMissing)
	if qerr != nil {
		return nil, qerr
	}
	return out, nil
}

// Inspect scans root, asserts the measurements and any extra rows (agent
// sources, role claims), and evaluates. extra's doc_tie rows are ignored:
// this function assigns the single ordinal space. emb may be nil; similarity
// is then skipped and the report says so.
//
// Init retains its engine between transduction and materialization; Inspect
// is the read-only library view over one measurement pass.
func Inspect(ctx context.Context, root string, cfg *config.OrientConfig, emb embedding.EmbeddingEngine, extra []types.Fact) (*Report, error) {
	if cfg == nil {
		return nil, fmt.Errorf("orient config is nil")
	}
	root, err := absPath(root)
	if err != nil {
		return nil, err
	}
	hist, err := ScanHistory(ctx, root)
	if err != nil {
		return nil, err
	}
	docs, err := CollectDocs(ctx, root, *cfg, emb)
	if err != nil {
		return nil, err
	}
	facts := make([]types.Fact, 0, len(hist.Facts)+len(docs.Facts)+len(extra))
	facts = append(facts, hist.Facts...)
	facts = append(facts, docs.Facts...)
	for _, f := range extra {
		if f.Predicate == "doc_tie" {
			continue
		}
		facts = append(facts, f)
	}
	facts = append(facts, DocTieFacts(tiePaths(facts))...)

	eng, err := NewEngine(cfg)
	if err != nil {
		return nil, err
	}
	defer eng.Close()
	if err := eng.Assert(facts); err != nil {
		return nil, err
	}
	if err := eng.Evaluate(ctx); err != nil {
		return nil, err
	}
	rep, err := buildReport(eng)
	if err != nil {
		return nil, err
	}
	rep.Workspace = root
	rep.Shallow = hist.Shallow
	rep.Empty = hist.Empty
	rep.Commits = hist.Commits
	rep.SimilarityNote = docs.SimilarityNote
	rep.Unreadable = docs.Unreadable
	if docs.CacheWriteError != "" {
		rep.CacheNote = docs.CacheWriteError
	}
	if hist.Shallow {
		rep.HistoryNote = "History is shallow (git rev-parse --is-shallow-repository). Eras, generations, bursts, cohorts, evolution and origin sources were not drawn, because first-commit times in a truncated clone are not the repository's."
	} else if hist.Empty {
		rep.HistoryNote = "The repository has no commits. There is no timeline to draw."
	}
	if len(rep.Vision) == 0 {
		rep.VisionNote = "none derivable (no doc_role_claim)"
	}
	return rep, nil
}

// JSON is the report, indented. Every omitted candidate and every
// unreadable path is included; nothing is trimmed to fit a screen.
func (r *Report) JSON() ([]byte, error) {
	if r.Unreadable == nil {
		r.Unreadable = []string{}
	}
	if r.Bursts == nil {
		r.Bursts = []string{}
	}
	if r.Cohorts == nil {
		r.Cohorts = []string{}
	}
	return json.MarshalIndent(r, "", "  ")
}
