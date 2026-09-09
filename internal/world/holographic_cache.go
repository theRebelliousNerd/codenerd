package world

// =============================================================================
// PACKAGE-PARSE MEMOISATION
// =============================================================================
// HolographicProvider.PromptSection is called from Executor.withFileContext
// (internal/session/executor_tools.go) once per LLM turn that names a file
// target. Before this cache existed, every one of those turns re-read the
// target's directory and re-parsed up to 100 sibling files with
// parser.ParseComments to produce ~1.6 KB of prompt text that is byte-identical
// until a file in the package changes.
//
// Measured on internal/core (150+ files) before the cache:
//
//	BenchmarkHolographicPromptSection-4  5  60960352 ns/op  12282140 B/op  293557 allocs/op
//
// 61 ms and 12 MB of garbage per turn, repeated for every turn in a session
// that stays in one package — which is what a coding session is.
//
// The cached value is deliberately only the *filesystem-derived* half of a
// HolographicContext. Anything the kernel answers (call graph, impact
// priorities) is recomputed every call, because kernel state changes without
// any file changing and a stale answer there would be a correctness bug rather
// than a slow path.

import (
	"container/list"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// maxCachedPackages bounds the cache to 64 directories. A coding session moves
// between a handful of packages; 64 covers that with room for a campaign that
// sweeps a subsystem, while capping worst-case retention at roughly the parse
// of 64 packages rather than of the whole workspace.
const maxCachedPackages = 64

// packageParse is everything buildGoContext can learn from a directory's bytes.
// It is a pure function of those bytes, which is what makes it cacheable.
type packageParse struct {
	// goFiles is every non-test .go file in the directory, in read order,
	// after the maxPackageFilesToParse cap.
	goFiles []string
	// allGoFiles is the uncapped list, used for the sibling roster so the
	// model is told the real package size even when parsing was truncated.
	allGoFiles []string
	signatures []SymbolSignature
	types      []TypeDefinition
	constants  []ConstDefinition
	imports    map[string][]string
	// pkgName maps a file's base name to its package clause. Held per file
	// because a directory can legally hold `foo` and `foo_test`.
	pkgName map[string]string

	// localRefs maps a file's base name to the package-level symbols it
	// references but does not itself define.
	//
	// This is what makes the prompt's signature list relevant rather than
	// alphabetical. When the target file defines little or nothing of its own —
	// a package-marker file whose implementation lives in siblings, a main.go,
	// a thin wrapper — the eight signature slots were filled from whichever
	// sibling sorted first in directory order. On internal/core/kernel.go that
	// meant eight symbols from action_validator.go and a "… and 592 more":
	// tokens spent, signal zero.
	//
	// Filtered to package-level names before storage, so its size is bounded by
	// the package's own symbol count rather than by every identifier in every
	// file.
	localRefs map[string]map[string]struct{}

	// refCount maps a package-level symbol to how many other files in the
	// package reference it. It is a cheap in-package centrality measure, and it
	// is what stops the last resort from being alphabetical: for a file that
	// defines nothing and references nothing — a package-marker file — the most
	// useful eight symbols are the ones the rest of the package leans on.
	refCount map[string]int
}

// packageCacheEntry pairs a parse with the directory fingerprint it was taken
// from.
type packageCacheEntry struct {
	fingerprint string
	parse       *packageParse
}

// packageParseCache is a bounded LRU over directory parses.
//
// It deep-copies on the way in and on the way out. internal/diff learned this
// the hard way: a cache that hands a caller its own memory turns any downstream
// append into silent corruption of every later reader, and the failure surfaces
// far from the mutation.
type packageParseCache struct {
	mu      sync.Mutex
	entries map[string]*list.Element // dir -> element holding *packageCacheItem
	order   *list.List               // front = most recently used
	hits    int64
	misses  int64
}

type packageCacheItem struct {
	dir   string
	entry *packageCacheEntry
}

func newPackageParseCache() *packageParseCache {
	return &packageParseCache{
		entries: make(map[string]*list.Element, maxCachedPackages),
		order:   list.New(),
	}
}

// get returns a private copy of the parse for dir when the cached fingerprint
// still matches, and reports whether it hit.
func (c *packageParseCache) get(dir, fingerprint string) (*packageParse, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[dir]
	if !ok {
		c.misses++
		return nil, false
	}
	item, _ := el.Value.(*packageCacheItem)
	if item == nil || item.entry == nil || item.entry.fingerprint != fingerprint {
		// Stale: a file changed. Drop it rather than keep a wrong answer.
		c.order.Remove(el)
		delete(c.entries, dir)
		c.misses++
		return nil, false
	}
	c.order.MoveToFront(el)
	c.hits++
	return item.entry.parse.clone(), true
}

// put stores a private copy of parse under dir/fingerprint, evicting the least
// recently used directory when the cache is full.
func (c *packageParseCache) put(dir, fingerprint string, parse *packageParse) {
	if c == nil || parse == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[dir]; ok {
		item, _ := el.Value.(*packageCacheItem)
		if item != nil {
			item.entry = &packageCacheEntry{fingerprint: fingerprint, parse: parse.clone()}
		}
		c.order.MoveToFront(el)
		return
	}
	for c.order.Len() >= maxCachedPackages {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		c.order.Remove(oldest)
		if item, _ := oldest.Value.(*packageCacheItem); item != nil {
			delete(c.entries, item.dir)
		}
	}
	el := c.order.PushFront(&packageCacheItem{
		dir:   dir,
		entry: &packageCacheEntry{fingerprint: fingerprint, parse: parse.clone()},
	})
	c.entries[dir] = el
}

// stats reports hit/miss counts. Used by tests and by the world cache metrics.
func (c *packageParseCache) stats() (hits, misses int64) {
	if c == nil {
		return 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.misses
}

// clone returns a deep copy. The nested Fields/Methods slices are copied too:
// a shallow copy would share them, and TypeDefinition.Fields is exactly the
// kind of slice a renderer is tempted to append to.
func (p *packageParse) clone() *packageParse {
	if p == nil {
		return nil
	}
	out := &packageParse{
		goFiles:    append([]string(nil), p.goFiles...),
		allGoFiles: append([]string(nil), p.allGoFiles...),
		signatures: append([]SymbolSignature(nil), p.signatures...),
		constants:  append([]ConstDefinition(nil), p.constants...),
		imports:    make(map[string][]string, len(p.imports)),
		pkgName:    make(map[string]string, len(p.pkgName)),
	}
	out.types = make([]TypeDefinition, len(p.types))
	for i, td := range p.types {
		td.Fields = append([]string(nil), td.Fields...)
		td.Methods = append([]string(nil), td.Methods...)
		out.types[i] = td
	}
	for k, v := range p.imports {
		out.imports[k] = append([]string(nil), v...)
	}
	for k, v := range p.pkgName {
		out.pkgName[k] = v
	}
	// localRefs and refCount are shared, not copied. Both are written once by
	// narrowLocalRefs at the end of parsePackage and only ever read afterwards
	// — applyTo reads them, nothing appends to them — so a per-hit deep copy
	// buys no safety and costs real time: copying internal/core's ~600-entry
	// refCount and its per-file reference sets on every cache hit tripled the
	// cost of the hit, which is the whole thing the cache exists to make cheap.
	// TestPackageParse_FrozenMapsAreNotMutated pins the invariant.
	out.localRefs = p.localRefs
	out.refCount = p.refCount
	return out
}

// referencedBy returns the package-level symbols the given file uses but does
// not define.
func (p *packageParse) referencedBy(fileBase string) map[string]struct{} {
	if p == nil || p.localRefs == nil {
		return nil
	}
	return p.localRefs[fileBase]
}

// directoryFingerprint identifies a directory's Go source state by the
// (name, size, mtime) of every non-test .go file in it.
//
// Content hashing would be exact but costs a full read of the package on every
// turn, which is the cost the cache exists to remove. stat is ~1 µs per file
// against ~400 µs to parse one, so the fingerprint is ~0.5% of the work it
// avoids. The tradeoff it accepts is an edit that preserves both size and mtime
// — which on any real editor means an mtime change, and which the world
// model's own file-hash facts catch independently.
//
// It returns the entries it fingerprinted so the caller does not stat twice.
func directoryFingerprint(dir string) (string, []os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, err
	}
	var b strings.Builder
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b.WriteString(name)
		b.WriteByte('|')
		if info, statErr := entry.Info(); statErr == nil {
			b.WriteString(strconv.FormatInt(info.Size(), 10))
			b.WriteByte('|')
			b.WriteString(strconv.FormatInt(info.ModTime().UnixNano(), 10))
		} else {
			// A file that vanished between ReadDir and Info still has to
			// change the fingerprint, or the next call reuses a parse that
			// includes it.
			b.WriteString("stat_error")
		}
		b.WriteByte('\n')
	}
	return b.String(), entries, nil
}

// applyTo copies the cached parse into a HolographicContext for one target
// file. Siblings exclude the target; the package clause is read from the parse
// when the target is one of the parsed files.
func (p *packageParse) applyTo(hc *HolographicContext, targetFile string) {
	if p == nil || hc == nil {
		return
	}
	hc.PackageSiblings = hc.PackageSiblings[:0]
	for _, f := range p.allGoFiles {
		if f != targetFile {
			hc.PackageSiblings = append(hc.PackageSiblings, f)
		}
	}
	hc.PackageSignatures = p.signatures
	hc.PackageTypes = p.types
	hc.PackageConstants = p.constants
	if hc.PackageImports == nil {
		hc.PackageImports = make(map[string][]string, len(p.imports))
	}
	for k, v := range p.imports {
		hc.PackageImports[k] = v
	}
	base := filepath.Base(targetFile)
	if name, ok := p.pkgName[base]; ok && name != "" {
		hc.TargetPkg = name
	}
	if refs := p.referencedBy(base); len(refs) > 0 {
		hc.ReferencedSymbols = make([]string, 0, len(refs))
		for name := range refs {
			hc.ReferencedSymbols = append(hc.ReferencedSymbols, name)
		}
		// Sorted so the rendered section is byte-stable across turns; map
		// iteration order is randomised, and a section that reshuffles for no
		// reason throws away the model provider's prompt-cache hit.
		sort.Strings(hc.ReferencedSymbols)
	}
	// Shared for the same reason: frozen after the parse, read-only from here.
	if len(p.refCount) > 0 {
		hc.SymbolRefCount = p.refCount
	}
}
