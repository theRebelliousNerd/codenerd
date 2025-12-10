package prompt

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// Fact represents a logic fact (predicate + args).
type Fact struct {
	Predicate string
	Args      []interface{}
}

// KernelQuerier defines the interface for querying the Mangle kernel.
// This abstracts the kernel to avoid circular imports.
type KernelQuerier interface {
	// Query retrieves facts matching a predicate.
	Query(predicate string) ([]Fact, error)

	// AssertBatch adds multiple facts efficiently.
	AssertBatch(facts []interface{}) error
}

// VectorSearcher defines the interface for semantic search.
type VectorSearcher interface {
	// Search performs semantic search and returns atom IDs with scores.
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
}

// SearchResult represents a semantic search result.
type SearchResult struct {
	AtomID string
	Score  float64
}

// CompilationResult holds the output of a prompt compilation.
type CompilationResult struct {
	// The compiled prompt text
	Prompt string

	// Atoms included in the prompt (for debugging/audit)
	IncludedAtoms []*PromptAtom

	// Token usage statistics
	TotalTokens    int
	BudgetUsed     float64 // Percentage of budget used
	MandatoryCount int
	OptionalCount  int

	// Categories breakdown
	CategoryTokens map[AtomCategory]int

	// Compilation metadata
	CompilationTimeMs int64
	AtomsCandidates   int // Total atoms considered
	AtomsSelected     int // Atoms that matched context
	AtomsIncluded     int // Atoms that fit in budget
	// Compilation manifest (Flight Recorder)
	Manifest *PromptManifest
}

// JITPromptCompiler compiles optimal system prompts from atomic fragments.
// It combines rule-based selection (Mangle) with semantic search (vectors)
// to select the most relevant prompt atoms for a given context.
type JITPromptCompiler struct {
	// Embedded corpus (baked-in atoms)
	embeddedCorpus *EmbeddedCorpus

	// Project-level atom database (.nerd/prompts/atoms.db)
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

	// Concurrency control
	mu sync.RWMutex

	// Configuration
	config CompilerConfig
}

// CompilerConfig holds configuration for the JIT compiler.
type CompilerConfig struct {
	// DefaultTokenBudget is the default token budget if not specified in context
	DefaultTokenBudget int

	// EnableVectorSearch enables semantic search for atom selection
	EnableVectorSearch bool

	// VectorSearchWeight is the weight of vector scores vs logic scores (0.0-1.0)
	VectorSearchWeight float64

	// MaxAtomsPerCategory caps atoms selected per category
	MaxAtomsPerCategory int

	// EnableCaching enables caching of compiled prompts
	EnableCaching bool

	// CacheTTLSeconds is the cache TTL in seconds
	CacheTTLSeconds int
}

// DefaultCompilerConfig returns a sensible default configuration.
func DefaultCompilerConfig() CompilerConfig {
	return CompilerConfig{
		DefaultTokenBudget:  100000,
		EnableVectorSearch:  true,
		VectorSearchWeight:  0.3, // 70% logic, 30% vector
		MaxAtomsPerCategory: 10,
		EnableCaching:       true,
		CacheTTLSeconds:     300, // 5 minutes
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
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(compiler); err != nil {
			return nil, fmt.Errorf("failed to apply compiler option: %w", err)
		}
	}

	logging.Get(logging.CategoryContext).Info("JITPromptCompiler initialized")
	return compiler, nil
}

// CompilerOption is a functional option for configuring the compiler.
type CompilerOption func(*JITPromptCompiler) error

// WithEmbeddedCorpus sets the embedded atom corpus.
func WithEmbeddedCorpus(corpus *EmbeddedCorpus) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.embeddedCorpus = corpus
		return nil
	}
}

// WithProjectDB sets the project-level atom database.
func WithProjectDB(db *sql.DB) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.projectDB = db
		return nil
	}
}

// WithKernel sets the Mangle kernel for rule-based selection.
func WithKernel(kernel KernelQuerier) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.kernel = kernel
		c.selector.SetKernel(kernel)
		return nil
	}
}

// WithVectorSearcher sets the vector searcher for semantic selection.
func WithVectorSearcher(vs VectorSearcher) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.vectorSearcher = vs
		c.selector.SetVectorSearcher(vs)
		return nil
	}
}

// WithConfig sets the compiler configuration.
func WithConfig(config CompilerConfig) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.config = config
		return nil
	}
}

// Compile generates a system prompt for the given context.
// This is the main entry point for prompt compilation.
func (c *JITPromptCompiler) Compile(ctx context.Context, cc *CompilationContext) (*CompilationResult, error) {
	timer := logging.StartTimer(logging.CategoryContext, "JITPromptCompiler.Compile")
	defer timer.Stop()

	if cc == nil {
		return nil, fmt.Errorf("compilation context is required")
	}

	if err := cc.Validate(); err != nil {
		return nil, fmt.Errorf("invalid compilation context: %w", err)
	}

	logging.Get(logging.CategoryContext).Info("Compiling prompt: %s", cc.String())

	// Step 1: Collect all candidate atoms from all sources
	candidates, err := c.collectAtoms(ctx, cc)
	if err != nil {
		return nil, fmt.Errorf("failed to collect atoms: %w", err)
	}

	logging.Get(logging.CategoryContext).Debug("Collected %d candidate atoms", len(candidates))

	// Step 2: Select atoms based on context (Mangle rules + vector search)
	scored, err := c.selector.SelectAtoms(ctx, candidates, cc)
	if err != nil {
		return nil, fmt.Errorf("failed to select atoms: %w", err)
	}

	logging.Get(logging.CategoryContext).Debug("Selected %d atoms after scoring", len(scored))

	// Step 3: Resolve dependencies and handle conflicts
	ordered, err := c.resolver.Resolve(scored)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	logging.Get(logging.CategoryContext).Debug("Resolved to %d ordered atoms", len(ordered))

	// Step 4: Fit within token budget
	budget := cc.AvailableTokens()
	if budget <= 0 {
		budget = c.config.DefaultTokenBudget
	}

	fitted, err := c.budgetMgr.Fit(ordered, budget)
	if err != nil {
		return nil, fmt.Errorf("failed to fit budget: %w", err)
	}

	logging.Get(logging.CategoryContext).Debug("Fitted %d atoms within budget of %d tokens", len(fitted), budget)

	// Step 5: Assemble final prompt
	prompt, err := c.assembler.Assemble(fitted, cc)
	if err != nil {
		return nil, fmt.Errorf("failed to assemble prompt: %w", err)
	}

	// Build result
	result := c.buildResult(candidates, scored, fitted, prompt, budget)

	logging.Get(logging.CategoryContext).Info(
		"Compiled prompt: %d atoms, %d tokens (%.1f%% budget)",
		result.AtomsIncluded, result.TotalTokens, result.BudgetUsed*100,
	)

	return result, nil
}

// collectAtoms gathers all candidate atoms from all sources.
func (c *JITPromptCompiler) collectAtoms(ctx context.Context, cc *CompilationContext) ([]*PromptAtom, error) {
	var allAtoms []*PromptAtom

	c.mu.RLock()
	defer c.mu.RUnlock()

	// 1. Embedded corpus (always first)
	if c.embeddedCorpus != nil {
		allAtoms = append(allAtoms, c.embeddedCorpus.All()...)
	}

	// 2. Project database
	if c.projectDB != nil {
		projectAtoms, err := c.loadAtomsFromDB(ctx, c.projectDB)
		if err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to load project atoms: %v", err)
		} else {
			allAtoms = append(allAtoms, projectAtoms...)
		}
	}

	// 3. Shard-specific database (if shard context is set)
	if cc.ShardID != "" {
		if shardDB, ok := c.shardDBs[cc.ShardID]; ok {
			shardAtoms, err := c.loadAtomsFromDB(ctx, shardDB)
			if err != nil {
				logging.Get(logging.CategoryContext).Warn("Failed to load shard atoms: %v", err)
			} else {
				allAtoms = append(allAtoms, shardAtoms...)
			}
		}
	}

	return allAtoms, nil
}

// loadAtomsFromDB loads atoms from a SQLite database.
func (c *JITPromptCompiler) loadAtomsFromDB(ctx context.Context, db *sql.DB) ([]*PromptAtom, error) {
	if db == nil {
		return nil, nil
	}

	timer := logging.StartTimer(logging.CategoryContext, "JITPromptCompiler.loadAtomsFromDB")
	defer timer.Stop()

	// 1. Load Base Atoms
	query := `
		SELECT atom_id, version, content, token_count, content_hash,
		       description, content_concise, content_min,
		       category, subcategory, priority, is_mandatory, is_exclusive, created_at
		FROM prompt_atoms
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query atoms: %w", err)
	}
	defer rows.Close()

	var atoms []*PromptAtom
	atomMap := make(map[string]*PromptAtom)

	for rows.Next() {
		atom, err := scanAtom(rows)
		if err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to scan atom: %v", err)
			continue
		}
		atoms = append(atoms, atom)
		atomMap[atom.ID] = atom
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating atoms: %w", err)
	}

	// 2. Load Context Tags
	tagRows, err := db.QueryContext(ctx, "SELECT atom_id, dimension, tag FROM atom_context_tags")
	if err != nil {
		// Log warning but don't fail, maybe table is empty or migration pending
		logging.Get(logging.CategoryContext).Warn("Failed to query atom tags: %v", err)
		return atoms, nil
	}
	defer tagRows.Close()

	var atomID, dim, tag string
	for tagRows.Next() {
		if err := tagRows.Scan(&atomID, &dim, &tag); err != nil {
			continue
		}

		if atom, exists := atomMap[atomID]; exists {
			c.appendTag(atom, dim, tag)
		}
	}

	return atoms, nil
}

// scanAtom scans a database row into a PromptAtom.
func scanAtom(rows *sql.Rows) (*PromptAtom, error) {
	var atom PromptAtom
	var category string
	var desc, conc, min, sub, excl sql.NullString
	// Note: depends_on and conflicts_with are also relations now (via tags or separate table)
	// We load them via tags if we stored them there.

	err := rows.Scan(
		&atom.ID, &atom.Version, &atom.Content, &atom.TokenCount, &atom.ContentHash,
		&desc, &conc, &min,
		&category, &sub, &atom.Priority, &atom.IsMandatory, &excl, &atom.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	atom.Category = AtomCategory(category)
	atom.Subcategory = sub.String
	atom.Description = desc.String
	atom.ContentConcise = conc.String
	atom.ContentMin = min.String
	atom.IsExclusive = excl.String

	return &atom, nil
}

// appendTag helper to hydrate atom slices based on dimension
func (c *JITPromptCompiler) appendTag(atom *PromptAtom, dim, tag string) {
	switch dim {
	case "mode":
		atom.OperationalModes = append(atom.OperationalModes, tag)
	case "phase":
		atom.CampaignPhases = append(atom.CampaignPhases, tag)
	case "layer":
		atom.BuildLayers = append(atom.BuildLayers, tag)
	case "init_phase":
		atom.InitPhases = append(atom.InitPhases, tag)
	case "northstar_phase":
		atom.NorthstarPhases = append(atom.NorthstarPhases, tag)
	case "ouroboros_stage":
		atom.OuroborosStages = append(atom.OuroborosStages, tag)
	case "intent":
		atom.IntentVerbs = append(atom.IntentVerbs, tag)
	case "shard":
		atom.ShardTypes = append(atom.ShardTypes, tag)
	case "lang":
		atom.Languages = append(atom.Languages, tag)
	case "framework":
		atom.Frameworks = append(atom.Frameworks, tag)
	case "state":
		atom.WorldStates = append(atom.WorldStates, tag)
	case "depends_on":
		atom.DependsOn = append(atom.DependsOn, tag)
	case "conflicts_with":
		atom.ConflictsWith = append(atom.ConflictsWith, tag)
	}
}

// buildResult constructs the compilation result.
func (c *JITPromptCompiler) buildResult(
	candidates []*PromptAtom,
	scored []*ScoredAtom,
	fitted []*OrderedAtom,
	prompt string,
	budget int,
) *CompilationResult {
	result := &CompilationResult{
		Prompt:          prompt,
		IncludedAtoms:   make([]*PromptAtom, 0, len(fitted)),
		CategoryTokens:  make(map[AtomCategory]int),
		AtomsCandidates: len(candidates),
		AtomsSelected:   len(scored),
		AtomsIncluded:   len(fitted),
	}

	for _, oa := range fitted {
		result.IncludedAtoms = append(result.IncludedAtoms, oa.Atom)
		result.TotalTokens += oa.Atom.TokenCount
		result.CategoryTokens[oa.Atom.Category] += oa.Atom.TokenCount

		if oa.Atom.IsMandatory {
			result.MandatoryCount++
		} else {
			result.OptionalCount++
		}
	}

	if budget > 0 {
		result.BudgetUsed = float64(result.TotalTokens) / float64(budget)
	}

	// --- Build Manifest ---
	manifest := &PromptManifest{
		Timestamp:   time.Now(),
		TokenUsage:  result.TotalTokens,
		BudgetLimit: budget,
		Selected:    make([]AtomManifestEntry, 0, len(fitted)),
		Dropped:     make([]DroppedAtomEntry, 0),
	}

	// Lookup map for scores
	scoreMap := make(map[string]*ScoredAtom)
	for _, sa := range scored {
		scoreMap[sa.Atom.ID] = sa
	}

	// Lookup map for fitted
	fittedMap := make(map[string]bool)
	for _, oa := range fitted {
		fittedMap[oa.Atom.ID] = true

		// Find scores
		var logic, vector float64
		if sa, ok := scoreMap[oa.Atom.ID]; ok {
			logic = sa.LogicScore
			vector = sa.VectorScore
		}

		manifest.Selected = append(manifest.Selected, AtomManifestEntry{
			ID:          oa.Atom.ID,
			Category:    string(oa.Atom.Category),
			Source:      "mixed", // TODO: Distinguish skeleton vs flesh
			Priority:    oa.Atom.Priority,
			LogicScore:  logic,
			VectorScore: vector,
			RenderMode:  oa.RenderMode,
		})
	}

	// Identify dropped
	// 1. Candidates rejected by Selector (Logic/Vector)
	for _, cand := range candidates {
		if _, ok := scoreMap[cand.ID]; !ok {
			manifest.Dropped = append(manifest.Dropped, DroppedAtomEntry{
				ID:     cand.ID,
				Reason: "Selector (Logic/Vector)",
			})
		}
	}

	// 2. Scored atoms rejected by Budget/Resolver
	for _, sa := range scored {
		if !fittedMap[sa.Atom.ID] {
			manifest.Dropped = append(manifest.Dropped, DroppedAtomEntry{
				ID:     sa.Atom.ID,
				Reason: "Budget/Dependency",
			})
		}
	}

	result.Manifest = manifest

	if budget > 0 {
		result.BudgetUsed = float64(result.TotalTokens) / float64(budget)
	}

	return result
}

// RegisterShardDB registers a shard-specific atom database.
// The DB should be the agent's unified knowledge database (.nerd/shards/{name}_knowledge.db)
// which contains both knowledge_atoms and prompt_atoms tables.
func (c *JITPromptCompiler) RegisterShardDB(shardID string, db *sql.DB) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shardDBs[shardID] = db
}

// UnregisterShardDB removes a shard database registration.
func (c *JITPromptCompiler) UnregisterShardDB(shardID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.shardDBs, shardID)
}

// LoadAtoms loads atoms from a database into memory.
// This is useful for pre-loading atoms for faster compilation.
func (c *JITPromptCompiler) LoadAtoms(ctx context.Context, db *sql.DB) ([]*PromptAtom, error) {
	return c.loadAtomsFromDB(ctx, db)
}

// GetConfig returns the current compiler configuration.
func (c *JITPromptCompiler) GetConfig() CompilerConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// SetConfig updates the compiler configuration.
func (c *JITPromptCompiler) SetConfig(config CompilerConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
}

// Stats returns compilation statistics.
type CompilerStats struct {
	EmbeddedAtomCount int
	ProjectAtomCount  int
	ShardDBCount      int
	TotalCompilations int64
	AverageTimeMs     float64
}

// GetStats returns current compiler statistics.
func (c *JITPromptCompiler) GetStats() CompilerStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CompilerStats{
		ShardDBCount: len(c.shardDBs),
	}

	if c.embeddedCorpus != nil {
		stats.EmbeddedAtomCount = c.embeddedCorpus.Count()
	}

	return stats
}

// Close releases all resources held by the compiler.
func (c *JITPromptCompiler) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close shard databases
	for id, db := range c.shardDBs {
		if err := db.Close(); err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to close shard DB %s: %v", id, err)
		}
	}
	c.shardDBs = make(map[string]*sql.DB)

	// Close project database
	if c.projectDB != nil {
		if err := c.projectDB.Close(); err != nil {
			return fmt.Errorf("failed to close project DB: %w", err)
		}
		c.projectDB = nil
	}

	return nil
}
