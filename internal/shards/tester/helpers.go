package tester

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// hashContent returns a hash of the content for deduplication.
func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:8]) // First 8 bytes for brevity
}

// detectLanguage detects the programming language from file extension.
func detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "/go"
	case ".py":
		return "/python"
	case ".ts", ".tsx":
		return "/typescript"
	case ".js", ".jsx":
		return "/javascript"
	case ".rs":
		return "/rust"
	case ".java":
		return "/java"
	case ".cs":
		return "/csharp"
	case ".rb":
		return "/ruby"
	case ".php":
		return "/php"
	default:
		return "/unknown"
	}
}

// Ensure fmt is used (for parsing.go which needs it)
var _ = fmt.Sprintf
