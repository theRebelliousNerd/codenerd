# Live Feature Matrix Summary (2026-07-13)

## App built by codeNERD (vehicle)
Path: `.nerd/live_feature_matrix/polystack`

| Stack | Status |
|-------|--------|
| Go backend :8080 | Real files; go build OK; /health /echo /status live |
| React+Vite frontend | Real scaffold; npm install+build produced dist/ |
| Rust sidecar :8081 | Real Cargo stdlib HTTP; cargo check OK; /ping OK |
| Python sidecar :8082 | Real stdlib http.server; /transform OK |

## codeNERD bugs found
1. **FIXED+PUSHED (d7a7ba2f)**: `nerd -w <dir> create` wrote files to monorepo CWD because modular tools use CODENERD_WORKSPACE_ROOT / Getwd, not VirtualStore workingDir.
2. **OPEN**: create/spawn often hang after printing Result (likely cortex.Close/maintenance); harness kills after ~45–55s post-result.
3. **ENV**: SuperGrok OAuth refresh token revoked (`invalid_grant`); live matrix used engine=api + xai_api_key. Re-run `nerd auth grok` for subscription path.
4. **CLI UX**: `tool`, `check-mangle` require args; `test-context` fails type assert CortexKernel vs RealKernel.

## Surfaces exercised (not exhaustive of every stress-tester workflow)
- Diagnostics: status, agents, jit, glassbox, transparency, reflection, embedding, sessions, logs, mcp, memory, logic, knowledge, campaign list, autopoiesis, why
- App path: init, northstar, scan, create (polyglot), spawn tester, analyze, explain, review, shadow, security, whatif
- NOT fully proven this run: dream marathon, campaign multi-phase, browser/rod, Ouroboros tool gen, mangle adversarial suite, long chaos, image_generator, interactive TUI

## Evidence
- MATRIX.md and *.out under %TEMP%\codenerd-live-matrix
- App under .nerd/live_feature_matrix/polystack
