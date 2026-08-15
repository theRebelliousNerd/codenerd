package transparency

import (
	"sync/atomic"
	"time"

	"codenerd/internal/types"
)

// Process-wide transparency handles.
//
// Most producers receive their transparency handle by injection: chat boot
// calls ShardManager.SetTransparencyManager and VirtualStore.SetGlassBoxBus.
// Two important producers cannot be reached that way:
//
//   - Constitutional deny sites (internal/core VirtualStore routing). They are
//     spread across the routing path, run on shard goroutines, and hold no
//     manager reference. Before this indirection a denial reached the audit log
//     and nothing else — no operator surface listed it, so `/transparency`
//     reported "Recent Safety Blocks" that could never appear.
//   - The JIT prompt compiler (internal/prompt). It is constructed at boot
//     BEFORE the Glass Box bus exists and exposes no setter, so a
//     construction-time wire is impossible; a call-time lookup is not.
//
// The repo already uses this shape for the same reason: logging.Audit() is a
// process-wide sink called directly from the same deny sites.
//
// Registration is first-writer-wins (boot constructs exactly one manager and
// one bus) and can always be overridden explicitly with SetProcessManager /
// SetProcessBus, which is what tests use. Producers that DO get injection keep
// using it — this is a fallback channel, not the primary one.
var (
	processManager atomic.Pointer[TransparencyManager]
	processBus     atomic.Pointer[GlassBoxEventBus]
)

// adoptProcessManager installs tm as the process manager if none is set.
func adoptProcessManager(tm *TransparencyManager) {
	processManager.CompareAndSwap(nil, tm)
}

// adoptProcessBus installs bus as the process Glass Box bus if none is set.
func adoptProcessBus(bus *GlassBoxEventBus) {
	processBus.CompareAndSwap(nil, bus)
}

// SetProcessManager overrides the process-wide manager. Pass nil to clear.
// Returns the previous value so a test can restore it.
func SetProcessManager(tm *TransparencyManager) *TransparencyManager {
	return processManager.Swap(tm)
}

// ProcessManager returns the process-wide manager, or nil.
func ProcessManager() *TransparencyManager {
	return processManager.Load()
}

// SetProcessBus overrides the process-wide Glass Box bus. Pass nil to clear.
// Returns the previous value so a test can restore it.
func SetProcessBus(bus *GlassBoxEventBus) *GlassBoxEventBus {
	return processBus.Swap(bus)
}

// ProcessBus returns the process-wide Glass Box bus, or nil.
func ProcessBus() *GlassBoxEventBus {
	return processBus.Load()
}

// ReportDeny records a constitutional / policy denial on the process manager
// and mirrors it onto the Glass Box control stream.
//
// Call this from every path that refuses an action. It is a no-op when no
// manager is registered or transparency is off, never blocks, and never
// influences the verdict — the deny has already happened when this runs.
//
// action is the action verb (e.g. "/exec_cmd"), target the file/resource, and
// rule the policy identity that refused ("permitted", "checkConstitution", …).
func ReportDeny(action, target, rule string) *SafetyViolation {
	tm := processManager.Load()
	violation := tm.ReportSafetyViolation(action, target, rule)

	// The Glass Box line is independent of the manager toggle: a refusal is
	// exactly the kind of milestone the debug stream exists to show.
	if bus := processBus.Load(); bus != nil {
		summary := "DENIED " + action
		if target != "" {
			summary += " " + target
		}
		details := "rule: " + rule
		if violation != nil {
			details = violation.Summary + " (" + details + ")"
		}
		bus.EmitImmediate(GlassBoxEvent{
			Timestamp: time.Now(),
			Category:  CategoryControl,
			Summary:   summary,
			Details:   details,
			Source:    rule,
		})
	}

	return violation
}

// JITExplainEnabled reports whether JIT atom-selection telemetry is wanted.
// Producers should check this before doing any work to build the detail text.
func JITExplainEnabled() bool {
	tm := processManager.Load()
	if tm == nil {
		return false
	}
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.enabled && tm.config != nil && tm.config.JITExplain
}

// EmitJIT publishes a CategoryJIT Glass Box event describing prompt-atom
// selection. No-op unless JITExplain is on and a bus is registered.
func EmitJIT(summary, details, source string, dur time.Duration) {
	if !JITExplainEnabled() {
		return
	}
	bus := processBus.Load()
	if bus == nil {
		return
	}
	bus.EmitImmediate(GlassBoxEvent{
		Timestamp: time.Now(),
		Category:  CategoryJIT,
		Summary:   summary,
		Details:   details,
		Source:    source,
		Duration:  dur,
	})
}

// RecordOperation files a post-operation summary on the process manager.
// Producers holding an injected types.TransparencyManager should call that
// instead; this exists for producers with no handle.
func RecordOperation(rec types.OperationRecord) {
	processManager.Load().RecordOperation(rec)
}
