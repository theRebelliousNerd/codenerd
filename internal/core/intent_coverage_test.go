package core

import (
	"testing"
)

// --- isSupportedVerb ---

func TestIsSupportedVerb_WhenValid_ShouldReturnTrue(t *testing.T) {
	verbs := []string{"/fix", "/debug", "/refactor", "/test", "/review", "/security", "/research", "/explain", "/create"}
	for _, v := range verbs {
		if !isSupportedVerb(v) {
			t.Errorf("isSupportedVerb(%q) = false, want true", v)
		}
	}
}

func TestIsSupportedVerb_WhenInvalid_ShouldReturnFalse(t *testing.T) {
	if isSupportedVerb("/unknown") {
		t.Error("expected false for /unknown")
	}
	if isSupportedVerb("fix") { // no slash
		t.Error("expected false for 'fix' without slash")
	}
}

// --- inferCategoryFromVerb ---

func TestInferCategoryFromVerb_WhenMutation_ShouldReturnMutation(t *testing.T) {
	mutations := []string{"/fix", "/debug", "/refactor", "/test", "/create"}
	for _, v := range mutations {
		got := inferCategoryFromVerb(v)
		if got != "/mutation" {
			t.Errorf("inferCategoryFromVerb(%q) = %q, want /mutation", v, got)
		}
	}
}

func TestInferCategoryFromVerb_WhenQuery_ShouldReturnQuery(t *testing.T) {
	queries := []string{"/review", "/security", "/research", "/explain"}
	for _, v := range queries {
		got := inferCategoryFromVerb(v)
		if got != "/query" {
			t.Errorf("inferCategoryFromVerb(%q) = %q, want /query", v, got)
		}
	}
}

func TestInferCategoryFromVerb_WhenUnknown_ShouldDefaultToQuery(t *testing.T) {
	got := inferCategoryFromVerb("/unknown")
	if got != "/query" {
		t.Errorf("inferCategoryFromVerb(/unknown) = %q, want /query", got)
	}
}

// --- inferTargetFromText ---

func TestInferTargetFromText_WhenGoFile_ShouldReturnPath(t *testing.T) {
	got := inferTargetFromText("look at kernel.go for issues")
	if got != "kernel.go" {
		t.Errorf("got %q, want kernel.go", got)
	}
}

func TestInferTargetFromText_WhenPythonFile_ShouldReturnPath(t *testing.T) {
	got := inferTargetFromText("check main.py")
	if got != "main.py" {
		t.Errorf("got %q, want main.py", got)
	}
}

func TestInferTargetFromText_WhenNoFile_ShouldReturnEmpty(t *testing.T) {
	got := inferTargetFromText("fix the bug")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestInferTargetFromText_WhenMangleFile_ShouldReturnPath(t *testing.T) {
	got := inferTargetFromText("update policy.mg rules")
	if got != "policy.mg" {
		t.Errorf("got %q, want policy.mg", got)
	}
}

// --- splitFirstToken ---

func TestSplitFirstToken_WhenMultipleWords_ShouldSplitCorrectly(t *testing.T) {
	first, rest := splitFirstToken("hello world foo")
	if first != "hello" {
		t.Errorf("first = %q, want 'hello'", first)
	}
	if rest != "world foo" {
		t.Errorf("rest = %q, want 'world foo'", rest)
	}
}

func TestSplitFirstToken_WhenSingleWord_ShouldReturnWordAndEmpty(t *testing.T) {
	first, rest := splitFirstToken("hello")
	if first != "hello" || rest != "" {
		t.Errorf("splitFirstToken('hello') = (%q, %q)", first, rest)
	}
}

func TestSplitFirstToken_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	first, rest := splitFirstToken("")
	if first != "" || rest != "" {
		t.Errorf("splitFirstToken('') = (%q, %q)", first, rest)
	}
}

// --- InferIntentFromTask additional cases ---

func TestInferIntentFromTask_WhenAllVerbs_ShouldMapCorrectly(t *testing.T) {
	tests := []struct {
		input    string
		wantVerb string
	}{
		{"explain how it works", "/explain"},
		{"describe the architecture", "/explain"},
		{"summarize the changes", "/explain"},
		{"debug the crash", "/debug"},
		{"investigate the issue", "/debug"},
		{"diagnose timeout", "/debug"},
		{"fix compilation error", "/fix"},
		{"repair broken tests", "/fix"},
		{"resolve conflict", "/fix"},
		{"refactor the module", "/refactor"},
		{"rewrite this function", "/refactor"},
		{"test all endpoints", "/test"},
		{"review the PR", "/review"},
		{"audit the code", "/review"},
		{"security scan it", "/security"},
		{"harden the API", "/security"},
		{"research best practices", "/research"},
		{"find similar examples", "/research"},
		{"create a new service", "/create"},
		{"build a tool", "/create"},
		{"implement the feature", "/create"},
		{"add logging", "/create"},
		{"random words", "/explain"}, // default
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			intent := InferIntentFromTask(tt.input)
			if intent.Verb != tt.wantVerb {
				t.Errorf("InferIntentFromTask(%q).Verb = %q, want %q", tt.input, intent.Verb, tt.wantVerb)
			}
		})
	}
}

func TestIntentDefaults(t *testing.T) {
	files := DefaultIntentSchemaFiles()
	if len(files) == 0 {
		t.Error("expected non-empty default intent schema files")
	}
	for _, f := range files {
		if f == "" {
			t.Error("expected non-empty schema file path")
		}
	}

	preds := defaultIntentFactPredicates()
	if len(preds) == 0 {
		t.Error("expected non-empty default intent fact predicates")
	}
	if _, ok := preds["intent_definition"]; !ok {
		t.Error("expected 'intent_definition' in default intent fact predicates")
	}
}
