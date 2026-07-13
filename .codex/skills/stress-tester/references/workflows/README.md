# Historical scenario library

The files below this directory are threat-model and scenario-design inputs retained from earlier and concurrent codeNERD stress work. They are **advisory, not an executable command contract**. The package validator reports the live count.

Before using one:

1. Verify every CLI form against `nerd.exe --help`, the relevant subcommand help, or `cmd/nerd/` source.
2. Translate Unix-only snippets to the current host without changing their safety boundary.
3. Replace hard-coded predicate, queue, token, timeout, and workflow counts with live measurement.
4. Identify a deterministic oracle and bounded stop rule.
5. Promote the verified command into `../profile-registry.json` or execute it with an explicit one-off receipt.

Current supported surfaces include Cobra commands such as `status`, `query`, `spawn`, and `campaign start`; adversarial assault is an interactive chat command: `/campaign assault ...`. Historical `/new-session` and Unix `/tmp` examples must not be copied blindly.
