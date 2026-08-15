package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"context"
	"fmt"
	"io/fs"
	"maps"
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

// fileFingerprint returns a cheap content-identity hint built from size +
// mtime. The mtime is captured at NANOSECOND resolution so back-to-back
// writes within the same second (formatter on save, go-generate loops)
// still invalidate the cache. Earlier this used Unix() (second
// resolution), which let stale facts persist across rapid edits.
func fileFingerprint(info os.FileInfo) string {
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
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
	maps.Copy(prevEntries, cache.Entries)
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
				Args:      []any{canonicalScanPath(root, path), name},
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

		res := &IncrementalResult{
			Full:            true,
			NewFacts:        fullFacts,
			FileCount:       fileCount,
			DirectoryCount:  dirCount,
			Duration:        time.Since(start),
			ProjectLanguage: detectProjectLanguage(fullFacts),
		}

		// project_language / entry_point are emitted by ScanDirectory itself
		// now. They used to be appended here, which meant `nerd scan` (which
		// calls ScanWorkspaceCtx directly) produced neither.
		if db != nil {
			// One persistence pass, root-aware. There used to be a second,
			// near-identical loop above this one that stat'ed canonical paths
			// as if they were openable: it only worked when the process
			// happened to be chdir'd into the workspace, and silently persisted
			// nothing otherwise.
			if err := PersistFastSnapshotToDBInRoot(db, root, res.NewFacts); err != nil {
				logging.WorldWarn("ScanWorkspaceIncremental: failed to persist full world snapshot: %v", err)
			}
		}

		return res, nil
	}

	changed := make([]string, 0)
	newFiles := make([]string, 0)
	for path, info := range currentFiles {
		if prev, ok := prevEntries[path]; ok {
			// Nanosecond comparison — see fileFingerprint comment. First scan
			// after upgrade may flag every row as changed (one-time re-scan);
			// steady state stabilises after that.
			if prev.ModTime == info.ModTime().UnixNano() && prev.Size == info.Size() {
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
	// Keyed by CANONICAL path: the store rows are written under the canonical
	// identity, and looking them up by the absolute walk path (as this did)
	// missed every row, so no scan ever retracted anything and superseded facts
	// piled up in the kernel forever.
	retractFacts := make([]core.Fact, 0)
	if db != nil {
		for _, p := range append(changed, deleted...) {
			oldInputs, _, err := db.LoadWorldFactsForFile(canonicalScanPath(root, p), "fast")
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
	var updates []store.FileUpdates
	newFacts := make([]core.Fact, 0, len(dirFacts)+len(pathsToParse)*2)

	// Always refresh directory facts on delta scans.
	newFacts = append(newFacts, dirFacts...)

	// Import resolution index over the WHOLE current file set, not just the
	// delta: an edge from a changed file into an untouched package still has to
	// resolve. Built once, read-only, shared by the workers so each file's
	// resolved edges become part of that file's fact set — which is what makes
	// them persist and retract with the file. Resolving after the DB write
	// instead would leave resolved edges in the kernel that no later scan could
	// retract, so a deleted import kept its edge forever.
	canonicalAll := make([]string, 0, len(currentFiles))
	for p := range currentFiles {
		canonicalAll = append(canonicalAll, canonicalScanPath(root, p))
	}
	importIndex := newRepoFileIndex(root, canonicalAll)

	for _, p := range pathsToParse {
		path := p
		info := currentFiles[p]
		wg.Go(func() {
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

			canonical := canonicalScanPath(root, path)
			ft := core.Fact{
				Predicate: "file_topology",
				Args: []any{
					canonical,
					hash,
					core.MangleAtom("/" + lang),
					info.ModTime().UnixNano(),
					core.MangleAtom(isTestStr),
				},
			}

			additional := make([]core.Fact, 0)

			// file_dir companion fact (mirrors the full scan in fs.go): keyed to the
			// same path as file_topology above so mock_file and other rules can join
			// files within one package directory instead of Cartesian-joining the
			// whole repo. Emitted here too so incrementally re-scanned files keep
			// their directory key.
			additional = append(additional, core.Fact{
				Predicate: "file_dir",
				Args:      []any{canonical, canonicalDir(canonical)},
			})
			// test_file_for(TestFile, SourceFile): pairing computed by the world
			// scanner because Mangle has no string manipulation for the x_test.go
			// convention. Coverage is deliberately conservative: a source file
			// covered only by a package-level test with another name is missed,
			// which under-reports coverage and leaves the four gating rules
			// cautious rather than falsely permissive — cautious is the correct
			// direction to be wrong in for a rule that gates refactors and writes.
			if strings.HasSuffix(canonical, "_test.go") {
				sourceCanonical := strings.TrimSuffix(canonical, "_test.go") + ".go"
				sourceAbs := strings.TrimSuffix(path, "_test.go") + ".go"
				if _, err := os.Stat(sourceAbs); err == nil {
					additional = append(additional, core.Fact{
						Predicate: "test_file_for",
						Args:      []any{types.MangleString(canonical), types.MangleString(sourceCanonical)},
					})
				}
			}
			if !isTest && (s.config.MaxASTFileBytes <= 0 || info.Size() <= s.config.MaxASTFileBytes) {
				parser := s.parserPool.Get().(*TreeSitterParser)
				defer s.parserPool.Put(parser)

				content, readErr := os.ReadFile(path)
				if readErr == nil {
					// The parsers are handed the CANONICAL path, not the walk
					// path: every fact they emit carries it as the file
					// identity. Passing the absolute walk path here (as this
					// did) gave symbol_graph and dependency_link an identity no
					// file_topology row shared, so every rule joining symbols to
					// files derived nothing after an incremental scan.
					switch lang {
					case "go":
						if facts, parseErr := parser.ParseGo(canonical, content); parseErr == nil {
							additional = append(additional, facts...)
						}
					case "mangle":
						additional = append(additional, extractMangleSymbolFacts(canonical, string(content))...)
					case "python":
						if facts, parseErr := parser.ParsePython(canonical, content); parseErr == nil {
							additional = append(additional, facts...)
						}
					case "rust":
						if facts, parseErr := parser.ParseRust(canonical, content); parseErr == nil {
							additional = append(additional, facts...)
						}
					case "javascript":
						if facts, parseErr := parser.ParseJavaScript(canonical, content); parseErr == nil {
							additional = append(additional, facts...)
						}
					case "typescript":
						if facts, parseErr := parser.ParseTypeScript(canonical, content); parseErr == nil {
							additional = append(additional, facts...)
						}
					}
				}
			}
			// Resolve this file's imports into file->file edges while its facts
			// are still a unit, so they are stored and retracted with it.
			additional = append(additional, resolveDependencyLinksWithIndex(importIndex, additional)...)

			// Update file cache entry.
			cache.Update(path, info, hash)

			mu.Lock()
			newFacts = append(newFacts, ft)
			newFacts = append(newFacts, additional...)
			if db != nil {
				fp := fileFingerprint(info)
				meta := store.WorldFileMeta{
					// Canonical, matching PersistFastSnapshotToDB and the
					// retraction lookup above. Absolute keys here made full and
					// incremental scans write two rows per file.
					Path:        canonical,
					Lang:        lang,
					Size:        info.Size(),
					ModTime:     info.ModTime().UnixNano(),
					Hash:        hash,
					Fingerprint: fp,
				}
				inputs := make([]store.WorldFactInput, 0, 1+len(additional))
				inputs = append(inputs, store.WorldFactInput{Predicate: ft.Predicate, Args: ft.Args})
				for _, f := range additional {
					inputs = append(inputs, store.WorldFactInput{Predicate: f.Predicate, Args: f.Args})
				}
				updates = append(updates, store.FileUpdates{
					Meta:  meta,
					Facts: inputs,
				})
			}
			mu.Unlock()

		})
	}

	wg.Wait()

	if db != nil {
		if err := db.UpdateWorldFilesAndFacts("fast", updates); err != nil {
			logging.WorldWarn("ScanWorkspaceIncremental: failed to batch update files and facts: %v", err)
		}
	}

	// Handle deletions: drop from DB and cache. DB rows are keyed canonically,
	// the cache by walk path.
	if db != nil && len(deleted) > 0 {
		canonicalDeleted := make([]string, 0, len(deleted))
		for _, p := range deleted {
			canonicalDeleted = append(canonicalDeleted, canonicalScanPath(root, p))
		}
		if err := db.DeleteWorldFiles(canonicalDeleted); err != nil {
			logging.WorldWarn("ScanWorkspaceIncremental: failed to batch delete world files: %v", err)
		}
	}
	for _, p := range deleted {
		cache.mu.Lock()
		delete(cache.Entries, p)
		cache.Dirty = true
		cache.mu.Unlock()
	}

	// project_language and entry_point are whole-snapshot properties, so a delta
	// scan that only re-emitted facts for changed files left them frozen at
	// whatever the first full scan saw: a repo that migrated from Python to Go
	// kept claiming /python until someone deleted the cache. Both are recomputed
	// here from the CURRENT file set. project_language is single-valued, so
	// ApplyIncrementalResult retracts it before loading (see
	// SnapshotGlobalPredicates); entry_point is per-file and retracts with its
	// file.
	res := &IncrementalResult{
		NewFacts:       newFacts,
		RetractFacts:   retractFacts,
		ChangedFiles:   changed,
		NewFiles:       newFiles,
		DeletedFiles:   deleted,
		FileCount:      fileCount,
		DirectoryCount: dirCount,
		Duration:       time.Since(start),
	}
	cache.LogStats("incremental")

	globals := s.deriveSnapshotGlobals(root, currentFiles, newFacts)
	res.NewFacts = append(res.NewFacts, globals.facts...)
	res.ProjectLanguage = globals.projectLanguage

	return res, nil
}

// snapshotGlobals holds the whole-snapshot derivations that cannot be computed
// from a single file: the majority language and the entry-point set.
type snapshotGlobals struct {
	projectLanguage string
	facts           []core.Fact
}

// deriveSnapshotGlobals recomputes project_language and entry_point from the
// current file set. Language detection is extension-based (no hashing, no
// parsing) so this stays cheap on a delta scan; entry points combine the same
// path heuristics the full scan uses with the AST evidence available for the
// files this scan actually parsed.
func (s *Scanner) deriveSnapshotGlobals(root string, currentFiles map[string]os.FileInfo, deltaFacts []core.Fact) snapshotGlobals {
	topology := make([]core.Fact, 0, len(currentFiles))
	for p := range currentFiles {
		lang := detectLanguage(filepath.Ext(p), p)
		topology = append(topology, core.Fact{
			Predicate: "file_topology",
			Args:      []any{canonicalScanPath(root, p), "", core.MangleAtom("/" + lang), int64(0), core.MangleAtom("/false")},
		})
	}

	var out snapshotGlobals
	if lang := detectProjectLanguage(topology); lang != "" {
		out.projectLanguage = lang
		out.facts = append(out.facts, core.Fact{
			Predicate: "project_language",
			Args:      []any{core.MangleAtom("/" + lang)},
		})
	}
	// AST-derived entry points (func main / package main) are only available
	// for files this delta parsed; path heuristics cover the rest.
	out.facts = append(out.facts, detectEntryPoints(append(topology, deltaFacts...))...)
	return out
}

// groupFactsByPath buckets a scan's facts by the file each one belongs to, so
// that file's rows can be replaced or deleted as a unit.
//
// Which file a fact belongs to is decided by matching its arguments against the
// file_topology paths in the same snapshot — the authoritative list of what was
// scanned — rather than by guessing which argument looks like a path.
//
// The guess used to be "a string containing a slash", and it silently excluded
// every file at the repository root. "sub/gamma.go" matched; "alpha.go" did not,
// so its symbol_graph, file_dir and entry_point facts were filed under the
// global bucket instead of under the file. Nothing failed — the facts were
// stored, just not against their file — and the cost only appeared two steps
// later: deleting a root-level file retracted its file_topology and left every
// symbol it defined in the kernel forever. Measured on a two-file fixture: a
// nested file persisted 4 rows, a root-level one persisted 1.
//
// Matching against the known set also removes the false-positive half of the
// heuristic, where a symbol id like "pkg/thing.Method" would have been read as a
// path to a file that does not exist.
//
// PRECONDITION: facts is a whole snapshot. file_topology is the file list, so a
// fact naming a file with no file_topology in the same slice is filed as global
// — correct for project_language and directory facts, wrong for a symbol whose
// file was omitted. Both production callers pass a full ScanWorkspaceCtx result,
// which always carries file_topology for every file it walked. Do not call this
// with a partial set; the failure is silent.
func groupFactsByPath(facts []core.Fact) map[string][]core.Fact {
	out := make(map[string][]core.Fact)

	// Pass 1: file_topology is the file list. Nothing else establishes a file.
	knownFiles := make(map[string]struct{})
	for _, f := range facts {
		if f.Predicate != "file_topology" || len(f.Args) == 0 {
			continue
		}
		p, ok := f.Args[0].(string)
		if !ok || p == "" {
			continue
		}
		knownFiles[p] = struct{}{}
		out[p] = append(out[p], f)
	}

	// Pass 2: every other fact goes to the first of its arguments that names a
	// scanned file. For a fact relating two files (a dependency edge) that is
	// the source, which is the file whose parse produced it and therefore the
	// file it must be retracted with.
	for _, f := range facts {
		if f.Predicate == "file_topology" {
			continue
		}
		var owner string
		for _, a := range f.Args {
			s, ok := a.(string)
			if !ok || s == "" {
				continue
			}
			if _, isFile := knownFiles[s]; isFile {
				owner = s
				break
			}
		}
		if owner == "" {
			// Genuinely global: project_language, directory facts, and anything
			// naming no scanned file.
			owner = globalWorldFactsPath
		}
		out[owner] = append(out[owner], f)
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
			// ExtractString rather than a core.MangleAtom assertion: scan
			// output carries the atom, but query readback renders a /name as a
			// plain string, so the assertion silently skipped every row when
			// these facts came back from the kernel.
			lang := strings.TrimPrefix(types.ExtractString(f.Args[2]), "/")
			if lang != "" && lang != "unknown" && lang != "text" {
				counts[lang]++
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
				if (kind == "/package" || kind == "package") && id == "package:main" {
					hasMainSymbol[path] = true
				}
				// Go: func main
				if (kind == "/function" || kind == "function") && id == "func:main" {
					hasMainSymbol[path] = true
				}
			}
		}
	}

	// Pass 2: Identify files and apply heuristics
	emitted := make(map[string]struct{})
	for _, f := range facts {
		if f.Predicate == "file_topology" && len(f.Args) > 0 {
			path, ok := f.Args[0].(string)
			if !ok {
				continue
			}
			// The incremental path feeds this both a synthetic topology row per
			// current file and the delta's real rows, so the same file appears
			// twice; a duplicate entry_point is a duplicate EDB fact.
			if _, dup := emitted[path]; dup {
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
				emitted[path] = struct{}{}
				entryPoints = append(entryPoints, core.Fact{
					Predicate: "entry_point",
					Args:      []any{path},
				})
			}
		}
	}
	return entryPoints
}
