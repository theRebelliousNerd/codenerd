# CLAUDE.md

This directory (`.claude/skills/skill-creator`) needs Claude-facing guidance.
Summarize expectations, SAF rules, and prompts specific to this module here.

## Production-Grade Code: The Zero-Tolerance Policy

**SymbioGen is enterprise infrastructure, not a prototype. Every line must be production-ready.**

**ABSOLUTELY FORBIDDEN - Never Create:**
- **Stubs/Placeholders**: No `pass`, no `raise NotImplementedError()`, no "TODO: implement later"
- **Mock Implementations**: No fake data generators, no hardcoded test responses in production paths
- **Shims**: No temporary compatibility layers or workarounds awaiting "proper" solutions
- **Future Flags**: No `if FEATURE_ENABLED:` gates for incomplete features
- **Backwards Compatibility Hacks**: No legacy code paths maintained for "old clients"
- **Commented-Out Alternatives**: No "keeping this just in case" code blocks
- **Aspirational Imports**: No imports for functionality not yet implemented
- **Partial Implementations**: No half-written functions with only the "happy path"

**The Completion Imperative - When You Encounter:**

**Stubs/Mocks/Placeholders Found**:
- **DO NOT DELETE**: This reduces functionality and masks the incompleteness
- **DO NOT SKIP**: Moving on leaves broken infrastructure
- **DO COMPLETE**: Write the full, production-grade implementation immediately
- **DO VERIFY**: Test edge cases, error paths, and integration points

**Unused Imports/Variables Found**:
- **DO NOT DELETE**: The import indicates intended functionality
- **DO NOT IGNORE**: This is a signal of incomplete work
- **DO INVESTIGATE**: Understand the original intent from context
- **DO IMPLEMENT**: Complete the feature the import/variable was meant to support
- **IF GENUINELY UNUSED**: Only after thorough investigation and confirmation that it represents abandoned work, then remove with explicit comment explaining why

**The "Leave It Better" Doctrine:**
Every file you touch must be more complete, more robust, and more production-ready when you're done. If you find technical debt, you pay it down immediately.

**Why This Matters:**
- **Compounding Debt**: One stub becomes ten becomes a legacy migration project
- **Hidden Fragility**: Mocks in production mean untested failure modes waiting to surface
- **Cognitive Load**: Future developers waste time distinguishing real from temporary code
- **Enterprise Trust**: Fortune 500 clients expect infrastructure that works, completely, always
- **The Sentience Gap**: Half-implemented features cannot contribute to wisdom

**The Standard is Binary:**
Code is either production-complete or it doesn't exist. There is no intermediate state.

---


WARNING! MAJOR FILE REORGANIZATION, SEARCH HEAVILY FOR MISSING IMPORTS OR FAILED IMPORTS.
