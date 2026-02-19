package perception

// =============================================================================
// REAL TORTURE TESTS — Designed to BREAK the perception pipeline
// =============================================================================
//
// These tests are adversarial. Each one targets a specific implementation
// weakness found by code review. They aim to trigger:
//   - Panics (nil derefs, index out of range, stack overflow)
//   - Memory exhaustion (unbounded allocation)
//   - Data races (concurrent access to globals)
//   - Silent data corruption (UTF-8 mangling, nondeterminism)
//   - Hangs (infinite loops, deadlocks)
//
// Run with: CGO_CFLAGS="-IC:/CodeProjects/codeNERD/sqlite_headers" \
//   go test ./internal/perception/ -run TestBreak -count=1 -timeout 120s -v
//
// Run with race detector (CRITICAL for data race tests):
//   go test ./internal/perception/ -run TestBreak -count=1 -timeout 120s -v -race

import (
	"encoding/json"
	"math"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// =============================================================================
// extractJSON TORTURE
// =============================================================================

func TestBreak_ExtractJSON_UnbalancedBraces_10K(t *testing.T) {
	// Attack: 10,000 open braces with no closing braces.
	// The stack slice grows to 10K entries. Should not panic or OOM.
	input := strings.Repeat("{", 10_000)
	result := extractJSON(input)
	if result != "" {
		t.Errorf("expected empty result for unbalanced braces, got %d bytes", len(result))
	}
}

func TestBreak_ExtractJSON_UnbalancedBraces_1M(t *testing.T) {
	// Attack: 1 million open braces. Stack grows to 1M int entries (~8MB).
	// Tests whether the function allocates unboundedly.
	input := strings.Repeat("{", 1_000_000)

	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	result := extractJSON(input)

	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	if result != "" {
		t.Errorf("expected empty result for unbalanced braces, got %d bytes", len(result))
	}

	allocMB := float64(memAfter.TotalAlloc-memBefore.TotalAlloc) / 1024 / 1024
	t.Logf("1M unbalanced braces: allocated %.1f MB", allocMB)
	if allocMB > 100 {
		t.Errorf("excessive allocation: %.1f MB for 1M braces (expected <100MB)", allocMB)
	}
}

func TestBreak_ExtractJSON_ThousandCandidates(t *testing.T) {
	// Attack: "{}{}{}{}" repeated 5000 times = 10,000 balanced pairs.
	// Each pair becomes a candidate. All 10K candidates are checked with json.Valid().
	// The candidates slice grows to 10K entries.
	input := strings.Repeat("{}", 5_000)

	start := time.Now()
	result := extractJSON(input)
	elapsed := time.Since(start)

	t.Logf("5K balanced pairs: result=%q, took %v", result, elapsed)
	if elapsed > 5*time.Second {
		t.Errorf("took %v for 5K pairs — possible O(n^2) in json.Valid calls", elapsed)
	}
}

func TestBreak_ExtractJSON_NestedBraces_500Deep(t *testing.T) {
	// Attack: 500 levels of nested braces: {{{...}}}
	// Each closing brace pops the stack and adds a candidate.
	// The innermost candidate is "{}", then "{{}}", etc.
	// 500 candidates are created, each larger than the last.
	depth := 500
	input := strings.Repeat("{", depth) + strings.Repeat("}", depth)

	result := extractJSON(input)
	// The innermost {} is valid JSON (empty object)
	if result == "" {
		t.Error("expected to find at least the innermost {} as valid JSON")
	}
	t.Logf("500-deep nesting: result length=%d", len(result))
}

func TestBreak_ExtractJSON_5MB_Response(t *testing.T) {
	// Attack: 5MB response string with valid JSON buried in the middle.
	// Tests linear scan performance on large inputs.
	padding := strings.Repeat("x", 2_500_000)
	validJSON := `{"primary_intent":"fix","confidence":0.9}`
	input := padding + validJSON + padding

	start := time.Now()
	result := extractJSON(input)
	elapsed := time.Since(start)

	if result != validJSON {
		t.Errorf("failed to extract JSON from 5MB input, got %q", truncateForLog(result, 100))
	}
	t.Logf("5MB input: extraction took %v", elapsed)
	if elapsed > 5*time.Second {
		t.Errorf("extraction took %v on 5MB input — too slow", elapsed)
	}
}

func TestBreak_ExtractJSON_UnterminatedString(t *testing.T) {
	// Attack: JSON with unterminated string containing braces.
	// The brace tracker should stay in "inString" mode and not count these.
	// But what happens when the string never closes?
	input := `{"key": "this string never closes { { { { {`
	result := extractJSON(input)
	// Should NOT find valid JSON because the string is unterminated
	if result != "" {
		// If it found something, check it's actually valid
		if !json.Valid([]byte(result)) {
			t.Errorf("extractJSON returned invalid JSON from unterminated string: %q", result)
		}
	}
}

func TestBreak_ExtractJSON_EscapeSequenceAtEnd(t *testing.T) {
	// Attack: string ending with backslash — the escapeNext flag is set
	// but there's no next character.
	input := `{"key": "value\`
	result := extractJSON(input)
	if result != "" {
		t.Logf("unexpected result from trailing backslash: %q", result)
	}
}

func TestBreak_ExtractJSON_AlternatingBracesInStrings(t *testing.T) {
	// Attack: braces inside strings that look like they balance but shouldn't.
	// The tracker must correctly ignore braces inside quotes.
	input := `prefix {"outer": "has } inside", "real": true} suffix`
	result := extractJSON(input)
	if result == "" {
		t.Error("failed to extract JSON with braces inside strings")
	} else {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Errorf("extracted invalid JSON: %v (got: %q)", err, result)
		}
	}
}

func TestBreak_ExtractJSON_BinaryPayload(t *testing.T) {
	// Attack: binary data mixed with JSON.
	binary := make([]byte, 1000)
	for i := range binary {
		binary[i] = byte(i % 256)
	}
	input := string(binary) + `{"safe": true}` + string(binary)

	result := extractJSON(input)
	// Should either find the valid JSON or return empty, but NOT panic
	t.Logf("binary payload: result=%q (len=%d)", truncateForLog(result, 50), len(result))
}

func TestBreak_ExtractJSON_NullBytes(t *testing.T) {
	// Attack: null bytes throughout the input
	input := "{\x00\"key\"\x00:\x00\"value\"\x00}\x00"
	result := extractJSON(input)
	t.Logf("null bytes: result=%q", result)
}

// =============================================================================
// sanitizeFactArg TORTURE
// =============================================================================

func TestBreak_SanitizeFactArg_UTF8Truncation(t *testing.T) {
	// Attack: 2046 ASCII bytes followed by a 4-byte emoji (U+1F600 = 😀).
	// Total = 2050 bytes. Truncation at 2048 bytes splits the emoji mid-codepoint.
	// After truncation, iterating with `range s` should handle the split,
	// but the output may contain a replacement character or corrupt byte.
	prefix := strings.Repeat("a", 2046)
	emoji := "😀" // 4 bytes in UTF-8
	input := prefix + emoji

	if len(input) != 2050 {
		t.Fatalf("test setup error: expected 2050 bytes, got %d", len(input))
	}

	result := sanitizeFactArg(input)
	if !utf8.ValidString(result) {
		t.Errorf("CORRUPTION: sanitizeFactArg produced invalid UTF-8 (len=%d)", len(result))
		// Dump the last few bytes for diagnosis
		if len(result) > 5 {
			t.Logf("last 5 bytes: %x", []byte(result[len(result)-5:]))
		}
	} else {
		t.Logf("UTF-8 truncation: output is valid UTF-8, len=%d", len(result))
	}
}

func TestBreak_SanitizeFactArg_UTF8Truncation_2Byte(t *testing.T) {
	// Attack: 2047 ASCII bytes + 2-byte char (é = U+00E9 = 0xC3 0xA9).
	// Total = 2049 bytes. Truncation at 2048 splits the 2-byte char.
	prefix := strings.Repeat("a", 2047)
	twoByteChar := "é" // 2 bytes in UTF-8
	input := prefix + twoByteChar

	if len(input) != 2049 {
		t.Fatalf("test setup error: expected 2049 bytes, got %d", len(input))
	}

	result := sanitizeFactArg(input)
	if !utf8.ValidString(result) {
		t.Errorf("CORRUPTION: sanitizeFactArg produced invalid UTF-8 from 2-byte split")
	}
}

func TestBreak_SanitizeFactArg_UTF8Truncation_3Byte(t *testing.T) {
	// Attack: 2046 ASCII bytes + 3-byte char (€ = U+20AC = 0xE2 0x82 0xAC).
	// Total = 2049 bytes. Truncation at 2048 splits the 3-byte char.
	prefix := strings.Repeat("a", 2046)
	threeByteChar := "€" // 3 bytes in UTF-8
	input := prefix + threeByteChar

	result := sanitizeFactArg(input)
	if !utf8.ValidString(result) {
		t.Errorf("CORRUPTION: sanitizeFactArg produced invalid UTF-8 from 3-byte split")
	}
}

func TestBreak_SanitizeFactArg_MangleInjection(t *testing.T) {
	// Attack: Mangle rule injection through fact arguments.
	// If this passes through sanitizeFactArg, the intent's ToFact() could
	// inject rules into the kernel when the fact string is parsed.
	injections := []struct {
		name  string
		input string
	}{
		{"rule_injection", `foo"). malicious(X) :- permitted(X). bar("`},
		{"atom_terminator", `target). next_action(/rm_rf).`},
		{"comment_injection", `target # this comments out the rest`},
		{"predicate_spoof", `user_intent("/evil", "/mutation", "/delete", "/all", "none")`},
		{"nested_parens", `a(b(c(d(e(f(g(h(i(j)))))))))`},
		{"pipe_aggregation", `x |> do fn:group_by(Y), let Z = fn:count()`},
	}

	for _, tc := range injections {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeFactArg(tc.input)
			// Document what passes through — these are all KNOWN GAPS
			t.Logf("injection %q -> %q (len: %d -> %d)", tc.name, result, len(tc.input), len(result))

			// Check if dangerous chars survived
			hasParens := strings.Contains(result, "(") || strings.Contains(result, ")")
			hasPeriod := strings.Contains(result, ".")
			hasColonDash := strings.Contains(result, ":-")

			if hasParens || hasPeriod || hasColonDash {
				t.Logf("WARNING: Mangle syntax chars survived sanitization: parens=%v period=%v colonDash=%v",
					hasParens, hasPeriod, hasColonDash)
			}
		})
	}
}

func TestBreak_SanitizeFactArg_10MB_Input(t *testing.T) {
	// Attack: 10MB string. Truncation should cap at 2048 quickly.
	// But the input allocation itself is 10MB.
	input := strings.Repeat("A", 10_000_000)

	start := time.Now()
	result := sanitizeFactArg(input)
	elapsed := time.Since(start)

	if len(result) > 2048 {
		t.Errorf("truncation failed: output is %d bytes (expected <=2048)", len(result))
	}
	t.Logf("10MB input: truncation took %v, output=%d bytes", elapsed, len(result))
	if elapsed > 100*time.Millisecond {
		t.Errorf("truncation took %v — expected <100ms for simple slice", elapsed)
	}
}

func TestBreak_SanitizeFactArg_AllControlChars(t *testing.T) {
	// Attack: every control character from 0x00 to 0x1F.
	var input strings.Builder
	for i := 0; i < 32; i++ {
		input.WriteByte(byte(i))
	}

	result := sanitizeFactArg(input.String())
	// Only \n (0x0A), \r (0x0D), \t (0x09) should survive
	for _, r := range result {
		if r != '\n' && r != '\r' && r != '\t' {
			if r < 0x20 {
				t.Errorf("control char U+%04X survived sanitization", r)
			}
		}
	}
}

func TestBreak_SanitizeFactArg_ZeroWidthChars(t *testing.T) {
	// Attack: zero-width characters that are visually invisible but semantically significant.
	// These could cause confusion in Mangle atom matching.
	input := "fix\u200B\u200C\u200D\uFEFF the bug" // zero-width space, non-joiner, joiner, BOM
	result := sanitizeFactArg(input)
	// These are valid Unicode (>= 0x20), so they pass through.
	// This documents that zero-width chars are NOT stripped.
	t.Logf("zero-width chars: input=%d bytes, output=%d bytes, runes=%d",
		len(input), len(result), utf8.RuneCountInString(result))
	if result != input {
		t.Logf("NOTE: zero-width chars were modified: %q -> %q", input, result)
	}
}

// =============================================================================
// parseResponse TORTURE
// =============================================================================

func TestBreak_ParseResponse_NaN_Confidence(t *testing.T) {
	// Attack: JSON with NaN as confidence value.
	// Go's json.Unmarshal does NOT parse NaN — it should error.
	trans := &LLMTransducer{}
	response := `{"primary_intent":"fix","semantic_type":"action","action_type":"modify","domain":"general","confidence":NaN}`

	result, err := trans.parseResponse(response)
	if err == nil && result != nil {
		t.Logf("NaN confidence: parsed as %v (field value: %f)", result.PrimaryIntent, result.Confidence)
		if math.IsNaN(result.Confidence) {
			t.Error("DANGEROUS: NaN confidence value accepted — will corrupt downstream comparisons")
		}
	} else {
		t.Logf("NaN correctly rejected: %v", err)
	}
}

func TestBreak_ParseResponse_Infinity_Confidence(t *testing.T) {
	// Attack: JSON with Infinity as confidence value.
	trans := &LLMTransducer{}
	responses := []string{
		`{"primary_intent":"fix","confidence":Infinity}`,
		`{"primary_intent":"fix","confidence":-Infinity}`,
		`{"primary_intent":"fix","confidence":1e308}`,  // near max float64
		`{"primary_intent":"fix","confidence":1e309}`,  // overflow to +Inf
		`{"primary_intent":"fix","confidence":-1e309}`, // overflow to -Inf
	}

	for _, resp := range responses {
		result, err := trans.parseResponse(resp)
		if err == nil && result != nil {
			if math.IsInf(result.Confidence, 0) {
				t.Errorf("DANGEROUS: Infinity confidence accepted: %f (input: %s)", result.Confidence, truncateForLog(resp, 60))
			}
			t.Logf("confidence: %f for input %s", result.Confidence, truncateForLog(resp, 60))
		}
	}
}

func TestBreak_ParseResponse_DeeplyNestedJSON(t *testing.T) {
	// Attack: 1000 levels of nested JSON objects.
	// Go's json.Unmarshal has a default max nesting depth of 10000.
	// But this creates a 1000-deep structure that may be slow to parse.
	depth := 1000
	var sb strings.Builder
	for i := 0; i < depth; i++ {
		sb.WriteString(`{"a":`)
	}
	sb.WriteString(`"leaf"`)
	for i := 0; i < depth; i++ {
		sb.WriteString(`}`)
	}

	trans := &LLMTransducer{}
	start := time.Now()
	result, err := trans.parseResponse(sb.String())
	elapsed := time.Since(start)

	t.Logf("1000-deep JSON: err=%v, took %v", err, elapsed)
	if result != nil {
		t.Logf("result: primary_intent=%q", result.PrimaryIntent)
	}
}

func TestBreak_ParseResponse_HugeStringValue(t *testing.T) {
	// Attack: valid JSON with a 1MB string value for primary_intent.
	// Tests memory allocation during json.Unmarshal.
	hugeString := strings.Repeat("A", 1_000_000)
	response := `{"primary_intent":"` + hugeString + `","semantic_type":"action","action_type":"modify","domain":"general","confidence":0.5}`

	trans := &LLMTransducer{}
	result, err := trans.parseResponse(response)
	if err != nil {
		t.Logf("1MB string rejected: %v", err)
	} else if result != nil {
		t.Logf("1MB string accepted: primary_intent length = %d", len(result.PrimaryIntent))
	}
}

func TestBreak_ParseResponse_DuplicateKeys(t *testing.T) {
	// Attack: duplicate JSON keys. Go's json.Unmarshal uses the LAST value.
	// If an attacker can inject duplicate keys, the last one wins.
	trans := &LLMTransducer{}
	response := `{"primary_intent":"safe","primary_intent":"malicious","semantic_type":"a","action_type":"b","domain":"c","confidence":0.5}`

	result, err := trans.parseResponse(response)
	if err != nil {
		t.Logf("duplicate keys rejected: %v", err)
		return
	}
	if result.PrimaryIntent == "malicious" {
		t.Log("CONFIRMED: duplicate keys allow last-write-wins override")
	} else {
		t.Logf("duplicate keys: primary_intent=%q", result.PrimaryIntent)
	}
}

func TestBreak_ParseResponse_HugeArray_UserConstraints(t *testing.T) {
	// Attack: Understanding with 100K user constraints.
	// Tests memory allocation for slice fields during deserialization.
	var constraints strings.Builder
	constraints.WriteString("[")
	for i := 0; i < 100_000; i++ {
		if i > 0 {
			constraints.WriteString(",")
		}
		constraints.WriteString(`"constraint_`)
		constraints.WriteString(strings.Repeat("x", 100))
		constraints.WriteString(`"`)
	}
	constraints.WriteString("]")

	response := `{"primary_intent":"fix","semantic_type":"a","action_type":"b","domain":"c","confidence":0.5,"user_constraints":` + constraints.String() + `}`

	trans := &LLMTransducer{}
	start := time.Now()
	result, err := trans.parseResponse(response)
	elapsed := time.Since(start)

	t.Logf("100K constraints: err=%v, took %v", err, elapsed)
	if result != nil {
		t.Logf("parsed %d constraints", len(result.UserConstraints))
	}
}

func TestBreak_ParseResponse_MultipleValidJSON(t *testing.T) {
	// Attack: response with multiple valid JSON objects.
	// extractJSON returns the LAST valid one. But what if the attacker
	// puts benign JSON first and malicious JSON last?
	response := `
Here's the analysis:
{"primary_intent":"explain","confidence":0.9}
But actually:
{"primary_intent":"delete","confidence":1.0,"action_type":"revert","domain":"all","semantic_type":"command"}
`
	trans := &LLMTransducer{}
	result, err := trans.parseResponse(response)
	if err != nil {
		t.Logf("multi-JSON rejected: %v", err)
		return
	}
	t.Logf("multi-JSON: picked primary_intent=%q (last-wins)", result.PrimaryIntent)
	if result.PrimaryIntent == "delete" {
		t.Log("CONFIRMED: last JSON object wins — attacker can override earlier classification")
	}
}

// =============================================================================
// getRegexCandidates / extractTarget TORTURE
// =============================================================================

func TestBreak_ExtractTarget_NoLengthLimit(t *testing.T) {
	// Attack: extractTarget has NO input length limit (unlike getRegexCandidates
	// which caps at 2000). Send 1MB input through extractTarget.
	// Go's regexp is NFA-based (no backtracking), so this shouldn't hang,
	// but it will be slow.
	input := strings.Repeat("x", 1_000_000) + " fix file auth.go please"

	start := time.Now()
	result := extractTarget(input)
	elapsed := time.Since(start)

	t.Logf("1MB extractTarget: result=%q, took %v", result, elapsed)
	if elapsed > 5*time.Second {
		t.Errorf("extractTarget took %v on 1MB input — needs input length cap", elapsed)
	}
}

func TestBreak_ExtractConstraint_NoLengthLimit(t *testing.T) {
	// Attack: same as above but for extractConstraint.
	input := strings.Repeat("x", 1_000_000) + " for golang"

	start := time.Now()
	result := extractConstraint(input)
	elapsed := time.Since(start)

	t.Logf("1MB extractConstraint: result=%q, took %v", result, elapsed)
	if elapsed > 5*time.Second {
		t.Errorf("extractConstraint took %v on 1MB input", elapsed)
	}
}

func TestBreak_GetRegexCandidates_AllSynonymsMatch(t *testing.T) {
	// Attack: craft input that contains synonyms from EVERY verb in the taxonomy.
	// This maximizes the candidates slice and scoring loop iterations.
	var sb strings.Builder
	for _, entry := range DefaultTaxonomyData {
		for _, syn := range entry.Synonyms {
			sb.WriteString(syn)
			sb.WriteString(" ")
		}
	}
	input := sb.String()
	if len(input) > maxRegexInputLen {
		input = input[:maxRegexInputLen] // Respect the cap
	}

	start := time.Now()
	candidates := getRegexCandidates(input)
	elapsed := time.Since(start)

	t.Logf("all-synonyms input: %d candidates from %d bytes, took %v",
		len(candidates), len(input), elapsed)
	if elapsed > 1*time.Second {
		t.Errorf("getRegexCandidates took %v with all synonyms matching", elapsed)
	}
}

// =============================================================================
// refineCategory NONDETERMINISM
// =============================================================================

func TestBreak_RefineCategory_Nondeterministic(t *testing.T) {
	// Attack: input that matches patterns in MULTIPLE categories.
	// CategoryPatterns is a map[string][]*regexp.Regexp. Map iteration order
	// in Go is randomized. If multiple categories match, the result depends
	// on which category is checked first — nondeterministic.
	//
	// "what would you change" matches:
	//   /query: "what" at start (^what\s+)
	//   /mutation: "change" (change|update|modify)
	input := "what would you change about this code"

	results := make(map[string]int)
	for i := 0; i < 1000; i++ {
		cat := refineCategory(input, "/default")
		results[cat]++
	}

	t.Logf("nondeterminism test (1000 runs): %v", results)
	if len(results) > 1 {
		t.Errorf("NONDETERMINISTIC: refineCategory returned %d different results for same input: %v",
			len(results), results)
	}
}

// =============================================================================
// VerbCorpus DATA RACE
// =============================================================================

func TestBreak_VerbCorpus_DataRace(t *testing.T) {
	// Attack: concurrent read (getRegexCandidates) and write (SetVerbCorpus)
	// of the global verbCorpus variable.
	// This MUST be run with -race to verify the mutex protection works.
	//
	// HydrateFromDB() at taxonomy.go:186 does: SetVerbCorpus(verbs)
	// getRegexCandidates() at transducer.go:271 does: for _, entry := range GetVerbCorpus()
	// These are now protected by verbCorpusMu.

	// Save and restore
	original := GetVerbCorpus()
	defer func() { SetVerbCorpus(original) }()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writer goroutine: continuously replace VerbCorpus
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				newCorpus := make([]VerbEntry, len(DefaultTaxonomyData))
				for i, def := range DefaultTaxonomyData {
					newCorpus[i] = VerbEntry{
						Verb:     def.Verb,
						Category: def.Category,
						Synonyms: def.Synonyms,
						Priority: def.Priority,
					}
				}
				SetVerbCorpus(newCorpus) // NOW SYNCHRONIZED
			}
		}
	}()

	// Reader goroutines: continuously call getRegexCandidates
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = getRegexCandidates("fix the login bug") // NOW SYNCHRONIZED
				}
			}
		}()
	}

	// Run for 500ms
	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()

	t.Log("VerbCorpus data race test completed — should pass with -race now")
}

// =============================================================================
// truncateForLog TORTURE
// =============================================================================

func TestBreak_TruncateForLog_UTF8Split(t *testing.T) {
	// Attack: truncateForLog uses len(s) which is byte count.
	// Truncating at a byte boundary can split multi-byte UTF-8 characters.
	input := strings.Repeat("a", 99) + "😀" // 99 + 4 = 103 bytes

	result := truncateForLog(input, 100)
	if !utf8.ValidString(result) {
		t.Errorf("CORRUPTION: truncateForLog produced invalid UTF-8")
		t.Logf("last bytes: %x", []byte(result[len(result)-5:]))
	}
}

// =============================================================================
// ClassifyInput TOKEN EXPLOSION
// =============================================================================

func TestBreak_ClassifyInput_10K_Tokens(t *testing.T) {
	// Attack: 10,000 unique words. Each becomes a context_token fact.
	// With singular forms, that's ~20K facts added to the Mangle engine.
	// Then the engine must recompute all derived facts.
	//
	// This tests whether ClassifyInput has any token count limits.
	var words []string
	for i := 0; i < 10_000; i++ {
		words = append(words, strings.Repeat("w", 5)+"_"+string(rune('a'+i%26))+string(rune('0'+i%10)))
	}
	input := "fix " + strings.Join(words, " ")

	// We need a TaxonomyEngine for this test
	engine, err := NewTaxonomyEngine()
	if err != nil {
		t.Skipf("cannot create taxonomy engine: %v", err)
	}

	candidates := getRegexCandidates(input)

	start := time.Now()
	verb, conf, classErr := engine.ClassifyInput(input, candidates)
	elapsed := time.Since(start)

	t.Logf("10K tokens: verb=%q, conf=%.2f, err=%v, took %v", verb, conf, classErr, elapsed)
	if elapsed > 30*time.Second {
		t.Errorf("ClassifyInput took %v with 10K tokens — needs token count limit", elapsed)
	}
}

func TestBreak_ClassifyInput_RepeatedCalls(t *testing.T) {
	// Attack: ClassifyInput calls engine.Reset() on every call.
	// Call it rapidly to test for resource leaks.
	// Reduced count under -race due to instrumentation overhead
	// (each call rebuilds the Mangle program, ~2s each with -race).
	engine, err := NewTaxonomyEngine()
	if err != nil {
		t.Skipf("cannot create taxonomy engine: %v", err)
	}

	count := 50
	if testing.Short() {
		count = 5
	}
	// Detect race detector by checking if a single call is slow
	start := time.Now()
	candidates := getRegexCandidates("fix the bug")
	_, _, _ = engine.ClassifyInput("fix the bug", candidates)
	if time.Since(start) > 3*time.Second {
		// Race detector overhead detected, reduce iterations
		count = 10
	}

	for i := 0; i < count; i++ {
		candidates := getRegexCandidates("fix the bug")
		_, _, _ = engine.ClassifyInput("fix the bug", candidates)
	}
	t.Logf("%d rapid ClassifyInput calls completed without crash", count)
}

// =============================================================================
// Understanding Validate EDGE CASES
// =============================================================================

func TestBreak_Understanding_NaN_Confidence(t *testing.T) {
	// Attack: Understanding with NaN confidence.
	// Validate() checks `u.Confidence < 0 || u.Confidence > 1`
	// But NaN comparisons are ALWAYS false in IEEE 754.
	// So NaN < 0 = false, NaN > 1 = false → validation PASSES.
	u := &Understanding{
		PrimaryIntent: "fix",
		SemanticType:  "action",
		ActionType:    "modify",
		Domain:        "general",
		Confidence:    math.NaN(),
	}

	err := u.Validate()
	if err == nil {
		t.Error("DANGEROUS: NaN confidence passed validation — IEEE 754 comparison bypass")
		t.Log("NaN < 0 =", math.NaN() < 0, "; NaN > 1 =", math.NaN() > 1)
	} else {
		t.Logf("NaN correctly rejected: %v", err)
	}
}

func TestBreak_Understanding_NegativeZero_Confidence(t *testing.T) {
	// Attack: -0.0 confidence. IEEE 754 negative zero.
	u := &Understanding{
		PrimaryIntent: "fix",
		SemanticType:  "action",
		ActionType:    "modify",
		Domain:        "general",
		Confidence:    math.Copysign(0, -1), // -0.0
	}

	err := u.Validate()
	if err != nil {
		t.Errorf("unexpected: -0.0 rejected: %v", err)
	} else {
		t.Logf("-0.0 accepted (0 == -0 in Go: %v)", 0.0 == math.Copysign(0, -1))
	}
}

func TestBreak_Understanding_Inf_Confidence(t *testing.T) {
	// Attack: +Inf confidence passes through json.Unmarshal (if set programmatically).
	u := &Understanding{
		PrimaryIntent: "fix",
		SemanticType:  "action",
		ActionType:    "modify",
		Domain:        "general",
		Confidence:    math.Inf(1),
	}

	err := u.Validate()
	if err == nil {
		t.Error("DANGEROUS: +Inf confidence passed validation")
	} else {
		t.Logf("+Inf correctly rejected: %v", err)
	}
}

// =============================================================================
// MEMORY PRESSURE
// =============================================================================

func TestBreak_ParseResponse_MemoryPressure(t *testing.T) {
	// Attack: call parseResponse 1000 times with different inputs.
	// Check that memory doesn't grow unboundedly.
	trans := &LLMTransducer{}

	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	for i := 0; i < 1000; i++ {
		resp := `{"primary_intent":"fix","semantic_type":"action","action_type":"modify","domain":"general","confidence":0.5}`
		_, _ = trans.parseResponse(resp)
	}

	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	// HeapInuse shows currently allocated heap memory
	leakMB := float64(int64(memAfter.HeapInuse)-int64(memBefore.HeapInuse)) / 1024 / 1024
	t.Logf("1000 parseResponse calls: heap delta = %.2f MB", leakMB)
	if leakMB > 50 {
		t.Errorf("possible memory leak: %.2f MB after 1000 calls", leakMB)
	}
}

func TestBreak_ExtractJSON_MemoryPressure(t *testing.T) {
	// Attack: call extractJSON 100 times on 100KB inputs.
	// Total input processed: 10MB.
	input := strings.Repeat("text ", 20000) + `{"key":"value"}` // ~100KB

	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	for i := 0; i < 100; i++ {
		_ = extractJSON(input)
	}

	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	leakMB := float64(int64(memAfter.HeapInuse)-int64(memBefore.HeapInuse)) / 1024 / 1024
	t.Logf("100x 100KB extractJSON: heap delta = %.2f MB", leakMB)
}
