package tools

import (
	"fmt"
	"sort"
)

// Capability is a reviewed declaration of what a tool offers that the rest of
// the catalog does not. It is the first of the three ways a tool earns its
// place (internal/types/tool_value.go): doing something nothing else in the
// harness can. The other two, condensation and token leverage, are measured;
// this one is declared, because "no other tool reaches this end" is a fact
// about the catalog, not about any call.
//
// Like Effect, a built-in takes its capability from the reviewed manifest
// below, by name, and sets no field; a generated or plugin tool supplies
// Tool.Capability itself or declares none (the ledger then holds its value as
// unknown rather than assuming either).
type Capability string

const (
	// CapabilityUnique: no other registered tool reaches this end. The tool
	// earns by being used at all; it is not judged on cost against value.
	CapabilityUnique Capability = "unique"
	// CapabilityOverlapping: another registered tool, or run_command, reaches
	// the same end. The tool keeps its place only by being cheaper, which the
	// ledger measures: condensation and token leverage over real calls.
	CapabilityOverlapping Capability = "overlapping"
)

// Valid reports whether c is one of the declared classes.
func (c Capability) Valid() bool {
	return c == CapabilityUnique || c == CapabilityOverlapping
}

// builtinCapabilities is the reviewed manifest, by tool name.
var builtinCapabilities = func() map[string]Capability {
	m := map[string]Capability{}
	for _, name := range []string{
		// Bytes only the filesystem holds, and the only writers.
		"read_file", "write_file", "edit_file", "delete_file",
		// Retained observations the harness archived: nothing else redeems a handle.
		"recall_context", "search_expand", "subagent_expand",
		// Address-by-symbol reads and edits over the parsed element model.
		"get_element", "get_elements", "edit_element", "apply_edits", "repoint",
		// Questions only the structure index answers.
		"find_symbol", "package_outline", "callers_of", "callees_of",
		"unreferenced_symbols", "importers_of", "predicate_outline",
		// Evidence and settlement the kernel records.
		"get_impacted_tests", "run_impacted_tests", "run_check", "family_no_change",
		// The shell.
		"run_command",
		// The only writer of generated files under the shell gate: it runs a
		// project-declared generator and nothing else.
		"regenerate",
		// The network and the browser: no other path reaches them.
		"web_fetch", "web_search", "grounded_web_search", "context7_fetch",
		"browser_navigate", "browser_click", "browser_type", "browser_close",
		"browser_screenshot", "browser_observe", "browser_act", "browser_evidence",
		"browser_specs", "browser_wait", "browser_audit", "browser_test",
		"browser_mangle", "browser_reason",
		"mcp_call", "mcp_map", "mcp_probe", "mcp_expand",
		"research_cache_get", "research_cache_set", "research_cache_clear", "research_cache_stats",
	} {
		m[name] = CapabilityUnique
	}
	for _, name := range []string{
		// run_command (git, ls, grep, cat, go build) reaches these ends.
		"bash", "run_build", "run_tests", "run_lint", "git_operation", "git_diff", "git_log",
		"grep", "glob", "list_files", "search_code", "find_text",
		// edit_file or apply_edits writes what these write.
		"create_file", "edit_lines", "insert_lines", "delete_lines",
		"replace_element", "insert_element", "delete_element",
		// browser_observe reads what browser_extract reads; mcp_map lists what mcp_context lists.
		"browser_extract", "mcp_context",
	} {
		m[name] = CapabilityOverlapping
	}
	return m
}()

// BuiltinCapability is the complete capability manifest for the built-in
// catalog. A built-in missing from it is a defect
// TestBuiltinCapabilityCoversCatalog fails on, so adding a tool forces the
// question "what does this do that nothing else does?" to be answered in
// review.
func BuiltinCapability(name string) Capability { return builtinCapabilities[name] }

// BuiltinCapabilityNames lists every name the manifest declares, sorted, so a
// test can find a declaration for a tool that no longer exists.
func BuiltinCapabilityNames() []string {
	names := make([]string, 0, len(builtinCapabilities))
	for name := range builtinCapabilities {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DeclaredCapability resolves the tool's capability: the reviewed manifest for
// a built-in, the tool's own field otherwise. An empty field is not an error
// here (a dynamic tool may declare none); an invalid one is.
func (t *Tool) DeclaredCapability() (Capability, error) {
	if t == nil {
		return "", ErrToolNil
	}
	if c := BuiltinCapability(t.Name); c != "" {
		return c, nil
	}
	if t.Capability == "" {
		return "", nil
	}
	if !t.Capability.Valid() {
		return "", fmt.Errorf("tool %q declares capability %q, which is not %q or %q",
			t.Name, t.Capability, CapabilityUnique, CapabilityOverlapping)
	}
	return t.Capability, nil
}

// LookupCapability is the declared capability of a registered or built-in
// tool, "" when none is declared.
func LookupCapability(name string) Capability {
	if c := BuiltinCapability(name); c != "" {
		return c
	}
	if t := Global().Get(name); t != nil {
		c, _ := t.DeclaredCapability()
		return c
	}
	return ""
}
