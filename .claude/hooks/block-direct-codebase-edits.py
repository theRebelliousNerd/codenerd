#!/usr/bin/env python3
"""Block Claude from editing codeNERD's source directly.

The point of this repo's dogfooding sessions is that codeNERD fixes codeNERD.
When Claude edits the Go source by hand, the thing under test never runs, and
the session silently turns into "Claude writes Go" — which is exactly the
outcome the exercise exists to avoid.

So: Edit/Write/NotebookEdit against source is refused. Use

    ./nerd.exe fix "<precise defect description>"

and then review, verify and commit what it produced.

WHAT IS STILL ALLOWED
  .claude/**, .codex/**   agent configuration, hooks, skills, ledgers
  .nerd/**                runtime state (config.json has its own separate rule)
  scratchpad / temp dirs  throwaway analysis
  test-only *_test.go     NO — tests are source too, codeNERD writes those

OVERRIDE
  Real exception: codeNERD is blocked by something it cannot fix itself (its
  own build is broken, the tool loop cannot reach the file). The standing
  agreement is that Claude fixes the BLOCKER so codeNERD can continue.

  To take it, write a reason first:

      echo "why codeNERD cannot do this" > .claude/.dogfood-override

  The override is SINGLE USE — consumed by the next write — and every use is
  appended to .claude/dogfood-overrides.log with the file and the reason.
  There is no silent bypass, on purpose: an override Claude can take without
  leaving a trace is not a constraint.

Exit codes: 0 allow, 2 block (stderr goes back to Claude).
"""

import json
import os
import subprocess
import sys
from datetime import datetime, timezone

# Paths that are configuration or scratch, not the program under test.
ALLOWED_PREFIXES = (
    ".claude/",
    ".codex/",
    ".nerd/",
    ".git/",
)

# Everything that constitutes the program itself.
BLOCKED_PREFIXES = (
    "internal/",
    "cmd/",
    "pkg/",
    "Docs/",
)

BLOCKED_SUFFIXES = (".go", ".mg")

OVERRIDE_FILE = ".claude/.dogfood-override"
OVERRIDE_LOG = ".claude/dogfood-overrides.log"


def repo_root() -> str:
    try:
        out = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True, text=True, timeout=10,
        )
        if out.returncode == 0 and out.stdout.strip():
            return os.path.realpath(out.stdout.strip())
    except Exception:
        pass
    return os.path.realpath(os.getcwd())


def rel_to_repo(path: str, root: str):
    """Return the repo-relative POSIX path, or None if outside the repo."""
    if not path:
        return None
    try:
        real = os.path.realpath(os.path.abspath(path))
        rel = os.path.relpath(real, root)
    except Exception:
        return None
    if rel.startswith(os.pardir):
        return None  # outside the repo: scratchpad, temp, elsewhere
    return rel.replace(os.sep, "/")


def is_source(rel: str) -> bool:
    for prefix in ALLOWED_PREFIXES:
        if rel.startswith(prefix):
            return False
    if rel.endswith(BLOCKED_SUFFIXES):
        return True
    for prefix in BLOCKED_PREFIXES:
        if rel.startswith(prefix):
            return True
    return False


def consume_override(root: str, rel: str, tool: str):
    """Return the override reason if one is staged, consuming it. Else None."""
    path = os.path.join(root, OVERRIDE_FILE)
    if not os.path.exists(path):
        return None
    try:
        with open(path, "r", encoding="utf-8") as fh:
            reason = fh.read().strip()
    except Exception:
        reason = ""
    if not reason:
        return None

    try:
        os.remove(path)  # single use
    except Exception:
        pass

    try:
        stamp = datetime.now(timezone.utc).isoformat(timespec="seconds")
        with open(os.path.join(root, OVERRIDE_LOG), "a", encoding="utf-8") as fh:
            fh.write(f"{stamp}\t{tool}\t{rel}\t{reason}\n")
    except Exception:
        pass
    return reason


def main() -> int:
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return 0  # never break the session on a malformed payload

    tool = payload.get("tool_name", "")
    if tool not in ("Edit", "Write", "MultiEdit", "NotebookEdit"):
        return 0

    tool_input = payload.get("tool_input") or {}
    target = (
        tool_input.get("file_path")
        or tool_input.get("notebook_path")
        or ""
    )

    root = repo_root()
    rel = rel_to_repo(target, root)
    if rel is None or not is_source(rel):
        return 0

    reason = consume_override(root, rel, tool)
    if reason:
        print(
            f"[dogfood] override consumed for {rel}: {reason}\n"
            f"[dogfood] logged to {OVERRIDE_LOG}; the next write is blocked again.",
            file=sys.stderr,
        )
        return 0

    print(
        f"BLOCKED: {tool} on {rel}\n"
        f"\n"
        f"This repo dogfoods itself — codeNERD fixes codeNERD. Editing the source\n"
        f"by hand means the system under test never runs.\n"
        f"\n"
        f"Do this instead:\n"
        f"  ./nerd.exe fix \"<precise description: symptom, file:line, root cause,\n"
        f"                   what to change, how to verify>\"\n"
        f"\n"
        f"Then review its diff, build it, test it, and commit. Sized-to-one-turn\n"
        f"tasks land; multi-file ones hit the tool-iteration ceiling — split them.\n"
        f"\n"
        f"If codeNERD genuinely cannot do this (its build is broken, the blocker is\n"
        f"in its own tool loop), stage a single-use override with the reason:\n"
        f"  echo \"why codeNERD cannot do this\" > {OVERRIDE_FILE}\n"
        f"Every override is recorded in {OVERRIDE_LOG}.",
        file=sys.stderr,
    )
    return 2


if __name__ == "__main__":
    sys.exit(main())
