package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	nerdsystem "codenerd/internal/system"
	"codenerd/internal/types"
)

// initEntryClient is an LLM client that reports an identity and records
// which init phase each request belongs to.
type initEntryClient struct {
	model    string
	mu       sync.Mutex
	requests []string
}

var chatInitDocLine = regexp.MustCompile(`(?m)^@@DOC ([^\n]+)`)

func (c *initEntryClient) ModelIdentity() (string, string) { return "fixture", c.model }

func (c *initEntryClient) answer(system, user string) (string, error) {
	c.mu.Lock()
	c.requests = append(c.requests, system+"\n----\n"+user)
	c.mu.Unlock()
	switch {
	case strings.HasPrefix(user, "@@DOC "):
		var documents []map[string]any
		for _, match := range chatInitDocLine.FindAllStringSubmatch(user, -1) {
			if match[1] == "README.md" {
				documents = append(documents, map[string]any{"path": "README.md", "roles": []map[string]any{{"role": "vision", "confidence_pct": 95, "evidence": "Build a coding agent that wires what it writes."}}})
			}
		}
		body, err := json.Marshal(map[string]any{"documents": documents})
		return string(body), err
	case strings.HasPrefix(user, "@@BRIEF"):
		return `{"Mission":"Build a coding agent that wires what it writes.","Problem":"Agents forget to wire code.","Vision":"Every change is reached.","FieldSources":{"Mission":["README.md"],"Problem":["README.md"],"Vision":["README.md"]}}`, nil
	case strings.Contains(user, "Propose the domain experts"):
		return `{"experts":[{"name":"AgentWiringExpert","mission":"Keep agent-written code reachable","why":"the north star requires it","cites":["north_star"],"topics":["entry-point wiring"]}]}`, nil
	case strings.Contains(user, "Choose which of this repository's own commands"):
		return `{"tools":[{"command":"make test","name":"test","category":"test","description":"Runs the tests."}]}`, nil
	case strings.Contains(user, "prompt atom"):
		return "## Notes\n\n- Trace every artifact to its reader.", nil
	}
	return "", fmt.Errorf("unscripted request: %.80q", user)
}

func (c *initEntryClient) Complete(_ context.Context, prompt string) (string, error) {
	return c.answer("", prompt)
}
func (c *initEntryClient) CompleteWithSystem(_ context.Context, system, user string) (string, error) {
	return c.answer(system, user)
}
func (c *initEntryClient) CompleteWithStreaming(_ context.Context, system, user string, _ bool) (<-chan string, <-chan types.StreamResult) {
	return types.StreamWhole(context.Background(), func(context.Context) (string, []string, error) {
		text, err := func() (string, error) {
			return c.answer(system, user)
		}()
		return text, nil, err
	})
}
func (c *initEntryClient) CompleteWithTools(_ context.Context, system, user string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	text, err := c.answer(system, user)
	return &types.LLMToolResponse{Text: text}, err
}

func (c *initEntryClient) phases() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	seen := map[string]bool{}
	var phases []string
	for _, request := range c.requests {
		phase := "other"
		switch {
		case strings.Contains(request, "@@DOC "):
			phase = "document_roles"
		case strings.Contains(request, "@@BRIEF"):
			phase = "north_star"
		case strings.Contains(request, "Propose the domain experts"):
			phase = "domain_experts"
		case strings.Contains(request, "Choose which of this repository's own commands"):
			phase = "project_tools"
		case strings.Contains(request, "prompt atom"):
			phase = "specialist_prompts"
		}
		if !seen[phase] {
			seen[phase] = true
			phases = append(phases, phase)
		}
	}
	slices.Sort(phases)
	return phases
}

type initEntryEmbedder struct{}

func (initEntryEmbedder) Embed(context.Context, string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3, 0.4}, nil
}
func (initEntryEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1, 0.2, 0.3, 0.4}
	}
	return out, nil
}
func (initEntryEmbedder) Dimensions() int { return 4 }
func (initEntryEmbedder) Name() string    { return "fixed" }

func chatInitGit(t *testing.T, root string, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// TestRunInitialization_PlannerPhasesUseTheSessionPlanner drives the chat
// /init path (runInitialization, what /init and /init --force dispatch) with
// the session's main client and the planner slot the Cortex booted. The
// planner phases reach the Cortex planner, nothing reaches the main client,
// and the completion message says what happened.
func TestRunInitialization_PlannerPhasesUseTheSessionPlanner(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"README.md":    "# Agent\nBuild a coding agent that wires what it writes.\n",
		"go.mod":       "module example.com/agent\n\ngo 1.24\n",
		"main.go":      "package main\n\nfunc main() {}\n",
		"main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}\n",
		"Makefile":     "test:\n\tgo test ./...\n",
	}
	for _, name := range []string{"planning", "delivery", "review"} {
		files["docs/"+name+".md"] = "# " + name + "\n[Project](../README.md)\nReview repository changes.\n"
	}
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	chatInitGit(t, root, "init")
	chatInitGit(t, root, "add", ".")
	chatInitGit(t, root, "commit", "-m", "origin")

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	main := &initEntryClient{model: "main-model"}
	planner := &initEntryClient{model: "planner-model"}
	m := NewTestModel()
	m.workspace = root
	m.kernel = kernel
	m.client = main
	m.cortex = &nerdsystem.Cortex{PlannerLLMClient: planner}
	m.embeddingEngine = initEntryEmbedder{}

	msg := m.runInitialization(true)()
	complete, ok := msg.(initCompleteMsg)
	if !ok {
		t.Fatalf("runInitialization returned %T: %+v", msg, msg)
	}
	want := []string{"document_roles", "domain_experts", "north_star", "project_tools", "specialist_prompts"}
	if got := planner.phases(); !slices.Equal(got, want) {
		t.Fatalf("Cortex planner served %v, want %v", got, want)
	}
	if got := main.phases(); len(got) != 0 {
		t.Fatalf("the session's main client was sent planner work: %v", got)
	}
	rendered := m.renderInitComplete(complete.result)
	if !strings.Contains(rendered, "| planner | fixture/planner-model |") {
		t.Fatalf("the completion message does not show the planner's work:\n%s", rendered)
	}
}

// The heading reports a failed init as incomplete and lists the failures.
func TestRenderInitComplete_ReportsRequiredFailures(t *testing.T) {
	m := NewTestModel()
	rendered := m.renderInitComplete(&nerdinit.InitResult{Success: false, Failures: []string{"Failed to save tools: disk full"}})
	if strings.Contains(rendered, "## Initialization Complete") || !strings.Contains(rendered, "Failed to save tools: disk full") {
		t.Fatalf("a failed init rendered as:\n%s", rendered)
	}
}

// The init summary must show what the run logged, not only the warnings it
// curated. 2026-10-27 live: 44 embedding/store errors sat in the problems
// log while this message showed planner metrics and research warnings.
func TestRenderInitComplete_ShowsLoggedProblems(t *testing.T) {
	m := NewTestModel()
	rendered := m.renderInitComplete(&nerdinit.InitResult{
		Success: true,
		LogProblems: nerdinit.InitLogProblems{
			Path:     "/ws/.nerd/logs/run_problems.log",
			Warnings: 118,
			Errors:   44,
		},
	})
	for _, want := range []string{"44 error(s)", "118 warning(s)", "run_problems.log"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered summary missing %q:\n%s", want, rendered)
		}
	}
	quiet := m.renderInitComplete(&nerdinit.InitResult{Success: true})
	if strings.Contains(quiet, "Logged During Init") {
		t.Fatalf("a run that logged nothing must not claim a log line:\n%s", quiet)
	}
}

func TestInitProgressStatus_ShowsMeasuredPositionOnly(t *testing.T) {
	got := initProgressStatus(nerdinit.InitProgress{Message: "Scanning codebase...", CurrentPhaseNo: 3, TotalPhases: 17})
	if !strings.HasPrefix(got, "[init 3/17, ") || !strings.HasSuffix(got, "] Scanning codebase...") {
		t.Fatalf("status = %q", got)
	}
	if got := initProgressStatus(nerdinit.InitProgress{Message: "Agent X: ready"}); got != "Agent X: ready" {
		t.Fatalf("status = %q", got)
	}
}
