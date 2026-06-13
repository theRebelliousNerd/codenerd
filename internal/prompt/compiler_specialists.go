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

// InjectAvailableSpecialists populates the context with discovered specialists.
// TODO: Performance: This reads a file on EVERY compilation. Implement in-memory caching with filesystem watcher or TTL.
// This enables the LLM to know what domain experts are available for consultation.
// Reads from .nerd/agents.json and formats as a markdown list for template injection.
func InjectAvailableSpecialists(ctx *CompilationContext, workspace string) error {
	if ctx == nil {
		return nil
	}
	if workspace == "" {
		if cwd, err := os.Getwd(); err == nil {
			workspace = cwd
		}
	}
	if workspace == "" {
		return nil
	}

	registryPath := filepath.Join(workspace, ".nerd", "agents.json")

	stat, err := os.Stat(registryPath)
	if err != nil {
		// Graceful degradation - no custom specialists available, but we'll still add core shards
		data := []byte("{}")

		// Parse the agent registry (which is empty)
		var registry struct {
			Agents []struct {
				Name        string `json:"name"`
				Type        string `json:"type"`
				Status      string `json:"status"`
				Description string `json:"description"`
				Topics      string `json:"topics"`
			} `json:"agents"`
		}
		_ = json.Unmarshal(data, &registry)

		var specialists []string
		// Add core shards as implicit specialists (from centralized definitions)
		for name, desc := range shards.CoreShardDescriptions {
			specialists = append(specialists, fmt.Sprintf("- **%s**: %s", name, desc))
		}

		var result string
		if len(specialists) == 0 {
			result = "No specialists available. Use **researcher** for general knowledge gathering."
		} else {
			result = strings.Join(specialists, "\n")
		}

		ctx.AvailableSpecialists = result
		return nil
	}

	modTime := stat.ModTime()

	// Check cache
	specialistCacheMu.RLock()
	entry, found := specialistCache[registryPath]
	specialistCacheMu.RUnlock()

	if found && !modTime.After(entry.modTime) {
		ctx.AvailableSpecialists = entry.content
		return nil
	}

	// Cache miss or stale - read file
	data, err := os.ReadFile(registryPath)
	if err != nil {
		// Should be rare since Stat succeeded, but possible
		logging.Get(logging.CategoryJIT).Warn("Failed to read agents.json: %v", err)
		data = []byte("{}") // Proceed with empty registry to inject core shards
	}

	// Parse the agent registry
	var registry struct {
		Agents []struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Status      string `json:"status"`
			Description string `json:"description"`
			Topics      string `json:"topics"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		logging.Get(logging.CategoryJIT).Warn("Failed to parse agents.json: %v", err)
		// Proceed anyway to at least inject core shards
	}

	// Build specialist descriptions
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

	// Add core shards as implicit specialists (from centralized definitions)
	for name, desc := range shards.CoreShardDescriptions {
		specialists = append(specialists, fmt.Sprintf("- **%s**: %s", name, desc))
	}

	var result string
	if len(specialists) == 0 {
		result = "No specialists available. Use **researcher** for general knowledge gathering."
	} else {
		result = strings.Join(specialists, "\n")
	}

	// Update cache
	specialistCacheMu.Lock()
	specialistCache[registryPath] = specialistCacheEntry{
		modTime: modTime,
		content: result,
	}
	specialistCacheMu.Unlock()

	ctx.AvailableSpecialists = result
	logging.Get(logging.CategoryJIT).Debug("Injected %d available specialists into context", len(specialists))
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
