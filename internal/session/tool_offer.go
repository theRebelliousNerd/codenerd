package session

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// The tool offer is what a request describes to the model, as distinct from
// what policy grants (Docs/audits/request-visible-tool-contract-and-
// investigation-affordances.md). A granted name is callable on a channel only
// when a backend owns it and the channel can carry its argument contract:
//
//   - a modular tool (e.modularTools()) has a typed schema and is offered on
//     both channels, rendered from the same definition on each;
//   - a generated (Ouroboros) tool has no typed argument schema -- its binary
//     receives the call's arguments as one JSON object -- so it is offered
//     only on the Piggyback text channel, where that contract can be stated
//     in words. A native provider needs a typed object schema and the host
//     does not invent one;
//   - a name with no backend is not offered anywhere.
//
// Registration is never permission: every name here is already in the
// effective allowlist, and every call is still admitted at dispatch.

// buildToolDefinitions is the native offer: every granted modular tool with
// its schema.
func (e *Executor) buildToolDefinitions(cfg *config.EffectiveAgentRuntimeConfig) []types.ToolDefinition {
	if cfg == nil || len(cfg.AllowedTools) == 0 {
		logging.SessionDebug("buildToolDefinitions: no tools configured (cfg=%v)", cfg != nil)
		return nil
	}
	registry := e.modularTools()
	var defs []types.ToolDefinition
	for _, toolName := range cfg.AllowedTools {
		tool := registry.Get(toolName)
		if tool == nil {
			continue
		}
		defs = append(defs, types.ToolDefinition{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema()})
	}
	logging.Session("Built %d tool definitions from %d allowed tools", len(defs), len(cfg.AllowedTools))
	return defs
}

// generatedToolDefinitions is the part of the Piggyback offer the native
// channel cannot carry: every granted generated tool no modular tool shadows
// (dispatch prefers the modular handler, so the offer does too). Its schema
// is an object with no properties, which the renderer states as untyped.
func (e *Executor) generatedToolDefinitions(cfg *config.EffectiveAgentRuntimeConfig) []types.ToolDefinition {
	if cfg == nil || len(cfg.AllowedTools) == 0 {
		return nil
	}
	e.mu.RLock()
	reg := e.ouroborosRegistry
	e.mu.RUnlock()
	if reg == nil {
		return nil
	}
	modular := e.modularTools()
	var defs []types.ToolDefinition
	for _, name := range cfg.AllowedTools {
		if modular.Has(name) {
			continue
		}
		tool, ok := reg.GetTool(name)
		if !ok || tool == nil {
			continue
		}
		desc := tool.Description
		if len(tool.Capabilities) > 0 {
			desc = strings.TrimSpace(desc + " Capabilities: " + strings.Join(tool.Capabilities, ", ") + ".")
		}
		defs = append(defs, types.ToolDefinition{Name: tool.Name, Description: desc, InputSchema: map[string]any{"type": "object"}})
	}
	return defs
}

// toolUnavailability says why a granted name is not callable on this
// channel, or "" when it is.
func (e *Executor) toolUnavailability(name string, textChannel bool) string {
	if e.modularTools().Has(name) {
		return ""
	}
	e.mu.RLock()
	reg := e.ouroborosRegistry
	e.mu.RUnlock()
	if reg != nil {
		if _, ok := reg.GetTool(name); ok {
			if textChannel {
				return ""
			}
			return "generated tool with no typed argument schema; this model's native tool channel cannot carry it"
		}
	}
	return "no backend is registered for it on this runtime"
}

// splitOfferable separates granted names into those callable on the channel
// and a disposition line for each that is not.
func (e *Executor) splitOfferable(granted []string, textChannel bool) (callable, unavailable []string) {
	for _, name := range granted {
		if why := e.toolUnavailability(name, textChannel); why != "" {
			unavailable = append(unavailable, name+": "+why)
			continue
		}
		callable = append(callable, name)
	}
	if len(unavailable) > 0 {
		logging.Get(logging.CategorySession).Warn("granted but not callable this turn: %s", strings.Join(unavailable, "; "))
	}
	return callable, unavailable
}

// buildToolCatalogForPiggyback renders the single-shot Piggyback offer: the
// native definitions plus the generated tools only this channel carries.
func (e *Executor) buildToolCatalogForPiggyback(cfg *config.EffectiveAgentRuntimeConfig) string {
	if cfg == nil || len(cfg.AllowedTools) == 0 {
		return ""
	}
	return renderPiggybackCatalog(append(e.buildToolDefinitions(cfg), e.generatedToolDefinitions(cfg)...))
}

// piggybackCatalog renders the catalog for the tools a working request
// offers, narrowed to the allowlist so a definition can never widen it.
func (e *Executor) piggybackCatalog(cfg *config.EffectiveAgentRuntimeConfig, offered []types.ToolDefinition) string {
	if len(offered) == 0 || cfg == nil {
		return ""
	}
	kept := make([]types.ToolDefinition, 0, len(offered))
	for _, def := range offered {
		if e.isToolAllowed(def.Name, cfg) {
			kept = append(kept, def)
		}
	}
	return renderPiggybackCatalog(kept)
}

// renderPiggybackCatalog writes the offered definitions as the text channel
// reads them. Every property is listed with its type, whether it is
// required, its description, and its enum, default and item type when the
// schema has them: the same contract the native channel carries as JSON
// Schema. Before 2026-10-28 this channel printed only the required names, so
// optional controls (list_files' recursive/include_hidden, get_element's refs
// array) were invisible to a text-channel model.
func renderPiggybackCatalog(defs []types.ToolDefinition) string {
	if len(defs) == 0 {
		return ""
	}
	exampleJSON, _ := json.MarshalIndent([]map[string]any{{
		"id":        "req_1",
		"tool_name": "<tool_name>",
		"tool_args": map[string]any{"arg_name": "arg_value"},
		"purpose":   "why this tool is needed",
	}}, "", "  ")

	var b strings.Builder
	b.WriteString("\n## Available Tools\n\n")
	b.WriteString("Request tools via `tool_requests` in control_packet:\n```json\n\"tool_requests\": ")
	b.Write(exampleJSON)
	b.WriteString("\n```\n\n")
	for _, def := range defs {
		fmt.Fprintf(&b, "**%s**: %s\n", def.Name, def.Description)
		writeSchemaArguments(&b, def.InputSchema)
	}
	b.WriteString("\n### Missing a Tool?\n\n")
	b.WriteString("If you need a capability not available above:\n")
	b.WriteString("1. Add a mangle_update: `missing_tool_for(\"<capability>\", \"<description>\")`\n")
	b.WriteString("2. The Ouroboros system will generate, compile, and register the tool\n")
	b.WriteString("3. The tool will be available in subsequent turns\n\n")
	b.WriteString("Example:\n```json\n")
	b.WriteString("\"mangle_updates\": [\"missing_tool_for(\\\"/parse_yaml\\\", \\\"Parse YAML files and return structured data\\\")\"]\n")
	b.WriteString("```\n")
	logging.Session("Built Piggyback tool catalog: %d tools", len(defs))
	return b.String()
}

// writeSchemaArguments renders one definition's argument contract.
func writeSchemaArguments(b *strings.Builder, schema map[string]any) {
	props := schemaProperties(schema["properties"])
	if props == nil {
		b.WriteString("  Arguments: one JSON object, passed unchanged to the generated binary as its input; this tool declares no typed schema, so follow its description.\n")
		return
	}
	if len(props) == 0 {
		b.WriteString("  Arguments: none.\n")
		return
	}
	required := map[string]bool{}
	switch req := schema["required"].(type) {
	case []string:
		for _, r := range req {
			required[r] = true
		}
	case []any:
		for _, r := range req {
			if s, ok := r.(string); ok {
				required[s] = true
			}
		}
	}
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	// Required first, then by name: the order is stable, so the rendered
	// prompt is too.
	sort.Slice(names, func(i, j int) bool {
		if required[names[i]] != required[names[j]] {
			return required[names[i]]
		}
		return names[i] < names[j]
	})
	b.WriteString("  Arguments:\n")
	for _, name := range names {
		p := props[name]
		need := "optional"
		if required[name] {
			need = "required"
		}
		typ := p.Type
		if typ == "array" && p.Items != nil && p.Items.Type != "" {
			typ = "array of " + p.Items.Type
		}
		fmt.Fprintf(b, "  - %s (%s, %s)", name, typ, need)
		if d := strings.TrimSpace(p.Description); d != "" {
			fmt.Fprintf(b, ": %s", d)
		}
		if len(p.Enum) > 0 {
			vals := make([]string, len(p.Enum))
			for i, v := range p.Enum {
				vals[i] = fmt.Sprint(v)
			}
			fmt.Fprintf(b, " [one of: %s]", strings.Join(vals, ", "))
		}
		if p.Default != nil {
			fmt.Fprintf(b, " [default: %v]", p.Default)
		}
		b.WriteString("\n")
	}
}

// schemaProperties reads a definition's properties, or nil when the schema
// declares none (an untyped generated tool).
func schemaProperties(v any) map[string]tools.Property {
	switch props := v.(type) {
	case map[string]tools.Property:
		if props == nil {
			return map[string]tools.Property{}
		}
		return props
	case nil:
		return nil
	default:
		return nil
	}
}
