package prompt

import "strings"

// IsStructuredOutputOnly returns true when a shard's output is parsed by Go and
// must therefore not carry the Piggyback envelope or reasoning-trace protocol.
//
// Two mandatory output contracts in one prompt is not a style problem, it is a
// silent failure. Observed live: the campaign Decomposer compiles as "planner",
// so its prompt contained BOTH the plan schema it needs
// (`"phases": [{"name", "order", "category", ...}]`) and, twice,
// "## OUTPUT PROTOCOL: PIGGYBACK ENVELOPE (MANDATORY)". The model obeyed the one
// that shouted MANDATORY and returned a control_packet with tool_requests.
// decomposer_planning.go found no "phases" key, fell back to a generic
// three-task scaffold, and the CLI printed "Campaign Plan ... Confidence: 50%"
// as if it had planned. A goal naming six specific deliverables produced two
// files, neither of them requested.
//
// Every campaign role is here because every one of them is json.Unmarshal'd:
// planner and taxonomy in decomposer_planning.go, extractor in
// decomposer_requirements.go, librarian in decomposer_documents.go, replanner in
// replan.go, analyzer in assault_tasks.go. The mapping from role to shard type
// is campaign.GetShardTypeForRole.
func IsStructuredOutputOnly(shardType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(shardType, "/")))
	switch normalized {
	case "mangle_repair", "legislator",
		// Campaign roles (campaign.GetShardTypeForRole).
		"planner", "librarian", "extractor", "analyzer":
		return true
	default:
		return false
	}
}

func isPiggybackProtocolAtom(atom *PromptAtom) bool {
	if atom == nil {
		return false
	}
	return strings.HasPrefix(atom.ID, "protocol/piggyback/")
}

func isReasoningProtocolAtom(atom *PromptAtom) bool {
	if atom == nil {
		return false
	}
	return strings.HasPrefix(atom.ID, "protocol/reasoning/")
}

func filterAtomsForStructuredOutput(atoms []*PromptAtom, cc *CompilationContext) []*PromptAtom {
	if len(atoms) == 0 || cc == nil || !IsStructuredOutputOnly(cc.ShardType) {
		return atoms
	}
	filtered := make([]*PromptAtom, 0, len(atoms))
	for _, atom := range atoms {
		if isPiggybackProtocolAtom(atom) || isReasoningProtocolAtom(atom) {
			continue
		}
		filtered = append(filtered, atom)
	}
	return filtered
}
