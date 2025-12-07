package coder

import (
	"fmt"
	"strings"
	"time"
)

// =============================================================================
// OUTPUT FORMATTING
// =============================================================================

// buildResponse creates a human-readable response from the result.
func (c *CoderShard) buildResponse(result *CoderResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Coding task completed in %v.\n", result.Duration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("Summary: %s\n", result.Summary))

	if len(result.Edits) > 0 {
		sb.WriteString(fmt.Sprintf("Applied %d edit(s):\n", len(result.Edits)))
		for _, edit := range result.Edits {
			sb.WriteString(fmt.Sprintf("  - %s: %s (%s)\n", edit.Type, edit.File, edit.Language))
		}
	}

	if result.BuildPassed {
		sb.WriteString("Build: PASSED\n")
	} else if len(result.Diagnostics) > 0 {
		sb.WriteString(fmt.Sprintf("Build: FAILED (%d errors)\n", len(result.Diagnostics)))
		for i, diag := range result.Diagnostics {
			if i >= 3 {
				sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(result.Diagnostics)-3))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s:%d: %s\n", diag.FilePath, diag.Line, diag.Message))
		}
	}

	return sb.String()
}
