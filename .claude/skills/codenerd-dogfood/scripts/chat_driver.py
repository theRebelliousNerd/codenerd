"""Turn-by-turn driver for `nerd chat` (headless main agent).

Usage, from the repo root:
  python .claude/skills/codenerd-dogfood/scripts/chat_driver.py <outfile> "turn 1" "turn 2" ...

Starts `./nerd.exe chat`, waits for the `ready` line, then sends each turn
only after the previous turn's footer line `[tools executed=...]` (or an
`error:` line) has been seen, so every turn is a genuine follow-up. Records
wall time per turn and mirrors everything to <outfile>.
"""
import queue
import subprocess
import sys
import threading
import time

FOOTER = "[tools executed="
READY = "ready"


def main() -> int:
    if len(sys.argv) < 3:
        print(__doc__)
        return 2
    outfile = sys.argv[1]
    turns = sys.argv[2:]
    proc = subprocess.Popen(
        ["./nerd.exe", "chat"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        encoding="utf-8",
        errors="replace",
        bufsize=1,
    )
    lines: "queue.Queue[str | None]" = queue.Queue()

    def pump() -> None:
        assert proc.stdout is not None
        for line in proc.stdout:
            lines.put(line.rstrip("\n"))
        lines.put(None)

    threading.Thread(target=pump, daemon=True).start()
    log = open(outfile, "w", encoding="utf-8")

    def wait_for(pred, timeout: float) -> bool:
        deadline = time.time() + timeout
        while time.time() < deadline:
            try:
                line = lines.get(timeout=1.0)
            except queue.Empty:
                continue
            if line is None:
                log.write("<<EOF from nerd>>\n")
                log.flush()
                return False
            log.write(line + "\n")
            log.flush()
            if pred(line):
                return True
        log.write("<<timeout waiting>>\n")
        log.flush()
        return False

    t0 = time.time()
    ok = wait_for(lambda l: l.strip() == READY, timeout=600)
    log.write(f"## boot: ready={ok} in {time.time() - t0:.1f}s\n")
    log.flush()
    if not ok:
        proc.kill()
        return 2

    assert proc.stdin is not None
    for i, turn in enumerate(turns, 1):
        t = time.time()
        log.write(f"## send turn {i}: {turn}\n")
        log.flush()
        proc.stdin.write(turn + "\n")
        proc.stdin.flush()
        done = wait_for(lambda l: l.startswith(FOOTER) or l.startswith("error:"), timeout=1800)
        log.write(f"## turn {i} done={done} wall={time.time() - t:.1f}s\n")
        log.flush()
        if not done:
            break

    proc.stdin.write("/quit\n")
    proc.stdin.flush()
    proc.stdin.close()
    try:
        proc.wait(timeout=120)
    except subprocess.TimeoutExpired:
        proc.kill()
    while True:
        try:
            line = lines.get(timeout=2.0)
        except queue.Empty:
            break
        if line is None:
            break
        log.write(line + "\n")
    log.write(f"## exit={proc.returncode} total={time.time() - t0:.1f}s\n")
    log.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
