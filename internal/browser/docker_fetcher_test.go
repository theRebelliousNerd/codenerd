package browser

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestLookupDockerBinary_WhenNotInAllowlist_ShouldReturnEmpty(t *testing.T) {
	// Allowlist without docker; must return "" even if docker exists on PATH.
	// This asserts authorization-before-capability.
	allowlist := []string{"git", "npm", "node"}
	got := LookupDockerBinary(allowlist)
	if got != "" {
		t.Fatalf("expected empty when docker not in allowlist, got %q", got)
	}
	// Also test with similar but not equal names.
	allowlist2 := []string{"dockerd", "docker-compose"}
	got2 := LookupDockerBinary(allowlist2)
	if got2 != "" {
		t.Fatalf("expected empty for dockerd/docker-compose, got %q", got2)
	}
}

func TestLookupDockerBinary_WhenAllowlistEmpty_ShouldReturnEmpty(t *testing.T) {
	if got := LookupDockerBinary(nil); got != "" {
		t.Fatalf("expected empty for nil allowlist, got %q", got)
	}
	if got := LookupDockerBinary([]string{}); got != "" {
		t.Fatalf("expected empty for empty allowlist, got %q", got)
	}
	if got := LookupDockerBinary([]string{"", " "}); got != "" {
		t.Fatalf("expected empty for blank entries, got %q", got)
	}
}

func TestLookupDockerBinary_WhenAllowlistNamesDockerExe_ShouldBeAccepted(t *testing.T) {
	// Behaviour must not depend on whether docker is installed: if LookPath fails result is "",
	// so assert only that the allowlist entry is not the reason for rejection.
	// Structure: never panics, and result is "" only when LookPath also fails.
	got := LookupDockerBinary([]string{"docker.exe"})
	// Should not panic; check consistency with LookPath.
	_, lookErr := exec.LookPath("docker")
	if got == "" && lookErr == nil {
		t.Fatalf("expected non-empty when docker.exe allowed and docker binary present at %v, got empty", lookErr)
	}
	if got != "" && lookErr != nil {
		t.Fatalf("expected empty when docker not on PATH, got %q but LookPath failed: %v", got, lookErr)
	}
	// Also check case-insensitive: "Docker.EXE" should be accepted same way.
	got2 := LookupDockerBinary([]string{"Docker.EXE"})
	_, lookErr2 := exec.LookPath("docker")
	if got2 == "" && lookErr2 == nil {
		t.Fatalf("expected non-empty for case-insensitive docker.exe, got empty while docker present")
	}
	// Verify case-insensitive "DOCKER" also maps to same result class.
	got3 := LookupDockerBinary([]string{"DOCKER"})
	if (got == "" && got3 != "") || (got != "" && got3 == "") {
		t.Fatalf("case-insensitive mismatch: docker.exe=%q DOCKER=%q", got, got3)
	}
}

func TestNewDockerLogFetcher_WhenPathEmpty_ShouldReturnNil(t *testing.T) {
	fetcher := NewDockerLogFetcher("")
	if fetcher != nil {
		t.Fatalf("expected nil fetcher for empty path, got %v", fetcher)
	}
	// Assert nil composes end-to-end: pass nil fetcher to CorrelateContainerLogs with containers configured.
	events := []RuntimeErrorEvent{{Kind: "console_error", Detail: "detail", Timestamp: time.Now()}}
	result := CorrelateContainerLogs(context.Background(), ContainerCorrelationRequest{
		Fetcher:    fetcher,
		Containers: []string{"app"},
		Events:     events,
		Window:     time.Second,
		Redactor:   nil,
	})
	if len(result.Correlations) != 0 {
		t.Fatalf("expected no correlations with nil fetcher, got %d", len(result.Correlations))
	}
	found := false
	for _, n := range result.Notes {
		if strings.Contains(strings.ToLower(n), "no container log source is configured") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected note about no container log source is configured, got %v", result.Notes)
	}
}

func TestParseDockerLogLines_ShouldParseTimestampAndMessage(t *testing.T) {
	ts := time.Now().UTC().Truncate(time.Nanosecond)
	container := "myapp"
	raw := []byte(ts.Format(time.RFC3339Nano) + " hello world\n" + ts.Add(time.Second).Format(time.RFC3339Nano) + " second line")
	lines := parseDockerLogLines(container, raw)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %+v", len(lines), lines)
	}
	if !lines[0].Timestamp.Equal(ts) {
		t.Fatalf("expected timestamp %v, got %v", ts, lines[0].Timestamp)
	}
	if lines[0].Message != "hello world" {
		t.Fatalf("expected message %q, got %q", "hello world", lines[0].Message)
	}
	if lines[0].Container != container {
		t.Fatalf("expected container %q, got %q", container, lines[0].Container)
	}
	if lines[1].Message != "second line" {
		t.Fatalf("expected second message %q, got %q", "second line", lines[1].Message)
	}
}

func TestParseDockerLogLines_WhenCRLF_ShouldStripCarriageReturn(t *testing.T) {
	ts := time.Now().UTC().Truncate(time.Nanosecond)
	raw := []byte(ts.Format(time.RFC3339Nano) + " line one\r\n" + ts.Format(time.RFC3339Nano) + " line two\r\n")
	lines := parseDockerLogLines("c", raw)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %+v", len(lines), lines)
	}
	if lines[0].Message != "line one" {
		t.Fatalf("expected stripped CR, got %q", lines[0].Message)
	}
	if strings.Contains(lines[0].Message, "\r") {
		t.Fatalf("message still contains CR: %q", lines[0].Message)
	}
	if lines[1].Message != "line two" {
		t.Fatalf("expected second stripped, got %q", lines[1].Message)
	}
}

func TestParseDockerLogLines_WhenTimestampUnparseable_ShouldKeepLineWithZeroTime(t *testing.T) {
	raw := []byte("not-a-timestamp this is message\n2025-13-99T99:99:99Z bad timestamp\nplain line without space")
	lines := parseDockerLogLines("app", raw)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines kept, got %d: %+v", len(lines), lines)
	}
	for i, line := range lines {
		if !line.Timestamp.IsZero() {
			t.Fatalf("line %d expected zero Timestamp for unparseable, got %v", i, line.Timestamp)
		}
	}
	if lines[0].Message != "not-a-timestamp this is message" {
		t.Fatalf("expected full line preserved, got %q", lines[0].Message)
	}
	if lines[2].Message != "plain line without space" {
		// For line without space, whole line should be message. Actually our code keeps whole line when no space.
		// This line has no leading timestamp parse, but also no space? It does have spaces.
		// Ensure message survives intact.
		t.Fatalf("expected preserved message, got %q", lines[2].Message)
	}
	// Ensure container set even for unparseable.
	for _, l := range lines {
		if l.Container != "app" {
			t.Fatalf("expected container app, got %q", l.Container)
		}
	}
}

func TestParseDockerLogLines_WhenBlankLines_ShouldSkipThem(t *testing.T) {
	ts := time.Now().UTC().Truncate(time.Nanosecond)
	raw := []byte("\n" + ts.Format(time.RFC3339Nano) + " keep\n\n\n" + ts.Format(time.RFC3339Nano) + " also keep\n\n")
	lines := parseDockerLogLines("app", raw)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines skipping blanks, got %d: %+v", len(lines), lines)
	}
	if lines[0].Message != "keep" || lines[1].Message != "also keep" {
		t.Fatalf("unexpected messages: %+v", lines)
	}
}

func TestParseDockerLogLines_ShouldSetContainerOnEveryLine(t *testing.T) {
	ts := time.Now().UTC().Truncate(time.Nanosecond)
	raw := []byte(ts.Format(time.RFC3339Nano) + " a\n" + "bad line without timestamp\n" + ts.Format(time.RFC3339Nano) + " b\r\n")
	lines := parseDockerLogLines("expected-container", raw)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	for i, l := range lines {
		if l.Container != "expected-container" {
			t.Fatalf("line %d expected container %q, got %q", i, "expected-container", l.Container)
		}
	}
}

func TestNewDockerLogFetcher_WhenContainerNameInvalid_ShouldError(t *testing.T) {
	// Use a dummy path; validation happens before exec, so this tests the guard without needing docker.
	fetcher := NewDockerLogFetcher("docker")
	if fetcher == nil {
		t.Fatal("expected non-nil fetcher for dummy path")
	}
	ctx := context.Background()
	since := time.Now().Add(-time.Minute)
	until := time.Now()

	if _, err := fetcher(ctx, "", since, until); err == nil || !strings.Contains(err.Error(), `""`) {
		t.Fatalf("expected error naming empty container, got %v", err)
	}
	if _, err := fetcher(ctx, "-evil", since, until); err == nil || !strings.Contains(err.Error(), "-evil") {
		t.Fatalf("expected error naming dash-prefixed container, got %v", err)
	}
	// Valid container with dummy path will fail exec, but should include container name in error.
	if _, err := fetcher(ctx, "valid", since, until); err == nil || !strings.Contains(err.Error(), "valid") {
		t.Fatalf("expected error naming container on exec failure, got %v", err)
	}
}

 // TestParseDockerLogLines_ShouldParseRealDockerTimestampFormat pins the real
// Docker wire format. Docker emits a fixed nine-digit nanosecond field (e.g.
// .100000000Z) while time.RFC3339Nano elides trailing zeros (e.g. .1Z), so a
// round trip through Go's formatter cannot prove compatibility with the real
// docker logs stream. This test therefore uses string literals copied verbatim
// from Docker 29.6.1 output rather than time.Format.
func TestParseDockerLogLines_ShouldParseRealDockerTimestampFormat(t *testing.T) {
	container := "myapp"
	// String literals copied verbatim from real Docker output — never time.Format —
	// so the test pins the wire format rather than Go's formatting of it.
	raw := []byte("2026-08-16T18:45:07.175445217Z seaweed volume server started\n" +
		"2026-08-16T18:45:12.175925082Z heartbeat ok\n" +
		"2026-08-16T18:45:20.100000000Z tick")
	lines := parseDockerLogLines(container, raw)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %+v", len(lines), lines)
	}
	tests := []struct {
		wantMsg  string
		wantTime time.Time
	}{
		{
			wantMsg:  "seaweed volume server started",
			wantTime: time.Date(2026, time.August, 16, 18, 45, 7, 175445217, time.UTC),
		},
		{
			wantMsg:  "heartbeat ok",
			wantTime: time.Date(2026, time.August, 16, 18, 45, 12, 175925082, time.UTC),
		},
		{
			wantMsg:  "tick",
			wantTime: time.Date(2026, time.August, 16, 18, 45, 20, 100000000, time.UTC),
		},
	}
	for i, tc := range tests {
		if lines[i].Container != container {
			t.Fatalf("line %d: expected container %q, got %q", i, container, lines[i].Container)
		}
		if lines[i].Message != tc.wantMsg {
			t.Fatalf("line %d: expected message %q, got %q", i, tc.wantMsg, lines[i].Message)
		}
		if !lines[i].Timestamp.Equal(tc.wantTime) {
			t.Fatalf("line %d: expected timestamp %v, got %v", i, tc.wantTime, lines[i].Timestamp)
		}
	}
	// Specifically assert the trailing-zero line's nanosecond field so a parser
	// that mishandled a fixed-width fraction would fail rather than silently round.
	if got := lines[2].Timestamp.Nanosecond(); got != 100000000 {
		t.Fatalf("trailing-zero line: expected Nanosecond() == 100000000, got %d (timestamp %v)", got, lines[2].Timestamp)
	}
}

