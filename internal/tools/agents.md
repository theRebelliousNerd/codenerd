# Tool boundaries

- Every executable tool needs an explicit `Effect`. Builtin effects are fixed in
  `effects.go`; dynamic tools declare their effects at registration. Unknown
  names or missing declarations fail execution.
- Write, execute and external effects require the installed executive gate on
  production registry paths. Tests executing effectful fixtures must supply a
  real gate or an explicit test adapter, never a production fallback.
- Use `CanonicalWorkspaceRoot`, `WorkspaceRoot` and `ResolveWorkspacePath` as
  one identity contract. Do not compute `filepath.Rel` across unresolved and
  resolved roots; preserve containment across symlinks and Windows aliases.
- From the repository root on Windows, `scripts/test_windows_path_identity.ps1 -Full` runs the complete
  tagged suite with an actual 8.3 TEMP alias. Preserve canonical expectations
  without replacing the aliased input passed to the component under test.
