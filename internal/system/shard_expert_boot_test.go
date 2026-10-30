package system

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/experts"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// teaExpertPromptsYAML is a shard agent whose identity, methodology and domain
// atoms carry distinct markers. The domain atom declares languages: [/mangle],
// which is how an agent's KB claims a technology.
const teaExpertPromptsYAML = `- id: "teaexpert/identity"
  category: "identity"
  subcategory: "teaexpert"
  is_mandatory: true
  description: "Identity for TeaExpert"
  content: |
    TEA_EXPERT_IDENTITY_MARKER You are TeaExpert.
- id: "teaexpert/methodology"
  category: "methodology"
  subcategory: "teaexpert"
  is_mandatory: true
  description: "Methodology for TeaExpert"
  content: |
    TEA_EXPERT_METHOD_MARKER Read the model's update loop before touching a view.
- id: "teaexpert/domain/policy"
  category: "domain"
  subcategory: "teaexpert"
  is_mandatory: false
  languages: ["/mangle"]
  description: "Policy files for TeaExpert"
  content: |
    TEA_EXPERT_DOMAIN_MARKER
`

// The roster is asserted on the real boot path and the decision drives the
// compile: BootCortex reads agents.json `domains` and the agent's KB tags into
// shard_agent / shard_agent_domain, plus init's import vocabulary; the kernel
// then derives that a fix on a policy file needs TeaExpert; and the coder's
// compile carries TeaExpert's methodology and knowledge but never its
// identity or its self-description. Without the experts in the compile the
// methodology is absent: the knowledge was on disk and never reached a turn.
func TestBootAssertsShardExpertRosterAndTheCompileCarriesIt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	agentDir := filepath.Join(ws, ".nerd", "agents", "TeaExpert")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "prompts.yaml"), []byte(teaExpertPromptsYAML), 0o644); err != nil {
		t.Fatalf("write prompts.yaml: %v", err)
	}
	writeAgentsJSON(t, ws, []map[string]any{{
		"name":    "TeaExpert",
		"type":    "persistent",
		"status":  "ready",
		"domains": []string{"bubbletea"},
	}})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)

	// Roster facts.
	if rows, err := cortex.Kernel.Query("shard_agent"); err != nil || !factWith(rows, "/teaexpert") {
		t.Fatalf("shard_agent after boot = %v, %v; want /teaexpert", rows, err)
	}
	domains, err := cortex.Kernel.Query("shard_agent_domain")
	if err != nil {
		t.Fatalf("query shard_agent_domain: %v", err)
	}
	if !factWith(domains, "/teaexpert", "/bubbletea", "/registry") {
		t.Errorf("registry domain missing: %v", domains)
	}
	if !factWith(domains, "/teaexpert", "/mangle", "/knowledge_base") {
		t.Errorf("knowledge-base domain missing: %v", domains)
	}
	if roots, err := cortex.Kernel.Query("technology_import_exact"); err != nil || !factWith(roots, "github.com/charmbracelet/bubbletea", "/bubbletea") {
		t.Errorf("technology_import_exact after boot = %v, %v", roots, err)
	}
	// agents.json kept the domains through the boot-time registry sync.
	if data, err := os.ReadFile(filepath.Join(ws, ".nerd", "agents.json")); err != nil || !strings.Contains(string(data), `"bubbletea"`) {
		t.Errorf("agents.json lost the agent's domains on boot: %s (%v)", data, err)
	}

	// The decision.
	sel, err := experts.Select(cortex.Kernel, experts.Task{Verb: "/fix", Targets: []string{"internal/x/policy.mg"}, Workspace: ws})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if got := sel.Agents(); len(got) != 1 || got[0] != "teaexpert" {
		t.Fatalf("a fix on a policy file needs %v, want [teaexpert]", got)
	}

	// The compile. A knowledge row about the subject and the init seed's
	// self-description sit in the agent's KB.
	db, ok := cortex.JITCompiler.LookupShardDB("teaexpert")
	if !ok {
		t.Fatal("TeaExpert's KB is not registered with the JIT compiler")
	}
	insertKnowledgeRow(t, db, "domain/policy_files", "TEA_EXPERT_KNOWLEDGE_MARKER policy files load in order")
	insertKnowledgeRow(t, db, "agent_identity", "You are TeaExpert, a specialist agent in the codeNERD ecosystem. TEA_EXPERT_SELF_MARKER policy files")

	compile := func(expertsFor []string) string {
		t.Helper()
		result, err := cortex.JITCompiler.Compile(ctx, &prompt.CompilationContext{
			IntentVerb:      "/fix",
			IntentTarget:    "internal/x/policy.mg",
			OperationalMode: "/active",
			ShardType:       "/coder",
			ShardID:         "coder",
			Language:        "/mangle",
			TokenBudget:     64000,
			SemanticQuery:   "policy files",
			Experts:         expertsFor,
		})
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		return result.Prompt
	}
	without := compile(nil)
	if strings.Contains(without, "TEA_EXPERT_METHOD_MARKER") {
		t.Fatalf("control: the expert's methodology reached a compile that did not name it")
	}
	with := compile(sel.Agents())
	for _, marker := range []string{"TEA_EXPERT_METHOD_MARKER", "TEA_EXPERT_KNOWLEDGE_MARKER"} {
		if !strings.Contains(with, marker) {
			t.Errorf("the compile the kernel named TeaExpert for lacks %s", marker)
		}
	}
	for _, marker := range []string{"TEA_EXPERT_IDENTITY_MARKER", "TEA_EXPERT_SELF_MARKER"} {
		if strings.Contains(with, marker) {
			t.Errorf("a pushed expert lent its voice: %s reached the coder's compile", marker)
		}
	}
}

func insertKnowledgeRow(t *testing.T, db *sql.DB, concept, content string) {
	t.Helper()
	// Init's research writes this table; a KB synced from prompts.yaml alone
	// has only prompt_atoms.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_atoms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		concept TEXT NOT NULL,
		content TEXT NOT NULL,
		confidence REAL DEFAULT 1.0,
		content_hash TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, content, confidence, content_hash) VALUES (?, ?, 0.9, ?)`,
		concept, content, "hash-"+concept); err != nil {
		t.Fatalf("insert knowledge row: %v", err)
	}
}

// factWith reports whether some row's leading arguments render as want.
func factWith(rows []types.Fact, want ...string) bool {
	for _, f := range rows {
		if len(f.Args) < len(want) {
			continue
		}
		match := true
		for i, w := range want {
			if types.ExtractString(f.Args[i]) != w {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
