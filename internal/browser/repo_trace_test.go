package browser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	browsersecurity "codenerd/internal/browser/security"
)

func writeTraceFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		t.Fatalf("mkdir %q: %v", full, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write %q: %v", full, err)
	}
}

func containsNote(notes []string, sub string) bool {
	for _, n := range notes {
		if strings.Contains(strings.ToLower(n), strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

func TestTraceRepository_WhenNeedleInFile_ShouldReturnRelativePathAndLine(t *testing.T) {
	root := t.TempDir()
	writeTraceFile(t, root, "sub/file.txt", "first line\nhello needle world\nsecond\n")
	res, err := TraceRepository(context.Background(), root, []string{"needle"}, RepoTraceLimits{})
	if err != nil {
		t.Fatalf("TraceRepository error = %v", err)
	}
	if len(res.Matches) == 0 {
		t.Fatal("expected at least one match")
	}
	m := res.Matches[0]
	if filepath.IsAbs(m.Path) {
		t.Fatalf("Path is absolute %q", m.Path)
	}
	if strings.Contains(m.Path, root) {
		t.Fatalf("Path discloses root %q contains %q", m.Path, root)
	}
	if !strings.Contains(m.Path, "file.txt") {
		t.Fatalf("Path %q should contain file.txt", m.Path)
	}
	if m.Line != 2 {
		t.Fatalf("Line = %d, want 2", m.Line)
	}
	if m.Needle != "needle" {
		t.Fatalf("Needle = %q, want %q", m.Needle, "needle")
	}
}

func TestTraceRepository_WhenNoNeedles_ShouldError(t *testing.T) {
	root := t.TempDir()
	writeTraceFile(t, root, "a.txt", "hello")
	_, err := TraceRepository(context.Background(), root, nil, RepoTraceLimits{})
	if err == nil {
		t.Fatal("expected error for no needles")
	}
	_, err = TraceRepository(context.Background(), root, []string{"   "}, RepoTraceLimits{})
	if err == nil {
		t.Fatal("expected error for empty needle")
	}
}

func TestTraceRepository_WhenRootMissing_ShouldError(t *testing.T) {
	_, err := TraceRepository(context.Background(), "", []string{"needle"}, RepoTraceLimits{})
	if err == nil {
		t.Fatal("expected error for empty root")
	}
	badRoot := filepath.Join(t.TempDir(), "nope")
	_, err = TraceRepository(context.Background(), badRoot, []string{"needle"}, RepoTraceLimits{})
	if err == nil {
		t.Fatal("expected error for missing root")
	}
	fileRoot := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(fileRoot, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = TraceRepository(context.Background(), fileRoot, []string{"needle"}, RepoTraceLimits{})
	if err == nil {
		t.Fatal("expected error for file as root")
	}
}

func TestTraceRepository_WhenBinaryFile_ShouldSkip(t *testing.T) {
	root := t.TempDir()
	content := "needle here\x00 and more"
	writeTraceFile(t, root, "bin.dat", content)
	res, err := TraceRepository(context.Background(), root, []string{"needle"}, RepoTraceLimits{})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(res.Matches) != 0 {
		t.Fatalf("expected 0 matches for binary, got %d", len(res.Matches))
	}
	if res.Matches == nil {
		t.Fatal("Matches should be non-nil empty slice")
	}
}

func TestTraceRepository_WhenFileExceedsByteLimit_ShouldTruncateAndNote(t *testing.T) {
	root := t.TempDir()
	limit := 100
	over := strings.Repeat("a", limit+50)
	content := "needle at start\n" + over
	writeTraceFile(t, root, "big.txt", content)
	res, err := TraceRepository(context.Background(), root, []string{"needle"}, RepoTraceLimits{MaxFileBytes: limit})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !containsNote(res.Notes, "truncated file") {
		t.Fatalf("expected truncated file note, got %v", res.Notes)
	}
	if len(res.Matches) == 0 {
		t.Fatal("expected match in truncated prefix")
	}
}

func TestTraceRepository_WhenMatchesExceedLimit_ShouldTruncateAndNote(t *testing.T) {
	root := t.TempDir()
	var b strings.Builder
	for i := 0; i < 10; i++ {
		b.WriteString("needle line\n")
	}
	writeTraceFile(t, root, "many.txt", b.String())
	res, err := TraceRepository(context.Background(), root, []string{"needle"}, RepoTraceLimits{MaxMatches: 3})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !res.Truncated {
		t.Fatal("expected Truncated true")
	}
	if !containsNote(res.Notes, "truncated matches") {
		t.Fatalf("expected truncated matches note, got %v", res.Notes)
	}
	if len(res.Matches) != 3 {
		t.Fatalf("Matches = %d, want 3", len(res.Matches))
	}
}

func TestTraceRepository_WhenLimitAboveCeiling_ShouldClampAndNote(t *testing.T) {
	root := t.TempDir()
	var b strings.Builder
	for i := 0; i < 150; i++ {
		b.WriteString("needle\n")
	}
	writeTraceFile(t, root, "clamp.txt", b.String())
	huge := 10000
	res, err := TraceRepository(context.Background(), root, []string{"needle"}, RepoTraceLimits{MaxMatches: huge})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !containsNote(res.Notes, "clamped") {
		t.Fatalf("expected clamp note, got %v", res.Notes)
	}
	if len(res.Matches) > maxRepoTraceMatches {
		t.Fatalf("Matches %d exceeds ceiling %d", len(res.Matches), maxRepoTraceMatches)
	}
	if len(res.Matches) != maxRepoTraceMatches {
		t.Fatalf("Matches = %d, want ceiling %d", len(res.Matches), maxRepoTraceMatches)
	}
}

func TestTraceRepository_WhenZeroLimits_ShouldUseDefaultsNotUnlimited(t *testing.T) {
	root := t.TempDir()
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("zero needle\n")
	}
	writeTraceFile(t, root, "zero.txt", b.String())
	res, err := TraceRepository(context.Background(), root, []string{"zero"}, RepoTraceLimits{})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !res.Truncated {
		t.Fatal("zero limits should still truncate at default")
	}
	if len(res.Matches) != maxRepoTraceMatches {
		t.Fatalf("zero limits Matches = %d, want %d", len(res.Matches), maxRepoTraceMatches)
	}
	if !containsNote(res.Notes, "truncated matches") {
		t.Fatalf("expected truncation note with zero limits, got %v", res.Notes)
	}
}

func TestTraceRepository_WhenSkipDirectory_ShouldIgnoreNodeModules(t *testing.T) {
	root := t.TempDir()
	writeTraceFile(t, root, "node_modules/hidden.txt", "needle inside node_modules")
	writeTraceFile(t, root, "src/visible.txt", "needle visible")
	res, err := TraceRepository(context.Background(), root, []string{"needle"}, RepoTraceLimits{})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	for _, m := range res.Matches {
		if strings.Contains(m.Path, "node_modules") {
			t.Fatalf("should have skipped node_modules, got %q", m.Path)
		}
	}
	found := false
	for _, m := range res.Matches {
		if strings.Contains(m.Path, "visible.txt") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected visible.txt match")
	}
}

func TestTraceRepository_WhenSecretInMatchedLine_ShouldRedact(t *testing.T) {
	secretRaw := "password=hunter2"
	red := browsersecurity.NewRedactor(nil)
	sanitized := red.SanitizeString(secretRaw)
	if !strings.Contains(sanitized, browsersecurity.Redacted) {
		t.Fatalf("redactor does not catch %q -> %q", secretRaw, sanitized)
	}
	if strings.Contains(sanitized, "hunter2") {
		t.Fatalf("redactor failed to redact hunter2: %q", sanitized)
	}
	root := t.TempDir()
	line := "findme " + secretRaw + " end"
	writeTraceFile(t, root, "secret.txt", line)
	res, err := TraceRepository(context.Background(), root, []string{"findme"}, RepoTraceLimits{})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(res.Matches) == 0 {
		t.Fatal("expected match")
	}
	snippet := res.Matches[0].Snippet
	if strings.Contains(snippet, "hunter2") {
		t.Fatalf("snippet not redacted: %q", snippet)
	}
	if !strings.Contains(snippet, browsersecurity.Redacted) {
		t.Fatalf("snippet missing redaction marker: %q", snippet)
	}
}

func TestTraceRepository_WhenContextCancelled_ShouldStopEarly(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		writeTraceFile(t, root, filepath.Join("dir", "file.txt"), strings.Repeat("needle\n", 50))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := TraceRepository(ctx, root, []string{"needle"}, RepoTraceLimits{})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !containsNote(res.Notes, "context cancelled") && !containsNote(res.Notes, "cancelled") {
		t.Fatalf("expected cancellation note, got %v", res.Notes)
	}
}

func TestTraceRepository_ShouldSortDeterministically(t *testing.T) {
	root := t.TempDir()
	writeTraceFile(t, root, "b.txt", "needle b1\nneedle b2\n")
	writeTraceFile(t, root, "a.txt", "needle a1\nneedle a2\n")
	first, err := TraceRepository(context.Background(), root, []string{"needle"}, RepoTraceLimits{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := TraceRepository(context.Background(), root, []string{"needle"}, RepoTraceLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Matches) != len(second.Matches) {
		t.Fatalf("length mismatch %d vs %d", len(first.Matches), len(second.Matches))
	}
	for i := range first.Matches {
		if first.Matches[i] != second.Matches[i] {
			t.Fatalf("mismatch at %d: %+v vs %+v", i, first.Matches[i], second.Matches[i])
		}
	}
	for i := 1; i < len(first.Matches); i++ {
		prev := first.Matches[i-1]
		curr := first.Matches[i]
		if prev.Path > curr.Path {
			t.Fatalf("not sorted by path: %q > %q", prev.Path, curr.Path)
		}
		if prev.Path == curr.Path && prev.Line > curr.Line {
			t.Fatalf("not sorted by line: %d > %d", prev.Line, curr.Line)
		}
	}
}

func TestTraceRepository_WhenNeedleHasRegexMetacharacters_ShouldTreatAsLiteral(t *testing.T) {
	root := t.TempDir()
	writeTraceFile(t, root, "a.txt", "ab\n")
	writeTraceFile(t, root, "b.txt", "a(b\n")
	res, err := TraceRepository(context.Background(), root, []string{"a(b"}, RepoTraceLimits{})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(res.Matches) != 1 {
		t.Fatalf("Matches = %d, want 1", len(res.Matches))
	}
	if !strings.Contains(res.Matches[0].Path, "b.txt") {
		t.Fatalf("expected b.txt, got %q", res.Matches[0].Path)
	}
}

func TestTraceRepository_WhenSymlinkEscapesRoot_ShouldNotFollow(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	writeTraceFile(t, outside, "outside.txt", "needle outside")
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	writeTraceFile(t, root, "inside.txt", "needle inside")
	res, err := TraceRepository(context.Background(), root, []string{"needle"}, RepoTraceLimits{})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	for _, m := range res.Matches {
		if strings.Contains(m.Path, "outside") {
			t.Fatalf("should not follow symlink escape, got %q", m.Path)
		}
		if strings.Contains(m.Path, "escape") {
			t.Fatalf("should not return escape symlink path %q", m.Path)
		}
	}
	foundInside := false
	for _, m := range res.Matches {
		if strings.Contains(m.Path, "inside.txt") {
			foundInside = true
		}
	}
	if !foundInside {
		t.Fatal("expected inside.txt match")
	}
}
