package prompt

import (
	"context"
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/store"
)

// Source generations: the non-kernel half of a compiled prompt's identity.
//
// A compile reads atoms from the project and selected-expert databases,
// knowledge from the project LocalStore and the expert database, learnings
// from the LearningStore, the evolved set and the strategy store. None of that
// is in the kernel snapshot fingerprint. Before the source vector existed the
// cache key carried only context, kernel fingerprint and configuration, so an
// in-place source write (a sync, a learning, a refined strategy) was invisible
// to every later identical call until LRU eviction, and a compile that loaded
// the old corpus could publish after a registration cleared the cache.
//
// Every source is now named by an owned generation (see
// store.ReadSourceGeneration for the SQLite ones), captured before the compile
// reads it and re-captured after. The cache key carries the first capture; the
// result is published only if the second matches, so old work can neither
// populate nor satisfy a newer generation. The embedded corpus and the vector
// searcher are fixed for a compiler's lifetime (they are construction
// options), so they need no entry.

// promptAtomSourceTables are what loadAtomsFromDB reads; expert databases
// additionally hold the knowledge_atoms the expert lookup searches.
var (
	promptAtomSourceTables = []string{"prompt_atoms", "atom_context_tags"}
	expertSourceTables     = []string{"prompt_atoms", "atom_context_tags", "knowledge_atoms"}
)

// SourceGenerationReporter is implemented by a pluggable source whose content
// can change without a registration: the strategy provider today. The token
// must change whenever anything the source would return changes.
type SourceGenerationReporter interface {
	SourceGeneration(ctx context.Context) (string, error)
}

// sourceVector is one capture of every non-kernel source generation.
type sourceVector struct {
	parts map[string]string
	// untracked names a source whose generation could not be read. Such a
	// compile neither consults nor populates the cache: an unreadable
	// generation is not evidence that nothing changed.
	untracked []string
}

func (v sourceVector) token() string {
	keys := make([]string, 0, len(v.parts))
	for k := range v.parts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(strconv.Itoa(len(v.parts[k])))
		b.WriteByte(':')
		b.WriteString(v.parts[k])
		b.WriteByte(';')
	}
	return b.String()
}

func (v sourceVector) tracked() bool { return len(v.untracked) == 0 }

func (v sourceVector) equal(other sourceVector) bool {
	return v.tracked() && other.tracked() && v.token() == other.token()
}

// compileSources is the set of source handles one compile reads, copied once
// under the compiler's locks so the capture, the re-capture and the reads all
// address the same handles even if a registration lands mid-compile.
type compileSources struct {
	epoch          uint64
	embedded       *EmbeddedCorpus
	projectDB      *sql.DB
	shardKey       string
	shardDB        *sql.DB
	experts        []expertSource
	localDB        *store.LocalStore
	learning       *store.LearningStore
	evolved        *EvolvedAtomManager
	strategy       StrategyProvider
	vectorSearcher VectorSearcher
}

// consultedExperts are the shard agent KBs a compile over these sources
// searches for knowledge: the speaker's own and every pushed expert's.
func (src compileSources) consultedExperts() []string {
	var out []string
	if src.shardDB != nil && src.shardKey != "" {
		out = append(out, src.shardKey)
	}
	for _, expert := range src.experts {
		out = append(out, expert.key)
	}
	return out
}

func (c *JITPromptCompiler) snapshotSources(cc *CompilationContext) compileSources {
	c.dbMu.RLock()
	src := compileSources{
		epoch:          c.sourceEpoch.Load(),
		embedded:       c.embeddedCorpus,
		projectDB:      c.projectDB,
		localDB:        c.localDB,
		learning:       c.learningStore,
		evolved:        c.evolvedAtomMgr,
		strategy:       c.strategyProvider,
		vectorSearcher: c.vectorSearcher,
	}
	c.dbMu.RUnlock()
	if cc != nil && cc.ShardID != "" {
		src.shardKey = shardDBKey(cc.ShardID)
		c.shardMu.RLock()
		src.shardDB = c.shardDBs[src.shardKey]
		c.shardMu.RUnlock()
	}
	src.experts = c.expertSources(cc, src.shardKey)
	return src
}

// captureSourceVector reads every source generation. It runs on every compile,
// hit or miss, so each read is a single small query.
func (src compileSources) captureSourceVector(ctx context.Context) sourceVector {
	v := sourceVector{parts: map[string]string{
		"registration": strconv.FormatUint(src.epoch, 10),
	}}
	record := func(name string, token string, err error) {
		if err != nil {
			v.untracked = append(v.untracked, name)
			logging.Get(logging.CategoryJIT).Debug("Source %s has no readable generation; compile is uncacheable: %v", name, err)
			return
		}
		v.parts[name] = token
	}
	if src.projectDB != nil {
		token, err := store.ReadSourceGeneration(ctx, src.projectDB, promptAtomSourceTables...)
		record("project", token, err)
	}
	if src.shardDB != nil {
		token, err := store.ReadSourceGeneration(ctx, src.shardDB, expertSourceTables...)
		record("expert:"+src.shardKey, token, err)
	}
	for _, expert := range src.experts {
		token, err := store.ReadSourceGeneration(ctx, expert.db, expertSourceTables...)
		record(expertSourceName(expert.key), token, err)
	}
	if src.localDB != nil {
		token, err := src.localDB.KnowledgeSourceGeneration(ctx)
		record("knowledge", token, err)
	}
	if src.learning != nil {
		token, err := src.learning.SourceGeneration(ctx)
		record("learning", token, err)
	}
	if src.evolved != nil {
		v.parts["evolved"] = strconv.FormatUint(src.evolved.Generation(), 10)
	}
	if src.strategy != nil {
		token, err := src.strategy.SourceGeneration(ctx)
		record("strategy", token, err)
	}
	return v
}

// degradation records an optional source that failed during collection. A
// degraded prompt is still returned (a prompt without one knowledge lookup is
// a working prompt) but it is never cached: caching it would keep serving the
// outage's prompt after the dependency recovers, for as long as the context
// stays the same. Each later identical call re-probes instead.
type degradation struct {
	mu      sync.Mutex
	reasons []string
}

func (d *degradation) add(reason string) {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.reasons = append(d.reasons, reason)
	d.mu.Unlock()
}

func (d *degradation) list() []string {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	out := append([]string(nil), d.reasons...)
	sort.Strings(out)
	return out
}

// atomCatalog keeps the typed atoms parsed from one source database, keyed by
// the handle and the generation they were read at. A compile whose captured
// generation matches reuses them instead of re-running the 10,000-row tag
// join; any write moves the generation and the next miss reloads.
//
// Catalog atoms are shared, immutable values, exactly as embedded-corpus atoms
// already are: nothing in selection, resolution, fitting or assembly writes
// to a candidate atom.
type atomCatalog struct {
	mu      sync.Mutex
	entries map[string]atomCatalogEntry
}

type atomCatalogEntry struct {
	db    *sql.DB
	token string
	atoms []*PromptAtom
}

func (cat *atomCatalog) lookup(source string, db *sql.DB, token string) ([]*PromptAtom, bool) {
	if token == "" {
		return nil, false
	}
	cat.mu.Lock()
	defer cat.mu.Unlock()
	entry, ok := cat.entries[source]
	if !ok || entry.db != db || entry.token != token {
		return nil, false
	}
	return entry.atoms, true
}

func (cat *atomCatalog) install(source string, entry atomCatalogEntry) {
	cat.mu.Lock()
	defer cat.mu.Unlock()
	if cat.entries == nil {
		cat.entries = make(map[string]atomCatalogEntry)
	}
	cat.entries[source] = entry
}

// forgetHandle drops every catalog built from db (it is being closed).
func (cat *atomCatalog) forgetHandle(db *sql.DB) {
	if db == nil {
		return
	}
	cat.mu.Lock()
	defer cat.mu.Unlock()
	for source, entry := range cat.entries {
		if entry.db == db {
			delete(cat.entries, source)
		}
	}
}

// pendingCatalogs are loads made during one compile, installed only after the
// re-capture proves the generation they were read under is still current.
type pendingCatalogs struct {
	mu      sync.Mutex
	entries map[string]atomCatalogEntry
}

func (p *pendingCatalogs) add(source string, entry atomCatalogEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.entries == nil {
		p.entries = make(map[string]atomCatalogEntry)
	}
	p.entries[source] = entry
}

// loadCatalogAtoms returns the atoms of one source database, from the catalog
// when its generation is current.
func (c *JITPromptCompiler) loadCatalogAtoms(ctx context.Context, source string, db *sql.DB, token string, pending *pendingCatalogs) ([]*PromptAtom, error) {
	if atoms, ok := c.catalog.lookup(source, db, token); ok {
		c.counters.catalogReuses.Add(1)
		return atoms, nil
	}
	atoms, err := c.loadAtomsFromDB(ctx, db)
	if err != nil {
		return nil, err
	}
	if token != "" && pending != nil {
		pending.add(source, atomCatalogEntry{db: db, token: token, atoms: atoms})
	}
	return atoms, nil
}

// bumpSourceEpoch records a registration: a source handle was added, replaced
// or removed. It is part of every source vector, so a replaced handle can
// never alias the old handle's generation values.
func (c *JITPromptCompiler) bumpSourceEpoch() {
	c.sourceEpoch.Add(1)
}
