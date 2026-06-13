package sync

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/sqlpragmas"

	_ "github.com/mattn/go-sqlite3"
)

// AgentSynchronizer handles the synchronization of agent definitions
// from YAML files into shard-specific SQLite databases.
type AgentSynchronizer struct {
	baseDir          string // .nerd/agents
	shardsDir        string // .nerd/shards
	atomLoader       *prompt.AtomLoader
	discoveredAgents []DiscoveredAgent // Agents found during last SyncAll
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

	// Ensure directories exist
	if err := os.MkdirAll(s.shardsDir, 0755); err != nil {
		return fmt.Errorf("failed to create shards dir: %w", err)
	}

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
	s.discoveredAgents = make([]DiscoveredAgent, 0)

	for _, entry := range entries {
		// Agents are stored in subdirectories: .nerd/agents/{agentName}/prompts.yaml
		if !entry.IsDir() {
			continue
		}

		agentID := entry.Name()
		yamlPath := filepath.Join(s.baseDir, agentID, "prompts.yaml")

		// Check if prompts.yaml exists in this subdirectory
		if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
			continue
		}

		skipped, err := s.syncAgent(ctx, agentID, yamlPath)
		if err != nil {
			logging.Get(logging.CategoryStore).Error("Failed to sync agent %s: %v", agentID, err)
			continue
		}

		// Track discovered agent for JIT registration
		dbPath := filepath.Join(s.shardsDir, fmt.Sprintf("%s_knowledge.db", strings.ToLower(agentID)))
		s.discoveredAgents = append(s.discoveredAgents, DiscoveredAgent{
			ID:     agentID,
			DBPath: dbPath,
		})
		if skipped {
			skippedCount++
		} else {
			syncedCount++
		}
	}

	logging.Get(logging.CategoryStore).Info("Agent sync: %d synced, %d skipped (unchanged)", syncedCount, skippedCount)
	return nil
}

// GetDiscoveredAgents returns all agents found during the last SyncAll call.
// Used by BootCortex to register agents with JIT compiler and ShardManager.
func (s *AgentSynchronizer) GetDiscoveredAgents() []DiscoveredAgent {
	return s.discoveredAgents
}

// syncAgent syncs a single agent's atoms to its shard database.
// Returns (skipped, error) where skipped=true means the YAML hasn't changed since last sync.
func (s *AgentSynchronizer) syncAgent(ctx context.Context, agentID string, yamlPath string) (bool, error) {
	// 1. Read YAML and compute content hash
	yamlContent, err := os.ReadFile(yamlPath)
	if err != nil {
		return false, fmt.Errorf("read YAML failed: %w", err)
	}
	hash := sha256.Sum256(yamlContent)
	contentHash := hex.EncodeToString(hash[:])

	// 2. Open Shard Database
	dbPath := filepath.Join(s.shardsDir, fmt.Sprintf("%s_knowledge.db", strings.ToLower(agentID)))
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return false, fmt.Errorf("db open failed: %w", err)
	}
	defer db.Close()
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
