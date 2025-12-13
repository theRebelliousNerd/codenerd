package coder

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// resolvePath resolves a relative path to absolute.
func (c *CoderShard) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.coderConfig.WorkingDir, path)
}

// hashContent returns SHA256 hash of content.
func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:8]) // First 8 bytes for brevity
}

// detectLanguage detects programming language from file extension.
func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescript"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".kt":
		return "kotlin"
	case ".swift":
		return "swift"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".cs":
		return "csharp"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".c":
		return "c"
	case ".h", ".hpp":
		return "c"
	case ".sql":
		return "sql"
	case ".sh", ".bash":
		return "bash"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".md":
		return "markdown"
	case ".html":
		return "html"
	case ".css":
		return "css"
	case ".scss", ".sass":
		return "scss"
	default:
		return "unknown"
	}
}

// languageDisplayName returns a display name for a language.
func languageDisplayName(lang string) string {
	names := map[string]string{
		"go":         "Go",
		"python":     "Python",
		"typescript": "TypeScript",
		"javascript": "JavaScript",
		"rust":       "Rust",
		"java":       "Java",
		"kotlin":     "Kotlin",
		"swift":      "Swift",
		"ruby":       "Ruby",
		"php":        "PHP",
		"csharp":     "C#",
		"cpp":        "C++",
		"c":          "C",
		"sql":        "SQL",
		"bash":       "Bash",
		"yaml":       "YAML",
		"json":       "JSON",
		"markdown":   "Markdown",
		"html":       "HTML",
		"css":        "CSS",
		"scss":       "SCSS",
	}
	if name, ok := names[lang]; ok {
		return name
	}
	return "code"
}

// isTestFile determines if a file is a test file.
func isTestFile(path string) bool {
	base := filepath.Base(path)
	lowerBase := strings.ToLower(base)

	// Go tests
	if strings.HasSuffix(lowerBase, "_test.go") {
		return true
	}

	// JavaScript/TypeScript tests
	if strings.Contains(lowerBase, ".test.") || strings.Contains(lowerBase, ".spec.") {
		return true
	}

	// Python tests
	if strings.HasPrefix(lowerBase, "test_") || strings.HasSuffix(lowerBase, "_test.py") {
		return true
	}

	// Test directories
	dir := filepath.ToSlash(filepath.Dir(path))
	dir = "/" + strings.Trim(dir, "/") + "/"
	if strings.Contains(dir, "/tests/") || strings.Contains(dir, "/test/") || strings.Contains(dir, "/__tests__/") {
		return true
	}

	return false
}
