// Package types provides shared type definitions used across codeNERD packages.
// This package exists to break import cycles between core, articulation, and autopoiesis.
// Types in this package should be foundational data structures with no complex dependencies.
package types

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// =============================================================================
// CONTEXT KEYS & HELPERS
// =============================================================================

// sessionContextKey is the context key for passing SessionContext.
type sessionContextKeyType struct{}

var sessionContextKey = sessionContextKeyType{}

// WithSessionContext returns a context with the SessionContext attached.
// This enables passing session context through execution loops (thread-safe).
func WithSessionContext(ctx context.Context, sessionCtx *SessionContext) context.Context {
	return context.WithValue(ctx, sessionContextKey, sessionCtx)
}

// GetSessionContext retrieves the SessionContext from the context if it exists.
func GetSessionContext(ctx context.Context) *SessionContext {
	if sCtx, ok := ctx.Value(sessionContextKey).(*SessionContext); ok {
		return sCtx
	}
	return nil
}

// =============================================================================
// MANGLE FACT TYPES
// =============================================================================

// MangleAtom represents a Mangle name constant (starting with /).
// This explicit type avoids ambiguity between strings and atoms.
type MangleAtom string

// MangleString represents an explicit Mangle string constant.
// It always produces a string constant, never a name, whatever the value looks like.
// It is the explicit counterpart to MangleAtom.
type MangleString string

// Fact represents a single logical fact (atom) in the EDB.
type Fact struct {
	Predicate string
	Args      []any
}

func isValidMangleNameConstant(v string) bool {
	if !strings.HasPrefix(v, "/") {
		return false
	}

	// Whitespace is never valid in Mangle name constants
	if strings.ContainsAny(v, " \t\n\r") {
		return false
	}

	// File paths should NOT be treated as name constants.
	// Mangle atoms are typically short like /true, /markdown, /coder
	// while file paths look like /mnt/c/path/to/file.go

	// Just "/" with nothing after is invalid
	if v == "/" {
		return false
	}

	// Double slash is never valid in Mangle names
	if strings.Contains(v, "//") {
		return false
	}

	// Mangle supports hierarchical names like /a/b but deep paths
	// like /a/b/c/d are likely file paths, not atoms.
	if strings.Count(v, "/") > 2 {
		return false
	}

	// Common file extensions indicate a file path
	if hasFileExtension(v) {
		return false
	}

	_, err := ast.Name(v)
	return err == nil
}

// hasFileExtension checks if the string ends with a common file extension.
func hasFileExtension(v string) bool {
	commonExts := []string{
		".go", ".md", ".py", ".js", ".ts", ".tsx", ".jsx",
		".yaml", ".yml", ".json", ".txt", ".mg", ".html", ".css",
		".sh", ".bash", ".ps1", ".bat", ".exe", ".dll", ".so",
		".c", ".h", ".cpp", ".hpp", ".rs", ".rb", ".java",
		".xml", ".toml", ".ini", ".cfg", ".conf", ".log",
	}
	lowerV := strings.ToLower(v)
	for _, ext := range commonExts {
		if strings.HasSuffix(lowerV, ext) {
			return true
		}
	}
	return false
}

// String returns the Datalog string representation of the fact.
func (f Fact) String() string {
	var args []string
	for _, arg := range f.Args {
		switch v := arg.(type) {
		case MangleAtom:
			args = append(args, string(v))
		case MangleString:
			args = append(args, fmt.Sprintf("%q", v))
		case string:
			// Handle valid Mangle name constants (start with /).
			// NOTE: Many normal strings can start with "/" (e.g., Go comments "//", Unix paths),
			// so we only treat it as a name constant if it parses as one.
			if isValidMangleNameConstant(v) {
				args = append(args, v)
			} else {
				args = append(args, fmt.Sprintf("%q", v))
			}
		case int:
			args = append(args, fmt.Sprintf("%d", v))
		case int64:
			args = append(args, fmt.Sprintf("%d", v))
		case float64:
			// %f, not %v or the AST renderer: mangle-go prints Float64(2.0) as
			// "2", which re-parses as an int64. Keeping the decimal point is
			// what makes a whole float survive a round trip through a .mg file.
			args = append(args, fmt.Sprintf("%f", v))
		case float32:
			args = append(args, fmt.Sprintf("%f", float64(v)))
		case time.Time:
			// Quoted RFC3339Nano matches what ExtractString produces when the
			// same argument is read back, so writer and reader agree.
			args = append(args, fmt.Sprintf("%q", v.Format(time.RFC3339Nano)))
		case time.Duration:
			args = append(args, fmt.Sprintf("%q", v.String()))
		case bool:
			if v {
				args = append(args, "/true")
			} else {
				args = append(args, "/false")
			}
		case map[string]any, []any, []string, []int, []int64, []float64:
			// Containers must render the way ToAtom encodes them: as a quoted
			// JSON string. The previous %v fallback emitted a bare `map[a:b]`,
			// which is not merely lossy — it is not lexically valid Mangle, and
			// this output is not display-only. northstar.RenderVisionMangle
			// writes Fact.String() into a .mg file that the kernel loads at
			// boot, so one container-valued fact would have made the whole
			// generated file fail to parse.
			args = append(args, quoteJSON(v))
		default:
			// Same reasoning as the container branch: whatever this is, it has
			// to leave here as a single valid Mangle token. ToAtom rejects
			// unknown types outright, but String cannot return an error, so it
			// quotes instead of emitting a bare pointer address.
			args = append(args, quoteJSONOrValue(v))
		}
	}
	return fmt.Sprintf("%s(%s).", f.Predicate, strings.Join(args, ", "))
}

// quoteJSON renders a container as a quoted JSON string constant, matching what
// ToAtom stores. A nil container encodes as "null", exactly as ToAtom does, so
// the two renderings cannot disagree about an empty-vs-absent value.
func quoteJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%q", fmt.Sprintf("%v", v))
	}
	return fmt.Sprintf("%q", string(b))
}

// quoteJSONOrValue renders an argument of unknown type as a quoted Mangle string
// constant, preferring its JSON encoding. Values that JSON-encode to nothing
// useful (pointers to unexported-only structs, funcs, channels) fall back to
// their %v form — still quoted, because the one thing that must not happen is a
// bare token that makes the whole fact unparseable.
func quoteJSONOrValue(v any) string {
	if b, err := json.Marshal(v); err == nil && len(b) > 0 && string(b) != "null" && string(b) != "{}" {
		return fmt.Sprintf("%q", string(b))
	}
	return fmt.Sprintf("%q", fmt.Sprintf("%v", v))
}

// ToAtom converts a Fact to a Mangle AST Atom for direct store insertion.
func (f Fact) ToAtom() (ast.Atom, error) {
	var terms []ast.BaseTerm
	for _, arg := range f.Args {
		switch v := arg.(type) {
		case MangleAtom:
			s := string(v)
			// MangleAtom should always start with / for name constants.
			// If it doesn't, treat it as a string constant instead of failing.
			// This provides defense against malformed MangleAtom values.
			if !strings.HasPrefix(s, "/") {
				terms = append(terms, ast.String(s))
				continue
			}
			c, err := ast.Name(s)
			if err != nil {
				return ast.Atom{}, err
			}
			terms = append(terms, c)
		case MangleString:
			terms = append(terms, ast.String(string(v)))
		case string:
			if isValidMangleNameConstant(v) {
				// Name constant
				c, _ := ast.Name(v)
				terms = append(terms, c)
			} else {
				// String constant
				terms = append(terms, ast.String(v))
			}
		case int:
			terms = append(terms, ast.Number(int64(v)))
		case int64:
			terms = append(terms, ast.Number(v))
		case float32:
			terms = append(terms, ast.Float64(float64(v)))
		case float64:
			terms = append(terms, ast.Float64(v))
		case time.Time:
			// Native TimeType: store as Unix nanoseconds
			terms = append(terms, ast.Time(v.UnixNano()))
		case time.Duration:
			// Native DurationType: store as nanoseconds
			terms = append(terms, ast.Duration(int64(v)))
		case bool:
			if v {
				terms = append(terms, ast.TrueConstant)
			} else {
				terms = append(terms, ast.FalseConstant)
			}
		case map[string]any, []any, []string, []int, []int64, []float64:
			// Containers — serialize to JSON. Many call sites (pending_action,
			// virtual_store payloads, intent metadata) pass containers as
			// opaque /string-shaped blobs; rejecting them here would break
			// production assert paths. JSON keeps the content reproducible
			// for downstream consumers without leaking pointer-shaped data.
			b, err := json.Marshal(v)
			if err != nil {
				return ast.Atom{}, fmt.Errorf(
					"Fact(%s): container arg at index %d failed JSON-encode: %w",
					f.Predicate, len(terms), err)
			}
			terms = append(terms, ast.String(string(b)))
		case nil:
			// Explicit nil: caller almost certainly intended a typed value;
			// reject loudly so the offending assert site is identified
			// instead of poisoning the kernel with the literal string
			// "<nil>" (which then breaks numeric rules at eval time).
			return ast.Atom{}, fmt.Errorf(
				"Fact(%s): nil arg at index %d — assert with a typed value",
				f.Predicate, len(terms))
		default:
			// Unknown argument type. The fallback used to silently coerce
			// via fmt.Sprintf("%v", v), which for struct pointers /
			// interfaces produces strings like "0x7ff63be770e0" — a Go
			// memory address. That value then propagates into the kernel
			// as a StringType constant and, when a numeric builtin
			// (fn:plus, fn:minus, fn:max, etc.) fires on it, evaluation
			// dies with "value 0x... (1) is not a number". The diff
			// engine then falls back to full eval and dies the same way.
			//
			// Best-effort: if the value JSON-encodes to something sensible
			// (a struct with public fields, a wrapped scalar), use that.
			// Pointers, channels, funcs, and unexported-only structs will
			// produce "null" or empty objects — bounce those with a
			// structured error naming the predicate and arg index so the
			// offending caller is identifiable at assert time.
			b, jerr := json.Marshal(v)
			if jerr == nil && len(b) > 0 && string(b) != "null" && string(b) != "{}" {
				terms = append(terms, ast.String(string(b)))
				break
			}
			return ast.Atom{}, fmt.Errorf(
				"Fact(%s): unsupported arg type %T at index %d (value %v); "+
					"assert with string/int/int64/float64/bool/time/MangleAtom or a JSON-encodable container",
				f.Predicate, v, len(terms), v)
		}
	}

	return ast.NewAtom(f.Predicate, terms...), nil
}

// =============================================================================
// KERNEL INTERFACE - Bridge to Mangle Logic Core
// =============================================================================

// KernelFact is the fact type historically carried by the deprecated kernel bridge.
//
// Deprecated: use Fact. This is now an alias, not a separate struct — step 1 of
// the KernelFact deprecation path. It was a byte-identical copy of Fact whose only purpose was to keep the
// autopoiesis bridge from naming Fact, which cost every bridge method a full
// slice copy (core.AutopoiesisBridge.QueryPredicate rebuilt each result) and
// gave callers two names for one concept — the same confusion that lets an
// assert site pick the wrong constructor. Aliasing makes those copies identity
// conversions and makes every Fact helper (ToAtom, ArgString, Extract*)
// immediately available on kernel-bridge facts.
type KernelFact = Fact

// ToFact returns the fact unchanged.
//
// Deprecated: KernelFact is an alias for Fact, so this conversion is a no-op.
// It survives only so call sites written against the old two-type world keep
// compiling; delete it when KernelFact itself is removed.
func (f Fact) ToFact() Fact { return f }


// =============================================================================
// STRUCTURED INTENT - Parsed User Intent
// =============================================================================

// StructuredIntent represents the parsed user intent from the perception transducer.
type StructuredIntent struct {
	ID         string // Unique intent ID
	Category   string // /query, /mutation, /instruction
	Verb       string // /explain, /refactor, /debug, /generate
	Target     string // File, symbol, or concept target
	Constraint string // Additional constraints
}

// =============================================================================
// TOOL INFO - Registered Tool Metadata
// =============================================================================

// ToolInfo contains information about a registered tool.
// This is the canonical definition - both core and autopoiesis should use this.
type ToolInfo struct {
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	BinaryPath   string    `json:"binary_path"`
	Hash         string    `json:"hash"`
	RegisteredAt time.Time `json:"registered_at"`
	ExecuteCount int64     `json:"execute_count"`
}

// =============================================================================
// SHARD SUMMARY - Compressed Execution History
// =============================================================================

// ShardSummary represents a compressed summary of a prior shard execution.
type ShardSummary struct {
	ShardType string    // "reviewer", "coder", "tester", "researcher"
	Task      string    // Original task (truncated)
	Summary   string    // Compressed output summary
	Timestamp time.Time // When executed
	Success   bool      // Whether it succeeded
}

// =============================================================================
// KNOWLEDGE SUMMARY - LLM-First Knowledge Discovery
// =============================================================================

// KnowledgeSummary represents knowledge gathered from a specialist consultation.
// Used for handoff to action shards (coder, tester) when user wants to act on
// information gathered during knowledge discovery.
type KnowledgeSummary struct {
	Specialist string // The specialist/agent that provided this knowledge
	Topic      string // The query/topic that was researched
	Summary    string // Truncated summary for context budget management
	FullOutput string // Complete response (may be stored separately for retrieval)
}

// ToolExecutionSummary provides a summary of a tool execution for LLM context.
// This is a lightweight representation for SessionContext (full data in ToolStore SQLite).
type ToolExecutionSummary struct {
	CallID     string // Unique identifier for the execution
	ToolName   string // Name of the tool executed
	Action     string // The action that triggered the tool
	Success    bool   // Whether execution succeeded
	ResultSize int    // Size of the result in bytes
	DurationMs int64  // Execution duration in milliseconds
	Summary    string // Truncated result for context budget (first 500 chars)
}

// =============================================================================
// SESSION CONTEXT - Blackboard Pattern
// =============================================================================

// AmbientContext provides context about the user's active IDE workspace environment.
type AmbientContext struct {
	ActiveFile   string
	CursorLine   int
	SelectedText string
	Diagnostics  []string
}

// SessionContext holds compressed session context for shard injection (Blackboard Pattern).
// This enables shards to understand the full session history without token explosion.
// Extended to include all context types specified in the codeNERD architecture.
//
// Layout decision (OPEN-QUESTIONS Q4): the field groups below stay FLAT.
// Nesting them into sub-structs (Git, TDD, Campaign, …) was considered and
// rejected: every populate* function in cmd/nerd/chat/model_session_context.go
// and every prompt assembler reads these by direct field name, so nesting is a
// rename of ~40 fields across packages that buys navigability only — the
// section banners already give that. Revisit only when a section needs its own
// behaviour (methods, zero-value semantics, or independent serialization),
// which is the point at which a struct earns its existence.
type SessionContext struct {
	// ==========================================================================
	// CORE CONTEXT (Original)
	// ==========================================================================
	CompressedHistory string            // Semantically compressed session (from compressor)
	RecentFindings    []string          // Recent reviewer/tester findings
	RecentActions     []string          // Recent shard actions taken
	ActiveFiles       []string          // Files currently in focus
	ExtraContext      map[string]string // Additional context key-values
	Ambient           *AmbientContext   // Current ambient IDE context (cursor, selection, etc.)

	// ==========================================================================
	// DREAM MODE (Simulation/Learning)
	// ==========================================================================
	DreamMode bool // When true, shard should ONLY describe what it would do, not execute

	// ==========================================================================
	// WORLD MODEL / EDB FACTS
	// ==========================================================================
	ImpactedFiles      []string // Files transitively affected by current changes (impacted/1)
	CurrentDiagnostics []string // Active errors/warnings from diagnostic/5
	SymbolContext      []string // Relevant symbols in scope (symbol_graph)
	DependencyContext  []string // 1-hop dependencies for target file(s)

	// ==========================================================================
	// USER INTENT & FOCUS
	// ==========================================================================
	UserIntent       *StructuredIntent // Parsed intent from perception transducer
	FocusResolutions []string          // Resolved paths from fuzzy references

	// ==========================================================================
	// CAMPAIGN CONTEXT (Multi-Phase Goals)
	// ==========================================================================
	CampaignActive     bool     // Whether a campaign is in progress
	CampaignPhase      string   // Current phase name/ID
	CampaignGoal       string   // Current phase objective
	TaskDependencies   []string // What this task depends on (blocking tasks)
	LinkedRequirements []string // Requirements/specs this task fulfills

	// ==========================================================================
	// GIT STATE / CHESTERTON'S FENCE
	// ==========================================================================
	GitBranch        string   // Current branch name
	GitModifiedFiles []string // Uncommitted/modified files
	GitRecentCommits []string // Recent commit messages (for Chesterton's Fence)
	GitUnstagedCount int      // Number of unstaged changes

	// ==========================================================================
	// TEST STATE (TDD LOOP)
	// ==========================================================================
	TestState     string   // /passing, /failing, /pending, /unknown
	FailingTests  []string // Names/paths of failing tests
	TDDRetryCount int      // Current TDD repair loop iteration

	// ==========================================================================
	// CROSS-SHARD EXECUTION HISTORY
	// ==========================================================================
	PriorShardOutputs []ShardSummary // Recent shard executions with summaries

	// ==========================================================================
	// DOMAIN KNOWLEDGE (Type B Specialists)
	// ==========================================================================
	KnowledgeAtoms  []string // Relevant domain expertise facts
	SpecialistHints []string // Hints from specialist knowledge base

	// ==========================================================================
	// REFLECTION HITS (System 2 Memory)
	// ==========================================================================
	ReflectionHits []string // Summaries of recalled traces/learnings

	// ==========================================================================
	// GATHERED KNOWLEDGE (LLM-First Knowledge Discovery)
	// ==========================================================================
	// Knowledge gathered from specialists during the current session.
	// Populated by the TUI when LLM requests specialist consultation.
	GatheredKnowledge []KnowledgeSummary

	// ==========================================================================
	// AVAILABLE TOOLS (Autopoiesis/Ouroboros)
	// ==========================================================================
	AvailableTools []ToolInfo // Self-generated tools available for execution

	// ==========================================================================
	// RECENT TOOL EXECUTIONS (for LLM context awareness)
	// ==========================================================================
	RecentToolExecutions []ToolExecutionSummary // Recent tool results for context

	// ==========================================================================
	// CONSTITUTIONAL CONSTRAINTS
	// ==========================================================================
	AllowedActions []string // Permitted actions for this shard
	BlockedActions []string // Explicitly denied actions
	SafetyWarnings []string // Active safety concerns
}
