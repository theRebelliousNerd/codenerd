# Pre-Implementation Honesty Markers

Apply these rules to every architecture corpus created before the feature exists.

1. Put this banner near the top of IMPLEMENTED_SPEC.md:

   **Status: Pre-Implementation. This document describes target state; it is not
   evidence that the feature is implemented.**

2. Mark implementation rows `Not Implemented` unless live code and tests prove
   otherwise.

3. Separate verified adjacent current state from planned feature state. Cite
   adjacent code with real file and symbol references.

4. Never invent a planned package merely because the feature name suggests one.
   State candidate paths as planned until created.

5. Describe tests as planned acceptance gates. Do not claim passing coverage or
   runtime metrics before measurement.

6. Record every assumption and unresolved decision in OPEN-QUESTIONS.md.

7. Keep status language evidence-based: proposed, planned, partially present,
   verified, blocked, or rejected.

8. Register the corpus in the Proposed section of Docs/architecture/INDEX.md.
   Promote it only after implementation evidence exists.

9. Avoid wall-clock, sprint, person-day, and cost estimates. Express order,
   dependencies, and gates instead.

10. Do not copy Vectryx, GraphCAD, Storyworld, Marine Layer, or other source-repo
    concepts unless live codeNERD evidence establishes a real integration.
