package system

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// AgentOnDisk represents a user-defined agent discovered under .nerd/agents/.
// The canonical storage key is the directory name; its knowledge DB is stored
// under .nerd/shards/{lower(name)}_knowledge.db.
type AgentOnDisk struct {
	ID     string
	DBPath string
}

// DiscoverAgentsOnDisk scans .nerd/agents/* for prompts.yaml and returns discovered agents.
func DiscoverAgentsOnDisk(workspace string) ([]AgentOnDisk, error) {
	if strings.TrimSpace(workspace) == "" {
		return nil, fmt.Errorf("workspace is empty")
	}

	agentsDir := filepath.Join(workspace, ".nerd", "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read agents dir: %w", err)
	}

	discovered := make([]AgentOnDisk, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agentID := entry.Name()
		if strings.TrimSpace(agentID) == "" {
			continue
		}

		promptsPath := filepath.Join(agentsDir, agentID, "prompts.yaml")
		if _, err := os.Stat(promptsPath); err != nil {
			continue
		}

		dbPath := filepath.Join(workspace, ".nerd", "shards", fmt.Sprintf("%s_knowledge.db", strings.ToLower(agentID)))
		discovered = append(discovered, AgentOnDisk{ID: agentID, DBPath: dbPath})
	}

	sort.Slice(discovered, func(i, j int) bool {
		return strings.ToLower(discovered[i].ID) < strings.ToLower(discovered[j].ID)
	})
	return discovered, nil
}

type agentRegistryFile struct {
	Version   string               `json:"version"`
	CreatedAt string               `json:"created_at"`
	Agents    []agentRegistryAgent `json:"agents"`
}

type agentRegistryAgent struct {
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	KnowledgePath string            `json:"knowledge_path"`
	KBSize        int               `json:"kb_size"`
	CreatedAt     string            `json:"created_at,omitempty"`
	Status        string            `json:"status"`
	Tools         []string          `json:"tools,omitempty"`
	ToolPrefs     map[string]string `json:"tool_preferences,omitempty"`
}

// SyncAgentRegistryFromDisk ensures .nerd/agents.json is present and up-to-date with
// the current set of .nerd/agents/*/prompts.yaml directories. It is intentionally
// best-effort: failures should not block boot.
func SyncAgentRegistryFromDisk(workspace string) error {
	discovered, err := DiscoverAgentsOnDisk(workspace)
	if err != nil {
		return err
	}
	_, err = SyncAgentRegistryFromDiscovered(workspace, discovered)
	return err
}

// SyncAgentRegistryFromDiscovered upserts discovered agents into .nerd/agents.json.
// Returns true when the registry file was modified.
func SyncAgentRegistryFromDiscovered(workspace string, discovered []AgentOnDisk) (bool, error) {
	if strings.TrimSpace(workspace) == "" {
		return false, fmt.Errorf("workspace is empty")
	}

	nerdDir := filepath.Join(workspace, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		return false, fmt.Errorf("create .nerd dir: %w", err)
	}

	registryPath := filepath.Join(nerdDir, "agents.json")
	var existingBytes []byte
	if data, err := os.ReadFile(registryPath); err == nil {
		existingBytes = data
	}

	reg := agentRegistryFile{
		Version:   "1.5.0",
		CreatedAt: time.Now().Format(time.RFC3339),
		Agents:    []agentRegistryAgent{},
	}
	if len(existingBytes) > 0 {
		_ = json.Unmarshal(existingBytes, &reg)
		if strings.TrimSpace(reg.Version) == "" {
			reg.Version = "1.5.0"
		}
		if strings.TrimSpace(reg.CreatedAt) == "" {
			reg.CreatedAt = time.Now().Format(time.RFC3339)
		}
	}

	// Index existing agents case-insensitively.
	index := make(map[string]int, len(reg.Agents))
	for i, a := range reg.Agents {
		name := strings.TrimSpace(a.Name)
		if name == "" {
			continue
		}
		index[strings.ToLower(name)] = i
	}

	now := time.Now().Format(time.RFC3339)
	changed := false

	for _, agent := range discovered {
		id := strings.TrimSpace(agent.ID)
		if id == "" {
			continue
		}

		dbPath := strings.TrimSpace(agent.DBPath)
		status := "ready"
		kbSize := 0
		if _, err := os.Stat(dbPath); err != nil {
			status = "missing_db"
		} else {
			if count, err := countAgentAtoms(dbPath); err == nil {
				kbSize = count
			}
		}

		key := strings.ToLower(id)
		if idx, ok := index[key]; ok {
			existing := reg.Agents[idx]
			// Preserve user-edited fields when present.
			if strings.TrimSpace(existing.Type) == "" {
				existing.Type = "user"
			}
			if strings.TrimSpace(existing.Status) == "" {
				existing.Status = status
			}
			if strings.TrimSpace(existing.CreatedAt) == "" {
				existing.CreatedAt = now
			}

			// Always refresh knowledge path (it is derivable from disk).
			if existing.KnowledgePath != dbPath {
				existing.KnowledgePath = dbPath
				changed = true
			}
			// Refresh status/size when we can compute it.
			if existing.Status != status {
				existing.Status = status
				changed = true
			}
			if existing.KBSize != kbSize && kbSize > 0 {
				existing.KBSize = kbSize
				changed = true
			}

			reg.Agents[idx] = existing
			continue
		}

		reg.Agents = append(reg.Agents, agentRegistryAgent{
			Name:          id,
			Type:          "user",
			KnowledgePath: dbPath,
			KBSize:        kbSize,
			CreatedAt:     now,
			Status:        status,
		})
		index[key] = len(reg.Agents) - 1
		changed = true
	}

	// Stable ordering by name.
	sort.Slice(reg.Agents, func(i, j int) bool {
		return strings.ToLower(reg.Agents[i].Name) < strings.ToLower(reg.Agents[j].Name)
	})

	newBytes, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal agents registry: %w", err)
	}
	newBytes = append(newBytes, '\n')

	// Avoid touching the file if nothing changed.
	if !changed && len(existingBytes) > 0 && string(existingBytes) == string(newBytes) {
		return false, nil
	}
	if len(existingBytes) > 0 && string(existingBytes) == string(newBytes) {
		return false, nil
	}

	if err := os.WriteFile(registryPath, newBytes, 0644); err != nil {
		return false, fmt.Errorf("write agents registry: %w", err)
	}
	return true, nil
}

func countAgentAtoms(dbPath string) (int, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	for _, table := range []string{"knowledge_atoms", "prompt_atoms"} {
		var count int
		err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err == nil {
			return count, nil
		}
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			continue
		}
	}
	return 0, nil
}
