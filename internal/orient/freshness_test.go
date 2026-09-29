package orient

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_AUTHOR_NAME=Orientation fixture", "GIT_AUTHOR_EMAIL=orientation@example.invalid", "GIT_COMMITTER_NAME=Orientation fixture", "GIT_COMMITTER_EMAIL=orientation@example.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestRefreshStalenessPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, head, digest string
		stale              bool
	}{
		{"unchanged", "old", "one", false}, {"new commit", "new", "one", true}, {"edited document", "old", "two", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultOrientConfig()
			e, err := NewEngine(&cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			if err := e.Assert([]types.Fact{
				{Predicate: "oriented_head", Args: []any{"old"}}, {Predicate: "current_repo_head", Args: []any{tc.head}},
				{Predicate: "oriented_document", Args: []any{"DESIGN.md", "one"}}, {Predicate: "current_document", Args: []any{"DESIGN.md", tc.digest}},
			}); err != nil {
				t.Fatal(err)
			}
			if err := e.Evaluate(context.Background()); err != nil {
				t.Fatal(err)
			}
			rows, err := e.Query("orient_stale")
			if err != nil {
				t.Fatal(err)
			}
			if (len(rows) > 0) != tc.stale {
				t.Fatalf("stale rows=%v want=%v", rows, tc.stale)
			}
		})
	}
}

func TestRefreshHistoryDeltaPreservesDays(t *testing.T) {
	base, _, err := factsFromGitLog(strings.NewReader(commitMarker+"aaaa|1767225600\nA\tDESIGN.md\n"), false)
	if err != nil {
		t.Fatal(err)
	}
	delta, _, err := factsFromGitLog(strings.NewReader(commitMarker+"bbbb|1767225601\nM\tDESIGN.md\nA\tnew.go\n"), false)
	if err != nil {
		t.Fatal(err)
	}
	rows := mergeHistory(base, delta)
	for _, row := range rows {
		if row.Predicate == "repo_file_history" && row.Args[0] == "DESIGN.md" {
			if number(row.Args[3]) != 2 || number(row.Args[4]) != 1 {
				t.Fatalf("history row=%v", row)
			}
			return
		}
	}
	t.Fatal("merged history lost the document")
}

func TestRefreshNewCommitAnswersSurvive(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	runGit(t, root, "init")
	if err := os.WriteFile(filepath.Join(root, "DESIGN.md"), []byte("# Design\nKeep the origin.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "DESIGN.md")
	runGit(t, root, "commit", "-m", "origin")
	cfg := config.DefaultOrientConfig()
	s, err := Measure(ctx, root, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Facts = append(s.Facts, types.Fact{Predicate: "doc_role_claim", Args: []any{"DESIGN.md", types.MangleAtom("/vision"), int64(90)}})
	e, err := s.Engine(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err := s.Save(root, e); err != nil {
		t.Fatal(err)
	}
	answers := []byte("{\n  \"keep\": \"operator decision\"\n}\n")
	answersPath := filepath.Join(root, ".nerd", "orientation", "answers.json")
	if err := os.WriteFile(answersPath, answers, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("package fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "new.go")
	runGit(t, root, "commit", "-m", "extend")
	out, err := Refresh(ctx, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Refreshed || len(out.Stale) == 0 {
		t.Fatalf("refresh result=%+v", out)
	}
	addedHistory, removedHead := false, false
	for _, row := range out.Added {
		if row.Predicate == "repo_file_history" && row.Args[0] == "new.go" {
			addedHistory = true
		}
	}
	for _, row := range out.Removed {
		if row.Predicate == "oriented_head" {
			removedHead = true
		}
	}
	if !addedHistory || !removedHead {
		t.Fatalf("incremental publication omitted history/head changes: %+v", out)
	}
	fresh, err := LoadSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	found, claim := false, false
	for _, row := range fresh.Facts {
		if row.Predicate == "repo_file_history" && row.Args[0] == "new.go" {
			found = true
		}
		if row.Predicate == "doc_role_claim" {
			claim = true
		}
	}
	if !found || !claim {
		t.Fatalf("new history=%v unchanged role=%v", found, claim)
	}
	after, err := os.ReadFile(answersPath)
	if err != nil || !bytes.Equal(after, answers) {
		t.Fatalf("answers changed: %s err=%v", after, err)
	}
	if err := os.WriteFile(filepath.Join(root, "DESIGN.md"), []byte("# Changed\nA different intent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = Refresh(ctx, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Pending) != 1 {
		t.Fatalf("pending=%v", out.Pending)
	}
	fresh, err = LoadSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	digests := 0
	for _, row := range fresh.Facts {
		if row.Predicate == "doc_role_claim" {
			t.Fatalf("changed document kept stale role: %v", row)
		}
		if row.Predicate == "doc_body_digest" && row.Args[0] == "DESIGN.md" {
			digests++
			if types.ExtractString(row.Args[1]) != documentDigest("# Changed\nA different intent.") {
				t.Fatalf("changed document kept stale body measurement: %v", row)
			}
		}
	}
	if digests != 1 {
		t.Fatalf("body digest census=%d want=1", digests)
	}
}
