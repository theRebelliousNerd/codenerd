# internal/init - Workspace Initialization

This package handles initialization of codeNERD workspaces, including codebase scanning, profile generation, and agent setup.

## Architecture

The initializer performs comprehensive workspace setup:
1. Scan codebase structure
2. Detect project type and framework
3. Generate Mangle profile facts
4. Create system agents (Type 1)
5. Initialize knowledge stores

## File Index

| File | Description |
|------|-------------|
| `initializer.go` | Main initialization orchestration with 10-phase Initialize() method and progress reporting. Exports Initializer struct, InitConfig, InitProgress, InitResult, RecommendedAgent types. Includes Gemini grounding integration - when Gemini is the LLM provider, Google Search and URL Context grounding are automatically enabled for research-heavy phases. |
| `strategic_knowledge.go` | Deep strategic knowledge generation using LLM analysis with Gemini grounding support. Exports StrategicKnowledge struct, generateStrategicKnowledge(), GatherProjectDocumentation() for doc discovery, filterDocumentsByRelevance() for LLM-based filtering, and ProcessDocumentsWithTracking() for incremental knowledge extraction. Uses grounded completions when Gemini is configured. |
| `scanner.go` | File system traversal with dependency detection and directory structure creation. Exports createDirectoryStructure(), detectLanguageFromFiles() checking 12+ config files, detectDependencies() parsing go.mod/package.json with version extraction. |
| `profile.go` | Profile generation with facts file creation and session state management. Exports buildProjectProfile(), saveProfile(), generateFactsFile(), initPreferences(), initSessionState(), and createCodebaseKnowledgeBase() for project-specific atoms. |
| `agents.go` | Agent recommendation and creation with knowledge base hydration and registration. Exports determineRequiredAgents() analyzing dependencies, createType3Agents() with research, generateAgentPromptsYAML() for JIT atoms, and registerAgentsWithShardManager(). |
| `interactive.go` | Interactive mode for agent curation during initialization with user prompts. Exports DetectedAgent, AgentSelectionPreferences, InteractiveConfig, and runInteractiveAgentSelection() for CLI-based agent approval workflow. |
| `jit_integration.go` | JIT prompt compilation for init phases with fallback to simple prompts. Exports assembleJITPrompt() creating phase-specific CompilationContext, createJITCompiler() with hardcoded init atoms, and BuildInitCompilationContext(). |
| `shared_kb.go` | Shared knowledge pool for common concepts inherited by all specialist agents. Exports SharedKnowledgeTopics (error handling, logging, testing), BaseSharedAtoms with hardcoded best practices, and createSharedKnowledgeBase(). |
| `tools.go` | Tool definitions for language-specific build/test/lint commands with shard affinity. Exports ToolDefinition struct with command/category/conditions, GetLanguageTools() for Go/Python/TS/Rust, and saveToolRegistry() for persistence. |
| `typeu_agents.go` | Type U user-defined agent support for --define-agent CLI flag. Exports TypeUAgentDefinition parsed from "Name:role:topic1,topic2" format, ParseTypeUAgentFlag() with validation, and integration with agent creation pipeline. |
| `validation.go` | Post-initialization validation of agent knowledge databases for schema and content. Exports ValidationResult, ValidationSummary, ValidateAgentDB() checking tables/atoms/hashes, and ValidateAllAgentDBs() for comprehensive verification. |
| `scanner_test.go` | Unit tests for entry point detection across Go, Python, and Node.js project layouts. Tests detectEntryPoints() with main.go patterns, Python __main__ detection, and package.json parsing. |

## Key Types

### Initializer
```go
type Initializer struct {
    config     InitConfig
    researcher *shards.ResearcherShard
    scanner    *world.Scanner
    localDB    *store.LocalStore
    shardMgr   *core.ShardManager
    kernel     *core.RealKernel
}

func (i *Initializer) Initialize(ctx context.Context) (*InitResult, error)
```

### InitResult
```go
type InitResult struct {
    Success           bool
    Profile           ProjectProfile
    Preferences       UserPreferences
    NerdDir           string
    FilesCreated      []string
    FactsGenerated    int
    Duration          time.Duration
    Warnings          []string
    RecommendedAgents []RecommendedAgent
    CreatedAgents     []CreatedAgent
    AgentKBs          map[string]int

    // Gemini Grounding (when Gemini is the LLM provider)
    GroundingSources []string // URLs used to ground LLM responses
    GroundingEnabled bool     // Whether grounding was active
}
```

## Gemini Grounding Integration

When Gemini is configured as the LLM provider, the init system automatically enables grounding features:

### Features Enabled
- **Google Search**: Real-time search grounding for strategic knowledge generation
- **URL Context**: Documentation URLs for the project's tech stack (Go, Python, React, etc.)

### How It Works
```
NewInitializer()
    │
    ▼
Detect Gemini client via types.GroundingController
    │
    ▼
Enable Google Search grounding
    │
    ▼
generateStrategicKnowledge()
    ├── Get tech stack doc URLs (research.GetDocURLsForTech)
    ├── Enable URL Context with doc URLs
    ├── CompleteWithGrounding()
    └── Capture grounding sources
    │
    ▼
filterDocumentsByRelevance()
    ├── CompleteWithGrounding()
    └── Capture grounding sources
    │
    ▼
InitResult.GroundingSources populated
```

### Using Grounding in Other Systems

The grounding helper from `internal/tools/research` can be used anywhere:

```go
import "codenerd/internal/tools/research"

// Create helper from any LLM client
helper := research.NewGroundingHelper(llmClient)

// Check if grounding is available (only for Gemini)
if helper.IsGroundingAvailable() {
    helper.EnableGoogleSearch()
    helper.EnableURLContext([]string{"https://docs.example.com"})

    response, sources, err := helper.CompleteWithGrounding(ctx, prompt)
    // sources contains URLs used for grounding
}
```

### CreatedAgent
```go
type CreatedAgent struct {
    Name          string
    Type          string
    KnowledgePath string
    KBSize        int
    CreatedAt     time.Time
    Status        string
}
```

## Initialization Steps (initializer.go)

1. **Validate Workspace**
   - Check directory exists
   - Verify not already initialized (unless --force)

2. **Phase 1: Directory Structure & Database Setup**
   - Create .nerd/ directory structure (scanner.go)
   - Initialize local knowledge database

3. **Phase 2: Deep Codebase Scan**
   - Use world.Scanner for comprehensive file analysis
   - Assert scan results as Mangle facts to kernel

4. **Phase 3: Run Researcher Shard**
   - Deep analysis of codebase via ResearcherShard
   - Generate analysis summary

5. **Phase 4: Build Project Profile** (profile.go)
   - Extract language, framework, architecture
   - Detect dependencies (scanner.go)
   - Save profile.json

6. **Phase 5: Generate Mangle Facts** (profile.go)
   - Create profile.mg with Mangle facts
   - Include project identity, language, framework

7. **Phase 6: Determine Required Type 3 Agents** (agents.go)
   - Analyze project profile
   - Recommend language-specific agents
   - Recommend framework-specific agents
   - Recommend dependency-specific agents

8. **Phase 7: Create Knowledge Bases & Type 3 Agents** (agents.go)
   - Create SQLite knowledge bases for each agent
   - Hydrate with base knowledge atoms
   - Perform deep research for each agent's topics
   - Generate prompts.yaml for each agent (identity, methodology, domain atoms)
   - Register agents with shard manager

9. **Phase 7b: Create Codebase Knowledge Base** (profile.go)
   - Store project identity, language, framework
   - Store file topology summary
   - Store dependencies and patterns

10. **Phase 7c: Create Core Shard Knowledge Bases** (agents.go)
    - Create KBs for Coder, Reviewer, Tester shards
    - Store shard identity and capabilities
    - Research domain-specific topics

11. **Phase 7d: Create Campaign Knowledge Base** (profile.go)
    - Store campaign orchestration concepts
    - Store project context
    - Research workflow patterns

12. **Phase 8: Initialize Preferences** (profile.go)
    - Create default preferences
    - Apply user hints
    - Save preferences.json

13. **Phase 9: Create Session State** (profile.go)
    - Initialize session.json
    - Set up session tracking

14. **Phase 10: Generate Agent Registry** (agents.go)
    - Save agents.json
    - Include all created agent metadata

## Module Responsibilities

### initializer.go (Core Orchestration)
- Main `Initialize()` method that coordinates all phases
- Phase progression and error handling
- Progress updates via channel
- Result aggregation and summary printing
- Type definitions for all structs

### scanner.go (File System Operations)
- `createDirectoryStructure()` - Creates .nerd/ directory tree
- `detectLanguageFromFiles()` - Detects primary language from config files
- `detectDependencies()` - Scans go.mod and package.json for dependencies

### profile.go (Profile & Session Management)
- `buildProjectProfile()` - Constructs ProjectProfile from analysis
- `saveProfile()` / `LoadProjectProfile()` - Profile persistence
- `generateFactsFile()` - Creates Mangle facts file
- `initPreferences()` / `savePreferences()` / `LoadPreferences()` - Preferences
- `initSessionState()` / `LoadSessionState()` / `SaveSessionState()` - Session state
- `SaveSessionHistory()` / `LoadSessionHistory()` / `ListSessionHistories()` - Session history
- `createCodebaseKnowledgeBase()` - Project-specific knowledge atoms
- `createCampaignKnowledgeBase()` - Campaign orchestration knowledge
- Helper functions: `IsInitialized()`, `GetLatestSession()`, `sanitizeForMangle()`, etc.

### agents.go (Agent Management)
- `determineRequiredAgents()` - Analyzes project and recommends agents
- `createType3Agents()` - Creates agent knowledge bases
- `createAgentKnowledgeBase()` - Creates SQLite KB for single agent
- `generateAgentPromptsYAML()` - Generates prompts.yaml for Type B agents
- `generateBaseKnowledgeAtoms()` - Base identity/mission atoms
- `registerAgentsWithShardManager()` - Registers agents for dynamic calling
- `saveAgentRegistry()` - Persists agent registry
- `createCoreShardKnowledgeBases()` - Creates KBs for Coder, Reviewer, Tester
- `sendAgentProgress()` - Progress updates for agent creation

## Generated Facts (profile.go)

```datalog
# Project metadata
project_profile("proj_abc123", "codeNERD", "Logic-first agent").
project_language(/go).
project_framework(/bubbletea).
project_architecture(/neuro_symbolic).

# Build system
build_system(/go_mod).

# Patterns
architectural_pattern(/ioc).
architectural_pattern(/tdd).

# Entry points
entry_point("/cmd/nerd/main.go").
```

## Directory Structure Created (scanner.go)

```
.nerd/
├── profile.mg          # Generated Mangle facts
├── profile.json        # Project metadata
├── session.json        # Session state
├── preferences.json    # User preferences
├── agents.json         # Agent registry
├── knowledge.db        # Main knowledge store
├── prompts/            # Project prompt corpus (JIT)
│   └── corpus.db       # Shared/project-scoped prompt atoms DB
├── .gitignore          # Git ignore rules
├── sessions/           # Session histories
├── shards/             # Shard knowledge DBs
│   ├── codebase_knowledge.db
│   ├── campaign_knowledge.db
│   ├── coder_knowledge.db
│   ├── reviewer_knowledge.db
│   ├── tester_knowledge.db
│   ├── goexpert_knowledge.db
│   ├── rodexpert_knowledge.db
│   └── ...
├── cache/              # Temporary cache
├── tools/              # Autopoiesis generated tools
│   ├── .compiled/      # Compiled tool binaries
│   ├── .learnings/     # Tool execution learnings
│   ├── .profiles/      # Tool quality profiles
│   └── .traces/        # Reasoning traces
├── agents/             # Persistent agent definitions
│   ├── goexpert/       # Example Type B agent
│   │   └── prompts.yaml  # JIT prompt atoms (identity, methodology, domain)
│   ├── rodexpert/
│   │   └── prompts.yaml
│   └── ...
└── campaigns/          # Campaign checkpoints
```

All directories are created by `nerd init`. The `tools/` and `agents/` directories
are NOT gitignored by default so users can commit generated tools and agent prompts.

## Agent Recommendation Logic (agents.go)

The `determineRequiredAgents()` function recommends Type 3 agents based on:

1. **Language-specific**: GoExpert, PythonExpert, TSExpert, RustExpert
2. **Framework-specific**: WebAPIExpert, FrontendExpert
3. **Dependency-specific**:
   - RodExpert (Rod browser automation)
   - BrowserAutomationExpert (Chromedp/Puppeteer/Playwright)
   - MangleExpert (Datalog/Mangle)
   - LLMIntegrationExpert (OpenAI/Anthropic)
   - BubbleTeaExpert, CobraExpert (CLI/TUI frameworks)
   - DatabaseExpert (GORM/SQLx/Prisma/TypeORM)
4. **Always included**: SecurityAuditor, TestArchitect

Each agent includes:
- Topics for deep research
- Permissions (read_file, code_graph, exec_cmd, network, browser)
- Priority score
- Reason for recommendation

## Force Reinit

`nerd init --force` will:
- Preserve learned preferences from `.nerd/preferences.json`
- Regenerate profile.mg with fresh codebase scan
- Keep existing agent definitions

## Dependencies

- `internal/core` - Kernel for fact loading, ShardManager
- `internal/perception` - LLMClient for analysis
- `internal/world` - Scanner for codebase
- `internal/shards` - ResearcherShard for deep research
- `internal/store` - LocalStore for SQLite knowledge bases

## Testing

```bash
go test ./internal/init/...
```

## Notes

- The modularization keeps initializer.go as the orchestrator
- Each module has clear responsibilities
- All functions remain as methods on `Initializer` struct
- Total line count increased slightly due to file headers/imports
- Build should be verified after modularization

---

**Remember: Push to GitHub regularly!**
