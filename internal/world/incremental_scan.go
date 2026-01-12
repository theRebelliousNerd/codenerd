package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// IncrementalOptions controls incremental scan behavior.
type IncrementalOptions struct {
	// SkipWhenUnchanged returns Unchanged=true when no deltas detected.
	SkipWhenUnchanged bool
}

// IncrementalResult describes an incremental fast scan.
// If Full=true, NewFacts contains a full world snapshot.
type IncrementalResult struct {
	Full           bool
	Unchanged      bool
	NewFacts       []core.Fact
	RetractFacts   []core.Fact
	ChangedFiles   []string
	NewFiles       []string
	DeletedFiles   []string
	FileCount      int
	DirectoryCount int
	Duration       time.Duration

	// New fields for Mangle integration
	ProjectLanguage string
}

func fileFingerprint(info os.FileInfo) string {
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().Unix())
}

// ScanWorkspaceIncremental performs a fast, cache-aware scan.
// It uses FileCache for change detection and LocalStore (if provided) for per-file fact caching.
func (s *Scanner) ScanWorkspaceIncremental(ctx context.Context, root string, db *store.LocalStore, opts IncrementalOptions) (*IncrementalResult, error) {
	start := time.Now()
	logging.World("Starting incremental workspace scan: %s", root)

	cache := NewFileCache(root)
	defer func() {
		if err := cache.Save(); err != nil {
			logging.Get(logging.CategoryWorld).Error("Failed to save file cache: %v", err)
		}
	}()

	// Snapshot previous entries for diffing.
	cache.mu.RLock()
	prevEntries := make(map[string]CacheEntry, len(cache.Entries))
	for k, v := range cache.Entries {
		prevEntries[k] = v
	}
	cache.mu.RUnlock()

	patterns := s.config.IgnorePatterns

	currentFiles := make(map[string]os.FileInfo)
	dirFacts := make([]core.Fact, 0)
	var fileCount, dirCount int

	// Lightweight walk: build current file set and directory facts.
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		name := d.Name()

		if d.IsDir() {
			// Hidden directory handling mirrors full scan.
			if strings.HasPrefix(name, ".") && name != "." && path != root {
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
			if path != root && isIgnoredRel(rel, name, patterns) {
				return filepath.SkipDir
			}
			dirCount++
			dirFacts = append(dirFacts, core.Fact{
				Predicate: "directory",
				Args:      []interface{}{path, name},
			})
			return nil
		}

		if isIgnoredRel(rel, name, patterns) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		currentFiles[path] = info
		fileCount++
		return nil
	}); err != nil {
		logging.WorldWarn("ScanWorkspaceIncremental: walkdir failed for root %s: %v", root, err)
	}

	// If no prior cache, fall back to full scan (first run).
	if len(prevEntries) == 0 {
		fullFacts, err := s.ScanWorkspaceCtx(ctx, root)
		if err != nil {
			return nil, err
		}
		// Persist full snapshot into DB for future incrementals.
		if db != nil {
			grouped := groupFactsByPath(fullFacts)
			for path, facts := range grouped {
				info, statErr := os.Stat(path)
				if statErr != nil {
					continue
				}
				lang := "unknown"
				if len(facts) > 0 {
					for _, f := range facts {
						if f.Predicate == "file_topology" && len(f.Args) >= 3 {
							if la, ok := f.Args[2].(core.MangleAtom); ok {
								lang = strings.TrimPrefix(string(la), "/")
							}
							break
						}
					}
				}
				fp := fileFingerprint(info)
				if err := db.UpsertWorldFile(store.WorldFileMeta{
					Path:        path,
					Lang:        lang,
					Size:        info.Size(),
					ModTime:     info.ModTime().Unix(),
					Hash:        extractHashFromFacts(facts),
					Fingerprint: fp,
				}); err != nil {
					logging.WorldWarn("ScanWorkspaceIncremental: failed to upsert world file %s (full scan): %v", path, err)
				}
				inputs := make([]store.WorldFactInput, 0, len(facts))
				for _, f := range facts {
					inputs = append(inputs, store.WorldFactInput{Predicate: f.Predicate, Args: f.Args})
				}
				if err := db.ReplaceWorldFactsForFile(path, "fast", fp, inputs); err != nil {
					logging.WorldWarn("ScanWorkspaceIncremental: failed to replace world facts for file %s (full scan): %v", path, err)
				}
			}
		}

		res := &IncrementalResult{
			Full:           true,
			NewFacts:       fullFacts,
			FileCount:      fileCount,
			DirectoryCount: dirCount,
			Duration:       time.Since(start),
		}

		// Calculate project language from full set
		if lang := detectProjectLanguage(fullFacts); lang != "" {
			res.ProjectLanguage = lang
			res.NewFacts = append(res.NewFacts, core.Fact{
				Predicate: "project_language",
				Args:      []interface{}{core.MangleAtom("/" + lang)},
			})
		}

		// Entry point detection for full scan
		entryPointFacts := detectEntryPoints(fullFacts)
		res.NewFacts = append(res.NewFacts, entryPointFacts...)

		return res, nil
	}

	changed := make([]string, 0)
	newFiles := make([]string, 0)
	for path, info := range currentFiles {
		if prev, ok := prevEntries[path]; ok {
			if prev.ModTime == info.ModTime().Unix() && prev.Size == info.Size() {
				continue
			}
			changed = append(changed, path)
		} else {
			newFiles = append(newFiles, path)
		}
	}

	deleted := make([]string, 0)
	for path := range prevEntries {
		if _, ok := currentFiles[path]; !ok {
			deleted = append(deleted, path)
		}
	}

	if len(changed) == 0 && len(newFiles) == 0 && len(deleted) == 0 && opts.SkipWhenUnchanged {
		return &IncrementalResult{
			Unchanged:      true,
			FileCount:      fileCount,
			DirectoryCount: dirCount,
			Duration:       time.Since(start),
		}, nil
	}

	// Gather old facts for retraction (fast depth) before mutating cache/DB.
	retractFacts := make([]core.Fact, 0)
	if db != nil {
		for _, p := range append(changed, deleted...) {
			oldInputs, _, err := db.LoadWorldFactsForFile(p, "fast")
			if err != nil || len(oldInputs) == 0 {
				continue
			}
			for _, in := range oldInputs {
				retractFacts = append(retractFacts, core.Fact{Predicate: in.Predicate, Args: in.Args})
			}
		}
	}

	pathsToParse := append([]string{}, changed...)
	pathsToParse = append(pathsToParse, newFiles...)

	maxConc := s.config.MaxConcurrency
	if maxConc <= 0 {
		maxConc = DefaultScannerConfig().MaxConcurrency
	}
	sem := make(chan struct{}, maxConc)

	var wg sync.WaitGroup
	var mu sync.Mutex
	newFacts := make([]core.Fact, 0, len(dirFacts)+len(pathsToParse)*2)

	// Always refresh directory facts on delta scans.
	newFacts = append(newFacts, dirFacts...)

	for _, p := range pathsToParse {
		info := currentFiles[p]
		wg.Add(1)
		go func(path string, info os.FileInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Compute new hash (cache miss by definition)
			hash, err := calculateHash(path)
			if err != nil {
				return
			}

			ext := filepath.Ext(path)
			lang := detectLanguage(ext, path)
			isTest := isTestFile(path)
			isTestStr := "/false"
			if isTest {
				isTestStr = "/true"
			}

			ft := core.Fact{
				Predicate: "file_topology",
				Args: []interface{}{
					path,
					hash,
					core.MangleAtom("/" + lang),
					info.ModTime().Unix(),
					core.MangleAtom(isTestStr),
				},
			}

			additional := make([]core.Fact, 0)
			if !isTest && (s.config.MaxASTFileBytes <= 0 || info.Size() <= s.config.MaxASTFileBytes) {
				parser := s.parserPool.Get().(*TreeSitterParser)
				defer s.parserPool.Put(parser)

				content, readErr := os.ReadFile(path)
				if readErr == nil {
					switch lang {
					case "go":
						if facts, parseErr := parser.ParseGo(path, content); parseErr == nil {
							additional = append(additional, facts...)
						}
					case "mangle":
						additional = append(additional, extractMangleSymbolFacts(path, string(content))...)
					case "python":
						if facts, parseErr := parser.ParsePython(path, content); parseErr == nil {
							additional = append(additional, facts...)
						}
					case "rust":
						if facts, parseErr := parser.ParseRust(path, content); parseErr == nil {
							additional = append(additional, facts...)
						}
					case "javascript":
						if facts, parseErr := parser.ParseJavaScript(path, content); parseErr == nil {
							additional = append(additional, facts...)
						}
					case "typescript":
						if facts, parseErr := parser.ParseTypeScript(path, content); parseErr == nil {
							additional = append(additional, facts...)
						}
					}
				}
			}

			// Update file cache entry.
			cache.Update(path, info, hash)

			// Persist to DB (fast depth).
			if db != nil {
				fp := fileFingerprint(info)
				if err := db.UpsertWorldFile(store.WorldFileMeta{
					Path:        path,
					Lang:        lang,
					Size:        info.Size(),
					ModTime:     info.ModTime().Unix(),
					Hash:        hash,
					Fingerprint: fp,
				}); err != nil {
					logging.WorldWarn("ScanWorkspaceIncremental: failed to upsert world file %s: %v", path, err)
				}
				inputs := make([]store.WorldFactInput, 0, 1+len(additional))
				inputs = append(inputs, store.WorldFactInput{Predicate: ft.Predicate, Args: ft.Args})
				for _, f := range additional {
					inputs = append(inputs, store.WorldFactInput{Predicate: f.Predicate, Args: f.Args})
				}
				if err := db.ReplaceWorldFactsForFile(path, "fast", fp, inputs); err != nil {
					logging.WorldWarn("ScanWorkspaceIncremental: failed to replace world facts for file %s: %v", path, err)
				}
			}

			mu.Lock()
			newFacts = append(newFacts, ft)
			newFacts = append(newFacts, additional...)
			mu.Unlock()
		}(p, info)
	}

	wg.Wait()

	// Handle deletions: drop from DB and cache.
	for _, p := range deleted {
		if db != nil {
			if err := db.DeleteWorldFile(p); err != nil {
				logging.WorldWarn("ScanWorkspaceIncremental: failed to delete world file %s: %v", p, err)
			}
		}
		cache.mu.Lock()
		delete(cache.Entries, p)
		cache.Dirty = true
		cache.mu.Unlock()
	}

	return &IncrementalResult{
		NewFacts:       newFacts,
		RetractFacts:   retractFacts,
		ChangedFiles:   changed,
		NewFiles:       newFiles,
		DeletedFiles:   deleted,
		FileCount:      fileCount,
		DirectoryCount: dirCount,
		Duration:       time.Since(start),
	}, nil
}

func groupFactsByPath(facts []core.Fact) map[string][]core.Fact {
	out := make(map[string][]core.Fact)
	for _, f := range facts {
		switch f.Predicate {
		case "file_topology":
			if len(f.Args) > 0 {
				if p, ok := f.Args[0].(string); ok {
					out[p] = append(out[p], f)
				}
			}
		case "symbol_graph", "dependency_link", "code_defines", "code_calls", "assigns",
			"guards_return", "guards_block", "guard_dominates", "safe_access",
			"uses", "call_arg", "error_checked_return", "error_checked_block", "function_scope":
			// These world facts include a path arg somewhere; for persistence we key by file_topology path.
			// We will attach them later when iterating grouped files.
		default:
			// Ignore other predicates here.
		}
	}
	// Attach non-topology world facts to their file by scanning args for a path.
	for _, f := range facts {
		if f.Predicate == "file_topology" {
			continue
		}
		var pathArg string
		for _, a := range f.Args {
			if s, ok := a.(string); ok && strings.Contains(s, string(filepath.Separator)) {
				pathArg = s
				break
			}
		}
		if pathArg != "" {
			out[pathArg] = append(out[pathArg], f)
		}
	}
	return out
}

func extractHashFromFacts(facts []core.Fact) string {
	for _, f := range facts {
		if f.Predicate == "file_topology" && len(f.Args) >= 2 {
			if h, ok := f.Args[1].(string); ok {
				return h
			}
		}
	}
	return ""
}

// detectProjectLanguage aggregates file stats to identify dominant language
func detectProjectLanguage(facts []core.Fact) string {
	counts := make(map[string]int)
	for _, f := range facts {
		if f.Predicate == "file_topology" && len(f.Args) >= 3 {
			if langAtom, ok := f.Args[2].(core.MangleAtom); ok {
				lang := strings.TrimPrefix(string(langAtom), "/")
				if lang != "unknown" && lang != "text" {
					counts[lang]++
				}
			}
		}
	}

	// Simple majority wins
	bestLang := ""
	maxCount := 0
	for lang, count := range counts {
		if count > maxCount {
			maxCount = count
			bestLang = lang
		}
	}
	return bestLang
}

// detectEntryPoints uses heuristics to identify entry points based on file paths and content facts
func detectEntryPoints(facts []core.Fact) []core.Fact {
	entryPoints := make([]core.Fact, 0)
	hasMainSymbol := make(map[string]bool)

	// Pass 1: Collect AST-based entry point candidates
	for _, f := range facts {
		if f.Predicate == "symbol_graph" && len(f.Args) >= 4 {
			// Args: [id, kind, visibility, path, signature]
			id, _ := f.Args[0].(string)
			kind, _ := f.Args[1].(string)
			path, ok := f.Args[3].(string)

			if ok {
				// Go: package main
				if kind == "package" && id == "package:main" {
					hasMainSymbol[path] = true
				}
				// Go: func main
				if kind == "function" && id == "func:main" {
					hasMainSymbol[path] = true
				}
			}
		}
	}

	// Pass 2: Identify files and apply heuristics
	for _, f := range facts {
		if f.Predicate == "file_topology" && len(f.Args) > 0 {
			path, ok := f.Args[0].(string)
			if !ok {
				continue
			}

			isEntry := false

			// Simple Path Heuristics
			if strings.HasSuffix(path, "main.go") ||
				strings.HasSuffix(path, "__main__.py") ||
				strings.HasSuffix(path, "index.js") ||
				strings.HasSuffix(path, "index.ts") {
				isEntry = true
			}

			// AST Heuristics
			if !isEntry && hasMainSymbol[path] {
				isEntry = true
			}

			if isEntry {
				entryPoints = append(entryPoints, core.Fact{
					Predicate: "entry_point",
					Args:      []interface{}{path},
				})
			}
		}
	}
	return entryPoints
}
