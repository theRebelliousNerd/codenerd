# Ralph Wiggum Plugin

Implementation of the Ralph Wiggum technique - continuous self-referential AI loops for interactive iterative development.

## What is Ralph?

The Ralph Wiggum technique is an iterative development methodology where:

```bash
while :; do
  cat PROMPT.md | claude-code --continue
done
```

The same prompt is fed to Claude repeatedly. The "self-referential" aspect comes from Claude seeing its own previous work in files and git history, not from feeding output back as input.

## Installation

This plugin is installed locally in `C:\CodeProjects\codeNERD\.claude\plugins\ralph-wiggum\`

To use it, run Claude Code with the plugin directory:

```bash
claude --plugin-dir .claude/plugins/ralph-wiggum
```

Or load from the project root:

```bash
cd C:\CodeProjects\codeNERD
claude --plugin-dir .claude/plugins/ralph-wiggum
```

## Commands

### `/ralph-wiggum:ralph-loop`

Start an iterative Ralph loop.

```bash
/ralph-wiggum:ralph-loop "Build a REST API" --max-iterations 20 --completion-promise "DONE"
```

Options:
- `--max-iterations <n>` - Stop after N iterations (default: unlimited)
- `--completion-promise <text>` - Signal completion by outputting `<promise>TEXT</promise>`

### `/ralph-wiggum:cancel-ralph`

Cancel an active Ralph loop.

```bash
/ralph-wiggum:cancel-ralph
```

### `/ralph-wiggum:help`

Show help about the Ralph Wiggum technique.

## How It Works

1. You provide a prompt with clear success criteria
2. Claude works on the task
3. When Claude tries to exit, the Stop hook intercepts
4. Same prompt is fed back to Claude
5. Claude sees its previous work in files
6. Loop continues until completion promise or max iterations

## Best Practices

1. **Clear completion criteria** - Be explicit about what "done" means
2. **Set iteration limits** - Use `--max-iterations` for safety
3. **Testable goals** - Ralph works best with automatic verification (tests, linters)

## Credits

- Original technique: [Geoffrey Huntley](https://ghuntley.com/ralph/)
- Plugin author: Daisy Hollman (Anthropic)
