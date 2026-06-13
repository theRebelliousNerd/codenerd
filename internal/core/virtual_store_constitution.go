package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (v *VirtualStore) initConstitution() {
	v.constitution = []ConstitutionalRule{
		{
			Name:        "no_destructive_commands",
			Description: "Prevent destructive shell commands",
			Check: func(req ActionRequest) error {
				if req.Type != ActionExecCmd {
					return nil
				}
				cmd := strings.ToLower(req.Target)
				forbidden := []string{"rm -rf", "mkfs", "dd if=", ":(){", "chmod 777"}
				for _, f := range forbidden {
					if strings.Contains(cmd, f) {
						return fmt.Errorf("constitutional violation: destructive command '%s' blocked", f)
					}
				}
				return nil
			},
		},
		{
			Name:        "no_secret_exfiltration",
			Description: "Prevent exfiltration of secrets",
			Check: func(req ActionRequest) error {
				payload := fmt.Sprintf("%v", req.Payload)
				secrets := []string{".env", "credentials", "secret", "api_key", "password"}
				dangerous := []string{"curl", "wget", "nc ", "netcat"}
				hasSecret := false
				hasDangerous := false
				for _, s := range secrets {
					if strings.Contains(strings.ToLower(payload), s) {
						hasSecret = true
						break
					}
				}
				for _, d := range dangerous {
					if strings.Contains(strings.ToLower(req.Target), d) {
						hasDangerous = true
						break
					}
				}
				if hasSecret && hasDangerous {
					return fmt.Errorf("constitutional violation: potential secret exfiltration blocked")
				}
				return nil
			},
		},
		{
			Name:        "path_traversal_protection",
			Description: "Prevent path traversal attacks",
			Check: func(req ActionRequest) error {
				// Apply to ALL file operations, including edit (was missing)
				if req.Type != ActionReadFile && req.Type != ActionWriteFile && req.Type != ActionDeleteFile && req.Type != ActionEditFile {
					return nil
				}
				// Normalize path before checking — raw strings.Contains("..") is
				// trivially bypassed with symlinks or encoded sequences.
				cleaned := filepath.Clean(req.Target)
				if strings.Contains(cleaned, "..") {
					return fmt.Errorf("constitutional violation: path traversal blocked")
				}
				// On systems that support symlinks, resolve and verify the target
				// stays within the workspace. EvalSymlinks is best-effort here
				// (target may not exist yet for writes).
				if abs, err := filepath.EvalSymlinks(cleaned); err == nil {
					if strings.Contains(filepath.Clean(abs), "..") {
						return fmt.Errorf("constitutional violation: symlink path traversal blocked")
					}
				}
				return nil
			},
		},
		{
			Name:        "no_system_file_modification",
			Description: "Prevent modification of system files",
			Check: func(req ActionRequest) error {
				if req.Type != ActionWriteFile && req.Type != ActionDeleteFile && req.Type != ActionEditFile {
					return nil
				}
				systemPaths := []string{"/etc/", "/usr/", "/bin/", "/sbin/", "c:\\windows\\", "c:/windows/"}
				// Normalize to forward slashes and lowercase for case-insensitive comparison (Windows)
				target := strings.ToLower(filepath.ToSlash(req.Target))
				for _, sp := range systemPaths {
					if strings.HasPrefix(target, sp) {
						return fmt.Errorf("constitutional violation: system file modification blocked")
					}
				}
				return nil
			},
		},
	}
}

// checkConstitution verifies the action against all constitutional rules.
func (v *VirtualStore) checkConstitution(req ActionRequest) error {
	for _, rule := range v.constitution {
		if err := rule.Check(req); err != nil {
			return err
		}
	}
	return nil
}

// isDestructiveAction returns true for action types that could cause irreversible damage.
// These are routed through the Dreamer safety gate for speculative pre-evaluation.
// Aggressive list: shell execution, file mutations, git ops, campaign mutations.
func isDestructiveAction(t ActionType) bool {
	switch t {
	// Shell execution — arbitrary code
	case ActionExecCmd, ActionRunCommand, ActionBash, ActionRunBuild, ActionExecTool:
		return true
	// File mutations
	case ActionWriteFile, ActionEditFile, ActionDeleteFile, ActionFSWrite:
		return true
	// Line-level mutations
	case ActionEditLines, ActionInsertLines, ActionDeleteLines:
		return true
	// Code DOM element edits
	case ActionEditElement:
		return true
	// Git operations — can rewrite history
	case ActionGitOperation:
		return true
	// Campaign file mutations
	case ActionCampaignCreateFile, ActionCampaignModifyFile, ActionCampaignRefactor:
		return true
	// Python environment — arbitrary code execution
	case ActionPythonEnvExec, ActionPythonApplyPatch:
		return true
	default:
		return false
	}
}

// =============================================================================
// UTILITY METHODS
// =============================================================================
