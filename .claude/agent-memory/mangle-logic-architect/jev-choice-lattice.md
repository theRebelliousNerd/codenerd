---
name: jev-choice-lattice
description: Shipped 2026-10-28 - Jev Choice as jev_choice EDB rows read by a merge lattice (jev_chosen); the conversation-act layer; companion Noul was a dead end; probe numbers
metadata:
  type: project
---

The conversation act (turnjudge `conversation_act` Choice, 8 options) lands as `jev_choice(Q, Option, Pct)` for the whole distribution; `jev_choice_top` lattice (policy/jev_evidence.mg) keeps the top `fn:pair`, tie kept twice -> `jev_choice_tied`; `jev_chosen` needs top alone at or above yes. `policy/conversation_acts.mg` maps acts to `/answer_in_conversation` or `/ask_first`; never starts work. Replaced the `asks_assistant_state` Noul.
- Lab vs plain: fn:max join-back needs 3 strata (max, join, count for ties); lattice is one relation. Both give the same answers.
- Dead end: a companion "does any option fit" Noul never read no on 78 real messages (min 0.45) - not shipped.
- Probe (Jev 1.14, 70/30, production request): holdout 37/40 right 0 wrong; real messages 57/78, 3 wrong (same disposition), 18 unsettled.
- Wording that mattered: request_for_work must include "in codeNERD itself" or "debug yourself" reads as question_about_codenerd.

**Why:** reusable pattern for any Jev Choice (learnjudge ChooseVerb next).
**How to apply:** project a Choice as jev_choice and read jev_chosen; never pick the top in Go. See [[lattice-host-cone-and-preorder]], [[mangle-playbook]].
