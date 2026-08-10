package core

import (
	"fmt"
	"path/filepath"

	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/tools"
	"codenerd/internal/tools/codedom"
	"codenerd/internal/tools/core"
	"codenerd/internal/tools/research"
	"codenerd/internal/tools/shell"
	"codenerd/internal/types"
)

// GetToolRegistry returns the tool registry.
func (v *VirtualStore) GetToolRegistry() *ToolRegistry {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.toolRegistry
}

// GetModularTools returns the modular tools registry.
func (v *VirtualStore) GetModularTools() *tools.Registry {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.modularTools
}

// RegisterModularTool registers a modular tool that any agent can use.
func (v *VirtualStore) RegisterModularTool(tool *tools.Tool) error {
	v.mu.RLock()
	registry := v.modularTools
	v.mu.RUnlock()

	if registry == nil {
		return fmt.Errorf("modular tools registry not initialized")
	}

	return registry.Register(tool)
}

// HydrateModularTools registers all built-in modular tools.
// This should be called during session initialization.
func (v *VirtualStore) HydrateModularTools(searchers ...types.GroundedWebSearcher) error {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "HydrateModularTools")
	defer timer.Stop()

	v.mu.RLock()
	registry := v.modularTools
	v.mu.RUnlock()

	if registry == nil {
		return fmt.Errorf("modular tools registry not initialized")
	}

	logging.VirtualStore("Hydrating modular tools")

	// Get the global registry for tools.Global() access by session.Executor
	globalRegistry := tools.Global()

	// Install the nerd.md write guard on BOTH registries before any tool is
	// registered, so no window exists where a tool is callable but unguarded.
	v.installToolWriteGuard(registry, globalRegistry)

	// Register all core filesystem tools (to both registries)
	if err := core.RegisterAll(registry); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to register core tools: %v", err)
		return fmt.Errorf("failed to register core tools: %w", err)
	}
	if err := core.RegisterAll(globalRegistry); err != nil {
		logging.Get(logging.CategoryVirtualStore).Warn("Failed to register core tools to global registry: %v", err)
		// Don't return error - global registry may already have tools from previous call
	}

	// Register all shell execution tools (to both registries)
	if err := shell.RegisterAll(registry); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to register shell tools: %v", err)
		return fmt.Errorf("failed to register shell tools: %w", err)
	}
	if err := shell.RegisterAll(globalRegistry); err != nil {
		logging.Get(logging.CategoryVirtualStore).Warn("Failed to register shell tools to global registry: %v", err)
	}

	// Register all Code DOM tools (to both registries)
	if err := codedom.RegisterAll(registry); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to register codedom tools: %v", err)
		return fmt.Errorf("failed to register codedom tools: %w", err)
	}
	if err := codedom.RegisterAll(globalRegistry); err != nil {
		logging.Get(logging.CategoryVirtualStore).Warn("Failed to register codedom tools to global registry: %v", err)
	}

	// Register all research tools (to both registries)
	if err := research.RegisterAll(registry); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to register research tools: %v", err)
		return fmt.Errorf("failed to register research tools: %w", err)
	}
	if err := research.RegisterAll(globalRegistry); err != nil {
		logging.Get(logging.CategoryVirtualStore).Warn("Failed to register research tools to global registry: %v", err)
	}

	// Select at most the first non-nil searcher deterministically.
	var searcher types.GroundedWebSearcher
	for _, s := range searchers {
		if s != nil {
			searcher = s
			break
		}
	}

	// Conditionally register grounded_web_search on both registries with matching error/warn behavior.
	if err := func() error {
		_, err := research.RegisterGroundedWebSearchIfSupported(registry, searcher)
		return err
	}(); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to register grounded_web_search: %v", err)
		return fmt.Errorf("failed to register grounded_web_search: %w", err)
	}
	if _, err := research.RegisterGroundedWebSearchIfSupported(globalRegistry, searcher); err != nil {
		logging.Get(logging.CategoryVirtualStore).Warn("Failed to register grounded_web_search to global registry: %v", err)
	}

	logging.VirtualStore("Modular tools hydrated: %d tools in registry, %d tools in global", registry.Count(), globalRegistry.Count())
	return nil
}

// RegisterTool registers a tool with the registry and injects facts into the kernel.
func (v *VirtualStore) RegisterTool(name, command, shardAffinity string) error {
	v.mu.RLock()
	registry := v.toolRegistry
	v.mu.RUnlock()

	if registry == nil {
		logging.Get(logging.CategoryVirtualStore).Error("Cannot register tool %s: registry not initialized", name)
		return fmt.Errorf("tool registry not initialized")
	}

	logging.VirtualStore("Registering tool: name=%s, shardAffinity=%s", name, shardAffinity)
	if err := registry.RegisterTool(name, command, shardAffinity); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to register tool %s: %v", name, err)
		return err
	}

	logging.VirtualStoreDebug("Tool %s registered successfully", name)
	return nil
}

// GetToolsForShard returns all tools available for a specific shard type.
func (v *VirtualStore) GetToolsForShard(shardType string) []*Tool {
	v.mu.RLock()
	registry := v.toolRegistry
	v.mu.RUnlock()

	if registry == nil {
		return nil
	}

	return registry.GetToolsForShard(shardType)
}

// HydrateToolsFromDisk restores compiled tools from the .compiled directory
// and syncs from the Ouroboros executor if available.
// This should be called during session boot after the kernel is ready.
func (v *VirtualStore) HydrateToolsFromDisk(nerdDir string) error {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "HydrateToolsFromDisk")
	defer timer.Stop()

	v.mu.RLock()
	registry := v.toolRegistry
	kernel := v.kernel
	executor := v.toolExecutor
	v.mu.RUnlock()

	if registry == nil {
		logging.VirtualStoreDebug("HydrateToolsFromDisk: no registry, skipping")
		return nil
	}

	logging.VirtualStore("Hydrating tools from disk: %s", nerdDir)

	// Ensure kernel is set for fact injection
	if kernel != nil {
		registry.SetKernel(kernel)
	}

	// 1. Restore compiled tools from disk (.nerd/tools/.compiled/)
	compiledDir := filepath.Join(nerdDir, "tools", ".compiled")
	if err := registry.RestoreFromDisk(compiledDir); err != nil {
		logging.Get(logging.CategoryVirtualStore).Warn("Partial error restoring tools from disk: %v", err)
	} else {
		logging.VirtualStoreDebug("Tools restored from compiled directory")
	}

	// 2. Sync from Ouroboros if tool executor exists
	if executor != nil {
		if err := registry.SyncFromOuroboros(executor); err != nil {
			logging.Get(logging.CategoryVirtualStore).Warn("Failed to sync from Ouroboros: %v", err)
		} else {
			logging.VirtualStoreDebug("Tools synced from Ouroboros executor")
		}
	}

	return nil
}

// HydrateStaticTools loads static tool definitions into the registry.
// This is used to hydrate tools from available_tools.json at session boot.
func (v *VirtualStore) HydrateStaticTools(defs []StaticToolDef) error {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "HydrateStaticTools")
	defer timer.Stop()

	v.mu.RLock()
	registry := v.toolRegistry
	kernel := v.kernel
	v.mu.RUnlock()

	if registry == nil {
		logging.VirtualStoreDebug("HydrateStaticTools: no registry, skipping")
		return nil
	}

	logging.VirtualStore("Hydrating %d static tool definitions", len(defs))

	// Ensure kernel is set for fact injection
	if kernel != nil {
		registry.SetKernel(kernel)
	}

	if err := registry.RestoreFromStaticDefs(defs); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to hydrate static tools: %v", err)
		return err
	}

	logging.VirtualStoreDebug("Static tools hydrated successfully")
	return nil
}

// SetLocalDB sets the knowledge database for virtual predicate queries.
func (v *VirtualStore) SetLocalDB(db *store.LocalStore) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.localDB = db
	logging.VirtualStoreDebug("LocalDB (knowledge.db) attached for memory store queries")
}

// GetLocalDB returns the current knowledge database.
func (v *VirtualStore) GetLocalDB() *store.LocalStore {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.localDB
}

// SetLearningStore sets the learning database for shard autopoiesis.
func (v *VirtualStore) SetLearningStore(ls *store.LearningStore) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.learningStore = ls
	logging.VirtualStoreDebug("LearningStore attached for autopoiesis persistence")
}

// GetLearningStore returns the current learning database.
func (v *VirtualStore) GetLearningStore() *store.LearningStore {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.learningStore
}

// initConstitution initializes the constitutional safety rules.
