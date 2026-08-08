package system

import (
	"codeberg.org/TauCeti/mangle-go/analysis"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
	"strings"

	"codenerd/internal/store"
	"context"
	"fmt"
	"os"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/parse"
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

			parsed, err := parse.Unit(strings.NewReader(input))
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

	parsed, err := parse.Unit(strings.NewReader(input))
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
		} else {
			// Fallback for 0-arity or const-only queries: return usage of predicate as a flag?
			// Mangle convention for boolean query is strict, but here we return empty map for match
		}

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
// NOTE: VirtualStore doesn't directly expose ReadFile/WriteFile/Exec methods.
// These route through VirtualStore's HandleAction internally. For now, this
// adapter provides fallback implementations using the os package directly.
type sessionVirtualStoreAdapter struct {
	vs *core.VirtualStore
}

func (a *sessionVirtualStoreAdapter) ReadFile(path string) ([]string, error) {
	// Fallback: use os.ReadFile directly
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

func (a *sessionVirtualStoreAdapter) WriteFile(path string, content []string) error {
	// Fallback: use os.WriteFile directly
	return os.WriteFile(path, []byte(strings.Join(content, "\n")), 0644)
}

func (a *sessionVirtualStoreAdapter) Exec(ctx context.Context, cmd string, env []string) (string, string, error) {
	return a.vs.Exec(ctx, cmd, env)
}

func (a *sessionVirtualStoreAdapter) ReadRaw(path string) ([]byte, error) {
	// Route through VirtualStore if available, else fallback to os.ReadFile
	if a.vs != nil {
		return a.vs.ReadRaw(path)
	}
	return os.ReadFile(path)
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

// CompleteWithToolResults forwards multi-turn tool results when the underlying
// perception client implements types.ToolResultsProvider (e.g. XAIClient).
// Without this, the session executor always falls back to single-turn tools
// and coding agents stop after the first tool_use batch.
func (a *sessionLLMAdapter) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if trp, ok := a.client.(types.ToolResultsProvider); ok {
		return trp.CompleteWithToolResults(ctx, systemPrompt, history, tools)
	}
	// perception clients may implement the interface with perception-local aliases
	type perceptionTRP interface {
		CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error)
	}
	if trp, ok := a.client.(perceptionTRP); ok {
		return trp.CompleteWithToolResults(ctx, systemPrompt, history, tools)
	}
	return nil, fmt.Errorf("LLM client %T does not implement ToolResultsProvider", a.client)
}

func (a *sessionKernelAdapter) GetProgramInfo() *analysis.ProgramInfo {
	return a.kernel.GetProgramInfo()
}

// missingLLMClient.CompleteWithStreaming is defined on the type in factory.go.

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
