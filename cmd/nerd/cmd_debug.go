// Package main implements CLI debug features for codeNERD.
// This file provides the --verbose flag and execution tracing.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/persist/factsnap"
	"codenerd/internal/persist/snapshot"

	"github.com/spf13/cobra"
)

// Debug mode flags (defined here, registered in init())
var (
	dumpKernel bool // --dump-kernel: Export Mangle facts after execution
)

// DebugTracer provides verbose execution tracing for CLI debugging.
type DebugTracer struct {
	enabled   bool
	startTime time.Time
	events    []traceEvent
}

type traceEvent struct {
	timestamp time.Time
	category  string
	message   string
}

// NewDebugTracer creates a new tracer, enabled only if verbose is true.
func NewDebugTracer() *DebugTracer {
	return &DebugTracer{
		enabled:   verbose,
		startTime: time.Now(),
		events:    make([]traceEvent, 0, 100),
	}
}

// Trace logs a debug event if verbose mode is enabled.
func (t *DebugTracer) Trace(category, format string, args ...any) {
	if !t.enabled {
		return
	}
	msg := fmt.Sprintf(format, args...)
	t.events = append(t.events, traceEvent{
		timestamp: time.Now(),
		category:  category,
		message:   msg,
	})
	// Print immediately in verbose mode
	elapsed := time.Since(t.startTime).Milliseconds()
	fmt.Fprintf(os.Stderr, "[%6dms] [%-12s] %s\n", elapsed, category, msg)
}

// TracePhase logs a phase transition with timing.
func (t *DebugTracer) TracePhase(phase string) {
	t.Trace("PHASE", ">>> %s", phase)
}

// TraceAPI logs API-related events.
func (t *DebugTracer) TraceAPI(format string, args ...any) {
	t.Trace("API", format, args...)
}

// TraceShard logs shard-related events.
func (t *DebugTracer) TraceShard(format string, args ...any) {
	t.Trace("SHARD", format, args...)
}

// TraceKernel logs Mangle kernel events.
func (t *DebugTracer) TraceKernel(format string, args ...any) {
	t.Trace("KERNEL", format, args...)
}

// TraceContext logs context creation/cancellation.
func (t *DebugTracer) TraceContext(format string, args ...any) {
	t.Trace("CONTEXT", format, args...)
}

// TraceError logs errors.
func (t *DebugTracer) TraceError(format string, args ...any) {
	t.Trace("ERROR", format, args...)
}

// TraceTool logs tool invocation.
func (t *DebugTracer) TraceTool(format string, args ...any) {
	t.Trace("TOOL", format, args...)
}

// Summary prints a summary of the trace at the end.
func (t *DebugTracer) Summary() {
	if !t.enabled || len(t.events) == 0 {
		return
	}
	totalTime := time.Since(t.startTime)
	fmt.Fprintf(os.Stderr, "\n%s\n", strings.Repeat("─", 60))
	fmt.Fprintf(os.Stderr, "📊 Verbose Trace Summary\n")
	fmt.Fprintf(os.Stderr, "   Total time:  %v\n", totalTime.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "   Events:      %d\n", len(t.events))

	// Count by category
	cats := make(map[string]int)
	for _, e := range t.events {
		cats[e.category]++
	}
	for cat, count := range cats {
		fmt.Fprintf(os.Stderr, "   %-12s %d\n", cat+":", count)
	}
	fmt.Fprintf(os.Stderr, "%s\n", strings.Repeat("─", 60))
}

// registerDebugFlags registers debug-related flags on commands.
// Called from init() in main.go for relevant commands.
func registerDebugFlags(cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		cmd.Flags().BoolVar(&dumpKernel, "dump-kernel", false, "Export Mangle facts after execution")
	}
}

// VerboseContextDeadlineLogger wraps a context to log deadline info.
func VerboseContextDeadlineLogger(ctx context.Context, tracer *DebugTracer, name string) context.Context {
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		tracer.TraceContext("%s: deadline in %v", name, remaining.Round(time.Second))
	} else {
		tracer.TraceContext("%s: no deadline set", name)
	}
	return ctx
}

// PrintVerboseHeader prints debug header if verbose mode is enabled.
func PrintVerboseHeader() {
	if !verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "\n%s\n", strings.Repeat("═", 60))
	fmt.Fprintf(os.Stderr, "🔬 VERBOSE MODE ENABLED\n")
	fmt.Fprintf(os.Stderr, "   Showing detailed execution trace to stderr\n")
	fmt.Fprintf(os.Stderr, "%s\n\n", strings.Repeat("═", 60))
}

// ShouldDumpKernel returns true if kernel dump is requested.
func ShouldDumpKernel() bool {
	return dumpKernel
}

// DumpKernelSnapshot writes the kernel's base facts to .nerd/snapshots/ when
// --dump-kernel was passed, returning the snapshot path (empty when the flag
// is off).
//
// The flag and ShouldDumpKernel have existed since the debug commands were
// added, and nothing ever called them — "Export Mangle facts after execution"
// was a promise with no implementation behind it. internal/persist/factsnap
// was the other half of the same gap: a serializer with no production caller.
// This connects them.
//
// Base facts only. Re-importing derived conclusions would turn them into
// premises, which is why `nerd snapshot export` makes --derived opt-in too.
func DumpKernelSnapshot(kernel core.Kernel, workspace string) string {
	if !dumpKernel || kernel == nil {
		return ""
	}
	rk, ok := kernel.(*core.RealKernel)
	if !ok || rk == nil {
		return ""
	}

	facts := rk.GetBaseFacts()
	path, err := snapshot.Export(workspace, snapshot.DefaultName("kernel"), facts, factsnap.CodecGzip)
	if err != nil {
		// A debug aid must not fail the command whose output it was recording.
		logging.PersistWarn("--dump-kernel: snapshot export failed: %v", err)
		return ""
	}
	logging.Persist("--dump-kernel: wrote %d facts to %s", len(facts), path)
	return path
}

