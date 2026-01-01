package chat

import (
	"fmt"
	"strings"
)

const maxKnowledgeResultChars = 2000

func formatKnowledgeResults(results []KnowledgeResult) string {
	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Specialist Knowledge Results\n\n")

	for _, kr := range results {
		if kr.Specialist == "" && kr.Query == "" && kr.Response == "" && kr.Error == nil {
			continue
		}

		name := kr.Specialist
		if name == "" {
			name = "specialist"
		}

		sb.WriteString(fmt.Sprintf("### %s\n", name))
		if kr.Query != "" {
			sb.WriteString(fmt.Sprintf("**Query:** %s\n\n", kr.Query))
		}
		if kr.Purpose != "" {
			sb.WriteString(fmt.Sprintf("**Purpose:** %s\n\n", kr.Purpose))
		}

		if kr.Error != nil {
			sb.WriteString(fmt.Sprintf("**Error:** %v\n\n", kr.Error))
			continue
		}

		if kr.Response == "" {
			sb.WriteString("_No response._\n\n")
			continue
		}

		response := kr.Response
		if len(response) > maxKnowledgeResultChars {
			response = response[:maxKnowledgeResultChars] + "\n\n[...truncated for brevity]"
		}

		sb.WriteString(response)
		sb.WriteString("\n\n")
	}

	return strings.TrimSpace(sb.String())
}
