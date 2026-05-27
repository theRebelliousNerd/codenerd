package core

import (
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/mangle/ast"
)

// DreamResult captures the speculative evaluation of a single action.
type DreamResult struct {
	ActionID       string
	Request        ActionRequest
	ProjectedFacts []Fact
	Unsafe         bool
	Reason         string
}

// DreamCache is a threadsafe cache of dream results keyed by action type + target.
// This avoids redundant kernel clones when the same action is simulated multiple times.
type DreamCache struct {
	mu      sync.RWMutex
	results map[string]DreamResult
}

const dreamCacheMaxSize = 256

// NewDreamCache creates an empty dream cache.
func NewDreamCache() *DreamCache {
	logging.DreamDebug("Creating new DreamCache")
	return &DreamCache{
		results: make(map[string]DreamResult),
	}
}

// cacheKey generates a deterministic key from action type and target.
func dreamCacheKey(req ActionRequest) string {
	return string(req.Type) + ":" + req.Target
}

// Store saves a result, evicting oldest entries if cache exceeds max size.
func (c *DreamCache) Store(key string, result DreamResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Simple eviction: if at capacity, clear half the cache
	if len(c.results) >= dreamCacheMaxSize {
		count := 0
		for k := range c.results {
			delete(c.results, k)
			count++
			if count >= dreamCacheMaxSize/2 {
				break
			}
		}
		logging.DreamDebug("DreamCache: evicted %d entries (capacity reached)", count)
	}
	c.results[key] = result
	logging.DreamDebug("DreamCache: stored result for %s (unsafe=%v, size=%d)", key, result.Unsafe, len(c.results))
}

// Get retrieves a cached result by key.
func (c *DreamCache) Get(key string) (DreamResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res, ok := c.results[key]
	if ok {
		logging.DreamDebug("DreamCache: cache hit for %s", key)
	}
	return res, ok
}

// Invalidate clears the entire cache. Call when kernel state changes
// (new facts asserted, policy updated) to prevent stale verdicts.
func (c *DreamCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	prevSize := len(c.results)
	c.results = make(map[string]DreamResult)
	if prevSize > 0 {
		logging.DreamDebug("DreamCache: invalidated %d entries", prevSize)
	}
}

// Dreamer simulates the impact of actions before execution.
type Dreamer struct {
	mu          sync.RWMutex
	kernel      *RealKernel
	cache       *DreamCache
	router      *DreamRouter      // Routes confirmed learnings to persistence stores
	planManager *DreamPlanManager // Manages dream plan lifecycle and execution state
}

// NewDreamer creates a Dreamer backed by the provided kernel.
func NewDreamer(kernel *RealKernel) *Dreamer {
	logging.Dream("Creating new Dreamer instance with cache")
	d := &Dreamer{
		kernel: kernel,
		cache:  NewDreamCache(),
	}
	d.assertCriticalPathFacts()
	return d
}

// SetDreamRouter connects a DreamRouter for persisting confirmed learnings.
func (d *Dreamer) SetDreamRouter(router *DreamRouter) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.router = router
	logging.Dream("Dreamer: DreamRouter connected for learning persistence")
}

// GetDreamRouter returns the Dreamer's learning router, if connected.
func (d *Dreamer) GetDreamRouter() *DreamRouter {
	if d == nil {
		return nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.router
}

// SetDreamPlanManager connects a DreamPlanManager for plan lifecycle tracking.
func (d *Dreamer) SetDreamPlanManager(pm *DreamPlanManager) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.planManager = pm
	logging.Dream("Dreamer: DreamPlanManager connected for plan lifecycle")
}

// GetDreamPlanManager returns the Dreamer's plan manager, if connected.
func (d *Dreamer) GetDreamPlanManager() *DreamPlanManager {
	if d == nil {
		return nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.planManager
}

// SetKernel updates the kernel reference (used when the virtual store swaps kernels).
func (d *Dreamer) SetKernel(kernel *RealKernel) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.kernel = kernel
	logging.DreamDebug("Dreamer: kernel reference updated")
	// Re-assert critical path facts into the new kernel
	d.assertCriticalPathFactsLocked()
}

// criticalPathPrefixes are the paths the Dreamer treats as catastrophic to delete.
// These are the source-of-truth for both the Go criticalPrefix() function and
// the Mangle critical_path_prefix() facts.
var criticalPathPrefixes = []string{
	".git",
	".nerd",
	"internal/mangle",
	"internal/core",
	"cmd/nerd",
}

// assertCriticalPathFacts populates the Mangle critical_path_prefix(Prefix) schema
// from the Go hardcoded constants, giving policy rules visibility into critical paths.
func (d *Dreamer) assertCriticalPathFacts() {
	d.mu.RLock()
	defer d.mu.RUnlock()
	d.assertCriticalPathFactsLocked()
}

// assertCriticalPathFactsLocked does the actual assertion without locking.
// Caller must hold d.mu.
func (d *Dreamer) assertCriticalPathFactsLocked() {
	if d.kernel == nil {
		return
	}
	for _, prefix := range criticalPathPrefixes {
		d.kernel.AssertWithoutEval(Fact{
			Predicate: "critical_path_prefix",
			Args:      []interface{}{prefix},
		})
	}
	d.kernel.Evaluate()
	logging.DreamDebug("Dreamer: asserted %d critical_path_prefix facts", len(criticalPathPrefixes))
}

func (d *Dreamer) getKernel() *RealKernel {
	if d == nil {
		return nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.kernel
}

// SimulateAction performs a speculative evaluation of a single action.
// It returns a DreamResult with any panic_state detections.
// Results are cached by action type + target to avoid redundant kernel clones.
func (d *Dreamer) SimulateAction(ctx context.Context, req ActionRequest) DreamResult {
	if ctx == nil {
		ctx = context.Background()
	}

	timer := logging.StartTimer(logging.CategoryDream, fmt.Sprintf("SimulateAction(%s)", req.Type))
	actionID := fmt.Sprintf("dream:%s:%d", req.Type, time.Now().UnixNano())
	logging.Dream("SimulateAction: starting simulation for %s (target=%s)", req.Type, req.Target)

	// Check cache first — avoid cloning the 277KB kernel for repeated actions
	cacheKey := dreamCacheKey(req)
	if d.cache != nil {
		if cached, ok := d.cache.Get(cacheKey); ok {
			// Return cached verdict with fresh ActionID
			cached.ActionID = actionID
			cached.Request = req
			logging.Dream("SimulateAction: returning cached verdict for %s (unsafe=%v)", cacheKey, cached.Unsafe)
			timer.Stop()
			return cached
		}
	}

	result := DreamResult{
		ActionID: actionID,
		Request:  req,
	}

	// No kernel available -> fail closed (safety system must not default-allow on internal failure).
	kernel := d.getKernel()
	if kernel == nil {
		result.Unsafe = true
		result.Reason = "dreamer kernel unavailable"
		logging.Get(logging.CategoryDream).Error("SimulateAction: %s", result.Reason)
		timer.Stop()
		return result
	}

	// Build projected facts for this action
	logging.DreamDebug("SimulateAction: projecting effects for action %s", actionID)
	projected := d.projectEffects(kernel, actionID, req)
	result.ProjectedFacts = projected
	logging.DreamDebug("SimulateAction: projected %d facts", len(projected))

	// Allow cancellation (Fail Closed)
	select {
	case <-ctx.Done():
		logging.Get(logging.CategoryDream).Warn("SimulateAction: context canceled for %s", actionID)
		result.Unsafe = true
		result.Reason = ctx.Err().Error()
		timer.Stop()
		return result
	default:
	}

	logging.DreamDebug("SimulateAction: evaluating projection for safety")
	unsafe, reason := d.evaluateProjection(kernel, actionID, projected)
	result.Unsafe = unsafe
	result.Reason = reason

	if unsafe {
		logging.Dream("SimulateAction: ACTION BLOCKED - %s on %s (reason: %s)", req.Type, req.Target, reason)
	} else {
		logging.Dream("SimulateAction: action %s deemed safe", req.Type)
	}

	// Cache the verdict
	if d.cache != nil {
		d.cache.Store(cacheKey, result)
	}

	timer.Stop()
	return result
}

// InvalidateCache clears the dream cache. Should be called when kernel state changes
// (e.g., new facts asserted, policy updated) to prevent stale safety verdicts.
func (d *Dreamer) InvalidateCache() {
	if d == nil || d.cache == nil {
		return
	}
	d.cache.Invalidate()
}

// evaluateProjection loads projected facts into a sandboxed kernel and queries panic_state.
func (d *Dreamer) evaluateProjection(kernel *RealKernel, actionID string, projected []Fact) (bool, string) {
	timer := logging.StartTimer(logging.CategoryDream, "evaluateProjection")
	logging.DreamDebug("evaluateProjection: cloning kernel for sandbox evaluation")
	clone := kernel.Clone()

	// Assert projected facts into the sandbox clone so panic_state rules can fire.
	// Without this, the clone has no projected_action facts for rules to match against.
	for _, fact := range projected {
		clone.AssertWithoutEval(fact)
	}

	logging.DreamDebug("evaluateProjection: evaluating sandbox kernel with %d projected facts", len(projected))
	if err := clone.Evaluate(); err != nil {
		// If evaluation fails, treat as unsafe to be conservative
		logging.Get(logging.CategoryDream).Error("evaluateProjection: sandbox evaluation failed: %v", err)
		timer.Stop()
		return true, fmt.Sprintf("dream evaluation failed: %v", err)
	}

	// Ensure panic_state is declared (Fail Closed contract)
	hasPanicState := false
	if clone.programInfo != nil && clone.programInfo.Decls != nil {
		for pred := range clone.programInfo.Decls {
			if pred.Symbol == "panic_state" {
				hasPanicState = true
				break
			}
		}
	}
	if !hasPanicState {
		timer.Stop()
		return true, "dream query failed: panic_state predicate not declared"
	}

	logging.DreamDebug("evaluateProjection: querying panic_state predicate")
	results, err := clone.Query("panic_state")
	if err != nil {
		// Conservative: if we can't query panic_state, we can't prove safety.
		logging.Get(logging.CategoryDream).Error("evaluateProjection: panic_state query failed: %v", err)
		timer.Stop()
		return true, fmt.Sprintf("dream query failed: %v", err)
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
			reason = types.ExtractString(fact.Args[1])
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
func (d *Dreamer) projectEffects(kernel *RealKernel, actionID string, req ActionRequest) []Fact {
	logging.DreamDebug("projectEffects: projecting effects for action %s (type=%s, target=%s)", actionID, req.Type, req.Target)

	path := strings.TrimSpace(req.Target)
	projected := []Fact{
		{
			Predicate: "projected_action",
			Args: []interface{}{
				actionID,
				MangleAtom("/" + string(req.Type)),
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
		projected = append(projected, d.codeGraphProjections(kernel, actionID, path)...)

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
		projected = append(projected, d.codeGraphProjections(kernel, actionID, path)...)

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
func (d *Dreamer) codeGraphProjections(kernel *RealKernel, actionID, path string) []Fact {
	logging.DreamDebug("codeGraphProjections: analyzing code graph for %s", path)

	if kernel == nil {
		logging.DreamDebug("codeGraphProjections: no kernel, skipping")
		return nil
	}

	// Lock kernel for direct store access to avoid O(N) allocations in QueryCallback
	kernel.mu.RLock()
	defer kernel.mu.RUnlock()

	programInfo := kernel.programInfo
	if programInfo == nil {
		return nil
	}

	// Find code_defines predicate
	var codeDefinesPred ast.PredicateSym
	foundDefines := false
	for pred := range programInfo.Decls {
		// code_defines is currently declared as /5 (File, Symbol, Type, StartLine, EndLine).
		// Be tolerant of older arities to avoid schema drift breakage.
		if pred.Symbol == "code_defines" && (pred.Arity == 5 || pred.Arity == 2) {
			codeDefinesPred = pred
			foundDefines = true
			break
		}
	}

	if !foundDefines {
		logging.DreamDebug("codeGraphProjections: code_defines predicate not found")
		return nil
	}

	symbolsInFile := make(map[string]bool)
	testSymbols := make(map[string]bool)

	// OPTIMIZATION: Direct store access avoids converting every fact to Go types.
	// We iterate ast.Atom directly and filter using fast checks.
	pathClean := filepath.Clean(path)

	err := kernel.store.GetFacts(ast.NewQuery(codeDefinesPred), func(a ast.Atom) error {
		if len(a.Args) < 2 {
			return nil
		}

		// Check file (arg 0)
		fileTerm := a.Args[0]
		fileStr, ok := fastTermToString(fileTerm)
		if !ok {
			return nil
		}

		// Check if file matches target path
		if filepath.Clean(fileStr) == pathClean {
			if sym, ok := fastTermToString(a.Args[1]); ok {
				symbolsInFile[sym] = true
			}
		}

		// Check if file is a test file
		if strings.Contains(fileStr, "_test.go") {
			if sym, ok := fastTermToString(a.Args[1]); ok {
				testSymbols[sym] = true
			}
		}
		return nil
	})

	if err != nil {
		logging.DreamDebug("codeGraphProjections: error querying code_defines: %v", err)
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

	// Find code_calls predicate
	var codeCallsPred ast.PredicateSym
	foundCalls := false
	for pred := range programInfo.Decls {
		if pred.Symbol == "code_calls" && pred.Arity == 2 {
			codeCallsPred = pred
			foundCalls = true
			break
		}
	}

	if !foundCalls {
		logging.DreamDebug("codeGraphProjections: code_calls predicate not found")
		return projected
	}

	// Stream code_calls to find impacts
	impactedTests := 0
	err = kernel.store.GetFacts(ast.NewQuery(codeCallsPred), func(a ast.Atom) error {
		if len(a.Args) != 2 {
			return nil
		}

		// Check callee (arg 1) first as filter
		calleeStr, ok := fastTermToString(a.Args[1])
		if !ok || !symbolsInFile[calleeStr] {
			return nil
		}

		// Check caller (arg 0)
		callerStr, ok := fastTermToString(a.Args[0])
		if !ok {
			return nil
		}

		// Check if caller is a test symbol
		if testSymbols[callerStr] {
			impactedTests++
			projected = append(projected, Fact{
				Predicate: "projected_fact",
				Args: []interface{}{
					actionID,
					MangleAtom("/impacts_test"),
					callerStr,
				},
			})
		}
		return nil
	})

	if err != nil {
		logging.DreamDebug("codeGraphProjections: error querying code_calls: %v", err)
	}

	if impactedTests > 0 {
		logging.DreamDebug("codeGraphProjections: detected %d impacted tests", impactedTests)
	}

	logging.DreamDebug("codeGraphProjections: returning %d total projections", len(projected))
	return projected
}

// isDangerousCommand flags obviously destructive commands.
// It normalizes whitespace to prevent bypass via extra spaces/tabs
// and checks comprehensive patterns including flag reordering.
func isDangerousCommand(cmd string) bool {
	// Normalize: lowercase + collapse whitespace runs to single space
	lc := strings.ToLower(cmd)
	// Replace tabs with spaces first
	lc = strings.ReplaceAll(lc, "\t", " ")
	// Collapse multiple spaces into one
	for strings.Contains(lc, "  ") {
		lc = strings.ReplaceAll(lc, "  ", " ")
	}
	lc = strings.TrimSpace(lc)

	dangerous := []string{
		// rm variants: -rf, -fr, -r -f, -f -r, plus --recursive --force
		"rm -rf",
		"rm -fr",
		"rm -r -f",
		"rm -f -r",
		"rm -r",
		"rm --recursive",
		"rm --force",
		// git reset
		"git reset --hard",
		// terraform
		"terraform destroy",
		// dd
		"dd if=",
		// format/wipe
		"mkfs.",
		"format c:",
		// chmod/chown -R on root
		"chmod -r 777 /",
		"chown -r",
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
	normalizedPath := filepath.ToSlash(filepath.Clean(path))
	for _, p := range criticalPathPrefixes {
		if strings.Contains(normalizedPath, p) {
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

// fastTermToString extracts a string from a BaseTerm if it's a name or string constant.
// This avoids interface conversion and allocation for non-matching types.
func fastTermToString(term ast.BaseTerm) (string, bool) {
	if c, ok := term.(ast.Constant); ok {
		if c.Type == ast.NameType || c.Type == ast.StringType {
			return c.Symbol, true
		}
	}
	return "", false
}
