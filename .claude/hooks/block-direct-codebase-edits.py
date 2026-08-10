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

  An override must PROVE codeNERD was actually tried. Free-text reasons were
  self-granted in practice — 40 of them in a single session — which made this
  a speed bump rather than a constraint. So the file must now carry:

      ATTEMPTED: <path to a .nerd/logs/*.log from the failed run>
      REASON: <why codeNERD could not do it>

  The hook verifies that the named log exists, is recent, and actually records
  a failure. A reason with no verifiable attempt is refused.

  Overrides are also CAPPED at MAX_OVERRIDES_PER_DAY. Past the cap the answer
  is no until the next day — scarcity is the point. Ask the user to make the
  edit, or to lift the cap deliberately.

  Still SINGLE USE, and every use is appended to
  .claude/dogfood-overrides.log. An override that can be taken without leaving
  a trace is not a constraint.

  NOTE: .claude/settings.json also carries permissions.deny for internal/**,
  cmd/** and pkg/**. That is enforced by the harness, not by this hook, and
  Claude cannot grant it to itself — only the user can approve. This hook
  remains as defense in depth and as the place the reasoning is recorded.

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

# Scarcity is the mechanism. 40 overrides were taken in one session under the
# old free-text rule, which is what a non-constraint looks like.
MAX_OVERRIDES_PER_DAY = 3

# A cited codeNERD run has to be from this session's work, not yesterday's.
MAX_ATTEMPT_AGE_S = 60 * 60

# Substrings that show the cited run actually failed rather than succeeded.
FAILURE_MARKERS = (
    "shard execution failed",
    "execution failed",
    "llm generation failed",
    "broke the tests",
    "did not fix them",
    "build failed",
    "panic:",
    "blocked by shell-effect gate",
    "tool-iteration",
    "max iterations",
    "error:",
)


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


def overrides_used_today(root: str) -> int:
    """Count override log entries stamped with today's UTC date."""
    today = datetime.now(timezone.utc).strftime("%Y-%m-%d")
    try:
        with open(os.path.join(root, OVERRIDE_LOG), "r", encoding="utf-8") as fh:
            return sum(1 for line in fh if line.startswith(today))
    except Exception:
        return 0


def attempt_is_verifiable(root: str, body: str):
    """Return (ok, detail). An override must cite a real, failed codeNERD run.

    Free text was self-granted in practice, so the ATTEMPTED: line must name a
    log this hook can open, that is recent, and that actually records a
    failure. This cannot be satisfied without having run codeNERD.
    """
    attempted = ""
    for line in body.splitlines():
        if line.strip().upper().startswith("ATTEMPTED:"):
            attempted = line.split(":", 1)[1].strip()
            break
    if not attempted:
        return False, "no ATTEMPTED: line naming the log of a failed codeNERD run"

    log_path = attempted if os.path.isabs(attempted) else os.path.join(root, attempted)
    if not os.path.exists(log_path):
        return False, f"ATTEMPTED names {attempted!r}, which does not exist"

    age_s = abs(datetime.now(timezone.utc).timestamp() - os.path.getmtime(log_path))
    if age_s > MAX_ATTEMPT_AGE_S:
        return False, (
            f"ATTEMPTED log is {int(age_s // 60)} minutes old; "
            f"re-run codeNERD (limit {MAX_ATTEMPT_AGE_S // 60} minutes)"
        )

    try:
        with open(log_path, "r", encoding="utf-8", errors="replace") as fh:
            text = fh.read().lower()
    except Exception as exc:
        return False, f"could not read ATTEMPTED log: {exc}"

    if not any(m in text for m in FAILURE_MARKERS):
        return False, (
            "ATTEMPTED log records no failure — if codeNERD did not fail, "
            "it was not blocked, so fix it there"
        )
    return True, attempted


def consume_override(root: str, rel: str, tool: str):
    """Return the override reason if a VALID one is staged, consuming it.

    Returns None when no override is staged, or a ("__refused__", detail)
    tuple when one is staged but does not justify itself.
    """
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

    used = overrides_used_today(root)
    if used >= MAX_OVERRIDES_PER_DAY:
        try:
            os.remove(path)
        except Exception:
            pass
        return ("__refused__", f"daily override cap reached ({used}/{MAX_OVERRIDES_PER_DAY})")

    ok, detail = attempt_is_verifiable(root, reason)
    if not ok:
        try:
            os.remove(path)  # a refused override is still spent, so this cannot be brute-forced
        except Exception:
            pass
        return ("__refused__", detail)

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
    if isinstance(reason, tuple):
        # An override was staged but did not justify itself. It is already
        # spent, so this cannot be retried by simply writing the file again.
        print(
            f"[dogfood] OVERRIDE REFUSED for {rel}: {reason[1]}\n"
            f"[dogfood] An override must cite a codeNERD run that actually failed:\n"
            f"    ATTEMPTED: .nerd/logs/<the failed run>.log\n"
            f"    REASON: <why codeNERD could not do it>\n"
            f"[dogfood] Run ./nerd.exe fix \"...\" first. If it succeeds, there was "
            f"nothing to override.",
            file=sys.stderr,
        )
        return 2
    if reason:
        used = overrides_used_today(root)
        print(
            f"[dogfood] override consumed for {rel}: {reason}\n"
            f"[dogfood] {used}/{MAX_OVERRIDES_PER_DAY} used today; "
            f"logged to {OVERRIDE_LOG}. The next write is blocked again.",
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
