# internal/shards/researcher - Deep Research ShardAgent

This package implements the ResearcherShard, a Type B Persistent Specialist that performs deep web research to build knowledge bases for specialist agents.

## Architecture

The ResearcherShard follows the Cortex 1.5.0 §9.0 Dynamic Shard Configuration:
- Performs LLM-optimized documentation fetching (Context7)
- GitHub repository documentation scraping
- Web search and content extraction
- Codebase analysis for project initialization
- Knowledge atom generation and persistence

## File Index

| File | Description |
|------|-------------|
| `researcher.go` | Core ResearcherShard struct with Execute() orchestrating multi-strategy research, batch processing, and lifecycle management. Exports ResearcherShard, ResearchConfig, ResearchResult, KnowledgeAtom types and dependency injection methods (SetLLMClient, SetKernel, SetLocalDB). |
| `scraper.go` | Web scraping with HTTP fetching, HTML parsing, and domain filtering for quality control. Exports KnowledgeSource type and knownSources map containing pre-defined documentation locations for 20+ common libraries (Rod, Mangle, Gin, etc.). |
| `extract.go` | Knowledge extraction via GitHub docs parsing, LLM synthesis, and Context7 integration. Exports conductWebResearch() for multi-strategy orchestration, parseReadmeContent(), enrichAtomWithLLM(), persistKnowledge(), and generateFacts() for kernel propagation. |
| `analyze.go` | Codebase analysis with dependency scanning and project type detection for initialization. Exports analyzeCodebase(), detectProjectType(), analyzeDependencies(), and language-specific parsers (parseGoMod, parsePackageJSON, parseRequirements, parseCargoToml). |
| `tools.go` | Research toolkit bundling all research tool implementations into a unified interface. Exports ResearchToolkit struct with BrowserResearchTool, WebSearchTool, GitHubResearchTool, Context7Tool, and ResearchCache for result caching. |
| `quality.go` | Quality metrics calculation for research results using weighted scoring. Exports QualityMetrics struct, CalculateQualityMetrics() with 4-factor scoring (atom count 30%, source diversity 25%, code snippets 25%, topic coverage 20%), and qualityRating() for grade assignment. |
| `retry.go` | Exponential backoff retry logic with graceful degradation for unreliable APIs. Exports RetryConfig, WithRetry() helper, FallbackStrategy enum, and GracefulResearchResult supporting 4-tier fallback (full → fewer topics → simpler names → minimal). |
| `local_ingest.go` | Local documentation ingestion for project-specific knowledge bases. Exports IngestLocalDocs() which walks directories, chunks markdown documents, and persists to LocalStore with embeddings. |
| `concept_coverage.go` | Concept coverage analysis to avoid redundant Context7 API queries. Exports ConceptCoverage struct, ExistingKnowledge wrapper, and AnalyzeTopicCoverage() returning skip decisions based on existing atom quality. |
| `prompt_atoms.go` | Prompt atom generation from research results for JIT prompt compilation. Exports PromptAtomData struct for converting KnowledgeAtoms to prompt corpus format with category and selector metadata. |
| `analyze_test.go` | Unit tests for workspace path extraction and codebase analysis functions. Tests extractWorkspacePath() with various path formats and analyzeCodebase() edge cases. |
| `concept_coverage_test.go` | Unit tests for topic coverage analysis and API skip decisions. Tests AnalyzeTopicCoverage() with empty, sufficient, and insufficient coverage scenarios. |
| `researcher_ingest_test.go` | Integration tests for documentation ingestion with complex folder structures. Tests IngestLocalDocs() handling of messy directory layouts with heuristic matching. |
| `researcher_parse_test.go` | Unit tests for task parsing with topics, keywords, and URLs. Tests parseTask() multi-word topic extraction, URL stripping, and keyword fallback behavior. |

## Key Types

### ResearcherShard
The main researcher struct with configuration and components.

```go
type ResearcherShard struct {
    mu             sync.RWMutex
    id             string
    config         core.ShardConfig
    state          core.ShardState
    researchConfig ResearchConfig
    httpClient     *http.Client
    kernel         *core.RealKernel
    scanner        *world.Scanner
    llmClient      perception.LLMClient
    localDB        *store.LocalStore
    toolkit        *ResearchToolkit
    llmSemaphore   chan struct{}
    startTime      time.Time
    stopCh         chan struct{}
    visitedURLs    map[string]bool
}
```

### KnowledgeAtom
Represents a piece of extracted knowledge.

```go
type KnowledgeAtom struct {
    SourceURL   string
    Title       string
    Content     string
    Concept     string
    CodePattern string
    AntiPattern string
    Confidence  float64
    Metadata    map[string]interface{}
    ExtractedAt time.Time
}
```

### ResearchResult
The output of a research task.

```go
type ResearchResult struct {
    Query          string
    Keywords       []string
    Atoms          []KnowledgeAtom
    Summary        string
    PagesScraped   int
    Duration       time.Duration
    FactsGenerated int
}
```

### ResearchConfig
Configuration for research behavior.

```go
type ResearchConfig struct {
    MaxPages        int
    MaxDepth        int
    Timeout         time.Duration
    ConcurrentFetch int
    AllowedDomains  []string
    BlockedDomains  []string
    UserAgent       string
}
```

## Module Responsibilities

### researcher.go (Core Orchestration)
- `NewResearcherShard()` - Constructor with default config
- `Execute()` - Main task execution method
- `ResearchTopicsParallel()` - Batch research for multiple topics
- `DeepResearch()` - Comprehensive research with all tools
- `GetID()`, `GetState()`, `GetConfig()`, `Stop()` - Lifecycle methods
- `parseTask()`, `isCodebaseTask()` - Task parsing
- `buildSummary()` - Result formatting

### scraper.go (Web Fetching & HTML Parsing)
- `findKnowledgeSource()` - Lookup known documentation sources
- `fetchFromKnownSource()` - Fetch from known sources (GitHub, pkg.go.dev)
- `fetchRawContent()` - HTTP GET with error handling
- `fetchAndExtract()` - Fetch URL and extract atoms
- `isDomainAllowed()` - Domain whitelist/blacklist checking
- `extractAtomsFromHTML()` - Parse HTML and extract knowledge
- `extractTitle()`, `extractTextContent()` - HTML parsing helpers
- `containsKeywords()`, `calculateConfidence()` - Keyword matching
- `truncate()` - String truncation utility
- Known sources map for common libraries (Rod, Mangle, Gin, etc.)

### extract.go (Content Extraction & Knowledge Creation)
- `conductWebResearch()` - Multi-strategy research orchestration
- `fetchGitHubDocs()` - GitHub README and docs fetching
- `parseLlmsTxt()` - Parse llms.txt files (Context7 standard)
- `enrichAtomWithLLM()` - LLM-based atom enrichment
- `calculateC7Score()` - Context7-style quality scoring
- `parseReadmeContent()` - Extract atoms from README markdown
- `fetchPkgGoDev()` - pkg.go.dev documentation fetching
- `synthesizeKnowledgeFromLLM()` - LLM knowledge synthesis
- `parseLLMKnowledgeResponse()` - Parse LLM JSON responses
- `generateResearchSummary()` - Create research summaries
- `persistKnowledge()` - Save atoms to database
- `generateFacts()` - Convert results to Mangle facts

### analyze.go (Codebase Analysis)
- `analyzeCodebase()` - Main codebase analysis entry point
- `detectProjectType()` - Detect language, framework, architecture
- `analyzeDependencies()` - Extract project dependencies
- `parseGoMod()`, `parsePackageJSON()`, `parseRequirements()`, `parseCargoToml()` - Dependency parsers
- `detectArchitecturalPatterns()` - Identify code patterns
- `findImportantFiles()` - Locate key project files
- `summarizeFile()` - Create file summaries
- `generateCodebaseSummary()` - LLM-based project summary

### tools.go (Research Toolkit)
- `ResearchToolkit` - Bundles all research tools
- `BrowserResearchTool` - Browser automation for dynamic content
- `WebSearchTool` - Web search via APIs
- `GitHubResearchTool` - GitHub API integration
- `Context7Tool` - Context7 API for LLM-optimized docs
- `ResearchCache` - Caching layer for research results

## Research Strategies

The researcher uses a multi-strategy approach in `conductWebResearch()`:

1. **Primary Strategy: Context7** - LLM-optimized documentation (preferred)
2. **Fallback Strategy 1: Known Sources** - GitHub repos with known structure
3. **Fallback Strategy 2: Web Search** - General web search for unknown topics
4. **Fallback Strategy 3: LLM Synthesis** - Generate knowledge from LLM
5. **Extended Strategy: Deep Research** - Additional URL scraping for deep mode

Strategies run in parallel using goroutines for efficiency.

## Usage Examples

### Basic Research
```go
researcher := researcher.NewResearcherShard()
researcher.SetLLMClient(llmClient)
result, err := researcher.Execute(ctx, "rod browser automation")
```

### Batch Research
```go
topics := []string{"golang concurrency", "testing patterns", "error handling"}
result, err := researcher.ResearchTopicsParallel(ctx, topics)
```

### Codebase Analysis
```go
result, err := researcher.Execute(ctx, "init analyze codebase")
```

### With Context7 API
```go
researcher.SetContext7APIKey("your-api-key")
result, err := researcher.DeepResearch(ctx, "kubernetes operators", []string{"k8s", "controller"})
```

## Knowledge Atom Concepts

Knowledge atoms are tagged with concept types for categorization:

- `overview` - High-level descriptions
- `code_example` - Code snippets
- `documentation_section` - Documentation sections
- `best_practice` - Best practices
- `anti_pattern` - Common pitfalls
- `key_concept` - Core concepts
- `pattern` - Design patterns
- `dependency` - Project dependencies
- `project_identity` - Project metadata
- `architecture` - Architectural patterns

## Quality Scoring

The `calculateC7Score()` function implements Context7-style quality scoring:

- Base score: 0.5
- +0.1 for content > 50 chars
- +0.1 for content > 200 chars
- +0.15 for code examples
- +0.05 for quality titles
- +0.05 for GitHub sources
- -0.3 for content < 20 chars
- -0.5 for garbage indicators (captcha, access denied, etc.)

Atoms with scores < 0.5 are discarded.

## Known Sources

The scraper maintains a map of known documentation sources:

- **Go Libraries**: Rod, Mangle, Cobra, Gin, Echo, Fiber, Zap, GORM, SQLite
- **Generic Topics**: React, TypeScript, Kubernetes, Docker, Security, Testing

Each source includes:
- Type (github, pkggodev, llm)
- Repository owner/name
- Package URL
- Documentation URL

## Dependencies

- `codenerd/internal/core` - Kernel, facts, shard config
- `codenerd/internal/perception` - LLM client interface
- `codenerd/internal/store` - Local knowledge database
- `codenerd/internal/world` - Scanner for codebase analysis
- `codenerd/internal/browser` - Browser automation (tools.go)
- `golang.org/x/net/html` - HTML parsing

## Testing

```bash
go test ./internal/shards/researcher/...
```

Test the toolkit:
```bash
go run ./cmd/test-research/main.go
```

## Configuration

Default research configuration:
- MaxPages: 20
- MaxDepth: 2
- Timeout: 90 seconds
- ConcurrentFetch: 2
- UserAgent: "codeNERD/1.5.0 (Deep Research Agent)"
- Blocked domains: Social media sites

## Performance

The researcher uses:
- Parallel goroutines for multi-topic research
- LLM rate limiting (1 call at a time via semaphore)
- HTTP client connection pooling
- Research result caching (tools.go)
- Batch processing (size: 2) to avoid API overload

## Future Enhancements

- Support for more documentation formats (OpenAPI, GraphQL introspection)
- Incremental knowledge updates (detect staleness)
- Multi-language documentation (i18n)
- Custom scraping rules per domain
- Better code example extraction (Tree-sitter parsing)
