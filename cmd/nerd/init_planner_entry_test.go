package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// fakeOllama is an Ollama endpoint: the OpenAI-compatible chat route the
// chat client calls, and the embedding routes the embedding engine calls.
// It records every chat request so a test can see which configured slot
// served which init phase.
type fakeOllama struct {
	server   *httptest.Server
	mu       sync.Mutex
	requests []string
}

var initEntryDocLine = regexp.MustCompile(`(?m)^@@DOC ([^\n]+)`)

func newFakeOllama(t *testing.T) *fakeOllama {
	t.Helper()
	f := &fakeOllama{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_, _ = io.WriteString(w, `{"models":[{"name":"fixture-embed","model":"fixture-embed"}]}`)
		case "/api/embeddings", "/api/embed":
			_, _ = io.WriteString(w, `{"embedding":[0.1,0.2,0.3,0.4]}`)
		case "/v1/chat/completions":
			var body struct {
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			var system, user string
			for _, message := range body.Messages {
				switch message.Role {
				case "system":
					system += message.Content
				case "user":
					user += message.Content
				}
			}
			f.mu.Lock()
			f.requests = append(f.requests, system+"\n----\n"+user)
			f.mu.Unlock()
			reply, err := initEntryReply(system + "\n----\n" + user)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload, _ := json.Marshal(map[string]any{
				"id": "fixture", "object": "chat.completion", "model": "fixture",
				"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": reply}, "finish_reason": "stop"}},
				"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
			})
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeOllama) phases() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := map[string]bool{}
	var phases []string
	for _, request := range f.requests {
		phase := initEntryPhase(request)
		if !seen[phase] {
			seen[phase] = true
			phases = append(phases, phase)
		}
	}
	slices.Sort(phases)
	return phases
}

func initEntryPhase(request string) string {
	switch {
	case strings.Contains(request, "@@DOC "):
		return "document_roles"
	case strings.Contains(request, "@@BRIEF"):
		return "north_star"
	case strings.Contains(request, "Propose the domain experts"):
		return "domain_experts"
	case strings.Contains(request, "Choose which of this repository's own commands"):
		return "project_tools"
	case strings.Contains(request, "methodology atoms for the specialist"), strings.Contains(request, "domain atoms for the specialist"):
		return "specialist_prompts"
	}
	return "other"
}

func initEntryReply(request string) (string, error) {
	// Each init phase's instructions are compiled from its role atoms into
	// the system prompt; the user message carries only the phase's data. The
	// reply is keyed off the same phase classification the assertions use.
	switch initEntryPhase(request) {
	case "document_roles":
		var documents []map[string]any
		for _, match := range initEntryDocLine.FindAllStringSubmatch(request, -1) {
			if match[1] != "README.md" {
				continue
			}
			documents = append(documents, map[string]any{"path": match[1], "roles": []map[string]any{{"role": "vision", "confidence_pct": 95, "evidence": "Build a coding agent that wires what it writes."}}, "themes": []string{"coding agents"}})
		}
		body, err := json.Marshal(map[string]any{"documents": documents})
		return string(body), err
	case "north_star":
		return `{"Mission":"Build a coding agent that wires what it writes.","Problem":"Agents forget to wire code.","Vision":"Every change is reached from a shipped entry point.","FieldSources":{"Mission":["README.md"],"Problem":["README.md"],"Vision":["README.md"]}}`, nil
	case "domain_experts":
		return `{"experts":[{"name":"AgentWiringExpert","mission":"Keep agent-written code reachable","why":"the north star requires reachable changes","cites":["north_star"],"topics":["entry-point wiring"],"research":[{"question":"how do coding agents forget to wire new code","fast_moving":false}]}]}`, nil
	case "project_tools":
		return `{"tools":[{"command":"make test","name":"test","category":"test","description":"Runs the fixture tests."}]}`, nil
	case "specialist_prompts":
		if strings.Contains(request, "domain atoms for the specialist") {
			return "## Domain\n\n- Agents fail by building code nothing calls.", nil
		}
		return "## Methodology\n\n- Trace every artifact to its reader.", nil
	}
	return "", fmt.Errorf("unscripted request: %.100q", request)
}

func initEntryGit(t *testing.T, root string, args ...string) {
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

func writeEntryFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestNerdInit_PlannerSlotServesPlannerPhases drives `nerd init` through its
// real entry point and its production client construction (configureInitLLM
// reading .nerd/config.json), with the main and planner slots on two
// separate fake endpoints. Orientation, the north star, the domain-expert
// proposal, specialist prompts and project tools reach the planner endpoint;
// none reaches the main one. `init --force` then leaves the user's
// config.json byte-identical and the workspace .env untouched.
func TestNerdInit_PlannerSlotServesPlannerPhases(t *testing.T) {
	logger = zap.NewNop()
	main := newFakeOllama(t)
	planner := newFakeOllama(t)
	embed := newFakeOllama(t)

	ws := t.TempDir()
	writeEntryFile(t, ws, "README.md", "# Agent\nBuild a coding agent that wires what it writes.\n")
	writeEntryFile(t, ws, "go.mod", "module example.com/agent\n\ngo 1.24\n")
	writeEntryFile(t, ws, "main.go", "package main\n\nfunc main() {}\n")
	writeEntryFile(t, ws, "main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}\n")
	writeEntryFile(t, ws, "Makefile", "test:\n\tgo test ./...\n")
	for _, name := range []string{"planning", "delivery", "review"} {
		writeEntryFile(t, ws, "docs/"+name+".md", "# "+name+"\n[Project](../README.md)\nReview repository changes.\n")
	}
	initEntryGit(t, ws, "init")
	initEntryGit(t, ws, "add", ".")
	initEntryGit(t, ws, "commit", "-m", "origin")
	envBody := "SECRET_TOKEN=keep-me\n"
	writeEntryFile(t, ws, ".env", envBody)
	configBody := fmt.Sprintf(`{
  "provider": "ollama",
  "ollama": {"endpoint": %q, "model": "main-model"},
  "planner": {"provider": "ollama", "endpoint": %q, "model": "planner-model"},
  "embedding": {"provider": "ollama", "ollama_endpoint": %q, "ollama_model": "fixture-embed", "dimensions": 4}
}
`, main.server.URL, planner.server.URL, embed.server.URL)
	writeEntryFile(t, ws, ".nerd/config.json", configBody)

	savedWorkspace, savedForce, savedNoInteractive := workspace, forceInit, noInteractiveInit
	t.Cleanup(func() { workspace, forceInit, noInteractiveInit = savedWorkspace, savedForce, savedNoInteractive })
	workspace, noInteractiveInit = ws, true

	cmd := &cobra.Command{}
	if err := runInitWithLLMConfigurer(cmd, nil, configureInitLLM); err != nil {
		t.Fatalf("nerd init: %v", err)
	}

	want := []string{"document_roles", "domain_experts", "north_star", "project_tools", "specialist_prompts"}
	if got := planner.phases(); !slices.Equal(got, want) {
		t.Fatalf("planner endpoint served %v, want %v", got, want)
	}
	if got := main.phases(); len(got) != 0 {
		t.Fatalf("main endpoint was sent planner work: %v", got)
	}
	registry, err := os.ReadFile(filepath.Join(ws, ".nerd", "agents.json"))
	if err != nil || !strings.Contains(string(registry), "AgentWiringExpert") {
		t.Fatalf("the planner's domain expert is not registered: %v", err)
	}
	tools, err := os.ReadFile(filepath.Join(ws, ".nerd", "tools", "available_tools.json"))
	if err != nil || !strings.Contains(string(tools), `"make test"`) {
		t.Fatalf("the chosen project command tool is not registered: %v", err)
	}

	forceInit = true
	if err := runInitWithLLMConfigurer(cmd, nil, configureInitLLM); err != nil {
		t.Fatalf("nerd init --force: %v", err)
	}
	gotConfig, err := os.ReadFile(filepath.Join(ws, ".nerd", "config.json"))
	if err != nil || !bytes.Equal(gotConfig, []byte(configBody)) {
		t.Fatalf("init --force changed .nerd/config.json (err %v):\n%s", err, gotConfig)
	}
	gotEnv, err := os.ReadFile(filepath.Join(ws, ".env"))
	if err != nil || string(gotEnv) != envBody {
		t.Fatalf("init --force changed .env (err %v): %q", err, gotEnv)
	}
}
