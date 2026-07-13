# Codex workspace guidance

- Keep `.codex/agents/*.toml`, `.codex/config.toml`, skill attachments, and hook registrations synchronized; validate all TOML/JSON after changes.
- Custom agents must have `name`, `description`, and `developer_instructions`. Fleet workers do not delegate recursively.
- Repository skills are codeNERD-native: preserve Mangle executive control, constitutional default deny, JIT prompt atoms, and VirtualStore action routing.
- Treat `.agents/skills` as the shared discovery surface and `.codex/skills` as the governed Codex workflow surface used by explicit agent attachments. Document intentional divergence instead of silently overwriting either tree.
- `arch-propose` creates pre-implementation architecture corpora. `corpus-build` consumes them through dependency-ordered specialist roles. Keep their agent registries and lifecycle hooks aligned.
- Stress validation starts with `.codex/skills/stress-tester/scripts/preflight.py` and a registered deterministic profile. Live `/campaign assault` runs require explicit campaign scope and retain artifacts under `.nerd/campaigns/<campaign>/assault/`.
- Hooks consume one JSON object on stdin, emit bounded JSON/context, fail safely, and never guess token usage.
- After changing this subtree, run the relevant package validators, Python tests/compilation, TOML/JSON parsing, and a Codex discovery probe.
