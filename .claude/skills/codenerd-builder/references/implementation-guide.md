# codeNERD Go Implementation Guide

Comprehensive implementation patterns for the codeNERD Logic-First agent framework.

## Project Structure

```
codeNERD/
├── cmd/nerd/              # CLI entry points
│   └── chat.go            # Main chat loop
├── internal/
│   ├── core/              # Kernel & orchestration
│   │   ├── kernel.go      # Mangle kernel wrapper
│   │   ├── virtual_store.go   # FFI router
│   │   ├── shard_manager.go   # ShardAgent lifecycle
│   │   ├── tdd_loop.go    # TDD state machine
│   │   └── llm_client.go  # LLM API abstraction
│   ├── perception/        # Input transduction
│   │   ├── transducer.go  # NL -> Mangle atoms
│   │   └── client.go      # LLM client for perception
│   ├── articulation/      # Output transduction
│   │   └── emitter.go     # Mangle atoms -> NL + control
│   ├── world/             # EDB projections
│   │   ├── fs.go          # Filesystem facts
│   │   └── ast.go         # AST/symbol facts
│   ├── mangle/            # Logic engine
│   │   ├── engine.go      # Mangle wrapper
│   │   ├── schemas.gl     # Core declarations
│   │   ├── policy.gl      # Constitutional rules
│   │   ├── coder.gl       # Coder shard logic
│   │   ├── tester.gl      # Tester shard logic
│   │   └── reviewer.gl    # Reviewer shard logic
│   ├── shards/            # Specialized agents
│   │   ├── coder.go       # Coder shard implementation
│   │   ├── tester.go      # Tester shard implementation
│   │   ├── reviewer.go    # Reviewer shard implementation
│   │   └── researcher.go  # Researcher shard implementation
│   ├── tactile/           # Tool execution
│   │   └── executor.go    # Shell/file operations
│   ├── browser/           # Browser peripheral
│   │   ├── session_manager.go
│   │   └── honeypot.go
│   ├── store/             # Persistence
│   │   ├── local.go       # SQLite storage
│   │   └── learning.go    # Autopoiesis storage
│   ├── campaign/          # Multi-step orchestration
│   │   ├── orchestrator.go
│   │   ├── decomposer.go
│   │   ├── context_pager.go
│   │   ├── checkpoint.go
│   │   └── replan.go
│   ├── config/
│   │   └── config.go
│   └── init/
│       └── initializer.go
└── Docs/research/         # Architectural docs
```

## 1. Kernel Implementation

### 1.1 Core Kernel Structure

```go
// internal/core/kernel.go

package core

import (
    "fmt"
    "strings"
    "sync"

    "github.com/google/mangle/analysis"
    "github.com/google/mangle/ast"
    _ "github.com/google/mangle/builtin" // Critical: Register standard functions (fn:plus, etc.)
    "github.com/google/mangle/engine"
    "github.com/google/mangle/factstore"
    "github.com/google/mangle/parse"
)

// RealKernel wraps the google/mangle engine with proper EDB/IDB separation.
type RealKernel struct {
    mu          sync.RWMutex
    facts       []Fact
    store       factstore.FactStore
    programInfo *analysis.ProgramInfo
    schemas     string
    policy      string
    initialized bool
}

func NewRealKernel() *RealKernel {
    return &RealKernel{
        facts: make([]Fact, 0),
        store: factstore.NewSimpleInMemoryStore(),
    }
}

// evaluate populates the store with facts and evaluates to fixpoint.
// Uses cached programInfo for efficiency.
func (k *RealKernel) evaluate() error {
    // ... (rebuildProgram logic if dirty) ...

    // Create fresh store and populate with EDB facts
    k.store = factstore.NewSimpleInMemoryStore()
    for _, f := range k.facts {
        atom, err := f.ToAtom()
        if err != nil {
            return err
        }
        k.store.Add(atom)
    }

    // DECIDE: Run Mangle to fixpoint
    // Use EvalProgramWithStats which updates the store in-place with derived facts (IDB).
    // This fixes the "Split Brain" issue where decisions were lost.
    _, err := engine.EvalProgramWithStats(k.programInfo, k.store)
    if err != nil {
        return fmt.Errorf("failed to evaluate program: %w", err)
    }

    k.initialized = true
    return nil
}
```

### 1.2 Virtual Store (FFI Router)

```go
// internal/core/virtual_store.go

package core

import (
    "github.com/google/mangle/ast"
    "github.com/google/mangle/factstore"
)

type VirtualStore struct {
    MemStore    factstore.FactStore
    MCPClient   *MCPClient
    Executor    *tactile.Executor
    FSProjector *world.FilesystemProjector
}

func NewVirtualStore() *VirtualStore {
    return &VirtualStore{
        MemStore: factstore.NewSimpleInMemoryStore(),
        Executor: tactile.NewExecutor(),
    }
}

// GetFacts routes predicate queries to appropriate handlers
func (v *VirtualStore) GetFacts(pred ast.PredicateSym) []ast.Atom {
    switch pred.Symbol {

    // Virtual predicates - computed on demand
    case "file_content":
        return v.fetchFileContent(pred)
    case "shell_exec_result":
        return v.executeShell(pred)
    case "mcp_tool_result":
        return v.callMCPTool(pred)
    case "structural_match":
        return v.runStructuralSearch(pred)

    // Standard predicates - from memory
    default:
        return v.MemStore.GetFacts(pred)
    }
}

// Add delegates to memory store
func (v *VirtualStore) Add(atom ast.Atom) {
    v.MemStore.Add(atom)
}

// Permission check before execution
func (v *VirtualStore) executeShell(pred ast.PredicateSym) []ast.Atom {
    // Extract request from predicate
    req := v.parseShellRequest(pred)

    // Check permission via Mangle
    permitted := v.MemStore.GetFacts(ast.PredicateSym{
        Symbol: "permitted",
        Arity:  1,
    })

    if !v.isPermitted(req, permitted) {
        return []ast.Atom{v.createErrorAtom("AccessDenied")}
    }

    result := v.Executor.Execute(req)
    return []ast.Atom{v.createResultAtom(result)}
}
```

### 1.3 Shard Manager

```go
// internal/core/shard_manager.go

package core

type ShardManager struct {
    ActiveShards map[string]*ShardAgent
    Profiles     map[string]*ShardProfile
}

type ShardAgent struct {
    ID           string
    Type         ShardType
    Kernel       *Kernel
    ContextBudget int
    Permissions  []string
}

type ShardType int

const (
    Generalist ShardType = iota  // Type A: Ephemeral
    Specialist                    // Type B: Persistent
)

type ShardProfile struct {
    Name             string
    Description      string
    ResearchKeywords []string
    AllowedTools     []string
    KnowledgeDB      string  // Path to SQLite
}

// SpawnShard creates a new ephemeral shard
func (m *ShardManager) SpawnShard(shardType ShardType, task string) (*ShardAgent, error) {
    cfg := &KernelConfig{
        MaxRetries:    3,
        ContextBudget: 8000,
    }

    kernel, err := NewKernel(cfg)
    if err != nil {
        return nil, err
    }

    shard := &ShardAgent{
        ID:     generateShardID(),
        Type:   shardType,
        Kernel: kernel,
    }

    // Load shard-specific logic
    switch shardType {
    case Generalist:
        // Minimal logic, RAM only
    case Specialist:
        profile := m.Profiles[task]
        if profile != nil {
            err = shard.MountKnowledgeBase(profile.KnowledgeDB)
        }
    }

    m.ActiveShards[shard.ID] = shard
    return shard, nil
}

// ExecuteTask runs task in isolated shard
func (m *ShardManager) ExecuteTask(shardType ShardType, task string) (string, error) {
    shard, err := m.SpawnShard(shardType, task)
    if err != nil {
        return "", err
    }
    defer m.DestroyShard(shard.ID)

    // Inject task as user_intent
    response, err := shard.Kernel.Run(task)
    if err != nil {
        return "", err
    }

    return response.Summary, nil
}

func (m *ShardManager) DestroyShard(id string) {
    delete(m.ActiveShards, id)
}
```

## 2. Perception Transducer

### 2.1 Input Transduction

```go
// internal/perception/transducer.go

package perception

import (
    "encoding/json"
)

type Transducer struct {
    Client *LLMClient
    Schema *PerceptionSchema
}

type PerceptionSchema struct {
    IntentVerbs    []string  // Controlled vocabulary
    Categories     []string  // query, mutation, instruction
}

type PerceptionResult struct {
    Intent          IntentAtom          `json:"intent"`
    FocusResolution []FocusAtom         `json:"focus_resolution"`
    AmbiguityFlags  []AmbiguityAtom     `json:"ambiguity_flags,omitempty"`
}

type IntentAtom struct {
    Category   string `json:"category"`
    Verb       string `json:"verb"`
    Target     string `json:"target"`
    Constraint string `json:"constraint,omitempty"`
}

type FocusAtom struct {
    RawReference string  `json:"raw_reference"`
    ResolvedPath string  `json:"resolved_path"`
    SymbolName   string  `json:"symbol_name,omitempty"`
    Confidence   float64 `json:"confidence"`
}

// Perceive converts natural language to Mangle atoms
func (t *Transducer) Perceive(input string, context []Atom) (*PerceptionResult, error) {
    prompt := t.buildPerceptionPrompt(input, context)

    // Use structured output for guaranteed schema
    response, err := t.Client.Complete(prompt, &StructuredOutputConfig{
        Schema: t.Schema.ToJSONSchema(),
    })
    if err != nil {
        return nil, err
    }

    var result PerceptionResult
    if err := json.Unmarshal([]byte(response), &result); err != nil {
        // Trigger repair loop
        return t.repairAndRetry(response, err)
    }

    // Validate against controlled vocabulary
    if !t.Schema.ValidateVerb(result.Intent.Verb) {
        return t.repairAndRetry(response, ErrInvalidVerb)
    }

    return &result, nil
}

func (t *Transducer) buildPerceptionPrompt(input string, context []Atom) string {
    return fmt.Sprintf(`You are a Perception Transducer. Convert the user input into structured Mangle atoms.

CONTEXT (Current State):
%s

USER INPUT:
%s

OUTPUT REQUIREMENTS:
1. intent: Classify using ONLY these verbs: %v
2. focus_resolution: Map fuzzy references to concrete paths with confidence scores
3. ambiguity_flags: Flag any missing required parameters

Return ONLY valid JSON matching the schema.`,
        formatContext(context),
        input,
        t.Schema.IntentVerbs,
    )
}
```

## 3. Articulation Transducer

### 3.1 Output Transduction (Piggyback Protocol)

```go
// internal/articulation/emitter.go

package articulation

type Emitter struct {
    Client *LLMClient
}

type DualPayload struct {
    SurfaceResponse string        `json:"surface_response"`
    ControlPacket   ControlPacket `json:"control_packet"`
}

type ControlPacket struct {
    MangleUpdates       []string          `json:"mangle_updates"`
    MemoryOperations    []MemoryOp        `json:"memory_operations"`
    AbductiveHypothesis string            `json:"abductive_hypothesis,omitempty"`
}

type MemoryOp struct {
    Operation string `json:"op"`     // promote_to_long_term, forget, archive
    Fact      string `json:"fact"`
}

// Articulate converts execution results to dual payload
func (e *Emitter) Articulate(results []ExecutionResult, context []Atom) (*DualPayload, error) {
    prompt := e.buildArticulationPrompt(results, context)

    response, err := e.Client.Complete(prompt, &StructuredOutputConfig{
        Schema: DualPayloadSchema,
    })
    if err != nil {
        return nil, err
    }

    var payload DualPayload
    if err := json.Unmarshal([]byte(response), &payload); err != nil {
        return e.repairAndRetry(response, err)
    }

    // Validate Mangle syntax
    for _, update := range payload.ControlPacket.MangleUpdates {
        if err := validateMangleSyntax(update); err != nil {
            return e.repairMangleUpdate(update, err)
        }
    }

    return &payload, nil
}

func (e *Emitter) buildArticulationPrompt(results []ExecutionResult, context []Atom) string {
    return fmt.Sprintf(`You are an Articulation Transducer. Generate a dual-payload response.

EXECUTION RESULTS:
%s

CURRENT STATE:
%s

OUTPUT REQUIREMENTS:
1. surface_response: Human-readable status for the user
2. control_packet.mangle_updates: Valid Mangle atoms representing state changes
3. control_packet.memory_operations: Facts to promote/forget

The control_packet is HIDDEN from the user. Use it to maintain kernel state.`,
        formatResults(results),
        formatContext(context),
    )
}
```

## 4. World Model Projection

### 4.1 Filesystem Facts

```go
// internal/world/fs.go

package world

import (
    "crypto/sha256"
    "os"
    "path/filepath"
)

type FilesystemProjector struct {
    RootPath string
    Store    FactStore
}

type FileTopology struct {
    Path         string
    Hash         string
    Language     string
    LastModified int64
    IsTestFile   bool
    Size         int64
}

// ProjectFilesystem scans directory and creates file_topology atoms
func (p *FilesystemProjector) ProjectFilesystem() error {
    return filepath.Walk(p.RootPath, func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return err
        }

        topology := &FileTopology{
            Path:         path,
            Hash:         p.computeHash(path),
            Language:     detectLanguage(path),
            LastModified: info.ModTime().Unix(),
            IsTestFile:   isTestFile(path),
            Size:         info.Size(),
        }

        atom := p.toAtom(topology)
        p.Store.Add(atom)
        return nil
    })
}

func detectLanguage(path string) string {
    ext := filepath.Ext(path)
    switch ext {
    case ".go":
        return "/go"
    case ".py":
        return "/python"
    case ".ts", ".tsx":
        return "/typescript"
    case ".rs":
        return "/rust"
    default:
        return "/unknown"
    }
}

func isTestFile(path string) bool {
    return strings.Contains(path, "_test.") ||
           strings.Contains(path, ".test.") ||
           strings.Contains(path, "/test/")
}
```

### 4.2 AST Projection

```go
// internal/world/ast.go

package world

import (
    "go/ast"
    "go/parser"
    "go/token"
)

type ASTProjector struct {
    Store FactStore
}

type SymbolInfo struct {
    SymbolID   string
    Type       string  // /function, /struct, /interface
    Visibility string  // /public, /private
    DefinedAt  string
    Signature  string
}

// ProjectAST parses Go file and creates symbol_graph atoms
func (p *ASTProjector) ProjectGoFile(path string) error {
    fset := token.NewFileSet()
    node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
    if err != nil {
        return err
    }

    ast.Inspect(node, func(n ast.Node) bool {
        switch x := n.(type) {
        case *ast.FuncDecl:
            symbol := &SymbolInfo{
                SymbolID:   fmt.Sprintf("func:%s:%s", node.Name.Name, x.Name.Name),
                Type:       "/function",
                Visibility: getVisibility(x.Name.Name),
                DefinedAt:  fmt.Sprintf("%s:%d", path, fset.Position(x.Pos()).Line),
                Signature:  formatFuncSignature(x),
            }
            p.Store.Add(p.toSymbolAtom(symbol))

            // Extract dependency links
            p.extractDependencies(x, symbol.SymbolID)

        case *ast.TypeSpec:
            // Handle structs, interfaces
        }
        return true
    })

    return nil
}

func (p *ASTProjector) extractDependencies(fn *ast.FuncDecl, callerID string) {
    ast.Inspect(fn.Body, func(n ast.Node) bool {
        if call, ok := n.(*ast.CallExpr); ok {
            calleeID := extractCalleeID(call)
            if calleeID != "" {
                atom := createDependencyAtom(callerID, calleeID)
                p.Store.Add(atom)
            }
        }
        return true
    })
}
```

## 5. TDD Loop Implementation

```go
// internal/core/tdd_loop.go

package core

type TDDLoop struct {
    Kernel     *Kernel
    MaxRetries int
    State      TDDState
}

type TDDState int

const (
    StateUnknown TDDState = iota
    StatePassing
    StateFailing
    StateLogRead
    StateCauseFound
    StatePatchApplied
)

// Run executes the TDD repair loop
func (t *TDDLoop) Run(target string) error {
    retryCount := 0

    for {
        // Run tests
        result := t.runTests(target)
        t.updateTestState(result)

        if result.Passed {
            t.State = StatePassing
            return nil
        }

        t.State = StateFailing
        retryCount++

        if retryCount >= t.MaxRetries {
            return t.escalateToUser()
        }

        // Read and analyze error log
        t.State = StateLogRead
        errors := t.parseErrors(result.Output)

        // Abductive reasoning for root cause
        cause := t.analyzeRootCause(errors)
        t.State = StateCauseFound

        // Generate and apply patch
        patch := t.generatePatch(cause)
        if err := t.applyPatch(patch); err != nil {
            continue
        }
        t.State = StatePatchApplied
    }
}

func (t *TDDLoop) updateTestState(result *TestResult) {
    // Update Mangle facts
    t.Kernel.Store.Add(createAtom("test_state", stateToAtom(t.State)))
    t.Kernel.Store.Add(createAtom("retry_count", retryCount))

    // Add diagnostics
    for _, err := range result.Errors {
        t.Kernel.Store.Add(createDiagnosticAtom(err))
    }
}

func (t *TDDLoop) escalateToUser() error {
    t.Kernel.Store.Add(createAtom("next_action", "/escalate_to_user"))
    return ErrMaxRetriesExceeded
}
```

## 6. Spreading Activation

```go
// internal/core/activation.go

package core

type ActivationEngine struct {
    Store         FactStore
    DecayFactor   float64
    Threshold     float64
    MaxIterations int
}

func NewActivationEngine(store FactStore) *ActivationEngine {
    return &ActivationEngine{
        Store:         store,
        DecayFactor:   0.85,
        Threshold:     30.0,
        MaxIterations: 5,
    }
}

// RunSpreadingActivation flows energy through fact graph
func (a *ActivationEngine) RunSpreadingActivation(seeds []string) []Atom {
    // Initialize activation scores
    scores := make(map[string]float64)
    for _, seed := range seeds {
        scores[seed] = 100.0
    }

    // Iterate spreading
    for i := 0; i < a.MaxIterations; i++ {
        newScores := make(map[string]float64)

        for fact, score := range scores {
            // Get neighbors via dependency_link
            neighbors := a.getNeighbors(fact)
            for _, neighbor := range neighbors {
                propagated := score * a.DecayFactor
                if propagated > newScores[neighbor] {
                    newScores[neighbor] = propagated
                }
            }

            // Keep original scores
            if score > newScores[fact] {
                newScores[fact] = score
            }
        }

        scores = newScores
    }

    // Select facts above threshold
    var result []Atom
    for fact, score := range scores {
        if score > a.Threshold {
            result = append(result, a.getFactAtom(fact))
        }
    }

    return result
}

func (a *ActivationEngine) getNeighbors(fact string) []string {
    deps := a.Store.GetFacts(ast.PredicateSym{Symbol: "dependency_link"})
    var neighbors []string
    for _, dep := range deps {
        if dep.Args[0].String() == fact {
            neighbors = append(neighbors, dep.Args[1].String())
        }
    }
    return neighbors
}
```

## 7. Safety Checks

```go
// internal/core/safety.go

package core

type SafetyChecker struct {
    Store FactStore
}

// CheckPermission verifies action is permitted
func (s *SafetyChecker) CheckPermission(action string) bool {
    // Query Mangle for permitted(action)
    permitted := s.Store.GetFacts(ast.PredicateSym{
        Symbol: "permitted",
        Arity:  1,
    })

    for _, p := range permitted {
        if p.Args[0].String() == action {
            return true
        }
    }

    return false
}

// CheckDangerous verifies if action requires admin override
func (s *SafetyChecker) CheckDangerous(action string) bool {
    dangerous := s.Store.GetFacts(ast.PredicateSym{
        Symbol: "dangerous_action",
        Arity:  1,
    })

    for _, d := range dangerous {
        if d.Args[0].String() == action {
            return true
        }
    }

    return false
}

// ValidateBeforeExec is the hallucination firewall
func (s *SafetyChecker) ValidateBeforeExec(action Action) error {
    if !s.CheckPermission(action.Name) {
        // Check if dangerous action needs override
        if s.CheckDangerous(action.Name) {
            return ErrRequiresApproval
        }
        return ErrAccessDenied
    }

    // Additional checks
    if action.Type == ShellExec {
        if err := s.validateShellCommand(action.Command); err != nil {
            return err
        }
    }

    return nil
}

func (s *SafetyChecker) validateShellCommand(cmd string) error {
    // Check binary allowlist
    binary := extractBinary(cmd)
    allowlist := s.Store.GetFacts(ast.PredicateSym{
        Symbol: "binary_allowlist",
        Arity:  1,
    })

    for _, allowed := range allowlist {
        if allowed.Args[0].String() == binary {
            return nil
        }
    }

    return ErrBinaryNotAllowed
}
```

## 8. MCP Integration

```go
// internal/core/mcp_client.go

package core

type MCPClient struct {
    Servers map[string]*MCPServer
}

type MCPServer struct {
    Name    string
    Command string
    Tools   []MCPTool
}

// CallTool invokes MCP tool and returns Mangle atoms
func (m *MCPClient) CallTool(serverName, toolName string, args map[string]interface{}) ([]Atom, error) {
    server, ok := m.Servers[serverName]
    if !ok {
        return nil, ErrServerNotFound
    }

    // Find tool
    var tool *MCPTool
    for _, t := range server.Tools {
        if t.Name == toolName {
            tool = &t
            break
        }
    }
    if tool == nil {
        return nil, ErrToolNotFound
    }

    // Execute via MCP protocol
    result, err := m.executeToolCall(server, tool, args)
    if err != nil {
        return nil, err
    }

    // Convert result to Mangle atoms
    return m.resultToAtoms(toolName, result), nil
}

// AutoDiscoverTools queries server's list_tools
func (m *MCPClient) AutoDiscoverTools(serverName string) error {
    server := m.Servers[serverName]
    tools, err := m.listTools(server)
    if err != nil {
        return err
    }

    server.Tools = tools

    // Generate Mangle declarations for each tool
    for _, tool := range tools {
        m.generateMangleDecl(tool)
    }

    return nil
}
```

## 9. Campaign Orchestration

For multi-step coding tasks:

```go
// internal/campaign/orchestrator.go

package campaign

type Orchestrator struct {
    Kernel       *Kernel
    ShardManager *ShardManager
    Checkpointer *Checkpointer
}

type Campaign struct {
    ID          string
    Goal        string
    Steps       []Step
    CurrentStep int
    Status      CampaignStatus
}

// Run executes multi-step campaign with checkpointing
func (o *Orchestrator) Run(campaign *Campaign) error {
    for campaign.CurrentStep < len(campaign.Steps) {
        step := campaign.Steps[campaign.CurrentStep]

        // Checkpoint before step
        o.Checkpointer.Save(campaign)

        // Delegate to appropriate shard
        result, err := o.executeStep(step)
        if err != nil {
            // Attempt replan
            if err := o.replan(campaign, err); err != nil {
                return err
            }
            continue
        }

        // Update campaign state
        campaign.CurrentStep++
        o.updateProgress(campaign, result)
    }

    campaign.Status = StatusComplete
    return nil
}

func (o *Orchestrator) executeStep(step Step) (*StepResult, error) {
    switch step.Type {
    case StepCode:
        return o.ShardManager.ExecuteTask(Generalist, step.Task)
    case StepTest:
        return o.ShardManager.ExecuteTask(Generalist, step.Task)
    case StepReview:
        return o.ShardManager.ExecuteTask(Specialist, step.Task)
    default:
        return nil, ErrUnknownStepType
    }
}
```

## Key Integration Points

1. **Kernel <- Perception**: Atoms flow into FactStore
2. **Kernel -> VirtualStore**: next_action atoms trigger FFI
3. **VirtualStore -> Executors**: Shell, MCP, File operations
4. **Kernel -> ShardManager**: delegate_task spawns sub-kernels
5. **Kernel -> Articulation**: State atoms become dual payload
6. **Articulation -> User**: Surface response displayed
7. **Articulation -> Kernel**: Control packet updates state
