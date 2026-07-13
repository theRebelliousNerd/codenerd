---
name: corpus-jules-dispatcher
description: >
  Packages exhausted corpus-build failures into deterministic remediation
  packets for codeNERD's .quality_assurance/remediation workflow and Jules
  prompt contract. Use only after the owning worker and review lane exhausted
  their bounded fix budget.
metadata:
  version: 2.0.0
  author: codeNERD
  last-verified: 2026-07-13
---

# Corpus Jules Dispatcher

Create a failure packet; do not claim an external remediation run occurred
unless it was actually dispatched and observed.

Read:

- `.quality_assurance/remediation/`
- `Docs/jules-patch-remediation-prompt.md`
- the failing packet, contract, diff, and gate artifacts

Use `scripts/build_failure_packet.py` to write
`.corpus-build/jules/<attempt-id>.json` with:

- stable attempt and work-unit IDs
- exact failing commands and output paths
- spec and contract references
- allowed/forbidden files
- prior fixes attempted
- acceptance commands
- rollback boundary
- evidence required for closure

Never include credentials, fabricate Jules status, or broaden ownership beyond
the failed packet. The root orchestrator decides whether and how to submit the
packet.
