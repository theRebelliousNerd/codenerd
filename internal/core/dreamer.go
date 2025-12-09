package core

import (
	"codenerd/internal/logging"
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
	logging.DreamDebug("Creating new DreamCache")
	return &DreamCache{
		results: make(map[string]DreamResult),
	}
}

// Store saves a result.
func (c *DreamCache) Store(result DreamResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[result.ActionID] = result
	logging.DreamDebug("DreamCache: stored result for action %s (unsafe=%v)", result.ActionID, result.Unsafe)
}

// Get retrieves a result by action ID.
func (c *DreamCache) Get(actionID string) (DreamResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res, ok := c.results[actionID]
	if ok {
		logging.DreamDebug("DreamCache: cache hit for action %s", actionID)
	} else {
		logging.DreamDebug("DreamCache: cache miss for action %s", actionID)
	}
	return res, ok
}

// Dreamer simulates the impact of actions before execution.
type Dreamer struct {
	kernel *RealKernel
}

// NewDreamer creates a Dreamer backed by the provided kernel.
func NewDreamer(kernel *RealKernel) *Dreamer {
	logging.Dream("Creating new Dreamer instance")
	return &Dreamer{kernel: kernel}
}

// SetKernel updates the kernel reference (used when the virtual store swaps kernels).
func (d *Dreamer) SetKernel(kernel *RealKernel) {
	d.kernel = kernel
	logging.DreamDebug("Dreamer: kernel reference updated")
}

// SimulateAction performs a speculative evaluation of a single action.
// It returns a DreamResult with any panic_state detections.
func (d *Dreamer) SimulateAction(ctx context.Context, req ActionRequest) DreamResult {
	timer := logging.StartTimer(logging.CategoryDream, fmt.Sprintf("SimulateAction(%s)", req.Type))
	actionID := fmt.Sprintf("dream:%s:%d", req.Type, time.Now().UnixNano())
	logging.Dream("SimulateAction: starting simulation for %s (target=%s)", req.Type, req.Target)
	logging.DreamDebug("SimulateAction: actionID=%s", actionID)

	result := DreamResult{
		ActionID: actionID,
		Request:  req,
	}

	// No kernel available -> nothing to simulate
	if d == nil || d.kernel == nil {
		logging.DreamDebug("SimulateAction: no kernel available, returning safe (no simulation)")
		timer.Stop()
		return result
	}

	// Build projected facts for this action
	logging.DreamDebug("SimulateAction: projecting effects for action %s", actionID)
	projected := d.projectEffects(actionID, req)
	result.ProjectedFacts = projected
	logging.DreamDebug("SimulateAction: projected %d facts", len(projected))

	// Allow cancellation
	select {
	case <-ctx.Done():
		logging.Get(logging.CategoryDream).Warn("SimulateAction: context canceled for %s", actionID)
		result.Reason = ctx.Err().Error()
		timer.Stop()
		return result
	default:
	}

	logging.DreamDebug("SimulateAction: evaluating projection for safety")
	unsafe, reason := d.evaluateProjection(actionID, projected)
	result.Unsafe = unsafe
	result.Reason = reason

	if unsafe {
		logging.Dream("SimulateAction: ACTION BLOCKED - %s on %s (reason: %s)", req.Type, req.Target, reason)
	} else {
		logging.Dream("SimulateAction: action %s deemed safe", req.Type)
	}

	timer.Stop()
	return result
}

// evaluateProjection loads projected facts into a sandboxed kernel and queries panic_state.
func (d *Dreamer) evaluateProjection(actionID string, projected []Fact) (bool, string) {
	timer := logging.StartTimer(logging.CategoryDream, "evaluateProjection")
	logging.DreamDebug("evaluateProjection: cloning kernel for sandbox evaluation")
	clone := d.kernel.Clone()

	// Batch-assert projections for performance
	logging.DreamDebug("evaluateProjection: asserting %d projected facts", len(projected))
	for _, fact := range projected {
		clone.AssertWithoutEval(fact)
	}

	logging.DreamDebug("evaluateProjection: evaluating sandbox kernel")
	if err := clone.Evaluate(); err != nil {
		// If evaluation fails, treat as unsafe to be conservative
		logging.Get(logging.CategoryDream).Error("evaluateProjection: sandbox evaluation failed: %v", err)
		timer.Stop()
		return true, fmt.Sprintf("dream evaluation failed: %v", err)
	}

	logging.DreamDebug("evaluateProjection: querying panic_state predicate")
	results, err := clone.Query("panic_state")
	if err != nil {
		logging.DreamDebug("evaluateProjection: panic_state query failed: %v", err)
		timer.Stop()
		return false, ""
	}

	logging.DreamDebug("evaluateProjection: found %d panic_state results", len(results))
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
		logging.Dream("evaluateProjection: UNSAFE - panic_state detected for %s: %s", actionID, reason)
		timer.Stop()
		return true, reason
	}

	logging.DreamDebug("evaluateProjection: no panic_state detected, action is safe")
	timer.Stop()
	return false, ""
}

// projectEffects converts an ActionRequest into a set of projected facts.
func (d *Dreamer) projectEffects(actionID string, req ActionRequest) []Fact {
	logging.DreamDebug("projectEffects: projecting effects for action %s (type=%s, target=%s)", actionID, req.Type, req.Target)

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
		logging.DreamDebug("projectEffects: projecting delete_file effects for %s", path)
		projected = append(projected, Fact{
			Predicate: "projected_fact",
			Args: []interface{}{
				actionID,
				MangleAtom("/file_missing"),
				path,
			},
		})
		if prefix := criticalPrefix(path); prefix != "" {
			logging.DreamDebug("projectEffects: critical path detected: %s", prefix)
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
		logging.DreamDebug("projectEffects: projecting file modification effects for %s", path)
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
			logging.DreamDebug("projectEffects: critical path detected: %s", prefix)
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
		logging.DreamDebug("projectEffects: projecting exec_cmd effects for command: %s", path)
		projected = append(projected, Fact{
			Predicate: "projected_fact",
			Args: []interface{}{
				actionID,
				MangleAtom("/exec_cmd"),
				path,
			},
		})
		if isDangerousCommand(path) {
			logging.Dream("projectEffects: DANGEROUS COMMAND detected: %s", path)
			projected = append(projected, Fact{
				Predicate: "projected_fact",
				Args: []interface{}{
					actionID,
					MangleAtom("/exec_danger"),
					path,
				},
			})
		}

	default:
		logging.DreamDebug("projectEffects: no special projections for action type %s", req.Type)
	}

	logging.DreamDebug("projectEffects: generated %d projected facts", len(projected))
	return projected
}

// codeGraphProjections emits projections based on the code graph for a file path:
// - touches_symbol(Symbol)
// - impacts_test(TestSymbol) when a touched symbol is called by a test
func (d *Dreamer) codeGraphProjections(actionID, path string) []Fact {
	logging.DreamDebug("codeGraphProjections: analyzing code graph for %s", path)

	if d == nil || d.kernel == nil {
		logging.DreamDebug("codeGraphProjections: no kernel, skipping")
		return nil
	}

	// Collect symbols defined in the target file
	defs, err := d.kernel.Query("code_defines")
	if err != nil || len(defs) == 0 {
		logging.DreamDebug("codeGraphProjections: no code_defines found (err=%v, count=%d)", err, len(defs))
		return nil
	}
	logging.DreamDebug("codeGraphProjections: found %d code_defines facts", len(defs))

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
		logging.DreamDebug("codeGraphProjections: no symbols found in file %s", path)
		return nil
	}

	logging.DreamDebug("codeGraphProjections: found %d symbols in file %s", len(symbolsInFile), path)

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
		logging.DreamDebug("codeGraphProjections: no code_calls found, returning %d symbol projections", len(projected))
		return projected
	}
	logging.DreamDebug("codeGraphProjections: analyzing %d code_calls for test impacts", len(callFacts))

	impactedTests := 0
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
			impactedTests++
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

	if impactedTests > 0 {
		logging.DreamDebug("codeGraphProjections: detected %d impacted tests", impactedTests)
	}

	logging.DreamDebug("codeGraphProjections: returning %d total projections", len(projected))
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
