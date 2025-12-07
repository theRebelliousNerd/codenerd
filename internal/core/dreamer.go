package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// DreamResult captures the speculative evaluation of a single action.
type DreamResult struct {
	ActionID       string
	Request        ActionRequest
	ProjectedFacts []Fact
	Unsafe         bool
	Reason         string
}

// DreamCache is a threadsafe cache of dream results (action -> verdict).
type DreamCache struct {
	mu      sync.RWMutex
	results map[string]DreamResult
}

// NewDreamCache creates an empty dream cache.
func NewDreamCache() *DreamCache {
	return &DreamCache{
		results: make(map[string]DreamResult),
	}
}

// Store saves a result.
func (c *DreamCache) Store(result DreamResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[result.ActionID] = result
}

// Get retrieves a result by action ID.
func (c *DreamCache) Get(actionID string) (DreamResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res, ok := c.results[actionID]
	return res, ok
}

// Dreamer simulates the impact of actions before execution.
type Dreamer struct {
	kernel *RealKernel
}

// NewDreamer creates a Dreamer backed by the provided kernel.
func NewDreamer(kernel *RealKernel) *Dreamer {
	return &Dreamer{kernel: kernel}
}

// SetKernel updates the kernel reference (used when the virtual store swaps kernels).
func (d *Dreamer) SetKernel(kernel *RealKernel) {
	d.kernel = kernel
}

// SimulateAction performs a speculative evaluation of a single action.
// It returns a DreamResult with any panic_state detections.
func (d *Dreamer) SimulateAction(ctx context.Context, req ActionRequest) DreamResult {
	actionID := fmt.Sprintf("dream:%s:%d", req.Type, time.Now().UnixNano())
	result := DreamResult{
		ActionID: actionID,
		Request:  req,
	}

	// No kernel available -> nothing to simulate
	if d == nil || d.kernel == nil {
		return result
	}

	// Build projected facts for this action
	projected := d.projectEffects(actionID, req)
	result.ProjectedFacts = projected

	// Allow cancellation
	select {
	case <-ctx.Done():
		result.Reason = ctx.Err().Error()
		return result
	default:
	}

	unsafe, reason := d.evaluateProjection(actionID, projected)
	result.Unsafe = unsafe
	result.Reason = reason
	return result
}

// evaluateProjection loads projected facts into a sandboxed kernel and queries panic_state.
func (d *Dreamer) evaluateProjection(actionID string, projected []Fact) (bool, string) {
	clone := d.kernel.Clone()

	// Batch-assert projections for performance
	for _, fact := range projected {
		clone.AssertWithoutEval(fact)
	}

	if err := clone.Evaluate(); err != nil {
		// If evaluation fails, treat as unsafe to be conservative
		return true, fmt.Sprintf("dream evaluation failed: %v", err)
	}

	results, err := clone.Query("panic_state")
	if err != nil {
		return false, ""
	}

	for _, fact := range results {
		if len(fact.Args) == 0 {
			continue
		}
		id, ok := fact.Args[0].(string)
		if !ok || id != actionID {
			continue
		}
		reason := ""
		if len(fact.Args) > 1 {
			reason = fmt.Sprintf("%v", fact.Args[1])
		}
		return true, reason
	}

	return false, ""
}

// projectEffects converts an ActionRequest into a set of projected facts.
func (d *Dreamer) projectEffects(actionID string, req ActionRequest) []Fact {
	path := strings.TrimSpace(req.Target)
	projected := []Fact{
		{
			Predicate: "projected_action",
			Args: []interface{}{
				actionID,
				string(req.Type),
				path,
			},
		},
	}

	switch req.Type {
	case ActionDeleteFile:
		projected = append(projected, Fact{
			Predicate: "projected_fact",
			Args: []interface{}{
				actionID,
				MangleAtom("/file_missing"),
				path,
			},
		})
	case ActionWriteFile, ActionEditFile, ActionEditLines, ActionInsertLines, ActionDeleteLines:
		projected = append(projected, Fact{
			Predicate: "projected_fact",
			Args: []interface{}{
				actionID,
				MangleAtom("/modified"),
				path,
			},
		})
		projected = append(projected, Fact{
			Predicate: "projected_fact",
			Args: []interface{}{
				actionID,
				MangleAtom("/file_exists"),
				path,
			},
		})
	}

	return projected
}
