# sqlpragmas — Open Questions

> Last verified: **2026-07-13**  
> Real open questions — not rhetorical filler.

---

### Q1 — Should modest-host defaults ship?

Workstation-class 2–4 GiB caches and 8–16 GiB mmap windows are documented for large RAM machines. Should there be a fifth profile (`ProfileModest`) or config-driven scale factor for laptops?

**Tension:** leaf purity + simplicity vs broader hardware support.  
**Status:** Open; no field failure mandating change yet.

---

### Q2 — Unify call sites on `sqlpragmas` only?

The dual surface (`store.ApplyDefaultPragmas` vs direct leaf) helps migration and same-package store calls, but confuses greps and docs.

**Options:**  
A) Keep dual forever (current).  
B) Migrate all external callers to leaf; keep store aliases for store internals only.  
C) Deprecate store re-export with a long window.

**Status:** Open; B is mildly preferred for mid-layer clarity.

---

### Q3 — When do we enable foreign_keys by default?

Schemas declare FKs without enforcement. Enabling is a behavior change.

**Questions:**  
- Is there a migration that repairs orphans first?  
- Per-DB enable at schema version N?  
- Forever opt-in?

**Status:** Open; package comment currently freezes shared defaults to OFF.

---

### Q4 — How to guarantee multi-conn pragma inheritance?

If any hot path raises `MaxOpenConns` significantly, per-connection PRAGMAs may drift.

**Options:** driver-specific hooks, re-apply wrapper, document “keep MaxOpen=1 for critical DBs”.

**Status:** Open; needs owner in store architecture.

---

### Q5 — Is modernc a first-class supported driver?

Design assumes coexistence; tests only mattn. If pure-Go builds become primary for some targets, CI must reflect that.

**Status:** Open — depends on product packaging strategy.

---

### Q6 — Should successful apply ever log at Trace?

Silent success is intentional. For long-horizon campaign debugging, a Trace-level “applied ProfileHot to …” might help.

**Risk:** noise if Trace is commonly enabled.  
**Status:** Open, low priority.

---

### Q7 — Journal mode on shared network / cloud FS

WAL on some network filesystems is problematic. codeNERD assumes local NVMe-class paths for agent DBs.

**Question:** Do we need a “safe remote FS” profile that avoids WAL?

**Status:** Open only if remote workspace roots become first-class.

---

## Resolved for this corpus rebuild

| Question | Resolution |
|----------|------------|
| Is the package pre-implementation? | **No** — full production leaf with wide fan-out |
| Does it belong on OODA fact-flow? | **No** — precondition infrastructure |
| Is empty gap analysis correct? | **No** — real gaps are config, dual-driver tests, pool semantics |
