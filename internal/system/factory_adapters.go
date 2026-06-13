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

// NewKernelAdapter creates a new KernelAdapter for the given kernel.
// This adapter bridges core.Kernel to prompt.KernelQuerier interface,
// enabling the JIT Prompt Compiler to query the Mangle kernel for
// skeleton atom selection.
func NewKernelAdapter(kernel core.Kernel) *KernelAdapter {
	return &KernelAdapter{kernel: kernel}
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
						// Float constants
						args[i] = t.Float64Value
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
				args[i] = t.Float64Value
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
	// Parse string fact into core.Fact
	input := fact
	if !strings.HasSuffix(input, ".") {
		input += "."
	}

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

func (a *sessionKernelAdapter) GetProgramInfo() *analysis.ProgramInfo {
	return a.kernel.GetProgramInfo()
}

func (m *missingLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, forceJSON bool) (<-chan string, <-chan error) {
	errChan := make(chan error, 1)
	errChan <- fmt.Errorf("no LLM client configured")
	close(errChan)
	return nil, errChan
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
