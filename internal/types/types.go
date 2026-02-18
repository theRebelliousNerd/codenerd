// Package types provides shared type definitions used across codeNERD packages.
// This package exists to break import cycles between core, articulation, and autopoiesis.
// Types in this package should be foundational data structures with no complex dependencies.
package types

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/mangle/ast"
)

// =============================================================================
// MANGLE FACT TYPES
// =============================================================================

// MangleAtom represents a Mangle name constant (starting with /).
// This explicit type avoids ambiguity between strings and atoms.
type MangleAtom string

// Fact represents a single logical fact (atom) in the EDB.
type Fact struct {
	Predicate string
	Args      []interface{}
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

	// More than 2 path segments indicates a file path
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
			args = append(args, fmt.Sprintf("%f", v))
		case bool:
			if v {
				args = append(args, "/true")
			} else {
				args = append(args, "/false")
			}
		default:
			args = append(args, fmt.Sprintf("%v", v))
		}
	}
	return fmt.Sprintf("%s(%s).", f.Predicate, strings.Join(args, ", "))
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
			terms = append(terms, ast.Number(int64(float64(v)*100)))
		case float64:
			terms = append(terms, ast.Number(int64(v*100)))
		case time.Time:
			// Store as Unix nanoseconds (ast.TimeType not available in v0.4.0)
			terms = append(terms, ast.Number(v.UnixNano()))
		case time.Duration:
			// Store as nanoseconds integer
			terms = append(terms, ast.Number(int64(v)))
		case bool:
			if v {
				terms = append(terms, ast.TrueConstant)
			} else {
				terms = append(terms, ast.FalseConstant)
			}
		default:
			terms = append(terms, ast.String(fmt.Sprintf("%v", v)))
		}
	}

	return ast.NewAtom(f.Predicate, terms...), nil
}

// =============================================================================
// KERNEL INTERFACE - Bridge to Mangle Logic Core
// =============================================================================

// KernelFact represents a fact that can be asserted to the kernel.
// This is the interface-friendly version of Fact for the kernel bridge.
type KernelFact struct {
	Predicate string
	Args      []interface{}
}

// ToFact converts a KernelFact to a Fact.
func (kf KernelFact) ToFact() Fact {
	return Fact{
		Predicate: kf.Predicate,
		Args:      kf.Args,
	}
}

// KernelInterface defines the interface for interacting with the Mangle kernel.
// This allows packages to assert facts and query for derived actions without
// importing the full kernel implementation.
type KernelInterface interface {
	// AssertFact adds a fact to the kernel's EDB
	AssertFact(fact KernelFact) error
	// AssertFactBatch adds multiple facts and evaluates once (much faster than multiple AssertFact calls)
	AssertFactBatch(facts []KernelFact) error
	// QueryPredicate queries for facts matching a predicate
	QueryPredicate(predicate string) ([]KernelFact, error)
	// QueryBool returns true if any facts match the predicate
	QueryBool(predicate string) bool
	// RetractFact removes a fact from the kernel (matching predicate and first arg)
	RetractFact(fact KernelFact) error
}

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

// SessionContext holds compressed session context for shard injection (Blackboard Pattern).
// This enables shards to understand the full session history without token explosion.
// Extended to include all context types specified in the codeNERD architecture.
type SessionContext struct {
	// ==========================================================================
	// CORE CONTEXT (Original)
	// ==========================================================================
	CompressedHistory string            // Semantically compressed session (from compressor)
	RecentFindings    []string          // Recent reviewer/tester findings
	RecentActions     []string          // Recent shard actions taken
	ActiveFiles       []string          // Files currently in focus
	ExtraContext      map[string]string // Additional context key-values

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
