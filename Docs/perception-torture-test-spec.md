# Perception Pipeline Torture Test Specification

**File:** `internal/perception/xai_torture_test.go`
**Tests:** 24 top-level test functions containing ~120+ subtests
**Lines:** ~1050
**Run commands:**

Pure Go tests (fast, no API key needed):
```bash
CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" \
  go test ./internal/perception/ -run TestTorture_PureGo -count=1 -timeout 60s -v
```

Full suite with LLM (requires `XAI_API_KEY`):
```bash
CODENERD_LIVE_LLM=1 CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" \
  go test ./internal/perception/ -run TestTorture -count=1 -timeout 600s -v
```

## Design Philosophy

Tests are split into two tiers:

1. **Pure Go tests** (`TestTorture_PureGo_*`): Deterministic, no network, no LLM. Test all internal Go logic functions directly. These run in CI without API keys.
2. **LLM integration tests** (`TestTorture_XAI_*`, `TestTorture_Intent_*`, etc.): Live LLM calls via xAI/Grok. Accept ranges of acceptable answers for LLM non-determinism. Gated behind `CODENERD_LIVE_LLM=1`.

LLM tests use rate limit handling via `skipOnRateLimit()` -- affected tests are skipped rather than failed.

## Test Gate and Client Setup

```go
func requireLiveXAIClient(t *testing.T) *XAIClient
```

1. Checks `CODENERD_LIVE_LLM=1` env var (skips if not set)
2. Reads API key from `XAI_API_KEY` env var, falls back to `config.LoadUserConfig()`
3. Creates `XAIClient` with model `grok-4-1-fast-reasoning`
4. Skips if no API key available

## Test Matrix

| # | Category | Test Count | Subtest Count | Convention | Purpose |
|---|----------|-----------|---------------|------------|---------|
| 1 | SanitizeFactArg | 1 | 13 | `TestTorture_PureGo_SanitizeFactArg` | Input sanitization for Mangle fact arguments |
| 2 | ExtractJSON | 1 | 12 | `TestTorture_PureGo_ExtractJSON` | JSON extraction from LLM responses |
| 3 | Understanding.Validate | 1 | 10 | `TestTorture_PureGo_UnderstandingValidate` | Understanding struct validation |
| 4 | Understanding Helpers | 1 | 3 | `TestTorture_PureGo_UnderstandingHelpers` | IsActionRequest, IsReadOnly, NeedsConfirmation |
| 5 | NormalizeLLMFields | 1 | 3 | `TestTorture_PureGo_NormalizeLLMFields` | Case normalization of LLM output fields |
| 6 | UnderstandingToIntent | 1 | 14 | `TestTorture_PureGo_UnderstandingToIntent` | Understanding -> UserIntent mapping |
| 7 | ParseResponse | 1 | 9 | `TestTorture_PureGo_ParseResponse` | JSON response parsing (envelope & direct) |
| 8 | GetRegexCandidates | 1 | 15 | `TestTorture_PureGo_GetRegexCandidates` | Verb taxonomy regex matching |
| 9 | ExtractTarget | 1 | 6 | `TestTorture_PureGo_ExtractTarget` | Target extraction from NL input |
| 10 | ExtractConstraint | 1 | 4 | `TestTorture_PureGo_ExtractConstraint` | Constraint extraction from NL input |
| 11 | RefineCategory | 1 | 5 | `TestTorture_PureGo_RefineCategory` | Category refinement heuristics |
| 12 | MemoryOperations | 1 | 3 | `TestTorture_PureGo_MemoryOperations` | Memory promote/forget operations |
| 13 | XAI Client | 4 | 4 | `TestTorture_XAI_*` | Client health, connectivity, protocol |
| 14 | Intent Classification | 1 | 6 | `TestTorture_Intent_*` | NL->Intent via UnderstandingTransducer |
| 15 | Edge Cases | 5 | 5 | `TestTorture_Edge_*` | Boundary inputs (empty, unicode, adversarial) |
| 16 | Multi-Step Intent | 1 | 5 | `TestTorture_MultiStep_*` | Compound/multi-verb intents |
| 17 | Ambiguous Intent | 1 | 6 | `TestTorture_Ambiguous_*` | Ambiguous/vague input handling |
| 18 | Signal Detection | 1 | 6 | `TestTorture_Signal_*` | Question, hypothetical, urgency signals |
| 19 | Domain Classification | 1 | 5 | `TestTorture_Domain_*` | Security, concurrency, testing domains |
| 20 | Confidence Calibration | 1 | 4 | `TestTorture_Confidence_*` | Confidence score range validation |
| 21 | Conversation Context | 1 | 2 | `TestTorture_ConversationContext` | Multi-turn context shifting |
| 22 | Concurrency/Stress | 1 | 1 | `TestTorture_Concurrent` | Parallel LLM call safety |
| 23 | Adversarial LLM | 1 | 6 | `TestTorture_Adversarial_*` | Injection, smuggling, escape attacks |
| 24 | Taxonomy Full Coverage | 1 | 26 | `TestTorture_Taxonomy_*` | All verb categories via LLM |

**Totals:** 24 top-level tests, ~120+ subtests

---

## Pure Go Tests (Categories 1-12)

These tests require no LLM calls and validate all internal Go logic deterministically.

### TestTorture_PureGo_SanitizeFactArg (13 subtests)

**Purpose:** Validates `sanitizeFactArg()` -- the input sanitizer for Mangle fact arguments.

| Subtest | Input | Expected Behavior |
|---------|-------|-------------------|
| `normal_text` | Clean ASCII | Passes through unchanged |
| `null_bytes_stripped` | `"hello\x00world"` | Null bytes removed |
| `control_chars_stripped` | `"a\x01b\x02c"` | Control chars removed |
| `preserves_newlines` | `"line1\nline2"` | Newlines kept |
| `preserves_tabs` | `"col1\tcol2"` | Tabs kept |
| `preserves_carriage_return` | `"a\rb"` | CR kept |
| `ansi_escape_stripped` | `"\x1b[31mred\x1b[0m"` | ANSI codes removed |
| `unicode_preserved` | `"cafe\u0301"` | Unicode kept |
| `empty_string` | `""` | Empty string |
| `only_null_bytes` | `"\x00\x00"` | Empty string |
| `truncates_at_2048` | 3000-char string | Truncated to 2048 |
| `mangle_injection_preserved_KNOWN_GAP` | `"foo). bar("` | Passes through (known gap) |
| `mixed_control_and_valid` | Mixed | Valid chars kept |

### TestTorture_PureGo_ExtractJSON (12 subtests)

**Purpose:** Validates `extractJSON()` -- extracts JSON from LLM responses that may include markdown fences, thinking preambles, or trailing text.

| Subtest | Input Pattern | Expected |
|---------|--------------|----------|
| `plain_json` | Raw JSON | Extract directly |
| `markdown_fenced` | ` ```json {...} ``` ` | Strip fences |
| `text_preamble` | Text before JSON | Find and extract |
| `multiple_json_returns_last` | Two JSON objects | Return last one |
| `nested_json` | JSON with nested objects | Handle nesting |
| `no_json` | Plain text | Return empty |
| `empty_response` | `""` | Return empty |
| `json_with_escaped_braces_in_string` | `{"k": "a{b}c"}` | Handle correctly |
| `thinking_preamble` | `<think>...</think>{...}` | Skip thinking |
| `json_with_trailing_text` | `{...} more text` | Extract JSON only |
| `deeply_nested` | 5-level nesting | Handle depth |
| `understanding_envelope` | Full envelope format | Extract |

### TestTorture_PureGo_UnderstandingValidate (10 subtests)

**Purpose:** Validates `Understanding.Validate()` -- struct validation with required fields and confidence bounds.

Tests: valid case, missing required fields (PrimaryIntent, SemanticType, ActionType, Domain), confidence out of bounds (>1.0, <0.0), edge values (0.0, 1.0), minimal valid.

### TestTorture_PureGo_UnderstandingHelpers (3 subtests)

**Purpose:** Tests `IsActionRequest()`, `IsReadOnly()`, `NeedsConfirmation()` helper methods.

### TestTorture_PureGo_NormalizeLLMFields (3 subtests)

**Purpose:** Tests `normalizeLLMFields()` -- case normalization, nil safety, empty field preservation.

### TestTorture_PureGo_UnderstandingToIntent (14 subtests)

**Purpose:** Tests `understandingToIntent()` -- the mapping from Understanding struct to UserIntent Mangle atoms.

Covers: investigate (testing/general), implement, modify, refactor, verify, explain, remember, forget, attack, review (security/general), unknown fallback, nil understanding.

### TestTorture_PureGo_ParseResponse (9 subtests)

**Purpose:** Tests `parseResponse()` -- the full JSON parsing pipeline from raw LLM response to Understanding struct.

**Bug found and fixed:** The envelope detection logic used `json.Unmarshal` error check alone, but Go's `json.Unmarshal` silently succeeds with a zero-valued struct when the JSON lacks the `"understanding"` key. Fixed to check `envelope.Understanding.PrimaryIntent != ""` after unmarshal.

| Subtest | Input | Expected |
|---------|-------|----------|
| `valid_envelope` | Full envelope JSON | Extract understanding |
| `understanding_only_no_envelope` | Direct understanding JSON | Parse directly |
| `markdown_wrapped` | Fenced JSON | Strip and parse |
| `no_json_error` | Plain text | Return error |
| `empty_response_error` | `""` | Return error |
| `malformed_json_error` | `"{{{"` | Return error |
| `mixed_case_normalized` | `"MUTATION"` category | Normalize to `"mutation"` |
| `surface_response_copied_from_envelope` | Envelope with surface_response | Copy to understanding |
| `thinking_preamble_with_json` | `<think>...</think>{...}` | Skip preamble, parse |

### TestTorture_PureGo_GetRegexCandidates (15 subtests)

**Purpose:** Tests `getRegexCandidates()` -- the regex-based verb matching against the SharedTaxonomy.

**Bug found and fixed:** Multi-word synonyms like `"what if"` were stored with `/` prefix due to `verb_synonym` Decl using `bound [/name, /name]` instead of `bound [/name, /string]`. The `/` prefix prevented `strings.Contains()` matching. Fixed the schema and added defense-in-depth stripping.

| Subtest | Input | Expected Verb |
|---------|-------|---------------|
| `fix_bug` | "fix the login bug" | fix |
| `explain_code` | "explain how the kernel works" | explain |
| `refactor` | "refactor the shard manager" | refactor |
| `run_tests` | "run the tests" | test |
| `review_code` | "review the authentication code" | review |
| `search_for` | "search for usages of ParseIntent" | search |
| `create_new` | "create a new middleware" | create |
| `debug_failure` | "debug why tests are failing" | debug |
| `research_docs` | "research Go context patterns" | research |
| `git_commit` | "commit and push to main" | git |
| `dream_what_if` | "what if we redesigned..." | dream |
| `security_scan` | "scan for security vulnerabilities" | security |
| `delete_file` | "delete the old config" | delete |
| `hello_greeting` | "hello" | converse |
| `empty_input` | "" | (none) |

### TestTorture_PureGo_ExtractTarget (6 subtests)

**Purpose:** Tests `extractTarget()` -- file path, directory, function name, quoted, struct, and no-target cases.

### TestTorture_PureGo_ExtractConstraint (4 subtests)

**Purpose:** Tests `extractConstraint()` -- language, exclusion, "only" constraints, and no-constraint case.

### TestTorture_PureGo_RefineCategory (5 subtests)

**Purpose:** Tests `refineCategory()` -- imperative->mutation, question->query, instruction patterns, default passthrough.

### TestTorture_PureGo_MemoryOperations (3 subtests)

**Purpose:** Tests memory operation detection -- remember->promote, forget->forget, explain->no ops.

---

## LLM Integration Tests (Categories 13-24)

All gated behind `CODENERD_LIVE_LLM=1`. Accept ranges of acceptable answers for LLM non-determinism.

### Category 13: XAI Client (4 tests)

| Test | Purpose |
|------|---------|
| `TestTorture_XAI_HealthCheck` | Basic connectivity, echo "OK" |
| `TestTorture_XAI_SentinelToken` | Instruction following, exact token echo |
| `TestTorture_XAI_JSONOutput` | Structured JSON output, schema compliance |
| `TestTorture_XAI_SystemPromptOverride` | System prompt constraint strength |

### Category 14: Intent Classification (6 subtests)

`TestTorture_Intent_Classification` -- Full `UnderstandingTransducer.ParseIntent()` pipeline.

| Subtest | Input | Acceptable Verbs |
|---------|-------|-----------------|
| `fix_request` | "fix the login bug in auth.go" | fix, debug |
| `explain_request` | "explain how the kernel evaluates rules" | explain, research |
| `test_request` | "write tests for the perception transducer" | test, implement, generate |
| `refactor_request` | "refactor the virtual store" | refactor, implement |
| `review_request` | "review my changes in the last commit" | review, explain |
| `research_request` | "what is the best practice for Go error handling?" | research, explain |

### Category 15: Edge Cases (5 tests)

| Test | Input | Checks |
|------|-------|--------|
| `TestTorture_Edge_EmptyInput` | `""` | No panic/hang, accepts error or response |
| `TestTorture_Edge_UnicodeInput` | Japanese, emoji, German, French, Greek | UTF-8 handling |
| `TestTorture_Edge_InjectionAttempt` | Prompt injection attack | System prompt holds |
| `TestTorture_Edge_VeryLongInput` | ~10K chars | Graceful handling |
| `TestTorture_Edge_SpecialCharacters` | Shell metacharacters, ANSI | No injection |

### Category 16: Multi-Step Intent (5 subtests)

`TestTorture_MultiStep_Intent` -- Compound intents with multiple verbs.

| Subtest | Input |
|---------|-------|
| `fix_then_test` | "fix the null pointer in auth.go and then run the tests" |
| `review_refactor_test` | "first review the token validation, then refactor it" |
| `create_and_document` | "create a new REST endpoint for user profiles" |
| `three_phase_pipeline` | "audit the auth module, fix any issues, then write tests" |
| `conditional_multi` | "if the tests pass, deploy to staging" |

### Category 17: Ambiguous Intent (6 subtests)

`TestTorture_Ambiguous_Intent` -- Vague or ambiguous inputs that could be interpreted multiple ways.

| Subtest | Input | Expected Behavior |
|---------|-------|-------------------|
| `question_or_instruction` | "how about a code review?" | Lower confidence |
| `hypothetical_deletion` | "what if we removed auth.go?" | Not classify as delete |
| `complaint_vs_fix` | "this code is terrible" | Not default to fix |
| `standalone_verb` | "review" | Parse with lower confidence |
| `vague_request` | "help" | Low confidence |
| `just_a_file_path` | "kernel.go" | Non-empty response |

### Category 18: Signal Detection (6 subtests)

`TestTorture_Signal_Detection` -- Detection of pragmatic signals in user input.

| Subtest | Input | Expected Signal |
|---------|-------|----------------|
| `question_signal` | "how does the kernel work?" | is_question=true |
| `hypothetical_signal` | "what if we changed the API?" | is_hypothetical=true |
| `multi_step_signal` | "first review, then fix, then test" | multi_step=true |
| `negation_signal` | "don't change the public API" | is_negated=true |
| `urgency_high` | "URGENT: production is down..." | urgency >= high |
| `confirmation_signal` | "confirm before deleting..." | requires_confirmation=true |

**Known flaky:** `confirmation_signal` -- LLM sometimes doesn't set `requires_confirmation=true`.

### Category 19: Domain Classification (5 subtests)

`TestTorture_Domain_Classification` -- Domain-specific classification accuracy.

Domains tested: security, concurrency, testing, performance, git.

### Category 20: Confidence Calibration (4 subtests)

`TestTorture_Confidence_Calibration` -- Confidence scores fall within expected ranges.

| Subtest | Input | Expected Range |
|---------|-------|---------------|
| `high_confidence_specific` | "fix the null pointer in auth.go:42" | [0.70, 1.00] |
| `medium_confidence` | "maybe look at the auth stuff?" | [0.40, 0.95] |
| `low_confidence_vague` | "hmm" | [0.00, 0.80] |
| `greeting_high_confidence` | "hello!" | [0.70, 1.00] |

### Category 21: Conversation Context (2 subtests)

`TestTorture_ConversationContext` -- Multi-turn context management.

| Subtest | Scenario |
|---------|----------|
| `context_shifts_target` | Prior turn mentions auth.go, follow-up says "refactor it" |
| `context_shifts_verb` | Prior turn says "review loginHandler", follow-up says "now test it" |

### Category 22: Concurrency/Stress (1 subtest)

`TestTorture_Concurrent/parallel_5_calls` -- 5 parallel LLM calls, all must succeed.

### Category 23: Adversarial LLM (6 subtests)

`TestTorture_Adversarial_LLM` -- Adversarial attacks against the perception pipeline.

| Subtest | Attack Vector |
|---------|--------------|
| `mangle_injection` | Mangle syntax in user input |
| `system_prompt_override_attempt` | "Ignore all instructions" |
| `json_escape_attempt` | JSON escape sequences |
| `nested_json_confusion` | JSON within JSON |
| `unicode_smuggling` | Homoglyph/invisible chars |
| `extremely_long_target` | 5000-char target string |

### Category 24: Taxonomy Full Coverage (26 subtests)

`TestTorture_Taxonomy_FullCoverage` -- Tests all 26 verb categories via live LLM classification.

**Known flaky:** Some subtests depend on LLM classification accuracy. The LLM sometimes picks semantically related but different verbs (e.g., "review" -> `/security`, "search" -> `/analyze`). These are inherent LLM nondeterminism, not code bugs.

---

## Bugs Found By These Tests

### Bug 1: `parseResponse` Envelope Detection (Fixed)

**File:** `internal/perception/transducer_llm.go:109-132`

`json.Unmarshal` into `UnderstandingEnvelope` always succeeds even when JSON has no `"understanding"` key (Go leaves the struct zero-valued). The fallback path to direct `Understanding` unmarshal never triggered.

**Fix:** Check `envelope.Understanding.PrimaryIntent != ""` after unmarshal.

### Bug 2: `verb_synonym` Schema Type (Fixed)

**File:** `internal/core/defaults/schemas_intent.mg:106`

`verb_synonym` was declared as `bound [/name, /name]`, but synonyms are strings. Multi-word synonyms like `"what if"`, `"code review"` were coerced to `/name` atoms with `/` prefix, breaking both Go-side regex matching and Mangle inference joins.

**Fix:** Changed to `bound [/name, /string]` + defense-in-depth `/` prefix stripping in `getSynonyms()`.

---

## Relationship to codeNERD Architecture

These tests validate the **Observe** phase of the OODA loop:

```
User Input
    |
    v
[XAI Client] -- TestTorture_XAI_* validates this layer
    |
    v
[extractJSON / parseResponse] -- TestTorture_PureGo_ExtractJSON/ParseResponse validates
    |
    v
[UnderstandingTransducer.ParseIntent()] -- TestTorture_Intent_* validates
    |
    v
[getRegexCandidates / extractTarget] -- TestTorture_PureGo_GetRegex/Extract* validates
    |
    v
user_intent(ID, Category, Verb, Target, Constraint) -- Mangle atom asserted to kernel
```

If the perception layer fails, the kernel receives wrong `user_intent` facts, which cascade through policy rules to derive wrong `next_action` facts.

## Known Gaps (Future Work)

- No tests for `GroundReferences()` (resolving "that file" to concrete paths)
- No tests for other LLM providers (Gemini, Anthropic, OpenAI) -- only xAI/Grok
- No tests for the `AtomValidator` (Grammar-Constrained Decoding) integration
- Taxonomy full coverage tests are inherently flaky due to LLM nondeterminism -- could benefit from retry-with-backoff or softer assertions
- No precision/recall benchmarking across multiple LLM runs
