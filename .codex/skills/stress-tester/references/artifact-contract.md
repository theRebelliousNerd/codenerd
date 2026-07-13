# Stress artifact contract

Deterministic runs persist beneath `.nerd/campaigns/stress-tester/<run-id>/`:

```text
run.json                 run metadata and final verdict
summary.md               compact human-readable result
preflight.json           repository/tool/resource readiness gate
preflight.stderr.txt     bounded preflight diagnostic stream
commands/001.json        argv, timing, exit code, timeout, output paths
commands/001.stdout.txt  bounded standard output
commands/001.stderr.txt  bounded standard error
nerd-smoke.exe           build output when the smoke profile reaches build
adversarial.json         build hash plus every fixture verdict/output tail
```

`run.json` must distinguish `passed`, `failed`, `blocked`, and `dry_run`. It records the registered whole-run wall-clock ceiling, per-command timeout, commit, dirty-entry count and status fingerprint, and a pointer to the preflight receipt. Each command receipt records the exact argument array rather than a shell-joined approximation.

An executing run refuses to reuse a non-empty artifact directory. Default run IDs include microseconds so concurrent starts cannot silently overwrite each other's evidence. `--max-wall-seconds` may lower a profile's registered ceiling but may not raise it.

The adversarial receipt must retain the isolated checker build duration/hash and
one result per fixture, including output byte count/hash plus bounded head and
tail. The head preserves the causal checker marker even when verbose parser
diagnostics exceed the tail budget. A nonzero exit counts as rejection only when
output matches the checker's expected `ERROR in <file>:` contract and contains
no panic, fatal runtime, out-of-memory, concurrent-map, or race signal.

Log analysis writes the requested Markdown path and a sibling `<name>.json` sidecar. Live assault campaigns retain their native artifact layout under `.nerd/campaigns/<campaign>/assault/` and should not be copied into the deterministic runner directory.

Receipts may contain local paths and bounded command output. Review them before publication; do not persist credentials, environment dumps, or complete provider payloads. A `blocked` preflight is environmental evidence, never a product pass.
