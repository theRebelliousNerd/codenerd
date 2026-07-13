package prompt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	modTime     time.Time
	content     string
	lastChecked time.Time
}

const specialistCacheTTL = 5 * time.Second

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
	sort.Strings(specialists)
	return strings.Join(specialists, "\n")
}

func updateSpecialistCache(registryPath string, modTime time.Time, content string) {
	specialistCacheMu.Lock()
	defer specialistCacheMu.Unlock()
	specialistCache[registryPath] = specialistCacheEntry{
		modTime:     modTime,
		content:     content,
		lastChecked: time.Now(),
	}
}

// InjectAvailableSpecialists populates the context with discovered specialists.
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

	// Fast path: check TTL before stat (avoid disk I/O)
	specialistCacheMu.RLock()
	entry, found := specialistCache[registryPath]
	specialistCacheMu.RUnlock()
	if found && time.Since(entry.lastChecked) < specialistCacheTTL {
		ctx.AvailableSpecialists = entry.content
		return nil
	}

	stat, err := os.Stat(registryPath)
	if err != nil {
		// Graceful degradation - load core shards
		result := formatSpecialists(agentRegistry{})
		ctx.AvailableSpecialists = result

		// Cache negative result
		specialistCacheMu.Lock()
		specialistCache[registryPath] = specialistCacheEntry{
			modTime:     time.Time{}, // zero time
			content:     result,
			lastChecked: time.Now(),
		}
		specialistCacheMu.Unlock()
		return nil
	}

	modTime := stat.ModTime()
	if content, hit := getCachedSpecialists(registryPath, modTime); hit {
		// Content is fresh, update TTL in cache
		specialistCacheMu.Lock()
		if e, ok := specialistCache[registryPath]; ok {
			e.lastChecked = time.Now()
			specialistCache[registryPath] = e
		}
		specialistCacheMu.Unlock()

		ctx.AvailableSpecialists = content
		return nil
	}

	registry := loadAgentRegistry(registryPath)
	result := formatSpecialists(registry)

	updateSpecialistCache(registryPath, modTime, result)
	ctx.AvailableSpecialists = result

	logging.Get(logging.CategoryJIT).Debug("Injected available specialists into context")
	return nil
}

func toInterfaceSlice(strs []string) []any {
	result := make([]any, len(strs))
	for i, s := range strs {
		result[i] = s
	}
	return result
}
