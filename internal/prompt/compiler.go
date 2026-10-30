package prompt

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	userconfig "codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// Fact represents a logic fact (predicate + args).
type Fact struct {
	Predicate string
	Args      []any
}

// String renders the fact as Mangle source text (no trailing period):
// strings quoted, types.MangleAtom name constants bare, numbers in decimal.
// It is the text a kernel parser turns back into this exact fact.
func (f Fact) String() string {
	var fb factBuilder
	fb.WriteString(f.Predicate)
	fb.WriteByte('(')
	for i, arg := range f.Args {
		if i > 0 {
			fb.WriteString(", ")
		}
		switch v := arg.(type) {
		case types.MangleAtom:
			fb.WriteString(string(v))
		case string:
			fb.WriteQuotedString(v)
		case int:
			fb.WriteInt(v)
		case int64:
			fb.WriteString(strconv.FormatInt(v, 10))
		case float64:
			fb.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
		default:
			fb.WriteQuotedString(fmt.Sprint(v))
		}
	}
	fb.WriteByte(')')
	return fb.String()
}

// KernelQuerier defines the interface for querying the Mangle kernel.
// This abstracts the kernel to avoid circular imports.
type KernelQuerier interface {
	// Query retrieves facts matching a predicate.
	Query(predicate string) ([]Fact, error)

	// AssertBatch adds multiple facts efficiently.
	AssertBatch(facts []any) error
}

// KernelRetracter is an optional extension of KernelQuerier for predicate retraction.
// Implementations that support it (e.g., RealKernel) enable cleanup of ephemeral facts.
type KernelRetracter interface {
	Retract(predicate string) error
}

// KernelCompilationScope is an isolated kernel view used for one prompt
// compilation. Closing the scope must discard every fact asserted through it.
type KernelCompilationScope interface {
	KernelQuerier
	Close() error
}

// KernelScopeProvider creates per-compilation kernel views. Production kernel
// adapters implement this using a snapshot/clone so parallel prompt compiles
// never share selector facts.
type KernelScopeProvider interface {
	NewCompilationScope() (KernelCompilationScope, error)
}

// KernelSnapshotIdentity names one state of the live kernel: which kernel,
// which generation of its facts and program, and whether equal identities
// imply equal derived state (false when rules read external predicates).
type KernelSnapshotIdentity struct {
	Instance   uint64
	Generation uint64
	Reusable   bool
}

// KernelSnapshotIdentifier is implemented by a kernel adapter that can name
// its current state without cloning it. The compiler uses it to reuse a
// dynamic-snapshot preflight while the kernel has not changed.
type KernelSnapshotIdentifier interface {
	SnapshotIdentity() (KernelSnapshotIdentity, bool)
}

// KernelScopeIdentity is implemented by a compilation scope that knows the
// identity of the state it was cloned from, read atomically with the clone.
type KernelScopeIdentity interface {
	BaseIdentity() (KernelSnapshotIdentity, bool)
}

// KernelContentFingerprinter names the complete evaluated content of an owned
// compilation scope -- every fact, including inputs to selection outside the
// injection rows -- without listing it. Equal fingerprints answer every query
// alike (core.RealKernel.ContentFingerprint).
type KernelContentFingerprinter interface {
	ContentFingerprint() (string, error)
}

var promptEphemeralPredicates = []string{
	"compile_context",
	"current_context",
	"atom",
	"prompt_atom",
	"atom_category",
	"atom_importance",
	"atom_tag",
	"is_mandatory",
	"vector_hit",
	"vector_slot",
	"atom_relevance",
	"atom_relevance_threshold",
	"flesh_carried",
	"atom_requires",
	"atom_conflicts",
	"atom_requires_tool",
	"available_tool",
	"compile_shard",
	"retrieved_context",
}

// VectorSearcher defines the interface for semantic search.
type VectorSearcher interface {
	// Search performs semantic search and returns atom IDs with scores.
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)

	// EmbedQuery embeds the query with its producing engine's identity, so
	// a semantic store can rank it only against rows of the same space.
	EmbedQuery(ctx context.Context, query string) (store.QueryEmbedding, error)
}

// SearchResult represents a semantic search result.
type SearchResult struct {
	AtomID string
	Score  float64
}

// JITPromptCompiler compiles optimal system prompts from atomic fragments.
// It combines rule-based selection (Mangle) with semantic search (vectors)
// to select the most relevant prompt atoms for a given context.
type JITPromptCompiler struct {
	// Embedded corpus (baked-in atoms)
	embeddedCorpus *EmbeddedCorpus

	// workspace is the project root whose .nerd/agents.json is the specialist
	// roster a compile advertises ({{available_specialists}}). Bound once by
	// the boot owner (WithWorkspace); a compiler with none advertises only
	// the built-in shards. It is never guessed from the process directory.
	workspace string

	// Project-level corpus database (.nerd/prompts/corpus.db)
	projectDB *sql.DB

	// Shard-specific atom databases (keyed by shard ID)
	shardDBs map[string]*sql.DB

	// Kernel for rule-based selection
	kernel KernelQuerier

	// Vector store for semantic search
	vectorSearcher VectorSearcher

	// Sub-components
	selector  *AtomSelector
	resolver  *DependencyResolver
	budgetMgr *TokenBudgetManager
	assembler *FinalAssembler

	// cache holds compiled results by promptCacheKey (context, kernel
	// fingerprint, configuration revision, source vector).
	cache *lruCache[string, *CompilationResult]
	// preflight memoizes the dynamic kernel snapshot by kernel identity and
	// context, so a repeated context over an unchanged kernel skips the clone
	// and the full-snapshot walk (see compiler_preflight.go).
	preflight *lruCache[preflightKey, preflightEntry]
	// catalog holds typed atoms per source database and generation.
	catalog atomCatalog
	// sourceEpoch advances on every source registration.
	sourceEpoch atomic.Uint64
	// counters are the hit/miss/joiner/preflight/uncacheable metrics.
	counters compilerCounters

	// Observability
	lastResult atomic.Pointer[CompilationResult]

	// Concurrency control
	configMu       sync.RWMutex
	dbMu           sync.RWMutex
	shardMu        sync.RWMutex
	compileGroup   singleflight.Group
	wg             sync.WaitGroup
	lifecycleMu    sync.Mutex
	closed         bool
	stopContext    context.Context
	stop           context.CancelFunc
	closeOnce      sync.Once
	closeErr       error
	configRevision uint64 // guarded by configMu; changes whenever SetConfig replaces compile behavior

	// Configuration
	config CompilerConfig

	// LocalDB for semantic knowledge atom queries (Semantic Knowledge Bridge)
	localDB *store.LocalStore

	// LearningStore for recalling learned intents and feedback (Semantic Knowledge Bridge)
	learningStore *store.LearningStore

	// Evolved atoms manager for System Prompt Learning (SPL)
	evolvedAtomMgr *EvolvedAtomManager

	// strategyProvider is the learned-strategy source (see strategy_atoms.go).
	// Nil until registered, which is the state for every compiler built
	// without a prompt evolver behind it.
	strategyProvider StrategyProvider
}

// CompilerConfig holds configuration for the JIT compiler.
type CompilerConfig struct {
	// DefaultTokenBudget is the default token budget if not specified in context
	DefaultTokenBudget int

	// VectorSearchWeight is the weight of vector scores vs logic scores (0.0-1.0)
	VectorSearchWeight float64

	// VectorSearchTimeout bounds one vector search: atom selection's and
	// learning recall's. It comes from jit.vector_search_timeout_ms; zero is
	// "unset" and keeps the selector's current bound.
	VectorSearchTimeout time.Duration

	// DebugMode enables verbose JIT manifest logging
	DebugMode bool

	// KnowledgeSearchTimeout is the max time to wait for knowledge atom embedding and search
	KnowledgeSearchTimeout time.Duration

	// KernelContextRows caps injectable_context rows merged into the single
	// kernel-context atom. A non-positive value means unset and resolves to
	// the configured jit.kernel_context_rows at use.
	KernelContextRows int

	// KernelContextRowChars caps one injectable_context row. A non-positive
	// value resolves to jit.kernel_context_row_chars.
	KernelContextRowChars int

	// KernelInjectedAtomChars is the ceiling on either merged kernel-injected
	// atom. A non-positive value resolves to jit.kernel_injected_atom_chars.
	KernelInjectedAtomChars int

	// SpecialistKnowledgeBlocks caps specialist_knowledge topics merged into
	// one atom. A non-positive value resolves to jit.specialist_knowledge_blocks.
	SpecialistKnowledgeBlocks int

	// SpecialistTopicChars caps a specialist_knowledge topic heading. A
	// non-positive value resolves to jit.specialist_topic_chars.
	SpecialistTopicChars int

	// SpecialistBlockChars caps one specialist_knowledge body. A non-positive
	// value resolves to jit.specialist_block_chars.
	SpecialistBlockChars int

	// FOUR FIELDS WERE REMOVED HERE on 2026-09-11, all set by
	// DefaultCompilerConfig and read by nothing in the repository -- not
	// production, not tests, except tests asserting the literal they had just
	// been handed. Three of them described behaviour that does not exist,
	// which is worse than an absent knob: a reader reasoning about this
	// compiler believed them.
	//
	//   EnableVectorSearch   the selector has no such switch. It runs the
	//                        search when a vectorSearcher is installed and
	//                        skips it when one is not, and the factory decides
	//                        that by whether an embedding engine exists.
	//   MaxAtomsPerCategory  no per-category cap exists anywhere. Selection is
	//                        bounded by the token budget.
	//   CacheTTLSeconds      the prompt LRU has NO TTL. Entries live until size
	//                        eviction. The "// 5 minutes" beside it was
	//                        especially expensive, because this branch spent
	//                        real effort on Anthropic's five-minute prompt-cache
	//                        TTL and a second inert five-minute TTL in the same
	//                        subsystem is a trap laid for exactly that reader.
	//   EnableCaching        the LRU is unconditional; nothing checks a flag.
	//
	// Deleted rather than wired, which is main's 938cd87 judgement on
	// shard_profiles.max_output_tokens applied again: each of the three would
	// have meant BUILDING the behaviour it claims, and that is a feature
	// decision. VectorSearchWeight is the one of the five that was a genuine
	// missing wire and it is wired above.
}

// defaultVectorSearchTimeout is jit.vector_search_timeout_ms's default, the
// one source of the vector-search bound for compilers built without config.
var defaultVectorSearchTimeout = time.Duration(userconfig.DefaultJITConfig().VectorSearchTimeoutMs) * time.Millisecond

// DefaultCompilerConfig returns a sensible default configuration.
// DefaultTokenBudget is a fallback: production boot (internal/system) overrides
// it from the configured JIT token budget through WithConfig. The injection
// bounds and the vector-search timeout are sourced from DefaultJITConfig, their
// single definition; a user-set jit.* value takes effect once boot maps it onto
// this struct (internal/system/factory.go initIntelligenceLayer).
func DefaultCompilerConfig() CompilerConfig {
	jitDefaults := userconfig.DefaultJITConfig()
	return CompilerConfig{
		DefaultTokenBudget: 200000, // 200k tokens default - callers should override from config
		// Mirrors AtomSelector's own default. The selector keeps one too, for
		// direct users that never see a CompilerConfig; this is the value the
		// compiler pushes into it.
		VectorSearchWeight:        0.3, // 70% logic, 30% vector
		DebugMode:                 false,
		KnowledgeSearchTimeout:    10 * time.Second,
		VectorSearchTimeout:       defaultVectorSearchTimeout,
		KernelContextRows:         jitDefaults.KernelContextRows,
		KernelContextRowChars:     jitDefaults.KernelContextRowChars,
		KernelInjectedAtomChars:   jitDefaults.KernelInjectedAtomChars,
		SpecialistKnowledgeBlocks: jitDefaults.SpecialistKnowledgeBlocks,
		SpecialistTopicChars:      jitDefaults.SpecialistTopicChars,
		SpecialistBlockChars:      jitDefaults.SpecialistBlockChars,
	}
}

// NewJITPromptCompiler creates a new JIT prompt compiler.
func NewJITPromptCompiler(opts ...CompilerOption) (*JITPromptCompiler, error) {
	config := DefaultCompilerConfig()

	compiler := &JITPromptCompiler{
		shardDBs:  make(map[string]*sql.DB),
		config:    config,
		selector:  NewAtomSelector(),
		resolver:  NewDependencyResolver(),
		budgetMgr: NewTokenBudgetManager(),
		assembler: NewFinalAssembler(),
		cache:     newLRUCache[string, *CompilationResult](promptCacheLimit),
		preflight: newLRUCache[preflightKey, preflightEntry](promptCacheLimit),
	}
	compiler.stopContext, compiler.stop = context.WithCancel(context.Background())

	// Default the canonical built-in atoms. The embedded corpus is a
	// compile-time constant and production treats it as mandatory (the system
	// factory fails startup without it), so a bare compiler must not silently
	// compile empty prompts. An explicit WithEmbeddedCorpus option — including
	// nil for isolation — still overrides this default.
	if corpus, err := getEmbeddedCorpusCached(); err != nil {
		compiler.stop()
		return nil, fmt.Errorf("failed to load embedded prompt corpus: %w", err)
	} else {
		compiler.embeddedCorpus = corpus
	}

	// Apply options
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(compiler); err != nil {
			return nil, fmt.Errorf("failed to apply compiler option: %w", err)
		}
	}

	// Ensure selector has the timeout from config
	compiler.selector.SetVectorSearchTimeout(compiler.config.VectorSearchTimeout)
	// The sibling knob, and it was wired nowhere. SetVectorWeight had no
	// production caller and CompilerConfig.VectorSearchWeight had no
	// reader: two halves of one missing wire, which is why the selector
	// and the config each carried their own 0.3 with the same
	// "70% logic, 30% vector" comment attached.
	//
	// The zero is guarded because the two setters do NOT agree about what
	// one means. SetVectorSearchTimeout reads zero as "unset" and
	// keeps its current bound; SetVectorWeight clamps to [0,1] and takes a
	// zero literally, as pure logic. So wiring this unguarded would make a
	// partially-filled CompilerConfig silently turn vector scoring off --
	// the exact silent-failure shape this is being fixed to remove. Pure
	// logic is expressed by installing no vector searcher at all
	// (selector.go skips the search when vectorSearcher is nil).
	if compiler.config.VectorSearchWeight > 0 {
		compiler.selector.SetVectorWeight(compiler.config.VectorSearchWeight)
	}

	logging.Get(logging.CategoryContext).Info("JITPromptCompiler initialized")
	return compiler, nil
}

// AssertFacts asserts raw Mangle fact strings into the kernel if available.
// This is used for telemetry/observability (e.g., jit_fallback/2).
// Facts may omit trailing periods; they will be normalized by the kernel adapter.
func (c *JITPromptCompiler) AssertFacts(facts []string) error {
	if c == nil || c.kernel == nil || len(facts) == 0 {
		return nil
	}
	return c.kernel.AssertBatch(toInterfaceSlice(facts))
}

// Close releases all resources held by the compiler.
func (c *JITPromptCompiler) Close() error {
	c.closeOnce.Do(func() {
		c.lifecycleMu.Lock()
		c.closed = true
		c.stop()
		c.lifecycleMu.Unlock()
		c.closeErr = c.closeResources()
	})
	return c.closeErr
}

func (c *JITPromptCompiler) closeResources() error {
	c.wg.Wait()

	func() {
		c.shardMu.Lock()
		defer c.shardMu.Unlock()
		// Close shard databases
		for id, db := range c.shardDBs {
			if err := db.Close(); err != nil {
				logging.Get(logging.CategoryContext).Warn("Failed to close shard DB %s: %v", id, err)
			}
		}
		c.shardDBs = make(map[string]*sql.DB)
	}()

	var finalErr error
	func() {
		c.dbMu.Lock()
		defer c.dbMu.Unlock()
		// Close project database
		if c.projectDB != nil {
			if err := c.projectDB.Close(); err != nil {
				finalErr = fmt.Errorf("failed to close project DB: %w", err)
				return
			}
			c.projectDB = nil
		}
	}()

	return finalErr
}
