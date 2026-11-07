package session

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/projectdoc"
)

func moduleDocs() []*projectdoc.Document {
	return []*projectdoc.Document{
		{Path: "internal/core/nerd.md", Body: "CORE-GUIDE"},
		{Path: "internal/session/nerd.md", Body: "SESSION-GUIDE"},
		{Path: "internal/tools/nerd.md", Body: "TOOLS-GUIDE"},
		{Path: "internal/tools/research/nerd.md", Body: "RESEARCH-GUIDE"},
	}
}

func chainPaths(chain []*projectdoc.Document) string {
	var out []string
	for _, d := range chain {
		out = append(out, d.Path)
	}
	return strings.Join(out, ",")
}

func TestModuleChain_NearestFirstAndNeverOutside(t *testing.T) {
	docs := moduleDocs()
	for target, want := range map[string]string{
		"internal/core/kernel.go":              "internal/core/nerd.md",
		"internal/core/sub/deep.go":            "internal/core/nerd.md",
		"internal/session":                     "internal/session/nerd.md",
		"internal/tools/research/fetch.go":     "internal/tools/research/nerd.md,internal/tools/nerd.md",
		"internal/tools/registry.go":           "internal/tools/nerd.md",
		"internal/coreish/x.go":                "",
		"cmd/nerd/main.go":                     "",
		"README.md":                            "",
		"":                                     "",
		"../elsewhere/internal/core/kernel.go": "",
	} {
		if got := chainPaths(moduleChain(docs, target)); got != want {
			t.Errorf("moduleChain(%q) = %q, want %q", target, got, want)
		}
	}
}

func TestWithProjectInstructions_AddsTheTargetsModuleGuidanceOutsideALoop(t *testing.T) {
	e := &Executor{}
	e.SetProjectDoc(&projectdoc.Document{Path: "nerd.md", Body: "ROOT-GUIDE"})
	e.SetProjectModuleDocs(moduleDocs())

	got := e.withProjectInstructions(context.Background(), "BASE", "internal/tools/research/fetch.go")
	for _, want := range []string{"BASE", "ROOT-GUIDE", "RESEARCH-GUIDE", "TOOLS-GUIDE"} {
		if strings.Count(got, want) != 1 {
			t.Fatalf("%q appears %d times in:\n%s", want, strings.Count(got, want), got)
		}
	}
	if strings.Contains(got, "CORE-GUIDE") || strings.Contains(got, "SESSION-GUIDE") {
		t.Fatalf("another module's guidance leaked in:\n%s", got)
	}
	if strings.Index(got, "ROOT-GUIDE") > strings.Index(got, "RESEARCH-GUIDE") || strings.Index(got, "RESEARCH-GUIDE") > strings.Index(got, "TOOLS-GUIDE") {
		t.Fatalf("order must be root, nearest module, then its ancestor:\n%s", got)
	}
	if got := e.withProjectInstructions(context.Background(), "BASE", "cmd/nerd/main.go"); strings.Contains(got, "Module guidance") {
		t.Fatalf("a target outside every module got module guidance:\n%s", got)
	}

	// Inside a working loop the modules ride the focus view, not the prompt.
	inLoop := context.WithValue(context.Background(), workingLoopKey{}, &workingLoop{focus: "internal/core/kernel.go"})
	if got := e.withProjectInstructions(inLoop, "BASE", "internal/core/kernel.go"); strings.Contains(got, "CORE-GUIDE") {
		t.Fatalf("module guidance was put on the system prompt inside a loop:\n%s", got)
	}
}

func TestWorkingFocusView_ModuleGuidanceFollowsTheFocus(t *testing.T) {
	e := &Executor{}
	e.SetProjectModuleDocs(moduleDocs())
	loop := &workingLoop{focus: "internal/core/kernel.go"}
	ctx := context.WithValue(context.Background(), workingLoopKey{}, loop)

	// No holographic provider: the guidance still forms the view.
	core := e.workingFocusView(ctx, loop)
	if strings.Count(core, "CORE-GUIDE") != 1 || strings.Contains(core, "SESSION-GUIDE") {
		t.Fatalf("core focus view:\n%s", core)
	}
	loop.focus = "internal/session/executor.go"
	session := e.workingFocusView(ctx, loop)
	if strings.Count(session, "SESSION-GUIDE") != 1 || strings.Contains(session, "CORE-GUIDE") {
		t.Fatalf("after the focus moved, view:\n%s", session)
	}
	loop.focus = "cmd/nerd/main.go"
	if view := e.workingFocusView(ctx, loop); view != "" {
		t.Fatalf("a focus outside every module, with no provider, rendered %q", view)
	}
}

// A module that does not fit the budget is named, not cut.
func TestModuleGuidance_BudgetDropsWholeModulesAndNamesThem(t *testing.T) {
	e := &Executor{}
	e.SetProjectModuleDocs([]*projectdoc.Document{
		{Path: "internal/tools/research/nerd.md", Body: strings.Repeat("r", 100)},
		{Path: "internal/tools/nerd.md", Body: strings.Repeat("t", 500)},
	})
	got := e.moduleGuidance("internal/tools/research/fetch.go", 200, "test")
	if !strings.Contains(got, strings.Repeat("r", 100)) {
		t.Fatalf("the nearest module, which fits, is missing:\n%s", got)
	}
	if strings.Contains(got, "ttt") {
		t.Fatalf("a module over the budget was cut in, not left out:\n%s", got)
	}
	if !strings.Contains(got, "not shown") || !strings.Contains(got, "internal/tools/nerd.md") {
		t.Fatalf("the module left out is not named:\n%s", got)
	}
	if got := e.moduleGuidance("internal/tools/research/fetch.go", -1, "test"); strings.Contains(got, "rrr") || !strings.Contains(got, "internal/tools/research/nerd.md") {
		t.Fatalf("no room must name every module and include none:\n%s", got)
	}
}

func TestModuleGuidance_AbsoluteTargetsResolveAgainstTheWorkspace(t *testing.T) {
	ws := t.TempDir()
	e := &Executor{config: ExecutorConfig{WorkspaceRoot: ws}}
	e.SetProjectModuleDocs(moduleDocs())
	if got := e.moduleGuidance(filepath.Join(ws, "internal", "core", "kernel.go"), 0, "test"); !strings.Contains(got, "CORE-GUIDE") {
		t.Fatalf("absolute target under the workspace got:\n%s", got)
	}
	if got := e.moduleGuidance(filepath.Join(filepath.Dir(ws), "internal", "core", "kernel.go"), 0, "test"); got != "" {
		t.Fatalf("a target outside the workspace got guidance:\n%s", got)
	}
}

// Module guidance has to reach where a turn's work happens: an inline task's
// cloned executor and a spawned subagent's executor, not only the session's.
func TestModuleGuidance_ReachesClonedAndSpawnedExecutors(t *testing.T) {
	parent, _ := routeFixture(t)
	parent.SetProjectModuleDocs(moduleDocs())
	if got := parent.CloneForTask().moduleGuidance("internal/core/kernel.go", 0, "test"); !strings.Contains(got, "CORE-GUIDE") {
		t.Fatalf("an inline task's executor lost the module docs: %q", got)
	}
	s := escalationSpawner(parent)
	s.SetProjectModuleDocs(moduleDocs())
	child := spawnEscalationFixture(t, s)
	if got := child.moduleGuidance("internal/core/kernel.go", 0, "test"); !strings.Contains(got, "CORE-GUIDE") {
		t.Fatalf("a spawned subagent's executor has no module docs: %q", got)
	}
}

// The ledger's append line names the modules a focus view carries.
func TestModuleGuidanceNote(t *testing.T) {
	e := &Executor{}
	e.SetProjectModuleDocs(moduleDocs())
	view := "outline...\n\n" + e.moduleGuidance("internal/tools/research/fetch.go", 0, guidanceInFocusView)
	if got, want := moduleGuidanceNote(view), ", module guidance: internal/tools/research, internal/tools"; got != want {
		t.Fatalf("note = %q, want %q", got, want)
	}
	if got := moduleGuidanceNote("outline only"); got != "" {
		t.Fatalf("a view with no guidance noted %q", got)
	}
}
