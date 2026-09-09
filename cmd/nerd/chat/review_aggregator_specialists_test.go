package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// matchSpecialistsForReview and loadAndQueryKnowledgeBase both returned empty
// unconditionally ("For now, return empty" / "Stub: return empty knowledge")
// while spawnMultiShardReview called both on the live /review path. The review
// therefore never consulted a specialist and never consulted a knowledge base,
// and said so only as "Matched 0 specialists", which reads as a result rather
// than a stub.

func readyGoRegistry(kbPath string) *AgentRegistry {
	return &AgentRegistry{
		Version:   "1.5.0",
		CreatedAt: time.Now(),
		Agents: []RegisteredAgent{
			{
				Name:          "GoExpert",
				Type:          "persistent",
				KnowledgePath: kbPath,
				KBSize:        3,
				Status:        "ready",
			},
		},
	}
}

func TestMatchSpecialistsForReview_MatchesRegisteredSpecialist(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "handler.go")
	if err := os.WriteFile(goFile, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	matches := matchSpecialistsForReview(context.Background(), []string{goFile}, readyGoRegistry("/tmp/kb.db"))

	if len(matches) == 0 {
		t.Fatal("no specialist matched a .go file with GoExpert registered and ready")
	}
	if matches[0].AgentName != "GoExpert" {
		t.Errorf("expected GoExpert, got %q", matches[0].AgentName)
	}
	if matches[0].Score <= 0 {
		t.Errorf("match carries no score: %+v", matches[0])
	}
	if len(matches[0].Files) == 0 {
		t.Errorf("match carries no files: %+v", matches[0])
	}
}

// Status is the field the shared matcher gates on. The chat-local registry
// struct did not decode it, so every agent read back as unavailable.
func TestMatchSpecialistsForReview_StatusIsDecodedFromRegistryJSON(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "agents.json")
	body := `{"version":"1.5.0","created_at":"2026-03-03T21:13:48Z","agents":[
	  {"name":"GoExpert","type":"persistent","knowledge_path":"kb.db","kb_size":3,"status":"ready"}]}`
	if err := os.WriteFile(registryPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	data, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	var reg AgentRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatalf("decode registry: %v", err)
	}
	if len(reg.Agents) != 1 || reg.Agents[0].Status != "ready" {
		t.Fatalf("registry decode dropped status: %+v", reg.Agents)
	}
	if reg.Agents[0].KBSize != 3 {
		t.Errorf("registry decode dropped kb_size: %+v", reg.Agents[0])
	}
}

func TestMatchSpecialistsForReview_UnreadyAgentIsNotMatched(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "handler.go")
	if err := os.WriteFile(goFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	reg := readyGoRegistry("/tmp/kb.db")
	reg.Agents[0].Status = "building"

	if matches := matchSpecialistsForReview(context.Background(), []string{goFile}, reg); len(matches) != 0 {
		t.Errorf("agent that is not ready was matched: %+v", matches)
	}
}

// A specialist's ingested knowledge must reach the review task.
func TestLoadAndQueryKnowledgeBase_ReturnsMatchingAtoms(t *testing.T) {
	kbPath := filepath.Join(t.TempDir(), "goexpert_knowledge.db")
	db, err := sql.Open("sqlite3", kbPath)
	if err != nil {
		t.Fatalf("open kb: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (
		id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL, created_at TIMESTAMP)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, content, confidence) VALUES
		('handler conventions', 'every handler returns an ActionResult', 0.9),
		('unrelated', 'kubernetes ingress annotations', 0.9)`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close kb: %v", err)
	}

	knowledge, err := loadAndQueryKnowledgeBase(context.Background(), kbPath,
		[]string{filepath.Join("internal", "core", "handler.go")})
	if err != nil {
		t.Fatalf("query kb: %v", err)
	}
	if !strings.Contains(knowledge, "handler conventions") {
		t.Errorf("knowledge base was not consulted, got: %q", knowledge)
	}
}

// A missing knowledge base is normal for a never-ingested agent, and must not
// create an empty database as a side effect.
func TestLoadAndQueryKnowledgeBase_MissingDBIsNotAnErrorAndCreatesNothing(t *testing.T) {
	kbPath := filepath.Join(t.TempDir(), "absent.db")

	knowledge, err := loadAndQueryKnowledgeBase(context.Background(), kbPath, []string{"main.go"})
	if err != nil {
		t.Fatalf("missing KB reported as an error: %v", err)
	}
	if knowledge != "" {
		t.Errorf("expected empty knowledge, got %q", knowledge)
	}
	if _, err := os.Stat(kbPath); err == nil {
		t.Error("querying a missing knowledge base created a database file")
	}
}

// The task string used to be "review files for <agent>", which dropped both the
// file list and the knowledge the caller had just loaded.
func TestFormatSpecialistReviewTask_CarriesFilesAndKnowledge(t *testing.T) {
	task := buildSpecialistTask(
		SpecialistMatch{AgentName: "GoExpert", Files: []string{"a.go", "b.go"}},
		[]string{"a.go", "b.go", "c.go"},
		"## Relevant knowledge\n\n- **handler conventions**: returns ActionResult\n",
	)

	if len(task.Files) != 2 {
		t.Errorf("specialist should review the files it matched, got %v", task.Files)
	}

	out := formatSpecialistReviewTask(task)
	if !strings.Contains(out, "files:a.go,b.go") {
		t.Errorf("task string lost the file list: %q", out)
	}
	if !strings.Contains(out, "handler conventions") {
		t.Errorf("task string lost the knowledge: %q", out)
	}
}
