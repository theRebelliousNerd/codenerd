package session

import (
	"codenerd/internal/projectdoc"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// A tool call whose target is a secret file is refused, whatever the tool.
//
// Everything a tool returns goes to the model's provider, and a workspace holds
// its own keys. Until 2026-09-22 read_file was a safe_action with no condition
// on its target: `read_file .env` would have sent this workspace's API key to
// the provider, and the file-not-found hint listed .env among its suggestions
// for any missing name in the workspace root.
//
// The decision is the constitution's. This file only measures: it asserts
// touches_secret_path(Target) beside the pending_action, and the policy turns
// that into dangerous_content, so permitted cannot derive
// (policy/constitution.mg). The patterns are execution.secret_paths in
// config.json, matched by tools.IsSecretPath.
//
// Hand-built and not routed through codeNERD, like the recoverable-delete
// narrowing in delete_recoverable.go: a model must not widen the rule that
// constrains it.

// secretPathPredicate is the fact the constitution reads. Declared in
// schemas_safety.mg.
const secretPathPredicate = "touches_secret_path"

// secretPathFact returns the fact to assert for this tool call, and whether
// there is one: the call's target names a secret file, one of its paths does
// (apply_edits carries several), or -- for a shell tool -- its command line
// has a token that does.
func secretPathFact(actionAtom types.MangleAtom, target string, args map[string]any) (types.Fact, bool) {
	if secretTarget(actionAtom, target, args) {
		return types.Fact{Predicate: secretPathPredicate, Args: []any{target}}, true
	}
	return types.Fact{}, false
}

func secretTarget(actionAtom types.MangleAtom, target string, args map[string]any) bool {
	if tools.IsSecretPath(target) {
		return true
	}
	if paths, err := projectdoc.TargetPaths(args); err == nil {
		for _, p := range paths {
			if tools.IsSecretPath(p) {
				return true
			}
		}
	}
	switch string(actionAtom) {
	case "/run_command", "/bash", "/exec_cmd":
		for _, key := range []string{"command", "script", "cmd"} {
			if s, ok := args[key].(string); ok {
				if _, hit := tools.SecretPathInCommand(s); hit {
					return true
				}
			}
		}
		if _, hit := tools.SecretPathInCommand(target); hit {
			return true
		}
	}
	return false
}
