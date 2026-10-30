package campaign

import (
	"sort"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// PlanAdvisory is one thing the kernel wants the user to know about a file the
// plan touches.
type PlanAdvisory struct {
	Kind    string // /modularize_first, /fence, /high_impact
	Path    string
	Message string
}

// PlanIntelligence is the kernel's reading of the plan it holds: the levels
// policy derives (one aggregation and a threshold each) and the advisories
// behind them. Policy decides the levels; this only reads them.
type PlanIntelligence struct {
	Risk        string   // /high, /medium or /low; empty when the kernel concluded nothing
	RiskReasons []string // why, for a level above /low
	Complexity  string   // /high, /medium or /low: the plan's blast radius
	Advisories  []PlanAdvisory

	// Strategy is the standing knowledge policy judged relevant to the plan:
	// the goal as vision and the patterns above the priority bar.
	Strategy []PlanStrategy
	// Related names what the knowledge graph links to the plan's files.
	Related []PlanRelation
}

// PlanStrategy is one piece of strategic knowledge the plan should honour.
type PlanStrategy struct{ Concept, Content string }

// PlanRelation is a knowledge-graph neighbour of a file the plan touches.
type PlanRelation struct{ Path, Relation, Entity string }

// planIntelligence asks the decomposer's kernel what the intelligence rules
// concluded about the plan facts it holds.
func (d *Decomposer) planIntelligence() PlanIntelligence {
	return ReadPlanIntelligence(d.kernel)
}

// ReadPlanIntelligence asks a kernel what the intelligence rules concluded about
// the plan facts it holds. Without a kernel, or when a query fails, it reports
// nothing: an advisory is never a reason to fail a decomposition.
func ReadPlanIntelligence(kernel types.Kernel) PlanIntelligence {
	var out PlanIntelligence
	if kernel == nil {
		return out
	}

	risks, err := kernel.Query("intelligence_plan_risk")
	if err != nil {
		logging.CampaignDebug("plan intelligence: risk query failed: %v", err)
	}
	// The kernel derives one level when the heaviest hazard lands in one band;
	// a stale row from an earlier plan in another band would show as two.
	// Show the heavier one: it is the safer thing to tell the user.
	rank := map[string]int{"/low": 1, "/medium": 2, "/high": 3}
	for _, f := range risks {
		if len(f.Args) < 2 {
			continue
		}
		level := types.ExtractString(f.Args[0])
		if rank[level] > rank[out.Risk] {
			out.Risk = level
			out.RiskReasons = nil
		}
		if level == out.Risk && level != "/low" {
			out.RiskReasons = append(out.RiskReasons, types.ExtractString(f.Args[1]))
		}
	}
	sort.Strings(out.RiskReasons)

	complexity, err := kernel.Query("intelligence_plan_complexity")
	if err != nil {
		logging.CampaignDebug("plan intelligence: complexity query failed: %v", err)
	}
	for _, f := range complexity {
		if len(f.Args) < 1 {
			continue
		}
		if level := types.ExtractString(f.Args[0]); rank[level] > rank[out.Complexity] {
			out.Complexity = level
		}
	}

	advisories, err := kernel.Query("intelligence_plan_advisory")
	if err != nil {
		logging.CampaignDebug("plan intelligence: advisory query failed: %v", err)
	}
	for _, f := range advisories {
		if len(f.Args) < 3 {
			continue
		}
		out.Advisories = append(out.Advisories, PlanAdvisory{
			Kind:    types.ExtractString(f.Args[0]),
			Path:    types.ExtractString(f.Args[1]),
			Message: types.ExtractString(f.Args[2]),
		})
	}
	strategies, err := kernel.Query("intelligence_relevant_strategy")
	if err != nil {
		logging.CampaignDebug("plan intelligence: strategy query failed: %v", err)
	}
	for _, f := range strategies {
		if len(f.Args) >= 3 {
			out.Strategy = append(out.Strategy, PlanStrategy{Concept: types.ExtractString(f.Args[1]), Content: types.ExtractString(f.Args[2])})
		}
	}
	sort.Slice(out.Strategy, func(i, j int) bool {
		return out.Strategy[i].Concept+out.Strategy[i].Content < out.Strategy[j].Concept+out.Strategy[j].Content
	})

	related, err := kernel.Query("intelligence_plan_related")
	if err != nil {
		logging.CampaignDebug("plan intelligence: related query failed: %v", err)
	}
	for _, f := range related {
		if len(f.Args) >= 3 {
			out.Related = append(out.Related, PlanRelation{Path: types.ExtractString(f.Args[0]), Relation: types.ExtractString(f.Args[1]), Entity: types.ExtractString(f.Args[2])})
		}
	}
	sort.Slice(out.Related, func(i, j int) bool {
		a, b := out.Related[i], out.Related[j]
		return a.Path+a.Relation+a.Entity < b.Path+b.Relation+b.Entity
	})

	sort.Slice(out.Advisories, func(i, j int) bool {
		a, b := out.Advisories[i], out.Advisories[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Kind < b.Kind
	})
	return out
}
