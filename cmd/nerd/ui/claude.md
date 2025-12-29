# cmd/nerd/ui - TUI Components

Reusable Bubble Tea components for the codeNERD CLI, including campaign dashboards, diff viewers, and split-pane logic visualization.

## File Index

| File | Description |
|------|-------------|
| `campaign_page.go` | Campaign dashboard component showing phase progress, task list, and status. Exports `CampaignPageModel` with viewport scrolling and progress bar display. |
| `diffview.go` | Interactive diff approval component for reviewing proposed code changes before apply. Exports `FileDiff`, `DiffHunk`, and `PendingMutation` types with syntax highlighting. |
| `splitpane.go` | Glass Box Interface showing live Mangle derivations alongside chat. Exports `LogicPane` with collapsible derivation trees and spreading activation scores. |
| `styles.go` | Visual styling with codeNERD brand colors and light/dark mode support. Exports `Theme`, `Styles`, and `DefaultStyles()` using Lipgloss. |
| `usage_page.go` | Token usage statistics dashboard showing input/output counts by shard. Exports `UsagePageModel` wrapping the `usage.Tracker` data. |

---

**Remember: Push to GitHub regularly!**
