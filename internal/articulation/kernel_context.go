package articulation

import (
	"fmt"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// =============================================================================
// KERNEL CONTEXT INJECTION HELPERS
// =============================================================================
// These functions allow shards to query the kernel for injectable context
// without fully replacing their existing prompt templates.

// GetKernelContext queries the kernel for injectable context atoms for a specific shard.
// This is used by shards that want to augment their existing prompts with kernel-derived context.
// Returns the context as a formatted string ready for insertion into prompts.
func GetKernelContext(kernel KernelQuerier, shardID string) (string, error) {
	if kernel == nil {
		return "", nil
	}

	pa, err := NewPromptAssembler(kernel)
	if err != nil {
		return "", err
	}

	return pa.BuildContextSection(shardID)
}

// BuildContextSection is a public wrapper around the context building logic.
// Returns a formatted string with all injectable context atoms for the shard.
func (pa *PromptAssembler) BuildContextSection(shardID string) (string, error) {
	if pa.kernel == nil {
		return "", nil
	}

	var sections []string

	// Query for injectable context
	contextSection := pa.queryAndFormatContext(shardID)
	if contextSection != "" {
		sections = append(sections, contextSection)
	}

	// Query for specialist knowledge
	specialistSection := pa.queryAndFormatSpecialistKnowledge(shardID)
	if specialistSection != "" {
		sections = append(sections, specialistSection)
	}

	if len(sections) == 0 {
		return "", nil
	}

	return strings.Join(sections, "\n\n"), nil
}

// queryAndFormatContext queries injectable_context and formats it for prompt injection.
func (pa *PromptAssembler) queryAndFormatContext(shardID string) string {
	facts, err := pa.getInjectableContextFacts(shardID)
	if err != nil {
		logging.ArticulationDebug("Failed to query injectable_context: %v", err)
		return ""
	}

	var atoms []string
	for _, fact := range facts {
		if len(fact.Args) < 2 {
			continue
		}

		if atom := types.ExtractString(fact.Args[1]); atom != "" {
			atoms = append(atoms, atom)
		}
	}

	if len(atoms) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("// KERNEL-INJECTED CONTEXT (from spreading activation)\n")
	for _, atom := range atoms {
		sb.WriteString("// - ")
		sb.WriteString(atom)
		sb.WriteString("\n")
	}

	logging.ArticulationDebug("Built kernel context section with %d atoms", len(atoms))
	return sb.String()
}

// queryAndFormatSpecialistKnowledge queries specialist_knowledge and formats it.
func (pa *PromptAssembler) queryAndFormatSpecialistKnowledge(shardID string) string {
	q := fmt.Sprintf("specialist_knowledge(%q, _, _)", shardID)
	facts, err := pa.kernel.Query(q)
	if err != nil {
		logging.ArticulationDebug("Failed to query specialist_knowledge: %v", err)
		return ""
	}

	var blocks []string
	for _, fact := range facts {
		if len(fact.Args) < 3 {
			continue
		}

		topic := types.ExtractString(fact.Args[1])
		content := types.ExtractString(fact.Args[2])
		if topic != "" && content != "" {
			blocks = append(blocks, fmt.Sprintf("## %s\n%s", topic, content))
		}
	}

	if len(blocks) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("// SPECIALIST KNOWLEDGE (Type B/U expertise)\n")
	sb.WriteString(strings.Join(blocks, "\n\n"))

	logging.ArticulationDebug("Built specialist knowledge section with %d topics", len(blocks))
	return sb.String()
}
