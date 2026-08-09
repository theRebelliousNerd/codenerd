# OPEN QUESTIONS — browser

> Last verified: 2026-08-09 · Only questions not already decided in code comments

## Reasoning and evidence

1. **Should `browser_mangle` ever accept caller-supplied rules?** The shipped BPAR-3 surface is deliberately read-only; a rule path needs a separate syntax, resource, lifetime, and constitutional authority contract.

2. **What retention window should supplement the shipped flight recorder's file-count and file-size ceilings?** Current behavior prunes oldest files globally; there is no age-based expiry.

## Safety policy

3. **Must ref-based interaction hard-fail inside SessionManager when honeypot rules match, or only at kernel permit time?** Hard-fail couples package to engine rules always; permit-time preserves operator escape hatches.

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

11. **Should unsafe JavaScript remain absent, or ship disabled-by-default with both config and explicit constitutional approval as BPAR-5 requires?**
