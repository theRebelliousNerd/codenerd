// Package init implements the "nerd init" cold-start initialization system.
// This file contains the shared knowledge pool for common concepts across all agents.
package init

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/logging"
	// researcher removed - JIT clean loop handles research
	"codenerd/internal/store"
)

// SharedKnowledgeTopics defines common topics that ALL specialists share.
// These are researched once and inherited by individual agents.
var SharedKnowledgeTopics = []string{
	"error handling best practices",
	"logging patterns",
	"testing strategies",
	"code review guidelines",
	"documentation standards",
}

// SharedKnowledgeAtom represents a pre-defined knowledge atom for shared concepts.
type SharedKnowledgeAtom struct {
	Concept    string
	Content    string
	Confidence float64
}

// BaseSharedAtoms contains foundational knowledge atoms that don't require API calls.
var BaseSharedAtoms = []SharedKnowledgeAtom{
	{
		Concept:    "error_handling_principle",
		Content:    "Always handle errors explicitly. Never ignore errors with _ assignment unless you have a documented reason. Wrap errors with context using fmt.Errorf with %w verb to maintain error chain.",
		Confidence: 1.0,
	},
	{
		Concept:    "logging_structured",
		Content:    "Use structured logging with key-value pairs. Include request IDs, timestamps, and contextual information. Use appropriate log levels: Debug for development, Info for normal operations, Warn for recoverable issues, Error for failures.",
		Confidence: 1.0,
	},
	{
		Concept:    "testing_table_driven",
		Content:    "Prefer table-driven tests for comprehensive coverage. Each test case should be independent and idempotent. Use subtests with t.Run() for better organization and parallel execution.",
		Confidence: 1.0,
	},
	{
		Concept:    "testing_mocking",
		Content:    "Use interfaces for dependencies to enable mocking. Keep mocks simple and focused. Consider using generated mocks for complex interfaces. Test behavior, not implementation details.",
		Confidence: 1.0,
	},
	{
		Concept:    "code_review_focus",
		Content:    "Code reviews should focus on: correctness, security, performance, maintainability, and readability. Provide specific, actionable feedback. Prefer questions over demands.",
		Confidence: 1.0,
	},
	{
		Concept:    "documentation_godoc",
		Content:    "Document all exported functions, types, and packages. First sentence should be a complete sentence starting with the name being documented. Include examples for complex functionality.",
		Confidence: 1.0,
	},
	{
		Concept:    "context_usage",
		Content:    "Pass context.Context as the first parameter to functions that perform I/O or may be long-running. Use context for cancellation, deadlines, and request-scoped values. Never store context in structs.",
		Confidence: 1.0,
	},
	{
		Concept:    "dependency_injection",
		Content:    "Inject dependencies through constructors rather than creating them internally. This improves testability and makes dependencies explicit. Use interfaces for external dependencies.",
		Confidence: 1.0,
	},
	{
		Concept:    "security_input_validation",
		Content:    "Validate all external input at system boundaries. Never trust user input. Use allowlists over denylists. Sanitize data before use in SQL, HTML, or shell commands.",
		Confidence: 1.0,
	},
	{
		Concept:    "performance_profiling",
		Content:    "Profile before optimizing. Use pprof for CPU and memory profiling. Benchmark critical paths with testing.B. Avoid premature optimization but address known hotspots.",
		Confidence: 1.0,
	},
	{
		Concept:    "concurrency_goroutine_lifecycle",
		Content:    "Always manage goroutine lifecycle explicitly. Use sync.WaitGroup for coordination. Ensure goroutines can be cancelled via context. Never start goroutines without a clear termination strategy.",
		Confidence: 1.0,
	},
	{
		Concept:    "api_design_rest",
		Content:    "Use appropriate HTTP methods (GET for reads, POST for creates, PUT/PATCH for updates, DELETE for deletions). Return meaningful status codes. Include pagination for list endpoints.",
		Confidence: 1.0,
	},
	{
		Concept:    "configuration_management",
		Content:    "Use environment variables for deployment-specific config. Support configuration files for complex settings. Validate configuration at startup. Provide sensible defaults.",
		Confidence: 1.0,
	},
}

// CreateSharedKnowledgePool creates the shared knowledge database with common concepts.
// This should be called BEFORE individual agent KBs are created.
// NOTE: Research functionality removed as part of JIT refactor - only base atoms are added.
func CreateSharedKnowledgePool(ctx context.Context, projectPath string, callback func(status string, progress float64)) error {
	timer := logging.StartTimer(logging.CategoryBoot, "CreateSharedKnowledgePool")
	defer timer.Stop()

	sharedDBPath := filepath.Join(projectPath, ".nerd", "shards", "core_concepts.db")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(sharedDBPath), 0755); err != nil {
		return fmt.Errorf("failed to create shards directory: %w", err)
	}

	logging.Boot("Creating shared knowledge pool at %s", sharedDBPath)
	if callback != nil {
		callback("Creating shared knowledge pool...", 0.0)
	}

	// Create/open the shared knowledge store
	sharedStore, err := store.NewLocalStore(sharedDBPath)
	if err != nil {
		return fmt.Errorf("failed to create shared knowledge store: %w", err)
	}
	defer sharedStore.Close()

	// Add base atoms (no API calls needed)
	// Research functionality removed - JIT clean loop handles on-demand research
	logging.Boot("Adding %d base shared atoms", len(BaseSharedAtoms))
	for _, baseAtom := range BaseSharedAtoms {
		if err := sharedStore.StoreKnowledgeAtom(baseAtom.Concept, baseAtom.Content, baseAtom.Confidence); err != nil {
			logging.Boot("Warning: failed to store base atom %s: %v", baseAtom.Concept, err)
		}
	}

	if callback != nil {
		callback("Base shared knowledge added", 0.5)
	}

	// Additional research topics are now handled on-demand via JIT clean loop
	logging.Boot("Additional research available on-demand via JIT (topics: %d)", len(SharedKnowledgeTopics))

	if callback != nil {
		callback("Shared knowledge pool created", 1.0)
	}

	logging.Boot("Shared knowledge pool created with base + researched atoms")
	return nil
}

// InheritSharedKnowledge copies shared atoms into an agent's knowledge base.
// This avoids redundant research and ensures consistency across agents.
func InheritSharedKnowledge(agentStore *store.LocalStore, sharedStorePath string) error {
	sharedStore, err := store.NewLocalStore(sharedStorePath)
	if err != nil {
		return fmt.Errorf("failed to open shared knowledge store: %w", err)
	}
	defer sharedStore.Close()

	// Get all shared atoms
	sharedAtoms, err := sharedStore.GetAllKnowledgeAtoms()
	if err != nil {
		return fmt.Errorf("failed to get shared atoms: %w", err)
	}

	logging.Boot("Inheriting %d shared atoms into agent KB", len(sharedAtoms))

	// Copy each atom to the agent's store
	inherited := 0
	for _, atom := range sharedAtoms {
		// Store with "inherited:" prefix in concept to mark source
		concept := "inherited:" + atom.Concept
		if err := agentStore.StoreKnowledgeAtom(concept, atom.Content, atom.Confidence); err != nil {
			logging.Boot("Warning: failed to inherit atom %s: %v", atom.Concept, err)
			continue
		}
		inherited++
	}

	logging.Boot("Inherited %d/%d shared atoms", inherited, len(sharedAtoms))
	return nil
}

// GetSharedKnowledgePath returns the path to the shared knowledge database.
func GetSharedKnowledgePath(projectPath string) string {
	return filepath.Join(projectPath, ".nerd", "shards", "core_concepts.db")
}

// SharedKnowledgePoolExists checks if the shared knowledge pool has been created.
func SharedKnowledgePoolExists(projectPath string) bool {
	path := GetSharedKnowledgePath(projectPath)
	_, err := os.Stat(path)
	return err == nil
}
