# Logging lifecycle

- Apply configuration before workspace initialization. Preserve workspace
  rebinding, log containment and sink ownership.
- `auditMu` protects lazy audit initialization and every access to the mutable
  audit sink, including close. Exercise concurrent writes and close under race.
- Logging failure must not flood the terminal once per runtime event.
