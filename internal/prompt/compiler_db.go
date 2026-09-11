package prompt

import (
	"container/list"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/sqlpragmas"
	"codenerd/internal/store"
)

// loadAtomsFromDB loads atoms from a SQLite database.
func (c *JITPromptCompiler) loadAtomsFromDB(ctx context.Context, db *sql.DB) ([]*PromptAtom, error) {
	if db == nil {
		return nil, nil
	}

	timer := logging.StartTimer(logging.CategoryContext, "JITPromptCompiler.loadAtomsFromDB")
	defer timer.Stop()

	// 1. Load Base Atoms and Context Tags combined via LEFT JOIN
	// Added LIMIT 10000 to prevent memory exhaustion from massive atom corpus
	query := `
		SELECT a.atom_id, a.version, a.content, a.token_count, a.content_hash,
		       a.description, a.content_concise, a.content_min,
		       a.category, a.subcategory, a.priority, a.is_mandatory, a.is_exclusive, a.created_at,
		       t.dimension, t.tag
		FROM (SELECT * FROM prompt_atoms LIMIT 10000) a
		LEFT JOIN atom_context_tags t ON a.atom_id = t.atom_id
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query atoms with tags: %w", err)
	}
	defer rows.Close()

	var atoms []*PromptAtom
	atomMap := make(map[string]*PromptAtom)

	for rows.Next() {
		var atomID, content, contentHash, category string
		var tokenCount, priority, version int
		var isMandatory bool
		var createdAt time.Time
		var desc, conc, min, sub, excl sql.NullString
		var dim, tag sql.NullString

		err := rows.Scan(
			&atomID, &version, &content, &tokenCount, &contentHash,
			&desc, &conc, &min,
			&category, &sub, &priority, &isMandatory, &excl, &createdAt,
			&dim, &tag,
		)
		if err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to scan atom row: %v", err)
			continue
		}

		atom, exists := atomMap[atomID]
		if !exists {
			atom = &PromptAtom{
				ID:             atomID,
				Version:        version,
				Content:        content,
				TokenCount:     tokenCount,
				ContentHash:    contentHash,
				Description:    desc.String,
				ContentConcise: conc.String,
				ContentMin:     min.String,
				Category:       AtomCategory(category),
				Subcategory:    sub.String,
				Priority:       priority,
				IsMandatory:    isMandatory,
				IsExclusive:    excl.String,
				CreatedAt:      createdAt,
			}
			atoms = append(atoms, atom)
			atomMap[atomID] = atom
		}

		// Apply tag if present
		if dim.Valid && tag.Valid {
			c.appendTag(atom, dim.String, tag.String)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating atoms: %w", err)
	}

	return atoms, nil
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
	case "requires_tool":
		atom.RequiresTools = append(atom.RequiresTools, tag)
	}
}

func (c *JITPromptCompiler) clearPromptCache(reason string) {
	c.cacheMu.Lock()
	c.cache = make(map[string]*list.Element)
	c.cacheList.Init()
	c.cacheMu.Unlock()

	atomic.StoreInt64(&c.cacheHits, 0)
	atomic.StoreInt64(&c.cacheMiss, 0)

	if reason != "" {
		logging.Get(logging.CategoryJIT).Info("Cleared prompt cache: %s", reason)
	}
}

// RegisterDB registers a named database with the JIT compiler.
// Known names:
//   - "corpus": Sets the project-level corpus database (embedded atoms synced to SQLite)
//   - "project": Alias for "corpus"
//
// The method opens the database file and registers it. The caller is responsible
// for ensuring the file exists. Call Close() to release all DB connections.
func (c *JITPromptCompiler) RegisterDB(name, dbPath string) error {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if c.closed {
		return fmt.Errorf("prompt compiler is closed")
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database %s: %w", dbPath, err)
	}
	sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileHot)

	// Verify connection is valid
	if pingErr := db.Ping(); pingErr != nil {
		db.Close()
		return fmt.Errorf("failed to ping database %s: %w", dbPath, pingErr)
	}

	c.dbMu.Lock()
	defer c.dbMu.Unlock()

	switch name {
	case "corpus", "project":
		// Close existing project DB if any
		if c.projectDB != nil {
			c.projectDB.Close()
		}
		c.projectDB = db
		logging.Get(logging.CategoryContext).Info("Registered corpus database: %s", dbPath)
	default:
		// Treat unknown names as shard IDs for flexibility
		c.shardMu.Lock()
		if previous := c.shardDBs[shardDBKey(name)]; previous != nil && previous != db {
			_ = previous.Close()
		}
		c.shardDBs[shardDBKey(name)] = db
		c.shardMu.Unlock()
		logging.Get(logging.CategoryContext).Info("Registered database %s: %s", name, dbPath)
	}

	c.clearPromptCache("database registration updated")
	return nil
}

// shardDBKey normalizes a shard/agent identifier for the shardDBs map.
//
// Registration and lookup disagreed on case. A user agent is registered under
// its on-disk directory name (internal/system/factory.go: RegisterAgentDBWithJIT
// with agent.ID from DiscoverAgentsOnDisk, e.g. "RustExpert"), but every verb
// that reaches the compiler has been lower-cased on the way in — `nerd spawn
// RustExpert` becomes "/rustexpert" in normalizeTaskIntentVerb
// (internal/session/task_executor.go). A case-sensitive map turned that into a
// silent miss: the agent's own prompt atoms were loaded, indexed, and never
// selected. Normalizing both ends removes the whole class.
func shardDBKey(shardID string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(shardID), "/")))
}

// RegisterShardDB registers a shard-specific atom database.
// The DB should be the agent's unified knowledge database (.nerd/shards/{name}_knowledge.db)
// which contains both knowledge_atoms and prompt_atoms tables.
func (c *JITPromptCompiler) RegisterShardDB(shardID string, db *sql.DB) {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if c.closed {
		if db != nil {
			_ = db.Close()
		}
		return
	}
	c.shardMu.Lock()
	if previous := c.shardDBs[shardDBKey(shardID)]; previous != nil && previous != db {
		_ = previous.Close()
	}
	c.shardDBs[shardDBKey(shardID)] = db
	c.shardMu.Unlock()
	c.clearPromptCache(fmt.Sprintf("shard database registered: %s", shardID))
}

// UnregisterShardDB removes a shard database registration.
func (c *JITPromptCompiler) UnregisterShardDB(shardID string) {
	c.shardMu.Lock()
	delete(c.shardDBs, shardDBKey(shardID))
	c.shardMu.Unlock()
	c.clearPromptCache(fmt.Sprintf("shard database unregistered: %s", shardID))
}

// LookupShardDB returns the atom database registered for an agent name, if any.
// Exported so callers (and tests) can assert that a user-defined agent's
// knowledge DB is actually reachable under the name the executor will use.
func (c *JITPromptCompiler) LookupShardDB(shardID string) (*sql.DB, bool) {
	c.shardMu.RLock()
	defer c.shardMu.RUnlock()
	db, ok := c.shardDBs[shardDBKey(shardID)]
	return db, ok
}

// LoadAtoms loads atoms from a database into memory.
// This is useful for pre-loading atoms for faster compilation.
func (c *JITPromptCompiler) LoadAtoms(ctx context.Context, db *sql.DB) ([]*PromptAtom, error) {
	return c.loadAtomsFromDB(ctx, db)
}

// GetConfig returns the current compiler configuration.
func (c *JITPromptCompiler) GetConfig() CompilerConfig {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	return c.config
}

// SetConfig updates the compiler configuration.
func (c *JITPromptCompiler) SetConfig(config CompilerConfig) {
	c.configMu.Lock()
	defer c.configMu.Unlock()
	c.config = config
	c.selector.SetVectorSearchTimeout(config.VectorSearchTimeout)
	// The sibling knob, and it was wired nowhere. SetVectorWeight had no
	// production caller and CompilerConfig.VectorSearchWeight had no
	// reader: two halves of one missing wire, which is why the selector
	// and the config each carried their own 0.3 with the same
	// "70% logic, 30% vector" comment attached.
	//
	// The zero is guarded because the two setters do NOT agree about what
	// one means. SetVectorSearchTimeout reads zero as "unset" and
	// substitutes ten seconds; SetVectorWeight clamps to [0,1] and takes a
	// zero literally, as pure logic. So wiring this unguarded would make a
	// partially-filled CompilerConfig silently turn vector scoring off --
	// the exact silent-failure shape this is being fixed to remove. Pure
	// logic is expressed by installing no vector searcher at all
	// (selector.go skips the search when vectorSearcher is nil).
	if config.VectorSearchWeight > 0 {
		c.selector.SetVectorWeight(config.VectorSearchWeight)
	}
}

// SetLocalDB sets the LocalStore for semantic knowledge atom queries.
// This enables the Semantic Knowledge Bridge, allowing JIT to query
// knowledge atoms with embeddings for context-aware prompt assembly.
func (c *JITPromptCompiler) SetLocalDB(db *store.LocalStore) {
	c.dbMu.Lock()
	defer c.dbMu.Unlock()
	c.localDB = db
}

// SetLearningStore sets the LearningStore for recalling learned intents.
func (c *JITPromptCompiler) SetLearningStore(ls *store.LearningStore) {
	c.dbMu.Lock()
	defer c.dbMu.Unlock()
	c.learningStore = ls
}

// collectKnowledgeAtoms queries project knowledge plus the selected expert's
// registered knowledge DB and converts hits to ephemeral PromptAtoms.
// This is the Semantic Knowledge Bridge with a bounded expert extension:
// the project LocalStore is searched semantically (with a lexical fallback
// when no embedding engine is configured) and the single selected expert DB
// is searched lexically through the shared handle. Sibling experts are never
// consulted. Missing knowledge reads as missing (nil); nothing is fabricated.
func (c *JITPromptCompiler) collectKnowledgeAtoms(ctx context.Context, cc *CompilationContext) []*PromptAtom {
	if cc == nil {
		return nil
	}
	c.dbMu.RLock()
	db := c.localDB
	c.dbMu.RUnlock()

	c.configMu.RLock()
	timeout := c.config.KnowledgeSearchTimeout
	c.configMu.RUnlock()
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	var shardDB *sql.DB
	var hasShard bool
	if cc.ShardID != "" {
		shardDB, hasShard = c.LookupShardDB(cc.ShardID)
		if hasShard && shardDB == nil {
			hasShard = false
		}
	}
	if db == nil && !hasShard {
		return nil
	}

	query := buildExpandedQuery(cc)
	if query == "" {
		return nil
	}

	searchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Explicit task terms precede expanded routing terms in bounded lexical
	// retrieval. Search the selected expert first so a slow project embedding
	// cannot consume its entire deadline before the expert is consulted.
	expertQuery := strings.TrimSpace(cc.SemanticQuery + " " + query)
	expertAtoms := collectExpertKnowledgeAtoms(searchCtx, cc, shardDB, hasShard, expertQuery)
	projectAtoms := c.collectProjectKnowledgeAtoms(searchCtx, db, query)
	if len(projectAtoms) == 0 && len(expertAtoms) == 0 {
		return nil
	}
	if ctx.Err() != nil {
		return nil
	}

	seen := make(map[string]struct{}, len(projectAtoms)+len(expertAtoms))
	result := make([]*PromptAtom, 0, len(projectAtoms)+len(expertAtoms))
	result = appendKnowledgePromptAtoms(result, seen, projectAtoms, cc.ShardID, "project")
	result = appendKnowledgePromptAtoms(result, seen, expertAtoms, cc.ShardID, "expert:"+shardDBKey(cc.ShardID))
	if len(result) == 0 {
		return nil
	}

	logging.Get(logging.CategoryJIT).Debug(
		"Collected %d knowledge atoms (project=%d expert=%d) for query: %s",
		len(result), len(projectAtoms), len(expertAtoms), truncateQuery(query, 50))

	return result
}

// collectProjectKnowledgeAtoms searches existing project knowledge.
// Semantic search runs first; when it yields nothing (for example no
// embedding engine) a bounded lexical search covers the same table.
func (c *JITPromptCompiler) collectProjectKnowledgeAtoms(ctx context.Context, db *store.LocalStore, query string) []store.KnowledgeAtom {
	if db == nil {
		return nil
	}
	if ctx.Err() != nil {
		return nil
	}
	atoms, err := db.SearchKnowledgeAtomsSemantic(ctx, query, 5)
	if err == nil && len(atoms) > 0 {
		return atoms
	}
	if ctx.Err() != nil {
		return nil
	}
	lex, lerr := db.SearchKnowledgeAtomsLexical(ctx, query, 5)
	if lerr != nil || len(lex) == 0 {
		if lerr != nil {
			logging.Get(logging.CategoryJIT).Debug("Project knowledge lookup failed: %v", lerr)
		}
		return nil
	}
	return lex
}

// collectExpertKnowledgeAtoms searches only the selected expert's registered
// DB via its shared handle. It never iterates sibling experts, never opens a
// new store, and never closes a DB owned by the shard registry.
func collectExpertKnowledgeAtoms(ctx context.Context, cc *CompilationContext, shardDB *sql.DB, hasShard bool, query string) []store.KnowledgeAtom {
	if !hasShard || shardDB == nil || cc == nil {
		return nil
	}
	if ctx.Err() != nil {
		return nil
	}
	atoms, err := store.SearchKnowledgeAtomsLexicalDB(ctx, shardDB, query, 5)
	if err != nil || len(atoms) == 0 {
		if err != nil {
			logging.Get(logging.CategoryJIT).Debug("Expert knowledge lookup failed for %s: %v", cc.ShardID, err)
		}
		return nil
	}
	return atoms
}

// knowledgeAtomToPromptAtom retains source identity and stored confidence in
// the actual context, so a tentative advisory is not rendered as an unqualified
// fact. Confidence describes the stored claim, not behavioral verification.
func knowledgeAtomToPromptAtom(atom store.KnowledgeAtom, shardID, source string) *PromptAtom {
	content := fmt.Sprintf("[knowledge source=%q concept=%q confidence=%g]\n%s", source, atom.Concept, atom.Confidence, atom.Content)
	atomID := "knowledge/" + HashContent(content)[:8]
	pa := NewPromptAtom(atomID, CategoryKnowledge, content)
	pa.RetrievedContext = true
	pa.Priority = 85
	pa.IsMandatory = false
	if shardID != "" {
		pa.ShardTypes = []string{shardDBKey(shardID)}
	}
	return pa
}

// appendKnowledgePromptAtoms converts hits, dedupes by ephemeral atom ID, and
// caps the merged project-plus-expert set so one compile stays bounded.
func appendKnowledgePromptAtoms(dst []*PromptAtom, seen map[string]struct{}, atoms []store.KnowledgeAtom, shardID, source string) []*PromptAtom {
	for _, atom := range atoms {
		if len(dst) >= 10 {
			break
		}
		pa := knowledgeAtomToPromptAtom(atom, shardID, source)
		if _, dup := seen[pa.ID]; dup {
			continue
		}
		seen[pa.ID] = struct{}{}
		dst = append(dst, pa)
		if len(dst) >= 10 {
			break
		}
	}
	return dst
}

// collectLearningAtoms queries the LearningStore for relevant past learnings.
func (c *JITPromptCompiler) collectLearningAtoms(ctx context.Context, cc *CompilationContext) []*PromptAtom {
	c.dbMu.RLock()
	ls := c.learningStore
	c.dbMu.RUnlock()

	c.configMu.RLock()
	timeout := c.config.VectorSearchTimeout
	c.configMu.RUnlock()

	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	if ls == nil || cc == nil {
		return nil
	}

	query := buildExpandedQuery(cc)
	if query == "" {
		return nil
	}

	searchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Search the explicit target before expanded routing terms. Always include
	// lexical retrieval: an older vector hit must not hide an unembedded fact.
	lexicalQuery := strings.TrimSpace(cc.IntentTarget + " " + cc.SemanticQuery + " " + query)
	lexical, lexErr := ls.RecallLearningsLexicalContext(searchCtx, lexicalQuery, 5)
	if lexErr != nil {
		logging.Get(logging.CategoryJIT).Debug("Lexical learning atom search failed: %v", lexErr)
	}
	var semantic []store.LearningRecallHit
	if c.vectorSearcher != nil {
		if queryEmbedding, err := c.vectorSearcher.EmbedQuery(searchCtx, query); err == nil {
			semantic, err = ls.RecallLearningsByEmbeddingContext(searchCtx, queryEmbedding, 5)
			if err != nil {
				logging.Get(logging.CategoryJIT).Debug("Semantic learning atom search failed, falling back to lexical: %v", err)
			}
		} else {
			logging.Get(logging.CategoryJIT).Debug("Failed to embed query for learning atom search: %v", err)
		}
	}

	hits := mergeLearningRecallHits(lexical, semantic, 5)

	if len(hits) == 0 {
		return nil
	}

	result := make([]*PromptAtom, 0, len(hits))
	for _, hit := range hits {
		body, err := ls.RecallLearningContentContext(searchCtx, hit)
		if err != nil {
			logging.Get(logging.CategoryJIT).Debug("Learning content unavailable: %v", err)
			continue
		}
		content := fmt.Sprintf("[learning shard=%q source_campaign=%q learned_at=%q confidence=%g]\n%s: %s",
			hit.ShardType, hit.SourceCampaign, hit.LearnedAt.Format(time.RFC3339), hit.Confidence, hit.Predicate, body)
		atomID := "learning/" + HashContent(content)[:8]
		pa := NewPromptAtom(atomID, CategoryKnowledge, content)
		pa.RetrievedContext = true
		pa.Priority = 88 // slightly above regular knowledge
		pa.IsMandatory = false
		if cc.ShardID != "" {
			pa.ShardTypes = []string{cc.ShardID}
		}
		result = append(result, pa)
	}

	logging.Get(logging.CategoryJIT).Debug(
		"Collected %d learning atoms for query: %s",
		len(result), truncateQuery(query, 50))

	return result
}

// Interleave ranked sources without comparing incomparable vector and lexical
// scores. Identity includes the shard because learning IDs are database-local.
func mergeLearningRecallHits(lexical, semantic []store.LearningRecallHit, limit int) []store.LearningRecallHit {
	type key struct {
		shard string
		id    int64
	}
	seen := make(map[key]bool)
	var result []store.LearningRecallHit
	for i := 0; i < max(len(lexical), len(semantic)) && len(result) < limit; i++ {
		for _, source := range [][]store.LearningRecallHit{lexical, semantic} {
			if i >= len(source) || len(result) >= limit {
				continue
			}
			hit := source[i]
			k := key{hit.ShardType, hit.LearningID}
			if !seen[k] {
				seen[k] = true
				result = append(result, hit)
			}
		}
	}
	return result
}

// truncateQuery truncates a query string for logging.
func truncateQuery(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
