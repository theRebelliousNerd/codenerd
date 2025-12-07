package core

import (
	"context"
	"fmt"
	"path/filepath"
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
		if prefix := criticalPrefix(path); prefix != "" {
			projected = append(projected, Fact{
				Predicate: "projected_fact",
				Args: []interface{}{
					actionID,
					MangleAtom("/critical_path_hit"),
					prefix,
				},
			})
		}
		projected = append(projected, d.codeGraphProjections(actionID, path)...)
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
		if prefix := criticalPrefix(path); prefix != "" {
			projected = append(projected, Fact{
				Predicate: "projected_fact",
				Args: []interface{}{
					actionID,
					MangleAtom("/critical_path_hit"),
					prefix,
				},
			})
		}
		projected = append(projected, d.codeGraphProjections(actionID, path)...)
	case ActionExecCmd:
		projected = append(projected, Fact{
			Predicate: "projected_fact",
			Args: []interface{}{
				actionID,
				MangleAtom("/exec_cmd"),
				path,
			},
		})
		if isDangerousCommand(path) {
			projected = append(projected, Fact{
				Predicate: "projected_fact",
				Args: []interface{}{
					actionID,
					MangleAtom("/exec_danger"),
					path,
				},
			})
		}
	}

	return projected
}

// codeGraphProjections emits projections based on the code graph for a file path:
// - touches_symbol(Symbol)
// - impacts_test(TestSymbol) when a touched symbol is called by a test
func (d *Dreamer) codeGraphProjections(actionID, path string) []Fact {
	if d == nil || d.kernel == nil {
		return nil
	}

	// Collect symbols defined in the target file
	defs, err := d.kernel.Query("code_defines")
	if err != nil || len(defs) == 0 {
		return nil
	}

	symbolsInFile := make(map[string]bool)
	symbolToFile := make(map[string]string)
	for _, def := range defs {
		if len(def.Args) < 2 {
			continue
		}
		file := toString(def.Args[0])
		sym := toString(def.Args[1])
		if file == "" || sym == "" {
			continue
		}
		symbolToFile[sym] = file
		if filepath.Clean(file) == filepath.Clean(path) {
			symbolsInFile[sym] = true
		}
	}

	if len(symbolsInFile) == 0 {
		return nil
	}

	// Build projected facts
	var projected []Fact
	for sym := range symbolsInFile {
		projected = append(projected, Fact{
			Predicate: "projected_fact",
			Args: []interface{}{
				actionID,
				MangleAtom("/touches_symbol"),
				sym,
			},
		})
	}

	// Find tests that call touched symbols
	callFacts, err := d.kernel.Query("code_calls")
	if err != nil || len(callFacts) == 0 {
		return projected
	}

	for _, cf := range callFacts {
		if len(cf.Args) < 2 {
			continue
		}
		caller := toString(cf.Args[0])
		callee := toString(cf.Args[1])
		if caller == "" || callee == "" {
			continue
		}
		if !symbolsInFile[callee] {
			continue
		}

		// Identify caller file to check if it's a test
		callerFile := symbolToFile[caller]
		if callerFile == "" {
			continue
		}
		if strings.Contains(callerFile, "_test.go") {
			projected = append(projected, Fact{
				Predicate: "projected_fact",
				Args: []interface{}{
					actionID,
					MangleAtom("/impacts_test"),
					caller,
				},
			})
		}
	}

	return projected
}

// isDangerousCommand flags obviously destructive commands.
func isDangerousCommand(cmd string) bool {
	lc := strings.ToLower(cmd)
	dangerous := []string{
		"rm -rf",
		"rm -r",
		"git reset --hard",
		"terraform destroy",
		"dd if=",
	}
	for _, token := range dangerous {
		if strings.Contains(lc, token) {
			return true
		}
	}
	return false
}

// criticalPrefix returns a critical prefix if the path falls under it.
func criticalPrefix(path string) string {
	prefixes := []string{
		".git",
		".nerd",
		"internal/mangle",
		"internal/core",
		"cmd/nerd",
	}
	for _, p := range prefixes {
		if strings.Contains(path, p) {
			return p
		}
	}
	return ""
}

// toString converts a fact argument to string, handling MangleAtom.
func toString(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v
	case MangleAtom:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
