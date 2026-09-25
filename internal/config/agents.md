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
- A removed key stays removed. `rejectRemovedKeys` (`removed_keys.go`) fails
  the load and names the key and why it went: the tool loop is not bounded by
  counts (the working policy stops a stall) and runs are not bounded by wall
  clocks. When you delete a key, add it to those maps; never reintroduce a
  knob they name. The limits that remain reach chat, campaigns and spawned
  agents alike. `TestAgentsGuide_TeachesNoRemovedKey` fails if this file names
  a removed key again.
- A section the kernel's rules read gives them `config_param` rows through a
  `Params` method (`params.go`); a threshold that fails open when absent is
  declared `config_param_required` next to its rule.
- Root and secondary-slot `reasoning_effort` values are strict config fields.
  Provider factories decide support; Meta accepts only
  `minimal|low|medium|high|xhigh`, while other providers must omit the wire field.
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
