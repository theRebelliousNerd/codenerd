---
description: "Start the codeNERD Perfection Loop - systematic stress testing and root-cause fixing"
argument-hint: "[--max-iterations N]"
allowed-tools: ["Bash(${CLAUDE_PLUGIN_ROOT}/scripts/setup-ralph-loop.sh)"]
hide-from-slash-command-tool: "true"
---

# codeNERD Perfection Loop

This launches a specialized Ralph Wiggum loop designed to bring codeNERD to 100% tested reliability.

Execute the setup script with the perfection prompt:

```!
# Read the perfection prompt
PROMPT_FILE="${CLAUDE_PLUGIN_ROOT}/prompts/CODENERD_PERFECTION.md"

# Default iterations for perfection loop (high because we need thoroughness)
DEFAULT_ITERATIONS="100"

# Parse arguments
MAX_ITERATIONS="$DEFAULT_ITERATIONS"
for arg in $ARGUMENTS; do
  case "$arg" in
    --max-iterations)
      shift_next=true
      ;;
    *)
      if [ "$shift_next" = "true" ]; then
        MAX_ITERATIONS="$arg"
        shift_next=false
      fi
      ;;
  esac
done

# Read prompt content
if [ -f "$PROMPT_FILE" ]; then
  PROMPT_CONTENT=$(cat "$PROMPT_FILE")

  # Call the setup script with the prompt
  "${CLAUDE_PLUGIN_ROOT}/scripts/setup-ralph-loop.sh" "$PROMPT_CONTENT" --max-iterations "$MAX_ITERATIONS" --completion-promise "CODENERD PERFECTION ACHIEVED"
else
  echo "ERROR: Perfection prompt not found at $PROMPT_FILE"
  exit 1
fi
```

The perfection loop will:
1. Systematically test all 31+ codeNERD subsystems
2. Apply root-cause analysis to every failure
3. Build a demonstration app to prove stability
4. Continue until all logs are clean and demo works

CRITICAL: Only output the completion promise when it is GENUINELY TRUE.
