---
schema: nerd/v1
northstar:
  purpose: >-
    Run one turn of the clean execution loop and report truthfully what it did.
    Everything else in this package serves that: tool dispatch, verification
    gates, and the signals a turn emits.
  requirements:
    - id: SESSION-VERIFY-HONEST
      statement: >-
        A verification gate that could not run must never report PASS. An unrun
        check is not a passed check, and reporting otherwise makes "we did not
        check" indistinguishable from "we checked and it was fine".
      severity: blocker
    - id: SESSION-NO-HOLLOW-SUCCESS
      statement: >-
        A turn whose intent requires side effects must not report success when
        no tool call succeeded, and a write-oriented turn must not report
        success without a recognised write-mutation tool.
      severity: blocker
    - id: SESSION-GATE-NAMES-STABLE
      statement: >-
        The verification gate indices and their names must stay in agreement;
        GateCount and the gateNames array are compile-time assertions and must
        not drift apart.
      severity: major
---

This package owns the clean execution loop, the tool dispatch path, and the verification gates. It is where a turn's honesty about its own work is enforced.
