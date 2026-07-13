# 12 — Failure Modes: articulation

> Last verified: 2026-07-13  
> Concrete modes seen in code paths + mitigations.

## FM1 — Model returns plain prose (no JSON)

**Symptom:** `ParseMethod=fallback`, empty control, surface = raw text.  
**Log:** Error “Fallback parse: no valid Piggyback JSON…” (unless AllowPlain).  
**Mitigation:** Piggyback suffix in system prompt; schema-capable `CompleteWithSchema`; operator re-ask.  
**Risk:** No tools/mangle for that turn.

## FM2 — Partial / truncated envelope (token limit)

**Symptom:** Raw starts with `{` or fence and contains control field names; surface empty or incomplete.  
**Mitigation:** `looksLikePartialEnvelope` → `salvageSurfaceFromPartial` or `truncatedEnvelopeMessage` (may include truncated reasoning). Appends ` […truncated]` if mid-string salvage ≥ 8 chars.  
**Risk:** Partial user message; still no control.

## FM3 — Markdown-wrapped JSON only

**Symptom:** Direct parse fails; markdown path succeeds at confidence 0.95.  
**Mitigation:** `AllowMarkdownWrapped` default true.  
**Risk:** Low.

## FM4 — Mixed content / leading chatter + JSON

**Symptom:** `json_extracted`, confidence 0.85, warning about mixed content.  
**Mitigation:** Candidate scanner + last-match-wins for rich envelopes.  
**Risk:** Wrong candidate if multiple full envelopes (decoy defense prefers last).

## FM5 — Leading decoy envelope (injection)

**Symptom:** Attacker/model prepends fake control JSON.  
**Mitigation:** Pass 1 iterates candidates reverse; prefers both keys. Tests: `ExtractEmbeddedJSON_DecoyInjection`.  
**Risk:** Residual if only one incomplete rich candidate early.

## FM6 — Deeply nested / huge JSON (DoS)

**Symptom:** CPU/memory spike during extract.  
**Mitigation:** `maxJSONDepth=200`, `maxJSONCandidateSize=5MB`, noise candidate limit. Tests cover depth/size.  
**Risk:** Still allocates up to caps.

## FM7 — Invalid mangle updates

**Symptom:** Warnings about syntax/length/metacharacters; updates dropped.  
**Mitigation:** applyCaps filters. Session may further block.  
**Risk:** Model believes fact asserted when it was dropped — surface may overclaim (premature articulation class).

## FM8 — Constitutional block after parse

**Symptom:** Safety notice prepended; atoms removed.  
**Mitigation:** `ApplyConstitutionalOverride` from session path.  
**Risk:** Under-use on non-session paths.

## FM9 — JIT compile failure

**Symptom:** Warn log; legacy assembly used; optional `jit_fallback` fact.  
**Mitigation:** Multi-level baseline/template fallback.  
**Risk:** Larger or drift-prone prompts; missing atom selection quality.

## FM10 — Kernel query failures during assembly

**Symptom:** Warn on context atoms / missing template; continues without section.  
**Mitigation:** Non-fatal; hard-coded templates last resort.  
**Risk:** Under-informed model.

## FM11 — Strict mode total failure

**Symptom:** `Process` returns error; ValidationFailures++.  
**Mitigation:** Callers rarely set `RequireValidJSON=true` in interactive paths.  
**Risk:** Hard failure if enabled without schema-guaranteed models.

## FM12 — StreamParser never finds surface key

**Symptom:** Stream UI silent until end; final Process still parses buffer.  
**Mitigation:** Final full-buffer Process in chat helpers.  
**Risk:** Poor perceived latency; if final also fails → FM1/FM2.

## FM13 — Type coercion failures on control fields

**Symptom:** Unmarshal errors for confidence/required/mangle_updates shapes.  
**Mitigation:** Custom UnmarshalJSON tolerant paths; tests for coercion.  
**Risk:** Exotic shapes still fail → fallback.

## FM14 — Control leak to UI via caller bug

**Symptom:** User sees JSON control packet.  
**Mitigation:** Salvage + formatResponse patterns; package helpers return Surface.  
**Risk:** New caller prints `RawResponse` or entire envelope.

## FM15 — Schema / struct / suffix drift

**Symptom:** Schema-capable models omit new fields or include unknown fields under strict mode.  
**Mitigation:** Manual keep-in-sync; strict tests for unknown fields.  
**Risk:** Silent empty optional fields or strict parse fails.

## Operator response summary

| Mode | Immediate action |
|------|------------------|
| High fallback rate | Check system prompt includes Piggyback; enable schema completion |
| Control leaks | Audit display path for RawResponse |
| Assert failures | Inspect applyCaps warnings + session block reasons |
| Empty stream | Inspect full buffer; token limits |
| Bad tools | Cap hit? tool_name registration? |
