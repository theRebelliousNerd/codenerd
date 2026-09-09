package tools

import "fmt"

// Effect is a reviewed declaration of a tool's possible effects, independent
// of capability and constitutional permission. Unknown is never read-only.
type Effect string

const (
	EffectRead     Effect = "read"
	EffectWrite    Effect = "write"
	EffectExecute  Effect = "execute"
	EffectExternal Effect = "external"
)

// BuiltinEffect is the complete effect manifest for the built-in catalog.
// Generated and plugin tools must supply their own explicit Tool.Effect.
func BuiltinEffect(name string) Effect {
	switch name {
	case "read_file", "list_files", "glob", "grep", "search_code", "get_element", "get_elements",
		"get_impacted_tests", "git_diff", "git_log", "research_cache_get",
		"research_cache_stats", "browser_extract", "browser_observe",
		"browser_evidence", "browser_specs", "browser_wait",
		"mcp_map", "mcp_probe", "mcp_expand":
		return EffectRead
	case "write_file", "edit_file", "delete_file", "edit_lines", "insert_lines",
		"delete_lines", "edit_element", "apply_edits", "research_cache_set", "research_cache_clear":
		return EffectWrite
	case "run_command", "bash", "run_build", "run_tests", "run_impacted_tests", "git_operation":
		return EffectExecute
	case "browser_navigate", "browser_click", "browser_type", "browser_close",
		"browser_screenshot", "browser_audit", "browser_test", "browser_act",
		"browser_mangle", "browser_reason", "context7_fetch", "web_fetch",
		"web_search", "grounded_web_search",
		"mcp_call", "mcp_context":
		return EffectExternal
	default:
		return ""
	}
}

func (t *Tool) DeclaredEffect() (Effect, error) {
	if t == nil {
		return "", ErrToolNil
	}
	effect := BuiltinEffect(t.Name)
	if effect == "" {
		effect = t.Effect
	}
	switch effect {
	case EffectRead, EffectWrite, EffectExecute, EffectExternal:
		return effect, nil
	default:
		return "", fmt.Errorf("tool %q has no valid effect declaration", t.Name)
	}
}

// LookupEffect fails closed for a name absent from both reviewed manifests
// and the registered dynamic catalog.
func LookupEffect(name string) (Effect, error) {
	if effect := BuiltinEffect(name); effect != "" {
		return effect, nil
	}
	if t := Global().Get(name); t != nil {
		return t.DeclaredEffect()
	}
	return "", fmt.Errorf("tool %q has no effect declaration", name)
}
