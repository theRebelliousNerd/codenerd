package core

import (
	"context"
	"fmt"

	"codenerd/internal/logging"
	"codenerd/internal/projectdoc"
	"codenerd/internal/tools"
)

// installToolWriteGuard makes nerd.md's forbidden-path protection and
// fail-closed shell-effect denial enforceable at the tool layer.
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
// gates cannot disagree. No baseline-aware exact-path authorization exists in
// this pass, so shell mutations are fail-closed denied. Remaining gap:
// immutable pre-task baseline + deterministic scope check for in-scope shell writes.
func (v *VirtualStore) toolWriteGuard() tools.WriteGuard {
	return func(_ context.Context, toolName string, args map[string]any) error {
		// Shell gate: lowest registry/VirtualStore chokepoint so direct
		// tools.Global().Execute without the session executor cannot bypass.
		if projectdoc.IsShellTool(toolName) {
			kind, _, err := projectdoc.ValidateShellToolInvocation(toolName, args)
			if err != nil {
				reason := fmt.Sprintf("%s effect=%s: %v", toolName, kind, err)
				logging.Get(logging.CategoryVirtualStore).Warn("tool-layer shell guard blocked %s effect=%s", toolName, kind)
				logging.Audit().SafetyCheck("shell_effect_guard", false, reason)
				return err
			}
		}
		if !projectdoc.IsWriteMutationTool(toolName) {
			return nil
		}
		target := projectdoc.TargetPath(args)
		if target == "" {
			return fmt.Errorf("blocked by nerd.md: %s has no recognized target path", toolName)
		}
		v.mu.RLock()
		kernel := v.kernel
		v.mu.RUnlock()
		if kernel == nil {
			return fmt.Errorf("blocked by nerd.md: write protection authority is unavailable for %s", target)
		}
		reason, forbidden, err := projectdoc.ForbiddenByKernel(kernel, target)
		if err != nil {
			reason := fmt.Sprintf("write protection could not be evaluated: %v", err)
			logging.Get(logging.CategoryVirtualStore).Warn("nerd.md blocked %s on %s because protection could not be evaluated: %v", toolName, target, err)
			logging.Audit().SafetyCheck("nerd.md_write_guard", false, reason)
			return fmt.Errorf("blocked by nerd.md: %s (%s)", target, reason)
		}
		if !forbidden {
			return nil
		}
		logging.Get(logging.CategoryVirtualStore).Warn("tool-layer guard blocked %s on %s: %s", toolName, target, reason)
		return fmt.Errorf("blocked by nerd.md: %s is write-protected (%s)", target, reason)
	}
}
