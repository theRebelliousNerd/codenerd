package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/tools"
	"codenerd/internal/types"
	"context"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"sort"
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

// isNonCanonicalWorldPath reports whether a cached world file path was written
// by a pre-canonicalisation scanner: it contains a backslash, is absolute
// (filepath.IsAbs or a Windows drive-letter prefix, which IsAbs misses on
// Linux), or differs from types.SlashClean(path).
func isNonCanonicalWorldPath(p string) bool {
	if p == "" {
		return true
	}
	if strings.Contains(p, "\\") {
		return true
	}
	if filepath.IsAbs(p) {
		return true
	}
	if len(p) >= 2 && p[1] == ':' && ((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z')) {
		return true
	}
	if types.SlashClean(p) != p {
		return true
	}
	return false
}

// ScanWorkspaceIncremental performs a fast, cache-aware scan.
// It uses FileCache for change detection and LocalStore (if provided) for per-file fact caching.
func (s *Scanner) ScanWorkspaceIncremental(ctx context.Context, root string, db *store.LocalStore, opts IncrementalOptions) (*IncrementalResult, error) {
	canonical, canonicalErr := tools.CanonicalWorkspaceRoot(root)
	if canonicalErr != nil {
		return nil, canonicalErr
	}
	root = canonical

	start := time.Now()
	logging.World("Starting incremental workspace scan: %s", root)

	// Retire rows written by pre-canonicalisation scanners: absolute Windows
	// paths (C:\...), backslash-laden keys, or anything that is not already
	// types.SlashClean(path). An incremental scan with
	// SkipWhenUnchanged keys by canonical path, so it would never touch these
	// rows: they are immortal duplicates next to the canonical rows for the
	// same files. Deleting them here lets this pass re-scan their canonical
	// keys as new (no fingerprint yet) — the intended migration.
	if db != nil {
		if cachedPaths, listErr := db.ListWorldFilePaths(); listErr != nil {
			logging.WorldWarn("world cache: failed to list cached paths for canonicalisation: %v", listErr)
		} else {
			nonCanonical := make([]string, 0)
			for _, p := range cachedPaths {
				if isNonCanonicalWorldPath(p) {
					nonCanonical = append(nonCanonical, p)
				}
			}
			if len(nonCanonical) > 0 {
				if delErr := db.DeleteWorldFiles(nonCanonical); delErr != nil {
					logging.WorldWarn("world cache: failed to retire %d non-canonical rows: %v", len(nonCanonical), delErr)
				} else {
					logging.World("world cache: retired %d non-canonical rows", len(nonCanonical))
				}
			}
		}
	}

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

	mem, memErr := s.membership(root)
	if memErr != nil {
		return nil, memErr
	}

	currentFiles := make(map[string]os.FileInfo)
	currentStamps := make(map[string]contentStamp)
	var walkGens dirGenSnapshot
	dirFacts := make([]core.Fact, 0)
	var fileCount, dirCount int

	// Lightweight walk: build current file set and directory facts.
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		name := d.Name()
		member, admErr := mem.Admit(path, d.IsDir())
		if admErr != nil {
			return admErr
		}
		if !member {
			return nil
		}

		if d.IsDir() {
			dirCount++
			dirFacts = append(dirFacts, core.Fact{
				Predicate: "directory",
				Args:      []any{canonicalScanPath(root, path), name},
			})
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		currentFiles[path] = info
		currentStamps[path] = stampFromInfo(path, info, walkGens.of(filepath.Dir(path)))
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
	now := time.Now().UnixNano()
	for path, info := range currentFiles {
		prev, ok := prevEntries[path]
		if !ok {
			newFiles = append(newFiles, path)
			continue
		}
		// Size and mtime are only a pre-check, and only together with the
		// content generation. A legacy entry has no generation, so it is
		// hashed once and the stamp is stored; the file is a change only
		// when that hash differs. A touch of identical bytes refreshes the
		// stamp and stays out of changed, so SkipWhenUnchanged can still
		// return Unchanged.
		stored := stampFromEntry(prev)
		cur := currentStamps[path]
		hash, contentChanged, hashErr := resolveContent(path, cur, stored, now)
		if hashErr != nil {
			logging.WorldWarn("incremental scan: leaving stored rows for %s: %v", path, hashErr)
			continue
		}
		if contentChanged {
			changed = append(changed, path)
			continue
		}
		if !stored.trusts(cur, now) {
			cache.storeStamp(path, info, hash, cur.gen, cur.genOK, cur.genIsClock)
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

	// A Go file's code_calls rows are spelled from the package's symbols, and
	// those symbols are read from every sibling in the directory
	// (cartographer.go symbolsFor). Declaring or removing a type rewrites a
	// sibling that did not change: T(x) in b.go is a call when a.go declares
	// func T and a conversion when a.go declares type T. The deep cache keys
	// by this file's own size and mtime, so it would keep b.go's old rows
	// until a full rescan. Remap the siblings only when the package
	// declaration set actually changed — a body edit compares equal and stays
	// on the one file whose bytes moved.
	callSiblings := goCallSiblings(root, db, currentFiles, changed, newFiles, deleted)
	if len(callSiblings) > 0 {
		logging.World("incremental scan: declaration change remaps %d Go sibling(s)", len(callSiblings))
	}

	// Gather old facts for retraction (fast depth) before mutating cache/DB.
	// Keyed by CANONICAL path: the store rows are written under the canonical
	// identity, and looking them up by the absolute walk path (as this did)
	// missed every row, so no scan ever retracted anything and superseded facts
	// piled up in the kernel forever.
	// Siblings join this list so their fast rows retract and reassert with the
	// file that owns them, the same way a changed file already does.
	retractFacts := make([]core.Fact, 0)
	retractPaths := make([]string, 0, len(changed)+len(deleted)+len(callSiblings))
	retractPaths = append(retractPaths, changed...)
	retractPaths = append(retractPaths, deleted...)
	retractPaths = append(retractPaths, callSiblings...)
	if db != nil {
		for _, p := range retractPaths {
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
	pathsToParse = append(pathsToParse, callSiblings...)

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
			// is_test_file mirrors the full scan (fs.go): the scanner already
			// classifies the file, and the chain negates this mark.
			if isTest {
				additional = append(additional, core.Fact{
					Predicate: "is_test_file",
					Args:      []any{canonical},
				})
			}
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
			if isTest && lang == "go" {
				// Test Go files skip the tree-sitter walker below (as in the full
				// scan), so the header-only parse supplies their file_package row
				// and import tokens; resolution into edges happens below.
				if content, readErr := os.ReadFile(path); readErr == nil {
					additional = append(additional, goTestFileHeaderFacts(canonical, content)...)
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
				fp := formatContentFingerprint(currentStamps[path], hash)
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

	// Call rows are not fast facts: code_calls names functions, not files, so
	// groupFactsByPath would file them under the global bucket and the next
	// delete could not take them with their file. They live in the deep cache,
	// keyed by the file the cartographer mapped. Retract the sibling's previous
	// deep rows and assert the fresh map, per file, the same owner-keyed
	// replacement the fast loop above uses.
	if len(callSiblings) > 0 {
		deepNew, deepOld := refreshSiblingCallRows(ctx, root, db, currentFiles, callSiblings)
		newFacts = append(newFacts, deepNew...)
		retractFacts = append(retractFacts, deepOld...)
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

// goCallSiblings lists Go files in a touched directory whose call rows have to
// be mapped again because a sibling's package-level declaration set changed.
//
// Detection is the declaration set, not the diff. In-process, peekPkgSymbols
// still holds the set from the last map and loadSymbols recomputes it from
// the bytes now on disk; sameDeclarations ignores line spans, so a body edit
// compares equal and this returns nothing. A restarted process has an empty
// cache. The previous set is then the code_defines rows already stored for
// the package, and only when every pre-existing file of that package has a
// deep row — a partial deep scan is not a declaration set, and treating it
// as one would remap the package on a body edit. No stored rows and no cache
// means there is nothing stale to repair, so the siblings stay put.
func goCallSiblings(root string, db *store.LocalStore, current map[string]os.FileInfo, changed, added, deleted []string) []string {
	type dirTouch struct {
		live    []string
		deleted []string
	}
	dirs := map[string]*dirTouch{}
	note := func(p string, del bool) {
		if !strings.HasSuffix(p, ".go") {
			return
		}
		d := filepath.Dir(p)
		touch := dirs[d]
		if touch == nil {
			touch = &dirTouch{}
			dirs[d] = touch
		}
		if del {
			touch.deleted = append(touch.deleted, p)
			return
		}
		touch.live = append(touch.live, p)
	}
	for _, p := range changed {
		note(p, false)
	}
	for _, p := range added {
		note(p, false)
	}
	for _, p := range deleted {
		note(p, true)
	}
	if len(dirs) == 0 {
		return nil
	}

	already := make(map[string]struct{}, len(changed)+len(added))
	for _, p := range changed {
		already[p] = struct{}{}
	}
	for _, p := range added {
		already[p] = struct{}{}
	}
	addedSet := make(map[string]struct{}, len(added))
	for _, p := range added {
		addedSet[p] = struct{}{}
	}

	var out []string
	for dir, touch := range dirs {
		out = append(out, siblingsInGoDir(root, db, dir, current, touch.live, touch.deleted, already, addedSet)...)
	}
	sort.Strings(out)
	return out
}

type goPkgRoster struct {
	lib         []string
	test        []string
	deletedLib  []string
	deletedTest []string
}

func siblingsInGoDir(root string, db *store.LocalStore, dir string, current map[string]os.FileInfo, live, deleted []string, already, added map[string]struct{}) []string {
	byPkg := map[string]*goPkgRoster{}
	roster := func(pkg string) *goPkgRoster {
		r := byPkg[pkg]
		if r == nil {
			r = &goPkgRoster{}
			byPkg[pkg] = r
		}
		return r
	}
	for p := range current {
		if filepath.Dir(p) != dir || !strings.HasSuffix(p, ".go") {
			continue
		}
		pkg, ok := goPackageClause(p)
		if !ok {
			continue
		}
		r := roster(pkg)
		if strings.HasSuffix(p, "_test.go") {
			r.test = append(r.test, p)
		} else {
			r.lib = append(r.lib, p)
		}
	}
	for _, p := range deleted {
		pkg := storedGoPackage(db, root, p)
		if pkg == "" {
			continue
		}
		r := roster(pkg)
		if strings.HasSuffix(p, "_test.go") {
			r.deletedTest = append(r.deletedTest, p)
		} else {
			r.deletedLib = append(r.deletedLib, p)
		}
	}

	touched := touchedGoPackages(root, db, live, deleted)
	if len(touched) == 0 {
		// The changed file's clause did not parse and a deleted file had no
		// stored package. Compare every package in the directory: an
		// unchanged set adds no sibling, and skipping the directory would
		// leave a real declaration change unrepaired.
		for pkg := range byPkg {
			touched[pkg] = struct{}{}
		}
	}

	var out []string
	for pkg := range touched {
		r := byPkg[pkg]
		if r == nil {
			continue
		}
		libCandidates := notIn(r.lib, already)
		if len(libCandidates) > 0 && packageDeclsChanged(dir, pkg, false, db, root, evidenceFiles(r.lib, r.deletedLib, added)) {
			out = append(out, libCandidates...)
		}
		testCandidates := notIn(r.test, already)
		if len(testCandidates) == 0 {
			continue
		}
		testEvidence := evidenceFiles(append(append([]string{}, r.lib...), r.test...), append(append([]string{}, r.deletedLib...), r.deletedTest...), added)
		if packageDeclsChanged(dir, pkg, true, db, root, testEvidence) {
			out = append(out, testCandidates...)
		}
	}
	return out
}

func touchedGoPackages(root string, db *store.LocalStore, live, deleted []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, p := range live {
		if pkg, ok := goPackageClause(p); ok {
			out[pkg] = struct{}{}
		}
	}
	for _, p := range deleted {
		if pkg := storedGoPackage(db, root, p); pkg != "" {
			out[pkg] = struct{}{}
		}
	}
	return out
}

func storedGoPackage(db *store.LocalStore, root, abs string) string {
	if db == nil {
		return ""
	}
	inputs, _, err := db.LoadWorldFactsForFile(canonicalScanPath(root, abs), "fast")
	if err != nil {
		return ""
	}
	for _, in := range inputs {
		if in.Predicate != "file_package" || len(in.Args) < 2 {
			continue
		}
		if name, ok := in.Args[1].(string); ok && name != "" {
			return name
		}
	}
	return ""
}

// evidenceFiles is the pre-existing files of a package: current files that
// are not new, plus files just deleted. A new file has no stored
// code_defines yet; the old declaration set is the files that were already
// mapped.
func evidenceFiles(current, deleted []string, added map[string]struct{}) []string {
	out := make([]string, 0, len(current)+len(deleted))
	for _, p := range current {
		if _, ok := added[p]; ok {
			continue
		}
		out = append(out, p)
	}
	out = append(out, deleted...)
	return out
}

func notIn(paths []string, skip map[string]struct{}) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if _, ok := skip[p]; ok {
			continue
		}
		out = append(out, p)
	}
	return out
}

// packageDeclsChanged reports whether the on-disk declaration set differs
// from the set this package was last mapped with. had-cache is the
// in-process compare (peekPkgSymbols vs loadSymbols). Without a cache, the
// stored code_defines rows are the previous set, and only when every
// evidence file has a deep row. Returning false when that evidence is
// missing is deliberate: a body edit must not fan out across the package
// just because the previous set is unknown.
func packageDeclsChanged(dir, pkg string, includeTests bool, db *store.LocalStore, root string, evidence []string) bool {
	before, had := peekPkgSymbols(dir, pkg, includeTests)
	after, ok := loadSymbols(dir, pkg, includeTests)
	if !ok {
		return false
	}
	if had {
		return !before.sameDeclarations(after)
	}
	keys, complete := storedDeclKeys(db, root, evidence)
	if !complete {
		return false
	}
	return !declKeyEqual(keys, after.declKeys(pkg))
}

func storedDeclKeys(db *store.LocalStore, root string, files []string) (map[string]struct{}, bool) {
	if db == nil {
		return nil, false
	}
	keys := map[string]struct{}{}
	if len(files) == 0 {
		return keys, true
	}
	for _, abs := range files {
		inputs, _, err := db.LoadWorldFactsForFile(canonicalScanPath(root, abs), "deep")
		if err != nil || len(inputs) == 0 {
			return nil, false
		}
		for _, in := range inputs {
			k, ok := declKeyFromDefine(in.Predicate, in.Args)
			if ok {
				keys[k] = struct{}{}
			}
		}
	}
	return keys, true
}

func declKeyFromDefine(pred string, args []any) (string, bool) {
	if pred != "code_defines" || len(args) < 3 {
		return "", false
	}
	id := types.ExtractString(args[1])
	kind := strings.TrimPrefix(types.ExtractString(args[2]), "/")
	if id == "" || kind == "" {
		return "", false
	}
	switch kind {
	case "function":
		// pkg.Name is a function; pkg.Recv.Name is a method. Package clauses
		// do not contain dots, so the second dot is the receiver.
		if strings.Count(id, ".") >= 2 {
			return "method:" + id, true
		}
		return "func:" + id, true
	case "struct", "interface", "type":
		return "type:" + id, true
	default:
		return "", false
	}
}

func declKeyEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// refreshSiblingCallRows remaps each sibling through the cartographer and
// replaces its deep rows. The previous rows are returned for retraction and
// the fresh map for assertion; both are the whole per-file deep set, because
// that is the unit EnsureDeepFacts replaces.
func refreshSiblingCallRows(ctx context.Context, root string, db *store.LocalStore, current map[string]os.FileInfo, siblings []string) (fresh, old []core.Fact) {
	c := NewCartographer()
	defer c.Close()
	for _, p := range siblings {
		if err := ctx.Err(); err != nil {
			return fresh, old
		}
		info := current[p]
		if info == nil {
			continue
		}
		canonical := canonicalScanPath(root, p)
		var prior []core.Fact
		if db != nil {
			inputs, _, err := db.LoadWorldFactsForFile(canonical, "deep")
			if err != nil {
				logging.WorldWarn("sibling call rows: load %s: %v", canonical, err)
				continue
			}
			prior = make([]core.Fact, len(inputs))
			for i, in := range inputs {
				prior[i] = core.Fact{Predicate: in.Predicate, Args: in.Args}
			}
		}
		mapped, err := c.MapFileAs(p, canonical)
		if err != nil {
			logging.WorldWarn("sibling call rows: map %s: %v", canonical, err)
			continue
		}
		if db != nil {
			inputs := make([]store.WorldFactInput, len(mapped))
			for i, f := range mapped {
				inputs[i] = store.WorldFactInput{Predicate: f.Predicate, Args: f.Args}
			}
			sum, sumErr := calculateHash(p)
			if sumErr != nil {
				logging.WorldWarn("sibling call rows: hash %s: %v", canonical, sumErr)
				sum = ""
			}
			fp := formatContentFingerprint(stampFromInfo(p, info, nil), sum)
			if err := db.ReplaceWorldFactsForFile(canonical, "deep", fp, inputs); err != nil {
				logging.WorldWarn("sibling call rows: store %s: %v", canonical, err)
				continue
			}
		}
		old = append(old, prior...)
		fresh = append(fresh, mapped...)
	}
	return fresh, old
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

	// Simple majority wins, and ties are broken by name rather than by luck.
	//
	// Ranging a map with a strict `>` picks whichever key Go's randomised
	// iteration reaches first, so a workspace with equal Go and Python files
	// reported a different primary language on different runs — and the
	// primary language decides which build and test commands the agent
	// reaches for. The tie-break being alphabetical is arbitrary; its being
	// FIXED is not. A stable wrong answer can be found and argued with, a
	// varying one cannot.
	langs := make([]string, 0, len(counts))
	for lang := range counts {
		langs = append(langs, lang)
	}
	sort.Strings(langs)

	bestLang := ""
	maxCount := 0
	for _, lang := range langs {
		if counts[lang] > maxCount {
			maxCount = counts[lang]
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
