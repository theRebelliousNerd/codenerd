package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The delete_lines branch of LineEditValidator was unreachable in production
// and vacuous when reached: it read start_line/end_line with a bare .(int)
// assertion (every payload arriving from the kernel or a tool call has been
// through JSON, so those are float64 — handleDeleteLines itself reads float64),
// and the only check it ever ran was len(strings.Split(content, "\n")) == 0,
// which no string can satisfy. Every delete therefore came back verified.

func writeDeleteFixture(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.txt")
	body := ""
	if len(lines) > 0 {
		body = strings.Join(lines, "\n") + "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func deleteRequest(path string, start, end float64) ActionRequest {
	return ActionRequest{
		Type:   ActionDeleteLines,
		Target: path,
		// float64, as JSON decoding produces and handleDeleteLines reads.
		Payload: map[string]any{"start_line": start, "end_line": end},
	}
}

func deleteResult(path string, linesDeleted int) ActionResult {
	return ActionResult{
		Success:  true,
		Metadata: map[string]any{"path": path, "lines_deleted": linesDeleted},
	}
}

// A JSON-shaped payload must reach the delete checks at all. Before the fix the
// int assertion failed, the branch was skipped, and the generic tail returned
// Verified: true, Confidence: 0.85 with no delete-specific detail.
func TestLineEditValidator_DeleteLines_ValidatesJSONNumberPayload(t *testing.T) {
	path := writeDeleteFixture(t, "a", "b", "e")
	v := NewLineEditValidator()

	res := v.Validate(context.Background(), deleteRequest(path, 3, 4), deleteResult(path, 2))

	if !res.Verified {
		t.Fatalf("valid delete reported unverified: %+v", res)
	}
	if _, ok := res.Details["actual_deleted"]; !ok {
		t.Errorf("delete-specific checks did not run for a float64 payload: %+v", res.Details)
	}
	if res.Confidence != 0.9 {
		t.Errorf("confidence %v does not reflect a count-and-floor check", res.Confidence)
	}
}

// A delete that removed nothing (range entirely past EOF) used to verify.
func TestLineEditValidator_DeleteLines_NoOpIsNotVerified(t *testing.T) {
	path := writeDeleteFixture(t, "a", "b", "c")
	v := NewLineEditValidator()

	res := v.Validate(context.Background(), deleteRequest(path, 90, 95), deleteResult(path, 0))

	if res.Verified {
		t.Fatalf("delete that removed 0 lines was verified: %+v", res)
	}
	if !strings.Contains(res.Error, "removed nothing") {
		t.Errorf("error should say nothing was deleted, got: %q", res.Error)
	}
}

// The untouched prefix is the floor: lines before start_line cannot disappear.
func TestLineEditValidator_DeleteLines_OverDeletionIsCaught(t *testing.T) {
	// Requested 5-6, but only one line survives — lines 1-4 were not in range.
	path := writeDeleteFixture(t, "a")
	v := NewLineEditValidator()

	res := v.Validate(context.Background(), deleteRequest(path, 5, 6), deleteResult(path, 2))

	if res.Verified {
		t.Fatalf("delete that ate lines outside the range was verified: %+v", res)
	}
	if !strings.Contains(res.Error, "more than the requested range") {
		t.Errorf("error should name over-deletion, got: %q", res.Error)
	}
}

// A range running off the end of the file is legal — the editor clamps it — but
// only if the file was truncated exactly at start_line.
func TestLineEditValidator_DeleteLines_ClampedAtEOFIsVerified(t *testing.T) {
	path := writeDeleteFixture(t, "a", "b")
	v := NewLineEditValidator()

	res := v.Validate(context.Background(), deleteRequest(path, 3, 9), deleteResult(path, 4))

	if !res.Verified {
		t.Fatalf("EOF-clamped delete reported unverified: %+v", res)
	}
	if res.Details["clamped_at_eof"] != true {
		t.Errorf("clamp should be recorded in details: %+v", res.Details)
	}
}

// Same clamp shape, but the file does not agree with the reported count.
func TestLineEditValidator_DeleteLines_ClampCountMustMatchFile(t *testing.T) {
	path := writeDeleteFixture(t, "a", "b", "c", "d", "e")
	v := NewLineEditValidator()

	// Reported a short delete (only possible at EOF) yet five lines remain.
	res := v.Validate(context.Background(), deleteRequest(path, 3, 9), deleteResult(path, 4))

	if res.Verified {
		t.Fatalf("count/file disagreement was verified: %+v", res)
	}
	if !strings.Contains(res.Error, "only happens at EOF") {
		t.Errorf("error should explain the contradiction, got: %q", res.Error)
	}
}

// Reporting more deleted lines than the range holds is a handler bug.
func TestLineEditValidator_DeleteLines_ExcessCountIsCaught(t *testing.T) {
	path := writeDeleteFixture(t, "a", "b", "c")
	v := NewLineEditValidator()

	res := v.Validate(context.Background(), deleteRequest(path, 2, 2), deleteResult(path, 7))

	if res.Verified {
		t.Fatalf("impossible delete count was verified: %+v", res)
	}
}

// Without lines_deleted only the floor can be checked, and the confidence must
// say so rather than claiming a count check that never ran.
func TestLineEditValidator_DeleteLines_MissingCountLowersConfidence(t *testing.T) {
	path := writeDeleteFixture(t, "a", "b", "c")
	v := NewLineEditValidator()

	res := v.Validate(context.Background(), deleteRequest(path, 2, 2),
		ActionResult{Success: true, Metadata: map[string]any{"path": path}})

	if !res.Verified {
		t.Fatalf("floor check should still verify: %+v", res)
	}
	if res.Confidence >= 0.9 {
		t.Errorf("confidence %v claims more than the floor check proved", res.Confidence)
	}
}
