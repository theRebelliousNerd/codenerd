package session

import (
	"fmt"
	"slices"
	"strings"

	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// subagentReturnHandlePrefix marks a subagent-return handle in a turn's input
// (internal/observation: subagentHandlePrefix; recall_context and
// subagent_expand redeem it).
const subagentReturnHandlePrefix = "obs:sa:"

// turnWithheldTools asks the kernel which of the persona's tools this turn
// does not get (policy/jit_tools.mg): those its target's language withholds,
// and those that need something the turn lacks. Go measures the turn -- the
// language, whether an MCP server is registered, whether the input carries a
// subagent-return handle -- and names each measurement to the kernel as a
// constant; the kernel says what it withholds. The result only ever narrows
// the envelope.
func (e *Executor) turnWithheldTools(language, input string) []string {
	if e.kernel == nil {
		return nil
	}
	var withheld []string
	add := func(predicate, bound string, facts []types.Fact, err error) {
		if err != nil {
			logging.Get(logging.CategorySession).Warn("%s(%s, Tool) failed: %v; the turn keeps its envelope", predicate, bound, err)
			return
		}
		for _, f := range facts {
			if len(f.Args) == 0 {
				continue
			}
			tool := strings.TrimPrefix(types.ExtractString(f.Args[len(f.Args)-1]), "/")
			if tool != "" && !slices.Contains(withheld, tool) {
				withheld = append(withheld, tool)
			}
		}
	}
	if validMangleVerb(language) {
		facts, err := e.kernel.Query(fmt.Sprintf("target_withholds_tool(%s, Tool)", language))
		add("target_withholds_tool", language, facts, err)
	}
	for _, absent := range e.turnAbsences(input) {
		facts, err := e.kernel.Query(fmt.Sprintf("absence_withholds_tool(%s, Tool)", absent))
		add("absence_withholds_tool", absent, facts, err)
	}
	if len(withheld) > 0 {
		logging.Session("Turn catalog: the kernel withholds %v (language %q)", withheld, language)
	}
	return withheld
}

// turnAbsences measures what this turn lacks that some tool exists for: no
// registered MCP server (/mcp_server), no subagent-return handle in the input
// (/subagent_return_handle). A registry the kernel cannot be asked about is not
// measured as absent: an unknown keeps the tools.
func (e *Executor) turnAbsences(input string) []string {
	var absent []string
	if servers, err := e.kernel.Query("mcp_server_registered"); err != nil {
		logging.Get(logging.CategorySession).Warn("mcp_server_registered query failed: %v", err)
	} else if len(servers) == 0 {
		absent = append(absent, "/mcp_server")
	}
	if !strings.Contains(input, subagentReturnHandlePrefix) {
		absent = append(absent, "/subagent_return_handle")
	}
	return absent
}

// withoutTools returns the tools not in withheld, in their order. It never
// adds: a nil or empty list stays empty.
func withoutTools(tools, withheld []string) []string {
	if len(withheld) == 0 || len(tools) == 0 {
		return tools
	}
	kept := make([]string, 0, len(tools))
	for _, t := range tools {
		if !slices.Contains(withheld, t) {
			kept = append(kept, t)
		}
	}
	return kept
}

// configWithoutTools is cfg with the withheld tools removed from its
// allowlist, as a copy: the config may be a precompiled one shared across
// turns, and one turn's catalog must not narrow the next.
func configWithoutTools(cfg *config.EffectiveAgentRuntimeConfig, withheld []string) *config.EffectiveAgentRuntimeConfig {
	if cfg == nil || len(withheld) == 0 {
		return cfg
	}
	narrowed := *cfg
	narrowed.AllowedTools = withoutTools(slices.Clone(cfg.AllowedTools), withheld)
	return &narrowed
}
