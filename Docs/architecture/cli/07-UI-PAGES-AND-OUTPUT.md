# 07 — UI Pages & Output (`cmd/nerd/ui`)

> Last verified: 2026-07-13

## 1. Role

`cmd/nerd/ui` provides reusable Bubble Tea / Lipgloss components used by the chat shell and specialized pages (campaign, JIT, shards, diff, autopoiesis).

## 2. Component inventory (selected)

| File | ≈Lines | Responsibility |
|------|-------:|----------------|
| `splitpane.go` | 864 | Multi-pane layout, filtering hooks |
| `diffview.go` | 687 | Diff rendering / scrolling |
| `shard_page.go` | 362 | Shard-focused page |
| `campaign_page.go` | ~ | Campaign visualization |
| `jit_page.go` | ~ | JIT prompt inspection UI |
| `autopoiesis_page.go` | ~ | Autopoiesis / Ouroboros UI |
| `styles.go` | ~ | Shared styles |
| `layout.go` | ~ | Layout primitives |
| `render_cache.go` | ~ | Render caching |
| `debounce.go` | ~ | Input debounce |
| `simple_table.go` | ~ | Table widget |
| `usage_page.go` | ~ | Usage display |
| `word_diff` tests | tests | Word-level diff coverage |

## 3. Output principles

1. **Stdout vs stderr** — prefer keeping machine-readable streams stable when adding progress UI (align with interactive vs scripted modes).
2. **Markdown** — chat message rendering uses Glamour (chat view path).
3. **Diff safety** — large diffs must remain scrollable (`diffview` tests exist).
4. **Performance** — `render_cache` and benchmarks exist to avoid full re-render storms.

## 4. Testing

UI package carries multiple `*_test.go` files (splitpane filter, diffview, keyboard navigation, styles, debounce, render cache). Prefer table-driven widget tests over full TUI golden screenshots unless flaky harness is controlled.
