package tools

import (
	"context"
	"fmt"
)

const (
	// DreamModeConsultationScenarDescription describes the dream mode consultation scenario tool
	DreamModeConsultationScenarDescription = `Dream Mode Consultation: Creates agents.md documentation files in every subdirectory under internal/. 
Each file would describe what's in that directory, what consumes it, and tips for working there.
This is a hypothetical scenario - the tool only describes what it would do without executing any actions.
Input: string (ignored, for compatibility)
Output: string (description of the hypothetical action and its implications)`
)

// dreamModeConsultationScenar provides a hypothetical consultation about creating documentation
// It describes what would happen if we were to create agents.md files in internal/ subdirectories
func dreamModeConsultationScenar(ctx context.Context, input string) (string, error) {
	// This is a dream mode consultation - we only describe what we would do
	consultation := `DREAM MODE CONSULTATION: agents.md Documentation Generation

HYPOTHETICAL ACTION:
I would traverse the internal/ directory structure and create an agents.md file in each subdirectory.

WHAT EACH agents.md WOULD CONTAIN:
1. Directory Purpose: Clear description of what this directory contains and its role in the system
2. Dependencies: What other parts of the system consume this directory's code
3. Key Components: Important files, types, and functions within the directory
4. Working Tips: Best practices, common patterns, and gotchas for developers working here
5. Examples: Typical usage patterns and code snippets

IMPLICATIONS:
- Improved Onboarding: New developers can quickly understand each component's purpose
- Better Architecture Documentation: Living documentation that stays close to the code
- Knowledge Preservation: Critical information about design decisions and patterns
- Maintenance Overhead: Documentation must be kept in sync with code changes
- Consistency Challenge: Ensuring uniform quality and format across all directories

POTENTIAL CHALLENGES:
- Keeping documentation current as code evolves
- Determining the right level of detail for each directory
- Handling directories with mixed responsibilities
- Ensuring the documentation adds value without becoming noise

RECOMMENDATION:
If implementing this, consider:
1. Starting with core directories first
2. Establishing clear templates and guidelines
3. Making documentation updates part of the code review process
4. Automating validation where possible

This consultation is hypothetical - no files were created or modified.`

	return consultation, nil
}

// RegisterDreamModeConsultationScenar registers the dream mode consultation scenario tool
func RegisterDreamModeConsultationScenar(registry ToolRegistry) error {
	if registry == nil {
		return fmt.Errorf("tool registry cannot be nil")
	}

	tool := Tool{
		Name:        "dream_mode_consultation_scenar",
		Description: DreamModeConsultationScenarDescription,
		Handler:     dreamModeConsultationScenar,
	}

	return registry.Register(tool)
}