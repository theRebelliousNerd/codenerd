package verification

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/evidence"
	"codenerd/internal/observation"
)

// V1 (Docs/audits/verification-applicability-and-progressive-evidence.md): a
// pass cites nothing, so grounding never checked its freshness, and a pass
// over a tree another writer changed during the judge call was Accepted.
// Applicability is measured after every round, whatever the judge said.
func TestVerifyWithRetry_APassOverASupersededWorkspaceIsNotAccepted(t *testing.T) {
	for name, writer := range map[string]func(dir string, write observation.FileWrite) error{
		// The changed file itself is replaced while the judge reads.
		"the changed file": func(_ string, w observation.FileWrite) error {
			return os.WriteFile(w.Path, []byte("package d\n\nfunc D() int { return 2 }\n"), 0o644)
		},
		// A file the attempt never touched -- a dependency -- changes while
		// the one it wrote stays as it was left.
		"an unchanged dependency": func(dir string, _ observation.FileWrite) error {
			return os.WriteFile(filepath.Join(dir, "dep.go"), []byte("package d\n\nconst Dep = 2\n"), 0o644)
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "dep.go"), []byte("package d\n\nconst Dep = 1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			write := changedFile(t, dir, "d.go", "package d\n\nfunc D() int { return 1 }\n")
			snap, err := evidence.Snapshot(context.Background(), dir)
			if err != nil {
				t.Fatal(err)
			}
			exec := &stubTaskExecutor{result: "done", outcome: "/done", writes: []observation.FileWrite{write}, changed: []string{"d.go"}, snapshot: snap, workspace: dir}
			client := &stubLLMClient{completeWithSystem: func(context.Context, string, string) (string, error) {
				if err := writer(dir, write); err != nil {
					return "", err
				}
				return judgeSays, nil
			}}
			v := newDelegationVerifier(t, client, nil, exec)

			_, verdict, err := v.VerifyWithRetry(context.Background(), delegation("implement D", 3))
			if !errors.Is(err, ErrVerificationUnavailable) {
				t.Fatalf("VerifyWithRetry = %v, want unverifiable for a superseded workspace", err)
			}
			if verdict == nil || verdict.Accepted {
				t.Fatalf("verdict = %#v, want not accepted", verdict)
			}
			if exec.calls != 1 {
				t.Fatalf("ran %d attempts; a concurrent writer is not the worker's defect", exec.calls)
			}
			if verdict.Judgment == nil || verdict.Judgment.Status != JudgePass {
				t.Fatalf("judgment = %+v, want the judge's pass kept as history", verdict.Judgment)
			}
			if verdict.Evidence.Applicability != ApplicabilitySuperseded || !strings.Contains(verdict.Reason, "no longer describes the workspace") {
				t.Fatalf("applicability %q, reason %q; want superseded and said so", verdict.Evidence.Applicability, verdict.Reason)
			}
		})
	}
}

// Evidence whose currency cannot be established is not current either.
func TestVerifyWithRetry_UnknownApplicabilityIsNotAccepted(t *testing.T) {
	client, _ := judgeClient(judgeSays)
	exec := &stubTaskExecutor{result: "done", outcome: "/done", workspace: filepath.Join(t.TempDir(), "gone")}
	v := newDelegationVerifier(t, client, nil, exec)

	_, verdict, err := v.VerifyWithRetry(context.Background(), delegation("implement E", 3))
	if !errors.Is(err, ErrVerificationUnavailable) || verdict.Accepted {
		t.Fatalf("VerifyWithRetry = %v (accepted %t), want unverifiable", err, verdict.Accepted)
	}
	if verdict.Evidence.Applicability != ApplicabilityUnknown || !strings.Contains(verdict.Reason, "could not be established") {
		t.Fatalf("applicability %q, reason %q; want unknown and said so", verdict.Evidence.Applicability, verdict.Reason)
	}
	if exec.calls != 1 {
		t.Fatalf("ran %d attempts", exec.calls)
	}
}

// The control: an untouched workspace is current, and the acceptance names
// the snapshot it was current for.
func TestVerifyWithRetry_ACurrentWorkspaceIsAcceptedAndNamed(t *testing.T) {
	dir := t.TempDir()
	write := changedFile(t, dir, "f.go", "package f\n")
	snap, err := evidence.Snapshot(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	client, _ := judgeClient(judgeSays)
	exec := &stubTaskExecutor{result: "done", outcome: "/done", writes: []observation.FileWrite{write}, snapshot: snap, workspace: dir}
	v := newDelegationVerifier(t, client, nil, exec)

	_, verdict, err := v.VerifyWithRetry(context.Background(), delegation("implement F", 1))
	if err != nil || !verdict.Accepted {
		t.Fatalf("VerifyWithRetry = %v, want accepted", err)
	}
	if !strings.Contains(verdict.Reason, snap) {
		t.Fatalf("reason %q does not name the snapshot %s", verdict.Reason, snap)
	}
}
