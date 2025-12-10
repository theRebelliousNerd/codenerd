package sync

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"

	_ "github.com/mattn/go-sqlite3"
)

// AgentSynchronizer handles the synchronization of agent definitions
// from YAML files into shard-specific SQLite databases.
type AgentSynchronizer struct {
	baseDir    string // .nerd/agents
	shardsDir  string // .nerd/shards
	atomLoader *prompt.AtomLoader
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
func (s *AgentSynchronizer) SyncAll(ctx context.Context) error {
	timer := logging.StartTimer(logging.CategoryStore, "AgentSynchronizer.SyncAll")
	defer timer.Stop()

	// Ensure directories exist
	if err := os.MkdirAll(s.shardsDir, 0755); err != nil {
		return fmt.Errorf("failed to create shards dir: %w", err)
	}

	// 1. Discover Agent YAMLs
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No agents defined yet, simpler return
			return nil
		}
		return fmt.Errorf("failed to read agents dir: %w", err)
	}

	syncedCount := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		agentID := strings.TrimSuffix(entry.Name(), ".yaml")
		yamlPath := filepath.Join(s.baseDir, entry.Name())

		if err := s.syncAgent(ctx, agentID, yamlPath); err != nil {
			logging.Get(logging.CategoryStore).Error("Failed to sync agent %s: %v", agentID, err)
			// Continue with other agents? Yes.
			continue
		}
		syncedCount++
	}

	logging.Get(logging.CategoryStore).Info("Synced %d agents to shard databases", syncedCount)
	return nil
}

// syncAgent syncs a single agent's atoms to its shard database.
func (s *AgentSynchronizer) syncAgent(ctx context.Context, agentID string, yamlPath string) error {
	// 1. Parse YAML to Atoms
	// We use the exposed ParseYAML method from AtomLoader
	atoms, err := s.atomLoader.ParseYAML(yamlPath)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	if len(atoms) == 0 {
		return nil // Nothing to sync
	}

	// 2. Open Shard Database
	dbPath := filepath.Join(s.shardsDir, fmt.Sprintf("%s_knowledge.db", agentID))
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("db open failed: %w", err)
	}
	defer db.Close()

	// 3. Ensure Table Schema
	// We reuse AtomLoader's EnsureTable method if exposed or duplicate it?
	// AtomLoader.EnsureTable is private: ensurePromptAtomsTable.
	// We should expose it or make it accessible.
	// Alternatively, we can rely on `internal/store` migrations if this was a shared DB.
	// But these are ad-hoc shard DBs.
	// Let's assume we need to instantiate the schema.
	// To minimize duplication, we should expose `EnsureTable` in `loader.go`.
	// For now, I'll assume valid schema or call a helper.
	// User rule: "normalized link tables".
	// I'll add `EnsureSchema(db)` to AtomLoader interface as well.

	if err := s.atomLoader.EnsureSchema(ctx, db); err != nil {
		return fmt.Errorf("schema init failed: %w", err)
	}

	// 4. Store Atoms
	count := 0
	for _, atom := range atoms {
		// Override ShardType in atom matches agentID?
		// Or assume YAML author set it correctly?
		// Authors usually define atoms for that agent.

		// We use AtomLoader.StoreAtom (private).
		// Need `StoreAtom` (public).
		if err := s.atomLoader.StoreAtom(ctx, db, atom); err != nil {
			return fmt.Errorf("store atom %s failed: %w", atom.ID, err)
		}
		count++
	}

	logging.Get(logging.CategoryStore).Debug("Agent %s: stored %d atoms in %s", agentID, count, dbPath)
	return nil
}
