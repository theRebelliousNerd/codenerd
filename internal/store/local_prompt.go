package store

import (
	"codenerd/internal/logging"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// =============================================================================
// PROMPT ATOMS (Universal JIT Prompt Compiler)
// =============================================================================

// PromptAtom represents an atomic unit of prompt content for JIT compilation.
// Atoms are selected based on contextual dimensions and assembled into complete prompts.
type PromptAtom struct {
	ID          int64
	AtomID      string // Unique identifier (e.g., "identity/coder/mission")
	Version     int
	Content     string
	TokenCount  int
	ContentHash string

	// Classification
	Category    string // Primary category (identity, protocol, safety, methodology, etc.)
	Subcategory string // Optional subcategory

	// Contextual Selectors (when this atom applies)
	OperationalModes []string // ["/active", "/debugging", "/dream", etc.]
	CampaignPhases   []string // ["/planning", "/active", "/completed", etc.]
	BuildLayers      []string // ["/scaffold", "/domain_core", "/service", etc.]
	InitPhases       []string // ["/analysis", "/kb_agent", etc.]
	NorthstarPhases  []string // ["/doc_ingestion", "/requirements", etc.]
	OuroborosStages  []string // ["/detection", "/specification", etc.]
	IntentVerbs      []string // ["/fix", "/debug", "/refactor", etc.]
	ShardTypes       []string // ["/coder", "/tester", "/reviewer", etc.]
	Languages        []string // ["/go", "/python", "/typescript", etc.]
	Frameworks       []string // ["/bubbletea", "/gin", "/rod", etc.]
	WorldStates      []string // ["failing_tests", "diagnostics", etc.]

	// Composition rules
	Priority      int    // Higher = more important (0-100)
	IsMandatory   bool   // Must always be included
	IsExclusive   string // Exclusion group (only one from group)
	DependsOn     []string
	ConflictsWith []string

	// Embeddings
	Embedding     []byte
	EmbeddingTask string // Task type used for embedding (RETRIEVAL_DOCUMENT)

	CreatedAt time.Time
}

// StorePromptAtom persists a prompt atom to the database.
func (s *LocalStore) StorePromptAtom(atom *PromptAtom) error {
	timer := logging.StartTimer(logging.CategoryStore, "StorePromptAtom")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing prompt atom: atom_id=%s category=%s tokens=%d",
		atom.AtomID, atom.Category, atom.TokenCount)

	// Serialize JSON arrays
	operationalModesJSON, _ := json.Marshal(atom.OperationalModes)
	campaignPhasesJSON, _ := json.Marshal(atom.CampaignPhases)
	buildLayersJSON, _ := json.Marshal(atom.BuildLayers)
	initPhasesJSON, _ := json.Marshal(atom.InitPhases)
	northstarPhasesJSON, _ := json.Marshal(atom.NorthstarPhases)
	ouroborosStagesJSON, _ := json.Marshal(atom.OuroborosStages)
	intentVerbsJSON, _ := json.Marshal(atom.IntentVerbs)
	shardTypesJSON, _ := json.Marshal(atom.ShardTypes)
	languagesJSON, _ := json.Marshal(atom.Languages)
	frameworksJSON, _ := json.Marshal(atom.Frameworks)
	worldStatesJSON, _ := json.Marshal(atom.WorldStates)
	dependsOnJSON, _ := json.Marshal(atom.DependsOn)
	conflictsWithJSON, _ := json.Marshal(atom.ConflictsWith)

	_, err := s.db.Exec(`
		INSERT INTO prompt_atoms (
			atom_id, version, content, token_count, content_hash,
			category, subcategory,
			operational_modes, campaign_phases, build_layers, init_phases,
			northstar_phases, ouroboros_stages, intent_verbs, shard_types,
			languages, frameworks, world_states,
			priority, is_mandatory, is_exclusive, depends_on, conflicts_with,
			embedding, embedding_task
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(atom_id) DO UPDATE SET
			version = excluded.version,
			content = excluded.content,
			token_count = excluded.token_count,
			content_hash = excluded.content_hash,
			category = excluded.category,
			subcategory = excluded.subcategory,
			operational_modes = excluded.operational_modes,
			campaign_phases = excluded.campaign_phases,
			build_layers = excluded.build_layers,
			init_phases = excluded.init_phases,
			northstar_phases = excluded.northstar_phases,
			ouroboros_stages = excluded.ouroboros_stages,
			intent_verbs = excluded.intent_verbs,
			shard_types = excluded.shard_types,
			languages = excluded.languages,
			frameworks = excluded.frameworks,
			world_states = excluded.world_states,
			priority = excluded.priority,
			is_mandatory = excluded.is_mandatory,
			is_exclusive = excluded.is_exclusive,
			depends_on = excluded.depends_on,
			conflicts_with = excluded.conflicts_with,
			embedding = excluded.embedding,
			embedding_task = excluded.embedding_task`,
		atom.AtomID, atom.Version, atom.Content, atom.TokenCount, atom.ContentHash,
		atom.Category, atom.Subcategory,
		string(operationalModesJSON), string(campaignPhasesJSON), string(buildLayersJSON), string(initPhasesJSON),
		string(northstarPhasesJSON), string(ouroborosStagesJSON), string(intentVerbsJSON), string(shardTypesJSON),
		string(languagesJSON), string(frameworksJSON), string(worldStatesJSON),
		atom.Priority, atom.IsMandatory, atom.IsExclusive, string(dependsOnJSON), string(conflictsWithJSON),
		atom.Embedding, atom.EmbeddingTask,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store prompt atom %s: %v", atom.AtomID, err)
		return fmt.Errorf("failed to store prompt atom: %w", err)
	}

	logging.StoreDebug("Prompt atom stored: atom_id=%s", atom.AtomID)
	return nil
}

// LoadPromptAtoms retrieves all prompt atoms from the database.
func (s *LocalStore) LoadPromptAtoms() ([]*PromptAtom, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadPromptAtoms")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Loading all prompt atoms")

	rows, err := s.db.Query(`
		SELECT id, atom_id, version, content, token_count, content_hash,
			   category, subcategory,
			   operational_modes, campaign_phases, build_layers, init_phases,
			   northstar_phases, ouroboros_stages, intent_verbs, shard_types,
			   languages, frameworks, world_states,
			   priority, is_mandatory, is_exclusive, depends_on, conflicts_with,
			   embedding, embedding_task, created_at
		FROM prompt_atoms
		ORDER BY priority DESC, category, atom_id`)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query prompt atoms: %v", err)
		return nil, fmt.Errorf("failed to query prompt atoms: %w", err)
	}
	defer rows.Close()

	atoms, err := s.scanPromptAtoms(rows)
	if err != nil {
		return nil, err
	}

	logging.StoreDebug("Loaded %d prompt atoms", len(atoms))
	return atoms, nil
}

// LoadPromptAtomsByCategory retrieves prompt atoms filtered by category.
func (s *LocalStore) LoadPromptAtomsByCategory(category string) ([]*PromptAtom, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadPromptAtomsByCategory")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Loading prompt atoms by category: %s", category)

	rows, err := s.db.Query(`
		SELECT id, atom_id, version, content, token_count, content_hash,
			   category, subcategory,
			   operational_modes, campaign_phases, build_layers, init_phases,
			   northstar_phases, ouroboros_stages, intent_verbs, shard_types,
			   languages, frameworks, world_states,
			   priority, is_mandatory, is_exclusive, depends_on, conflicts_with,
			   embedding, embedding_task, created_at
		FROM prompt_atoms
		WHERE category = ?
		ORDER BY priority DESC, atom_id`, category)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query prompt atoms by category %s: %v", category, err)
		return nil, fmt.Errorf("failed to query prompt atoms by category: %w", err)
	}
	defer rows.Close()

	atoms, err := s.scanPromptAtoms(rows)
	if err != nil {
		return nil, err
	}

	logging.StoreDebug("Loaded %d prompt atoms for category=%s", len(atoms), category)
	return atoms, nil
}

// GetPromptAtom retrieves a single prompt atom by its atom_id.
func (s *LocalStore) GetPromptAtom(atomID string) (*PromptAtom, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetPromptAtom")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Getting prompt atom: atom_id=%s", atomID)

	rows, err := s.db.Query(`
		SELECT id, atom_id, version, content, token_count, content_hash,
			   category, subcategory,
			   operational_modes, campaign_phases, build_layers, init_phases,
			   northstar_phases, ouroboros_stages, intent_verbs, shard_types,
			   languages, frameworks, world_states,
			   priority, is_mandatory, is_exclusive, depends_on, conflicts_with,
			   embedding, embedding_task, created_at
		FROM prompt_atoms
		WHERE atom_id = ?`, atomID)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query prompt atom %s: %v", atomID, err)
		return nil, fmt.Errorf("failed to query prompt atom: %w", err)
	}
	defer rows.Close()

	atoms, err := s.scanPromptAtoms(rows)
	if err != nil {
		return nil, err
	}

	if len(atoms) == 0 {
		logging.StoreDebug("Prompt atom not found: atom_id=%s", atomID)
		return nil, nil
	}

	logging.StoreDebug("Found prompt atom: atom_id=%s", atomID)
	return atoms[0], nil
}

// DeletePromptAtom removes a prompt atom by its atom_id.
func (s *LocalStore) DeletePromptAtom(atomID string) error {
	timer := logging.StartTimer(logging.CategoryStore, "DeletePromptAtom")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Deleting prompt atom: atom_id=%s", atomID)

	result, err := s.db.Exec("DELETE FROM prompt_atoms WHERE atom_id = ?", atomID)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to delete prompt atom %s: %v", atomID, err)
		return fmt.Errorf("failed to delete prompt atom: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		logging.StoreDebug("Prompt atom not found for deletion: atom_id=%s", atomID)
		return nil
	}

	logging.StoreDebug("Deleted prompt atom: atom_id=%s", atomID)
	return nil
}

// scanPromptAtoms scans rows into PromptAtom structs.
func (s *LocalStore) scanPromptAtoms(rows *sql.Rows) ([]*PromptAtom, error) {
	var atoms []*PromptAtom

	for rows.Next() {
		atom := &PromptAtom{}
		var operationalModesJSON, campaignPhasesJSON, buildLayersJSON, initPhasesJSON string
		var northstarPhasesJSON, ouroborosStagesJSON, intentVerbsJSON, shardTypesJSON string
		var languagesJSON, frameworksJSON, worldStatesJSON string
		var dependsOnJSON, conflictsWithJSON string
		var subcategory, isExclusive, embeddingTask sql.NullString
		var embedding []byte

		err := rows.Scan(
			&atom.ID, &atom.AtomID, &atom.Version, &atom.Content, &atom.TokenCount, &atom.ContentHash,
			&atom.Category, &subcategory,
			&operationalModesJSON, &campaignPhasesJSON, &buildLayersJSON, &initPhasesJSON,
			&northstarPhasesJSON, &ouroborosStagesJSON, &intentVerbsJSON, &shardTypesJSON,
			&languagesJSON, &frameworksJSON, &worldStatesJSON,
			&atom.Priority, &atom.IsMandatory, &isExclusive, &dependsOnJSON, &conflictsWithJSON,
			&embedding, &embeddingTask, &atom.CreatedAt,
		)
		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to scan prompt atom row: %v", err)
			continue
		}

		// Handle nullable fields
		if subcategory.Valid {
			atom.Subcategory = subcategory.String
		}
		if isExclusive.Valid {
			atom.IsExclusive = isExclusive.String
		}
		if embeddingTask.Valid {
			atom.EmbeddingTask = embeddingTask.String
		}
		atom.Embedding = embedding

		// Deserialize JSON arrays
		json.Unmarshal([]byte(operationalModesJSON), &atom.OperationalModes)
		json.Unmarshal([]byte(campaignPhasesJSON), &atom.CampaignPhases)
		json.Unmarshal([]byte(buildLayersJSON), &atom.BuildLayers)
		json.Unmarshal([]byte(initPhasesJSON), &atom.InitPhases)
		json.Unmarshal([]byte(northstarPhasesJSON), &atom.NorthstarPhases)
		json.Unmarshal([]byte(ouroborosStagesJSON), &atom.OuroborosStages)
		json.Unmarshal([]byte(intentVerbsJSON), &atom.IntentVerbs)
		json.Unmarshal([]byte(shardTypesJSON), &atom.ShardTypes)
		json.Unmarshal([]byte(languagesJSON), &atom.Languages)
		json.Unmarshal([]byte(frameworksJSON), &atom.Frameworks)
		json.Unmarshal([]byte(worldStatesJSON), &atom.WorldStates)
		json.Unmarshal([]byte(dependsOnJSON), &atom.DependsOn)
		json.Unmarshal([]byte(conflictsWithJSON), &atom.ConflictsWith)

		atoms = append(atoms, atom)
	}

	if err := rows.Err(); err != nil {
		logging.Get(logging.CategoryStore).Error("Error iterating prompt atom rows: %v", err)
		return nil, fmt.Errorf("error iterating prompt atom rows: %w", err)
	}

	return atoms, nil
}
