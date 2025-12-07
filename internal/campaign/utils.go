package campaign

import (
	"path/filepath"
	"regexp"
	"strings"
)

// chunkText splits a string into rune-safe chunks of maxLen (approx chars).
func chunkText(text string, maxLen int) []string {
	if maxLen <= 0 {
		maxLen = 2000
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	chunks := make([]string, 0, (len(runes)/maxLen)+1)
	for i := 0; i < len(runes); i += maxLen {
		end := i + maxLen
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
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
