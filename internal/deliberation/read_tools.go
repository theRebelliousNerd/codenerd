package deliberation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/sessionlog"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// ReadTools offers the roles the deliberation_read_tool envelope
// (deliberation.mg: the /core reads and the CodeDOM reads) through the
// existing registry's argument, workspace, allowlist and effect guards. Neither
// role gets a shell, a write tool, or permission to ask the person.
type ReadTools struct {
	Kernel       types.Kernel
	Registry     *tools.Registry
	ContextBytes int
}

func (b ReadTools) catalog() (map[string]bool, error) {
	rows, err := b.Kernel.Query("deliberation_read_tool")
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, row := range rows {
		if len(row.Args) == 1 {
			allowed[strings.TrimPrefix(types.ExtractString(row.Args[0]), "/")] = true
		}
	}
	return allowed, nil
}

func (b ReadTools) Definitions() ([]types.ToolDefinition, error) {
	allowed, err := b.catalog()
	if err != nil {
		return nil, err
	}
	var defs []types.ToolDefinition
	if b.Registry == nil {
		return defs, nil
	}
	for _, name := range b.Registry.Names() {
		if !allowed[name] {
			continue
		}
		tool := b.Registry.Get(name)
		defs = append(defs, types.ToolDefinition{Name: name, Description: tool.Description, InputSchema: tool.InputSchema()})
	}
	return defs, nil
}

func (b ReadTools) Execute(ctx context.Context, call types.ToolCall) (string, error) {
	if b.ContextBytes <= 0 {
		return "", fmt.Errorf("deliberation context bound must be positive")
	}
	allowed, err := b.catalog()
	if err != nil {
		return "", err
	}
	if !allowed[call.Name] || call.ArgsError != "" {
		return "", fmt.Errorf("deliberation tool %q refused", call.Name)
	}
	if b.Registry == nil {
		return "", fmt.Errorf("read-only registry unavailable")
	}
	sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: "tool_started", Tool: call.Name, CallID: call.ID, Text: sessionlog.LiveToolSubject(call.Name, call.Input)})
	result, err := b.Registry.Execute(ctx, call.Name, call.Input)
	sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: sessionlog.EventToolResult, Tool: call.Name, CallID: call.ID, Failed: err != nil})
	if err != nil {
		return "", err
	}
	text := result.Result
	if len(text) > b.ContextBytes {
		text = text[:b.ContextBytes] + "\n[bounded deliberation evidence; narrow the read to inspect more]"
	}
	return text, nil
}

// Agent runs one role's bounded evidence collection using the configured model
// and JIT prompt. Returned text is parsed at the conductor boundary.
func Agent(ctx context.Context, k types.Kernel, client types.LLMClient, prompts types.RolePromptCompiler, role, task string, read ReadTools) (string, error) {
	if client == nil || prompts == nil {
		return "", types.ErrRolePromptUnavailable
	}
	system, err := prompts.CompileRolePrompt(ctx, types.RolePromptRequest{Role: role, Intent: role, SemanticQuery: task})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(system) == "" {
		return "", types.ErrRolePromptUnavailable
	}
	defs, err := read.Definitions()
	if err != nil {
		return "", err
	}
	scope := sessionlog.ScopeFrom(ctx)
	if scope == nil {
		return "", fmt.Errorf("deliberation agent needs an execution scope")
	}
	actor := scope.Folder
	payload := struct {
		Frame        json.RawMessage `json:"frame"`
		Observations []ProbeResult   `json:"observations,omitempty"`
	}{Frame: json.RawMessage(task)}
	// used counts evidence: tool executions, which policy bounds
	// (deliberation_agent_continue over deliberation_tool_calls). When the
	// budget or the context bound is spent the role is not failed: tools are
	// withheld and it answers from the evidence it has.
	used := 0
	closed, nudged := false, false
	for {
		input, e := json.Marshal(payload)
		if e != nil {
			return "", e
		}
		if len(input) > read.ContextBytes {
			return "", fmt.Errorf("deliberation role context exceeds configured byte bound")
		}
		offer := defs
		if closed {
			offer = nil
		} else if allowed, e := AllowCall(k, actor, used); e != nil {
			return "", e
		} else if !allowed {
			closed, offer = true, nil
		}
		response, e := client.CompleteWithTools(ctx, system, string(input), offer)
		if e != nil {
			return "", e
		}
		if response == nil {
			return "", fmt.Errorf("empty deliberation model response")
		}
		if len(response.ToolCalls) == 0 {
			if strings.TrimSpace(response.Text) != "" {
				return response.Text, nil
			}
			// An empty reply once evidence is closed is a role that still
			// wanted tools. It is told once that only an answer will do.
			if nudged {
				return "", fmt.Errorf("deliberation role returned no answer")
			}
			closed, nudged = true, true
			payload.Observations = append(payload.Observations, ProbeResult{Key: "answer-required", Error: answerRequired})
			continue
		}
		if offer == nil {
			return "", fmt.Errorf("deliberation role asked for tools after its evidence was closed")
		}
		for i, call := range response.ToolCalls {
			observation := ProbeResult{Key: call.ID, Tool: call.Name}
			// Keep room for the closing notes of the calls still to come in
			// this reply, so refusing them cannot push the input over.
			limit := read.ContextBytes - (len(response.ToolCalls)-i-1)*observationOverhead
			allowed, e := AllowCall(k, actor, used)
			if e != nil {
				return "", e
			}
			room := limit - len(input) - observationOverhead
			// Less room than a closing note is no room for evidence.
			if !allowed || closed || room < observationOverhead {
				closed = true
				observation.Error = evidenceClosed
				payload.Observations = append(payload.Observations, observation)
				continue
			}
			used++
			text, callErr := read.Execute(ctx, call)
			if callErr != nil {
				observation.Error = callErr.Error()
			}
			payload.Observations = append(payload.Observations, observation)
			last := &payload.Observations[len(payload.Observations)-1]
			// JSON escaping can grow the text, so shrink the cut until the
			// encoded input fits the bound.
			for {
				last.Output = boundObservation(text, room)
				if input, e = json.Marshal(payload); e != nil {
					return "", e
				}
				over := len(input) - limit
				if over <= 0 || room <= 0 {
					break
				}
				room -= over
			}
		}
	}
}

// evidenceClosed tells a role its tool budget or context room is spent, so
// its next reply must be its answer.
const evidenceClosed = "evidence closed: the configured tool budget or context bound is spent; answer from the observations you have"

// answerRequired tells a role that replied with nothing that its next reply
// must be its JSON answer.
const answerRequired = "your reply was empty: tools are closed, so reply now with your JSON answer from the observations you have"

// observationOverhead is the JSON framing one observation adds around its
// output (keys, quoting, the error field), kept out of the output's room.
const observationOverhead = len(evidenceClosed) + 128

// boundObservation keeps an observation within the room left in the role's
// context. A cut is visible and says why, never silent.
func boundObservation(text string, room int) string {
	const note = "\n[cut at the deliberation context bound; narrow the read to see more]"
	if len(text) <= room {
		return text
	}
	if room <= len(note) {
		return note[1:]
	}
	return text[:room-len(note)] + note
}
