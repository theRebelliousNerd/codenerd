---
name: research-builder
description: Build production-ready knowledge research and ingestion systems for codeNERD ShardAgents. Use when implementing ResearcherShard functionality, llms.txt/Context7-style documentation gathering, knowledge atom extraction, 4-tier memory storage, or specialist knowledge hydration. Includes deep research patterns, quality scoring, LLM enrichment, and persistence strategies. (project)
---

# codeNERD Research Systems Builder

## Purpose

Guide the implementation of codeNERD's Deep Research infrastructure, which gathers, processes, and stores knowledge for Persistent Specialist ShardAgents. This skill covers:

- **llms.txt Standard** - AI-optimized documentation discovery and ingestion
- **Context7-Style Patterns** - Multi-stage documentation processing with quality scoring
- **KnowledgeAtom Schema** - Structured knowledge representation
- **4-Tier Memory Architecture** - RAM, Vector, Graph, and Cold Storage persistence
- **Specialist Hydration** - Pre-populating expert agents with domain knowledge

## When to Use This Skill

Deploy this skill when:

- Implementing or extending ResearcherShard functionality
- Adding new knowledge sources to the `knownSources` registry
- Building llms.txt parsing and ingestion pipelines
- Implementing quality scoring algorithms (Context7-style)
- Creating knowledge persistence workflows
- Debugging research task execution
- Adding LLM enrichment for documentation summarization
- Completing the specialist "viva voce" examination system

**Key Implementation Files:**

- [internal/shards/researcher.go](internal/shards/researcher.go) - ResearcherShard (1,821 lines)
- [internal/store/local.go](internal/store/local.go) - LocalStore with 4-tier memory (676 lines)
- [internal/store/learning.go](internal/store/learning.go) - LearningStore for autopoiesis (272 lines)

## Architecture Overview

```text
[ User: "nerd define-agent RustExpert --topic tokio" ]
       |
       v
[ ResearcherShard ]
       |
       +-- [1] Strategy Selection
       |       +-- Check knownSources map
       |       +-- Fallback to LLM synthesis
       |
       +-- [2] Documentation Fetching
       |       +-- Check for llms.txt (AI-optimized pointer)
       |       +-- Fetch README.md + docs/
       |       +-- Parse markdown sections
       |       +-- Extract code examples
       |
       +-- [3] LLM Enrichment
       |       +-- Summarize for AI consumption
       |       +-- Extract metadata
       |
       +-- [4] Quality Scoring (C7-style)
       |       +-- Content length checks
       |       +-- Code example bonuses
       |       +-- Garbage detection
       |
       +-- [5] Persistence
               +-- Shard B: Vector Store (semantic retrieval)
               +-- Shard C: Knowledge Graph (relational links)
               +-- Shard D: Cold Storage (persistent facts)
               |
               v
       [ .nerd/shards/{specialist}_knowledge.db ]
```

## The llms.txt Standard

The [llms.txt specification](https://llmstxt.org/) provides AI-optimized documentation pointers:

### Format

```markdown
# Project Name

> Brief description for LLM context

## Core Documentation

- [Getting Started](docs/getting-started.md): Setup and basic usage
- [API Reference](docs/api.md): Complete API documentation

## Optional

- [Contributing](CONTRIBUTING.md): How to contribute
```

### Implementation Pattern

codeNERD checks for llms.txt in multiple locations:

```go
llmsTxtURLs := []string{
    fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/llms.txt", owner, repo),
    fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/llms.txt", owner, repo),
    fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/.llms.txt", owner, repo),
}

for _, url := range llmsTxtURLs {
    content, err := r.fetchRawContent(ctx, url)
    if err == nil && len(content) > 10 {
        // Found llms.txt - use AI-optimized docs
        atoms := r.parseLlmsTxt(ctx, source, content)
        // llms.txt content gets higher base confidence (0.95)
        break
    }
}
```

### Parsing llms.txt

```go
func (r *ResearcherShard) parseLlmsTxt(ctx context.Context, source KnowledgeSource, content string) []KnowledgeAtom {
    var atoms []KnowledgeAtom
    lines := strings.Split(content, "\n")

    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }

        // Handle relative paths -> construct GitHub raw URL
        var docURL string
        if strings.HasPrefix(line, "http") {
            docURL = line
        } else {
            docURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s",
                source.RepoOwner, source.RepoName, strings.TrimPrefix(line, "/"))
        }

        content, err := r.fetchRawContent(ctx, docURL)
        if err == nil && len(content) > 50 {
            atom := KnowledgeAtom{
                SourceURL:   docURL,
                Title:       "AI-Optimized Documentation",
                Content:     r.truncate(content, 3000),
                Concept:     "llms_optimized",
                Confidence:  0.95,  // Higher for llms.txt
                ExtractedAt: time.Now(),
                Metadata:    map[string]interface{}{"source_type": "llms_txt"},
            }
            atoms = append(atoms, atom)
        }
    }
    return atoms
}
```

## KnowledgeAtom Schema

The core knowledge representation unit:

```go
type KnowledgeAtom struct {
    SourceURL   string                 `json:"source_url"`   // Where knowledge came from
    Title       string                 `json:"title"`        // Human-readable title
    Content     string                 `json:"content"`      // Main knowledge content
    Concept     string                 `json:"concept"`      // Categorical label
    CodePattern string                 `json:"code_pattern"` // Code example (optional)
    AntiPattern string                 `json:"anti_pattern"` // Anti-pattern warning (optional)
    Confidence  float64                `json:"confidence"`   // Quality score (0.0-1.0)
    Metadata    map[string]interface{} `json:"metadata"`     // Extensible metadata
    ExtractedAt time.Time              `json:"extracted_at"` // Timestamp
}
```

### Concept Categories

| Concept | Description | Typical Confidence |
|---------|-------------|-------------------|
| `overview` | High-level description | 0.85-0.95 |
| `code_example` | Working code snippet | 0.80-0.90 |
| `best_practice` | Recommended pattern | 0.75-0.85 |
| `anti_pattern` | What to avoid | 0.75-0.85 |
| `documentation_section` | Parsed doc section | 0.80-0.90 |
| `llms_optimized` | From llms.txt | 0.95 |
| `llm_synthesized` | LLM-generated | 0.70-0.85 |
| `project_identity` | Project metadata | 0.90-0.95 |
| `dependency` | Package dependency | 0.95 |

## Context7-Style Quality Scoring

Quality scoring filters low-value content:

```go
func (r *ResearcherShard) calculateC7Score(atom KnowledgeAtom) float64 {
    score := 0.5 // Base score

    // Content length checks
    if len(atom.Content) > 50 { score += 0.1 }
    if len(atom.Content) > 200 { score += 0.1 }
    if len(atom.Content) < 20 { score -= 0.3 }

    // Code example bonus
    if atom.CodePattern != "" && len(atom.CodePattern) > 20 {
        score += 0.15
    }

    // Title quality
    if atom.Title != "" && len(atom.Title) > 5 { score += 0.05 }

    // Source authority
    if strings.Contains(atom.SourceURL, "github") { score += 0.05 }

    // Garbage detection (heavy penalty)
    garbageIndicators := []string{
        "captcha", "robot", "verify you are human",
        "access denied", "403 forbidden", "404 not found",
        "please enable javascript", "cloudflare",
    }
    for _, indicator := range garbageIndicators {
        if strings.Contains(strings.ToLower(atom.Content), indicator) {
            score -= 0.5
        }
    }

    return clamp(score, 0, 1)
}
```

**Threshold:** Only atoms with `score >= 0.5` are persisted.

## 4-Tier Memory Architecture

Knowledge is stored across four complementary systems:

### Shard A: Working Memory (RAM)

```go
// In-memory Mangle FactStore
store := factstore.NewSimpleInMemoryStore()

// Content: Current turn atoms, active variables
// Lifecycle: Pruned every turn via spreading activation
```

### Shard B: Vector Store (Semantic Retrieval)

```sql
CREATE TABLE vectors (
    id INTEGER PRIMARY KEY,
    content TEXT NOT NULL,
    embedding BLOB,           -- For sqlite-vec (production)
    metadata TEXT,            -- JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

```go
// Storage
func (s *LocalStore) StoreVector(content string, metadata map[string]interface{}) error {
    metaJSON, _ := json.Marshal(metadata)
    _, err := s.db.Exec(`INSERT INTO vectors (content, metadata) VALUES (?, ?)`,
        content, string(metaJSON))
    return err
}

// Retrieval (keyword-based; production uses embeddings)
func (s *LocalStore) VectorRecall(query string, limit int) ([]VectorEntry, error) {
    keywords := strings.Fields(strings.ToLower(query))
    // Build OR conditions for each keyword
    // ORDER BY created_at DESC LIMIT ?
}
```

### Shard C: Knowledge Graph (Relational Links)

```sql
CREATE TABLE knowledge_graph (
    id INTEGER PRIMARY KEY,
    entity_a TEXT NOT NULL,
    relation TEXT NOT NULL,
    entity_b TEXT NOT NULL,
    weight REAL DEFAULT 1.0,
    metadata TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_kg_entity_a ON knowledge_graph(entity_a);
CREATE INDEX idx_kg_entity_b ON knowledge_graph(entity_b);
CREATE INDEX idx_kg_relation ON knowledge_graph(relation);
```

```go
// Store edges
func (s *LocalStore) StoreLink(entityA, relation, entityB string, weight float64, meta map[string]interface{}) error

// Retrieve links
func (s *LocalStore) QueryLinks(entity string, direction string) ([]KnowledgeLink, error)

// Path traversal (BFS)
func (s *LocalStore) TraversePath(from, to string, maxDepth int) ([]KnowledgeLink, error)
```

### Shard D: Cold Storage (Persistent Facts)

```sql
CREATE TABLE cold_storage (
    id INTEGER PRIMARY KEY,
    predicate TEXT NOT NULL,
    args TEXT NOT NULL,           -- JSON array
    fact_type TEXT DEFAULT 'general',
    priority INTEGER DEFAULT 50,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(predicate, args)       -- Prevent duplicates
);
```

```go
// Store fact
func (s *LocalStore) StoreFact(predicate string, args []interface{}, factType string, priority int) error

// Retrieve by predicate
func (s *LocalStore) LoadFacts(predicate string) ([]StoredFact, error)

// Bulk load with filtering
func (s *LocalStore) LoadAllFacts(factType string) ([]StoredFact, error)
```

## Known Sources Registry

The `knownSources` map provides direct paths to authoritative documentation:

```go
var knownSources = map[string]KnowledgeSource{
    "rod": {
        Name:       "Rod Browser Automation",
        Type:       "github",
        RepoOwner:  "go-rod",
        RepoName:   "rod",
        PackageURL: "github.com/go-rod/rod",
        DocURL:     "https://go-rod.github.io",
    },
    "mangle": {
        Name:       "Google Mangle Datalog",
        Type:       "github",
        RepoOwner:  "google",
        RepoName:   "mangle",
        PackageURL: "github.com/google/mangle",
    },
    "bubbletea": {
        Name:       "Bubble Tea TUI Framework",
        Type:       "github",
        RepoOwner:  "charmbracelet",
        RepoName:   "bubbletea",
    },
    // ... 19 total built-in sources
}
```

### Adding New Sources

```go
knownSources["mylib"] = KnowledgeSource{
    Name:       "My Library",
    Type:       "github",  // or "pkggodev", "llm"
    RepoOwner:  "owner",
    RepoName:   "repo",
    PackageURL: "github.com/owner/repo",
    DocURL:     "https://docs.mylib.io",  // Optional
}
```

**Source Types:**

- `github` - Fetch README + docs from GitHub raw URLs
- `pkggodev` - Falls back to GitHub (no public API)
- `llm` - Pure LLM synthesis (no authoritative source)

## LLM Knowledge Synthesis

For topics without authoritative sources, synthesize from LLM:

```go
func (r *ResearcherShard) synthesizeKnowledgeFromLLM(ctx context.Context, topic string, keywords []string) ([]KnowledgeAtom, error) {
    prompt := fmt.Sprintf(`Generate structured knowledge about "%s" in JSON:
{
  "overview": "2-3 sentence overview",
  "key_concepts": ["concept1", "concept2"],
  "best_practices": ["practice1", "practice2"],
  "common_patterns": [{"name": "...", "description": "...", "code": "..."}],
  "common_pitfalls": ["pitfall1", "pitfall2"],
  "related_technologies": ["tech1", "tech2"]
}`, topic)

    response, err := r.llmClient.Complete(ctx, prompt)
    // Parse JSON and convert to KnowledgeAtoms
    return r.parseLLMKnowledgeResponse(topic, response)
}
```

### LLM Enrichment (Context7-style)

Enrich raw documentation with AI summaries:

```go
func (r *ResearcherShard) enrichAtomWithLLM(ctx context.Context, atom KnowledgeAtom) KnowledgeAtom {
    if len(atom.Content) < 100 || atom.Concept == "llms_optimized" {
        return atom  // Skip short or already-optimized content
    }

    prompt := fmt.Sprintf(`Summarize this for an AI coding assistant (1-2 sentences):
%s`, r.truncate(atom.Content, 1000))

    summary, err := r.llmClient.Complete(ctx, prompt)
    if err == nil && len(summary) > 10 {
        atom.Metadata["original_content"] = atom.Content
        atom.Metadata["enriched"] = true
        atom.Content = strings.TrimSpace(summary)
    }
    return atom
}
```

## Research Execution Flow

### Mode 1: Codebase Analysis (`nerd init`)

```go
func (r *ResearcherShard) analyzeCodebase(ctx context.Context, workspace string) (*ResearchResult, error) {
    // 1. Scan file topology
    fileFacts, _ := r.scanner.ScanWorkspace(workspace)

    // 2. Detect project type (language, framework, build system)
    projectType := r.detectProjectType(workspace)

    // 3. Analyze dependencies (go.mod, package.json, etc.)
    deps := r.analyzeDependencies(workspace, projectType)

    // 4. Detect architectural patterns
    patterns := r.detectArchitecturalPatterns(workspace, fileFacts)

    // 5. Find important files (README, config, entry points)
    importantFiles := r.findImportantFiles(workspace)

    // 6. Generate summary via LLM
    result.Summary, _ = r.generateCodebaseSummary(ctx, result)

    return result, nil
}
```

### Mode 2: Web Research (knowledge building)

```go
func (r *ResearcherShard) conductWebResearch(ctx context.Context, topic string, keywords []string) (*ResearchResult, error) {
    // Strategy 1: Check knownSources
    if source, ok := r.findKnowledgeSource(topic); ok {
        atoms, _ := r.fetchFromKnownSource(ctx, source, keywords)
        result.Atoms = append(result.Atoms, atoms...)
    }

    // Strategy 2: LLM synthesis (always supplement)
    if r.llmClient != nil {
        llmAtoms, _ := r.synthesizeKnowledgeFromLLM(ctx, topic, keywords)
        result.Atoms = append(result.Atoms, llmAtoms...)
    }

    return result, nil
}
```

## Persistence Workflow

```go
func (r *ResearcherShard) persistKnowledge(result *ResearchResult) {
    for _, atom := range result.Atoms {
        // Shard B: Vector Store (semantic retrieval)
        r.localDB.StoreVector(atom.Content, map[string]interface{}{
            "source_url": atom.SourceURL,
            "concept":    atom.Concept,
            "confidence": atom.Confidence,
        })

        // Shard C: Knowledge Graph (relational links)
        r.localDB.StoreLink(atom.Concept, "has_instance", atom.Title, atom.Confidence, nil)
        if atom.CodePattern != "" {
            r.localDB.StoreLink(atom.Title, "has_pattern", atom.CodePattern, 0.9, nil)
        }

        // Shard D: Cold Storage (persistent facts)
        r.localDB.StoreFact("knowledge_atom",
            []interface{}{atom.SourceURL, atom.Concept, atom.Title, atom.Content},
            "research",
            int(atom.Confidence*100))
    }
}
```

## Fact Generation for Mangle

```go
func (r *ResearcherShard) generateFacts(result *ResearchResult) []core.Fact {
    facts := []core.Fact{
        {Predicate: "research_complete", Args: []interface{}{result.Query, len(result.Atoms), result.Duration.Seconds()}},
    }

    for _, atom := range result.Atoms {
        facts = append(facts, core.Fact{
            Predicate: "knowledge_atom",
            Args:      []interface{}{atom.SourceURL, atom.Concept, atom.Title, atom.Confidence},
        })

        if atom.CodePattern != "" {
            facts = append(facts, core.Fact{
                Predicate: "code_pattern",
                Args:      []interface{}{atom.Concept, atom.CodePattern},
            })
        }
    }
    return facts
}
```

## Specialist Hydration (Type B Shards)

When spawning a Persistent Specialist:

```mangle
# Trigger research if knowledge not ingested
needs_research(Agent) :-
    shard_profile(Agent, _, Topics, _),
    not knowledge_ingested(Agent).
```

**Hydration Flow:**

1. `nerd define-agent --name "RustExpert" --topic "Tokio Async"`
2. ResearcherShard executes Deep Research
3. KnowledgeAtoms written to `.nerd/shards/rustexpert_knowledge.db`
4. On spawn: mount knowledge DB as read-only
5. "Viva voce" exam validates knowledge

## Common Pitfalls

### 1. Not Checking llms.txt First

```go
// Wrong: Go straight to README
content, _ := r.fetchRawContent(ctx, readmeURL)

// Correct: Check llms.txt first (higher quality)
for _, url := range llmsTxtURLs {
    if content, err := r.fetchRawContent(ctx, url); err == nil {
        return r.parseLlmsTxt(ctx, source, content)
    }
}
// Fallback to README
```

### 2. Persisting Low-Quality Content

```go
// Wrong: Persist everything
atoms = append(atoms, atom)

// Correct: Filter by quality score
score := r.calculateC7Score(atom)
if score >= 0.5 {
    atoms = append(atoms, atom)
} else {
    fmt.Printf("Discarding low-quality: %s (%.2f)\n", atom.Title, score)
}
```

### 3. Missing Metadata for Retrieval

```go
// Wrong: Content only
r.localDB.StoreVector(atom.Content, nil)

// Correct: Include retrieval metadata
r.localDB.StoreVector(atom.Content, map[string]interface{}{
    "source_url": atom.SourceURL,
    "concept":    atom.Concept,
    "confidence": atom.Confidence,
    "enriched":   atom.Metadata["enriched"],
})
```

### 4. Not Handling LLM Synthesis Fallback

```go
// Wrong: Only use known sources
if source, ok := knownSources[topic]; ok {
    return r.fetchFromKnownSource(ctx, source, keywords)
}
return nil, errors.New("unknown topic")

// Correct: Always supplement with LLM
atoms, _ := r.fetchFromKnownSource(ctx, source, keywords)
llmAtoms, _ := r.synthesizeKnowledgeFromLLM(ctx, topic, keywords)
return append(atoms, llmAtoms...), nil
```

## Production Checklist

Before claiming research systems production-ready:

- [ ] llms.txt parsing covers all standard locations
- [ ] Quality scoring filters garbage content
- [ ] All 4 memory tiers have working persistence
- [ ] Known sources cover major Go/JS/Python libraries
- [ ] LLM enrichment produces concise summaries
- [ ] Specialist hydration writes to correct DB path
- [ ] Viva voce examination validates knowledge
- [ ] Research results generate valid Mangle facts
- [ ] Error handling covers network failures, rate limits
- [ ] No hardcoded API keys or secrets

## References

For detailed specifications:

- [llmstxt.org](https://llmstxt.org/) - The llms.txt standard specification
- [Context7](https://github.com/upstash/context7) - Upstash's MCP implementation (inspiration)
- [references/shard-agents.md](../codenerd-builder/references/shard-agents.md) - ShardAgent lifecycle and types
