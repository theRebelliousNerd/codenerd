package perception

import (
	"regexp"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
)

// =============================================================================
// EXTRACT TARGET TESTS
// =============================================================================

func TestExtractTarget_WhenFilePath_ShouldExtract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "file_keyword_with_path",
			input: "review file internal/core/kernel.go",
			want:  "internal/core/kernel.go",
		},
		{
			name:  "in_keyword_with_path",
			input: "fix the bug in internal/core/kernel.go",
			want:  "internal/core/kernel.go",
		},
		{
			name:  "quoted_path",
			input: `review "main.go"`,
			want:  "main.go",
		},
		{
			name:  "backtick_path",
			input: "review `internal/core/kernel.go`",
			want:  "internal/core/kernel.go",
		},
		{
			name:  "slash_path",
			input: "look at internal/core/types.go please",
			want:  "internal/core/types.go",
		},
		{
			name:  "function_keyword",
			input: "explain function MyHandler",
			want:  "MyHandler",
		},
		{
			name:  "method_keyword",
			input: "explain method Execute",
			want:  "Execute",
		},
		{
			name:  "class_keyword",
			input: "review class MyService",
			want:  "MyService",
		},
		{
			name:  "the_function_pattern",
			input: "explain the doWork function",
			want:  "doWork",
		},
		{
			name:  "no_match",
			input: "hello world",
			want:  "none",
		},
		{
			name:  "empty_input",
			input: "",
			want:  "none",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractTarget(tc.input)
			if got != tc.want {
				t.Errorf("extractTarget(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// =============================================================================
// EXTRACT CONSTRAINT TESTS
// =============================================================================

func TestExtractConstraint_WhenLanguagePresent_ShouldExtract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "for_go",
			input: "create a handler for go",
			want:  "go",
		},
		{
			name:  "using_python",
			input: "implement using python",
			want:  "python",
		},
		{
			name:  "with_typescript",
			input: "create a service with typescript",
			want:  "typescript",
		},
		{
			name:  "in_rust",
			input: "build a parser in rust",
			want:  "rust",
		},
		{
			name:  "but_exclusion",
			input: "review but skip tests",
			want:  "skip tests",
		},
		{
			name:  "without_exclusion",
			input: "review without vendor files",
			want:  "vendor files",
		},
		{
			name:  "only_constraint",
			input: "check only public functions",
			want:  "public functions",
		},
		{
			name:  "no_match",
			input: "review my code",
			want:  "none",
		},
		{
			name:  "empty_input",
			input: "",
			want:  "none",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractConstraint(tc.input)
			if got != tc.want {
				t.Errorf("extractConstraint(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// =============================================================================
// REFINE CATEGORY TESTS
// =============================================================================

func TestRefineCategory_WhenMutationPattern_ShouldReturnMutation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		input           string
		defaultCategory string
		want            string
	}{
		{
			name:            "imperative_make",
			input:           "make this function faster",
			defaultCategory: "/query",
			want:            "/mutation",
		},
		{
			name:            "imperative_fix",
			input:           "fix the bug in main.go",
			defaultCategory: "/query",
			want:            "/mutation",
		},
		{
			name:            "imperative_add",
			input:           "add a new endpoint",
			defaultCategory: "/query",
			want:            "/mutation",
		},
		{
			name:            "i_want_to",
			input:           "I want to refactor this",
			defaultCategory: "/query",
			want:            "/mutation",
		},
		{
			name:            "question_what",
			input:           "what does this function do?",
			defaultCategory: "/mutation",
			want:            "/query",
		},
		{
			name:            "question_how",
			input:           "how does the cache work?",
			defaultCategory: "/mutation",
			want:            "/query",
		},
		{
			name:            "question_mark",
			input:           "can you explain this code?",
			defaultCategory: "/mutation",
			want:            "/query",
		},
		{
			name:            "instruction_always",
			input:           "always use tabs for indentation",
			defaultCategory: "/query",
			want:            "/instruction",
		},
		{
			name:            "instruction_never",
			input:           "never use global variables",
			defaultCategory: "/query",
			want:            "/instruction",
		},
		{
			name:            "no_match_preserves_default",
			input:           "hello there",
			defaultCategory: "/query",
			want:            "/query",
		},
		{
			name:            "empty_input_preserves_default",
			input:           "",
			defaultCategory: "/mutation",
			want:            "/mutation",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := refineCategory(tc.input, tc.defaultCategory)
			if got != tc.want {
				t.Errorf("refineCategory(%q, %q) = %q, want %q", tc.input, tc.defaultCategory, got, tc.want)
			}
		})
	}
}

// =============================================================================
// CONTAINS ANY TESTS
// =============================================================================

func TestContainsAny_WhenMatch_ShouldReturnTrue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		s    string
		subs []string
		want bool
	}{
		{
			name: "single_match",
			s:    "hello world",
			subs: []string{"world"},
			want: true,
		},
		{
			name: "first_of_many",
			s:    "hello world",
			subs: []string{"hello", "foo", "bar"},
			want: true,
		},
		{
			name: "last_of_many",
			s:    "hello world",
			subs: []string{"foo", "bar", "world"},
			want: true,
		},
		{
			name: "no_match",
			s:    "hello world",
			subs: []string{"foo", "bar"},
			want: false,
		},
		{
			name: "empty_subs",
			s:    "hello world",
			subs: []string{},
			want: false,
		},
		{
			name: "nil_subs",
			s:    "hello world",
			subs: nil,
			want: false,
		},
		{
			name: "empty_string",
			s:    "",
			subs: []string{"hello"},
			want: false,
		},
		{
			name: "empty_sub_matches_any_string",
			s:    "hello",
			subs: []string{""},
			want: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := containsAny(tc.s, tc.subs)
			if got != tc.want {
				t.Errorf("containsAny(%q, %v) = %v, want %v", tc.s, tc.subs, got, tc.want)
			}
		})
	}
}

// =============================================================================
// GET SHARD TYPE FOR VERB TESTS
// =============================================================================

func TestGetShardTypeForVerb_WhenKnownVerb_ShouldReturnShardType(t *testing.T) {

	// Set up a known corpus for testing
	original := GetVerbCorpus()
	defer SetVerbCorpus(original) // Restore after test

	SetVerbCorpus([]VerbEntry{
		{Verb: "/review", ShardType: "/reviewer", Category: "/query", Priority: 100},
		{Verb: "/fix", ShardType: "/coder", Category: "/mutation", Priority: 90},
		{Verb: "/explain", ShardType: "", Category: "/query", Priority: 80},
	})

	cases := []struct {
		name string
		verb string
		want string
	}{
		{"known_reviewer", "/review", "/reviewer"},
		{"known_coder", "/fix", "/coder"},
		{"known_empty_shard", "/explain", ""},
		{"unknown_verb", "/nonexistent", ""},
		{"empty_verb", "", ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := GetShardTypeForVerb(tc.verb)
			if got != tc.want {
				t.Errorf("GetShardTypeForVerb(%q) = %q, want %q", tc.verb, got, tc.want)
			}
		})
	}
}

// =============================================================================
// VERB CORPUS CONCURRENCY TESTS
// =============================================================================

func TestVerbCorpus_ConcurrentAccess_ShouldNotRace(t *testing.T) {

	original := GetVerbCorpus()
	defer SetVerbCorpus(original)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			SetVerbCorpus([]VerbEntry{
				{Verb: "/test", Category: "/mutation", Priority: 88},
			})
		}()
		go func() {
			defer wg.Done()
			corpus := GetVerbCorpus()
			_ = len(corpus) // use the value
		}()
	}
	wg.Wait()
}

// =============================================================================
// SAFE TRUNCATE TESTS
// =============================================================================

func TestSafeTruncate_WhenShort_ShouldReturnUnchanged(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		limit int
		want  string
	}{
		{"short_string", "hello", 10, "hello"},
		{"exact_length", "hello", 5, "hello"},
		{"truncate", "hello world", 5, "hello"},
		{"empty", "", 5, ""},
		{"zero_limit", "hello", 0, ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := safeTruncate(tc.input, tc.limit)
			if got != tc.want {
				t.Errorf("safeTruncate(%q, %d) = %q, want %q", tc.input, tc.limit, got, tc.want)
			}
		})
	}
}

func TestSafeTruncate_WhenMultibyte_ShouldNotSplitRune(t *testing.T) {
	t.Parallel()

	// "世" is 3 bytes (0xE4 0xB8 0x96)
	input := "A世B"
	// A=1byte, 世=3bytes, B=1byte. Total 5 bytes.
	// Truncating at limit=2 should not split 世 (bytes 1-3)
	got := safeTruncate(input, 2)
	if strings.ContainsRune(got, '\uFFFD') {
		t.Errorf("safeTruncate produced invalid UTF-8: %q", got)
	}
	// Should return just "A" since cutting into 世 would split a rune
	if got != "A" {
		t.Errorf("safeTruncate(%q, 2) = %q, want %q", input, got, "A")
	}
}

// =============================================================================
// TRUNCATE FOR LOG TESTS
// =============================================================================

func TestTruncateForLog_WhenShort_ShouldReturnUnchanged(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"needs_truncation", "hello world", 5, "hello..."},
		{"empty", "", 5, ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncateForLog(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("truncateForLog(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}

// =============================================================================
// REQUIRES JSON OUTPUT TESTS
// =============================================================================

func TestRequiresJSONOutput_WhenMarkerPresent_ShouldReturnTrue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		system    string
		user      string
		wantMatch bool
	}{
		{
			name:      "mangle_synth_v1_in_system",
			system:    "This prompt uses mangle_synth_v1 format",
			user:      "do something",
			wantMatch: true,
		},
		{
			name:      "MangleSynth_in_user",
			system:    "system prompt",
			user:      "output MangleSynth JSON",
			wantMatch: true,
		},
		{
			name:      "application_json_in_system",
			system:    "Content-Type: application/json",
			user:      "do something",
			wantMatch: true,
		},
		{
			name:      "responseJsonSchema",
			system:    "responseJsonSchema required",
			user:      "",
			wantMatch: true,
		},
		{
			name:      "responseMimeType",
			system:    "",
			user:      "responseMimeType: application/json",
			wantMatch: true,
		},
		{
			name:      "no_markers",
			system:    "normal system prompt",
			user:      "normal user prompt",
			wantMatch: false,
		},
		{
			name:      "both_empty",
			system:    "",
			user:      "",
			wantMatch: false,
		},
		{
			name:      "output_only_MangleSynth",
			system:    "Output ONLY a MangleSynth JSON object",
			user:      "",
			wantMatch: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := requiresJSONOutput(tc.system, tc.user)
			if got != tc.wantMatch {
				t.Errorf("requiresJSONOutput(%q, %q) = %v, want %v", tc.system, tc.user, got, tc.wantMatch)
			}
		})
	}
}

// =============================================================================
// INTENT TO FACT TESTS
// =============================================================================

func TestIntent_ToFact_WhenValid_ShouldProduceCorrectFact(t *testing.T) {
	t.Parallel()

	intent := Intent{
		Category:   "/mutation",
		Verb:       "/fix",
		Target:     "internal/core/kernel.go",
		Constraint: "go",
		Confidence: 0.95,
	}

	fact := intent.ToFact()

	if fact.Predicate != "user_intent" {
		t.Errorf("Predicate = %q, want %q", fact.Predicate, "user_intent")
	}
	if len(fact.Args) != 5 {
		t.Fatalf("Args length = %d, want 5", len(fact.Args))
	}
	// Args[0] is MangleAtom("/current_intent")
	if arg0, ok := fact.Args[0].(core.MangleAtom); !ok || string(arg0) != "/current_intent" {
		t.Errorf("Args[0] = %v (type %T), want MangleAtom(/current_intent)", fact.Args[0], fact.Args[0])
	}
	// Args[1] is MangleAtom("/mutation")
	if arg1, ok := fact.Args[1].(core.MangleAtom); !ok || string(arg1) != "/mutation" {
		t.Errorf("Args[1] = %v (type %T), want MangleAtom(/mutation)", fact.Args[1], fact.Args[1])
	}
	// Args[2] is MangleAtom("/fix")
	if arg2, ok := fact.Args[2].(core.MangleAtom); !ok || string(arg2) != "/fix" {
		t.Errorf("Args[2] = %v (type %T), want MangleAtom(/fix)", fact.Args[2], fact.Args[2])
	}
	// Args[3] and Args[4] are sanitized strings
	if arg3, ok := fact.Args[3].(string); !ok || arg3 != "internal/core/kernel.go" {
		t.Errorf("Args[3] = %v, want %q", fact.Args[3], "internal/core/kernel.go")
	}
	if arg4, ok := fact.Args[4].(string); !ok || arg4 != "go" {
		t.Errorf("Args[4] = %v, want %q", fact.Args[4], "go")
	}
}

func TestIntent_ToFact_WhenControlCharsInTarget_ShouldSanitize(t *testing.T) {
	t.Parallel()

	intent := Intent{
		Category:   "/mutation",
		Verb:       "/fix",
		Target:     "file\x00with\x01nulls",
		Constraint: "none",
	}

	fact := intent.ToFact()

	target, ok := fact.Args[3].(string)
	if !ok {
		t.Fatal("Args[3] should be a string")
	}
	if strings.Contains(target, "\x00") {
		t.Error("Target should not contain null bytes after sanitization")
	}
	if strings.Contains(target, "\x01") {
		t.Error("Target should not contain control chars after sanitization")
	}
}

// =============================================================================
// FOCUS RESOLUTION TO FACT TESTS
// =============================================================================

func TestFocusResolution_ToFact_WhenValid_ShouldProduceCorrectFact(t *testing.T) {
	t.Parallel()

	focus := FocusResolution{
		RawReference:      "kernel.go",
		ResolvedPath:      "internal/core/kernel.go",
		SymbolName:        "Execute",
		ConfidencePercent: 95,
	}

	fact := focus.ToFact()

	if fact.Predicate != "focus_resolution" {
		t.Errorf("Predicate = %q, want %q", fact.Predicate, "focus_resolution")
	}
	if len(fact.Args) != 4 {
		t.Fatalf("Args length = %d, want 4", len(fact.Args))
	}
	if fact.Args[0] != "kernel.go" {
		t.Errorf("Args[0] = %v, want %q", fact.Args[0], "kernel.go")
	}
	if fact.Args[1] != "internal/core/kernel.go" {
		t.Errorf("Args[1] = %v, want %q", fact.Args[1], "internal/core/kernel.go")
	}
	if fact.Args[2] != "Execute" {
		t.Errorf("Args[2] = %v, want %q", fact.Args[2], "Execute")
	}
	if fact.Args[3] != int64(95) {
		t.Errorf("Args[3] = %v (type %T), want int64(95)", fact.Args[3], fact.Args[3])
	}
}

func TestFocusResolution_ToFact_WhenZeroConfidence_ShouldPreserve(t *testing.T) {
	t.Parallel()

	focus := FocusResolution{
		RawReference:      "something",
		ResolvedPath:      "",
		SymbolName:        "",
		ConfidencePercent: 0,
	}

	fact := focus.ToFact()
	if fact.Args[3] != int64(0) {
		t.Errorf("Args[3] = %v, want int64(0)", fact.Args[3])
	}
}


// =============================================================================
// GET REGEX CANDIDATES TESTS
// =============================================================================

func TestGetRegexCandidates_WhenEmptyCorpus_ShouldReturnEmpty(t *testing.T) {

	original := GetVerbCorpus()
	defer SetVerbCorpus(original)

	SetVerbCorpus(nil)

	candidates := getRegexCandidates("fix the bug")
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates with nil corpus, got %d", len(candidates))
	}
}

func TestGetRegexCandidates_WhenPatternMatches_ShouldReturnEntry(t *testing.T) {

	original := GetVerbCorpus()
	defer SetVerbCorpus(original)

	SetVerbCorpus([]VerbEntry{
		{
			Verb:     "/fix",
			Category: "/mutation",
			Synonyms: []string{"fix", "repair"},
			Patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)fix.*bug`)},
			Priority: 90,
		},
		{
			Verb:     "/review",
			Category: "/query",
			Synonyms: []string{"review"},
			Patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)review.*code`)},
			Priority: 100,
		},
	})

	candidates := getRegexCandidates("fix this bug please")
	found := false
	for _, c := range candidates {
		if c.Verb == "/fix" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected /fix to be a candidate for 'fix this bug please'")
	}
}

func TestGetRegexCandidates_WhenSynonymMatches_ShouldReturnEntry(t *testing.T) {

	original := GetVerbCorpus()
	defer SetVerbCorpus(original)

	SetVerbCorpus([]VerbEntry{
		{
			Verb:     "/fix",
			Category: "/mutation",
			Synonyms: []string{"fix", "repair", "patch"},
			Patterns: nil,
			Priority: 90,
		},
	})

	candidates := getRegexCandidates("please repair this code")
	found := false
	for _, c := range candidates {
		if c.Verb == "/fix" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected /fix to be a candidate when synonym 'repair' matches")
	}
}

func TestGetRegexCandidates_WhenNoDuplicate_ShouldDedup(t *testing.T) {

	original := GetVerbCorpus()
	defer SetVerbCorpus(original)

	SetVerbCorpus([]VerbEntry{
		{
			Verb:     "/fix",
			Category: "/mutation",
			Synonyms: []string{"fix", "repair"},
			Patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)fix`)},
			Priority: 90,
		},
	})

	// Both "fix" synonym and pattern should match, but only one entry returned
	candidates := getRegexCandidates("fix the issue")
	count := 0
	for _, c := range candidates {
		if c.Verb == "/fix" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected 1 candidate for /fix, got %d (dedup failure)", count)
	}
}

// =============================================================================
// CATEGORY PATTERNS VALIDATION TESTS
// =============================================================================

func TestCategoryPatterns_ShouldExist(t *testing.T) {
	t.Parallel()

	if _, ok := CategoryPatterns["/mutation"]; !ok {
		t.Error("expected /mutation category patterns to exist")
	}
	if _, ok := CategoryPatterns["/query"]; !ok {
		t.Error("expected /query category patterns to exist")
	}
	if _, ok := CategoryPatterns["/instruction"]; !ok {
		t.Error("expected /instruction category patterns to exist")
	}

	// Verify each category has at least one pattern
	for cat, patterns := range CategoryPatterns {
		if len(patterns) == 0 {
			t.Errorf("category %q has no patterns", cat)
		}
	}
}

// =============================================================================
// TARGET PATTERNS VALIDATION TESTS
// =============================================================================

func TestTargetPatterns_ShouldNotBeEmpty(t *testing.T) {
	t.Parallel()

	if len(TargetPatterns) == 0 {
		t.Error("TargetPatterns should not be empty")
	}
}

// =============================================================================
// CONSTRAINT PATTERNS VALIDATION TESTS
// =============================================================================

func TestConstraintPatterns_ShouldNotBeEmpty(t *testing.T) {
	t.Parallel()

	if len(ConstraintPatterns) == 0 {
		t.Error("ConstraintPatterns should not be empty")
	}
}
