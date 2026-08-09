package system

import (
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/tools/research"
	"context"
	"errors"
	"fmt"
	"time"
)

// closeStepTimeout bounds each Close step so one-shot CLI (create/spawn)
// cannot hang forever after printing Result. Windows SQLite + system-shard
// shutdown has historically blocked process exit for minutes.
const closeStepTimeout = 8 * time.Second

// Close releases resources held by a Cortex instance.
//
// This is especially important in tests on Windows, where open SQLite handles
// prevent TempDir cleanup (e.g. learned_patterns.db from the Perception layer).
func (c *Cortex) Close() error {
	if c == nil {
		return nil
	}
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	var errs []error

	// Stop maintenance BEFORE closing LocalDB — the loop calls
	// LocalDB.MaintenanceCleanup and can block process exit if left running.
	// Wait briefly for the goroutine so an in-flight cycle (if any) finishes
	// before LocalDB.Close; cancel alone is not enough on Windows SQLite.
	c.stopMaintenanceSchedule(maintenanceStopWait)

	if c.ShardManager != nil {
		shardManager := c.ShardManager
		// Stop admission before workers so the queue cannot start another shard
		// while shutdown is draining active agents.
		if err := runCloseStep("ShardManager.StopSpawnQueue", closeStepTimeout, func() error {
			shardManager.StopSpawnQueue(5 * time.Second)
			return nil
		}); err != nil {
			errs = append(errs, err)
		}
		if err := runCloseStep("ShardManager.StopAll", closeStepTimeout, func() error {
			shardManager.StopAll()
			return nil
		}); err != nil {
			errs = append(errs, err)
		}
		c.ShardManager = nil
	}

	if c.mcpCancel != nil {
		c.mcpCancel()
		c.mcpCancel = nil
	}
	if c.mcpBridge != nil {
		if err := runCloseStep("MCPBridge.Close", closeStepTimeout, c.mcpBridge.Close); err != nil {
			errs = append(errs, err)
		}
		c.mcpBridge = nil
	}
	if c.mcpDone != nil {
		select {
		case <-c.mcpDone:
		case <-time.After(closeStepTimeout):
			err := fmt.Errorf("MCP.ConnectAll timed out after %v", closeStepTimeout)
			logging.Get(logging.CategorySession).Warn("Cortex.Close: %v; continuing shutdown", err)
			errs = append(errs, err)
		}
		c.mcpDone = nil
	}

	if c.BrowserManager != nil {
		browserManager := c.BrowserManager
		research.ClearBrowserManager(browserManager)
		if err := runCloseStep("BrowserManager.Shutdown", closeStepTimeout, func() error {
			ctx, cancel := context.WithTimeout(context.Background(), closeStepTimeout)
			defer cancel()
			return browserManager.Shutdown(ctx)
		}); err != nil {
			errs = append(errs, err)
		}
		c.BrowserManager = nil
	}

	if c.JITCompiler != nil {
		if err := runCloseStep("JITCompiler.Close", closeStepTimeout, c.JITCompiler.Close); err != nil {
			errs = append(errs, err)
		}
		c.JITCompiler = nil
	}

	if closer, ok := c.EmbeddingEngine.(interface{ Close() error }); ok {
		if err := runCloseStep("EmbeddingEngine.Close", closeStepTimeout, closer.Close); err != nil {
			errs = append(errs, err)
		}
	}
	c.EmbeddingEngine = nil

	if c.perceptionInitialized {
		if err := runCloseStep("ClosePerceptionLayer", closeStepTimeout, perception.ClosePerceptionLayer); err != nil {
			errs = append(errs, err)
		}
		c.perceptionInitialized = false
	}

	if c.LocalDB != nil {
		if err := runCloseStep("LocalDB.Close", closeStepTimeout, c.LocalDB.Close); err != nil {
			errs = append(errs, err)
		}
		c.LocalDB = nil
	}

	if c.LearningStore != nil {
		if err := runCloseStep("LearningStore.Close", closeStepTimeout, c.LearningStore.Close); err != nil {
			errs = append(errs, err)
		}
		c.LearningStore = nil
	}

	// Evict from the keyed cache so a future GetOrBootCortex with the same
	// (workspace, provider, apiKey, model, disabled-shard set) tuple boots a fresh instance
	// instead of handing back this torn-down one.
	if c.cortexKey != "" {
		evictCortexByKey(c.cortexKey)
		c.cortexKey = ""
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// runCloseStep runs fn with a hard timeout. On timeout it logs and returns
// a timeout error so Close can continue tearing down remaining resources.
func runCloseStep(name string, timeout time.Duration, fn func() error) error {
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("%s panicked: %v", name, r)
			}
		}()
		done <- fn()
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		logging.Get(logging.CategorySession).Warn(
			"Cortex.Close: %s timed out after %v; continuing shutdown", name, timeout,
		)
		return fmt.Errorf("%s timed out after %v", name, timeout)
	}
}
