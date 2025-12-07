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
	"sync"
)

// Scanner handles file system indexing.
type Scanner struct{}

func NewScanner() *Scanner {
	return &Scanner{}
}

func (s *Scanner) ScanWorkspace(root string) ([]core.Fact, error) {
	var facts []core.Fact
	var mu sync.Mutex // Protects facts slice
	cache := NewFileCache(root)
	defer cache.Save()

	// Worker pool for hashing
	var wg sync.WaitGroup
	sem := make(chan struct{}, 20) // Limit concurrency to 20

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			// "Blind Spot" Fix: Allow specific hidden directories
			if strings.HasPrefix(name, ".") && name != "." {
				// Allowlist for hidden configuration directories
				allowed := map[string]bool{
					".github":   true,
					".vscode":   true,
					".circleci": true,
					".config":   true,
					".nerd":     false, // Internal, usually skip
					".git":      false, // Always skip
				}

				if allow, exists := allowed[name]; exists {
					if !allow {
						return filepath.SkipDir
					}
					return nil
				}

				// Default block for other hidden dirs
				return filepath.SkipDir
			}
			return nil
		}

		wg.Add(1)
		go func(path string, info os.FileInfo) {
			defer wg.Done()
			sem <- struct{}{} // Acquire token
			defer func() { <-sem }() // Release token

			// "Hash-Thrashing" Fix: Use Cache
			var hash string
			cachedHash, hit := cache.Get(path, info)
			if hit {
				hash = cachedHash
			} else {
				h, err := calculateHash(path)
				if err != nil {
					// Skip on error
					return
				}
				hash = h
				cache.Update(path, info, hash)
			}

			lang := detectLanguage(filepath.Ext(path), path)

			// Cortex 1.5.0: IsTestFile Logic (match ScanDirectory format)
			isTest := isTestFile(path)
			isTestStr := "/false"
			if isTest {
				isTestStr = "/true"
			}

			// file_topology(Path, Hash, Language, LastModified, IsTestFile)
			fact := core.Fact{
				Predicate: "file_topology",
				Args: []interface{}{
					path,
					hash,
					core.MangleAtom("/" + lang),
					info.ModTime().Unix(),
					core.MangleAtom(isTestStr),
				},
			}
			
			mu.Lock()
			facts = append(facts, fact)
			mu.Unlock()
		}(path, info)

		return nil
	})

	wg.Wait()
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
	var mu sync.Mutex // Protects result
	cache := NewFileCache(root)
	defer cache.Save()

	var wg sync.WaitGroup
	sem := make(chan struct{}, 20) // Limit concurrency

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
			name := info.Name()
			// "Blind Spot" Fix: Allow specific hidden directories
			if strings.HasPrefix(name, ".") && name != "." {
				allowed := map[string]bool{
					".github":   true,
					".vscode":   true,
					".circleci": true,
					".config":   true,
					".nerd":     false,
					".git":      false,
				}

				if allow, exists := allowed[name]; exists {
					if !allow {
						return filepath.SkipDir
					}
					return nil
				}
				return filepath.SkipDir
			}
			mu.Lock()
			result.DirectoryCount++
			mu.Unlock()
			return nil
		}

		wg.Add(1)
		go func(path string, info os.FileInfo) {
			defer wg.Done()
			sem <- struct{}{} // Acquire token
			defer func() { <-sem }() // Release token

			// "Hash-Thrashing" Fix: Use Cache
			var hash string
			cachedHash, hit := cache.Get(path, info)
			if hit {
				hash = cachedHash
			} else {
				h, err := calculateHash(path)
				if err != nil {
					// Skip files we can't hash but don't fail
					return
				}
				hash = h
				cache.Update(path, info, hash)
			}

			ext := filepath.Ext(path)
			lang := detectLanguage(ext, path)

			// Cortex 1.5.0: IsTestFile Logic
			isTest := isTestFile(path)
			isTestStr := "/false"
			if isTest {
				isTestStr = "/true"
			}

			// file_topology(Path, Hash, Language, LastModified, IsTestFile)
			fact := core.Fact{
				Predicate: "file_topology",
				Args: []interface{}{
					path,
					hash,
					core.MangleAtom("/" + lang),
					info.ModTime().Unix(),
					core.MangleAtom(isTestStr),
				},
			}

			mu.Lock()
			result.FileCount++
			result.Languages[lang]++
			if isTest {
				result.TestFileCount++
			}
			result.Facts = append(result.Facts, fact)
			mu.Unlock()
		}(path, info)

		return nil
	})

	wg.Wait()
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

	dirParts := strings.Split(filepath.ToSlash(dir), "/")
	inTestDir := false
	for _, part := range dirParts {
		if part == "tests" || part == "test" || part == "__tests__" {
			inTestDir = true
			break
		}
	}

	if inTestDir {
		ext := filepath.Ext(path)
		if ext == ".py" || ext == ".js" || ext == ".ts" || ext == ".tsx" || ext == ".rs" {
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
