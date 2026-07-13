package main

import (
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/mangle"
	coresys "codenerd/internal/system"
	"codenerd/internal/types"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// queryCmd queries the Mangle fact store
var queryCmd = &cobra.Command{
	Use:   "query [predicate]",
	Short: "Query facts from the Mangle kernel",
	Long: `Queries the fact store for matching predicates.
Returns all derived facts for the given predicate.

Example:
  nerd query next_action
  nerd query impacted`,
	Args: cobra.ExactArgs(1),
	RunE: queryFacts,
}

// statusCmd shows system status
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show codeNERD system status",
	RunE:  showStatus,
}

// whyCmd explains the reasoning behind the last action
var whyCmd = &cobra.Command{
	Use:   "why [predicate]",
	Short: "Explain why an action was taken or blocked",
	Long: `Shows the derivation trace (proof tree) for a logical conclusion.

This implements the "Glass Box" interface, allowing you to understand
why codeNERD made a specific decision.

Examples:
  nerd why blocked      # Why was the last action blocked?
  nerd why next_action  # Why was this action chosen?
  nerd why impacted     # What files would be impacted?`,
	Args: cobra.MaximumNArgs(1),
	RunE: runWhy,
}

// queryFacts queries the Mangle kernel
func queryFacts(cmd *cobra.Command, args []string) error {
	predicate := args[0]
	logger.Info("Querying facts", zap.String("predicate", predicate))

	baseCtx := cmd.Context()
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(baseCtx, timeout)
	defer cancel()

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex to load all persisted facts (including scan.mg)
	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	facts, err := cortex.Kernel.Query(predicate)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if len(facts) == 0 {
		fmt.Printf("No facts found for predicate '%s'\n", predicate)
		return nil
	}

	fmt.Printf("Facts for '%s':\n", predicate)
	for _, fact := range facts {
		fmt.Printf("  %s\n", fact.String())
	}
	return nil
}

// showStatus displays system status
func showStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("codeNERD System Status")
	fmt.Println("======================")
	fmt.Printf("Version: Cortex 1.5.0\n")
	fmt.Printf("Kernel:  Google Mangle (Datalog)\n")
	fmt.Printf("Runtime: Go %s\n", "1.24")
	fmt.Println()

	// Check API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}
	if key != "" {
		fmt.Println("✓ Z.AI API key configured")
	} else {
		fmt.Println("✗ Z.AI API key not configured")
	}

	// Check workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	fmt.Printf("✓ Workspace: %s\n", cwd)

	// Initialize kernel and show stats
	kern, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}
	fmt.Printf("✓ Mangle kernel initialized\n")
	fmt.Printf("  Schemas: %d bytes\n", len(kern.GetSchemas()))
	fmt.Printf("  Policy:  %d bytes\n", len(kern.GetPolicy()))

	return nil
}

// whyPredicateAliases maps user-facing shorthand to declared schema predicates.
var whyPredicateAliases = map[string]string{
	"blocked":   "action_blocked",
	"block":     "action_blocked",
	"forbidden": "forbidden",
	"denied":    "forbidden",
}

// runWhy explains the reasoning behind decisions.
// Prefer RealKernel.TraceQuery (store-backed, no mode requirement) over a hollow
// mangle.Engine Tracer, which fails on Decls without descr[mode(...)].
func runWhy(cmd *cobra.Command, args []string) error {
	predicate := "next_action"
	if len(args) > 0 {
		predicate = strings.TrimSpace(args[0])
	}

	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Check if initialized
	if !nerdinit.IsInitialized(cwd) {
		fmt.Println("Project not initialized. Run 'nerd init' first.")
		return nil
	}

	// Resolve aliases before display so the user sees the real predicate.
	if alias, ok := whyPredicateAliases[strings.ToLower(predicate)]; ok && !strings.Contains(predicate, "(") {
		predicate = alias
	}

	fmt.Printf("Explaining derivation for: %s\n", predicate)
	fmt.Println(strings.Repeat("=", 40))

	// Prefer cortex kernel (workspace facts) when boot is available; fall back
	// to a fresh RealKernel with schemas/policy only.
	var kern *core.RealKernel
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}
	baseCtx := cmd.Context()
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(baseCtx, timeout)
	defer cancel()

	if cortex, err := coresys.GetOrBootCortex(ctx, cwd, key, disableSystemShards); err == nil && cortex != nil {
		defer cortex.Close()
		if cortex.RealKernel != nil {
			kern = cortex.RealKernel
		} else if rk, ok := cortex.Kernel.(*core.RealKernel); ok {
			kern = rk
		}
	}
	if kern == nil {
		var err error
		kern, err = core.NewRealKernel()
		if err != nil {
			return fmt.Errorf("failed to create kernel: %w", err)
		}
	}

	query := buildWhyQuery(predicate, kern.GetDeclaredPredicates())
	fmt.Printf("Tracing query: %s\n", query)

	// RealKernel.Query is store-scan based: bare predicate name is most reliable
	// (free-var patterns like next_action(Var0) may be misparsed as constants).
	tracePred := whyTracePredicate(query)
	trace, err := kern.TraceQuery(ctx, tracePred)
	if err != nil {
		// Fallback: hollow engine tracer with mode synthesis for free-var queries.
		trace, err = runWhyViaHollowTracer(ctx, kern, query)
		if err != nil {
			return fmt.Errorf("trace failed: %w", err)
		}
	}

	if trace == nil || len(trace.RootNodes) == 0 {
		fmt.Printf("No derivations found for '%s'.\n", query)
		fmt.Println("(Kernel may have no matching facts in the current workspace state.)")
		return nil
	}

	fmt.Println(trace.RenderASCII())
	return nil
}

// whyTracePredicate returns the store-scan form of a why query (bare name).
func whyTracePredicate(query string) string {
	q := strings.TrimSpace(query)
	if i := strings.Index(q, "("); i > 0 {
		return strings.TrimSpace(q[:i])
	}
	return q
}

// buildWhyQuery turns a bare predicate name into a Mangle query atom with free vars.
func buildWhyQuery(predicate string, decls []string) string {
	query := strings.TrimSpace(predicate)
	if query == "" {
		return "next_action(X)"
	}
	if strings.Contains(query, "(") {
		return query
	}

	// Match "name/arity" declarations from the kernel validator.
	predName := query
	for _, decl := range decls {
		parts := strings.Split(decl, "/")
		if len(parts) != 2 || parts[0] != predName {
			continue
		}
		arity := 0
		_, _ = fmt.Sscanf(parts[1], "%d", &arity)
		if arity <= 0 {
			// 0-arity predicates are rare; query as bare atom via RealKernel path.
			return predName
		}
		vars := make([]string, arity)
		for i := 0; i < arity; i++ {
			vars[i] = fmt.Sprintf("Var%d", i)
		}
		return fmt.Sprintf("%s(%s)", predName, strings.Join(vars, ", "))
	}

	// Default: single free variable — valid Mangle atom for common 1-arity heads.
	return predName + "(X)"
}

func runWhyViaHollowTracer(ctx context.Context, kern *core.RealKernel, query string) (*mangle.DerivationTrace, error) {
	cfg := mangle.DefaultConfig()
	cfg.AutoEval = false
	engine, err := mangle.NewEngine(cfg, nil)
	if err != nil {
		return nil, err
	}
	program := kern.GetSchemas() + "\n" + kern.GetPolicy() + "\n" + kern.GetLearned()
	if err := engine.LoadSchemaString(program); err != nil {
		return nil, err
	}
	baseFacts := kern.GetBaseFacts()
	mangleFacts := make([]mangle.Fact, len(baseFacts))
	for i, f := range baseFacts {
		mangleFacts[i] = mangle.Fact{Predicate: f.Predicate, Args: f.Args}
	}
	if err := engine.AddFacts(mangleFacts); err != nil {
		return nil, err
	}
	tracer := mangle.NewProofTreeTracer(engine)
	tracer.IndexRules()
	return tracer.TraceQuery(ctx, query)
}

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

// sanitizeFactForMangle sanitizes fact arguments to be Mangle-parser friendly.
// Mangle's parser doesn't handle certain escape sequences like \r \n \t in strings.
func sanitizeFactForMangle(fact types.Fact) types.Fact {
	sanitized := types.Fact{
		Predicate: fact.Predicate,
		Args:      make([]any, len(fact.Args)),
	}

	for i, arg := range fact.Args {
		switch v := arg.(type) {
		case string:
			// Replace newlines, carriage returns, tabs with spaces
			s := strings.ReplaceAll(v, "\r\n", " ")
			s = strings.ReplaceAll(s, "\n", " ")
			s = strings.ReplaceAll(s, "\r", " ")
			s = strings.ReplaceAll(s, "\t", " ")
			// Collapse multiple spaces into one
			for strings.Contains(s, "  ") {
				s = strings.ReplaceAll(s, "  ", " ")
			}
			sanitized.Args[i] = strings.TrimSpace(s)
		default:
			sanitized.Args[i] = arg
		}
	}

	return sanitized
}
