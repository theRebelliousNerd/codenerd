# Known environment issues

Problems that come from the toolchain or machine rather than from codeNERD, and
that will otherwise be rediscovered as phantom codeNERD bugs.

---

## Go 1.26 Green Tea GC crashes on Zen 3 (illegal instruction)

**Symptom.** Random `fatal error: unexpected signal during runtime execution`
during `go test ./...`, usually in whichever package happens to be under memory
pressure at the time. No test reports a failure — the package just dies, often
in a couple of seconds. Reruns of the same package in isolation pass.

**Diagnosis.** The faulting stack is entirely inside the Go runtime's garbage
collector, and the signal is `0xc000001d`, which on Windows is
`STATUS_ILLEGAL_INSTRUCTION`:

```
[signal 0xc000001d code=0x0 addr=0x0 pc=0x7ff62a76f6a0]
runtime/internal/runtime/gc/scan/scan_amd64.go:31
runtime/mgcmark_greenteagc.go:906
runtime/mgcmark.go:1441
```

`mgcmark_greenteagc.go` is the Green Tea garbage collector, which is on by
default in Go 1.26. An illegal instruction in its vectorised scan routine points
at an instruction the CPU does not implement.

**Environment where this was observed** (2026-08-08):

| | |
|---|---|
| Go | 1.26.4 windows/amd64 |
| CPU | AMD Ryzen 9 5950X (Zen 3 — no AVX-512) |
| GOEXPERIMENT | unset, so Green Tea is the default |

**Confirmed by bisecting the GC, not by inference.** Same four packages, same
machine, back to back:

```
GOEXPERIMENT=nogreenteagc   ->  all 4 packages ok, zero crashes
GOEXPERIMENT unset          ->  fatal error reproduces on the first attempt
```

**Mitigation.**

```powershell
$env:GOEXPERIMENT = "nogreenteagc"
```

Set it wherever Go builds or tests run on this machine. Pinning an earlier Go
toolchain also works.

**Why this matters beyond the test suite.** This is a garbage collector fault,
so it is not test-specific. Any long-running codeNERD process on this hardware
can take the same illegal instruction under memory pressure — a campaign, an
overnight run, the TUI. If this machine shows unexplained dirty shutdowns or
crashes with no codeNERD error in the logs, check this first.

**Not changed for you.** The repo's build configuration has deliberately been
left alone. Setting GOEXPERIMENT in `.nerd/config.json`'s `build.env_vars` would
make codeNERD's own builds stable while leaving manual `go build` and `go test`
still crashing, which is a worse kind of confusing. It is a machine-level
setting and belongs in the environment.

---

## Default Go package fan-out is much slower than `-p 4` on this host

**Symptom.** Heavy packages appear hung or hit their package timeout during
`go test ./...`, while the same packages pass alone. On this 32-logical-CPU
Windows host, Go's default package fan-out also competes with nested Go, Git,
gofmt, and Chrome subprocesses used by integration-shaped tests.

**Do not diagnose this from a green isolated rerun alone.** The 2026-08-09
suite repair first captured timeout goroutines and closed separate test defects:
live LLM calls from ordinary CLI tests, leaked signal waiters, nil-stdin child
processes entering Windows `os.DevNull`/`GetConsoleMode`, and a browser negative
selector consuming a shared 60-second context.

After those repairs, both complete commands pass with Green Tea disabled:

| Command | Slowest package | Result |
|---|---:|---|
| `go test -p 4 ./... -count=1 -timeout=15m` | `internal/core` 138.526s | pass |
| `go test ./... -count=1 -timeout=15m` | `internal/core` 573.983s | pass |

**Practical use.** Keep the standard command as the release/handoff gate. Use
`-p 4` for a faster stable diagnostic pass on this machine, especially when
other codeNERD/Test Forge Go processes are already resident. A package that
still fails under `-p 4` needs its own stack capture; do not dismiss it as
parallelism.

No repository-wide `GOFLAGS=-p=4` setting was added. That would hide the host
characteristic from CI and other developers rather than make the tests more
hermetic.
