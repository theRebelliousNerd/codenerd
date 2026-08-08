# Internal / World Audit — Ranked Risk Report

**Scope:** Internal services & world-facing surfaces  
**Date:** 2026-03-28  
**Classification:** Internal  

## Summary
Short ranked view of residual risks from the internal/world audit. Items ordered by severity (impact × likelihood). Remediation owners and target dates are placeholders pending assignment.

## Ranked Risks

| Rank | ID | Risk | Severity | Likelihood | Impact | Status |
|------|-----|------|----------|------------|--------|--------|
| 1 | IW-01 | Unauthenticated world API endpoints expose internal state | Critical | High | High | Open |
| 2 | IW-02 | Over-privileged service accounts between internal and world tiers | Critical | Medium | High | Open |
| 3 | IW-03 | Missing rate limits / abuse controls on world ingress | High | High | Medium | Open |
| 4 | IW-04 | Incomplete audit logging for cross-boundary actions | High | Medium | High | Open |
| 5 | IW-05 | Secrets / config leakage via world error responses | Medium | Medium | Medium | Open |
| 6 | IW-06 | Weak isolation of multi-tenant world resources | Medium | Low | High | Open |
| 7 | IW-07 | Dependency / supply-chain drift on world edge services | Low | Medium | Medium | Accepted |

## Top Actions
1. **IW-01** — Enforce authn/authz on all world APIs; deny-by-default.  
2. **IW-02** — Least-privilege service identities; rotate and scope tokens.  
3. **IW-03** — Add rate limiting, WAF rules, and anomaly alerts on ingress.  
4. **IW-04** — Structured audit logs for internal↔world calls; retain and alert.  
5. **IW-05** — Sanitize errors; strip stack traces and internal identifiers.

## Notes
- No evidence of active exploitation in the audit window.  
- Re-test after remediation of Critical/High items.  
- Full findings and evidence: see companion audit notes.