package prompt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/core/shards"
	"codenerd/internal/logging"
)

// specialistCache stores cached specialist strings to avoid re-reading agents.json.
var (
	specialistCache   = make(map[string]specialistCacheEntry)
	specialistCacheMu sync.RWMutex
)

type specialistCacheEntry struct {
	modTime time.Time
	content string
}

type agentRegistry struct {
	Agents []struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		Status      string `json:"status"`
		Description string `json:"description"`
		Topics      string `json:"topics"`
	} `json:"agents"`
}

func getWorkspace(workspace string) string {
	if workspace == "" {
		if cwd, err := os.Getwd(); err == nil {
			return cwd
		}
	}
	return workspace
}

func getCachedSpecialists(registryPath string, modTime time.Time) (string, bool) {
	specialistCacheMu.RLock()
	defer specialistCacheMu.RUnlock()
	entry, found := specialistCache[registryPath]
	if found && !modTime.After(entry.modTime) {
		return entry.content, true
	}
	return "", false
}

func loadAgentRegistry(registryPath string) agentRegistry {
	var registry agentRegistry
	data, err := os.ReadFile(registryPath)
	if err != nil {
		logging.Get(logging.CategoryJIT).Warn("Failed to read agents.json: %v", err)
		data = []byte("{}")
	}

	if err := json.Unmarshal(data, &registry); err != nil {
		logging.Get(logging.CategoryJIT).Warn("Failed to parse agents.json: %v", err)
	}
	return registry
}

func formatSpecialists(registry agentRegistry) string {
	var specialists []string
	for _, agent := range registry.Agents {
		if agent.Status != "ready" {
			continue
		}
		desc := agent.Description
		if desc == "" && agent.Topics != "" {
			desc = fmt.Sprintf("%s specialist (%s)", agent.Type, agent.Topics)
		} else if desc == "" {
			desc = fmt.Sprintf("%s domain specialist", agent.Type)
		}
		specialists = append(specialists, fmt.Sprintf("- **%s**: %s", agent.Name, desc))
	}

	for name, desc := range shards.CoreShardDescriptions {
		specialists = append(specialists, fmt.Sprintf("- **%s**: %s", name, desc))
	}

	if len(specialists) == 0 {
		return "No specialists available. Use **researcher** for general knowledge gathering."
	}
	return strings.Join(specialists, "\n")
}

func updateSpecialistCache(registryPath string, modTime time.Time, content string) {
	specialistCacheMu.Lock()
	defer specialistCacheMu.Unlock()
	specialistCache[registryPath] = specialistCacheEntry{
		modTime: modTime,
		content: content,
	}
}

// InjectAvailableSpecialists populates the context with discovered specialists.
// TODO: Performance: This reads a file on EVERY compilation. Implement in-memory caching with filesystem watcher or TTL.
// This enables the LLM to know what domain experts are available for consultation.
// Reads from .nerd/agents.json and formats as a markdown list for template injection.
func InjectAvailableSpecialists(ctx *CompilationContext, workspace string) error {
	if ctx == nil {
		return nil
	}

	workspace = getWorkspace(workspace)
	if workspace == "" {
		return nil
	}

	registryPath := filepath.Join(workspace, ".nerd", "agents.json")

	stat, err := os.Stat(registryPath)
	if err != nil {
		ctx.AvailableSpecialists = formatSpecialists(agentRegistry{})
		return nil
	}

	modTime := stat.ModTime()
	if content, hit := getCachedSpecialists(registryPath, modTime); hit {
		ctx.AvailableSpecialists = content
		return nil
	}

	registry := loadAgentRegistry(registryPath)
	result := formatSpecialists(registry)

	updateSpecialistCache(registryPath, modTime, result)
	ctx.AvailableSpecialists = result

	// approximate length without counting
	logging.Get(logging.CategoryJIT).Debug("Injected available specialists into context")
	return nil
}

// toInterfaceSlice converts a string slice to an interface slice.
// Used to pass context facts to the kernel's AssertBatch method.
func toInterfaceSlice(strs []string) []any {
	result := make([]any, len(strs))
	for i, s := range strs {
		result[i] = s
	}
	return result
}
