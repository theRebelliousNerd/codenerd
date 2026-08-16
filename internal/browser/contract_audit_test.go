package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestAuditNeedles_ShouldSplitRoutesIntoSegments(t *testing.T) {
	needles := auditNeedles([]string{"/orders/checkout"}, nil, nil)
	if len(needles) != 2 {
		t.Fatalf("expected 2 needles, got %v", needles)
	}
	if !containsStr(needles, "orders") {
		t.Fatalf("expected orders in %v", needles)
	}
	if !containsStr(needles, "checkout") {
		t.Fatalf("expected checkout in %v", needles)
	}
	for _, n := range needles {
		if n == "/orders/checkout" {
			t.Fatalf("should not contain whole path")
		}
	}
	// api should be dropped as noise
	needles = auditNeedles([]string{"/api/orders/checkout"}, nil, nil)
	if containsStr(needles, "api") {
		t.Fatalf("api should be dropped, got %v", needles)
	}
	if !containsStr(needles, "orders") || !containsStr(needles, "checkout") {
		t.Fatalf("orders/checkout expected in %v", needles)
	}
	// deterministic sorted
	sorted := make([]string, len(needles))
	copy(sorted, needles)
	sort.Strings(sorted)
	for i := range needles {
		if needles[i] != sorted[i] {
			t.Fatalf("not sorted: %v", needles)
		}
	}
}

func TestAuditNeedles_WhenSegmentTooShort_ShouldDrop(t *testing.T) {
	needles := auditNeedles([]string{"/a/bc/def", "/ab"}, nil, nil)
	if len(needles) != 0 {
		t.Fatalf("expected 0 for short segments, got %v", needles)
	}
	needles = auditNeedles([]string{"/orders/ab", "/a/orders"}, nil, nil)
	if containsStr(needles, "ab") {
		t.Fatalf("ab too short should be dropped, got %v", needles)
	}
	if containsStr(needles, "a") {
		t.Fatalf("a too short should be dropped")
	}
	if !containsStr(needles, "orders") {
		t.Fatalf("orders expected in %v", needles)
	}
}

func TestAuditNeedles_WhenNumericSegment_ShouldDrop(t *testing.T) {
	needles := auditNeedles([]string{"/orders/1234/checkout", "/users/56789/profile"}, nil, nil)
	if containsStr(needles, "1234") {
		t.Fatalf("numeric 1234 should be dropped, got %v", needles)
	}
	if containsStr(needles, "56789") {
		t.Fatalf("numeric 56789 should be dropped")
	}
	if !containsStr(needles, "orders") || !containsStr(needles, "checkout") {
		t.Fatalf("orders/checkout expected in %v", needles)
	}
	if !containsStr(needles, "users") || !containsStr(needles, "profile") {
		t.Fatalf("users/profile expected in %v", needles)
	}
	needles = auditNeedles(nil, []string{"1234", "abcd"}, nil)
	if containsStr(needles, "1234") {
		t.Fatalf("numeric field should be dropped")
	}
	if !containsStr(needles, "abcd") {
		t.Fatalf("abcd expected")
	}
}

func TestAuditNeedles_WhenURLHasQuery_ShouldIgnoreQuery(t *testing.T) {
	needles := auditNeedles(nil, nil, []string{"https://example.com/orders/checkout?token=secret123&foo=bar"})
	if !containsStr(needles, "orders") {
		t.Fatalf("orders expected in %v", needles)
	}
	if !containsStr(needles, "checkout") {
		t.Fatalf("checkout expected")
	}
	if containsStr(needles, "secret123") {
		t.Fatalf("query value secret123 must not become needle, got %v", needles)
	}
	if containsStr(needles, "token") {
		t.Fatalf("query key token must not become needle")
	}
	if containsStr(needles, "bar") {
		t.Fatalf("query value bar must not become needle")
	}
	needles = auditNeedles(nil, nil, []string{"https://example.com/api/orders?secretToken=supersecretvalue"})
	if containsStr(needles, "supersecretvalue") {
		t.Fatalf("query value supersecretvalue must not leak, got %v", needles)
	}
	if containsStr(needles, "secrettoken") {
		t.Fatalf("query key secrettoken must not leak")
	}
	// fragment also ignored
	needles = auditNeedles(nil, nil, []string{"https://example.com/orders#section?token=secret"})
	if containsStr(needles, "secret") {
		t.Fatalf("fragment/query secret must not leak")
	}
}

func TestAuditNeedles_ShouldDeduplicateCaseInsensitively(t *testing.T) {
	needles := auditNeedles([]string{"/Orders", "/orders", "/ORDERS"}, []string{"orders", "ORDERS"}, []string{"https://example.com/Orders"})
	if len(needles) != 1 {
		t.Fatalf("expected dedup to 1, got %v", needles)
	}
	if needles[0] != "orders" {
		t.Fatalf("expected orders, got %q", needles[0])
	}
	needles = auditNeedles(nil, []string{"Email", "email", "EMAIL"}, nil)
	if len(needles) != 1 {
		t.Fatalf("expected 1 for email dedup, got %v", needles)
	}
}

func TestAuditNeedles_WhenTooMany_ShouldCapKeepingLongest(t *testing.T) {
	var fields []string
	for i := 0; i < 30; i++ {
		fields = append(fields, strings.Repeat("x", 4+i))
	}
	needles := auditNeedles(nil, fields, nil)
	if len(needles) != maxAuditNeedles {
		t.Fatalf("expected cap %d, got %d: %v", maxAuditNeedles, len(needles), needles)
	}
	if containsStr(needles, strings.Repeat("x", 4)) {
		t.Fatalf("shortest x4 should have been dropped after cap, got %v", needles)
	}
	if containsStr(needles, strings.Repeat("x", 5)) {
		t.Fatalf("x5 should have been dropped, got %v", needles)
	}
	if !containsStr(needles, strings.Repeat("x", 33)) {
		t.Fatalf("longest x33 should be kept, got %v", needles)
	}
	if !containsStr(needles, strings.Repeat("x", 32)) {
		t.Fatalf("x32 should be kept")
	}
	sorted := make([]string, len(needles))
	copy(sorted, needles)
	sort.Strings(sorted)
	for i := range needles {
		if needles[i] != sorted[i] {
			t.Fatalf("not sorted at %d: %v vs %v", i, needles, sorted)
		}
	}
	needles2 := auditNeedles(nil, fields, nil)
	if len(needles2) != len(needles) {
		t.Fatalf("determinism length mismatch")
	}
	for i := range needles {
		if needles[i] != needles2[i] {
			t.Fatalf("determinism mismatch at %d: %q vs %q", i, needles[i], needles2[i])
		}
	}
}

func TestDiscoverContract_WhenNoNeedles_ShouldEmitSkippedNotSilence(t *testing.T) {
	root := t.TempDir()
	writeTraceFile(t, root, "a.txt", "hello")
	in := ContractAuditInput{
		RepoRoot:     root,
		Routes:       []string{"/a/b", "/api/v1"},
		FormFields:   []string{"ab", "12"},
		RequestURLs:  []string{"https://example.com/api/v1?token=secret"},
		MutatingControls: nil,
	}
	disc, err := DiscoverContract(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(disc.Needles) != 0 {
		t.Fatalf("expected 0 needles, got %v", disc.Needles)
	}
	if len(disc.Matches) != 0 {
		t.Fatalf("expected 0 matches when no needles, got %d", len(disc.Matches))
	}
	foundSkipped := false
	for _, f := range disc.Findings {
		if f.Kind == AuditSkipped {
			foundSkipped = true
			if !strings.Contains(strings.ToLower(f.Detail), "no searchable") {
				t.Fatalf("skipped detail unexpected: %q", f.Detail)
			}
		}
	}
	if !foundSkipped {
		t.Fatalf("expected AuditSkipped finding, got %v", disc.Findings)
	}
}

func TestDiscoverContract_WhenNeedleMatches_ShouldEmitInferenceWithRelativeSources(t *testing.T) {
	root := t.TempDir()
	writeTraceFile(t, root, "src/orders.go", "package src\nfunc handleOrders() {}\n// orders handler")
	writeTraceFile(t, root, "other.txt", "nothing relevant")
	in := ContractAuditInput{
		RepoRoot: root,
		Routes:   []string{"/orders/checkout"},
	}
	disc, err := DiscoverContract(context.Background(), in)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(disc.Matches) == 0 {
		t.Fatalf("expected matches for orders")
	}
	found := false
	for _, f := range disc.Findings {
		if f.Kind == AuditInference && f.Subject == "orders" {
			found = true
			if len(f.Sources) == 0 {
				t.Fatalf("expected sources for inference")
			}
			for _, src := range f.Sources {
				if filepath.IsAbs(src) {
					t.Fatalf("source is absolute %q", src)
				}
				if strings.Contains(src, root) {
					t.Fatalf("source leaks temp dir %q contains %q", src, root)
				}
			}
			if !containsStr(f.Sources, "src/orders.go") {
				t.Fatalf("expected src/orders.go in sources %v", f.Sources)
			}
		}
	}
	if !found {
		t.Fatalf("expected inference finding for orders, got %v", disc.Findings)
	}
}

func TestDiscoverContract_WhenNeedleAbsent_ShouldEmitObservation(t *testing.T) {
	root := t.TempDir()
	writeTraceFile(t, root, "a.txt", "hello world")
	in := ContractAuditInput{
		RepoRoot: root,
		Routes:   []string{"/nonexistent"},
	}
	disc, err := DiscoverContract(context.Background(), in)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	found := false
	for _, f := range disc.Findings {
		if f.Kind == AuditObservation && f.Subject == "nonexistent" {
			found = true
			if len(f.Sources) != 0 {
				t.Fatalf("observation should have no sources")
			}
			if !strings.Contains(strings.ToLower(f.Detail), "nowhere") {
				t.Fatalf("observation detail should mention nowhere, got %q", f.Detail)
			}
		}
	}
	if !found {
		t.Fatalf("expected observation for nonexistent, got %v", disc.Findings)
	}
}

func TestDiscoverContract_WhenMutatingControl_ShouldEmitApprovalRequired(t *testing.T) {
	root := t.TempDir()
	content := "orders checkout content"
	writeTraceFile(t, root, "app.txt", content)
	filePath := filepath.Join(root, "app.txt")
	infoBefore, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}
	modBefore := infoBefore.ModTime()
	dataBefore, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	in := ContractAuditInput{
		RepoRoot:         root,
		Routes:           []string{"/orders"},
		MutatingControls: []string{"submitBtn", "#checkout-submit"},
	}
	disc, err := DiscoverContract(context.Background(), in)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	count := 0
	for _, f := range disc.Findings {
		if f.Kind == AuditApprovalRequired {
			count++
			if !strings.Contains(strings.ToLower(f.Detail), "approval") {
				t.Fatalf("approval detail missing approval word: %q", f.Detail)
			}
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 approval_required findings, got %d: %v", count, disc.Findings)
	}
	dataAfter, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(dataBefore) != string(dataAfter) {
		t.Fatalf("file content was modified")
	}
	infoAfter, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if !infoAfter.ModTime().Equal(modBefore) {
		t.Fatalf("file modtime changed: before %v after %v", modBefore, infoAfter.ModTime())
	}
	// ensure no execution happened: file still contains original and not extra
	if !strings.Contains(string(dataAfter), "orders") {
		t.Fatalf("file corrupted")
	}
	_ = fmt.Sprintf("keep fmt import %v", disc)
}

func TestDiscoverContract_ShouldNeverEmitMismatchOrExecutionFailure(t *testing.T) {
	root := t.TempDir()
	writeTraceFile(t, root, "a.txt", "orders checkout token")
	in := ContractAuditInput{
		RepoRoot:         root,
		Routes:           []string{"/orders", "/checkout"},
		FormFields:       []string{"email"},
		RequestURLs:      []string{"https://example.com/orders"},
		MutatingControls: []string{"btn"},
	}
	disc, err := DiscoverContract(context.Background(), in)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	for _, f := range disc.Findings {
		if f.Kind == AuditContractMismatch {
			t.Fatalf("should never emit contract_mismatch, got %v", f)
		}
		if f.Kind == AuditExecutionFailure {
			t.Fatalf("should never emit execution_failure, got %v", f)
		}
	}
	in2 := ContractAuditInput{
		RepoRoot: root,
		Routes:   []string{"/a"},
	}
	disc2, err := DiscoverContract(context.Background(), in2)
	if err != nil {
		t.Fatalf("error2: %v", err)
	}
	for _, f := range disc2.Findings {
		if f.Kind == AuditContractMismatch || f.Kind == AuditExecutionFailure {
			t.Fatalf("should never emit mismatch/failure even with no needles, got %v", f)
		}
	}
}

func TestDiscoverContract_ShouldSortDeterministically(t *testing.T) {
	root := t.TempDir()
	writeTraceFile(t, root, "alpha.txt", "alpha zebra")
	writeTraceFile(t, root, "beta.txt", "beta")
	in := ContractAuditInput{
		RepoRoot:         root,
		Routes:           []string{"/zebra/alpha", "/beta/gamma"},
		FormFields:       []string{"delta"},
		MutatingControls: []string{"btnB", "btnA"},
	}
	disc1, err := DiscoverContract(context.Background(), in)
	if err != nil {
		t.Fatalf("error1: %v", err)
	}
	disc2, err := DiscoverContract(context.Background(), in)
	if err != nil {
		t.Fatalf("error2: %v", err)
	}
	if len(disc1.Findings) != len(disc2.Findings) {
		t.Fatalf("findings length mismatch %d vs %d", len(disc1.Findings), len(disc2.Findings))
	}
	if !reflect.DeepEqual(disc1.Findings, disc2.Findings) {
		t.Fatalf("findings differ between runs:\n first: %+v\nsecond: %+v", disc1.Findings, disc2.Findings)
	}
	for i := 1; i < len(disc1.Findings); i++ {
		prev := disc1.Findings[i-1]
		curr := disc1.Findings[i]
		if prev.Kind > curr.Kind {
			t.Fatalf("not sorted by Kind at %d: %q > %q", i, prev.Kind, curr.Kind)
		}
		if prev.Kind == curr.Kind && prev.Subject > curr.Subject {
			t.Fatalf("not sorted by Subject at %d: %q > %q within Kind %q", i, prev.Subject, curr.Subject, prev.Kind)
		}
	}
	// also check needles sorted
	sortedNeedles := make([]string, len(disc1.Needles))
	copy(sortedNeedles, disc1.Needles)
	sort.Strings(sortedNeedles)
	for i := range disc1.Needles {
		if disc1.Needles[i] != sortedNeedles[i] {
			t.Fatalf("needles not sorted: %v", disc1.Needles)
		}
	}
}
