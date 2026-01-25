# Init System Prompt Atoms

This directory contains prompt atoms for the codeNERD initialization system (`nerd init`).

## Overview

The init system performs cold-start initialization of codeNERD in a new project, creating the `.nerd/` directory structure, analyzing the codebase, and setting up Type 3 persistent agents with knowledge bases. These atoms guide the Researcher shard through each initialization phase.

## Init Phases

The init pipeline has many phases (currently 22 in `internal/init/initializer.go`). These atoms focus on the phases that require LLM guidance:

| Phase | File | Description |
|-------|------|-------------|
| Phase 3: Analysis | `analysis.yaml` | Deep codebase analysis to understand project structure, tech stack, and patterns |
| Phase 4: Profile | `profile.yaml` | Project profile generation (mostly structural, minimal LLM use) |
| Phase 5: Facts | `facts.yaml` | Mangle facts generation from profile (mostly structural) |
| Phase 6: Agents | `agents.yaml` | Agent recommendation logic (mostly rule-based) |
| Phase 7a: Shared KB | `kb_shared.yaml` | Common knowledge pool creation (universal concepts) |
| Phase 7b: Agent KBs | `kb_agent.yaml` | Agent-specific knowledge base hydration |
| Phase 7b: Codebase KB | `kb_codebase.yaml` | Project-specific codebase knowledge |
| Phase 7c: Core Shards KB | `kb_core_shards.yaml` | Knowledge bases for Coder, Reviewer, Tester |
| Phase 7d: Campaign KB | `kb_campaign.yaml` | Campaign orchestration workflow knowledge |

## Atom Structure

Each atom follows the standard format:

```yaml
- id: "init/phase/aspect"
  category: "init"
  subcategory: "phase_name"
  priority: 75
  is_mandatory: true/false
  shard_types: ["/researcher"]
  init_phases: ["/phase_name"]
  depends_on: ["parent_atom_id"]  # optional
  content: |
    Prompt content here...
```

## Key Atoms

### Analysis Phase
- **init/analysis/mission** - Primary analysis objectives and approach
- **init/analysis/constraints** - Speed, relevance, and output constraints

### Knowledge Base Creation
- **init/kb_shared/creation** - Shared knowledge pool (universal concepts)
- **init/kb_agent/research** - Agent-specific KB research strategy
- **init/kb_agent/quality** - KB quality assurance and metrics
- **init/kb_core_shards/creation** - Core shard KB hydration
- **init/kb_campaign/creation** - Campaign orchestration knowledge

## Usage

These atoms are loaded by the JIT prompt compiler when:
1. The Researcher shard is executing during `nerd init`
2. The current phase matches an `init_phases` filter
3. Priority and mandatory flags determine inclusion

The compiler injects the appropriate atoms into the researcher's prompt based on which init phase is currently executing.

## Research Strategy

The init system uses a hierarchical research strategy:

1. **Context7 First** - Use Context7 API for LLM-optimized docs (preferred)
2. **Web Search Fallback** - Fall back to web search if Context7 unavailable
3. **Graceful Degradation** - Use model knowledge as last resort
4. **Concept-Aware** - Skip topics already covered to avoid redundancy

## Quality Targets

Knowledge base quality is measured on a 0-100 scale:
- **90-100% (Excellent)**: Comprehensive coverage from authoritative sources
- **75-89% (Good)**: Solid coverage with minor gaps
- **60-74% (Adequate)**: Basic coverage, usable but incomplete
- **<60% (Needs Improvement)**: Significant gaps, recommend re-research

## Timing

Expected phase durations:
- Analysis: 60-90 seconds (deep codebase scan)
- Agent KB creation: 90-120 seconds total (15-20s per agent)
- Core shard KBs: 5-10 seconds each
- Campaign KB: 10-15 seconds

## Integration

See `internal/init/initializer.go` for the initialization orchestration that uses these atoms.


> *[Archived & Reviewed by The Librarian on 2026-01-25]*