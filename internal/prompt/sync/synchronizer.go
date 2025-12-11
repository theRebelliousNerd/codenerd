package sync

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"

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

		if err := s.syncAgent(ctx, agentID, yamlPath); err != nil {
			logging.Get(logging.CategoryStore).Error("Failed to sync agent %s: %v", agentID, err)
			// Continue with other agents? Yes.
			continue
		}

		// Track discovered agent for JIT registration
		dbPath := filepath.Join(s.shardsDir, fmt.Sprintf("%s_knowledge.db", agentID))
		s.discoveredAgents = append(s.discoveredAgents, DiscoveredAgent{
			ID:     agentID,
			DBPath: dbPath,
		})
		syncedCount++
	}

	logging.Get(logging.CategoryStore).Info("Synced %d user agents to shard databases", syncedCount)
	return nil
}

// GetDiscoveredAgents returns all agents found during the last SyncAll call.
// Used by BootCortex to register agents with JIT compiler and ShardManager.
func (s *AgentSynchronizer) GetDiscoveredAgents() []DiscoveredAgent {
	return s.discoveredAgents
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
	if err := s.atomLoader.EnsureSchema(ctx, db); err != nil {
		return fmt.Errorf("schema init failed: %w", err)
	}

	// 4. Store Atoms
	count := 0
	for _, atom := range atoms {
		if err := s.atomLoader.StoreAtom(ctx, db, atom); err != nil {
			return fmt.Errorf("store atom %s failed: %w", atom.ID, err)
		}
		count++
	}

	logging.Get(logging.CategoryStore).Debug("Agent %s: stored %d atoms in %s", agentID, count, dbPath)
	return nil
}
