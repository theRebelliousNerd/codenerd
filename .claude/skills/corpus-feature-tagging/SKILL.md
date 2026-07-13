---
name: corpus-feature-tagging
description: >
  Normalizes machine-readable metadata for one codeNERD architecture corpus.
  Use when asked to add stable feature IDs, status, source paths, verification
  dates, or index-ready metadata without inferring implementation from prose.
metadata:
  version: 2.0.0
  author: codeNERD
  last-verified: 2026-07-13
---

# Corpus Feature Tagging

Work one corpus at a time.

Use existing metadata conventions in that corpus. When no local schema exists,
use only this minimal YAML frontmatter:

```yaml
---
feature-id: <stable-lowercase-id>
status: proposed | partial | implemented | blocked
source-paths:
  - <repo-relative-path>
last-verified: YYYY-MM-DD
---
```

Verify `status` and every source path against the live tree. Do not invent a
repo-wide registry or bulk-stamp unread documents. Update
`Docs/architecture/INDEX.md` only when the corpus index state changes.

