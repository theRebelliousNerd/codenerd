#!/bin/bash

# Perfection Loop Setup Script
# Specialized Ralph loop for codeNERD perfection testing

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PROMPT_FILE="$PLUGIN_ROOT/prompts/CODENERD_PERFECTION.md"

# Parse arguments
MAX_ITERATIONS=200
COMPLETION_PROMISE="CODENERD PERFECTION ACHIEVED"

while [[ $# -gt 0 ]]; do
  case $1 in
    -h|--help)
      cat << 'HELP_EOF'
codeNERD Perfection Loop - Systematic testing for 100% reliability

USAGE:
  /ralph-wiggum:perfection-loop [OPTIONS]

OPTIONS:
  --max-iterations <n>   Maximum iterations (default: 200)
  -h, --help            Show this help

DESCRIPTION:
  Starts the codeNERD Perfection Loop - a 14-phase systematic test of all
  31+ subsystems including Ouroboros, Nemesis, Thunderdome, Dream State,
  and Prompt Evolution.

  Progress is tracked in: .nerd/ralph/perfection_state.json
  Bug documentation in: .nerd/ralph/bugs/

  Completion promise: "CODENERD PERFECTION ACHIEVED"
  Only output when ALL systems pass with clean logs.

PHASES:
  1-5:   Core stability (kernel, perception, session, campaign, safety)
  6:     Integration sweep
  7:     Demo app generation (Mini-CRM)
  8:     Ouroboros - self-generating tools
  9:     Nemesis - adversarial review
  10:    Thunderdome - attack arena
  11:    Dream State - hypothetical exploration
  12:    Prompt Evolution - self-learning
  13:    Autopoiesis integration
  14:    Final verification

EXPECTED DURATION: 8-16 hours

ROOT-CAUSE MANDATE: NO BAND-AIDS ALLOWED.
  When stress testing reveals failures, trace to root cause.
  Deleting, commenting out, or patching artifacts is FORBIDDEN.
HELP_EOF
      exit 0
      ;;
    --max-iterations)
      if [[ -z "${2:-}" ]] || ! [[ "$2" =~ ^[0-9]+$ ]]; then
        echo "❌ Error: --max-iterations must be a positive integer" >&2
        exit 1
      fi
      MAX_ITERATIONS="$2"
      shift 2
      ;;
    *)
      echo "❌ Unknown option: $1" >&2
      echo "   Use --help for usage" >&2
      exit 1
      ;;
  esac
done

# Verify prompt file exists
if [[ ! -f "$PROMPT_FILE" ]]; then
  echo "❌ Error: Perfection prompt not found at $PROMPT_FILE" >&2
  exit 1
fi

# Create state file
mkdir -p .claude

cat > .claude/ralph-loop.local.md <<EOF
---
active: true
iteration: 1
max_iterations: $MAX_ITERATIONS
completion_promise: "$COMPLETION_PROMISE"
started_at: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
loop_type: "perfection"
---

$(cat "$PROMPT_FILE")
EOF

# Output setup message
cat <<EOF
╔══════════════════════════════════════════════════════════════╗
║          CODENERD PERFECTION LOOP - ACTIVATED                ║
╚══════════════════════════════════════════════════════════════╝

Iteration: 1 of $MAX_ITERATIONS maximum
Completion Promise: $COMPLETION_PROMISE

Phases to complete: 14
Subsystems to test: 31+
Expected duration: 8-16 hours

State tracking: .nerd/ralph/perfection_state.json
Bug documentation: .nerd/ralph/bugs/

╔══════════════════════════════════════════════════════════════╗
║                    ROOT-CAUSE MANDATE                         ║
║                                                               ║
║  When you find bugs, trace to ROOT CAUSE using Five Whys.   ║
║  NO BAND-AIDS: Do not comment out, delete, or patch around. ║
║  Fix the generation pipeline, not the generated artifact.    ║
╚══════════════════════════════════════════════════════════════╝

The stop hook is now active. You will iterate until:
  - All 14 phases complete with clean logs
  - Demo app builds and tests pass
  - All autopoiesis systems demonstrated
  - You output: <promise>$COMPLETION_PROMISE</promise>

Starting Phase 1: Kernel Core Stability...

EOF

# Output the prompt
cat "$PROMPT_FILE"
