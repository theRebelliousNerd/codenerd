package campaign

import (
	"path/filepath"
	"regexp"
	"strings"
)

// chunkText splits text into chunks of approximately maxLen runes,
// preferring to break on paragraph (\n\n), line (\n), or word (' ') boundaries
// to preserve semantic integrity for embeddings and LLM context.
func chunkText(text string, maxLen int) []string {
	if maxLen <= 0 {
		maxLen = 2000
	}
	if len(text) == 0 {
		return nil
	}

	var chunks []string
	remaining := text
	for len(remaining) > 0 {
		if len([]rune(remaining)) <= maxLen {
			chunks = append(chunks, remaining)
			break
		}

		rs := []rune(remaining)
		limit := maxLen
		if limit > len(rs) {
			limit = len(rs)
		}
		candidate := string(rs[:limit])

		splitIdx := -1
		if idx := strings.LastIndex(candidate, "\n\n"); idx > 0 {
			splitIdx = idx + 2
		} else if idx := strings.LastIndex(candidate, "\n"); idx > 0 {
			splitIdx = idx + 1
		} else if idx := strings.LastIndex(candidate, " "); idx > 0 {
			splitIdx = idx + 1
		}

		if splitIdx <= 0 {
			splitIdx = len(candidate)
		}

		chunks = append(chunks, strings.TrimRight(remaining[:splitIdx], " "))
		remaining = remaining[splitIdx:]
		remaining = strings.TrimLeft(remaining, " ")
	}
	return chunks
}

// sanitizeCampaignID removes the leading slash and non-alphanum for filesystem safety.
func sanitizeCampaignID(id string) string {
	id = strings.TrimPrefix(id, "/")
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
	id = re.ReplaceAllString(id, "_")
	return id
}

// isSupportedDocExt filters to typical text docs.
func isSupportedDocExt(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".mdx", ".txt", ".rst", ".adoc", ".asciidoc",
		".yaml", ".yml", ".json", ".toml", ".ini", ".cfg",
		".go", ".ts", ".tsx", ".js", ".jsx", ".cs", ".java", ".py":
		return true
	default:
		return false
	}
}
