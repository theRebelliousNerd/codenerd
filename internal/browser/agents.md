# Browser subsystem guidance

- Match BrowserNERD's observable behavior through codeNERD-native managers,
  tools, prompt atoms, and the live Cortex kernel. Do not embed its standalone
  MCP server or create a second reasoning engine.
- Normal tabs share the selected browser profile. Isolation must be explicit;
  forks are always isolated. Enforce configured browser/tab limits before
  allocating Rod targets.
- Browser and event-stream lifetimes are manager-owned, not request-owned.
  Every tab close must cancel its stream; shutdown closes tabs before browsers.
- Sanitize URLs, headers, console/input/DOM values, and React props before facts
  reach a sink. Browser artifacts use private permissions and must resolve under
  configured writable roots, including through existing symlink parents.
- Browser evidence belongs in the live Cortex kernel. Package-local sinks are
  acceptable only for focused tests and standalone CLI export workflows.
- Console, network, toast, DOM-update, click/input, and state-change events are
  session-scoped at argument zero. Preserve timestamp positions declared in
  `schemas_browser.mg`; waits and current-route diagnosis depend on them.
- Flight evidence is recorded only after fact redaction, under the configured
  writable-root policy, with a current-user-only ACL and hard file/size/read
  ceilings. Request lifecycle evidence must remain paired.
- Explicit evidence exports create new files only; never overwrite an existing
  path selected by the model.
- Browser spec roots, indexes, and Markdown links are read-only and confined to
  the active workspace through symlinks. Keep catalog/check limits hard and
  evaluate invariants only through the live kernel's browser allowlist.
- Progressive observations must be token-bounded and issue only opaque,
  session-scoped refs. Navigation invalidates the ref generation; actions must
  re-identify through the private fingerprint registry and fail closed when a
  ref is stale or ambiguous.
