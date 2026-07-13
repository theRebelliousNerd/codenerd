# OPEN QUESTIONS — browser

> Last verified: 2026-07-13 · Only questions not already decided in code comments

## Ownership

1. **Who owns the long-lived Chrome process in agent mode?** CLI launch process, research singleton, or a Cortex-supervised manager with explicit Shutdown on session end?

2. **Should modular research tools reify into the Cortex kernel engine, a side engine, or remain fact-free by design?** Fact-free is simpler for fetch-like use; fact-full is required for honeypot-safe agent browsing.

## Safety policy

3. **Must Click/Type hard-fail inside SessionManager when honeypot rules match, or only at kernel permit time?** Hard-fail couples package to engine rules always; permit-time preserves operator escape hatches.

4. **Is `honeypot_suspicious_url` still a product requirement without Mangle string predicates?** If yes, which URL patterns (tracking pixels, bait paths)?

## Session durability

5. **Is TargetID-based reattach a supported product feature across Chrome restarts, or best-effort only while the same browser process lives?**

6. **Should SessionStore persist cookies (encrypted) for true resume, or remain metadata-only forever?**

## Scope boundaries

7. **Where does browser automation stop and CodeDOM / tactile file editing begin?** Routing table has both `/browser` and code_graph/fs tools — when does the agent prefer which?

8. **Should headless default flip to true for automated agent runs while CLI remains headful?** DefaultConfig is currently headful.

## Testing / ops

9. **Will CI always install Chrome for lifecycle tests, or keep Skip + optional integration tag indefinitely?**

10. **Is exporting `.mg` snapshots the long-term audit format, or should facts stream into knowledge store / glass box only?**
