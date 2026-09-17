// change_benchmark compares an ordinary tool loop, the production executor,
// and caller-contracted execution on the same private Go regression task.
// Each invocation uses a fresh workspace; results are independently verified.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/evidence"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/system"
	"codenerd/internal/tools"
	coretools "codenerd/internal/tools/core"
	shelltools "codenerd/internal/tools/shell"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

const maxCalls = 10
const maxTools = 16

type boundedClient struct {
	types.LLMClient
	calls atomic.Int32
}

// Unwrap exposes the wrapped client so broker.Base and broker.IsBrokered can
// walk the decorator chain. Without it this type is opaque to both: Base stops
// here instead of reaching the concrete client, and IsBrokered reports an
// already-metered chain as un-metered.
func (c *boundedClient) Unwrap() types.LLMClient { return c.LLMClient }

func (c *boundedClient) admit() error {
	if c.calls.Add(1) > maxCalls {
		return errors.New("comparison LLM-call budget exhausted")
	}
	return nil
}
func (c *boundedClient) Complete(ctx context.Context, p string) (string, error) {
	if err := c.admit(); err != nil {
		return "", err
	}
	return c.LLMClient.Complete(ctx, p)
}
func (c *boundedClient) CompleteWithSystem(ctx context.Context, s, p string) (string, error) {
	if err := c.admit(); err != nil {
		return "", err
	}
	return c.LLMClient.CompleteWithSystem(ctx, s, p)
}
func (c *boundedClient) CompleteWithStreaming(ctx context.Context, s, p string, thinking bool) (<-chan string, <-chan error) {
	if err := c.admit(); err != nil {
		chunks, errs := make(chan string), make(chan error, 1)
		close(chunks)
		errs <- err
		close(errs)
		return chunks, errs
	}
	return c.LLMClient.CompleteWithStreaming(ctx, s, p, thinking)
}
func (c *boundedClient) CompleteWithTools(ctx context.Context, s, p string, d []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if err := c.admit(); err != nil {
		return nil, err
	}
	return c.LLMClient.CompleteWithTools(ctx, s, p, d)
}
func (c *boundedClient) CompleteWithToolResults(ctx context.Context, s string, h []types.Message, d []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if err := c.admit(); err != nil {
		return nil, err
	}
	p, ok := c.LLMClient.(types.ToolResultsProvider)
	if !ok {
		return nil, errors.New("native tool results required")
	}
	return p.CompleteWithToolResults(ctx, s, h, d)
}

var catalog = []string{"read_file", "list_files", "write_file", "edit_file", "run_tests"}

type comparison struct {
	Mode             string            `json:"mode"`
	Model            string            `json:"model"`
	MaxLLMCalls      int               `json:"max_llm_calls"`
	MaxToolCalls     int               `json:"max_tool_calls"`
	MaxOutputTokens  int               `json:"max_output_tokens"`
	LLMCalls         int32             `json:"llm_calls"`
	Tokens           usage.TokenCounts `json:"tokens"`
	Latency          time.Duration     `json:"latency_ns"`
	ReportedComplete bool              `json:"reported_complete"`
	FalseCompletion  bool              `json:"false_completion"`
	HumanRescues     int               `json:"human_rescues"`
	Error            string            `json:"error,omitempty"`
	Response         string            `json:"response"`
	Evaluation       evidence.Report   `json:"independent_evaluation"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	mode := flag.String("mode", "evidence", "minimal, current, or evidence")
	root := flag.String("workspace", "", "fresh task workspace")
	cfgPath := flag.String("config", "", "provider configuration (never included in reports)")
	contractPath := flag.String("contract", "", "caller regression contract JSON")
	output := flag.String("output", "", "result JSON path outside the task workspace")
	flag.Parse()
	if *root == "" || *output == "" || (*mode != "minimal" && *mode != "current" && *mode != "evidence") {
		return errors.New("require workspace, output and a supported mode")
	}
	c, err := evidence.LoadContract(*contractPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	tx, err := evidence.Begin(ctx, *root, c)
	if err != nil {
		return err
	}
	userCfg, err := config.LoadUserConfig(*cfgPath)
	if err != nil {
		return err
	}
	userCfg.Worker = nil
	userCfg.Planner = nil
	userCfg.MaxOutputTokens = 2048
	userCfg.ClassificationModel = userCfg.Model
	pc, err := perception.ProviderConfigFromUserConfig(userCfg)
	if err != nil {
		return err
	}
	base, err := perception.NewClientFromConfig(pc)
	if err != nil {
		return err
	}
	client := &boundedClient{LLMClient: base}
	tracker, err := usage.NewTracker(*root)
	if err != nil {
		return err
	}
	defer tracker.Close()
	ctx = usage.NewContext(ctx, tracker)
	r := comparison{Mode: *mode, Model: pc.Model, MaxLLMCalls: maxCalls, MaxToolCalls: maxTools, MaxOutputTokens: 2048}
	start := time.Now()
	if *mode == "minimal" {
		r.Response, err = minimal(ctx, client, *root, c.Task)
		r.ReportedComplete = err == nil && r.Response != ""
	} else {
		var cortex *system.Cortex
		cortex, err = system.BootCortexWithConfig(ctx, system.BootConfig{Workspace: *root, UserConfigOverride: userCfg, LLMClientOverride: client})
		if err == nil {
			cfg := session.DefaultExecutorConfig()
			cfg.WorkspaceRoot = *root
			cfg.MaxToolCalls = maxTools
			cfg.MaxToolIterations = maxCalls
			cfg.AdaptiveToolBudget = false
			cortex.SessionExecutor.SetConfig(cfg)
			cortex.SessionExecutor.SetAgentConfig(&jitconfig.EffectiveAgentRuntimeConfig{IdentityPrompt: "Go bug-fix task", AllowedTools: catalog, Policies: []string{"policy/validation.mg"}})
			if *mode == "evidence" {
				ctx = evidence.WithContract(ctx, c)
			}
			var result *session.ExecutionResult
			result, err = cortex.SessionExecutor.ProcessWithIntent(ctx, c.Task, &perception.Intent{Verb: "/fix", Category: "/action"})
			if result != nil {
				r.Response = result.Response
				r.ReportedComplete = result.TurnOutcome == "/done"
				if err == nil {
					err = result.Error
				}
			}
			if closeErr := cortex.Close(); err == nil {
				err = closeErr
			}
		}
	}
	r.Latency = time.Since(start)
	r.LLMCalls = client.calls.Load()
	r.Tokens = tracker.Stats().TotalProject
	if err != nil {
		r.Error = err.Error()
	}
	// The same independent evaluator judges every mode, including failed runs.
	verifyCtx, stopVerify := context.WithTimeout(context.Background(), 2*time.Minute)
	defer stopVerify()
	r.Evaluation = tx.Verify(verifyCtx)
	r.FalseCompletion = r.ReportedComplete && r.Evaluation.Status != "verified"
	data, marshalErr := json.MarshalIndent(r, "", "  ")
	if marshalErr != nil {
		return marshalErr
	}
	if writeErr := os.WriteFile(*output, data, 0600); writeErr != nil {
		return writeErr
	}
	fmt.Printf("%s: independently %s; %d tokens; %s; false completion=%v\n", r.Mode, r.Evaluation.Status, r.Tokens.Total, r.Latency.Round(time.Millisecond), r.FalseCompletion)
	return err
}

func minimal(ctx context.Context, client *boundedClient, root, task string) (string, error) {
	r := tools.NewRegistry()
	r.SetWorkspaceRoot(root)
	r.SetAllowlist(&tools.Allowlist{Enforced: true, Names: catalog})
	if err := coretools.RegisterAll(r); err != nil {
		return "", err
	}
	if err := shelltools.RegisterAll(r); err != nil {
		return "", err
	}
	var defs []types.ToolDefinition
	for _, name := range catalog {
		tool := r.Get(name)
		defs = append(defs, types.ToolDefinition{Name: name, Description: tool.Description, InputSchema: map[string]any{"type": "object", "properties": tool.Schema.Properties, "required": tool.Schema.Required}})
	}
	const prompt = "Complete the Go bug fix using the supplied tools. Preserve existing tests. Report what was changed and checked."
	history := []types.Message{{Role: "user", Text: task}}
	calls := 0
	for round := 0; round < maxCalls; round++ {
		response, err := client.CompleteWithToolResults(ctx, prompt, history, defs)
		if err != nil {
			return "", err
		}
		if response == nil {
			return "", errors.New("nil model response")
		}
		if len(response.ToolCalls) == 0 {
			return response.Text, nil
		}
		// AssistantMessageFrom rather than a literal: the literal is where a
		// turn's block order and its thinking signatures are dropped, and the
		// next round would replay a prefix the model never produced.
		history = append(history, types.AssistantMessageFrom(response))
		var results []types.ToolResult
		for _, call := range response.ToolCalls {
			calls++
			if calls > maxTools {
				return "", errors.New("comparison tool-call budget exhausted")
			}
			result, err := r.Execute(ctx, call.Name, call.Input)
			out := ""
			if result != nil {
				out = result.Result
			}
			if err != nil {
				out = err.Error()
			}
			results = append(results, types.ToolResult{ToolUseID: call.ID, Content: out, IsError: err != nil})
		}
		resultBlocks := make([]types.ContentBlock, 0, len(results))
		for _, r := range results {
			resultBlocks = append(resultBlocks, types.ToolResultBlock(r.ToolUseID, r.Content, r.IsError))
		}
		history = append(history, types.NewUserMessage(resultBlocks...))
	}
	return "", errors.New("comparison round budget exhausted")
}
