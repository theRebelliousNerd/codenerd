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
	}
}

// TODO: Memory Leak: Evaluate and implement strict cache size limit (LRU or TTL) in clearPromptCache or via a background goroutine to prevent memory explosion during long sessions.
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
		c.shardDBs[name] = db
		logging.Get(logging.CategoryContext).Info("Registered database %s: %s", name, dbPath)
	}

	c.clearPromptCache("database registration updated")
	return nil
}

// RegisterShardDB registers a shard-specific atom database.
// The DB should be the agent's unified knowledge database (.nerd/shards/{name}_knowledge.db)
// which contains both knowledge_atoms and prompt_atoms tables.
func (c *JITPromptCompiler) RegisterShardDB(shardID string, db *sql.DB) {
	c.shardMu.Lock()
	c.shardDBs[shardID] = db
	c.shardMu.Unlock()
	c.clearPromptCache(fmt.Sprintf("shard database registered: %s", shardID))
}

// UnregisterShardDB removes a shard database registration.
func (c *JITPromptCompiler) UnregisterShardDB(shardID string) {
	c.shardMu.Lock()
	delete(c.shardDBs, shardID)
	c.shardMu.Unlock()
	c.clearPromptCache(fmt.Sprintf("shard database unregistered: %s", shardID))
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

// collectKnowledgeAtoms queries the LocalStore for semantically relevant knowledge atoms
// and converts them to ephemeral PromptAtoms for JIT compilation.
// This is the core of the Semantic Knowledge Bridge - connecting stored documentation
// knowledge to runtime prompt assembly.
func (c *JITPromptCompiler) collectKnowledgeAtoms(ctx context.Context, cc *CompilationContext) []*PromptAtom {
	c.dbMu.RLock()
	db := c.localDB
	c.dbMu.RUnlock()

	c.configMu.RLock()
	timeout := c.config.KnowledgeSearchTimeout
	c.configMu.RUnlock()

	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	if db == nil || cc == nil {
		return nil
	}

	// Generate a comprehensive semantic query by applying keyword extraction
	// (removing stop words) and query expansion (adding synonyms).
	// This strategy optimizes retrieval quality for our vector embedding search
	// and is superior to the prior heuristic string duplication logic.
	query := buildExpandedQuery(cc)
	if query == "" {
		return nil
	}

	// Use a sub-deadline for knowledge atom search to avoid blocking JIT compilation.
	// If embedding takes too long, we gracefully skip rather than fail the whole compilation.
	searchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Search for semantically relevant knowledge atoms
	atoms, err := db.SearchKnowledgeAtomsSemantic(searchCtx, query, 5)
	if err != nil {
		if searchCtx.Err() != nil {
			logging.Get(logging.CategoryJIT).Warn("Knowledge atom search timed out (%v limit), skipping", timeout)
		} else {
			logging.Get(logging.CategoryJIT).Debug("Knowledge atom search failed: %v", err)
		}
		return nil
	}

	if len(atoms) == 0 {
		return nil
	}

	// Convert to ephemeral PromptAtoms
	result := make([]*PromptAtom, 0, len(atoms))
	for _, atom := range atoms {
		// Format content with concept context
		content := atom.Content
		if atom.Concept != "" {
			// Extract meaningful category from concept (e.g., "doc/path/architecture/patterns" -> "architecture/patterns")
			// Optimized to avoid strings.Split/Join allocation
			idx1 := strings.IndexByte(atom.Concept, '/')
			if idx1 != -1 {
				idx2 := strings.IndexByte(atom.Concept[idx1+1:], '/')
				if idx2 != -1 {
					// The second slash is at idx1 + 1 + idx2
					realIdx2 := idx1 + 1 + idx2
					if realIdx2+1 < len(atom.Concept) {
						category := atom.Concept[realIdx2+1:]
						// Optimized formatting to avoid fmt.Sprintf reflection
						content = "[" + category + "] " + atom.Content
					}
				}
			}
		}

		// Create prompt atom with appropriate priority
		// Priority 85 = below specialist_knowledge (90) but above regular context
		// Optimized to avoid fmt.Sprintf
		atomID := "knowledge/" + HashContent(content)[:8]
		pa := NewPromptAtom(atomID, CategoryKnowledge, content)
		pa.Priority = 85
		pa.IsMandatory = false // Knowledge is contextual, not mandatory
		if cc.ShardID != "" {
			pa.ShardTypes = []string{cc.ShardID}
		}

		result = append(result, pa)
	}

	logging.Get(logging.CategoryJIT).Debug(
		"Collected %d knowledge atoms for query: %s",
		len(result), truncateQuery(query, 50))

	return result
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

	// Wait for the context or do search
	// Currently LearningStore lexical search doesn't take context, so we just run it
	_ = searchCtx

	var hits []store.LearningRecallHit
	if c.vectorSearcher != nil {
		if queryEmbedding, err := c.vectorSearcher.EmbedQuery(searchCtx, query); err == nil {
			hits, err = ls.RecallLearningsByEmbedding(queryEmbedding, 5)
			if err != nil {
				logging.Get(logging.CategoryJIT).Debug("Semantic learning atom search failed, falling back to lexical: %v", err)
			}
		} else {
			logging.Get(logging.CategoryJIT).Debug("Failed to embed query for learning atom search: %v", err)
		}
	}

	// Fallback to lexical if no semantic hits or vector searcher missing
	if len(hits) == 0 {
		var err error
		hits, err = ls.RecallLearningsLexical(query, 5)
		if err != nil {
			logging.Get(logging.CategoryJIT).Debug("Lexical learning atom search failed: %v", err)
			return nil
		}
	}

	if len(hits) == 0 {
		return nil
	}

	result := make([]*PromptAtom, 0, len(hits))
	for _, hit := range hits {
		content := fmt.Sprintf("[%s] %s: %s", hit.ShardType, hit.Predicate, hit.Summary)
		atomID := "learning/" + HashContent(content)[:8]
		pa := NewPromptAtom(atomID, CategoryKnowledge, content)
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

// truncateQuery truncates a query string for logging.
func truncateQuery(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
