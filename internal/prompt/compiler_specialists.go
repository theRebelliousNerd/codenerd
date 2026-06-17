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
	modTime     time.Time
	content     string
	lastChecked time.Time
}

const specialistCacheTTL = 5 * time.Second

// InjectAvailableSpecialists populates the context with discovered specialists.
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

	// 1. Fast path: check TTL
	specialistCacheMu.RLock()
	entry, found := specialistCache[registryPath]
	specialistCacheMu.RUnlock()

	if found && time.Since(entry.lastChecked) < specialistCacheTTL {
		ctx.AvailableSpecialists = entry.content
		return nil
	}

	stat, err := os.Stat(registryPath)
	if err != nil {
		// File doesn't exist or is inaccessible. We still cache this "negative" result.
		// Graceful degradation - no custom specialists available, but we'll still add core shards.
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

		// Cache the negative result so we don't reconstruct it or os.Stat on every compilation
		specialistCacheMu.Lock()
		specialistCache[registryPath] = specialistCacheEntry{
			modTime:     time.Time{}, // zero time for not found
			content:     result,
			lastChecked: time.Now(),
		}
		specialistCacheMu.Unlock()

		return nil
	}

	modTime := stat.ModTime()

	// If file exists, check if modTime matches cached modTime
	if found && !modTime.After(entry.modTime) {
		// Content is fresh, just update the TTL
		specialistCacheMu.Lock()
		if e, ok := specialistCache[registryPath]; ok {
			e.lastChecked = time.Now()
			specialistCache[registryPath] = e
		}
		specialistCacheMu.Unlock()

		ctx.AvailableSpecialists = entry.content
		return nil
	}

	// Cache miss or file was modified - read file
	data, err := os.ReadFile(registryPath)
	if err != nil {
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

	// Update cache with the new content
	specialistCacheMu.Lock()
	specialistCache[registryPath] = specialistCacheEntry{
		modTime:     modTime,
		content:     result,
		lastChecked: time.Now(),
	}
	specialistCacheMu.Unlock()

	ctx.AvailableSpecialists = result
	logging.Get(logging.CategoryJIT).Debug("Injected %d available specialists into context", len(specialists))
	return nil
}

func toInterfaceSlice(strs []string) []any {
	result := make([]any, len(strs))
	for i, s := range strs {
		result[i] = s
	}
	return result
}
