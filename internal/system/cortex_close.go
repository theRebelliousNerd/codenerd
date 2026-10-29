package system

import (
	"codenerd/internal/broker"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools/research"
	"codenerd/internal/tools/shell"
	"context"
	"errors"
	"fmt"
	"time"
)

// closeStepTimeout bounds legacy Close steps so one-shot CLI (create/spawn)
// cannot hang forever after printing Result. Windows SQLite + system-shard
// shutdown has historically blocked process exit for minutes.
// The prompt compiler has cooperative cancellation and is joined explicitly.
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
	// Release the approval broker first: the last holder's release cancels
	// every request still awaiting a person, so no call stays blocked on a
	// decision while the resources it would resume into are torn down.
	if c.approvalsRelease != nil {
		if err := c.approvalsRelease(); err != nil {
			errs = append(errs, fmt.Errorf("approval broker: %w", err))
		}
		c.approvalsRelease = nil
		c.Approvals = nil
	}
	// Stop admitting delegated tasks and stop the ones running, then wait for
	// them: a worker is the holder of the kernel, stores and registries torn
	// down below. A worker that ignores the stop is named in the error rather
	// than reported as ended.
	if c.SessionSpawner != nil {
		spawner := c.SessionSpawner
		if err := runCloseStep("SessionSpawner.Shutdown", closeStepTimeout+time.Second, func() error {
			ctx, cancel := context.WithTimeout(context.Background(), closeStepTimeout)
			defer cancel()
			_, err := spawner.Shutdown(ctx)
			return err
		}); err != nil {
			errs = append(errs, err)
		}
	}
	// Stop generated admission and join execution, recording and refinement
	// before releasing any of their kernel, prompt or persistence dependencies.
	if c.generatedExecutor != nil {
		if err := c.generatedExecutor.Close(); err != nil {
			errs = append(errs, err)
		}
		c.generatedExecutor = nil
	}

	// Stop maintenance BEFORE closing LocalDB — the loop calls
	// LocalDB.MaintenanceCleanup and can block process exit if left running.
	// Wait briefly for the goroutine so an in-flight cycle (if any) finishes
	// before LocalDB.Close; cancel alone is not enough on Windows SQLite.
	c.stopMaintenanceSchedule(maintenanceStopWait)

	// Flush usage before anything heavier can fail or time out. Track only arms
	// a debounce timer, so without this the last few turns of every session were
	// dropped on exit.
	if c.UsageTracker != nil {
		if err := runCloseStep("UsageTracker.Close", closeStepTimeout, c.UsageTracker.Close); err != nil {
			errs = append(errs, err)
		}
		c.UsageTracker = nil
	}

	// Stop on-demand activation BEFORE draining shards: a trigger landing
	// mid-shutdown must not spawn a shard into a queue that is stopping.
	if c.onDemandStop != nil {
		if err := runCloseStep("OnDemandWatcher.Stop", closeStepTimeout, func() error {
			c.onDemandStop()
			return nil
		}); err != nil {
			errs = append(errs, err)
		}
		c.onDemandStop = nil
	}

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

	if c.ouroborosCancel != nil {
		c.ouroborosCancel()
		c.ouroborosCancel = nil
	}
	if c.ouroborosDone != nil {
		select {
		case <-c.ouroborosDone:
		case <-time.After(closeStepTimeout):
			err := fmt.Errorf("Ouroboros listener and queue shutdown timed out after %v", closeStepTimeout)
			logging.Get(logging.CategorySession).Warn("Cortex.Close: %v; continuing shutdown", err)
			errs = append(errs, err)
		}
		c.ouroborosDone = nil
	}

	if c.ToolStore != nil {
		if err := runCloseStep("ToolStore.Close", closeStepTimeout, c.ToolStore.Close); err != nil {
			errs = append(errs, err)
		}
		c.ToolStore = nil
	}

	if c.Executor != nil {
		shell.ClearCheckExecutor(c.Executor)
	}

	// Before the interactive manager: admitted research reads must drain, and
	// the headless Chrome this runtime launched must exit, while the kernel
	// that retires their observations is still alive.
	if c.researchBrowser != nil {
		researchBrowser := c.researchBrowser
		if err := runCloseStep("ResearchBrowser.Close", closeStepTimeout, func() error {
			ctx, cancel := context.WithTimeout(context.Background(), closeStepTimeout)
			defer cancel()
			return researchBrowser.Close(ctx)
		}); err != nil {
			errs = append(errs, err)
		}
		c.researchBrowser = nil
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

	// Before the JIT compiler: the evolver holds two SQLite handles under
	// .nerd/ and a cycle may still be draining after stopMaintenanceSchedule
	// returned on its timeout, which would keep those files locked on Windows.
	//
	// PromptEvolver.Close owns its ordering: it stops admission, cancels the
	// running cycle, drains accepted turn records (bounded by its configured
	// drain timeout, abandoned records reported as an error), joins the cycle,
	// and only then closes its stores. The step timeout therefore covers the
	// drain plus the ordinary step bound; if it still expires, the timeout is
	// an incomplete-shutdown result and the evolver finishes its ordered close
	// in the background -- no recorder can reach a closed store either way.
	if c.PromptEvolver != nil {
		evolverCloseTimeout := c.PromptEvolver.DrainTimeout() + closeStepTimeout
		if err := runCloseStep("PromptEvolver.Close", evolverCloseTimeout, c.PromptEvolver.Close); err != nil {
			errs = append(errs, err)
		}
		c.PromptEvolver = nil
	}

	if c.ContextFeedback != nil {
		if err := runCloseStep("ContextFeedback.Close", closeStepTimeout, c.ContextFeedback.Close); err != nil {
			errs = append(errs, err)
		}
		c.ContextFeedback = nil
	}

	if c.JITCompiler != nil {
		// Close cancels and joins compilation before closing its databases.
		// A timer cannot cancel SQLite cleanup: abandoning it leaves the corpus
		// locked on Windows and races teardown of the stores used by compilation.
		if err := c.JITCompiler.Close(); err != nil {
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
		if err := closePerceptionLayer(closeStepTimeout); err != nil {
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

	// The two meter logs are process-wide singletons installed at boot, so they
	// are released here rather than by whoever installed them. Cortex.Close is
	// where the Windows handle problem this comment block opens with gets
	// solved for every other file; these were simply never added to the list.
	//
	// Detaching also stops receipts and selections being written to a file the
	// caller has finished with, which matters for a one-shot CLI that boots,
	// works, closes, and expects the workspace to be quiet afterwards.
	// By identity, not unconditionally. Several Cortex instances can be live in
	// one process — cortexCache is keyed per workspace and provider — and both
	// of these are process singletons, so a blanket detach lets the one shutting
	// down close the log another is still writing to. A closed sink drops
	// records silently, so that second agent would go on working with its
	// metering switched off and nothing anywhere to say so.
	if c.meterSink != nil {
		if err := runCloseStep("Broker.DetachExtraSink", closeStepTimeout, func() error {
			_, detachErr := broker.DetachExtraSink(c.meterSink)
			return detachErr
		}); err != nil {
			errs = append(errs, err)
		}
		c.meterSink = nil
	}
	if c.couseLog != nil {
		if err := runCloseStep("CoUse.DetachLog", closeStepTimeout, func() error {
			_, detachErr := prompt.CoUse().DetachLog(c.couseLog)
			return detachErr
		}); err != nil {
			errs = append(errs, err)
		}
		c.couseLog = nil
	}
	c.closeSessionLog()

	// Evict from the keyed cache so a future GetOrBootCortex with the same
	// (workspace, provider, apiKey, model, disabled-shard set) tuple boots a fresh instance
	// instead of handing back this torn-down one.
	if c.cortexKey != "" {
		evictCortexByKey(c.cortexKey)
		c.cortexKey = ""
	}

	// The process-wide tool registry is governed by this cortex's store until
	// it closes, and by identity, as the meter logs above: another live
	// cortex's binding is left alone.
	if c.VirtualStore != nil {
		c.VirtualStore.ReleaseGlobalTools()
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// closePerceptionLayer joins the learning consolidation worker, then closes
// the perception layer under stepTimeout. The worker persists through the
// classifier and the stores closed after this step, and its drain waits for
// the in-flight critic call and embedding. That wait is budgeted inside
// ShutdownSharedTaxonomy (llm_timeouts.per_call_timeout plus
// embedding.request_timeout) and always ends with the worker joined, so it
// runs untimed: under stepTimeout a long drain would be abandoned while the
// worker went on using what Close tears down next. A drain that ran out of its
// own budget is reported, not swallowed. ClosePerceptionLayer joins the worker
// too; after this it is a no-op.
func closePerceptionLayer(stepTimeout time.Duration) error {
	var errs []error
	if err := perception.ShutdownSharedTaxonomy(); err != nil {
		errs = append(errs, fmt.Errorf("consolidation worker: %w", err))
	}
	if err := runCloseStep("ClosePerceptionLayer", stepTimeout, perception.ClosePerceptionLayer); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
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
