package chat

import (
	"fmt"
	"strings"

	"codenerd/internal/experts"
	"codenerd/internal/logging"
	"codenerd/internal/shards"
)

// selectShardExperts asks the kernel which shard agents a delegated task needs
// (internal/experts over policy/shard_experts.mg). The same derivation decides
// the knowledge every executor compile carries, so a delegation spawns the
// agents whose knowledge the turn would have been handed anyway. Without a
// kernel there is no decision and no specialist.
func (m Model) selectShardExperts(verb string, files []string) experts.Selection {
	if m.kernel == nil {
		return experts.Selection{}
	}
	sel, err := experts.Select(m.kernel, experts.Task{
		Verb:      verb,
		Targets:   files,
		Workspace: m.workspace,
	})
	if err != nil {
		logging.Get(logging.CategoryRouting).Warn("shard expert selection for %s failed: %v", verb, err)
		return experts.Selection{}
	}
	logging.Routing("[delegation] %s on %d file(s) needs shard agents %v", verb, len(files), sel.Agents())
	return sel
}

// specialistsFromSelection is the delegation view of the kernel's selection,
// in its order (strongest evidence first).
func specialistsFromSelection(sel experts.Selection) []shards.SpecialistMatch {
	out := make([]shards.SpecialistMatch, 0, len(sel.Needs))
	for _, need := range sel.Needs {
		var classification *shards.SpecialistClassification
		if class, ok := shards.GetSpecialistClassification(need.Agent); ok {
			classification = &class
		}
		out = append(out, shards.SpecialistMatch{
			AgentName:      need.Agent,
			Files:          need.Files,
			Score:          float64(need.Confidence) / 100,
			Reason:         expertReason(need),
			Classification: classification,
			ShouldExecute:  need.Execute,
		})
	}
	return out
}

// expertReason names what ties the task to the agent: its technologies.
func expertReason(need experts.Need) string {
	if len(need.Technologies) == 0 {
		return "shard agent domain"
	}
	return fmt.Sprintf("domain: %s", strings.Join(need.Technologies, ", "))
}
