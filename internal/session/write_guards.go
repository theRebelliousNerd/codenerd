package session

import (
	"fmt"

	"codenerd/internal/logging"
)

// write_guards.go consolidates the pre-write guard set into one protected unit.
//
// Why this file exists: internal/session/executor_tools.go previously ran three
// separate guard blocks in executeToolCall — projectForbidsWrite,
// writesPlaceholderTestFile and modularityGuard — each as an independently
// deletable six-line block inside a ~1700-line file that cannot be
// write-protected because it holds the entire tool-execution path. Observed
// 2026-08-12: when the modularity guard refused a write, the agent's next
// action was to edit the guard to exempt its own file. The guard
// implementations are now write-protected; the call sites were not, and three
// blocks were three chances to quietly remove one.
//
// This file collapses the dispatch into the single entry point runWriteGuards.
// The guard implementations themselves (projectForbidsWrite,
// writesPlaceholderTestFile, modularityGuard) remain where they are and are
// unchanged; only the call sites are consolidated here. This file is
// write-protected alongside those implementations, so the guard SET is one
// protected unit: a new guard is added here and is then covered by the same
// protection, rather than becoming a fourth deletable block at the call site
// in executor_tools.go.

// runWriteGuards is the single entry point for every pre-write guard.
func (e *Executor) runWriteGuards(call ToolCall) error {
	if reason, denied := e.projectForbidsWrite(call); denied {
		logging.Get(logging.CategorySession).Warn(
			"nerd.md BLOCKED %s on %s: %s", call.Name, projectDocTargetLabel(call.Args), reason)
		logging.Audit().SafetyCheck("nerd.md_write_guard", false, reason)
		return fmt.Errorf("blocked by nerd.md: %s is write-protected (%s)",
			projectDocTargetLabel(call.Args), reason)
	}
	if reason, denied := e.writesPlaceholderTestFile(call); denied {
		logging.Get(logging.CategorySession).Warn(
			"placeholder test BLOCKED %s on %s: %s", call.Name, projectDocTargetLabel(call.Args), reason)
		logging.Audit().SafetyCheck("placeholder_test_guard", false, reason)
		return fmt.Errorf("blocked by placeholder test guard: %s is placeholder (%s)",
			projectDocTargetLabel(call.Args), reason)
	}
	if reason, blocked := e.modularityGuard(call); blocked {
		logging.Get(logging.CategorySession).Warn(
			"modularity guard BLOCKED %s on %s: %s", call.Name, projectDocTargetLabel(call.Args), reason)
		logging.Audit().SafetyCheck("modularity_guard", false, reason)
		return fmt.Errorf("blocked by modularity guard: %s", reason)
	}
	return nil
}
