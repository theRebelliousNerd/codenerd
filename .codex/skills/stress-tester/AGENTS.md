# Stress-tester maintenance

- Keep `SKILL.md` concise; put executable profiles in
  `references/profile-registry.json` and deterministic behavior in `scripts/`.
- Every executing profile must remain opt-in, preflight-gated, receipt-producing,
  non-overwriting, and bounded by per-command and whole-run timeouts.
- Build checkers from the current source tree; never reuse a root binary as proof.
- Add tooling regressions under `tests/` and run `validate_skill.py`, the unittest
  suite, the system `quick_validate.py`, and the structural profile after changes.
- Keep `.codex/agents/stress-tester.toml` attached to native `.codex` skills.
- Do not add changelogs or migration reports to the skill package; receipts are
  the operational history.
