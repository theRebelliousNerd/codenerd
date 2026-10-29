# jit internals

Verified 2026-09-21 against commit `3463477` (`main`). Unqualified line
references are to `internal/jit/config/types.go` or
`internal/jit/config/types_test.go`.

## The struct (`types.go:14-20`)

Five fields, all plain data:

| Field | What the factory paths put in it |
|---|---|
| `IdentityPrompt` | `Generate` stamps `result.Prompt` (`internal/prompt/config_factory.go:141-146`); `GenerateFallback` stamps the fallback string (`internal/prompt/config_factory.go:226-231`) |
| `IntentVerb` | `Generate` stamps `intents[0]` (`internal/prompt/config_factory.go:139-146`); the fallback trims and stamps (`internal/prompt/config_factory.go:216-231`) |
| `AllowedTools` | merged config-atom tools (`internal/prompt/config_factory.go:144`) |
| `Policies` | merged config-atom policies (`internal/prompt/config_factory.go:145`) |
| `Persona` | neither factory literal sets it (`internal/prompt/config_factory.go:141-146`, `:226-231`); factory-built configs always carry `""` |

## `Validate` (`types.go:34-52`)

1. `IdentityPrompt` must be non-blank after trimming (`:35-37`).
2. `Policies` must be non-empty (`:38-40`).
3. Every entry must satisfy `core.IsDefaultPolicyFile` (`:42-45`), which
   rejects aliases, traversal, missing modules, whitespace-padded and
   backslash paths (`internal/core/policy_inventory.go:126-142`).
4. No duplicate policy references (`:46-49`).
5. `AllowedTools` and `Persona` are deliberately unchecked (`:31-33`): an
   empty allowlist is a safe zero value because execution fails closed.

Canonicality is membership in the embedded inventory (`DefaultPolicyFiles`,
`internal/core/policy_inventory.go:97-111`): `defaults/policy/*.mg` plus the
root modules. Set IDs (`base`, `coder`, `tester`, `reviewer`, `researcher`,
`nemesis`, `tool_generator`) resolve to file lists via
`DefaultAgentPolicySetFiles`, always including `policy/constitution.mg` and
`policy/validation.mg` (`internal/core/policy_inventory.go:15-23`, `:80-83`,
`:144-155`).

## Test pins (`types_test.go:7-91`)

| Rule | Case | Lines |
|---|---|---|
| valid config | `Valid Config` (`policy/constitution.mg`, `policy/coder_safety.mg`) | `:14-21` |
| blank identity rejected | `Missing Identity`, `Whitespace-only Identity` | `:23-30`, `:32-39` |
| empty/nil policies rejected | `Empty Policies`, `Nil Policies Files` | `:41-48`, `:50-57` |
| alias rejected | `Missing Policy Alias` (`base.mg`) | `:59-65` |
| traversal rejected | `Traversal Policy` (`../policy/constitution.mg`) | `:67-73` |
| duplicate rejected | `Duplicate Policy` | `:75-81` |

The runner (`:84-90`) asserts `Validate` errors exactly when `wantErr` says so.
