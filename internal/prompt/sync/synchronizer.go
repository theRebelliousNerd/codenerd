package sync

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/sqlpragmas"
	"codenerd/internal/types"

	_ "github.com/mattn/go-sqlite3"
)

// ErrNoAgentPrompts reports that an agent has no .nerd/agents/<name>/prompts.yaml.
var ErrNoAgentPrompts = errors.New("agent has no prompts.yaml")

// AgentSynchronizer handles the synchronization of agent definitions
// from YAML files into shard-specific SQLite databases. It is the only path
// agent prompts.yaml takes into a knowledge DB: boot, init, mid-session
// activation and the shard manager's per-spawn refresh all come through
// SyncAll or SyncAgent, so an unchanged file is skipped by content hash and a
// changed one replaces the atom set in one transaction.
type AgentSynchronizer struct {
	baseDir    string // .nerd/agents
	shardsDir  string // .nerd/shards
	atomLoader *prompt.AtomLoader

	// mu serializes syncs (one writer per knowledge DB) and guards
	// discoveredAgents against the concurrent per-spawn SyncAgent calls.
	mu               gosync.Mutex
	discoveredAgents []DiscoveredAgent // Agents synced by SyncAll/SyncAgent so far
}

// DiscoveredAgent contains info about a user-defined agent found during sync.
type DiscoveredAgent struct {
	ID     string // Agent name (e.g., "bubbleteaexpert")
	DBPath string // Path to knowledge DB (e.g., ".nerd/shards/bubbleteaexpert_knowledge.db")
}

// NewAgentSynchronizer creates a new synchronizer.
func NewAgentSynchronizer(projectRoot string, loader *prompt.AtomLoader) *AgentSynchronizer {
	return &AgentSynchronizer{
		baseDir:    filepath.Join(projectRoot, ".nerd", "agents"),
		shardsDir:  filepath.Join(projectRoot, ".nerd", "shards"),
		atomLoader: loader,
	}
}

// SyncAll syncs all agent configurations found in baseDir to their respective shard databases.
// It scans subdirectories of .nerd/agents/ looking for prompts.yaml files.
func (s *AgentSynchronizer) SyncAll(ctx context.Context) error {
	timer := logging.StartTimer(logging.CategoryStore, "AgentSynchronizer.SyncAll")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure directories exist
	if err := os.MkdirAll(s.shardsDir, 0755); err != nil {
		return fmt.Errorf("failed to create shards dir: %w", err)
	}

	s.warnLegacyProjectPrompts()

	// 1. Discover Agent subdirectories
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No agents defined yet, simpler return
			return nil
		}
		return fmt.Errorf("failed to read agents dir: %w", err)
	}

	syncedCount := 0
	skippedCount := 0
	var syncErrors []error
	s.discoveredAgents = make([]DiscoveredAgent, 0)

	for _, entry := range entries {
		// Agents are stored in subdirectories: .nerd/agents/{agentName}/prompts.yaml
		if !entry.IsDir() {
			continue
		}

		agentID := entry.Name()
		skipped, err := s.syncAgentLocked(ctx, agentID)
		if errors.Is(err, ErrNoAgentPrompts) {
			continue
		}
		if err != nil {
			logging.Get(logging.CategoryStore).Error("Failed to sync agent %s: %v", agentID, err)
			syncErrors = append(syncErrors, fmt.Errorf("agent %s: %w", agentID, err))
			continue
		}
		if skipped {
			skippedCount++
		} else {
			syncedCount++
		}
	}

	logging.Get(logging.CategoryStore).Info("Agent sync: %d synced, %d skipped (unchanged)", syncedCount, skippedCount)
	return errors.Join(syncErrors...)
}

// SyncAgent syncs one agent's prompts.yaml into its knowledge DB with the same
// hash-skip and transactional replacement as SyncAll, and records the agent as
// discovered. Returns skipped=true when the YAML is unchanged since the last
// sync. An agent without a prompts.yaml yields ErrNoAgentPrompts.
func (s *AgentSynchronizer) SyncAgent(ctx context.Context, agentID string) (skipped bool, err error) {
	timer := logging.StartTimer(logging.CategoryStore, "AgentSynchronizer.SyncAgent")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.shardsDir, 0755); err != nil {
		return false, fmt.Errorf("failed to create shards dir: %w", err)
	}
	return s.syncAgentLocked(ctx, agentID)
}

// GetDiscoveredAgents returns the agents synced so far by SyncAll and
// SyncAgent. Used by BootCortex to register agents with JIT compiler and
// ShardManager.
func (s *AgentSynchronizer) GetDiscoveredAgents() []DiscoveredAgent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]DiscoveredAgent(nil), s.discoveredAgents...)
}

// recordDiscovered notes a successfully synced agent, once per ID.
// The caller holds s.mu.
func (s *AgentSynchronizer) recordDiscovered(agentID, dbPath string) {
	for i := range s.discoveredAgents {
		if strings.EqualFold(s.discoveredAgents[i].ID, agentID) {
			s.discoveredAgents[i].DBPath = dbPath
			return
		}
	}
	s.discoveredAgents = append(s.discoveredAgents, DiscoveredAgent{ID: agentID, DBPath: dbPath})
}

// warnLegacyProjectPrompts flags .nerd/prompts/*.yaml. Nothing loads
// project-level prompt YAML; agent prompts live in
// .nerd/agents/<name>/prompts.yaml. The warning keeps those files from being
// silently ignored.
func (s *AgentSynchronizer) warnLegacyProjectPrompts() {
	promptsDir := filepath.Join(filepath.Dir(s.shardsDir), "prompts")
	entries, err := os.ReadDir(promptsDir)
	if err != nil {
		return
	}
	legacy := 0
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
			legacy++
		}
	}
	if legacy > 0 {
		logging.Get(logging.CategoryStore).Warn("Found %d YAML file(s) in .nerd/prompts/ that nothing loads; move them to .nerd/agents/<name>/prompts.yaml", legacy)
	}
}

// syncAgentLocked syncs a single agent's atoms to its shard database and
// records it as discovered. The caller holds s.mu.
// Returns (skipped, error) where skipped=true means the YAML hasn't changed since last sync.
func (s *AgentSynchronizer) syncAgentLocked(ctx context.Context, agentID string) (bool, error) {
	yamlPath := filepath.Join(s.baseDir, agentID, "prompts.yaml")
	skipped, err := s.syncAgent(ctx, agentID, yamlPath)
	if err != nil {
		return false, err
	}
	s.recordDiscovered(agentID, types.AgentKnowledgeDBPath(filepath.Dir(s.shardsDir), agentID))
	return skipped, nil
}

// syncAgent is the hash-gated sync of one prompts.yaml into the agent's DB.
func (s *AgentSynchronizer) syncAgent(ctx context.Context, agentID string, yamlPath string) (bool, error) {
	// 1. Read YAML and compute content hash
	yamlContent, err := os.ReadFile(yamlPath)
	if os.IsNotExist(err) {
		return false, fmt.Errorf("%w: %s", ErrNoAgentPrompts, agentID)
	}
	if err != nil {
		return false, fmt.Errorf("read YAML failed: %w", err)
	}
	hash := sha256.Sum256(yamlContent)
	contentHash := hex.EncodeToString(hash[:])

	// 2. Open Shard Database
	dbPath := types.AgentKnowledgeDBPath(filepath.Dir(s.shardsDir), agentID)
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return false, fmt.Errorf("db open failed: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileHot)

	// 3. Ensure Table Schema (including sync metadata)
	if err := s.atomLoader.EnsureSchema(ctx, db); err != nil {
		return false, fmt.Errorf("schema init failed: %w", err)
	}
	if err := ensureSyncMetadataTable(db); err != nil {
		return false, fmt.Errorf("sync metadata schema failed: %w", err)
	}

	// 4. Check if YAML has changed since last sync
	if storedHash, err := getLastSyncHash(db, agentID); err == nil && storedHash == contentHash {
		logging.Get(logging.CategoryStore).Debug("Agent %s: YAML unchanged (hash=%s), skipping re-embed", agentID, contentHash[:12])
		// Still heal knowledge vectors: prompt atoms are current but
		// knowledge_atoms written before embedding was wired stay unsearchable.
		if err := backfillKnowledgeVectors(ctx, db, dbPath, s.atomLoader.EmbeddingEngine()); err != nil {
			logging.Get(logging.CategoryStore).Warn("Agent %s: knowledge vector backfill failed: %v", agentID, err)
		}
		return true, nil
	}

	// 5. Parse YAML to Atoms
	atoms, err := s.atomLoader.ParseYAML(yamlPath)
	if err != nil {
		return false, fmt.Errorf("parse error: %w", err)
	}

	if len(atoms) == 0 {
		return false, nil // Nothing to sync
	}

	// 6. Replace prompt atom set transactionally to avoid stale partial state.
	if err := s.atomLoader.ReplaceAtoms(ctx, db, atoms); err != nil {
		return false, fmt.Errorf("replace atoms failed: %w", err)
	}

	// 6b. Knowledge backfill: atoms written before embedding was wired have
	// rows in knowledge_atoms but no vectors, so semantic search finds
	// nothing. Backfill them here so every boot heals existing agents
	// (e.g. bubbleteaexpert's 73 atoms with 0 vectors) and not just new ones.
	if err := backfillKnowledgeVectors(ctx, db, dbPath, s.atomLoader.EmbeddingEngine()); err != nil {
		logging.Get(logging.CategoryStore).Warn("Agent %s: knowledge vector backfill failed: %v", agentID, err)
	}

	// 7. Store the content hash for next boot
	if err := setLastSyncHash(db, agentID, contentHash); err != nil {
		logging.Get(logging.CategoryStore).Warn("Agent %s: failed to store sync hash: %v", agentID, err)
		// Non-fatal — next boot will just re-sync
	}

	logging.Get(logging.CategoryStore).Debug("Agent %s: synced %d atoms to %s (hash=%s)", agentID, len(atoms), dbPath, contentHash[:12])
	return false, nil
}

// ensureSyncMetadataTable creates the sync_metadata table if it doesn't exist.
func ensureSyncMetadataTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS sync_metadata (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	return err
}

// getLastSyncHash retrieves the last-synced content hash for an agent.
func getLastSyncHash(db *sql.DB, agentID string) (string, error) {
	var hash string
	err := db.QueryRow(`SELECT value FROM sync_metadata WHERE key = ?`, "yaml_hash_"+agentID).Scan(&hash)
	return hash, err
}

// setLastSyncHash stores the content hash after a successful sync.
func setLastSyncHash(db *sql.DB, agentID string, hash string) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO sync_metadata (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
		"yaml_hash_"+agentID, hash)
	return err
}
