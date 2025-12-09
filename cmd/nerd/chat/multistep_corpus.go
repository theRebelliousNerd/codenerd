// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains the encyclopedic multi-step task corpus for detecting
// and decomposing complex user requests into discrete, executable steps.
package chat

import (
	"regexp"
	"strings"

	"codenerd/internal/perception"
)

// =============================================================================
// MULTI-STEP PATTERN TAXONOMY
// =============================================================================
// This corpus provides comprehensive coverage of multi-step task patterns.
// Each pattern type has detection rules, decomposition logic, and dependency inference.

// StepRelation defines how steps relate to each other
type StepRelation string

const (
	RelationSequential  StepRelation = "sequential"  // Step N must complete before N+1
	RelationParallel    StepRelation = "parallel"    // Steps can run concurrently
	RelationConditional StepRelation = "conditional" // Step N+1 runs only if N succeeds
	RelationFallback    StepRelation = "fallback"    // Step N+1 runs only if N fails
	RelationIterative   StepRelation = "iterative"   // Step repeats over a collection
)

// MultiStepPattern defines a pattern for detecting and decomposing multi-step tasks
type MultiStepPattern struct {
	Name        string           // Human-readable name
	Category    string           // Pattern category
	Patterns    []*regexp.Regexp // Detection patterns
	Keywords    []string         // Trigger keywords
	Relation    StepRelation     // Default step relation
	Priority    int              // Higher = matched first
	Decomposer  string           // Which decomposition strategy to use
	Examples    []string         // Example user inputs
	VerbPairs   [][2]string      // Common verb combinations [verb1, verb2]
}

// =============================================================================
// PATTERN CATEGORIES
// =============================================================================

const (
	CategorySequentialExplicit   = "sequential_explicit"   // "first X, then Y"
	CategorySequentialImplicit   = "sequential_implicit"   // "X and Y" with implied order
	CategoryConditionalSuccess   = "conditional_success"   // "X, if it works, Y"
	CategoryConditionalFailure   = "conditional_failure"   // "X, if it fails, Y"
	CategoryParallelIndependent  = "parallel_independent"  // "X and Y" (no dependency)
	CategoryCompoundWithRef      = "compound_with_ref"     // "X and Y it" (pronoun ref)
	CategoryIterativeCollection  = "iterative_collection"  // "X each file"
	CategoryPipelineChain        = "pipeline_chain"        // "X then pass to Y"
	CategoryVerifyAfterMutation  = "verify_after_mutation" // "X and make sure/verify"
	CategoryResearchThenAct      = "research_then_act"     // "figure out X then do Y"
	CategoryReviewThenFix        = "review_then_fix"       // "review and fix"
	CategoryCreateThenValidate   = "create_then_validate"  // "create and test"
	CategoryRefactorPreserve     = "refactor_preserve"     // "refactor but keep X"
	CategoryBatchOperation       = "batch_operation"       // "do X to all Y"
	CategoryUndoRecovery         = "undo_recovery"         // "try X, revert if needed"
	CategoryCompareAndChoose     = "compare_and_choose"    // "compare X and Y, pick best"
	CategoryAnalyzeThenOptimize  = "analyze_then_optimize" // "analyze and improve"
	CategoryDocumentAfterChange  = "document_after_change" // "change X and update docs"
	CategoryTestDrivenFlow       = "test_driven_flow"      // "write tests then implement"
	CategorySecurityAuditFix     = "security_audit_fix"    // "scan and fix vulnerabilities"
)

// =============================================================================
// ENCYCLOPEDIC PATTERN CORPUS
// =============================================================================

// MultiStepCorpus contains all recognized multi-step patterns
var MultiStepCorpus = []MultiStepPattern{
	// =========================================================================
	// SEQUENTIAL EXPLICIT PATTERNS
	// =========================================================================
	{
		Name:     "explicit_first_then",
		Category: CategorySequentialExplicit,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)^first\s+(.+?),?\s+then\s+(.+?)(?:\s*,?\s*(?:and\s+)?finally\s+(.+))?$`),
			regexp.MustCompile(`(?i)^first\s+(.+?)\s+and\s+then\s+(.+)$`),
			regexp.MustCompile(`(?i)^start\s+by\s+(.+?),?\s+then\s+(.+)$`),
			regexp.MustCompile(`(?i)^begin\s+with\s+(.+?),?\s+then\s+(.+)$`),
		},
		Keywords:   []string{"first", "then", "finally", "start by", "begin with"},
		Relation:   RelationSequential,
		Priority:   100,
		Decomposer: "explicit_sequence",
		Examples: []string{
			"first review the code, then fix any issues",
			"first create the file, then add tests, finally commit",
			"start by analyzing the codebase, then refactor the hot spots",
		},
	},
	{
		Name:     "explicit_step_numbers",
		Category: CategorySequentialExplicit,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:step\s*)?1[.):]\s*(.+?)(?:(?:step\s*)?2[.):]\s*(.+?))?(?:(?:step\s*)?3[.):]\s*(.+))?`),
			regexp.MustCompile(`(?i)^\s*1\.\s*(.+?)\s*2\.\s*(.+?)(?:\s*3\.\s*(.+))?$`),
		},
		Keywords:   []string{"step 1", "step 2", "1.", "2.", "3."},
		Relation:   RelationSequential,
		Priority:   95,
		Decomposer: "numbered_steps",
		Examples: []string{
			"1. create the handler 2. add tests 3. update the router",
			"step 1: review, step 2: fix, step 3: test",
		},
	},
	{
		Name:     "explicit_after_that",
		Category: CategorySequentialExplicit,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?after\s+that\s+(.+)$`),
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?afterwards?\s+(.+)$`),
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?following\s+that\s+(.+)$`),
		},
		Keywords:   []string{"after that", "afterward", "afterwards", "following that"},
		Relation:   RelationSequential,
		Priority:   90,
		Decomposer: "explicit_sequence",
		Examples: []string{
			"fix the bug, after that run the tests",
			"refactor the function and afterward update the docs",
		},
	},
	{
		Name:     "explicit_next",
		Category: CategorySequentialExplicit,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?next\s+(.+)$`),
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?subsequently\s+(.+)$`),
		},
		Keywords:   []string{"next", "subsequently"},
		Relation:   RelationSequential,
		Priority:   85,
		Decomposer: "explicit_sequence",
		Examples: []string{
			"create the interface, next implement it",
			"review the PR and next merge it",
		},
	},
	{
		Name:     "explicit_once_done",
		Category: CategorySequentialExplicit,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?once\s+(?:that'?s?\s+)?done\s+(.+)$`),
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?when\s+(?:that'?s?\s+)?(?:done|finished|complete)\s+(.+)$`),
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?after\s+(?:you(?:'re)?\s+)?(?:done|finished)\s+(.+)$`),
		},
		Keywords:   []string{"once done", "when done", "when finished", "after done", "after finished"},
		Relation:   RelationSequential,
		Priority:   88,
		Decomposer: "explicit_sequence",
		Examples: []string{
			"fix the tests, once done commit the changes",
			"refactor that function and when you're done run the benchmarks",
		},
	},

	// =========================================================================
	// SEQUENTIAL IMPLICIT PATTERNS (verb semantics imply order)
	// =========================================================================
	{
		Name:     "implicit_review_fix",
		Category: CategoryReviewThenFix,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)review\s+(.+?)\s+(?:and\s+)?fix\s+(?:any\s+)?(?:issues?|problems?|bugs?|errors?)?`),
			regexp.MustCompile(`(?i)check\s+(.+?)\s+(?:and\s+)?(?:fix|repair|resolve)\s+`),
			regexp.MustCompile(`(?i)audit\s+(.+?)\s+(?:and\s+)?(?:fix|address|resolve)\s+`),
			regexp.MustCompile(`(?i)find\s+(?:and\s+)?fix\s+(?:issues?|bugs?|problems?)?\s*(?:in\s+)?(.+)?`),
		},
		Keywords: []string{"review and fix", "check and fix", "find and fix", "audit and fix"},
		Relation: RelationSequential,
		Priority: 92,
		VerbPairs: [][2]string{
			{"/review", "/fix"},
			{"/analyze", "/fix"},
			{"/security", "/fix"},
		},
		Decomposer: "verb_pair_chain",
		Examples: []string{
			"review auth.go and fix any issues",
			"check the handlers and fix any bugs",
			"find and fix all security issues",
		},
	},
	{
		Name:     "implicit_create_test",
		Category: CategoryCreateThenValidate,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:create|implement|add|write|build)\s+(.+?)\s+(?:and\s+)?(?:test|write\s+tests?|add\s+tests?)`),
			regexp.MustCompile(`(?i)(?:create|implement|add|write|build)\s+(.+?)\s+(?:and\s+)?(?:make\s+sure|verify|ensure)\s+(?:it\s+)?works?`),
		},
		Keywords: []string{"create and test", "implement and test", "add and test", "build and test"},
		Relation: RelationSequential,
		Priority: 90,
		VerbPairs: [][2]string{
			{"/create", "/test"},
			{"/fix", "/test"},
			{"/refactor", "/test"},
		},
		Decomposer: "verb_pair_chain",
		Examples: []string{
			"create a new handler and test it",
			"implement the feature and write tests",
			"add the endpoint and make sure it works",
		},
	},
	{
		Name:     "implicit_fix_verify",
		Category: CategoryVerifyAfterMutation,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:fix|change|update|modify|refactor)\s+(.+?)\s+(?:and\s+)?(?:verify|confirm|check|ensure|make\s+sure)`),
			regexp.MustCompile(`(?i)(?:fix|change|update|modify|refactor)\s+(.+?)\s+(?:and\s+)?(?:run\s+(?:the\s+)?tests?|test\s+it)`),
		},
		Keywords: []string{"fix and verify", "change and test", "update and check", "fix and run tests"},
		Relation: RelationSequential,
		Priority: 88,
		VerbPairs: [][2]string{
			{"/fix", "/test"},
			{"/refactor", "/test"},
			{"/create", "/test"},
		},
		Decomposer: "verb_pair_chain",
		Examples: []string{
			"fix the authentication and verify it works",
			"change the handler and run the tests",
			"update the config and make sure nothing breaks",
		},
	},
	{
		Name:     "implicit_research_implement",
		Category: CategoryResearchThenAct,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:research|figure\s+out|learn|understand|look\s+(?:up|into))\s+(?:how\s+to\s+)?(.+?)\s+(?:and\s+)?(?:then\s+)?(?:implement|create|build|add|do)`),
			regexp.MustCompile(`(?i)(?:find\s+out|investigate)\s+(.+?)\s+(?:and\s+)?(?:then\s+)?(?:implement|fix|create)`),
		},
		Keywords: []string{"research and implement", "figure out and", "learn how to and", "understand and then"},
		Relation: RelationSequential,
		Priority: 85,
		VerbPairs: [][2]string{
			{"/research", "/create"},
			{"/research", "/fix"},
			{"/explore", "/create"},
		},
		Decomposer: "verb_pair_chain",
		Examples: []string{
			"research how to implement OAuth and then add it",
			"figure out the API and implement the client",
			"understand the codebase structure and then refactor",
		},
	},
	{
		Name:     "implicit_analyze_optimize",
		Category: CategoryAnalyzeThenOptimize,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:analyze|profile|benchmark|measure)\s+(.+?)\s+(?:and\s+)?(?:optimize|improve|speed\s+up|fix)`),
			regexp.MustCompile(`(?i)(?:find|identify)\s+(?:performance\s+)?(?:issues?|bottlenecks?|problems?)\s+(?:in\s+)?(.+?)\s+(?:and\s+)?(?:fix|optimize|improve)`),
		},
		Keywords: []string{"analyze and optimize", "profile and improve", "find bottlenecks and fix"},
		Relation: RelationSequential,
		Priority: 85,
		VerbPairs: [][2]string{
			{"/analyze", "/refactor"},
			{"/analyze", "/fix"},
		},
		Decomposer: "verb_pair_chain",
		Examples: []string{
			"analyze the performance and optimize the hot paths",
			"profile the API and improve response times",
			"find bottlenecks in the database layer and fix them",
		},
	},
	{
		Name:     "implicit_security_fix",
		Category: CategorySecurityAuditFix,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:security\s+)?(?:scan|audit|check)\s+(.+?)\s+(?:for\s+(?:vulnerabilities|issues))?\s*(?:and\s+)?(?:fix|patch|resolve|address)`),
			regexp.MustCompile(`(?i)find\s+(?:security\s+)?(?:vulnerabilities|issues)\s+(?:in\s+)?(.+?)\s+(?:and\s+)?(?:fix|patch|resolve)`),
		},
		Keywords: []string{"security scan and fix", "audit and fix", "find vulnerabilities and fix"},
		Relation: RelationSequential,
		Priority: 93,
		VerbPairs: [][2]string{
			{"/security", "/fix"},
		},
		Decomposer: "verb_pair_chain",
		Examples: []string{
			"security scan the API handlers and fix any vulnerabilities",
			"audit the auth module and patch any issues",
			"find security issues in the input validation and fix them",
		},
	},
	{
		Name:     "implicit_change_document",
		Category: CategoryDocumentAfterChange,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:change|update|modify|refactor|add)\s+(.+?)\s+(?:and\s+)?(?:update\s+(?:the\s+)?(?:docs?|documentation)|document)`),
			regexp.MustCompile(`(?i)(?:change|update|modify|refactor|add)\s+(.+?)\s+(?:and\s+)?(?:add\s+(?:a\s+)?comment|comment)`),
		},
		Keywords: []string{"and update docs", "and document", "and add comments"},
		Relation: RelationSequential,
		Priority: 80,
		VerbPairs: [][2]string{
			{"/refactor", "/document"},
			{"/create", "/document"},
			{"/fix", "/document"},
		},
		Decomposer: "verb_pair_chain",
		Examples: []string{
			"refactor the handler and update the documentation",
			"add the new endpoint and document it",
			"change the algorithm and add comments explaining it",
		},
	},

	// =========================================================================
	// TEST-DRIVEN PATTERNS
	// =========================================================================
	{
		Name:     "tdd_test_first",
		Category: CategoryTestDrivenFlow,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:write|create|add)\s+(?:the\s+)?tests?\s+(?:first\s+)?(?:for\s+)?(.+?)\s+(?:and\s+)?(?:then\s+)?(?:implement|create|build|write\s+(?:the\s+)?(?:code|implementation))`),
			regexp.MustCompile(`(?i)tdd\s+(.+)`),
			regexp.MustCompile(`(?i)test[- ]?driven\s+(.+)`),
		},
		Keywords: []string{"write tests first", "tdd", "test-driven", "tests then implement"},
		Relation: RelationSequential,
		Priority: 88,
		VerbPairs: [][2]string{
			{"/test", "/create"},
		},
		Decomposer: "verb_pair_chain",
		Examples: []string{
			"write tests for the parser first, then implement it",
			"TDD the new authentication flow",
			"create tests and then the implementation for the cache",
		},
	},

	// =========================================================================
	// CONDITIONAL SUCCESS PATTERNS
	// =========================================================================
	{
		Name:     "conditional_if_success",
		Category: CategoryConditionalSuccess,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?if\s+(?:it\s+)?(?:works?|succeeds?|passes?|is\s+successful)\s*,?\s+(?:then\s+)?(.+)$`),
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?(?:assuming|provided)\s+(?:it\s+)?(?:works?|succeeds?)\s*,?\s+(.+)$`),
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?on\s+success\s*,?\s+(.+)$`),
		},
		Keywords: []string{"if it works", "if successful", "if it passes", "on success", "assuming it works"},
		Relation: RelationConditional,
		Priority: 85,
		Decomposer: "conditional_chain",
		Examples: []string{
			"run the tests, if they pass, commit",
			"fix the bug and if it works deploy to staging",
			"refactor and on success merge the PR",
		},
	},
	{
		Name:     "conditional_tests_pass",
		Category: CategoryConditionalSuccess,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?if\s+(?:the\s+)?tests?\s+pass(?:es)?\s*,?\s+(?:then\s+)?(.+)$`),
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?(?:once|when)\s+(?:the\s+)?tests?\s+(?:pass(?:es)?|(?:are\s+)?green)\s*,?\s+(.+)$`),
		},
		Keywords: []string{"if tests pass", "when tests pass", "once tests are green"},
		Relation: RelationConditional,
		Priority: 87,
		Decomposer: "conditional_chain",
		Examples: []string{
			"fix the handler, if the tests pass, push to main",
			"refactor and when tests are green merge",
		},
	},

	// =========================================================================
	// CONDITIONAL FAILURE / FALLBACK PATTERNS
	// =========================================================================
	{
		Name:     "fallback_if_fails",
		Category: CategoryConditionalFailure,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:try\s+(?:to\s+)?)?(.+?),?\s+(?:and\s+)?if\s+(?:it\s+)?(?:fails?|doesn'?t\s+work|breaks?)\s*,?\s+(?:then\s+)?(.+)$`),
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?(?:otherwise|or\s+else)\s+(.+)$`),
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?on\s+(?:failure|error)\s*,?\s+(.+)$`),
		},
		Keywords: []string{"if it fails", "if it doesn't work", "otherwise", "or else", "on failure"},
		Relation: RelationFallback,
		Priority: 83,
		Decomposer: "fallback_chain",
		Examples: []string{
			"try the migration, if it fails, rollback",
			"apply the patch, otherwise revert to the backup",
			"run the deployment and on failure alert the team",
		},
	},
	{
		Name:     "fallback_try_revert",
		Category: CategoryUndoRecovery,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:try\s+(?:to\s+)?)?(.+?),?\s+(?:and\s+)?(?:revert|rollback|undo)\s+if\s+(?:it\s+)?(?:fails?|(?:doesn'?t|does\s+not)\s+work|breaks?)`),
			regexp.MustCompile(`(?i)(.+?),?\s+(?:but\s+)?(?:be\s+ready\s+to\s+)?(?:revert|rollback)\s+if\s+(?:needed|necessary|something\s+goes\s+wrong)`),
		},
		Keywords: []string{"revert if fails", "rollback if", "undo if fails", "revert if needed"},
		Relation: RelationFallback,
		Priority: 82,
		Decomposer: "fallback_chain",
		Examples: []string{
			"try the database migration, revert if it fails",
			"apply the changes but be ready to rollback if needed",
			"deploy to production and rollback if something goes wrong",
		},
	},

	// =========================================================================
	// PARALLEL / INDEPENDENT PATTERNS
	// =========================================================================
	{
		Name:     "parallel_independent_and",
		Category: CategoryParallelIndependent,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(review|analyze|scan|check)\s+(.+?)\s+and\s+(review|analyze|scan|check)\s+(.+)`),
			regexp.MustCompile(`(?i)(?:both\s+)?(review|analyze|scan)\s+(.+?)\s+and\s+(.+)`),
		},
		Keywords: []string{"review X and review Y", "scan X and scan Y"},
		Relation: RelationParallel,
		Priority: 75,
		Decomposer: "parallel_split",
		Examples: []string{
			"review auth.go and review handler.go",
			"analyze the frontend and the backend",
			"scan the API and scan the database layer",
		},
	},
	{
		Name:     "parallel_also_additionally",
		Category: CategoryParallelIndependent,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?also\s+(.+)$`),
			regexp.MustCompile(`(?i)(.+?),?\s+additionally\s+(.+)$`),
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?(?:at\s+the\s+same\s+time|simultaneously|in\s+parallel)\s+(.+)$`),
		},
		Keywords: []string{"also", "additionally", "at the same time", "simultaneously", "in parallel"},
		Relation: RelationParallel,
		Priority: 70,
		Decomposer: "parallel_split",
		Examples: []string{
			"review the API, also check the tests",
			"fix the bug, additionally update the changelog",
			"run lint and at the same time run tests",
		},
	},

	// =========================================================================
	// COMPOUND WITH REFERENCE PATTERNS ("X and Y it")
	// =========================================================================
	{
		Name:     "compound_pronoun_ref",
		Category: CategoryCompoundWithRef,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(review|analyze|check|fix|create|implement|refactor)\s+(.+?)\s+and\s+(test|document|commit|deploy|push|merge)\s+it`),
			regexp.MustCompile(`(?i)(create|implement|write|build)\s+(.+?)\s+and\s+(test|run|execute|verify)\s+(?:it|them)`),
		},
		Keywords: []string{"and test it", "and commit it", "and deploy it", "and document it"},
		Relation: RelationSequential,
		Priority: 88,
		Decomposer: "pronoun_resolution",
		Examples: []string{
			"create the handler and test it",
			"fix the bug and commit it",
			"implement the feature and deploy it",
			"write the tests and run them",
		},
	},

	// =========================================================================
	// ITERATIVE / BATCH PATTERNS
	// =========================================================================
	{
		Name:     "iterative_each_every",
		Category: CategoryIterativeCollection,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(review|fix|refactor|test|update|check)\s+(?:each|every)\s+(.+?)(?:\s+in\s+(.+))?$`),
			regexp.MustCompile(`(?i)(review|fix|refactor|test|update|check)\s+all\s+(?:the\s+)?(.+?)(?:\s+in\s+(.+))?$`),
			regexp.MustCompile(`(?i)for\s+(?:each|every)\s+(.+?)\s*[,:]\s*(review|fix|refactor|test|update|check)`),
		},
		Keywords: []string{"each", "every", "all the", "for each", "for every"},
		Relation: RelationIterative,
		Priority: 80,
		Decomposer: "iterative_expansion",
		Examples: []string{
			"review each handler in cmd/api/",
			"fix every failing test",
			"refactor all the deprecated functions",
			"for each model, add validation",
		},
	},
	{
		Name:     "batch_all_files",
		Category: CategoryBatchOperation,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(review|fix|refactor|test|update|check|format|lint)\s+all\s+(?:the\s+)?(?:go|python|javascript|typescript|rust|java)\s+files?`),
			regexp.MustCompile(`(?i)(review|fix|refactor|test|update|check|format|lint)\s+(?:the\s+)?(?:entire\s+)?(?:codebase|project|repo(?:sitory)?)`),
		},
		Keywords: []string{"all files", "entire codebase", "whole project", "all go files"},
		Relation: RelationIterative,
		Priority: 78,
		Decomposer: "batch_expansion",
		Examples: []string{
			"format all go files",
			"lint the entire codebase",
			"review all typescript files",
		},
	},

	// =========================================================================
	// PIPELINE / CHAIN PATTERNS
	// =========================================================================
	{
		Name:     "pipeline_pass_output",
		Category: CategoryPipelineChain,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?(?:then\s+)?(?:pass|feed|send|pipe)\s+(?:the\s+)?(?:results?|output|findings?)\s+to\s+(.+)`),
			regexp.MustCompile(`(?i)(.+?),?\s+(?:and\s+)?use\s+(?:the\s+)?(?:results?|output|findings?)\s+(?:to|for)\s+(.+)`),
		},
		Keywords: []string{"pass the results to", "feed output to", "use the results to", "pipe to"},
		Relation: RelationSequential,
		Priority: 85,
		Decomposer: "pipeline_chain",
		Examples: []string{
			"analyze the code and pass the results to the optimizer",
			"review for security issues and use the findings to fix",
			"run static analysis and feed output to the report generator",
		},
	},
	{
		Name:     "pipeline_based_on",
		Category: CategoryPipelineChain,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(review|analyze|scan|check)\s+(.+?)\s+(?:and\s+)?(?:then\s+)?(fix|refactor|update|improve)\s+(?:based\s+on|according\s+to)\s+(?:the\s+)?(?:results?|findings?|issues?)`),
		},
		Keywords: []string{"based on the results", "according to findings", "based on issues"},
		Relation: RelationSequential,
		Priority: 86,
		Decomposer: "pipeline_chain",
		Examples: []string{
			"review the handlers and then fix based on the findings",
			"analyze complexity and refactor according to the results",
		},
	},

	// =========================================================================
	// COMPARE AND CHOOSE PATTERNS
	// =========================================================================
	{
		Name:     "compare_and_choose",
		Category: CategoryCompareAndChoose,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)compare\s+(.+?)\s+(?:and|with|to)\s+(.+?)\s+(?:and\s+)?(?:pick|choose|select|use)\s+(?:the\s+)?(?:best|better|preferred)`),
			regexp.MustCompile(`(?i)(?:evaluate|assess)\s+(.+?)\s+(?:and|vs\.?|versus)\s+(.+?)\s+(?:and\s+)?(?:recommend|suggest)`),
		},
		Keywords: []string{"compare and pick", "compare and choose", "evaluate and recommend"},
		Relation: RelationSequential,
		Priority: 75,
		Decomposer: "comparison_chain",
		Examples: []string{
			"compare the two implementations and pick the best",
			"evaluate approach A vs B and recommend one",
		},
	},

	// =========================================================================
	// CONSTRAINT / EXCLUSION PATTERNS
	// =========================================================================
	{
		Name:     "constraint_but_not",
		Category: CategoryRefactorPreserve,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(review|fix|refactor|change|update)\s+(.+?)\s+but\s+(?:not|without|don'?t(?:\s+touch)?|skip(?:ping)?|except(?:ing)?|excluding)\s+(.+)`),
			regexp.MustCompile(`(?i)(review|fix|refactor|change|update)\s+(.+?)\s+(?:while\s+)?(?:keeping|preserving|maintaining)\s+(.+)`),
		},
		Keywords: []string{"but not", "but skip", "except", "excluding", "while keeping", "preserving"},
		Relation: RelationSequential,
		Priority: 82,
		Decomposer: "constrained_operation",
		Examples: []string{
			"refactor the handlers but not the middleware",
			"review all files except tests",
			"update the API while keeping backwards compatibility",
			"fix the auth but don't touch the session logic",
		},
	},

	// =========================================================================
	// GIT WORKFLOW PATTERNS
	// =========================================================================
	{
		Name:     "git_commit_push",
		Category: CategorySequentialImplicit,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:commit|save)\s+(?:the\s+)?(?:changes?)?\s*(?:and\s+)?push`),
			regexp.MustCompile(`(?i)(?:stage|add)\s+(?:the\s+)?(?:changes?|files?)?\s*(?:,?\s*)?(?:and\s+)?commit\s*(?:,?\s*)?(?:and\s+)?push`),
		},
		Keywords: []string{"commit and push", "add and commit", "stage and commit and push"},
		Relation: RelationSequential,
		Priority: 85,
		VerbPairs: [][2]string{
			{"/git", "/git"},
		},
		Decomposer: "git_workflow",
		Examples: []string{
			"commit and push",
			"add the changes, commit, and push",
			"stage everything and commit and push to origin",
		},
	},
	{
		Name:     "git_branch_workflow",
		Category: CategorySequentialImplicit,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:create|make)\s+(?:a\s+)?(?:new\s+)?branch\s+(?:for\s+)?(.+?)\s+(?:and\s+)?(?:then\s+)?(?:start|begin|work\s+on)`),
			regexp.MustCompile(`(?i)(?:checkout|switch\s+to)\s+(.+?)\s+(?:and\s+)?(?:then\s+)?(.+)`),
		},
		Keywords: []string{"create branch and", "checkout and", "switch to and"},
		Relation: RelationSequential,
		Priority: 80,
		Decomposer: "git_workflow",
		Examples: []string{
			"create a new branch for the feature and start working",
			"checkout main and pull the latest",
		},
	},
}

// =============================================================================
// CORPUS ACCESS FUNCTIONS
// =============================================================================

// GetMultiStepPatterns returns all patterns, optionally filtered by category
func GetMultiStepPatterns(category string) []MultiStepPattern {
	if category == "" {
		return MultiStepCorpus
	}
	var filtered []MultiStepPattern
	for _, p := range MultiStepCorpus {
		if p.Category == category {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// GetPatternCategories returns all unique pattern categories
func GetPatternCategories() []string {
	seen := make(map[string]bool)
	var categories []string
	for _, p := range MultiStepCorpus {
		if !seen[p.Category] {
			seen[p.Category] = true
			categories = append(categories, p.Category)
		}
	}
	return categories
}

// MatchMultiStepPattern finds the best matching pattern for input
func MatchMultiStepPattern(input string) (*MultiStepPattern, []string) {
	lower := strings.ToLower(input)

	var bestMatch *MultiStepPattern
	var bestCaptures []string
	bestPriority := -1

	for i := range MultiStepCorpus {
		p := &MultiStepCorpus[i]

		// Check patterns (regex)
		for _, pattern := range p.Patterns {
			matches := pattern.FindStringSubmatch(lower)
			if len(matches) > 0 {
				if p.Priority > bestPriority {
					bestMatch = p
					bestCaptures = matches[1:] // Skip full match
					bestPriority = p.Priority
				}
				break
			}
		}

		// Check keywords if no pattern matched
		if bestMatch == nil || p.Priority > bestPriority {
			for _, keyword := range p.Keywords {
				if strings.Contains(lower, keyword) {
					if p.Priority > bestPriority {
						bestMatch = p
						bestCaptures = nil // Keywords don't capture
						bestPriority = p.Priority
					}
					break
				}
			}
		}
	}

	return bestMatch, bestCaptures
}

// DetectMultiStepFromCorpus uses the encyclopedic corpus for detection
func DetectMultiStepFromCorpus(input string, intent perception.Intent) (bool, *MultiStepPattern, []string) {
	// First try corpus matching
	pattern, captures := MatchMultiStepPattern(input)
	if pattern != nil {
		return true, pattern, captures
	}

	// Fallback to legacy detection
	if detectMultiStepTask(input, intent) {
		return true, nil, nil // Multi-step detected but no specific pattern matched
	}

	return false, nil, nil
}

// GetVerbPairsForPattern returns common verb combinations for a pattern
func GetVerbPairsForPattern(pattern *MultiStepPattern) [][2]string {
	if pattern == nil {
		return nil
	}
	return pattern.VerbPairs
}

// InferDependencies determines step dependencies based on pattern type
func InferDependencies(pattern *MultiStepPattern, steps []TaskStep) []TaskStep {
	if pattern == nil || len(steps) < 2 {
		return steps
	}

	switch pattern.Relation {
	case RelationSequential:
		// Each step depends on the previous
		for i := 1; i < len(steps); i++ {
			steps[i].DependsOn = []int{i - 1}
		}
	case RelationParallel:
		// No dependencies - steps can run concurrently
		for i := range steps {
			steps[i].DependsOn = nil
		}
	case RelationConditional:
		// Step N+1 depends on N succeeding
		for i := 1; i < len(steps); i++ {
			steps[i].DependsOn = []int{i - 1}
		}
	case RelationFallback:
		// Step N+1 runs only if N fails (handled in execution, not dependencies)
		for i := 1; i < len(steps); i++ {
			steps[i].DependsOn = []int{i - 1}
		}
	case RelationIterative:
		// Iterative steps are independent of each other
		for i := range steps {
			steps[i].DependsOn = nil
		}
	}

	return steps
}
