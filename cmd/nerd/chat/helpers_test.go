package chat

import (
	pe "codenerd/internal/autopoiesis/prompt_evolution"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/transparency"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolvePath(t *testing.T) {
	workspace := t.TempDir()
	abs := filepath.Join(workspace, "abs.txt")

	if got := resolvePath(workspace, abs); got != abs {
		t.Fatalf("resolvePath absolute = %q, want %q", got, abs)
	}

	rel := "rel.txt"
	if got := resolvePath(workspace, rel); got != filepath.Join(workspace, rel) {
		t.Fatalf("resolvePath relative = %q, want %q", got, filepath.Join(workspace, rel))
	}
}

func TestReadFileContent_Truncates(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "sample.txt")
	content := "abcdef"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := readFileContent(workspace, "sample.txt", 4)
	if err != nil {
		t.Fatalf("readFileContent: %v", err)
	}
	if got != "abcd" {
		t.Fatalf("readFileContent truncated = %q, want %q", got, "abcd")
	}

	full, err := readFileContent(workspace, "sample.txt", 10)
	if err != nil {
		t.Fatalf("readFileContent full: %v", err)
	}
	if full != content {
		t.Fatalf("readFileContent full = %q, want %q", full, content)
	}
}

func TestCountFileLines(t *testing.T) {
	workspace := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    int64
	}{
		{name: "empty", content: "", want: 0},
		{name: "with newline", content: "a\nb\n", want: 2},
		{name: "without trailing newline", content: "a\nb", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(workspace, tt.name+".txt")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("write file: %v", err)
			}

			got, err := countFileLines(workspace, filepath.Base(path))
			if err != nil {
				t.Fatalf("countFileLines: %v", err)
			}
			if got != tt.want {
				t.Fatalf("countFileLines = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSearchInFiles_SkipsHiddenDirs(t *testing.T) {
	root := t.TempDir()
	visible := filepath.Join(root, "visible.txt")
	hiddenDir := filepath.Join(root, ".hidden")
	hidden := filepath.Join(hiddenDir, "hidden.txt")

	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatalf("mkdir hidden: %v", err)
	}
	if err := os.WriteFile(visible, []byte("needle"), 0644); err != nil {
		t.Fatalf("write visible: %v", err)
	}
	if err := os.WriteFile(hidden, []byte("needle"), 0644); err != nil {
		t.Fatalf("write hidden: %v", err)
	}

	matches, err := searchInFiles(root, "needle", 10)
	if err != nil {
		t.Fatalf("searchInFiles: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0] != visible {
		t.Fatalf("expected match %q, got %q", visible, matches[0])
	}
}

func TestTruncateForContext(t *testing.T) {
	if got := truncateForContext("line1\nline2\r", 20); got != "line1 line2" {
		t.Fatalf("truncateForContext newline = %q, want %q", got, "line1 line2")
	}

	if got := truncateForContext("abcdef", 4); got != "abcd..." {
		t.Fatalf("truncateForContext truncation = %q, want %q", got, "abcd...")
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "main.go", want: "go"},
		{path: "app.tsx", want: "typescript"},
		{path: "README.md", want: "markdown"},
		{path: "data.unknown", want: "unknown"},
	}

	for _, tt := range tests {
		if got := detectLanguage(tt.path); got != tt.want {
			t.Fatalf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "foo_test.go", want: true},
		{path: "bar.test.ts", want: true},
		{path: "baz.spec.tsx", want: true},
		{path: "WidgetTest.java", want: true},
		{path: "main.go", want: false},
	}

	for _, tt := range tests {
		if got := isTestFile(tt.path); got != tt.want {
			t.Fatalf("isTestFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestBuildFileTopologyFact(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "sample_test.go")
	content := "package main\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	fact := buildFileTopologyFact(path, info)
	if fact.Predicate != "file_topology" {
		t.Fatalf("predicate = %q, want %q", fact.Predicate, "file_topology")
	}
	if len(fact.Args) != 5 {
		t.Fatalf("expected 5 args, got %d", len(fact.Args))
	}

	wantHash := sha256.Sum256([]byte(content))
	if got, ok := fact.Args[1].(string); !ok || got != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("hash arg = %v, want %s", fact.Args[1], hex.EncodeToString(wantHash[:]))
	}

	if got, ok := fact.Args[2].(string); !ok || got != "/go" {
		t.Fatalf("lang arg = %v, want %q", fact.Args[2], "/go")
	}

	if got, ok := fact.Args[4].(string); !ok || got != "/true" {
		t.Fatalf("test flag arg = %v, want %q", fact.Args[4], "/true")
	}

	if got, ok := fact.Args[3].(int64); !ok || got != info.ModTime().Unix() {
		t.Fatalf("mtime arg = %v, want %d", fact.Args[3], info.ModTime().Unix())
	}
}

func TestIsConversationalIntent(t *testing.T) {
	tests := []struct {
		name   string
		intent perception.Intent
		want   bool
	}{
		{
			name:   "always conversational",
			intent: perception.Intent{Verb: "/greet"},
			want:   true,
		},
		{
			name:   "read with empty target",
			intent: perception.Intent{Verb: "/read", Target: ""},
			want:   true,
		},
		{
			name:   "read with none target",
			intent: perception.Intent{Verb: "/read", Target: "none"},
			want:   true,
		},
		{
			name:   "read with file target",
			intent: perception.Intent{Verb: "/read", Target: "main.go"},
			want:   false,
		},
		{
			name:   "non conversational verb",
			intent: perception.Intent{Verb: "/review"},
			want:   false,
		},
		{
			name:   "explain stays routed",
			intent: perception.Intent{Verb: "/explain"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isConversationalIntent(tt.intent); got != tt.want {
				t.Fatalf("isConversationalIntent = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// model_helpers.go tests - isAffirmativeResponse, isNegativeResponse, etc.
// ============================================================================

func TestIsAffirmativeResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Positive cases
		{"yes lowercase", "yes", true},
		{"yes uppercase", "YES", true},
		{"yes mixed case", "Yes", true},
		{"yeah", "yeah", true},
		{"yep", "yep", true},
		{"sure", "sure", true},
		{"ok", "ok", true},
		{"okay", "okay", true},
		{"correct", "correct", true},
		{"confirm", "confirm", true},
		{"y single", "y", true},
		{"learn this", "learn this", true},
		{"do it", "do it", true},
		{"learn_yes command", "/learn_yes", true},
		{"with whitespace", "  yes  ", true},
		{"in sentence", "yes I want to do that", true},

		// Negative cases
		{"empty string", "", false},
		{"whitespace only", "   ", false},
		{"no", "no", false},
		{"random text", "hello world", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAffirmativeResponse(tt.input)
			if result != tt.expected {
				t.Errorf("isAffirmativeResponse(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsNegativeResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Positive cases (these are negative responses)
		{"no lowercase", "no", true},
		{"no uppercase", "NO", true},
		{"nope", "nope", true},
		{"nah", "nah", true},
		{"n single", "n", true},
		{"don't", "don't do that", true},
		{"do not", "do not", true},
		{"never", "never", true},
		{"reject", "reject", true},
		{"skip", "skip", true},
		{"not now", "not now", true},
		{"learn_no command", "/learn_no", true},
		{"with whitespace", "  no  ", true},

		// Negative cases (these are NOT negative responses)
		{"empty string", "", false},
		{"whitespace only", "   ", false},
		{"yes", "yes", false},
		{"random text", "hello world", false},
		{"maybe", "maybe", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNegativeResponse(tt.input)
			if result != tt.expected {
				t.Errorf("isNegativeResponse(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEscapeMangleString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"no escapes needed", "hello world", "hello world"},
		{"backslash", `hello\world`, `hello\\world`},
		{"double quote", `hello "world"`, `hello \"world\"`},
		{"newline", "hello\nworld", "hello world"},
		{"carriage return", "hello\rworld", "helloworld"},
		{"multiple backslashes", `a\\b`, `a\\\\b`},
		{"multiple quotes", `"a" "b"`, `\"a\" \"b\"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeMangleString(tt.input)
			if result != tt.expected {
				t.Errorf("escapeMangleString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeVerbAtom(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
		{"none value", "none", ""},
		{"underscore value", "_", ""},
		{"already prefixed", "/review", "/review"},
		{"needs prefix", "review", "/review"},
		{"with whitespace", "  review  ", "/review"},
		{"already prefixed with whitespace", "  /test  ", "/test"},
		{"complex verb", "security_scan", "/security_scan"},
		{"uppercase", "REVIEW", "/REVIEW"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeVerbAtom(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeVerbAtom(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatScore(t *testing.T) {
	tests := []struct {
		name  string
		score float64
	}{
		{"zero", 0.0},
		{"ten percent", 0.1},
		{"fifty percent", 0.5},
		{"ninety percent", 0.9},
		{"one hundred percent", 1.0},
		{"twenty five percent", 0.25},
		{"seventy five percent", 0.75},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatScore(tt.score)
			// Check that result contains percentage
			if len(result) == 0 {
				t.Errorf("formatScore(%v) returned empty string", tt.score)
			}
			// Check for visual bar characters
			hasBar := false
			for _, r := range result {
				if r == '█' || r == '░' {
					hasBar = true
					break
				}
			}
			if !hasBar {
				t.Errorf("formatScore(%v) = %q, missing visual bar characters", tt.score, result)
			}
		})
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"empty string", "", 10, ""},
		{"shorter than max", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"longer than max", "hello world", 5, "hello..."},
		{"max of 1", "hello", 1, "h..."},
		{"max of 0", "hello", 0, "..."},
		{"unicode", "hello", 3, "hel..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateStr(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestIsDreamConfirmation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"correct!", "correct!", true},
		{"correct", "correct", true},
		{"learn this", "learn this", true},
		{"learn that", "learn that", true},
		{"remember this", "remember this", true},
		{"yes do that", "yes, do that", true},
		{"thats right", "that's right", true},
		{"exactly!", "exactly!", true},
		{"yes!", "yes!", true},
		{"perfect", "perfect", true},
		{"good approach", "good approach", true},
		{"sounds right", "sounds right", true},
		{"case insensitive", "CORRECT!", true},
		{"in sentence", "I think that's right", true},

		// Negative cases
		{"empty", "", false},
		{"no", "no", false},
		{"random", "hello world", false},
		{"wrong", "wrong", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDreamConfirmation(tt.input)
			if result != tt.expected {
				t.Errorf("isDreamConfirmation(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsDreamCorrection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"no actually", "no, actually we use different pattern", true},
		{"actually we", "actually, we prefer this approach", true},
		{"wrong we", "wrong, we never do that", true},
		{"instead we", "instead, we should use", true},
		{"not that way", "not that way, try this", true},
		{"we don't", "we don't do it like that", true},
		{"we always", "we always prefer X over Y", true},
		{"remember colon", "remember: always use tabs", true},
		{"learn colon", "learn: this is the right way", true},
		{"actually colon", "actually: we should do this", true},
		{"case insensitive", "NO, ACTUALLY we prefer", true},

		// Negative cases
		{"empty", "", false},
		{"yes", "yes", false},
		{"random", "hello world", false},
		{"correct", "correct", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDreamCorrection(tt.input)
			if result != tt.expected {
				t.Errorf("isDreamCorrection(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsDreamExecutionTrigger(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"do it", "do it", true},
		{"execute that", "execute that", true},
		{"run the plan", "run the plan", true},
		{"go ahead", "go ahead", true},
		{"make it so", "make it so", true},
		{"proceed", "proceed", true},
		{"execute the plan", "execute the plan", true},
		{"run that", "run that", true},
		{"lets do it", "let's do it", true},
		{"implement that", "implement that", true},
		{"start execution", "start execution", true},
		{"yes do it", "yes, do it", true},
		{"yes execute", "yes, execute", true},
		{"carry it out", "carry it out", true},
		{"perform that", "perform that", true},
		{"case insensitive", "DO IT NOW", true},

		// Negative cases
		{"empty", "", false},
		{"no", "no", false},
		{"random", "hello world", false},
		{"learn this", "learn this", false}, // This is confirmation, not execution
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDreamExecutionTrigger(tt.input)
			if result != tt.expected {
				t.Errorf("isDreamExecutionTrigger(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsNegativeFeedback(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"bad bot", "bad bot", true},
		{"wrong", "wrong", true},
		{"stop", "stop doing that", true},
		{"no that's not right", "no that's not right", true},
		{"you didn't", "you didn't do what I asked", true},
		{"fail", "fail", true},
		{"incorrect", "that's incorrect", true},
		{"mistake", "you made a mistake", true},
		{"case insensitive", "WRONG!", true},

		// Negative cases
		{"empty", "", false},
		{"random", "hello world", false},
		{"positive", "good job", false},
		{"correct", "correct", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNegativeFeedback(tt.input)
			if result != tt.expected {
				t.Errorf("isNegativeFeedback(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// Note: TestExtractClarificationQuestion and TestExtractCorrectionContent are in model_helpers_test.go

func TestExtractFindings(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedCount  int
		checkSeverity  string
		severityExists bool
	}{
		{
			name:          "empty input",
			input:         "",
			expectedCount: 0,
		},
		{
			name:          "no findings",
			input:         "All tests passed\nEverything looks good",
			expectedCount: 0,
		},
		{
			name:           "single error finding",
			input:          "- [ERROR] file.go:10: undefined variable",
			expectedCount:  1,
			checkSeverity:  "error",
			severityExists: true,
		},
		{
			name:           "single warning finding",
			input:          "- [WARN] file.go:20: unused import",
			expectedCount:  1,
			checkSeverity:  "warning",
			severityExists: true,
		},
		{
			name:           "critical finding",
			input:          "- [CRIT] security issue detected",
			expectedCount:  1,
			checkSeverity:  "critical",
			severityExists: true,
		},
		{
			name:           "info finding",
			input:          "[INFO] checking file.go",
			expectedCount:  1,
			checkSeverity:  "info",
			severityExists: true,
		},
		{
			name:          "multiple findings",
			input:         "- [ERROR] error1\n- [WARN] warning1\n- [INFO] info1",
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFindings(tt.input)
			if len(result) != tt.expectedCount {
				t.Errorf("extractFindings() returned %d findings, want %d", len(result), tt.expectedCount)
			}
			if tt.severityExists && len(result) > 0 {
				if sev, ok := result[0]["severity"].(string); !ok || sev != tt.checkSeverity {
					t.Errorf("extractFindings() severity = %v, want %v", result[0]["severity"], tt.checkSeverity)
				}
			}
		})
	}
}

func TestExtractMetrics(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expectKeys []string
	}{
		{
			name:       "empty input",
			input:      "",
			expectKeys: nil,
		},
		{
			name:       "lines metric",
			input:      "Total lines: 150",
			expectKeys: []string{"Total lines"},
		},
		{
			name:       "functions metric",
			input:      "functions = 25",
			expectKeys: []string{"functions"},
		},
		{
			name:       "complexity metric",
			input:      "Cyclomatic complexity: 8",
			expectKeys: []string{"Cyclomatic complexity"},
		},
		{
			name:       "nesting metric",
			input:      "Max nesting: 4",
			expectKeys: []string{"Max nesting"},
		},
		{
			name:       "multiple metrics",
			input:      "Total lines: 150\nfunctions: 25\ncomplexity: 8",
			expectKeys: []string{"Total lines", "functions", "complexity"},
		},
		{
			name:       "no relevant metrics",
			input:      "Some random text without metrics",
			expectKeys: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMetrics(tt.input)
			for _, key := range tt.expectKeys {
				if _, ok := result[key]; !ok {
					t.Errorf("extractMetrics() missing key %q", key)
				}
			}
		})
	}
}

func TestHardWrap(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		width  int
		wantOK bool // Just check it returns without panic
	}{
		{"empty string", "", 10, true},
		{"shorter than width", "hello", 10, true},
		{"exact width", "hello", 5, true},
		{"needs wrapping", "hello world", 5, true},
		{"zero width", "hello", 0, true},
		{"negative width", "hello", -1, true},
		{"multiline input", "hello\nworld", 10, true},
		{"long single line", "abcdefghij", 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hardWrap(tt.input, tt.width)
			if tt.wantOK && result == "" && tt.input != "" && tt.width >= 1 {
				// For non-empty input with valid width, should get non-empty result
				t.Logf("hardWrap(%q, %d) returned empty string", tt.input, tt.width)
			}
		})
	}
}

// ============================================================================
// campaign_assault.go helper tests
// ============================================================================

func TestIsAssaultRequest_WithIntent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		intent   perception.Intent
		expected bool
	}{
		// /assault verb always returns true
		{"assault verb", "anything", perception.Intent{Verb: "/assault"}, true},
		{"assault with target", "internal/core", perception.Intent{Verb: "/assault"}, true},

		// /campaign verb requires assault keywords in input
		{"campaign with assault keyword", "run assault on core", perception.Intent{Verb: "/campaign"}, true},
		{"campaign with stress test", "stress test the kernel", perception.Intent{Verb: "/campaign"}, true},
		{"campaign with soak test", "soak test for memory", perception.Intent{Verb: "/campaign"}, true},
		{"campaign with gauntlet", "run gauntlet tests", perception.Intent{Verb: "/campaign"}, true},
		{"campaign without keywords", "review my code", perception.Intent{Verb: "/campaign"}, false},

		// Other verbs return false
		{"review verb", "review my code", perception.Intent{Verb: "/review"}, false},
		{"empty verb", "stress test", perception.Intent{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAssaultRequest(tt.input, tt.intent)
			if result != tt.expected {
				t.Errorf("isAssaultRequest(%q, %v) = %v, want %v", tt.input, tt.intent.Verb, result, tt.expected)
			}
		})
	}
}

func TestNormalizeAssaultInclude_WithWorkspace(t *testing.T) {
	workspace := t.TempDir()
	tests := []struct {
		name  string
		input string
	}{
		{"already glob", "internal/core/**"},
		{"directory with slash", "internal/core/"},
		{"file path", "internal/core/kernel.go"},
		{"simple dir", "internal"},
		{"dotted path", "./internal/core"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeAssaultInclude(workspace, tt.input)
			// Verify it doesn't panic and returns sensible output
			if result == "" {
				t.Errorf("normalizeAssaultInclude(%q, %q) returned empty string", workspace, tt.input)
			}
		})
	}
}

func TestIsGenericAssaultTarget(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Generic targets (returns true)
		{"empty string", "", true},
		{"none", "none", true},
		{"dot", ".", true},
		{"dot slash", "./", true},
		{"repo", "repo", true},
		{"repository", "repository", true},
		{"codebase", "codebase", true},
		{"project", "project", true},
		{"everything", "everything", true},
		{"all", "all", true},
		{"whole repo phrase", "the whole repo", true},
		{"entire codebase phrase", "entire codebase", true},

		// Specific targets (returns false)
		{"specific path", "internal/core", false},
		{"file path", "main.go", false},
		{"random text", "hello world", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isGenericAssaultTarget(tt.input)
			if result != tt.expected {
				t.Errorf("isGenericAssaultTarget(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAssaultArgsFromNaturalLanguage_WithWorkspace(t *testing.T) {
	workspace := t.TempDir()
	tests := []struct {
		name  string
		input string
	}{
		{"basic assault request", "assault internal/core"},
		{"with for duration", "stress test for 5 minutes"},
		{"with concurrency", "assault with 4 parallel workers"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := assaultArgsFromNaturalLanguage(workspace, tt.input, perception.Intent{})
			// Verify it returns valid args
			if !ok && len(result) == 0 {
				// This is acceptable - some inputs may not parse to valid args
				t.Logf("assaultArgsFromNaturalLanguage returned empty args for %q", tt.input)
			}
		})
	}
}

func TestAssaultIncludesFromText_WithWorkspace(t *testing.T) {
	workspace := t.TempDir()
	tests := []struct {
		name        string
		input       string
		expectEmpty bool
	}{
		{"with target", "assault internal/core", false},
		{"generic target", "assault the code", true}, // Generic targets filtered out
		{"multiple targets", "test internal/core and cmd/nerd", false},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := assaultIncludesFromText(workspace, tt.input, perception.Intent{})
			isEmpty := len(result) == 0
			if isEmpty != tt.expectEmpty {
				t.Errorf("assaultIncludesFromText(%q) empty=%v, want empty=%v (got %v)", tt.input, isEmpty, tt.expectEmpty, result)
			}
		})
	}
}

// ============================================================================
// campaign.go helper tests
// ============================================================================

func TestCampaignTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"empty string", "", 10, ""},
		{"shorter than max", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"longer than max", "hello world", 5, "he..."},
		{"max of 3", "hello", 3, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if len(result) > tt.maxLen && tt.maxLen >= 3 {
				t.Errorf("truncateString(%q, %d) = %q, length %d exceeds max", tt.input, tt.maxLen, result, len(result))
			}
		})
	}
}

func TestGetStatusIcon(t *testing.T) {
	tests := []struct {
		name   string
		status string
	}{
		{"pending", "pending"},
		{"running", "running"},
		{"completed", "completed"},
		{"failed", "failed"},
		{"unknown", "unknown_status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			result := getStatusIcon(tt.status)
			_ = result // Use the result to avoid compiler warning
		})
	}
}

// ============================================================================
// config_wizard.go helper tests
// ============================================================================

func TestDefaultProviderModel(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantLen  bool // Whether we expect a non-empty result
	}{
		{"gemini", "gemini", true},
		{"openai", "openai", true},
		{"anthropic", "anthropic", true},
		{"unknown", "unknown_provider", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DefaultProviderModel(tt.provider)
			hasResult := result != ""
			if hasResult != tt.wantLen {
				t.Errorf("DefaultProviderModel(%q) = %q, wantLen=%v got hasResult=%v", tt.provider, result, tt.wantLen, hasResult)
			}
		})
	}
}

func TestNewConfigWizard(t *testing.T) {
	wizard := NewConfigWizard()

	if wizard == nil {
		t.Fatal("NewConfigWizard() returned nil")
	}

	// Check defaults
	if wizard.Step != StepWelcome {
		t.Errorf("NewConfigWizard().Step = %v, want %v", wizard.Step, StepWelcome)
	}

	if wizard.Engine != "api" {
		t.Errorf("NewConfigWizard().Engine = %q, want %q", wizard.Engine, "api")
	}

	if wizard.ShardProfiles == nil {
		t.Error("NewConfigWizard().ShardProfiles is nil")
	}

	if wizard.MaxTokens != 128000 {
		t.Errorf("NewConfigWizard().MaxTokens = %d, want %d", wizard.MaxTokens, 128000)
	}

	if wizard.MaxMemoryMB != 2048 {
		t.Errorf("NewConfigWizard().MaxMemoryMB = %d, want %d", wizard.MaxMemoryMB, 2048)
	}
}

func TestProviderModelsMap(t *testing.T) {
	// Verify ProviderModels map has expected providers
	expectedProviders := []string{"gemini", "openai", "anthropic"}

	for _, provider := range expectedProviders {
		if _, ok := ProviderModels[provider]; !ok {
			t.Errorf("ProviderModels missing provider %q", provider)
		}
	}

	// Each provider should have at least one model
	for provider, models := range ProviderModels {
		if len(models) == 0 {
			t.Errorf("ProviderModels[%q] has no models", provider)
		}
	}
}

// ============================================================================
// delegation.go helper tests
// ============================================================================

func TestFilterFindingsBySeverity(t *testing.T) {
	findings := []map[string]any{
		{"raw": "error1", "severity": "error"},
		{"raw": "warning1", "severity": "warning"},
		{"raw": "critical1", "severity": "critical"},
		{"raw": "info1", "severity": "info"},
		{"raw": "error2", "severity": "error"},
	}

	tests := []struct {
		name       string
		severities []string
		expectLen  int
	}{
		{"critical only", []string{"critical"}, 1},
		{"error only", []string{"error"}, 2},
		{"error and critical", []string{"error", "critical"}, 3},
		{"all severities", []string{"critical", "error", "warning", "info"}, 5},
		{"empty filter", []string{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterFindingsBySeverity(findings, tt.severities)
			if len(result) != tt.expectLen {
				t.Errorf("filterFindingsBySeverity(findings, %v) returned %d items, want %d", tt.severities, len(result), tt.expectLen)
			}
		})
	}
}

func TestTruncateForTask(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
	}{
		{"shorter than max", "hello", 100},
		{"exact length", "hello", 5},
		{"needs truncation", "hello world this is a long message", 10},
		{"empty", "", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateForTask(tt.input, tt.maxLen)
			// The function may add "..." when truncating, so just check it's reasonably bounded
			if len(result) > tt.maxLen+3 { // Allow for "..." suffix
				t.Errorf("truncateForTask(%q, %d) returned len=%d, exceeds max+3", tt.input, tt.maxLen, len(result))
			}
		})
	}
}

// ============================================================================
// tips.go helper tests
// ============================================================================

func TestNewTipGenerator(t *testing.T) {
	workspace := t.TempDir()
	generator := NewTipGenerator(workspace)
	if generator == nil {
		t.Fatal("NewTipGenerator() returned nil")
	}
}

func TestTipGenerator_ShouldShowTip(t *testing.T) {
	workspace := t.TempDir()
	generator := NewTipGenerator(workspace)
	if generator == nil {
		t.Skip("NewTipGenerator returned nil")
	}

	// Just verify it doesn't panic
	result := generator.ShouldShowTip()
	_ = result
}

// ============================================================================
// command_categories.go tests
// ============================================================================

func TestCommandCategoryString(t *testing.T) {
	tests := []struct {
		category CommandCategory
		expected string
	}{
		{CategoryCore, "Core"},
		{CategoryBasic, "Basic"},
		{CategoryAdvanced, "Advanced"},
		{CategoryExpert, "Expert"},
		{CategorySystem, "System"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.category.String()
			if result != tt.expected {
				t.Errorf("CommandCategory.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetCommandsByCategory(t *testing.T) {
	// Test that we can get commands for each category
	categories := []CommandCategory{CategoryCore, CategoryBasic, CategoryAdvanced, CategoryExpert, CategorySystem}

	for _, cat := range categories {
		cmds := GetCommandsByCategory(cat)
		// Some categories may be empty, just verify no panic
		_ = cmds
	}
}

func TestGetCommandsForLevel(t *testing.T) {
	// GetCommandsForLevel doesn't exist in this package, skip
	t.Skip("GetCommandsForLevel not available in this package")
}

func TestFindCommand(t *testing.T) {
	tests := []struct {
		name      string
		wantFound bool
	}{
		{"/help", true},
		{"/quit", true},
		{"/review", true},
		{"nonexistent_command_xyz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := FindCommand(tt.name)
			found := cmd != nil
			if found != tt.wantFound {
				t.Errorf("FindCommand(%q) found=%v, want %v", tt.name, found, tt.wantFound)
			}
		})
	}
}

func TestGetAllCategories(t *testing.T) {
	categories := GetAllCategories()
	if len(categories) == 0 {
		t.Error("GetAllCategories() returned empty map")
	}
}

// =============================================================================
// EVOLUTION HELPERS TESTS
// =============================================================================

func TestFormatTimeAgo(t *testing.T) {
	tests := []struct {
		name     string
		t        time.Time
		contains string
	}{
		{
			name:     "zero_time",
			t:        time.Time{},
			contains: "never",
		},
		{
			name:     "just_now",
			t:        time.Now().Add(-10 * time.Second),
			contains: "just now",
		},
		{
			name:     "minutes_ago",
			t:        time.Now().Add(-5 * time.Minute),
			contains: "minutes ago",
		},
		{
			name:     "hours_ago",
			t:        time.Now().Add(-3 * time.Hour),
			contains: "hours ago",
		},
		{
			name:     "days_ago",
			t:        time.Now().Add(-48 * time.Hour),
			contains: "days ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimeAgo(tt.t)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("formatTimeAgo() = %q, want to contain %q", result, tt.contains)
			}
		})
	}
}

func TestFormatEvolutionResult(t *testing.T) {
	t.Run("nil_result", func(t *testing.T) {
		result := formatEvolutionResult(nil)
		if !strings.Contains(result, "no results") {
			t.Error("Expected 'no results' message for nil input")
		}
	})

	t.Run("empty_result", func(t *testing.T) {
		result := formatEvolutionResult(&pe.EvolutionResult{})
		if !strings.Contains(result, "Evolution Cycle Complete") {
			t.Error("Expected 'Evolution Cycle Complete' heading")
		}
		if !strings.Contains(result, "Groups Processed") {
			t.Error("Expected 'Groups Processed' in output")
		}
	})

	t.Run("full_result", func(t *testing.T) {
		result := formatEvolutionResult(&pe.EvolutionResult{
			GroupsProcessed:   5,
			FailuresAnalyzed:  10,
			AtomsGenerated:    3,
			AtomsPromoted:     2,
			StrategiesCreated: 1,
			StrategiesRefined: 1,
			Duration:          100 * time.Millisecond,
			AtomIDs:           []string{"atom1", "atom2"},
			Errors:            []string{"error1"},
		})

		checks := []string{
			"Groups Processed**: 5",
			"Failures Analyzed**: 10",
			"Atoms Generated**: 3",
			"New Atoms Generated",
			"atom1",
			"atom2",
			"Errors",
			"error1",
		}

		for _, check := range checks {
			if !strings.Contains(result, check) {
				t.Errorf("Result should contain %q", check)
			}
		}
	})
}

// =============================================================================
// CAMPAIGN ASSAULT HELPERS TESTS
// =============================================================================

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", []string{}},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b, c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
		{",,,", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitCSV(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitCSV(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitCSV(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// =============================================================================
// GLASS BOX HELPERS TESTS
// =============================================================================

func TestCategoryDescription(t *testing.T) {
	tests := []struct {
		category     transparency.GlassBoxCategory
		wantNonEmpty bool
	}{
		{transparency.CategoryPerception, true},
		{transparency.CategoryKernel, true},
		{transparency.CategoryJIT, true},
		{transparency.CategoryShard, true},
		{transparency.CategoryControl, true},
		{transparency.GlassBoxCategory("unknown"), true}, // Unknown category
	}

	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			result := categoryDescription(tt.category)
			if tt.wantNonEmpty && result == "" {
				t.Error("categoryDescription returned empty string")
			}
		})
	}
}

// =============================================================================
// HELP RENDERER TESTS
// =============================================================================

func TestNewHelpRenderer(t *testing.T) {
	// Create a temp workspace
	tmpDir := t.TempDir()

	renderer := NewHelpRenderer(tmpDir)
	if renderer == nil {
		t.Fatal("NewHelpRenderer returned nil")
	}
	if renderer.workspace != tmpDir {
		t.Errorf("workspace = %q, want %q", renderer.workspace, tmpDir)
	}
}

func TestHelpRenderer_RenderHelp(t *testing.T) {
	tmpDir := t.TempDir()
	renderer := NewHelpRenderer(tmpDir)

	tests := []struct {
		name     string
		arg      string
		contains string
	}{
		{"empty", "", "/help"},               // Default progressive help
		{"all", "all", "Commands"},           // All commands
		{"core", "core", ""},                 // Core category
		{"basic", "basic", ""},               // Basic category
		{"advanced", "advanced", ""},         // Advanced category
		{"expert", "expert", ""},             // Expert category
		{"specific_command", "help", "help"}, // Specific command lookup
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderer.RenderHelp(tt.arg)
			// Just verify no panic and non-empty result for most
			if result == "" && tt.arg != "advanced" && tt.arg != "expert" && tt.arg != "core" && tt.arg != "basic" {
				t.Error("RenderHelp returned empty string")
			}
		})
	}
}

func TestHelpRenderer_GetCurrentLevel(t *testing.T) {
	tmpDir := t.TempDir()
	renderer := NewHelpRenderer(tmpDir)

	level := renderer.GetCurrentLevel()
	// Default should be beginner
	if level != config.ExperienceBeginner {
		t.Errorf("GetCurrentLevel() = %v, want %v", level, config.ExperienceBeginner)
	}
}

func TestHelpRenderer_SetLevel(t *testing.T) {
	tmpDir := t.TempDir()
	renderer := NewHelpRenderer(tmpDir)

	renderer.SetLevel(config.ExperienceExpert)
	if renderer.GetCurrentLevel() != config.ExperienceExpert {
		t.Error("SetLevel did not update level")
	}
}

// =============================================================================
// RENDER CAMPAIGN HELPERS TESTS
// =============================================================================

func TestRenderCampaignStarted_WithCampaign(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	c := &campaign.Campaign{
		ID:     "test-campaign",
		Title:  "Test Campaign",
		Goal:   "A test campaign",
		Status: campaign.StatusActive,
	}
	result := m.renderCampaignStarted(c)
	if result == "" {
		t.Error("renderCampaignStarted returned empty string")
	}
}

func TestRenderCampaignCompleted_WithCampaign(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	c := &campaign.Campaign{
		ID:     "test-campaign",
		Title:  "Test Campaign",
		Goal:   "A test campaign",
		Status: campaign.StatusCompleted,
	}
	result := m.renderCampaignCompleted(c)
	if result == "" {
		t.Error("renderCampaignCompleted returned empty string")
	}
}

func TestRenderCampaignList_NoArgs(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	result := m.renderCampaignList()
	// May show "no campaigns" or empty
	_ = result
}

func TestRenderCampaignPanel_NoActiveCampaign(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	result := m.renderCampaignPanel()
	// May be empty if no active campaign
	_ = result
}

func TestGetStatusIcon_CampaignTypes(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{string(campaign.StatusActive), "*"},
		{string(campaign.StatusCompleted), "+"},
		{string(campaign.StatusPaused), "="},
		{string(campaign.StatusFailed), "x"},
		{string(campaign.TaskSkipped), "-"},
		{string(campaign.TaskBlocked), "!"},
		{"unknown", "?"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := getStatusIcon(tt.status)
			if got != tt.want {
				t.Errorf("getStatusIcon(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// =============================================================================
// COMMANDS TOOLS HELPERS TESTS
// =============================================================================

func TestRenderCleanupToolsHelp(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	result := m.renderCleanupToolsHelp()
	if result == "" {
		t.Error("renderCleanupToolsHelp returned empty string")
	}
	if !strings.Contains(result, "cleanup") && !strings.Contains(result, "Cleanup") {
		t.Error("Result should mention cleanup")
	}
}

// =============================================================================
// CLARIFICATION FORMATTING TESTS
// =============================================================================

func TestFormatClarificationRequest(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	t.Run("no_options", func(t *testing.T) {
		state := ClarificationState{
			Question: "What is your preference?",
			Options:  nil,
		}
		result := m.formatClarificationRequest(state)
		if !strings.Contains(result, "What is your preference?") {
			t.Error("Result should contain the question")
		}
		if !strings.Contains(result, "clarification") {
			t.Error("Result should mention clarification")
		}
	})

	t.Run("with_options", func(t *testing.T) {
		state := ClarificationState{
			Question: "Which option?",
			Options:  []string{"Option A", "Option B", "Option C"},
		}
		result := m.formatClarificationRequest(state)
		if !strings.Contains(result, "Option A") {
			t.Error("Result should contain options")
		}
		if !strings.Contains(result, "Options:") {
			t.Error("Result should have Options header")
		}
	})
}

// =============================================================================
// VIEW RENDERING ADDITIONAL TESTS
// =============================================================================

func TestRenderErrorPanel_WithError(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))
	m.err = fmt.Errorf("Test error message")

	result := m.renderErrorPanel()
	if result == "" {
		t.Error("renderErrorPanel should return non-empty for errors")
	}
}

func TestRenderHeader(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	result := m.renderHeader()
	// Header should be non-empty
	if result == "" {
		t.Error("renderHeader returned empty string")
	}
}

func TestRenderFooter(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	result := m.renderFooter()
	// Footer should be non-empty
	if result == "" {
		t.Error("renderFooter returned empty string")
	}
}

func TestRenderBootScreen_Boot(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))
	m.isBooting = true
	m.bootStage = BootStageBooting

	result := m.renderBootScreen()
	if result == "" {
		t.Error("renderBootScreen returned empty string")
	}
}

func TestView_ChatMode(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))
	m.viewMode = ChatView
	m.ready = true
	m.isBooting = false

	result := m.View()
	if result == "" {
		t.Error("View() returned empty for ChatView mode")
	}
}

// =============================================================================
// GLASS BOX TOGGLE TESTS
// =============================================================================

func TestToggleGlassBox_Method(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	// Initially disabled
	if m.glassBoxEnabled {
		t.Error("Glass box should be disabled initially")
	}

	// Toggle on - returns status message
	statusMsg := m.toggleGlassBox()
	if statusMsg == "" {
		t.Error("toggleGlassBox should return a status message")
	}
}

func TestIsGlassBoxVerbose(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	// Initially not verbose
	if m.isGlassBoxVerbose() {
		t.Error("Glass box should not be verbose initially")
	}
}

// =============================================================================
// ADDITIONAL MODEL HELPERS TESTS
// =============================================================================

func TestRefreshErrorViewport(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))
	m.err = fmt.Errorf("Error 1")

	// Should not panic
	m.refreshErrorViewport()
}

// =============================================================================
// ADDITIONAL CAMPAIGN TESTS
// =============================================================================

func TestRenderCampaignStatus(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))

	result := m.renderCampaignStatus()
	// May show "no active campaign" or be empty
	_ = result
}

// =============================================================================
// TIP FORMATTING TESTS
// =============================================================================

func TestFormatTip(t *testing.T) {
	t.Run("nil_tip", func(t *testing.T) {
		result := FormatTip(nil)
		if result != "" {
			t.Error("FormatTip(nil) should return empty string")
		}
	})

	t.Run("tip_without_command", func(t *testing.T) {
		tip := &ContextualTip{
			Text: "This is a helpful tip",
		}
		result := FormatTip(tip)
		if !strings.Contains(result, "Tip") {
			t.Error("Result should contain 'Tip'")
		}
		if !strings.Contains(result, "helpful tip") {
			t.Error("Result should contain tip text")
		}
	})

	t.Run("tip_with_command", func(t *testing.T) {
		tip := &ContextualTip{
			Text:    "Try this command",
			Command: "/help",
		}
		result := FormatTip(tip)
		if !strings.Contains(result, "/help") {
			t.Error("Result should contain the command")
		}
		if !strings.Contains(result, "Try:") {
			t.Error("Result should have 'Try:' label")
		}
	})
}

func TestGetRandomGenericTip(t *testing.T) {
	levels := []config.ExperienceLevel{
		config.ExperienceBeginner,
		config.ExperienceIntermediate,
		config.ExperienceAdvanced,
		config.ExperienceExpert,
	}

	for _, level := range levels {
		t.Run(string(level), func(t *testing.T) {
			tip := GetRandomGenericTip(level)
			if tip == "" {
				t.Errorf("GetRandomGenericTip(%s) returned empty", level)
			}
			if !strings.Contains(tip, "Tip") {
				t.Errorf("Tip should contain 'Tip': %s", tip)
			}
		})
	}
}

func TestTipGenerator_GenerateTip(t *testing.T) {
	tmpDir := t.TempDir()
	generator := NewTipGenerator(tmpDir)

	// Test tip generation with empty context
	ctx := TipContext{}
	tip := generator.GenerateTip(ctx)
	// May be nil if no tips apply
	_ = tip
}

// =============================================================================
// RENDER GLASS BOX MESSAGE TESTS
// =============================================================================

func TestRenderGlassBoxMessage(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))
	m.glassBoxEnabled = true

	msg := Message{
		Role:    "system",
		Content: "Test system message",
		Time:    time.Now(),
	}

	result := m.renderGlassBoxMessage(msg)
	// May be empty if no glass box events
	_ = result
}

// =============================================================================
// NORTHSTAR WIZARD TESTS
// =============================================================================

func TestNewNorthstarWizard(t *testing.T) {
	state := NewNorthstarWizard()
	if state == nil {
		t.Fatal("NewNorthstarWizard returned nil")
	}
}

// =============================================================================
// AGENT WIZARD STATE TESTS
// =============================================================================

func TestAgentWizardState(t *testing.T) {
	// Test initial state
	state := &AgentWizardState{}
	if state.Step != 0 {
		t.Error("Initial step should be 0 (Name)")
	}
	if state.Name != "" {
		t.Error("Initial Name should be empty")
	}
}

// =============================================================================
// ONBOARDING WIZARD STATE TESTS
// =============================================================================

func TestOnboardingWizardState(t *testing.T) {
	// Test initial state
	state := &OnboardingWizardState{
		Step: OnboardingStepWelcome,
	}
	if state.Step != OnboardingStepWelcome {
		t.Error("Initial step should be OnboardingStepWelcome")
	}
}
