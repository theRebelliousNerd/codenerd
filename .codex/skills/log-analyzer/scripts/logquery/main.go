// logquery - Mangle-powered log analysis tool for codeNERD
//
// A purpose-built query engine for declarative log analysis. Converts
// parsed log facts to Mangle and executes queries against them.
//
// Usage:
//
//	logquery facts.mg                           # Interactive REPL
//	logquery facts.mg --query "?error_entry(T, C, M)"  # Single query
//	logquery facts.mg --builtin errors          # Built-in analysis
//	cat facts.mg | logquery --stdin             # Pipe facts
//
// Built-in analyses:
//
//	errors       - All errors with timestamps
//	root-causes  - First errors in cascade chains
//	cascades     - Error propagation chains
//	slow-ops     - Operations over threshold
//	flow         - Category interaction graph
//	health       - Session health summary
//	anomalies    - High error rates and gaps
//	timing       - Timing-related events
//	coder        - Coder shard events
//	tester       - Tester shard events
//	reviewer     - Reviewer shard events
//	researcher   - Researcher shard events
//	dream        - Dream state events
//	browser      - Browser automation events
//	tactile      - Tactile/command execution events
//	store        - Store operation events
//	world        - World/filesystem events
//	context      - Context compression events
//	embedding    - Embedding events
//	all-errors   - Errors from all categories
//	summary      - Summary view showing counts per category
package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/mangle/analysis"
	"github.com/google/mangle/ast"
	_ "github.com/google/mangle/builtin"
	"github.com/google/mangle/engine"
	"github.com/google/mangle/factstore"
	"github.com/google/mangle/parse"
)

//go:embed schema.mg
var embeddedSchema string

// ANSI color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// colorEnabled tracks whether terminal colors are enabled
var colorEnabled = true

// Built-in query definitions
var builtinQueries = map[string]struct {
	Description string
	Query       string
	PostProcess func([]result) string
}{
	// Level-based queries
	"errors": {
		Description: "All error entries with timestamps and categories",
		Query:       "error_entry",
	},
	"warnings": {
		Description: "All warning entries",
		Query:       "warning_entry",
	},
	"all-errors": {
		Description: "Errors from all categories (same as errors)",
		Query:       "error_entry",
	},

	// Category queries
	"categories": {
		Description: "All active categories in the logs",
		Query:       "active_category",
	},
	"error-categories": {
		Description: "Categories that have errors",
		Query:       "error_category",
	},

	// System component queries
	"kernel": {
		Description: "All kernel events",
		Query:       "kernel_event",
	},
	"kernel-errors": {
		Description: "Kernel errors only",
		Query:       "kernel_error",
	},
	"shards": {
		Description: "All shard events",
		Query:       "shard_event",
	},
	"shard-errors": {
		Description: "Shard errors only",
		Query:       "shard_error",
	},
	"perception": {
		Description: "All perception events",
		Query:       "perception_event",
	},
	"api": {
		Description: "All API events",
		Query:       "api_event",
	},
	"api-errors": {
		Description: "API errors only",
		Query:       "api_error",
	},
	"boot": {
		Description: "All boot events",
		Query:       "boot_event",
	},
	"session": {
		Description: "All session events",
		Query:       "session_event",
	},
	"autopoiesis": {
		Description: "All autopoiesis events",
		Query:       "autopoiesis_event",
	},

	// Shard-specific queries
	"coder": {
		Description: "Coder shard events",
		Query:       "coder_event",
	},
	"tester": {
		Description: "Tester shard events",
		Query:       "tester_event",
	},
	"reviewer": {
		Description: "Reviewer shard events",
		Query:       "reviewer_event",
	},
	"researcher": {
		Description: "Researcher shard events",
		Query:       "researcher_event",
	},

	// Specialized component queries
	"dream": {
		Description: "Dream state events",
		Query:       "dream_event",
	},
	"browser": {
		Description: "Browser automation events",
		Query:       "browser_event",
	},
	"tactile": {
		Description: "Tactile/command execution events",
		Query:       "tactile_event",
	},
	"store": {
		Description: "Store operation events",
		Query:       "store_event",
	},
	"world": {
		Description: "World/filesystem events",
		Query:       "world_event",
	},
	"context": {
		Description: "Context compression events",
		Query:       "context_event",
	},
	"embedding": {
		Description: "Embedding events",
		Query:       "embedding_event",
	},
	"routing": {
		Description: "Routing events",
		Query:       "routing_event",
	},
	"tools": {
		Description: "Tools events",
		Query:       "tools_event",
	},
	"campaign": {
		Description: "Campaign events",
		Query:       "campaign_event",
	},
	"articulation": {
		Description: "Articulation events",
		Query:       "articulation_event",
	},
	"virtual-store": {
		Description: "Virtual store events",
		Query:       "virtual_store_event",
	},
	"system-shards": {
		Description: "System shards events",
		Query:       "system_shards_event",
	},

	// Special analyses (computed in Go, not Mangle)
	"timing": {
		Description: "Timing-related events (messages with duration info)",
		Query:       "_timing", // Special marker for Go-side processing
	},
	"summary": {
		Description: "Summary view showing counts per category",
		Query:       "_summary", // Special marker for Go-side processing
	},
}

type result struct {
	Predicate string
	Args      []interface{}
}

func (r result) String() string {
	var args []string
	for _, arg := range r.Args {
		switch v := arg.(type) {
		case string:
			if strings.HasPrefix(v, "/") {
				args = append(args, v)
			} else {
				args = append(args, fmt.Sprintf("%q", v))
			}
		case int64:
			args = append(args, fmt.Sprintf("%d", v))
		case float64:
			args = append(args, fmt.Sprintf("%.2f", v))
		default:
			args = append(args, fmt.Sprintf("%v", v))
		}
	}
	return fmt.Sprintf("%s(%s)", r.Predicate, strings.Join(args, ", "))
}

// colorize wraps text with ANSI color codes if colors are enabled.
func colorize(text, color string) string {
	if !colorEnabled {
		return text
	}
	return color + text + colorReset
}

// formatTimestamp converts millisecond timestamp to readable time.
func formatTimestamp(ms int64) string {
	t := time.UnixMilli(ms)
	return t.Format("15:04:05.000")
}

// formatLevel returns a colored level string.
func formatLevel(level string) string {
	level = strings.TrimPrefix(level, "/")
	switch level {
	case "error":
		return colorize("ERROR", colorRed)
	case "warn":
		return colorize("WARN ", colorYellow)
	case "info":
		return colorize("INFO ", colorGreen)
	case "debug":
		return colorize("DEBUG", colorGray)
	default:
		return level
	}
}

func main() {
	// Flags
	queryFlag := flag.String("query", "", "Mangle query predicate (e.g., error_entry)")
	builtinFlag := flag.String("builtin", "", "Run built-in analysis (see --list-builtins)")
	listBuiltins := flag.Bool("list-builtins", false, "List available built-in analyses")
	stdinFlag := flag.Bool("stdin", false, "Read facts from stdin")
	outputFormat := flag.String("format", "text", "Output format: text, json, table, color")
	limitFlag := flag.Int("limit", 100, "Maximum results to display (0 = unlimited)")
	schemaOnly := flag.Bool("schema-only", false, "Print embedded schema and exit")
	interactive := flag.Bool("i", false, "Interactive REPL mode")
	verbose := flag.Bool("v", false, "Verbose output (show parse/eval stats)")
	noColor := flag.Bool("no-color", false, "Disable colored output")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "logquery - Mangle-powered codeNERD log analyzer\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  logquery [options] <facts.mg>\n")
		fmt.Fprintf(os.Stderr, "  logquery --stdin [options]\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  logquery session.mg --builtin errors\n")
		fmt.Fprintf(os.Stderr, "  logquery session.mg --builtin summary\n")
		fmt.Fprintf(os.Stderr, "  logquery session.mg --query error_entry --format json\n")
		fmt.Fprintf(os.Stderr, "  logquery session.mg -i  # Interactive REPL\n")
		fmt.Fprintf(os.Stderr, "  cat facts.mg | logquery --stdin --builtin coder\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Handle color settings
	if *noColor {
		colorEnabled = false
	}

	// Handle special modes
	if *listBuiltins {
		printBuiltinList()
		return
	}

	if *schemaOnly {
		fmt.Println(embeddedSchema)
		return
	}

	// Load facts
	var factsContent string
	var err error

	if *stdinFlag {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
			os.Exit(1)
		}
		factsContent = string(data)
	} else if flag.NArg() > 0 {
		factsFile := flag.Arg(0)
		data, err := os.ReadFile(factsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", factsFile, err)
			os.Exit(1)
		}
		factsContent = string(data)
	} else if !*interactive {
		fmt.Fprintf(os.Stderr, "Error: No facts file specified. Use --stdin or provide a file.\n")
		flag.Usage()
		os.Exit(1)
	}

	// Build engine
	eng, err := buildEngine(factsContent, *verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building engine: %v\n", err)
		os.Exit(1)
	}

	// Execute mode
	if *interactive || (flag.NArg() > 0 && *queryFlag == "" && *builtinFlag == "") {
		runREPL(eng)
		return
	}

	if *builtinFlag != "" {
		results, err := runBuiltinQuery(eng, *builtinFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		printResults(results, *outputFormat, *limitFlag)
		return
	}

	if *queryFlag != "" {
		results, err := queryPredicate(eng, *queryFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		printResults(results, *outputFormat, *limitFlag)
		return
	}

	// Default: interactive
	runREPL(eng)
}

func printBuiltinList() {
	fmt.Println("Available built-in analyses:")
	fmt.Println()

	// Group queries by category
	categories := map[string][]string{
		"Level Filters": {"errors", "warnings", "all-errors"},
		"Categories":    {"categories", "error-categories"},
		"Core Systems":  {"kernel", "kernel-errors", "shards", "shard-errors", "perception", "api", "api-errors", "boot", "session"},
		"Shards":        {"coder", "tester", "reviewer", "researcher"},
		"Components":    {"dream", "browser", "tactile", "store", "world", "context", "embedding", "routing", "tools", "campaign", "articulation", "virtual-store", "system-shards", "autopoiesis"},
		"Analysis":      {"timing", "summary"},
	}

	categoryOrder := []string{"Level Filters", "Categories", "Core Systems", "Shards", "Components", "Analysis"}

	for _, cat := range categoryOrder {
		names := categories[cat]
		fmt.Printf("%s%s%s\n", colorBold, cat, colorReset)
		for _, name := range names {
			if q, ok := builtinQueries[name]; ok {
				fmt.Printf("  %-16s %s\n", name, q.Description)
			}
		}
		fmt.Println()
	}

	fmt.Println("Usage: logquery facts.mg --builtin <name>")
}

type logEngine struct {
	store       factstore.FactStore
	programInfo *analysis.ProgramInfo
}

func buildEngine(factsContent string, verbose bool) (*logEngine, error) {
	// Combine schema with facts
	var program strings.Builder
	program.WriteString(embeddedSchema)
	program.WriteString("\n\n# === LOADED FACTS ===\n\n")
	program.WriteString(factsContent)

	programStr := program.String()

	if verbose {
		fmt.Fprintf(os.Stderr, "[logquery] Program size: %d bytes\n", len(programStr))
	}

	// Parse
	parsed, err := parse.Unit(strings.NewReader(programStr))
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[logquery] Parsed %d clauses\n", len(parsed.Clauses))
	}

	// Analyze
	programInfo, err := analysis.AnalyzeOneUnit(parsed, nil)
	if err != nil {
		return nil, fmt.Errorf("analysis error: %w", err)
	}

	// Create store and evaluate
	store := factstore.NewSimpleInMemoryStore()

	stats, err := engine.EvalProgramWithStats(programInfo, store,
		engine.WithCreatedFactLimit(5000000)) // 5M fact limit for large log analysis
	if err != nil {
		return nil, fmt.Errorf("evaluation error: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[logquery] Evaluation complete: %d strata\n", len(stats.Strata))
	}

	return &logEngine{
		store:       store,
		programInfo: programInfo,
	}, nil
}

func queryPredicate(eng *logEngine, predicate string) ([]result, error) {
	var results []result

	// Find the predicate in declarations
	for pred := range eng.programInfo.Decls {
		if pred.Symbol == predicate {
			err := eng.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
				results = append(results, atomToResult(a))
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get facts: %w", err)
			}
			return results, nil
		}
	}

	return nil, fmt.Errorf("predicate '%s' not found. Use --list-builtins or check schema", predicate)
}

func runBuiltinQuery(eng *logEngine, name string) ([]result, error) {
	builtin, ok := builtinQueries[name]
	if !ok {
		return nil, fmt.Errorf("unknown builtin '%s'. Use --list-builtins to see available", name)
	}

	// Handle special computed analyses
	switch builtin.Query {
	case "_timing":
		return computeTimingEvents(eng)
	case "_summary":
		return computeSummary(eng)
	default:
		return queryPredicate(eng, builtin.Query)
	}
}

// computeTimingEvents finds log entries containing timing information.
func computeTimingEvents(eng *logEngine) ([]result, error) {
	var results []result

	// Query all log entries and filter for timing keywords
	timingKeywords := []string{"ms", "duration", "elapsed", "took", "latency", "timeout", "delay"}

	for pred := range eng.programInfo.Decls {
		if pred.Symbol == "log_entry" {
			err := eng.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
				if len(a.Args) >= 4 {
					msg := termToString(a.Args[3])
					msgLower := strings.ToLower(msg)
					for _, kw := range timingKeywords {
						if strings.Contains(msgLower, kw) {
							results = append(results, atomToResult(a))
							break
						}
					}
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed to query log entries: %w", err)
			}
			break
		}
	}

	return results, nil
}

// computeSummary generates a summary of log entries by category.
func computeSummary(eng *logEngine) ([]result, error) {
	categoryCounts := make(map[string]int)
	categoryErrors := make(map[string]int)
	categoryWarnings := make(map[string]int)
	totalEntries := 0
	totalErrors := 0
	totalWarnings := 0

	// Query all log entries
	for pred := range eng.programInfo.Decls {
		if pred.Symbol == "log_entry" {
			err := eng.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
				totalEntries++
				if len(a.Args) >= 3 {
					category := termToString(a.Args[1])
					level := termToString(a.Args[2])

					categoryCounts[category]++

					if level == "/error" {
						categoryErrors[category]++
						totalErrors++
					} else if level == "/warn" {
						categoryWarnings[category]++
						totalWarnings++
					}
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed to query log entries: %w", err)
			}
			break
		}
	}

	// Build summary results
	var results []result

	// Add header result
	results = append(results, result{
		Predicate: "summary_total",
		Args:      []interface{}{int64(totalEntries), int64(totalErrors), int64(totalWarnings)},
	})

	// Add per-category results, sorted by count
	type catInfo struct {
		name     string
		count    int
		errors   int
		warnings int
	}
	var cats []catInfo
	for cat, count := range categoryCounts {
		cats = append(cats, catInfo{
			name:     cat,
			count:    count,
			errors:   categoryErrors[cat],
			warnings: categoryWarnings[cat],
		})
	}
	sort.Slice(cats, func(i, j int) bool {
		return cats[i].count > cats[j].count
	})

	for _, c := range cats {
		results = append(results, result{
			Predicate: "summary_category",
			Args:      []interface{}{c.name, int64(c.count), int64(c.errors), int64(c.warnings)},
		})
	}

	return results, nil
}

func termToString(term ast.BaseTerm) string {
	switch t := term.(type) {
	case ast.Constant:
		return t.Symbol
	default:
		return fmt.Sprintf("%v", term)
	}
}

func atomToResult(a ast.Atom) result {
	args := make([]interface{}, len(a.Args))
	for i, term := range a.Args {
		args[i] = termToValue(term)
	}
	return result{
		Predicate: a.Predicate.Symbol,
		Args:      args,
	}
}

func termToValue(term ast.BaseTerm) interface{} {
	switch t := term.(type) {
	case ast.Constant:
		switch t.Type {
		case ast.NameType:
			return t.Symbol
		case ast.StringType:
			return t.Symbol
		case ast.NumberType:
			return t.NumValue
		case ast.Float64Type:
			return t.Float64Value
		default:
			return t.Symbol
		}
	case ast.Variable:
		return fmt.Sprintf("?%s", t.Symbol)
	default:
		return fmt.Sprintf("%v", term)
	}
}

func printResults(results []result, format string, limit int) {
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	switch format {
	case "json":
		printJSON(results)
	case "table":
		printTable(results)
	case "color":
		printColoredText(results)
	default:
		printText(results)
	}
}

func printText(results []result) {
	if len(results) == 0 {
		fmt.Println("No results.")
		return
	}

	fmt.Printf("Results: %d\n\n", len(results))
	for _, r := range results {
		fmt.Println(r.String())
	}
}

func printColoredText(results []result) {
	if len(results) == 0 {
		fmt.Println("No results.")
		return
	}

	fmt.Printf("Results: %d\n\n", len(results))
	for _, r := range results {
		// Detect result type and format accordingly
		switch r.Predicate {
		case "error_entry", "shard_error", "kernel_error", "api_error",
			"perception_error", "session_error", "boot_error", "store_error",
			"world_error", "coder_error", "tester_error", "reviewer_error",
			"researcher_error", "browser_error", "tactile_error", "dream_error",
			"context_error", "embedding_error":
			printColoredLogEntry(r, colorRed)
		case "warning_entry", "api_warning":
			printColoredLogEntry(r, colorYellow)
		case "summary_total":
			if len(r.Args) >= 3 {
				total := r.Args[0]
				errors := r.Args[1]
				warnings := r.Args[2]
				fmt.Printf("%s=== SUMMARY ===%s\n", colorBold, colorReset)
				fmt.Printf("Total entries: %v\n", total)
				fmt.Printf("Total errors:  %s%v%s\n", colorRed, errors, colorReset)
				fmt.Printf("Total warnings: %s%v%s\n\n", colorYellow, warnings, colorReset)
			}
		case "summary_category":
			if len(r.Args) >= 4 {
				cat := r.Args[0]
				count := r.Args[1]
				errors := r.Args[2]
				warnings := r.Args[3]
				errStr := fmt.Sprintf("%v", errors)
				warnStr := fmt.Sprintf("%v", warnings)
				if errors.(int64) > 0 {
					errStr = colorize(errStr, colorRed)
				}
				if warnings.(int64) > 0 {
					warnStr = colorize(warnStr, colorYellow)
				}
				fmt.Printf("%-20v %6v entries, %s errors, %s warnings\n", cat, count, errStr, warnStr)
			}
		default:
			// Generic log entry with timestamp
			if len(r.Args) >= 3 {
				printGenericLogEntry(r)
			} else {
				fmt.Println(r.String())
			}
		}
	}
}

func printColoredLogEntry(r result, color string) {
	if len(r.Args) >= 3 {
		ts := r.Args[0]
		cat := r.Args[1]
		msg := r.Args[2]

		// Format timestamp if it's a number
		tsStr := fmt.Sprintf("%v", ts)
		if tsInt, ok := ts.(int64); ok {
			tsStr = formatTimestamp(tsInt)
		}

		fmt.Printf("%s %s[%v]%s %s%v%s\n",
			colorize(tsStr, colorCyan),
			color, cat, colorReset,
			color, msg, colorReset)
	} else {
		fmt.Println(colorize(r.String(), color))
	}
}

func printGenericLogEntry(r result) {
	if len(r.Args) >= 3 {
		ts := r.Args[0]
		level := r.Args[1]
		msg := r.Args[2]

		// Format timestamp if it's a number
		tsStr := fmt.Sprintf("%v", ts)
		if tsInt, ok := ts.(int64); ok {
			tsStr = formatTimestamp(tsInt)
		}

		levelStr := fmt.Sprintf("%v", level)
		fmt.Printf("%s %s %v\n",
			colorize(tsStr, colorCyan),
			formatLevel(levelStr),
			msg)
	} else {
		fmt.Println(r.String())
	}
}

func printJSON(results []result) {
	type jsonResult struct {
		Predicate string        `json:"predicate"`
		Args      []interface{} `json:"args"`
	}

	jsonResults := make([]jsonResult, len(results))
	for i, r := range results {
		jsonResults[i] = jsonResult{
			Predicate: r.Predicate,
			Args:      r.Args,
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(jsonResults); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}

func printTable(results []result) {
	if len(results) == 0 {
		fmt.Println("No results.")
		return
	}

	// Determine column widths
	maxArgs := 0
	for _, r := range results {
		if len(r.Args) > maxArgs {
			maxArgs = len(r.Args)
		}
	}

	// Header
	fmt.Printf("%-20s", "Predicate")
	for i := 0; i < maxArgs; i++ {
		fmt.Printf(" | %-20s", fmt.Sprintf("Arg%d", i+1))
	}
	fmt.Println()

	// Separator
	fmt.Print(strings.Repeat("-", 22))
	for i := 0; i < maxArgs; i++ {
		fmt.Print("+", strings.Repeat("-", 22))
	}
	fmt.Println()

	// Rows
	for _, r := range results {
		fmt.Printf("%-20s", truncate(r.Predicate, 20))
		for i := 0; i < maxArgs; i++ {
			val := ""
			if i < len(r.Args) {
				// Special handling for timestamps in first column
				if i == 0 {
					if tsInt, ok := r.Args[i].(int64); ok {
						val = formatTimestamp(tsInt)
					} else {
						val = fmt.Sprintf("%v", r.Args[i])
					}
				} else {
					val = fmt.Sprintf("%v", r.Args[i])
				}
			}
			fmt.Printf(" | %-20s", truncate(val, 20))
		}
		fmt.Println()
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func runREPL(eng *logEngine) {
	fmt.Println("logquery REPL - Mangle Log Analyzer")
	fmt.Println("Commands:")
	fmt.Println("  ?<predicate>   Query predicate (e.g., ?error_entry)")
	fmt.Println("  :builtins      List built-in analyses")
	fmt.Println("  :<builtin>     Run built-in (e.g., :errors)")
	fmt.Println("  :predicates    List all available predicates")
	fmt.Println("  :stats         Show log statistics")
	fmt.Println("  :recent N      Show N most recent entries")
	fmt.Println("  :timeline      Show event timeline summary")
	fmt.Println("  :help          Show this help")
	fmt.Println("  :quit          Exit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("logquery> ")

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			fmt.Print("logquery> ")
			continue
		}

		// Commands
		if strings.HasPrefix(line, ":") {
			cmd := strings.TrimPrefix(line, ":")
			parts := strings.Fields(cmd)
			if len(parts) == 0 {
				fmt.Print("logquery> ")
				continue
			}

			switch parts[0] {
			case "quit", "exit", "q":
				return
			case "help", "h":
				fmt.Println("Commands:")
				fmt.Println("  ?<predicate>   Query predicate")
				fmt.Println("  :builtins      List built-in analyses")
				fmt.Println("  :<builtin>     Run built-in")
				fmt.Println("  :predicates    List all predicates")
				fmt.Println("  :stats         Show log statistics")
				fmt.Println("  :recent N      Show N most recent entries")
				fmt.Println("  :timeline      Show event timeline summary")
				fmt.Println("  :quit          Exit")
			case "builtins":
				printBuiltinList()
			case "predicates", "preds":
				listPredicates(eng)
			case "stats":
				showStats(eng)
			case "recent":
				n := 10 // default
				if len(parts) > 1 {
					if parsed, err := strconv.Atoi(parts[1]); err == nil {
						n = parsed
					}
				}
				showRecent(eng, n)
			case "timeline":
				showTimeline(eng)
			default:
				// Try as builtin
				results, err := runBuiltinQuery(eng, parts[0])
				if err != nil {
					fmt.Printf("Error: %v\n", err)
				} else {
					printColoredText(results)
				}
			}
			fmt.Print("logquery> ")
			continue
		}

		// Query
		if strings.HasPrefix(line, "?") {
			predicate := strings.TrimPrefix(line, "?")
			predicate = strings.TrimSpace(predicate)
			// Handle predicate(args) syntax - just extract predicate name
			if idx := strings.Index(predicate, "("); idx > 0 {
				predicate = predicate[:idx]
			}

			results, err := queryPredicate(eng, predicate)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				printColoredText(results)
			}
			fmt.Print("logquery> ")
			continue
		}

		// Unknown
		fmt.Println("Unknown command. Use :help for commands or ?predicate for queries.")
		fmt.Print("logquery> ")
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
	}
}

func listPredicates(eng *logEngine) {
	fmt.Println("Available predicates:")
	fmt.Println()

	preds := make([]string, 0, len(eng.programInfo.Decls))
	for pred := range eng.programInfo.Decls {
		preds = append(preds, pred.Symbol)
	}
	sort.Strings(preds)

	for _, p := range preds {
		fmt.Printf("  %s\n", p)
	}
}

// showStats displays log statistics including total entries, per-category counts, and error rate.
func showStats(eng *logEngine) {
	results, err := computeSummary(eng)
	if err != nil {
		fmt.Printf("Error computing stats: %v\n", err)
		return
	}

	fmt.Println()
	fmt.Printf("%s=== LOG STATISTICS ===%s\n\n", colorBold, colorReset)

	for _, r := range results {
		switch r.Predicate {
		case "summary_total":
			if len(r.Args) >= 3 {
				total := r.Args[0].(int64)
				errors := r.Args[1].(int64)
				warnings := r.Args[2].(int64)

				errorRate := float64(0)
				if total > 0 {
					errorRate = float64(errors) / float64(total) * 100
				}

				fmt.Printf("Total entries:   %d\n", total)
				fmt.Printf("Total errors:    %s%d%s\n", colorRed, errors, colorReset)
				fmt.Printf("Total warnings:  %s%d%s\n", colorYellow, warnings, colorReset)
				fmt.Printf("Error rate:      %.2f%%\n\n", errorRate)
			}
		case "summary_category":
			if len(r.Args) >= 4 {
				cat := r.Args[0]
				count := r.Args[1].(int64)
				errors := r.Args[2].(int64)
				warnings := r.Args[3].(int64)

				errStr := fmt.Sprintf("%d", errors)
				warnStr := fmt.Sprintf("%d", warnings)
				if errors > 0 {
					errStr = colorize(errStr, colorRed)
				}
				if warnings > 0 {
					warnStr = colorize(warnStr, colorYellow)
				}

				fmt.Printf("  %-25v %6d entries (%s err, %s warn)\n", cat, count, errStr, warnStr)
			}
		}
	}
	fmt.Println()
}

// showRecent displays the N most recent log entries.
func showRecent(eng *logEngine, n int) {
	var entries []result

	// Query all log entries
	for pred := range eng.programInfo.Decls {
		if pred.Symbol == "log_entry" {
			err := eng.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
				entries = append(entries, atomToResult(a))
				return nil
			})
			if err != nil {
				fmt.Printf("Error querying log entries: %v\n", err)
				return
			}
			break
		}
	}

	if len(entries) == 0 {
		fmt.Println("No log entries found.")
		return
	}

	// Sort by timestamp (descending)
	sort.Slice(entries, func(i, j int) bool {
		tsI, okI := entries[i].Args[0].(int64)
		tsJ, okJ := entries[j].Args[0].(int64)
		if okI && okJ {
			return tsI > tsJ
		}
		return false
	})

	// Take top N
	if n > len(entries) {
		n = len(entries)
	}
	entries = entries[:n]

	// Reverse to show oldest first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	fmt.Printf("\n%s=== %d MOST RECENT ENTRIES ===%s\n\n", colorBold, n, colorReset)

	for _, r := range entries {
		if len(r.Args) >= 4 {
			ts := r.Args[0]
			cat := r.Args[1]
			level := r.Args[2]
			msg := r.Args[3]

			tsStr := fmt.Sprintf("%v", ts)
			if tsInt, ok := ts.(int64); ok {
				tsStr = formatTimestamp(tsInt)
			}

			levelStr := fmt.Sprintf("%v", level)

			fmt.Printf("%s %-12v %s %v\n",
				colorize(tsStr, colorCyan),
				cat,
				formatLevel(levelStr),
				msg)
		}
	}
	fmt.Println()
}

// showTimeline displays an event timeline summary grouped by time intervals.
func showTimeline(eng *logEngine) {
	type timeSlot struct {
		start    int64
		end      int64
		total    int
		errors   int
		warnings int
	}

	var entries []result
	var minTime, maxTime int64

	// Query all log entries
	for pred := range eng.programInfo.Decls {
		if pred.Symbol == "log_entry" {
			err := eng.store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
				r := atomToResult(a)
				entries = append(entries, r)
				if len(r.Args) >= 1 {
					if ts, ok := r.Args[0].(int64); ok {
						if minTime == 0 || ts < minTime {
							minTime = ts
						}
						if ts > maxTime {
							maxTime = ts
						}
					}
				}
				return nil
			})
			if err != nil {
				fmt.Printf("Error querying log entries: %v\n", err)
				return
			}
			break
		}
	}

	if len(entries) == 0 {
		fmt.Println("No log entries found.")
		return
	}

	// Create time slots (1-minute intervals)
	slotDuration := int64(60000) // 1 minute in ms
	slots := make(map[int64]*timeSlot)

	for _, r := range entries {
		if len(r.Args) >= 3 {
			ts, ok := r.Args[0].(int64)
			if !ok {
				continue
			}
			level := fmt.Sprintf("%v", r.Args[2])

			slotStart := (ts / slotDuration) * slotDuration
			slot, exists := slots[slotStart]
			if !exists {
				slot = &timeSlot{start: slotStart, end: slotStart + slotDuration}
				slots[slotStart] = slot
			}

			slot.total++
			if level == "/error" {
				slot.errors++
			} else if level == "/warn" {
				slot.warnings++
			}
		}
	}

	// Sort slots by time
	var sortedSlots []*timeSlot
	for _, slot := range slots {
		sortedSlots = append(sortedSlots, slot)
	}
	sort.Slice(sortedSlots, func(i, j int) bool {
		return sortedSlots[i].start < sortedSlots[j].start
	})

	fmt.Printf("\n%s=== EVENT TIMELINE ===%s\n\n", colorBold, colorReset)
	fmt.Printf("Session duration: %s to %s\n\n",
		formatTimestamp(minTime),
		formatTimestamp(maxTime))

	fmt.Printf("%-15s %8s %8s %8s  %s\n", "Time", "Total", "Errors", "Warnings", "Activity")
	fmt.Println(strings.Repeat("-", 70))

	// Find max total for scaling the activity bar
	maxTotal := 0
	for _, slot := range sortedSlots {
		if slot.total > maxTotal {
			maxTotal = slot.total
		}
	}

	for _, slot := range sortedSlots {
		timeStr := formatTimestamp(slot.start)

		errStr := fmt.Sprintf("%d", slot.errors)
		warnStr := fmt.Sprintf("%d", slot.warnings)
		if slot.errors > 0 {
			errStr = colorize(errStr, colorRed)
		}
		if slot.warnings > 0 {
			warnStr = colorize(warnStr, colorYellow)
		}

		// Generate activity bar
		barLen := 20
		if maxTotal > 0 {
			barLen = (slot.total * 20) / maxTotal
		}
		if barLen < 1 && slot.total > 0 {
			barLen = 1
		}

		barColor := colorGreen
		if slot.errors > 0 {
			barColor = colorRed
		} else if slot.warnings > 0 {
			barColor = colorYellow
		}

		bar := colorize(strings.Repeat("|", barLen), barColor)

		fmt.Printf("%-15s %8d %8s %8s  %s\n",
			timeStr, slot.total, errStr, warnStr, bar)
	}
	fmt.Println()
}
