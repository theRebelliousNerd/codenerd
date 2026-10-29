# Polystack — live codeNERD validation (git-tracked evidence)

**Not a product app.** Multi-language **test vehicle** + durable live-run evidence for codeNERD (CLI, SuperGrok, campaign marathon).

## Paths

| Path | Purpose |
|------|---------|
| **Runtime vehicle** | `.nerd/live_feature_matrix/polystack/` (app sources + campaign DBs; under `.nerd/*` gitignore) |
| **This Docs tree** | **Git-tracked** reports and stdout for humans/agents |
| `LIVE_TEST_RESULTS/` | Marathon logs, CLI matrix, subagent REPORT.md files |
| `SPEC.md` | Vehicle goal |

## Agent rules

1. When you run live stress/campaigns, **write evidence under `Docs/live-validation/`** (and mirror next to the vehicle if useful).
2. Do **not** leave results only in `%TEMP%\codenerd-*`.
3. Never commit `config.json` API keys.
4. Prefer reading `LIVE_TEST_RESULTS/` before claiming “we tested X.”

## Re-run

```powershell
nerd auth status   # SuperGrok should be ready
nerd campaign status -w .nerd\live_feature_matrix\polystack
nerd campaign resume -w .nerd\live_feature_matrix\polystack --timeout 2h
# then copy new logs into Docs/live-validation/polystack/LIVE_TEST_RESULTS/
```

## Skill

Stress-tester **09-cli-workspace-matrix**.
