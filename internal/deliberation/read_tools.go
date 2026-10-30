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
	used := 0
	for {
		allowed, e := AllowCall(k, actor, used)
		if e != nil {
			return "", e
		}
		if !allowed {
			return "", fmt.Errorf("deliberation role exhausted its configured evidence budget")
		}
		used++
		input, e := json.Marshal(payload)
		if e != nil {
			return "", e
		}
		if len(input) > read.ContextBytes {
			return "", fmt.Errorf("deliberation role context exceeds configured byte bound")
		}
		response, e := client.CompleteWithTools(ctx, system, string(input), defs)
		if e != nil {
			return "", e
		}
		if response == nil {
			return "", fmt.Errorf("empty deliberation model response")
		}
		if len(response.ToolCalls) == 0 {
			return response.Text, nil
		}
		for _, call := range response.ToolCalls {
			allowed, e = AllowCall(k, actor, used)
			if e != nil {
				return "", e
			}
			if !allowed {
				return "", fmt.Errorf("deliberation role exhausted its configured evidence budget")
			}
			used++
			text, callErr := read.Execute(ctx, call)
			observation := ProbeResult{Key: call.ID, Tool: call.Name, Output: text}
			if callErr != nil {
				observation.Error = callErr.Error()
			}
			payload.Observations = append(payload.Observations, observation)
		}
	}
}
