package chat

import (
	"codeberg.org/TauCeti/mangle-go/analysis"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/session"

	// Domain shards removed - JIT clean loop handles these via prompt atoms:
	// "codenerd/internal/shards/coder"
	// "codenerd/internal/shards/nemesis"
	// "codenerd/internal/shards/researcher"
	// "codenerd/internal/shards/reviewer"
	// "codenerd/internal/shards/tester"
	// "codenerd/internal/shards/tool_generator"

	"codenerd/internal/store"
	"codenerd/internal/types"
	"context"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// =============================================================================
// TRACE STORE ADAPTER
// =============================================================================
// Adapts store.LocalStore to implement perception.TraceStore interface for
// reasoning trace persistence.

// LocalStoreTraceAdapter wraps LocalStore to implement perception.TraceStore.
type LocalStoreTraceAdapter struct {
	store *store.LocalStore
}

// NewLocalStoreTraceAdapter creates a new trace store adapter.
func NewLocalStoreTraceAdapter(s *store.LocalStore) *LocalStoreTraceAdapter {
	return &LocalStoreTraceAdapter{store: s}
}

// StoreReasoningTrace implements perception.TraceStore.
// Note: perception.TraceStore expects StoreReasoningTrace(*ReasoningTrace)
// but store.LocalStore.StoreReasoningTrace takes interface{}.
func (a *LocalStoreTraceAdapter) StoreReasoningTrace(trace *perception.ReasoningTrace) error {
	if a.store == nil || trace == nil {
		return nil
	}
	// Pass the trace directly - LocalStore accepts interface{} and handles conversion
	return a.store.StoreReasoningTrace(trace)
}

// =============================================================================
// LEARNING STORE ADAPTER (GAP-001 FIX)
// =============================================================================
// Adapts store.LearningStore to implement core.LearningStore interface for
// shard autopoiesis.

// coreLearningStoreAdapter wraps store.LearningStore to implement core.LearningStore.
type coreLearningStoreAdapter struct {
	store *store.LearningStore
}

func (a *coreLearningStoreAdapter) Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error {
	if a.store == nil {
		return nil
	}
	return a.store.Save(shardType, factPredicate, factArgs, sourceCampaign)
}

func (a *coreLearningStoreAdapter) SaveBatch(shardType string, learnings []types.ShardLearning, sourceCampaign string) error {
	if a.store == nil {
		return nil
	}
	return a.store.SaveBatch(shardType, learnings, sourceCampaign)
}

func (a *coreLearningStoreAdapter) Load(shardType string) ([]types.ShardLearning, error) {
	if a.store == nil {
		return nil, nil
	}
	// store.LearningStore.Load already returns []types.ShardLearning
	return a.store.Load(shardType)
}

func (a *coreLearningStoreAdapter) LoadByPredicate(shardType, predicate string) ([]types.ShardLearning, error) {
	if a.store == nil {
		return nil, nil
	}
	// store.LearningStore.LoadByPredicate already returns []types.ShardLearning
	return a.store.LoadByPredicate(shardType, predicate)
}

func (a *coreLearningStoreAdapter) DecayConfidence(shardType string, decayFactor float64) error {
	if a.store == nil {
		return nil
	}
	return a.store.DecayConfidence(shardType, decayFactor)
}

func (a *coreLearningStoreAdapter) Close() error {
	if a.store == nil {
		return nil
	}
	return a.store.Close()
}

// =============================================================================
// SESSION ADAPTERS (Clean Loop Architecture)
// =============================================================================
// These adapters bridge core.* types to types.* interfaces required by
// the session.Executor and session.Spawner.

// sessionKernelAdapter adapts *core.RealKernel to types.Kernel.
type sessionKernelAdapter struct {
	kernel *core.RealKernel
}

func (a *sessionKernelAdapter) LoadFacts(facts []types.Fact) error {
	return a.kernel.LoadFacts(facts)
}

func (a *sessionKernelAdapter) Query(predicate string) ([]types.Fact, error) {
	return a.kernel.Query(predicate)
}

func (a *sessionKernelAdapter) QueryAll() (map[string][]types.Fact, error) {
	return a.kernel.QueryAll()
}

func (a *sessionKernelAdapter) Assert(fact types.Fact) error {
	return a.kernel.Assert(fact)
}

func (a *sessionKernelAdapter) AssertBatch(facts []types.Fact) error {
	return a.kernel.AssertBatch(facts)
}

func (a *sessionKernelAdapter) Retract(predicate string) error {
	return a.kernel.Retract(predicate)
}

func (a *sessionKernelAdapter) RetractFact(fact types.Fact) error {
	return a.kernel.RetractFact(fact)
}

func (a *sessionKernelAdapter) UpdateSystemFacts() error {
	return a.kernel.UpdateSystemFacts()
}

func (a *sessionKernelAdapter) Reset() {
	a.kernel.Reset()
}

func (a *sessionKernelAdapter) AppendPolicy(policy string) {
	a.kernel.AppendPolicy(policy)
}

func (a *sessionKernelAdapter) RetractExactFactsBatch(facts []types.Fact) error {
	return a.kernel.RetractExactFactsBatch(facts)
}

func (a *sessionKernelAdapter) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	return a.kernel.RemoveFactsByPredicateSet(predicates)
}

// sessionVirtualStoreAdapter adapts *core.VirtualStore to types.VirtualStore.
// NOTE: VirtualStore doesn't directly expose these methods yet.
// The executor's tool execution is TODO and will route through VirtualStore.
// For now, this adapter provides stub implementations.
type sessionVirtualStoreAdapter struct {
	vs *core.VirtualStore
}

func (a *sessionVirtualStoreAdapter) ReadFile(path string) ([]string, error) {
	// Route through VirtualStore's FileEditor if available
	if a.vs != nil {
		if editor := a.vs.GetFileEditor(); editor != nil {
			return editor.ReadFile(path)
		}
	}
	// Fallback to direct OS read if no editor configured
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

func (a *sessionVirtualStoreAdapter) WriteFile(path string, content []string) error {
	// Route through VirtualStore's FileEditor if wired
	if a.vs != nil {
		if editor := a.vs.GetFileEditor(); editor != nil {
			_, err := editor.WriteFile(path, content)
			return err
		}
	}
	// Fallback to direct OS write if no editor configured
	return os.WriteFile(path, []byte(strings.Join(content, "\n")), 0644)
}

func (a *sessionVirtualStoreAdapter) Exec(ctx context.Context, cmd string, env []string) (string, string, error) {
	// Route through VirtualStore's Exec method
	if a.vs != nil {
		return a.vs.Exec(ctx, cmd, env)
	}
	return "", "", fmt.Errorf("exec not yet wired: VirtualStore not available for exec")
}

func (a *sessionVirtualStoreAdapter) ReadRaw(path string) ([]byte, error) {
	// Route through VirtualStore if available
	if a.vs != nil {
		return a.vs.ReadRaw(path)
	}
	// Fallback to direct OS read
	return os.ReadFile(path)
}

// PreflightDestructiveToolCall delegates to the real VirtualStore's Dreamer
// gate, satisfying session.InteractiveExecutiveGate so the interactive
// executor's tool loop can run the safety simulation before destructive tool
// calls. Nil store => allow (fail-open), preserving prior behavior.
func (a *sessionVirtualStoreAdapter) PreflightDestructiveToolCall(ctx context.Context, actionID, toolName string, args map[string]any) error {
	if a.vs == nil {
		return nil
	}
	return a.vs.PreflightDestructiveToolCall(ctx, actionID, toolName, args)
}

// ValidateInteractiveToolResult delegates to the real VirtualStore's post-action
// validator registry, satisfying session.InteractiveExecutiveGate. Nil store =>
// no validation (preserves prior behavior).
func (a *sessionVirtualStoreAdapter) ValidateInteractiveToolResult(ctx context.Context, actionID, toolName string, args map[string]any, output string, success bool) error {
	if a.vs == nil {
		return nil
	}
	return a.vs.ValidateInteractiveToolResult(ctx, actionID, toolName, args, output, success)
}

// sessionLLMAdapter adapts perception.LLMClient to types.LLMClient.
type sessionLLMAdapter struct {
	client perception.LLMClient
}

func (a *sessionLLMAdapter) Complete(ctx context.Context, prompt string) (string, error) {
	return a.client.Complete(ctx, prompt)
}

func (a *sessionLLMAdapter) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return a.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
}

func (a *sessionLLMAdapter) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return a.client.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
}

// CompleteWithToolResults forwards to the underlying LLMClient when it
// natively implements ToolResultsProvider (Anthropic, OpenAI). When it
// does NOT (e.g. providers using the Gemini Piggyback JSON envelope),
// this returns ErrToolResultsNotSupported so the session can fall back
// to single-turn CompleteWithTools instead of silently swallowing the
// multi-turn conversation history.
//
// Without this passthrough every blocked / completed tool call dropped
// straight on the floor: the model never saw its own tool_use ↔
// tool_result loop and the agent went deaf on multi-step tasks. See the
// "LLM client does not implement ToolResultsProvider" warning in
// session.log for the canary that traced this gap.
func (a *sessionLLMAdapter) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if trp, ok := a.client.(types.ToolResultsProvider); ok {
		return trp.CompleteWithToolResults(ctx, systemPrompt, history, tools)
	}
	return nil, fmt.Errorf("LLM client %T does not implement ToolResultsProvider; use single-turn CompleteWithTools instead", a.client)
}

func (a *sessionKernelAdapter) GetProgramInfo() *analysis.ProgramInfo {
	return a.kernel.GetProgramInfo()
}

func (s *sessionLLMAdapter) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, forceJSON bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 1)
	errorChan := make(chan error, 1)
	go func() {
		defer close(contentChan)
		defer close(errorChan)
		res, err := s.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			errorChan <- err
			return
		}
		contentChan <- res
	}()
	return contentChan, errorChan
}

type chatTaskDelegatorAdapter struct {
	executor session.TaskExecutor
}

func (a *chatTaskDelegatorAdapter) Execute(ctx context.Context, intent string, task string) (string, error) {
	req := session.TaskRequest{
		IntentVerb: intent,
		Task:       task,
	}
	return a.executor.Execute(ctx, req)
}
