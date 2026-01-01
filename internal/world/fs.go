package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Scanner handles file system indexing.
type Scanner struct {
	parserPool sync.Pool
	config     ScannerConfig
}

// NewScanner creates a new filesystem Scanner.
func NewScanner() *Scanner {
	return NewScannerWithConfig(DefaultScannerConfig())
}

// NewScannerWithConfig creates a new filesystem Scanner with custom config.
func NewScannerWithConfig(cfg ScannerConfig) *Scanner {
	logging.WorldDebug("Creating new filesystem Scanner")
	if cfg.MaxConcurrency <= 0 {
		cfg = DefaultScannerConfig()
	}
	return &Scanner{
		config: cfg,
		parserPool: sync.Pool{
			New: func() interface{} {
				logging.WorldDebug("Creating new TreeSitterParser in pool")
				return NewTreeSitterParser()
			},
		},
	}
}

// ScanWorkspace scans the entire workspace and returns topology facts.
func (s *Scanner) ScanWorkspace(root string) ([]core.Fact, error) {
	return s.ScanWorkspaceCtx(context.Background(), root)
}

// ScanWorkspaceCtx scans the entire workspace with context support.
func (s *Scanner) ScanWorkspaceCtx(ctx context.Context, root string) ([]core.Fact, error) {
	logging.World("Starting workspace scan: %s", root)
	timer := logging.StartTimer(logging.CategoryWorld, "ScanWorkspace")

	result, err := s.ScanDirectory(ctx, root)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Workspace scan failed: %v", err)
		return nil, err
	}

	elapsed := timer.StopWithInfo()
	logging.World("Workspace scan completed: %d files, %d directories in %v", result.FileCount, result.DirectoryCount, elapsed)
	return result.Facts, nil
}

// calculateHash computes a SHA256 hash of file content.
func calculateHash(path string) (string, error) {
	start := time.Now()
	f, err := os.Open(path)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to open file for hashing: %s - %v", path, err)
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to read file for hashing: %s - %v", path, err)
		return "", err
	}

	hash := hex.EncodeToString(h.Sum(nil))
	logging.WorldDebug("Hash calculated for %s: %s (took %v)", filepath.Base(path), hash[:16], time.Since(start))
	return hash, nil
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

// fileScanResult is sent by worker goroutines to the aggregator.
// OPTIMIZATION: Eliminates mutex convoy by using channel-based aggregation.
type fileScanResult struct {
	fact            core.Fact
	additionalFacts []core.Fact
	language        string
	isTest          bool
	cacheHit        bool
}

// dirScanResult is sent when a directory is discovered.
type dirScanResult struct {
	fact core.Fact
}

// ScanDirectory performs a comprehensive scan of a directory with context support.
// OPTIMIZATION: Uses channel-based result aggregation to eliminate mutex convoy (2-4x speedup).
func (s *Scanner) ScanDirectory(ctx context.Context, root string) (*ScanResult, error) {
	logging.World("Starting directory scan: %s", root)
	timer := logging.StartTimer(logging.CategoryWorld, "ScanDirectory")

	cache := NewFileCache(root)
	defer func() {
		if err := cache.Save(); err != nil {
			logging.Get(logging.CategoryWorld).Error("Failed to save file cache: %v", err)
		}
	}()

	var wg sync.WaitGroup
	maxConc := s.config.MaxConcurrency
	if maxConc <= 0 {
		maxConc = DefaultScannerConfig().MaxConcurrency
	}
	sem := make(chan struct{}, maxConc) // Limit concurrency

	// OPTIMIZATION: Channel-based result aggregation (no mutex needed!)
	fileResults := make(chan fileScanResult, maxConc)
	dirResults := make(chan dirScanResult, 100)
	var skippedDirs int

	// Aggregator goroutine: collects results from worker goroutines
	result := &ScanResult{
		Facts:     make([]core.Fact, 0),
		Languages: make(map[string]int),
	}
	var cacheHits, cacheMisses int
	aggregatorDone := make(chan struct{})
	go func() {
		defer close(aggregatorDone)
		dirCh := dirResults
		fileCh := fileResults
		for dirCh != nil || fileCh != nil {
			select {
			case dir, ok := <-dirCh:
				if !ok {
					dirCh = nil
					continue
				}
				result.DirectoryCount++
				result.Facts = append(result.Facts, dir.fact)

			case file, ok := <-fileCh:
				if !ok {
					fileCh = nil
					continue
				}
				result.FileCount++
				result.Languages[file.language]++
				if file.isTest {
					result.TestFileCount++
				}
				if file.cacheHit {
					cacheHits++
				} else {
					cacheMisses++
				}
				result.Facts = append(result.Facts, file.fact)
				result.Facts = append(result.Facts, file.additionalFacts...)
			}
		}
	}()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			logging.World("Directory scan cancelled via context")
			return ctx.Err()
		default:
		}

		if err != nil {
			logging.Get(logging.CategoryWorld).Warn("Walk error at %s: %v", path, err)
			return err
		}

		rel, _ := filepath.Rel(root, path)

		if info.IsDir() {
			name := info.Name()

			// OPTIMIZATION: Explicitly ignore heavy dependency directories
			// This prevents scanning tens of thousands of irrelevant files.
			ignoredDirs := map[string]bool{
				"node_modules": true,
				"vendor":       true,
				"dist":         true,
				"build":        true,
				".git":         true,
				".nerd":        true,
			}
			if ignoredDirs[name] {
				logging.WorldDebug("Skipping dependency/build directory: %s", path)
				skippedDirs++
				return filepath.SkipDir
			}

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
						logging.WorldDebug("Skipping excluded directory: %s", path)
						skippedDirs++
						return filepath.SkipDir
					}
					logging.WorldDebug("Including allowed hidden directory: %s", path)
					return nil
				}
				logging.WorldDebug("Skipping hidden directory: %s", path)
				skippedDirs++
				return filepath.SkipDir
			}
			// Ignore configured directories/patterns
			if path != root && isIgnoredRel(rel, name, s.config.IgnorePatterns) {
				logging.WorldDebug("Skipping ignored directory: %s", path)
				skippedDirs++
				return filepath.SkipDir
			}
			// OPTIMIZATION: Send to channel instead of locking mutex
			dirResults <- dirScanResult{
				fact: core.Fact{
					Predicate: "directory",
					Args:      []interface{}{path, name},
				},
			}
			logging.WorldDebug("Indexed directory: %s", path)
			return nil
		}

		// Ignore configured files/patterns
		if isIgnoredRel(rel, info.Name(), s.config.IgnorePatterns) {
			return nil
		}

		// CRITICAL FIX: Acquire semaphore BEFORE spawning goroutine
		// This blocks filepath.Walk when worker pool is full, preventing unbounded goroutine spawning
		sem <- struct{}{}

		wg.Add(1)
		go func(path string, info os.FileInfo) {
			defer wg.Done()
			defer func() { <-sem }() // Release token

			fileStart := time.Now()

			// "Hash-Thrashing" Fix: Use Cache
			var hash string
			var cacheHit bool
			cachedHash, hit := cache.Get(path, info)
			if hit {
				hash = cachedHash
				cacheHit = true
				logging.WorldDebug("Cache hit for file: %s", filepath.Base(path))
			} else {
				h, err := calculateHash(path)
				if err != nil {
					logging.Get(logging.CategoryWorld).Warn("Skipping file (hash error): %s - %v", path, err)
					return
				}
				hash = h
				cache.Update(path, info, hash)
				cacheHit = false
				logging.WorldDebug("Cache miss, hashed file: %s", filepath.Base(path))
			}

			ext := filepath.Ext(path)
			lang := detectLanguage(ext, path)

			// Cortex 1.5.0: IsTestFile Logic
			isTest := isTestFile(path)
			isTestStr := "/false"
			if isTest {
				isTestStr = "/true"
				logging.WorldDebug("Detected test file: %s", filepath.Base(path))
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

			var additionalFacts []core.Fact
			// If not a test file and supported language, extract symbols
			if !isTest && (s.config.MaxASTFileBytes <= 0 || info.Size() <= s.config.MaxASTFileBytes) {
				// Borrow a parser from the pool
				parser := s.parserPool.Get().(*TreeSitterParser)
				defer s.parserPool.Put(parser) // Return it when done

				content, err := os.ReadFile(path)
				if err == nil {
					parseStart := time.Now()
					var parseErr error
					switch lang {
					case "go":
						if facts, parseErr := parser.ParseGo(path, content); parseErr == nil {
							additionalFacts = append(additionalFacts, facts...)
							logging.WorldDebug("Parsed Go file: %s (%d symbols, %v)", filepath.Base(path), len(facts), time.Since(parseStart))
						} else {
							logging.Get(logging.CategoryWorld).Warn("Go parse failed: %s - %v", path, parseErr)
						}
					case "mangle":
						facts := extractMangleSymbolFacts(path, string(content))
						additionalFacts = append(additionalFacts, facts...)
						logging.WorldDebug("Parsed Mangle file: %s (%d symbols, %v)", filepath.Base(path), len(facts), time.Since(parseStart))
					case "python":
						if facts, parseErr := parser.ParsePython(path, content); parseErr == nil {
							additionalFacts = append(additionalFacts, facts...)
							logging.WorldDebug("Parsed Python file: %s (%d symbols, %v)", filepath.Base(path), len(facts), time.Since(parseStart))
						} else {
							logging.Get(logging.CategoryWorld).Warn("Python parse failed: %s - %v", path, parseErr)
						}
					case "rust":
						if facts, parseErr := parser.ParseRust(path, content); parseErr == nil {
							additionalFacts = append(additionalFacts, facts...)
							logging.WorldDebug("Parsed Rust file: %s (%d symbols, %v)", filepath.Base(path), len(facts), time.Since(parseStart))
						} else {
							logging.Get(logging.CategoryWorld).Warn("Rust parse failed: %s - %v", path, parseErr)
						}
					case "javascript":
						if facts, parseErr := parser.ParseJavaScript(path, content); parseErr == nil {
							additionalFacts = append(additionalFacts, facts...)
							logging.WorldDebug("Parsed JavaScript file: %s (%d symbols, %v)", filepath.Base(path), len(facts), time.Since(parseStart))
						} else {
							logging.Get(logging.CategoryWorld).Warn("JavaScript parse failed: %s - %v", path, parseErr)
						}
					case "typescript":
						if facts, parseErr := parser.ParseTypeScript(path, content); parseErr == nil {
							additionalFacts = append(additionalFacts, facts...)
							logging.WorldDebug("Parsed TypeScript file: %s (%d symbols, %v)", filepath.Base(path), len(facts), time.Since(parseStart))
						} else {
							logging.Get(logging.CategoryWorld).Warn("TypeScript parse failed: %s - %v", path, parseErr)
						}
					}
					_ = parseErr // Suppress unused warning
				} else {
					logging.Get(logging.CategoryWorld).Warn("Failed to read file for parsing: %s - %v", path, err)
				}
			} else if !isTest && s.config.MaxASTFileBytes > 0 && info.Size() > s.config.MaxASTFileBytes {
				logging.WorldDebug("Skipping fast AST parse for large file: %s (%d bytes)", filepath.Base(path), info.Size())
			}

			// OPTIMIZATION: Send to channel instead of locking mutex
			fileResults <- fileScanResult{
				fact:            fact,
				additionalFacts: additionalFacts,
				language:        lang,
				isTest:          isTest,
				cacheHit:        cacheHit,
			}

			logging.WorldDebug("Indexed file: %s (lang=%s, symbols=%d, took %v)", filepath.Base(path), lang, len(additionalFacts), time.Since(fileStart))
		}(path, info)

		return nil
	})

	// Wait for all file processing goroutines to complete
	wg.Wait()

	// Close channels to signal aggregator we're done
	close(fileResults)
	close(dirResults)

	// Wait for aggregator to finish processing
	<-aggregatorDone

	elapsed := timer.Stop()
	logging.World("Directory scan completed: %d files, %d dirs, %d skipped dirs, cache hits=%d misses=%d, %d facts generated in %v",
		result.FileCount, result.DirectoryCount, skippedDirs, cacheHits, cacheMisses, len(result.Facts), elapsed)

	// Log language breakdown
	if len(result.Languages) > 0 {
		logging.WorldDebug("Language breakdown: %v", result.Languages)
	}

	return result, err
}

// detectLanguage determines the programming language from file extension and path.
func detectLanguage(ext, path string) string {
	ext = strings.ToLower(ext)

	langMap := map[string]string{
		".go":    "go",
		".mg":    "mangle",
		".mangle": "mangle",
		".dl":    "mangle",
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
