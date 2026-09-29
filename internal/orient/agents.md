# Orientation package guidance

- This is one isolated Mangle engine; the embedded glob loads every policy file. Declare shared predicates exactly once.
- Go measures history, document contents, links and sources. Mangle chooses attention, source winners, agents, prompt sources and freshness.
- Keep the action-kernel projection separate from the engine policy and config facts. Avoid importing core: boot imports this package.
- History refresh uses a disjoint commit range and day witnesses; rewind, rename or missing witnesses require a whole-history pass.
- Replace changed roles/themes, preserve unchanged classifications, and expose pending transduction. Never rewrite orientation answers.
- Source review and authored tests are not runtime evidence. Acceptance requires the normal init/boot paths and the incremental-scan hook.
