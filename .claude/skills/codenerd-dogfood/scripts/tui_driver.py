"""Persistent TUI conversation with codeNERD, the way a person has one.

Runs the real `nerd` TUI (the chat Model with its whole Go routing layer:
work modes, hold and go, background jobs, side turns, the brief, Jev turn
judgments) in a detached tmux session, types turns into it, and reads the
replies from the session's own records under .nerd/sessions/<id>/ rather
than scraping the screen.

Usage, from the repo root (one command per call; the session persists):
  python3 .claude/skills/codenerd-dogfood/scripts/tui_driver.py start [--yolo]
  python3 .claude/skills/codenerd-dogfood/scripts/tui_driver.py say "turn text"
  python3 .claude/skills/codenerd-dogfood/scripts/tui_driver.py wait [--timeout 600] [--quiet 8]
  python3 .claude/skills/codenerd-dogfood/scripts/tui_driver.py read [N]
  python3 .claude/skills/codenerd-dogfood/scripts/tui_driver.py screen
  python3 .claude/skills/codenerd-dogfood/scripts/tui_driver.py progress
  python3 .claude/skills/codenerd-dogfood/scripts/tui_driver.py restart [--yolo]
  python3 .claude/skills/codenerd-dogfood/scripts/tui_driver.py stop

Watch it (and type into it) from another terminal with `tmux attach -t
nerd-dogfood`. `restart` relaunches nerd in the same pane after a rebuild, so
an attached terminal stays attached; `stop` ends the tmux session.

`wait` returns when the session record has stopped changing for --quiet
seconds after the last turn and the screen has settled; background jobs may
still be running (see `progress`).
"""
import glob
import json
import os
import subprocess
import sys
import time

TMUX_SESSION = "nerd-dogfood"
STATE = ".nerd/tui_driver.json"
SESSIONS = ".nerd/sessions"


def tmux(*args, check=True):
    return subprocess.run(["tmux", *args], capture_output=True, text=True, check=check)


def alive():
    return tmux("has-session", "-t", TMUX_SESSION, check=False).returncode == 0


def screen():
    return tmux("capture-pane", "-p", "-J", "-t", TMUX_SESSION).stdout


def load_state():
    try:
        with open(STATE) as f:
            return json.load(f)
    except (OSError, ValueError):
        return {}


def save_state(state):
    with open(STATE, "w") as f:
        json.dump(state, f)


def session_dir(state):
    """The session folder this TUI writes: the newest created after start."""
    if state.get("session_dir") and os.path.isdir(state["session_dir"]):
        return state["session_dir"]
    started = state.get("started", 0)
    candidates = [d for d in glob.glob(os.path.join(SESSIONS, "session-*"))
                  if os.path.isfile(os.path.join(d, "session.json")) and os.path.getmtime(d) >= started - 1]
    if not candidates:
        return None
    best = max(candidates, key=os.path.getmtime)
    state["session_dir"] = best
    save_state(state)
    return best


def messages(state):
    d = session_dir(state)
    if not d:
        return []
    try:
        with open(os.path.join(d, "session.json")) as f:
            doc = json.load(f)
    except (OSError, ValueError):
        return []
    hist = doc.get("history") or doc.get("messages") or doc.get("History") or []
    out = []
    for m in hist:
        role = m.get("role") or m.get("Role") or "?"
        content = m.get("content") if "content" in m else m.get("Content", "")
        if isinstance(content, list):
            content = "\n".join(content)
        out.append((role, content or ""))
    return out


def cmd_start(argv):
    if alive():
        print("already running; `stop` or `restart` first")
        return 1
    started = time.time()
    tmux("new-session", "-d", "-s", TMUX_SESSION, "-x", "220", "-y", "60", nerd_command(argv))
    save_state({"started": started})
    return wait_ready()


def cmd_restart(argv):
    """Relaunches nerd in the existing pane (respawn-pane -k), so a terminal
    attached to the session stays attached across a rebuild."""
    if not alive():
        return cmd_start(argv)
    started = time.time()
    tmux("respawn-pane", "-k", "-t", TMUX_SESSION, nerd_command(argv))
    save_state({"started": started})
    return wait_ready()


def nerd_command(argv):
    return " ".join(["./nerd"] + (["--yolo"] if "--yolo" in argv else []))


def wait_ready():
    deadline = time.time() + 300
    while time.time() < deadline:
        if not alive():
            print("nerd exited during boot")
            return 1
        if "Ask me anything" in screen() or "Enter to send" in screen():
            print("ready")
            return 0
        time.sleep(2)
    print("boot did not finish in 300s; check `screen`")
    return 1


def cmd_say(argv):
    if not alive():
        print("not running; `start` first")
        return 1
    text = " ".join(argv)
    state = load_state()
    state["said_at"] = time.time()
    state["count_before"] = len(messages(state))
    save_state(state)
    # A bracketed paste, the way a terminal delivers pasted text: the TUI
    # receives it as one paste. Typing it as keys (send-keys -l) arrived
    # reordered and with characters dropped twice on 2026-10-28, while tmux
    # itself delivered the same keys to `cat` intact.
    subprocess.run(["tmux", "load-buffer", "-b", "nerd-dogfood-say", "-"], input=text, text=True, check=True)
    tmux("paste-buffer", "-p", "-d", "-b", "nerd-dogfood-say", "-t", TMUX_SESSION)
    tmux("send-keys", "-t", TMUX_SESSION, "Enter")
    print("sent")
    return 0


def cmd_wait(argv):
    timeout = float(argv[argv.index("--timeout") + 1]) if "--timeout" in argv else 600
    quiet = float(argv[argv.index("--quiet") + 1]) if "--quiet" in argv else 8
    state = load_state()
    before = state.get("count_before", 0)
    deadline = time.time() + timeout
    last_screen, last_change = None, time.time()
    while time.time() < deadline:
        if not alive():
            print("nerd exited")
            return 1
        scr = screen()
        if scr != last_screen:
            last_screen, last_change = scr, time.time()
        msgs = messages(state)
        replied = any(r == "assistant" for r, _ in msgs[before:])
        d = session_dir(state)
        rec_age = time.time() - os.path.getmtime(os.path.join(d, "session.json")) if d else 0
        if replied and rec_age >= quiet and time.time() - last_change >= quiet:
            print(f"settled: {len(msgs) - before} new message(s)")
            return 0
        time.sleep(2)
    print(f"timeout after {timeout:.0f}s; the turn may still be running (`screen`, `progress`)")
    return 2


def cmd_read(argv):
    n = int(argv[0]) if argv else 6
    for role, content in messages(load_state())[-n:]:
        print(f"[{role}]\n{content}\n")
    return 0


def cmd_screen(argv):
    print(screen())
    return 0


def cmd_progress(argv):
    d = session_dir(load_state())
    if not d:
        print("no session folder yet")
        return 1
    for agent in sorted(os.listdir(d)):
        path = os.path.join(d, agent, "transcript.json")
        if not os.path.isfile(path):
            continue
        try:
            with open(path) as f:
                events = json.load(f).get("events", [])
        except (OSError, ValueError):
            print(f"{agent}: transcript unreadable (being written?)")
            continue
        kinds = {}
        for e in events:
            kinds[e.get("kind")] = kinds.get(e.get("kind"), 0) + 1
        last = events[-1] if events else {}
        print(f"{agent}: {kinds} last={last.get('kind')} {last.get('tool') or last.get('model') or ''} at {last.get('time', '')}")
    return 0


def cmd_stop(argv):
    if alive():
        tmux("send-keys", "-t", TMUX_SESSION, "C-c")
        time.sleep(3)
        if alive():
            tmux("kill-session", "-t", TMUX_SESSION, check=False)
    print("stopped")
    return 0


COMMANDS = {"start": cmd_start, "restart": cmd_restart, "say": cmd_say, "wait": cmd_wait, "read": cmd_read,
            "screen": cmd_screen, "progress": cmd_progress, "stop": cmd_stop}


def main():
    if len(sys.argv) < 2 or sys.argv[1] not in COMMANDS:
        print(__doc__)
        return 2
    return COMMANDS[sys.argv[1]](sys.argv[2:])


if __name__ == "__main__":
    sys.exit(main())
