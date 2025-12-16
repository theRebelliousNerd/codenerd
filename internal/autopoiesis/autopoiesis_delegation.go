package autopoiesis

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// KERNEL-MEDIATED TOOL GENERATION
// =============================================================================
// Process tool generation requests delegated via Mangle policy.
// This enables campaign orchestration and other systems to trigger tool
// generation by asserting facts, without direct coupling.

// ProcessKernelDelegations queries for pending delegate_task(/tool_generator, ...)
// facts and processes each one by generating the requested tool.
// Returns the number of tools generated.
func (o *Orchestrator) ProcessKernelDelegations(ctx context.Context) (int, error) {
	logging.AutopoiesisDebug("Processing kernel delegations")

	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		logging.AutopoiesisDebug("No kernel attached, skipping delegation processing")
		return 0, nil // No kernel attached
	}

	// Query for pending tool generator delegations
	// delegate_task(/tool_generator, Capability, /pending)
	facts, err := kernel.QueryPredicate("delegate_task")
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to query delegate_task: %v", err)
		return 0, fmt.Errorf("failed to query delegate_task: %w", err)
	}
	logging.AutopoiesisDebug("Found %d delegate_task facts", len(facts))

	generated := 0
	for _, fact := range facts {
		// Check if this is a tool_generator delegation
		if len(fact.Args) < 3 {
			continue
		}

		// First arg should be the shard type (string "/tool_generator" or name constant)
		shardType, ok := fact.Args[0].(string)
		if !ok {
			continue
		}
		if shardType != "/tool_generator" && shardType != "tool_generator" {
			continue
		}

		// Second arg is the capability/tool name
		capability, ok := fact.Args[1].(string)
		if !ok {
			continue
		}

		// Third arg is the status - only process pending
		status, ok := fact.Args[2].(string)
		if !ok {
			continue
		}
		if status != "/pending" && status != "pending" {
			continue
		}

		logging.Autopoiesis("Processing kernel delegation: capability=%s", capability)

		// Generate the tool
		if err := o.generateToolFromDelegation(ctx, capability); err != nil {
			logging.Get(logging.CategoryAutopoiesis).Error("Tool generation failed for delegation %s: %v",
				capability, err)
			// Assert failure fact
			_ = o.assertToKernel("tool_generation_failed", capability, err.Error())
			continue
		}

		generated++
		logging.Autopoiesis("Kernel delegation processed successfully: capability=%s", capability)
	}

	if generated > 0 {
		logging.Autopoiesis("Processed %d kernel delegations", generated)
	}
	return generated, nil
}

// generateToolFromDelegation generates a tool for a kernel-delegated capability request.
func (o *Orchestrator) generateToolFromDelegation(ctx context.Context, capability string) error {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "generateToolFromDelegation")
	defer timer.Stop()

	logging.Autopoiesis("Generating tool from kernel delegation: %s", capability)

	// If tool already exists, treat delegation as satisfied.
	if o.toolGen != nil && o.toolGen.HasTool(capability) {
		logging.AutopoiesisDebug("Delegated tool already exists: %s", capability)
		_ = o.assertToKernel("tool_delegation_complete", capability, capability)
		return nil
	}

	// Create a tool need from the capability
	need := &ToolNeed{
		Name:       capability,
		Purpose:    fmt.Sprintf("Auto-generate tool for capability: %s", capability),
		Reasoning:  "kernel_delegation",
		Confidence: 1.0, // Kernel delegations are authoritative
		Priority:   1.0, // Kernel delegations are high priority
	}

	// Inject learnings from past tool generation into prompts
	o.RefreshLearningsContext()

	// Use the ouroboros loop to generate the tool
	logging.AutopoiesisDebug("Invoking Ouroboros loop for delegation: %s", capability)
	result := o.ouroboros.Execute(ctx, need)

	// Record generation learning (success or failure)
	o.recordGenerationLearning(ctx, need, result)

	if !result.Success {
		if result.Error != "" {
			logging.Get(logging.CategoryAutopoiesis).Error("Delegation tool generation failed: %s: %s",
				capability, result.Error)
			return fmt.Errorf("failed to generate tool %s: %s", capability, result.Error)
		}
		logging.Get(logging.CategoryAutopoiesis).Error("Delegation tool generation failed at stage %v: %s",
			result.Stage, capability)
		return fmt.Errorf("failed to generate tool %s at stage %v", capability, result.Stage)
	}

	// Assert success to kernel
	if result.ToolHandle != nil {
		logging.Autopoiesis("Tool registered from delegation: %s -> %s", capability, result.ToolHandle.Name)
		o.assertToolRegistered(result.ToolHandle)
		// Also assert delegation completion
		_ = o.assertToKernel("tool_delegation_complete", capability, result.ToolHandle.Name)

		// Update throttling counters on success.
		o.mu.Lock()
		o.toolsGenerated++
		o.lastToolGen = time.Now()
		o.mu.Unlock()
	}

	return nil
}

// StartKernelListener starts a background goroutine that periodically
// checks for kernel delegations and processes them.
// Returns a channel that will be closed when the listener stops.
func (o *Orchestrator) StartKernelListener(ctx context.Context, pollInterval time.Duration) <-chan struct{} {
	done := make(chan struct{})

	logging.Autopoiesis("Starting kernel delegation listener (poll interval: %v)", pollInterval)

	go func() {
		defer close(done)
		defer logging.Autopoiesis("Kernel delegation listener stopped")

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logging.AutopoiesisDebug("Kernel listener context cancelled")
				return
			case <-ticker.C:
				// Process any pending delegations
				if n, err := o.ProcessKernelDelegations(ctx); err != nil {
					logging.Get(logging.CategoryAutopoiesis).Error("Kernel delegation error: %v", err)
				} else if n > 0 {
					logging.Autopoiesis("Kernel listener processed %d delegations", n)
				}
			}
		}
	}()

	return done
}
