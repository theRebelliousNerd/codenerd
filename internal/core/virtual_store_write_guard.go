package core

import (
	"context"
	"fmt"

	"codenerd/internal/logging"
	"codenerd/internal/projectdoc"
	"codenerd/internal/tools"
)

// installToolWriteGuard makes nerd.md's forbidden-path protection enforceable
// at the tool layer, where it cannot be skipped.
//
// Before this, enforcement lived in exactly two callers —
// session.Executor.executeToolCall and VirtualStore.executeAction — and the
// tools themselves checked nothing. The registry is reachable process-globally
// through tools.Execute / tools.Global(), so any code path calling it directly
// wrote protected paths unchecked. That is not hypothetical: the codebase
// already suffered this once and documents it at virtual_store_routing.go:317
// ("a shard could write .nerd/config.json"), fixed then by adding the second
// caller-side gate rather than closing the class. codeNERD's own security
// review of internal/tools/core/file_ops.go raised it again.
//
// Defense in depth: the caller-side gates stay exactly where they are. This
// guard exists so a call site that forgets them — or one added later by someone
// who does not know they exist — still cannot write a protected path.
func (v *VirtualStore) installToolWriteGuard(registries ...*tools.Registry) {
	guard := v.toolWriteGuard()
	for _, r := range registries {
		if r != nil {
			r.SetWriteGuard(guard)
		}
	}
}

// toolWriteGuard builds the guard closure. Classification and target
// extraction come from internal/projectdoc so this guard and the caller-side
// gates cannot disagree about what counts as a write or where the target is.
func (v *VirtualStore) toolWriteGuard() tools.WriteGuard {
	return func(_ context.Context, toolName string, args map[string]any) error {
		if !projectdoc.IsWriteMutationTool(toolName) {
			return nil
		}

		target := projectdoc.TargetPath(args)
		if target == "" {
			return nil
		}

		v.mu.RLock()
		kernel := v.kernel
		v.mu.RUnlock()
		if kernel == nil {
			return nil
		}

		reason, forbidden, err := projectdoc.ForbiddenByKernel(kernel, target)
		if err != nil {
			// Fail OPEN, loudly — matching both caller-side gates verbatim.
			// A kernel query failure is not evidence the path is protected, and
			// turning every transient error into a blocked write would make the
			// agent unusable the moment the kernel hiccups. The warning is what
			// makes the degraded state visible.
			logging.Get(logging.CategoryVirtualStore).Warn(
				"nerd.md write protection could not be evaluated for %s at the tool layer (%v); allowing the write", target, err)
			return nil
		}
		if !forbidden {
			return nil
		}

		logging.Get(logging.CategoryVirtualStore).Warn(
			"tool-layer guard blocked %s on %s: %s", toolName, target, reason)
		return fmt.Errorf("blocked by nerd.md: %s is write-protected (%s)", target, reason)
	}
}
