package projectdoc

import "strings"

// The write-mutation classification and target extraction live here so every
// gate shares one definition.
//
// They were private to internal/session, which meant the VirtualStore gate and
// any future gate had to restate them — and a write-protection rule that only
// fires for the tool names or argument names one gate happens to know about is
// a gate with holes in it. codeNERD's own security review of
// internal/tools/core/file_ops.go raised the consequence: enforcement lived
// only in callers, so tools.Global().Execute bypassed it entirely.

// PathArgs are the argument names a write-mutation tool may use to name its
// target. Tools disagree ("path", "file_path", "file", "filename"), so every
// gate checks all of them rather than trusting one convention.
var PathArgs = []string{"path", "file_path", "filepath", "file", "filename", "target", "dest", "destination"}

// TargetPath extracts the target path from a tool call's arguments, or "" when
// none of the known argument names carry one.
func TargetPath(args map[string]any) string {
	for _, key := range PathArgs {
		if raw, ok := args[key]; ok {
			if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

// IsWriteMutationTool reports whether a tool name durably mutates a file.
//
// The defensive aliases are names a model may plausibly emit that are not
// registered today. Accepting them costs nothing and keeps the gate closed if
// one is ever added.
func IsWriteMutationTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case // Registered VirtualStore write actions.
		"write_file", "edit_file", "delete_file",
		"edit_lines", "insert_lines", "delete_lines",
		"edit_element", "fs_write",
		// Defensive aliases.
		"apply_patch", "str_replace", "create_file", "replace_in_file", "multi_edit":
		return true
	default:
		return false
	}
}
