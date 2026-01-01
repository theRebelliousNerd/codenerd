package prompt

import "strings"

// IsStructuredOutputOnly returns true when a shard should emit structured output
// without Piggyback or reasoning-trace protocol requirements.
func IsStructuredOutputOnly(shardType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(shardType, "/")))
	switch normalized {
	case "mangle_repair", "legislator":
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
