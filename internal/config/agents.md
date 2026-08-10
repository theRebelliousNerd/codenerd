# Configuration maintenance guidance

- Treat `.nerd/config.json` as executable policy: reject unknown fields and
  trailing JSON instead of silently applying defaults.
- Persist API keys and OAuth settings with owner-only permissions through the
  atomic same-directory writer; never truncate a live config in place.
- Configuration wizards merge their owned fields into the existing
  `UserConfig`. They must not erase settings owned by another subsystem.
- Raw LLM I/O tracing is explicit opt-in and its files are private because
  prompts and responses can contain credentials or proprietary source.
- Normal chat, campaign, and factory boot paths must consume the same execution
  and provider settings. An invalid explicit config fails closed before ambient
  environment detection.
- Tool-budget adaptation is executable policy: preserve the explicit
  `adaptive_tool_budget=false` pointer value, bound extension size/count and
  repeat thresholds, and propagate the same resolved limits to chat, campaigns,
  and spawned agents.
- Native browser settings live under the top-level `browser` block, separate
  from `integrations.servers.browser` (an external MCP endpoint). Preserve
  pointer semantics for `multi_tab_default` so explicit isolation is not
  overwritten by shared-tab defaults.
- Browser evidence settings govern a redacted, rotated, current-user-only JSONL
  recorder under the workspace. Preserve default-on pointer semantics and hard
  file-count/file-size ceilings when adding config migrations or wizards.
- Native browser `specs` config is a nested bounded catalog. Preserve enabled
  pointer semantics and workspace-only roots; configuration must not grant
  arbitrary filesystem read authority.
