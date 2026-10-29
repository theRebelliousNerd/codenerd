package session

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"codenerd/internal/broker"
	"codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// =============================================================================
// TOOL METER - WHAT EACH TOOL COST AND WHAT IT RETURNED
// =============================================================================
// The executor reports every tool offer and every tool call, on every channel,
// to a types.ToolMeter (the ledger, autopoiesis.ProfileStore). It measures and
// reports; it decides nothing. Which tools earn their place is derived by
// policy/tool_value.mg from what is recorded here.
//
// Tokens are counted with the broker's calibrated text ruler, the same one the
// context compressor and the admission counter use, which is fitted to the
// provider's reported usage as the session runs. One ruler for every figure
// here, so a catalog entry, a call and a result are comparable.

// SetToolMeter installs the meter the executor reports tool offers and calls
// to. Nil clears it: an unmetered executor records nothing and fails nothing.
func (e *Executor) SetToolMeter(m types.ToolMeter) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.toolMeter = m
}

func (e *Executor) currentToolMeter() types.ToolMeter {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.toolMeter
}

func toolTokens(s string) int {
	return broker.Default().TextCounter("").EstimateTokens(s)
}

// verificationTools are the tools whose purpose is to establish that an
// earlier write is right. A call to one, after a write, is charged to the
// write as verification the write made necessary.
var verificationTools = map[string]bool{
	"run_build": true, "run_tests": true, "run_check": true, "run_impacted_tests": true,
}

// noteToolOffers reports what one compile put in front of the model: for every
// tool the turn may call, its catalog entry's tokens, and the tokens of the
// included atoms that declare they teach it (requires_tools). An atom naming
// several offered tools is split evenly among them, so a tool is charged its
// share of an atom and the shares of one atom never exceed its tokens.
func (e *Executor) noteToolOffers(compiled *prompt.CompilationResult, cfg *config.EffectiveAgentRuntimeConfig) {
	meter := e.currentToolMeter()
	if meter == nil || cfg == nil || len(cfg.AllowedTools) == 0 {
		return
	}
	session := e.SessionID()
	origin := map[string]types.ToolOrigin{}
	catalog := map[string]int{}
	for _, def := range e.buildToolDefinitions(cfg) {
		origin[def.Name] = types.ToolOriginBuiltin
		catalog[def.Name] = definitionTokens(def)
	}
	for _, def := range e.generatedToolDefinitions(cfg) {
		origin[def.Name] = types.ToolOriginGenerated
		catalog[def.Name] = definitionTokens(def)
	}
	atomTokens := map[string]int{}
	atomIDs := map[string][]string{}
	if compiled != nil {
		for _, atom := range compiled.IncludedAtoms {
			if atom == nil || len(atom.RequiresTools) == 0 {
				continue
			}
			var offered []string
			for _, name := range atom.RequiresTools {
				if _, ok := origin[name]; ok {
					offered = append(offered, name)
				}
			}
			if len(offered) == 0 {
				continue
			}
			share, extra := atom.TokenCount/len(offered), atom.TokenCount%len(offered)
			for i, name := range offered {
				atomTokens[name] += share
				if i < extra {
					atomTokens[name]++
				}
				atomIDs[name] = append(atomIDs[name], atom.ID)
			}
		}
	}
	names := make([]string, 0, len(origin))
	for name := range origin {
		names = append(names, name)
	}
	sort.Strings(names)
	offers := make([]types.ToolOffer, 0, len(names))
	for _, name := range names {
		offers = append(offers, types.ToolOffer{
			Tool: name, Origin: origin[name], SessionID: session,
			CatalogTokens: catalog[name], AtomTokens: atomTokens[name], Atoms: atomIDs[name],
		})
	}
	meter.NoteToolOffers(offers)
}

func definitionTokens(def types.ToolDefinition) int {
	encoded, err := json.Marshal(def)
	if err != nil {
		// A definition that cannot be encoded cannot be sent either; it
		// costs nothing the request carries.
		return 0
	}
	return toolTokens(string(encoded))
}

// toolChannelOf names the path a call arrived on: a generated tool, an MCP
// verb, or the model's native or Piggyback tool channel.
func (e *Executor) toolChannelOf(name string, result *ExecutionResult) (types.ToolChannel, types.ToolOrigin) {
	if !tools.Global().Has(name) {
		e.mu.RLock()
		reg := e.ouroborosRegistry
		e.mu.RUnlock()
		if reg != nil {
			if _, ok := reg.GetTool(name); ok {
				return types.ToolChannelGenerated, types.ToolOriginGenerated
			}
		}
	}
	if strings.HasPrefix(name, "mcp_") {
		return types.ToolChannelMCP, types.ToolOriginBuiltin
	}
	verb := ""
	if result != nil {
		verb = result.Intent.Verb
	}
	if usesPiggybackTools(e.llmForVerb(verb)) {
		return types.ToolChannelPiggyback, types.ToolOriginBuiltin
	}
	return types.ToolChannelNative, types.ToolOriginBuiltin
}

// noteToolCall reports one finished call. content is what the model is handed
// back (output, and the error text on a failure); consumed is what the tool
// reported scanning, and reported says whether it reported at all.
func (e *Executor) noteToolCall(call types.ToolCall, content string, execErr error,
	consumed tools.Consumption, reported bool, started time.Time, verified bool, result *ExecutionResult) {
	meter := e.currentToolMeter()
	if meter == nil {
		return
	}
	args, _ := json.Marshal(call.Input)
	channel, origin := e.toolChannelOf(call.Name, result)
	measure := types.ToolCallMeasure{
		Tool: call.Name, Origin: origin, Channel: channel, SessionID: e.SessionID(),
		Success: execErr == nil, Duration: time.Since(started),
		CallTokens:   toolTokens(call.Name + string(args)),
		ResultTokens: toolTokens(content), ResultBytes: int64(len(content)),
		Consumed: types.ToolConsumption{Known: reported, Bytes: consumed.Bytes, Items: consumed.Items},
	}
	meter.NoteToolCall(measure)

	// A write the model made earlier in the turn is what this verification is
	// for; the build or test run is charged to it.
	if result == nil {
		return
	}
	if isWriteMutationTool(call.Name) && measure.Success {
		result.lastWriteTool = call.Name
		return
	}
	if (verificationTools[call.Name] || verified) && result.lastWriteTool != "" {
		meter.NoteToolVerification(types.ToolVerification{
			Tool: result.lastWriteTool, Verifier: call.Name, Tokens: measure.CallTokens + measure.ResultTokens,
		})
	}
}

// SetToolMeter installs the tool ledger every subagent spawned from here on
// reports to. Already-spawned subagents keep the meter they were built with.
func (s *Spawner) SetToolMeter(m types.ToolMeter) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolMeter = m
}

func (s *Spawner) currentToolMeter() types.ToolMeter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.toolMeter
}
