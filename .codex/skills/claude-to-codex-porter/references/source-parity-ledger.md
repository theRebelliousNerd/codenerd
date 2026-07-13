# Source Parity Ledger

## Source Snapshot

The initial source inventory on 2026-07-09 contained 23 Markdown files under
`C:\CodeProjects\neurolog\.agents\skills\claude-to-codex-porter`. Every one of
those paths is present in this Vectryx package. Active policy files were adapted;
the source changelog and flat journal remain preserved evidence.

## Intentional Non-Parity

Two untracked NeuroLog-only helpers appeared in the source repository after the
initial snapshot:

| Source path | Classification | Reason |
|---|---|---|
| `scripts/refresh-agent-fleet.ps1` | `INTENTIONALLY UNCHANGED` | Hardcodes the NeuroLog root, role additions/retirements, memory paths, skill root, and a whole-fleet GPT-5.6 model rewrite. Its apply mode rewrites every target agent and deletes named roles. Running or adapting that campaign was not requested for Vectryx. |
| `scripts/sync-recent-skill-packages.ps1` | `INTENTIONALLY UNCHANGED` | Hardcodes a NeuroLog git baseline, package exception lists, `.agents/skills` ownership, model-name rewrites, and deletion propagation. It is a repository maintenance campaign, not a general Claude-to-Codex port primitive. |

These scripts were not copied as runnable artifacts. Their reusable lesson is
captured in the active workflow: bulk fleet refreshes and root consolidations
require explicit scope, current model guidance, a complete ledger, dry-run
evidence, and a human checkpoint before apply mode.

## Vectryx Additions

- `scripts/inventory_workspace.py`
- `scripts/test_inventory_workspace.py`
- `evals/evals.json`
- `references/journal/2026-07-09-vectryx-port-and-current-surface-refresh.md`

These additions are Vectryx-safe, read-only by default, and validated locally.
