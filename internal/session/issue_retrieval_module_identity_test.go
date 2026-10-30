package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/retrieval"
	"codenerd/internal/types"
)

// Observe the live turn's evidence at the model boundary, before ProcessWithIntent
// releases its issue-scoped facts. Retrieval itself is the production retriever.
type moduleIdentityProvider struct {
	*briefCapturingProvider
	kernel *core.RealKernel
	facts  []core.Fact
	err    error
}

func (p *moduleIdentityProvider) CompleteWithToolResults(ctx context.Context, system string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if len(p.histories) == 0 {
		p.facts, p.err = p.kernel.Query("tiered_context_file")
	}
	return p.briefCapturingProvider.CompleteWithToolResults(ctx, system, history, tools)
}

func TestProcessWithIntent_GoModuleIdentityPreservesNeighborEvidenceAndBriefFloor(t *testing.T) {
	for name, declaration := range map[string]string{
		"unquoted": "module example.com/proj\n",
		"quoted":   "module \"example.com/proj\" // workspace identity\n",
		"block":    "module (\n \"example.com/proj\"\n)\n",
	} {
		for _, policy := range []struct {
			name      string
			cfg       config.RetrievalConfig
			wantBrief bool
		}{
			{name: "floor50", cfg: config.RetrievalConfig{BriefMinRelevance: 50}, wantBrief: true},
			{name: "default60", cfg: config.DefaultRetrievalConfig(), wantBrief: false},
		} {
			t.Run(name+"/"+policy.name, func(t *testing.T) {
				root := t.TempDir()
				for name, body := range map[string]string{
					"go.mod":                  declaration + "\ngo 1.26\n",
					"internal/alpha/alpha.go": "package alpha\nimport \"example.com/proj/internal/beta\"\nfunc Run() { beta.Help() }\n",
					"internal/beta/beta.go":   "package beta\nfunc Help() {}\n",
				} {
					path := filepath.Join(root, name)
					if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				kernel, err := core.NewRealKernel()
				if err != nil {
					t.Fatal(err)
				}
				client := &moduleIdentityProvider{
					briefCapturingProvider: &briefCapturingProvider{MockLLMClient: &MockLLMClient{}},
					kernel:                 kernel,
				}
				executor := NewExecutor(kernel, &testExecutiveStore{}, client, &MockJITCompiler{}, journeyConfigFactory(), journeyIntent("/fix", "/mutation"))
				cfg := journeyConfig(t)
				cfg.WorkspaceRoot = root
				executor.SetConfig(cfg)
				executor.SetIssueRetriever(retrieval.NewTaskRetriever(kernel, retrieval.TaskRetrieverConfig{WorkDir: root, Params: policy.cfg.Params()}))

				const task = "Fix the failure in internal/alpha/alpha.go"
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				if _, err := executor.ProcessWithIntent(ctx, task, &perception.Intent{Verb: "/fix", Category: "/mutation", Target: "internal/alpha/alpha.go"}); err != nil {
					t.Logf("turn error (not under test): %v", err)
				}
				first := client.firstUserText(t)
				if !strings.HasPrefix(first, task) || !strings.Contains(first, "internal/alpha/alpha.go (tier1,") {
					t.Fatalf("first model request lost task or named-file brief:\n%s", first)
				}
				if client.err != nil {
					t.Fatal(client.err)
				}
				var neighbors []core.Fact
				for _, fact := range client.facts {
					if types.ArgString(fact, 1) == "internal/beta/beta.go" {
						neighbors = append(neighbors, fact)
					}
				}
				if len(neighbors) != 1 || types.ArgName(neighbors[0], 2) != "/tier3" {
					t.Fatalf("live neighbor facts = %v, want one Tier 3 beta.go fact; all facts: %v", neighbors, client.facts)
				}
				if score, ok := types.ArgInt64(neighbors[0], 3); !ok || score != 50 {
					t.Fatalf("neighbor relevance = %v (integer=%v), want 50", neighbors[0].Args[3], ok)
				}
				if got := strings.Contains(first, "internal/beta/beta.go (tier3, 50)"); got != policy.wantBrief {
					t.Fatalf("neighbor in model brief = %v, want %v at floor %d:\n%s", got, policy.wantBrief, policy.cfg.BriefMinRelevance, first)
				}
				if !policy.wantBrief && strings.Contains(first, "internal/beta/beta.go") {
					t.Fatalf("neighbor leaked into default-floor model request:\n%s", first)
				}
				if facts, err := kernel.Query("tiered_context_file"); err != nil || len(facts) != 0 {
					t.Fatalf("retrieval evidence outlived its turn: %v, %v", facts, err)
				}
			})
		}
	}
}
