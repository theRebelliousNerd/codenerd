# Porter Phase Pipeline

Execute phases in order. Each phase file owns one decision boundary and has an
explicit gate. Do not skip ahead because a source path looks familiar.

1. `01_scope/phase.md` -- define scope and classify Codex target surfaces.
2. `02_inventory/phase.md` -- inventory source, target, registrations, and
   preserved evidence.
3. `03_transform/phase.md` -- make bounded changes by target surface.
4. `04_validate/phase.md` -- run target-specific validation.
5. `05_report/phase.md` -- report exact changes, gaps, and evidence.

The core invariant: source path never chooses the target. Runtime behavior and
activation semantics choose the target.
