package init

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const projectToolsMakefile = `# build helpers
.PHONY: build test proto deploy
VERSION := 1.0
GOFLAGS ::= -mod=mod

build:
	go build ./...

test: build
	go test ./...

proto gen: tools
	buf generate

deploy:
	./scripts/release.sh $(VERSION)

%.o: %.c
	cc -c $<
`

const projectToolsPackageJSON = `{
  "scripts": {
    "lint": "eslint .",
    "prelint": "echo before",
    "postinstall": "patch-package",
    "e2e": "playwright test"
  }
}`

func TestDeclaredProjectCommands(t *testing.T) {
	ws := t.TempDir()
	writeInitFile(t, ws, "Makefile", projectToolsMakefile)
	writeInitFile(t, ws, "package.json", projectToolsPackageJSON)
	writeInitFile(t, ws, "pnpm-lock.yaml", "lockfileVersion: 9\n")

	commands, total, err := declaredProjectCommands(ws)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range commands {
		got = append(got, c.Command+" @ "+c.Source)
	}
	want := []string{
		"make build @ Makefile:6",
		"make test @ Makefile:9",
		"make proto @ Makefile:12",
		"make gen @ Makefile:12",
		"make deploy @ Makefile:15",
		"pnpm run e2e @ package.json scripts.e2e",
		"pnpm run lint @ package.json scripts.lint",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") || total != len(want) {
		t.Fatalf("declared commands =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if commands[2].Recipe != "buf generate" {
		t.Fatalf("make proto recipe = %q, want the rule's recipe", commands[2].Recipe)
	}
}

func TestParseProjectToolChoice_AcceptsOnlyDeclaredCommands(t *testing.T) {
	candidates := []projectCommand{
		{Command: "make test", Source: "Makefile:9"},
		{Command: "make proto", Source: "Makefile:12"},
	}
	reply := "```json\n" + `{"tools":[
		{"command":"make test","name":"test","category":"test","description":"Runs the Go test suite."},
		{"command":"make proto","name":"Proto Gen!","category":"generate","description":"Regenerates protobuf code with buf."},
		{"command":"rm -rf /","name":"wipe","category":"run","description":"invented"},
		{"command":"make test","name":"again","category":"test","description":"duplicate"},
		{"command":"make proto","name":"x","category":"deploy","description":"bad category"}
	]}` + "\n```"
	tools, rejected, err := parseProjectToolChoice(reply, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if rejected != 1 {
		t.Fatalf("rejected = %d, want the one undeclared command", rejected)
	}
	if len(tools) != 2 || tools[0].Name != "project_test" || tools[1].Name != "project_proto_gen" {
		t.Fatalf("tools = %+v", tools)
	}
	if tools[0].ShardAffinity != "TesterShard" || tools[0].Conditions[0] != "declared in Makefile:9" || tools[0].Command != "make test" {
		t.Fatalf("tool fields = %+v", tools[0])
	}
	if _, _, err := parseProjectToolChoice("I think you should run make test", candidates); err == nil {
		t.Fatal("prose instead of the requested JSON must be an error, never a selection")
	}
}

// Phase 7e runs on the planner, offers only declared commands, writes the
// choice through phase 10 into available_tools.json, records it, and reuses
// it on an init whose declared commands are unchanged.
func TestRunPhase7e_PlannerChoosesDeclaredCommandsAndReusesUnchanged(t *testing.T) {
	ws := t.TempDir()
	nerdDir := filepath.Join(ws, ".nerd")
	writeInitFile(t, ws, "Makefile", projectToolsMakefile)
	if err := os.MkdirAll(filepath.Join(nerdDir, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	main := &identifiedClient{provider: "meta", model: "worker-model"}
	planner := &identifiedClient{provider: "anthropic", model: "planner-model", reply: func(_, user string) (string, error) {
		if !strings.Contains(user, "make test") || strings.Contains(user, "%.o") {
			t.Errorf("the planner was not offered exactly the declared commands:\n%s", user)
		}
		return `{"tools":[{"command":"make test","name":"test","category":"test","description":"Runs the Go test suite."}]}`, nil
	}}
	var out bytes.Buffer
	newInit := func() *Initializer {
		return &Initializer{config: InitConfig{Workspace: ws, LLMClient: main, PlannerClient: planner}, out: &out}
	}
	profile := ProjectProfile{Name: "probe", Language: "go", BuildSystem: "makefile"}

	ini := newInit()
	result := &InitResult{}
	ini.runPhase7eProjectTools(context.Background(), newPhaseRunner(ini), result, nerdDir, profile)
	ini.runPhase10Tools(newPhaseRunner(ini), result, nerdDir, profile)
	if len(result.ProjectTools) != 1 || result.ProjectTools[0] != "project_test" {
		t.Fatalf("ProjectTools = %v (warnings %v)", result.ProjectTools, result.Warnings)
	}
	if n := len(planner.requests()); n != 1 {
		t.Fatalf("planner requests = %d, want 1", n)
	}
	if n := len(main.requests()); n != 0 {
		t.Fatalf("main requests = %d, want 0: project tool selection is planner work", n)
	}
	data, err := os.ReadFile(filepath.Join(nerdDir, "tools", "available_tools.json"))
	if err != nil {
		t.Fatal(err)
	}
	var registered []ToolDefinition
	if err := json.Unmarshal(data, &registered); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range registered {
		if tool.Name == "project_test" && tool.Command == "make test" {
			found = true
		}
	}
	if !found {
		t.Fatalf("available_tools.json does not register the chosen project tool: %s", data)
	}
	for _, lie := range []string{"Generated project", "ready to use"} {
		if strings.Contains(out.String(), lie) {
			t.Errorf("phase 7e output says %q:\n%s", lie, out.String())
		}
	}

	// Same declared commands: the selection is reused without a request.
	again := newInit()
	result = &InitResult{}
	again.runPhase7eProjectTools(context.Background(), newPhaseRunner(again), result, nerdDir, profile)
	if len(result.ProjectTools) != 1 || len(planner.requests()) != 1 {
		t.Fatalf("an unchanged repository asked the planner again (%d requests, tools %v)", len(planner.requests()), result.ProjectTools)
	}

	// A changed Makefile is chosen from again.
	writeInitFile(t, ws, "Makefile", projectToolsMakefile+"\nlint:\n\tgolangci-lint run\n")
	third := newInit()
	third.runPhase7eProjectTools(context.Background(), newPhaseRunner(third), &InitResult{}, nerdDir, profile)
	if len(planner.requests()) != 2 {
		t.Fatalf("a changed Makefile was not offered again (%d planner requests)", len(planner.requests()))
	}
}
