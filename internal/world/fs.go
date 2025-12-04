package world

import (
	"codenerd/internal/core"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Scanner handles file system indexing.
type Scanner struct{}

func NewScanner() *Scanner {
	return &Scanner{}
}

func (s *Scanner) ScanWorkspace(root string) ([]core.Fact, error) {
	var facts []core.Fact

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir // Skip hidden dirs like .git
			}
			return nil
		}

		hash, err := calculateHash(path)
		if err != nil {
			return err
		}

		ext := filepath.Ext(path)
		lang := strings.TrimPrefix(ext, ".")
		if lang == "" {
			lang = "unknown"
		}

		// Cortex 1.5.0: IsTestFile Logic
		isTest := "false"
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "test.py") {
			isTest = "true"
		}

		// file_topology(Path, Hash, Language, LastModified, IsTestFile)
		fact := core.Fact{
			Predicate: "file_topology",
			Args: []interface{}{
				path,
				hash,
				lang,
				info.ModTime().Unix(),
				isTest,
			},
		}
		facts = append(facts, fact)
		return nil
	})

	return facts, err
}

func calculateHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// ScanResult represents the result of a directory scan.
type ScanResult struct {
	FileCount      int
	DirectoryCount int
	Facts          []core.Fact
	Languages      map[string]int // language -> count
	TestFileCount  int
}

// ToFacts returns all facts from the scan result.
func (r *ScanResult) ToFacts() []core.Fact {
	return r.Facts
}

// ScanDirectory performs a comprehensive scan of a directory with context support.
func (s *Scanner) ScanDirectory(ctx context.Context, root string) (*ScanResult, error) {
	result := &ScanResult{
		Facts:     make([]core.Fact, 0),
		Languages: make(map[string]int),
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return err
		}

		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir // Skip hidden dirs like .git
			}
			result.DirectoryCount++
			return nil
		}

		result.FileCount++

		hash, err := calculateHash(path)
		if err != nil {
			// Skip files we can't hash but don't fail
			return nil
		}

		ext := filepath.Ext(path)
		lang := detectLanguage(ext, path)
		result.Languages[lang]++

		// Cortex 1.5.0: IsTestFile Logic
		isTest := isTestFile(path)
		if isTest {
			result.TestFileCount++
		}

		isTestStr := "false"
		if isTest {
			isTestStr = "true"
		}

		// file_topology(Path, Hash, Language, LastModified, IsTestFile)
		fact := core.Fact{
			Predicate: "file_topology",
			Args: []interface{}{
				path,
				hash,
				"/" + lang,
				info.ModTime().Unix(),
				"/" + isTestStr,
			},
		}
		result.Facts = append(result.Facts, fact)

		return nil
	})

	return result, err
}

// detectLanguage determines the programming language from file extension and path.
func detectLanguage(ext, path string) string {
	ext = strings.ToLower(ext)

	langMap := map[string]string{
		".go":    "go",
		".py":    "python",
		".js":    "javascript",
		".ts":    "typescript",
		".tsx":   "typescript",
		".jsx":   "javascript",
		".rs":    "rust",
		".java":  "java",
		".kt":    "kotlin",
		".rb":    "ruby",
		".php":   "php",
		".c":     "c",
		".cpp":   "cpp",
		".cc":    "cpp",
		".h":     "c",
		".hpp":   "cpp",
		".cs":    "csharp",
		".swift": "swift",
		".scala": "scala",
		".clj":   "clojure",
		".ex":    "elixir",
		".exs":   "elixir",
		".erl":   "erlang",
		".hs":    "haskell",
		".ml":    "ocaml",
		".lua":   "lua",
		".r":     "r",
		".sql":   "sql",
		".sh":    "shell",
		".bash":  "shell",
		".zsh":   "shell",
		".ps1":   "powershell",
		".yaml":  "yaml",
		".yml":   "yaml",
		".json":  "json",
		".xml":   "xml",
		".html":  "html",
		".css":   "css",
		".scss":  "scss",
		".sass":  "sass",
		".less":  "less",
		".md":    "markdown",
		".rst":   "rst",
		".txt":   "text",
		".toml":  "toml",
		".ini":   "ini",
		".cfg":   "config",
		".conf":  "config",
	}

	if lang, ok := langMap[ext]; ok {
		return lang
	}

	// Check for special files
	base := filepath.Base(path)
	switch base {
	case "Dockerfile", "dockerfile":
		return "dockerfile"
	case "Makefile", "makefile", "GNUmakefile":
		return "makefile"
	case "CMakeLists.txt":
		return "cmake"
	case "go.mod", "go.sum":
		return "go_mod"
	case "package.json":
		return "npm"
	case "Cargo.toml":
		return "cargo"
	case "requirements.txt", "setup.py", "pyproject.toml":
		return "python_config"
	}

	return "unknown"
}

// isTestFile determines if a file is a test file.
func isTestFile(path string) bool {
	base := filepath.Base(path)
	dir := filepath.Dir(path)

	// Go tests
	if strings.HasSuffix(path, "_test.go") {
		return true
	}

	// Python tests
	if strings.HasSuffix(path, "_test.py") || strings.HasPrefix(base, "test_") {
		return true
	}
	if strings.Contains(dir, "tests") || strings.Contains(dir, "test") {
		ext := filepath.Ext(path)
		if ext == ".py" || ext == ".js" || ext == ".ts" {
			return true
		}
	}

	// JavaScript/TypeScript tests
	if strings.HasSuffix(path, ".test.js") || strings.HasSuffix(path, ".test.ts") ||
		strings.HasSuffix(path, ".spec.js") || strings.HasSuffix(path, ".spec.ts") ||
		strings.HasSuffix(path, ".test.tsx") || strings.HasSuffix(path, ".spec.tsx") {
		return true
	}

	// Java tests
	if strings.HasSuffix(path, "Test.java") || strings.HasSuffix(path, "Tests.java") {
		return true
	}

	// Rust tests
	if strings.Contains(dir, "tests") && strings.HasSuffix(path, ".rs") {
		return true
	}

	return false
}
