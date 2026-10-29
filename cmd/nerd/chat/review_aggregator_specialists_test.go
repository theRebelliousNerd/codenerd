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

	"codenerd/internal/experts"
	"codenerd/internal/types"
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

// /review asks the kernel like every other delegation: a registered, ready
// agent the roster names, whose domain covers a file under review, is matched,
// and its knowledge DB comes from the registry for the brief.
func TestMatchSpecialistsForReview_MatchesRegisteredSpecialist(t *testing.T) {
	m := newRoundtripModel(t)
	assertExpertRoster(t, m)
	reg := readyGoRegistry("/tmp/kb.db")
	reg.Agents[0].Name = "MangleExpert"

	matches := m.matchSpecialistsForReview([]string{"internal/core/defaults/policy/shards.mg"}, reg)

	if len(matches) != 1 || matches[0].AgentName != "mangleexpert" {
		t.Fatalf("matches = %+v, want mangleexpert for a policy file", matches)
	}
	if matches[0].Score <= 0 || len(matches[0].Files) == 0 {
		t.Errorf("match carries no score or files: %+v", matches[0])
	}
	if matches[0].KnowledgePath != "/tmp/kb.db" {
		t.Errorf("match lost the registry's knowledge path: %+v", matches[0])
	}
}

// Status is decoded from the registry: boot lists only ready agents on the
// roster the kernel selects from.
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

// An agent off the roster (boot lists only ready agents) is never matched,
// even when a domain claim names it.
func TestMatchSpecialistsForReview_UnreadyAgentIsNotMatched(t *testing.T) {
	m := newRoundtripModel(t)
	if err := m.kernel.Assert(types.Fact{Predicate: "shard_agent_domain", Args: []any{
		experts.AgentAtom("GoExpert"), types.MangleAtom("/go"), types.MangleAtom("/registry")}}); err != nil {
		t.Fatalf("assert: %v", err)
	}
	if matches := m.matchSpecialistsForReview([]string{"handler.go"}, readyGoRegistry("/tmp/kb.db")); len(matches) != 0 {
		t.Errorf("agent that is not on the roster was matched: %+v", matches)
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
