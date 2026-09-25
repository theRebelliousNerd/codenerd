package system

import (
	"codeberg.org/TauCeti/mangle-go/analysis"

	"codenerd/internal/broker"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	manglepkg "codenerd/internal/mangle"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	"codenerd/internal/types"
	"codenerd/internal/usage"
	"strings"

	"codenerd/internal/store"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"codeberg.org/TauCeti/mangle-go/ast"
	_ "github.com/mattn/go-sqlite3" // SQLite driver for project corpus
)

// LocalStoreTraceAdapter wraps LocalStore to implement perception.TraceStore.
// Duplicated from chat/session.go to avoid import cycle or dependency on `chat`.
type LocalStoreTraceAdapter struct {
	store *store.LocalStore
}

func createTraceStoreAdapter(s *store.LocalStore) *LocalStoreTraceAdapter {
	return &LocalStoreTraceAdapter{store: s}
}

func (a *LocalStoreTraceAdapter) StoreReasoningTrace(trace *perception.ReasoningTrace) error {
	// perception.TraceStore expects StoreReasoningTrace(*ReasoningTrace)
	// store.LocalStore.StoreReasoningTrace takes interface{}.
	return a.store.StoreReasoningTrace(trace)
}

func (a *LocalStoreTraceAdapter) LoadReasoningTrace(traceID string) (*perception.ReasoningTrace, error) {
	// Not implemented for now in this adapter context
	return nil, nil
}

// KernelAdapter adapts core.RealKernel to prompt.KernelQuerier.
// It handles type conversion between []interface{} and []core.Fact.
type KernelAdapter struct {
	kernel core.Kernel
}

var (
	_ prompt.KernelScopeProvider = (*KernelAdapter)(nil)
	_ prompt.KernelRetracter     = (*KernelAdapter)(nil)
)

// NewKernelAdapter creates a new KernelAdapter for the given kernel.
// This adapter bridges core.Kernel to prompt.KernelQuerier interface,
// enabling the JIT Prompt Compiler to query the Mangle kernel for
// skeleton atom selection.
func NewKernelAdapter(kernel core.Kernel) *KernelAdapter {
	return &KernelAdapter{kernel: kernel}
}

type kernelCompilationScope struct {
	*KernelAdapter
}

var _ prompt.KernelCompilationScope = (*kernelCompilationScope)(nil)

func (s *kernelCompilationScope) Close() error {
	// The scope owns an in-memory RealKernel clone. Dropping the final adapter
	// reference discards every compile_context/selector fact in one operation.
	s.KernelAdapter = nil
	return nil
}

// NewCompilationScope snapshots the production kernel for one JIT prompt
// compilation. Selector assertions and queries are therefore isolated across
// concurrent compiles and never mutate the live executive kernel.
func (ka *KernelAdapter) NewCompilationScope() (prompt.KernelCompilationScope, error) {
	if ka == nil || ka.kernel == nil {
		return nil, fmt.Errorf("cannot create prompt compilation scope from nil kernel")
	}

	var live *core.RealKernel
	switch kernel := ka.kernel.(type) {
	case *core.RealKernel:
		live = kernel
	case interface{ GetPrimaryRealKernel() *core.RealKernel }:
		live = kernel.GetPrimaryRealKernel()
	}
	if live == nil {
		return nil, fmt.Errorf("kernel type %T does not expose a snapshot-capable RealKernel", ka.kernel)
	}

	return &kernelCompilationScope{
		KernelAdapter: NewKernelAdapter(live.Clone()),
	}, nil
}

func (ka *KernelAdapter) Query(predicate string) ([]prompt.Fact, error) {
	facts, err := ka.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}
	// Convert []core.Fact to []prompt.Fact
	result := make([]prompt.Fact, len(facts))
	for i, f := range facts {
		result[i] = prompt.Fact{
			Predicate: f.Predicate,
			Args:      f.Args,
		}
	}
	return result, nil
}

func (ka *KernelAdapter) AssertBatch(facts []any) error {
	var coreFacts []core.Fact
	for _, f := range facts {
		switch v := f.(type) {
		case core.Fact:
			coreFacts = append(coreFacts, v)
		case string:
			// Parse string fact
			// Mangle parser expects full clause syntax, typically ending with dot
			input := v
			if !strings.HasSuffix(input, ".") {
				input += "."
			}

			parsed, err := manglepkg.ParseUnit(strings.NewReader(input))
			if err != nil {
				return fmt.Errorf("failed to parse fact string '%s': %w", v, err)
			}

			if len(parsed.Clauses) != 1 {
				return fmt.Errorf("expected 1 clause in fact string, got %d", len(parsed.Clauses))
			}

			atom := parsed.Clauses[0].Head
			args := make([]any, len(atom.Args))
			for i, arg := range atom.Args {
				switch t := arg.(type) {

				case ast.Constant:
					// Handle different constant types based on the Type field
					switch t.Type {
					case ast.NameType:
						// Mangle name constants (start with /)
						args[i] = core.MangleAtom(t.Symbol)
					case ast.StringType:
						// String constants - Symbol contains the raw value (no quotes)
						args[i] = t.Symbol
					case ast.BytesType:
						// Byte string constants
						args[i] = t.Symbol
					case ast.NumberType:
						// Integer constants
						args[i] = t.NumValue
					case ast.Float64Type:
						// Float constants.
						//
						// Float64Value is a METHOD on ast.Constant, not a field
						// like NumValue beside it: func (c Constant)
						// Float64Value() (float64, error). Writing it without
						// parentheses compiles fine — Go produces a method
						// value — and stores a func() (float64, error) in the
						// fact argument. ToAtom then rejects the whole fact:
						//
						//   rejecting fact that fails ToAtom: vector_hit -
						//   unsupported arg type func() (float64, error) at index 1
						//
						// Logged 1,209 times in one day. vector_hit(atomID,
						// score) is the JIT compiler's semantic ranking signal,
						// so every score was dropped and Mangle flesh selection
						// ran blind, silently falling back to keyword matching.
						f, ferr := t.Float64Value()
						if ferr != nil {
							return fmt.Errorf("fact string '%s': float constant at index %d: %w", v, i, ferr)
						}
						args[i] = f
					default:
						// DEFENSIVE: Unknown constant type - log and use Symbol as fallback
						logging.Get(logging.CategoryContext).Warn("AssertBatch: unknown constant type %v, using Symbol fallback", t.Type)
						args[i] = t.Symbol
					}
				default:
					// Fallback for non-constant types (e.g., variables)
					args[i] = fmt.Sprintf("%v", arg)
				}
			}

			coreFacts = append(coreFacts, core.Fact{
				Predicate: atom.Predicate.Symbol,
				Args:      args,
			})
		default:
			return fmt.Errorf("unsupported fact type: %T", f)
		}
	}
	return ka.kernel.LoadFacts(coreFacts)
}

// Retract removes all facts for a predicate from this adapter's kernel. The
// prompt compiler uses this on compatibility adapters; production compilation
// scopes normally discard their private clone wholesale on Close.
func (ka *KernelAdapter) Retract(predicate string) error {
	if ka == nil || ka.kernel == nil {
		return fmt.Errorf("cannot retract %q from nil kernel", predicate)
	}
	return ka.kernel.Retract(predicate)
}

// perceptionLLMAdapter adapts perception.LLMClient to mcp.LLMClient.
type perceptionLLMAdapter struct {
	client perception.LLMClient
}

func (a *perceptionLLMAdapter) Complete(ctx context.Context, prompt string) (string, error) {
	return a.client.Complete(ctx, prompt)
}

func (a *perceptionLLMAdapter) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return a.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
}

// CompleteWithTools implements types.LLMClient interface for MCP integration.
func (a *perceptionLLMAdapter) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	// Forward to underlying client if it supports tool calling
	if toolClient, ok := a.client.(interface {
		CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error)
	}); ok {
		return toolClient.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
	}
	return nil, fmt.Errorf("underlying client does not support CompleteWithTools")
}

// mcpKernelAdapter adapts core.RealKernel to mcp.KernelInterface.
// It converts string facts to core.Fact and handles query results.
type mcpKernelAdapter struct {
	kernel core.Kernel
}

// newMCPKernelAdapter creates a new MCP kernel adapter.
func newMCPKernelAdapter(kernel core.Kernel) *mcpKernelAdapter {
	return &mcpKernelAdapter{kernel: kernel}
}

func (a *mcpKernelAdapter) Assert(fact string) error {
	// Parse string fact into core.Fact
	input := fact
	if !strings.HasSuffix(input, ".") {
		input += "."
	}

	parsed, err := manglepkg.ParseUnit(strings.NewReader(input))
	if err != nil {
		return fmt.Errorf("failed to parse fact '%s': %w", fact, err)
	}

	if len(parsed.Clauses) != 1 {
		return fmt.Errorf("expected 1 clause, got %d", len(parsed.Clauses))
	}

	atom := parsed.Clauses[0].Head
	args := make([]any, len(atom.Args))
	for i, arg := range atom.Args {
		switch t := arg.(type) {
		case ast.Constant:
			switch t.Type {
			case ast.NameType:
				args[i] = core.MangleAtom(t.Symbol)
			case ast.StringType, ast.BytesType:
				args[i] = t.Symbol
			case ast.NumberType:
				args[i] = t.NumValue
			case ast.Float64Type:
				// Float64Value is a method, not a field — see the identical
				// site above. Omitting the call stores a method value and the
				// kernel rejects the fact.
				f, ferr := t.Float64Value()
				if ferr != nil {
					return fmt.Errorf("float constant at index %d: %w", i, ferr)
				}
				args[i] = f
			default:
				args[i] = t.Symbol
			}
		default:
			args[i] = fmt.Sprintf("%v", arg)
		}
	}

	return a.kernel.LoadFacts([]core.Fact{{
		Predicate: atom.Predicate.Symbol,
		Args:      args,
	}})
}

func (a *mcpKernelAdapter) Query(predicate string) ([]map[string]any, error) {
	// 1. Parse the query pattern to identify variables
	queryFact, err := core.ParseFactString(predicate)
	if err != nil {
		// Provide a more helpful error if parsing fails
		return nil, fmt.Errorf("invalid query format '%s': %w", predicate, err)
	}

	// 2. Map variable names to argument indices
	variableMap := make(map[int]string)
	for i, arg := range queryFact.Args {
		if s, ok := arg.(string); ok && strings.HasPrefix(s, "?") {
			variableMap[i] = s[1:] // Trim "?" prefix
		}
	}

	// 3. Execute query to get raw facts
	facts, err := a.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}

	// 4. Transform facts into variable bindings maps
	results := make([]map[string]any, 0, len(facts))
	for _, f := range facts {
		binding := make(map[string]any)

		// If query had variables, extract them
		if len(variableMap) > 0 {
			for idx, varName := range variableMap {
				if idx < len(f.Args) {
					binding[varName] = f.Args[idx]
				}
			}
		}
		// A 0-arity or const-only query has no variables to bind; the empty
		// binding above is itself the match.

		results = append(results, binding)
	}
	return results, nil
}

func (a *mcpKernelAdapter) Retract(fact string) error {
	// Parse string fact into core.Fact. core.ParseFactString already appends the
	// trailing "." that the Mangle parser requires, so the input must NOT carry
	// one — otherwise the parser sees a doubled ".." and rejects the fact.
	input := strings.TrimSuffix(strings.TrimSpace(fact), ".")

	parsed, err := core.ParseFactString(input)
	if err != nil {
		return fmt.Errorf("failed to parse fact '%s': %w", fact, err)
	}

	return a.kernel.RetractExactFactsBatch([]core.Fact{parsed})
}

// ============================================================================
// Session Adapters for JITExecutor
// These adapters bridge core types to types.Kernel, types.VirtualStore, and
// types.LLMClient interfaces required by session.Executor and session.Spawner.
// Note: core.Kernel = types.Kernel and core.Fact = types.Fact (aliased).
// ============================================================================

// sessionKernelAdapter adapts core.Kernel to types.Kernel for session package.
type sessionKernelAdapter struct {
	kernel types.Kernel
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

// sessionVirtualStoreAdapter adapts core.VirtualStore to types.VirtualStore.
//
// File access through this adapter is read-only and contained
// (system-virtualstore-adapter-policy-v1, rollback posture). ReadFile and
// WriteFile used to call os.ReadFile / os.WriteFile on whatever path they were
// given: a session task asking the interface for a file got it from anywhere on
// disk, and a write skipped the constitution, the Dreamer and the post-action
// validator the executive applies to a routed /write_file. Until a typed,
// policy-preserving file capability exists on VirtualStore, reads resolve inside
// the workspace and writes are refused -- visibly, never by falling back to a
// raw write. Session file mutation goes through the executive (RouteAction).
type sessionVirtualStoreAdapter struct {
	vs *core.VirtualStore
}

// errSessionAdapterWrite is the refusal WriteFile returns.
var errSessionAdapterWrite = errors.New("session VirtualStore adapter does not write files: route the write through the executive (/write_file) so policy, Dreamer preflight and validation apply")

// containedRead reads path after resolving it inside the workspace root.
func (a *sessionVirtualStoreAdapter) containedRead(path string) ([]byte, error) {
	resolved, err := tools.ResolveWorkspacePath(context.Background(), "", path)
	if err != nil {
		return nil, fmt.Errorf("session read of %q refused: %w", path, err)
	}
	return os.ReadFile(resolved)
}

func (a *sessionVirtualStoreAdapter) ReadFile(path string) ([]string, error) {
	data, err := a.containedRead(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

func (a *sessionVirtualStoreAdapter) WriteFile(path string, content []string) error {
	return errSessionAdapterWrite
}

func (a *sessionVirtualStoreAdapter) Exec(ctx context.Context, cmd string, env []string) (string, string, error) {
	return a.vs.Exec(ctx, cmd, env)
}

func (a *sessionVirtualStoreAdapter) ReadRaw(path string) ([]byte, error) {
	return a.containedRead(path)
}

// Compile-time assertion that the production adapter exposes the Dreamer
// preflight and post-action validator seam. Without this, the session
// executor's type assertion silently skips both gates on every non-TUI path.
var _ session.InteractiveExecutiveGate = (*sessionVirtualStoreAdapter)(nil)

// isSessionAdapterDestructiveTool mirrors the core gate's destructive
// classification for interactive tool names (see
// internal/core/virtual_store_interactive_gate.go interactiveToolActionType
// filtered by isDestructiveAction). Only read_file is non-destructive; every
// other mapped tool mutates files or executes code.
func isSessionAdapterDestructiveTool(toolName string) bool {
	effect, err := tools.LookupEffect(toolName)
	return err != nil || effect != tools.EffectRead
}

// PreflightDestructiveToolCall delegates to the wrapped VirtualStore's Dreamer
// gate, satisfying session.InteractiveExecutiveGate so the session executor's
// tool loop runs the safety simulation before destructive tool calls.
//
// Nil handling is deliberately fail-CLOSED for destructive tools, matching the
// core gate's documented policy (every mapped destructive tool requires a
// usable Dreamer): a nil store blocks destructive tools instead of allowing
// them. This differs from the TUI chat adapter
// (cmd/nerd/chat/session_adapters.go), which is fail-OPEN on a nil store to
// preserve its prior behavior.
func (a *sessionVirtualStoreAdapter) PreflightDestructiveToolCall(ctx context.Context, actionID, toolName string, args map[string]any) error {
	if a == nil || a.vs == nil {
		if isSessionAdapterDestructiveTool(toolName) {
			return &core.InteractiveGateError{Reason: "interactive executive gate unavailable: VirtualStore is nil; blocked destructive tool " + toolName}
		}
		return nil
	}
	return a.vs.PreflightDestructiveToolCall(ctx, actionID, toolName, args)
}

// ValidateInteractiveToolResult delegates to the wrapped VirtualStore's
// post-action validator registry, satisfying session.InteractiveExecutiveGate.
// Nil store => no validation (a post-action check cannot fail closed without a
// side effect to verify).
func (a *sessionVirtualStoreAdapter) ValidateInteractiveToolResult(ctx context.Context, actionID, toolName string, args map[string]any, output string, success bool) error {
	if a == nil || a.vs == nil {
		return nil
	}
	return a.vs.ValidateInteractiveToolResult(ctx, actionID, toolName, args, output, success)
}

// Compile-time assertion that the production adapter exposes memory read-back.
// The session executor type-asserts its store against session.MemoryHydrator;
// without these delegations the assertion fails silently and every non-TUI
// turn starts with no learned facts and no prior-session context (observed
// live 2026-09-04: no HydrateLearnings line in the chat process's logs).
var _ session.MemoryHydrator = (*sessionVirtualStoreAdapter)(nil)

// HydrateLearnings delegates to the wrapped VirtualStore. A nil store hydrates
// nothing and reports zero facts, never an error.
func (a *sessionVirtualStoreAdapter) HydrateLearnings(ctx context.Context) (int, error) {
	if a == nil || a.vs == nil {
		return 0, nil
	}
	return a.vs.HydrateLearnings(ctx)
}

// HydrateSessionContext delegates to the wrapped VirtualStore; see
// HydrateLearnings for the nil policy.
func (a *sessionVirtualStoreAdapter) HydrateSessionContext(ctx context.Context, sessionID, query string, shardTypes []string) (int, error) {
	if a == nil || a.vs == nil {
		return 0, nil
	}
	return a.vs.HydrateSessionContext(ctx, sessionID, query, shardTypes)
}

// sessionLLMAdapter adapts perception.LLMClient to types.LLMClient.
//
// It also carries the process usage tracker so every LLM call made through the
// session executor is metered even when the incoming ctx was never tagged by a
// CLI entry point. The ten cmd/nerd call sites that attach the tracker by hand
// (cmd_chat.go, cmd_instruction.go, cmd_direct_actions.go, cmd_advanced.go,
// cmd_interactive.go, chat/process.go, chat/campaign.go, chat/campaign_assault.go)
// become redundant once this seam is proven live, but they are left alone here:
// redundant tagging is harmless because meteredContext never overrides a ctx
// that already carries a tracker.
type sessionLLMAdapter struct {
	client  perception.LLMClient
	tracker *usage.Tracker
}

// meteredContext returns ctx unchanged when it already carries a tracker or
// when the adapter has none (tracker init failed at boot); otherwise it tags
// ctx with the adapter's tracker. Session IDs and shard metadata already on
// the ctx are left untouched.
// Unwrap exposes the wrapped client so broker.Base and broker.IsBrokered can
// walk the decorator chain. Without it this type is opaque to both: Base stops
// here instead of reaching the concrete client, and IsBrokered reports an
// already-metered chain as un-metered. Neither surfaces as an error -- Base's
// callers use a comma-ok type assertion, so a miss reads as "not that engine".
func (a *sessionLLMAdapter) Unwrap() perception.LLMClient { return a.client }

func (a *sessionLLMAdapter) meteredContext(ctx context.Context) context.Context {
	if a == nil || a.tracker == nil {
		return ctx
	}
	if usage.FromContext(ctx) != nil {
		return ctx
	}
	return usage.NewContext(ctx, a.tracker)
}

// auditLLMCall records one call the session executor made in the audit trail
// (llm_response events; `nerd audit facts` exports them). Every model call a
// session makes passes through this adapter, which is why the record is made
// here: the audit trail carried turns, intents, tools, files and safety checks
// but no model call, so a forensic replay could see what the agent did and not
// what it asked. tokens is 0 for the text-only methods, whose clients do not
// return a count.
func (a *sessionLLMAdapter) auditLLMCall(start time.Time, tokens int, err error) {
	model := "unknown"
	if identifier, ok := broker.Base(a.client).(types.ModelIdentifier); ok {
		if provider, name := identifier.ModelIdentity(); name != "" {
			model = name
			if provider != "" {
				model = provider + "/" + name
			}
		}
	}
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	logging.Audit().LLMCall(model, tokens, time.Since(start).Milliseconds(), err == nil, errMsg)
}

func responseTokens(resp *types.LLMToolResponse) int {
	if resp == nil {
		return 0
	}
	if resp.Usage.TotalTokens > 0 {
		return resp.Usage.TotalTokens
	}
	return resp.Usage.InputTokens + resp.Usage.OutputTokens
}

func (a *sessionLLMAdapter) Complete(ctx context.Context, prompt string) (string, error) {
	start := time.Now()
	out, err := a.client.Complete(a.meteredContext(ctx), prompt)
	a.auditLLMCall(start, 0, err)
	return out, err
}

func (a *sessionLLMAdapter) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	start := time.Now()
	out, err := a.client.CompleteWithSystem(a.meteredContext(ctx), systemPrompt, userPrompt)
	a.auditLLMCall(start, 0, err)
	return out, err
}

func (a *sessionLLMAdapter) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	start := time.Now()
	resp, err := a.client.CompleteWithTools(a.meteredContext(ctx), systemPrompt, userPrompt, tools)
	a.auditLLMCall(start, responseTokens(resp), err)
	return resp, err
}

// ShouldUsePiggybackTools forwards types.PiggybackToolProvider. This adapter
// always claims ToolResultsProvider (it forwards or errors), so the Piggyback
// answer is the only thing that keeps an envelope-only client -- the two CLI
// engines, a grounded Gemini -- off the native tool path, whose first
// continuation fails "does not implement ToolResultsProvider" for them.
func (a *sessionLLMAdapter) ShouldUsePiggybackTools() bool {
	if a == nil || a.client == nil {
		return false
	}
	if p, ok := a.client.(types.PiggybackToolProvider); ok {
		return p.ShouldUsePiggybackTools()
	}
	return false
}

// CompleteWithToolResults forwards multi-turn tool results when the underlying
// perception client implements types.ToolResultsProvider (e.g. XAIClient).
// Without this, the session executor always falls back to single-turn tools
// and coding agents stop after the first tool_use batch.
func (a *sessionLLMAdapter) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	ctx = a.meteredContext(ctx)
	if trp, ok := a.client.(types.ToolResultsProvider); ok {
		start := time.Now()
		resp, err := trp.CompleteWithToolResults(ctx, systemPrompt, history, tools)
		a.auditLLMCall(start, responseTokens(resp), err)
		return resp, err
	}
	// perception clients may implement the interface with perception-local aliases
	type perceptionTRP interface {
		CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error)
	}
	if trp, ok := a.client.(perceptionTRP); ok {
		start := time.Now()
		resp, err := trp.CompleteWithToolResults(ctx, systemPrompt, history, tools)
		a.auditLLMCall(start, responseTokens(resp), err)
		return resp, err
	}
	return nil, fmt.Errorf("LLM client %T does not implement ToolResultsProvider", a.client)
}

func (a *sessionKernelAdapter) GetProgramInfo() *analysis.ProgramInfo {
	return a.kernel.GetProgramInfo()
}

// missingLLMClient.CompleteWithStreaming is defined on the type in factory.go.

func (a *sessionLLMAdapter) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, forceJSON bool) (<-chan string, <-chan error) {
	ctx = a.meteredContext(ctx)
	contentChan := make(chan string, 1)
	errorChan := make(chan error, 1)
	go func() {
		defer close(contentChan)
		defer close(errorChan)
		start := time.Now()
		res, err := a.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		a.auditLLMCall(start, 0, err)
		if err != nil {
			errorChan <- err
			return
		}
		contentChan <- res
	}()
	return contentChan, errorChan
}
