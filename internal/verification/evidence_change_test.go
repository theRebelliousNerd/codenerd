package verification

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"codenerd/internal/observation"
)

// The two removal witnesses go through the observed executor, real packet
// builder, actual judge requests, reply validation and production policy.
// Before the repair, deletion supplied only a marker; a surviving file's
// omitted diff expanded into a postimage that could never contain the sentinel.
func TestVerifyWithRetry_RetainedRemovedTextReachesTheJudge(t *testing.T) {
	const sentinel = "REMOVED_REVIEW_SENTINEL"
	var before, after strings.Builder
	for i := 0; i < 24; i++ {
		fmt.Fprintf(&before, "old decision %03d\n", i)
		fmt.Fprintf(&after, "new decision %03d\n", i)
		for j := 0; j < 10; j++ {
			line := fmt.Sprintf("unchanged context %03d %02d keeps hunks separate\n", i, j)
			before.WriteString(line)
			after.WriteString(line)
		}
	}
	before.WriteString(sentinel + "\n")
	for _, tc := range []struct {
		name     string
		after    string
		exists   bool
		preimage string
		elided   bool
	}{
		{"deleted", "", false, before.String(), true},
		{"modified with nonempty postimage", after.String(), true, before.String(), true},
		{"empty surviving postimage", "", true, before.String(), true},
		{"small complete deletion", "", false, sentinel + "\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "change.txt")
			if tc.exists {
				if err := os.WriteFile(path, []byte(tc.after), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			write := observation.FileWrite{Path: path,
				Before: observation.FileState{Known: true, Exists: true, Content: tc.preimage},
				After:  observation.FileState{Known: true, Exists: tc.exists, Content: tc.after},
			}
			const budget = 4096
			var users []string
			client := &stubLLMClient{completeWithSystem: func(_ context.Context, _, user string) (string, error) {
				part := strings.TrimPrefix(user, "## Task\nreview the removal")
				if len(users) > 0 {
					part = strings.TrimPrefix(strings.TrimPrefix(user, users[len(users)-1]), "\n\n")
				}
				if len(part) > budget || !utf8.ValidString(part) {
					t.Fatalf("judge received invalid or over-budget evidence (%d bytes)", len(part))
				}
				users = append(users, user)
				if len(users) == 1 && tc.elided && strings.Contains(user, sentinel) {
					t.Fatal("fixture did not put the removed sentinel beyond initial admission")
				}
				if strings.Contains(user, sentinel) {
					return judgeSays, nil
				}
				return `{"contract":"judge/v2","status":"insufficient_evidence","confidence":0.5,"explanation":"need the removed detail","requested_evidence":["E2"]}`, nil
			}}
			exec := &stubTaskExecutor{result: "done", outcome: "/done", writes: []observation.FileWrite{write}, changed: []string{"change.txt"}, workspace: dir}
			v := newDelegationVerifier(t, client, nil, exec)
			d := delegation("review the removal", 3)
			limit, rounds := budget, 5
			d.Config.JudgeEvidenceBytes, d.Config.JudgeEvidenceRounds = &limit, &rounds
			if problems := d.Config.Check("delegation"); len(problems) > 0 {
				t.Fatalf("fixture must use supported production configuration: %v", problems)
			}
			_, verdict, err := v.VerifyWithRetry(context.Background(), d)
			if err != nil || verdict == nil || !verdict.Accepted {
				t.Fatalf("removal evidence did not reach an accepted judgment: %v (%+v)", err, verdict)
			}
			if exec.calls != 1 || (tc.elided && len(users) < 2) || (!tc.elided && len(users) != 1) {
				t.Fatalf("worker runs=%d judge rounds=%d, elided=%t", exec.calls, len(users), tc.elided)
			}
			last := users[len(users)-1]
			if strings.Count(last, sentinel) != 1 || !strings.Contains(last, "representation=diff") {
				t.Fatal("the judge must see the removed text exactly once under the diff identity")
			}
			it, _ := verdict.Evidence.item("E2")
			if tc.elided && (it.Omitted == 0 || !strings.Contains(last, "diff of changed change.txt")) {
				t.Fatal("the fixture must exercise a bounded diff and named diff expansion")
			}
			if tc.exists && tc.after != "" && strings.Count(renderDiff("change.txt", tc.preimage, tc.after), "@@ -") < 2 {
				t.Fatal("the surviving-file witness must contain multiple hunks")
			}
		})
	}
}

// Requests for absent source detail stay named limitations, without a
// fabricated preimage or a retry of the worker. Empty and absent are known
// states; unknown is not a synonym for either of them.
func TestVerifyWithRetry_ChangeRepresentationStates(t *testing.T) {
	for _, tc := range []struct {
		name           string
		before, after  observation.FileState
		representation string
		want           string
	}{
		{"unknown deleted preimage", observation.FileState{}, observation.FileState{Known: true}, "unavailable", "the deleted file's preimage is unknown"},
		{"unknown surviving preimage", observation.FileState{}, observation.FileState{Known: true, Exists: true, Content: "final text\n"}, "postimage", "final text"},
		{"empty deleted preimage", observation.FileState{Known: true, Exists: true}, observation.FileState{Known: true}, "diff", "deleted an empty file"},
		{"absent both times", observation.FileState{Known: true}, observation.FileState{Known: true}, "diff", "absent before and after"},
		{"new text file", observation.FileState{Known: true}, observation.FileState{Known: true, Exists: true, Content: "new text\n"}, "diff", "+new text"},
		{"unknown final state", observation.FileState{Known: true, Exists: true, Content: "old text\n"}, observation.FileState{}, "unavailable", "could not be read"},
		{"binary deleted preimage", observation.FileState{Known: true, Exists: true, Content: "binary\x00secret"}, observation.FileState{Known: true}, "binary", "binary content has no retained text representation"},
		{"binary postimage", observation.FileState{Known: true}, observation.FileState{Known: true, Exists: true, Content: strings.Repeat("binary\x00secret", 1000)}, "binary", "binary content has no retained text representation"},
		{"invalid UTF8 preimage", observation.FileState{Known: true, Exists: true, Content: "\xff"}, observation.FileState{Known: true}, "binary", "binary content has no retained text representation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "state.txt")
			if tc.after.Exists {
				if err := os.WriteFile(path, []byte(tc.after.Content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			client, _, users := scriptedJudge(`{"contract":"judge/v2","status":"insufficient_evidence","confidence":0.5,"explanation":"need more source detail","requested_evidence":["E2"]}`)
			exec := &stubTaskExecutor{result: "done", outcome: "/done", workspace: dir, writes: []observation.FileWrite{{Path: path, Before: tc.before, After: tc.after}}}
			v := newDelegationVerifier(t, client, nil, exec)
			_, verdict, err := v.VerifyWithRetry(context.Background(), delegation("review the change", 3))
			if !errors.Is(err, ErrVerificationUnavailable) || verdict == nil || verdict.Accepted || exec.calls != 1 || len(*users) != 1 {
				t.Fatalf("unavailable detail: err=%v verdict=%+v worker runs=%d judge rounds=%d", err, verdict, exec.calls, len(*users))
			}
			it, _ := verdict.Evidence.item("E2")
			if it.Representation != tc.representation || !strings.Contains((*users)[0]+verdict.Reason, tc.want) {
				t.Fatalf("representation=%s, request=%q, reason=%q", it.Representation, (*users)[0], verdict.Reason)
			}
			if strings.Contains((*users)[0], "\x00") {
				t.Fatal("binary bytes entered the judge request")
			}
		})
	}
}

// Range offsets count the selected representation's bytes. Even a single
// line longer than a disclosure budget must progress without cutting a rune
// or switching away from the removed text.
func TestEvidencePacket_RemovedUnicodeLongLineIsRecoverable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unicode.txt")
	before := strings.Repeat("界é", 4000) + "\n"
	ret := observation.Return{Outcome: "/done", Workspace: dir, Writes: []observation.FileWrite{{
		Path: path, Before: observation.FileState{Known: true, Exists: true, Content: before}, After: observation.FileState{Known: true},
	}}}
	p, err := buildEvidencePacket(context.Background(), 1, "/implementation", ret, 2048)
	if err != nil {
		t.Fatal(err)
	}
	it, _ := p.item("E2")
	shown := it.Body
	if len(p.render(""))-len("## Task\n") > p.Budget || !utf8.ValidString(shown) {
		t.Fatal("initial admission exceeded the budget or cut a rune")
	}
	for rounds := 0; p.expandable(it.ID); rounds++ {
		if rounds >= 32 {
			t.Fatal("long-line disclosure stopped progressing")
		}
		chunk := p.expand([]string{it.ID})
		if len(chunk) > p.Budget || !utf8.ValidString(chunk) {
			t.Fatal("range exceeded the budget or cut a rune")
		}
		parts := strings.SplitN(chunk, "\n```\n", 3)
		if len(parts) != 3 {
			t.Fatalf("disclosure has no framed range: %q", chunk)
		}
		shown += parts[1]
	}
	if want := renderDiff(path, before, ""); shown != want {
		t.Fatalf("removed Unicode diff lost or repeated bytes: got %d, want %d", len(shown), len(want))
	}
}

func TestVerifyWithRetry_RecreatedDeletedPathInvalidatesRemovalEvidence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deleted.txt")
	write := observation.FileWrite{Path: path, Before: observation.FileState{Known: true, Exists: true, Content: strings.Repeat("removed text\n", 2000)}, After: observation.FileState{Known: true}}
	calls := 0
	client := &stubLLMClient{completeWithSystem: func(context.Context, string, string) (string, error) {
		calls++
		if calls == 1 {
			return `{"contract":"judge/v2","status":"insufficient_evidence","confidence":0.5,"explanation":"need the deletion diff","requested_evidence":["E2"]}`, nil
		}
		if err := os.WriteFile(path, []byte("recreated\n"), 0o644); err != nil {
			return "", err
		}
		return judgeSays, nil
	}}
	exec := &stubTaskExecutor{result: "done", outcome: "/done", workspace: dir, writes: []observation.FileWrite{write}}
	v := newDelegationVerifier(t, client, nil, exec)
	d := delegation("review the deletion", 3)
	budget := 4096
	d.Config.JudgeEvidenceBytes = &budget
	_, verdict, err := v.VerifyWithRetry(context.Background(), d)
	if !errors.Is(err, ErrVerificationUnavailable) || verdict == nil || verdict.Accepted || calls != 2 || exec.calls != 1 {
		t.Fatalf("recreated deletion: err=%v verdict=%+v judge rounds=%d worker runs=%d", err, verdict, calls, exec.calls)
	}
	if verdict.Evidence.Applicability != ApplicabilitySuperseded {
		t.Fatalf("applicability = %s, want superseded", verdict.Evidence.Applicability)
	}
}
