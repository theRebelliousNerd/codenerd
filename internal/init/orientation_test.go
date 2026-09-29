package init

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/orient"
	"codenerd/internal/types"
)

type orientationClient struct{ stubDistinctLLM }

func (c *orientationClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.HasPrefix(user, "@@DOC ") {
		var documents []map[string]any
		for _, match := range regexp.MustCompile("(?m)^@@DOC ([^\n]+)").FindAllStringSubmatch(user, -1) {
			p := match[1]
			role, evidence := "instructions", "Review repository changes."
			if p == "README.md" {
				role, evidence = "vision", "Make repository decisions from evidence."
			}
			documents = append(documents, map[string]any{"path": p, "roles": []map[string]any{{"role": role, "confidence_pct": 95, "evidence": evidence}}, "themes": []string{"repository reasoning"}})
		}
		body, err := json.Marshal(map[string]any{"documents": documents})
		return string(body), err
	}
	if strings.HasPrefix(user, "@@BRIEF") {
		return `{"Mission":"Make repository decisions from evidence.","Problem":"Repository intent is scattered.","Vision":"Ground repository work in durable evidence.","FieldSources":{"Mission":["README.md"],"Problem":["README.md"],"Vision":["README.md"]}}`, nil
	}
	return c.stubDistinctLLM.CompleteWithSystem(ctx, system, user)
}

func orientationFixtureGit(t *testing.T, root string, args ...string) {
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

func orientationFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeInitFile(t, root, "README.md", "# Project\nMake repository decisions from evidence.\n")
	writeInitFile(t, root, ".claude/agents/reviewer.md", "---\nname: reviewer\ndescription: Reviews changes\n---\nReview repository changes.\n")
	for _, name := range []string{"planning", "delivery", "review"} {
		writeInitFile(t, root, "docs/"+name+".md", "# "+name+"\n[Project](../README.md)\nReview repository changes.\n")
	}
	writeInitFile(t, root, "main.go", "package fixture\n")
	orientationFixtureGit(t, root, "init")
	orientationFixtureGit(t, root, "add", "README.md", ".claude/agents/reviewer.md", "main.go", "docs")
	orientationFixtureGit(t, root, "commit", "-m", "origin")
	return root
}

func initializeOrientationFixture(t *testing.T) (*Initializer, *InitResult, []string) {
	t.Helper()
	root := orientationFixture(t)
	cfg := DefaultInitConfig(root)
	cfg.LLMClient = &orientationClient{}
	cfg.SkipResearch, cfg.SkipAgentCreate, cfg.Interactive = true, true, false
	cfg.Timeout = 0
	var phases []string
	progress := make(chan InitProgress, 512)
	cfg.ProgressChan = progress
	i, err := NewInitializer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i.Close() })
	i.embedEngine = fixedInitEngine{}
	result, err := i.Initialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for len(progress) > 0 {
		update := <-progress
		if len(phases) == 0 || phases[len(phases)-1] != update.Phase {
			phases = append(phases, update.Phase)
		}
	}
	if !result.Success {
		t.Fatalf("init failures=%v warnings=%v", result.Failures, result.Warnings)
	}
	return i, result, phases
}

func TestOrientationPhaseOrder(t *testing.T) {
	_, _, phases := initializeOrientationFixture(t)
	orientation := slices.Index(phases, "orientation")
	for _, consumer := range []string{"profile", "agents", "prompt_atoms", "prompt_db", "codebase_kb", "core_shards_kb", "campaign_kb"} {
		index := slices.Index(phases, consumer)
		if orientation < 0 || index < 0 || orientation >= index {
			t.Fatalf("orientation must precede %s: %v", consumer, phases)
		}
	}
	count := 0
	for _, phase := range phases {
		if phase == "orientation" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("orientation ran %d times: %v", count, phases)
	}
}

func TestOrientationInitArtifacts(t *testing.T) {
	i, result, _ := initializeOrientationFixture(t)
	for _, rel := range []string{"orientation/agents.md", "orientation/README.md", "orientation/orientation.mg", "northstar.json"} {
		body, err := os.ReadFile(filepath.Join(i.config.Workspace, ".nerd", filepath.FromSlash(rel)))
		if err != nil || len(body) == 0 {
			t.Fatalf("artifact %s: bytes=%d err=%v", rel, len(body), err)
		}
		if rel == "orientation/agents.md" && !strings.Contains(string(body), "reviewer") {
			t.Fatal("agent corpus did not feed the orientation roster")
		}
	}
	if !result.VisionDerived {
		t.Fatalf("vision not derived: %v", result.Warnings)
	}
	rows, err := i.orientation.Query("doc_role_claim")
	if err != nil || len(rows) == 0 {
		t.Fatalf("claims=%v err=%v", rows, err)
	}
	found := false
	for _, row := range rows {
		if types.ExtractString(row.Args[0]) == "README.md" {
			found = true
		}
	}
	if !found {
		t.Fatal("classified vision document absent")
	}
	atoms, err := i.localDB.GetAllKnowledgeAtoms()
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, atom := range atoms {
		if atom.Concept == "strategic/vision" && strings.Contains(atom.Content, "durable evidence") {
			found = true
		}
	}
	if !found {
		t.Fatal("strategic consumers lost the derived vision")
	}
}

func TestOrientationCommitRefreshAnswersSurvive(t *testing.T) {
	i, _, _ := initializeOrientationFixture(t)
	root := i.config.Workspace
	answers := []byte("{\n  \"decision\": \"retain operator answer\"\n}\n")
	answersPath := filepath.Join(root, ".nerd", "orientation", "answers.json")
	if err := os.WriteFile(answersPath, answers, 0o644); err != nil {
		t.Fatal(err)
	}
	writeInitFile(t, root, "added.go", "package fixture\n")
	orientationFixtureGit(t, root, "add", "added.go")
	orientationFixtureGit(t, root, "commit", "-m", "extend after init")
	refresh, err := orient.Refresh(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !refresh.Refreshed || len(refresh.Stale) == 0 {
		t.Fatalf("new commit after init did not derive a refresh: %+v", refresh)
	}
	found := false
	for _, row := range refresh.Added {
		if row.Predicate == "repo_file_history" && types.ExtractString(row.Args[0]) == "added.go" {
			found = true
		}
	}
	if !found {
		t.Fatal("refresh did not publish added history")
	}
	after, err := os.ReadFile(answersPath)
	if err != nil || !bytes.Equal(after, answers) {
		t.Fatalf("operator answers changed: err=%v body=%s", err, after)
	}
}
