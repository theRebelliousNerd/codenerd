# ux — Open Questions

> Last verified: **2026-07-13**

## Q1. Who owns `.nerd/preferences.json`?

`internal/ux` defines the full schema; `internal/init` reads/writes agent selection with a different Go type. Which package is the long-term owner, and how do partial updates preserve foreign fields?

## Q2. Config onboarding vs UX journey — which is source of truth?

`config.OnboardingState` and `ux.JourneyPrefs` both track setup completeness and experience-ish signals. Wizard writes both in places. Should config become a projection of UX, or UX a projection of config?

## Q3. Should journey transitions ever demote?

Today only promotions exist. Long idle users or rising clarification rates do not move power→learning. Is demotion desirable, and would it annoy power users?

## Q4. Are transition thresholds final?

Values (`15` sessions, `20` successes, `0.15` clar rate, `50`/`200`/`5`) are hardcoded. Should they be config-tunable without code change?

## Q5. Where do intent corrections apply?

`RecordCorrection` stores pairs but no reader uses them for parse ranking. Perception? Retrieval? JIT atoms? Or retire the field?

## Q6. Is `GetDisclosureLevel` the public progressive API?

Help uses experience categories (`basic`/`advanced`/`expert`) rather than disclosure levels. Unify taxonomies or keep two layers (journey disclosure vs command category)?

## Q7. Session identity

`LastSession` is a timestamp string, not a session ID. Should UX link to usage tracker / session store IDs for analytics?

## Q8. Multi-workspace users

Preferences are per workspace directory. Is that correct for “user journey,” or should some state live in a user-global location?

## Q9. First-run definition

`IsFirstRun` = missing `.nerd` directory. Cloning a repo that already contains `.nerd` skips new-user path even for a human who never used codeNERD. Acceptable?

## Q10. Telemetry product decision

Keep schema-only forever, implement opt-in exporter, or remove fields to avoid false expectation?

## Q11. Chat PreferencesMgr liveness

Model holds a manager, but wizard constructs new ones. Should all mutations go through the model instance to avoid stale in-memory copies?

## Q12. Plan doc reference

`doc.go` cites `noble-sprouting-emerson.md`. Is that still the canonical product plan, and should architecture docs link it if it lives outside this tree?
