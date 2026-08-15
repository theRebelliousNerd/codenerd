package browser

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/mangle"
)

var honeypotReasonHeadPattern = regexp.MustCompile(`(?m)^\s*honeypot_reason\(\s*\w+\s*,\s*(/[a-z_0-9]+)\s*\)`)

func honeypotPolicyPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(getProjectRoot(t), "internal/core/defaults/policy/browser_honeypot.mg")
}

// TestHoneypotReasonCodes_WhenComparedToPolicy_ShouldMatchExactly is the
// alignment contract between the Go reason table and browser_honeypot.mg.
//
// The Go table used to be a private checklist of predicate names. It listed
// honeypot_clip_hidden and honeypot_overflow_hidden, which no rule derived, so
// two reasons could never be reported; and it treated every listed predicate as
// proof of a honeypot even though is_honeypot deliberately excludes
// honeypot_no_keyboard. A code in one file and not the other is now a failure.
func TestHoneypotReasonCodes_WhenComparedToPolicy_ShouldMatchExactly(t *testing.T) {
	data, err := os.ReadFile(honeypotPolicyPath(t))
	if err != nil {
		t.Fatalf("read policy: %v", err)
	}

	policyCodes := make([]string, 0, len(honeypotReasonCodes))
	for _, match := range honeypotReasonHeadPattern.FindAllStringSubmatch(string(data), -1) {
		policyCodes = append(policyCodes, match[1])
	}
	if len(policyCodes) == 0 {
		t.Fatal("no honeypot_reason rules found; the parser or the policy file is broken")
	}

	goCodes := make([]string, 0, len(honeypotReasonCodes))
	for _, entry := range honeypotReasonCodes {
		goCodes = append(goCodes, entry.Code)
	}

	sortedPolicy := append([]string(nil), policyCodes...)
	sortedGo := append([]string(nil), goCodes...)
	sort.Strings(sortedPolicy)
	sort.Strings(sortedGo)
	if !slices.Equal(sortedPolicy, sortedGo) {
		t.Errorf("reason codes differ.\n  browser_honeypot.mg: %v\n  honeypotReasonCodes: %v", sortedPolicy, sortedGo)
	}

	for _, entry := range honeypotReasonCodes {
		if strings.TrimSpace(entry.Text) == "" {
			t.Errorf("reason code %s has no operator-facing text", entry.Code)
		}
	}
}

// TestHoneypotPolicy_WhenEvidenceAsserted_ShouldDeriveExpectedVerdict drives
// the rule file directly: each case asserts evidence and checks both the
// derived reason set and the is_honeypot verdict, which are different
// questions.
func TestHoneypotPolicy_WhenEvidenceAsserted_ShouldDeriveExpectedVerdict(t *testing.T) {
	cases := []struct {
		name    string
		facts   []mangle.Fact
		elem    string
		verdict bool
		reasons []string
	}{
		{
			name: "suspicious bait path",
			facts: []mangle.Fact{
				{Predicate: "link_url_pattern", Args: []any{"u1", "/bait_path"}},
			},
			elem:    "u1",
			verdict: true,
			reasons: []string{"Suspicious URL pattern"},
		},
		{
			name: "suspicious trap query",
			facts: []mangle.Fact{
				{Predicate: "link_url_pattern", Args: []any{"u2", "/trap_query"}},
			},
			elem:    "u2",
			verdict: true,
			reasons: []string{"Suspicious URL pattern"},
		},
		{
			name: "empty js target is recorded but is not a trap",
			facts: []mangle.Fact{
				{Predicate: "link_url_pattern", Args: []any{"u3", "/empty_js_target"}},
			},
			elem:    "u3",
			verdict: false,
		},
		{
			name: "collapsed clip rectangle",
			facts: []mangle.Fact{
				{Predicate: "css_clip_rect", Args: []any{"c1", int64(1), int64(1), int64(1), int64(1)}},
			},
			elem:    "c1",
			verdict: true,
			reasons: []string{"Clipped to zero size"},
		},
		{
			name: "open clip rectangle",
			facts: []mangle.Fact{
				{Predicate: "css_clip_rect", Args: []any{"c2", int64(0), int64(200), int64(80), int64(0)}},
			},
			elem:    "c2",
			verdict: false,
		},
		{
			name: "overflow hidden alone is not a trap",
			facts: []mangle.Fact{
				{Predicate: "css_property", Args: []any{"o1", "overflow", "hidden"}},
				{Predicate: "position", Args: []any{"o1", int64(0), int64(0), int64(320), int64(240)}},
			},
			elem:    "o1",
			verdict: false,
		},
		{
			name: "overflow hidden on a collapsed box",
			facts: []mangle.Fact{
				{Predicate: "css_property", Args: []any{"o2", "overflow", "hidden"}},
				{Predicate: "position", Args: []any{"o2", int64(0), int64(0), int64(0), int64(0)}},
			},
			elem:    "o2",
			verdict: true,
			reasons: []string{"Zero or near-zero size", "Content clipped via overflow"},
		},
		{
			name: "negative tabindex is evidence, not a verdict",
			facts: []mangle.Fact{
				{Predicate: "attribute", Args: []any{"k1", "tabindex", "-1"}},
			},
			elem:    "k1",
			verdict: false,
			reasons: []string{"Not keyboard accessible (negative tabindex)"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := newBrowserTestEngine(t)
			detector := NewHoneypotDetector(engine)
			if err := engine.AddFacts(tc.facts); err != nil {
				t.Fatalf("add facts: %v", err)
			}
			if err := engine.RecomputeRules(); err != nil {
				t.Fatalf("recompute: %v", err)
			}

			if got := detector.isHoneypot(tc.elem); got != tc.verdict {
				t.Errorf("is_honeypot(%s) = %v, want %v (reasons: %v)", tc.elem, got, tc.verdict, detector.getHoneypotReasons(tc.elem))
			}
			reasons := detector.getHoneypotReasons(tc.elem)
			if tc.reasons != nil && !slices.Equal(reasons, tc.reasons) {
				t.Errorf("reasons(%s) = %v, want %v", tc.elem, reasons, tc.reasons)
			}
		})
	}
}

// TestSafeInteractable_WhenElementIsHoneypot_ShouldBeExcluded checks the
// negation in browser_honeypot.mg actually filters. A negated literal holding
// an anonymous wildcard excludes nothing in this Mangle build, so the rule is
// written with the variable bound by interactable/2.
func TestSafeInteractable_WhenElementIsHoneypot_ShouldBeExcluded(t *testing.T) {
	engine := newBrowserTestEngine(t)
	if err := engine.AddFacts([]mangle.Fact{
		{Predicate: "interactable", Args: []any{"good", "/click"}},
		{Predicate: "interactable", Args: []any{"trap", "/click"}},
		{Predicate: "css_property", Args: []any{"trap", "display", "none"}},
	}); err != nil {
		t.Fatalf("add facts: %v", err)
	}
	if err := engine.RecomputeRules(); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	safe := engine.QueryFacts("safe_interactable")
	ids := make([]string, 0, len(safe))
	for _, fact := range safe {
		if len(fact.Args) > 0 {
			ids = append(ids, strings.TrimPrefix(fact.Args[0].(string), "/"))
		}
	}
	sort.Strings(ids)
	if !slices.Equal(ids, []string{"good"}) {
		t.Errorf("safe_interactable = %v, want [good] — the honeypot must be excluded", ids)
	}
}

func TestClassifyLinkURL_WhenHrefIsBait_ShouldReportPattern(t *testing.T) {
	cases := []struct {
		href string
		want []string
	}{
		{"/products/42", nil},
		{"https://example.com/trapani/hotels", nil},
		{"https://example.com/honeypot", []string{"/bait_path"}},
		{"/bot-trap/index.html", []string{"/bait_path"}},
		{"/do-not-click", []string{"/bait_path"}},
		{"/page?src=spamtrap", []string{"/trap_query"}},
		{"/page?ok=1", nil},
		{"#", []string{"/empty_js_target"}},
		{"javascript:void(0)", []string{"/empty_js_target"}},
		{"/x#nocrawl", []string{"/bait_path"}},
		{"", nil},
		{"   ", nil},
	}
	for _, tc := range cases {
		t.Run(tc.href, func(t *testing.T) {
			got := ClassifyLinkURL(tc.href)
			if !slices.Equal(got, tc.want) {
				t.Errorf("ClassifyLinkURL(%q) = %v, want %v", tc.href, got, tc.want)
			}
		})
	}
}

func TestParseClipRect_WhenComputedValueVaries_ShouldParseOrReject(t *testing.T) {
	cases := []struct {
		value string
		ok    bool
		right int64
	}{
		{"auto", false, 0},
		{"", false, 0},
		{"rect(0px, 0px, 0px, 0px)", true, 0},
		{"rect(1px, 1px, 1px, 1px)", true, 1},
		{"rect(0px 0px 0px 0px)", true, 0},
		{"rect(0px, 200px, 80px, 0px)", true, 200},
		{"rect(0px, 0px, 0px)", false, 0},
		{"rect(auto, auto, auto, auto)", true, 1 << 20},
		{"rect(a, b, c, d)", false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			_, right, _, _, ok := parseClipRect(tc.value)
			if ok != tc.ok {
				t.Fatalf("parseClipRect(%q) ok = %v, want %v", tc.value, ok, tc.ok)
			}
			if ok && right != tc.right {
				t.Errorf("parseClipRect(%q) right = %d, want %d", tc.value, right, tc.right)
			}
		})
	}
}

// TestHoneypotScope_WhenTwoElementsChecked_ShouldNotShareEvidence covers the
// fixed-ID bug: every check used to assert under "check_elem", and because the
// fact store is monotonic, one hidden element made every later check report a
// honeypot forever.
func TestHoneypotScope_WhenTwoElementsChecked_ShouldNotShareEvidence(t *testing.T) {
	engine := newBrowserTestEngine(t)
	detector := NewHoneypotDetector(engine)

	first := newHoneypotScope() + "_check"
	second := newHoneypotScope() + "_check"
	if first == second {
		t.Fatal("honeypot scopes must be unique per check")
	}

	if err := engine.AddFacts([]mangle.Fact{
		{Predicate: "css_property", Args: []any{first, "display", "none"}},
		{Predicate: "css_property", Args: []any{second, "display", "block"}},
	}); err != nil {
		t.Fatalf("add facts: %v", err)
	}
	if err := engine.RecomputeRules(); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	if !detector.isHoneypot(first) {
		t.Error("hidden element should be a honeypot")
	}
	if detector.isHoneypot(second) {
		t.Error("visible element inherited the previous check's evidence")
	}
}
