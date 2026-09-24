package campaign

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/world"
)

const widgetSource = `package widget

// Widget renders a name.
type Widget struct{ name string }

// NewWidget builds a widget.
func NewWidget(name string) *Widget { return &Widget{name: name} }

// Render returns the widget's markup.
func (w *Widget) Render() string {
	return "<b>" + w.name + "</b>"
}
`

const utilSource = `package widget

// escape makes a name safe for markup.
func escape(s string) string {
	return s
}
`

// preloadWorkspace is a module with one package, pkg/widget, of two files.
func preloadWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	writeEvidenceDoc(t, ws, "go.mod", "module example.com/fx\n\ngo 1.22\n")
	writeEvidenceDoc(t, ws, "pkg/widget/widget.go", widgetSource)
	writeEvidenceDoc(t, ws, "pkg/widget/util.go", utilSource)
	return ws
}

func preloadCampaign(tasks ...Task) *Campaign {
	for i := range tasks {
		tasks[i].PhaseID = "/phase_0"
		tasks[i].Order = i
	}
	return &Campaign{ID: "/campaign_preload", Phases: []Phase{{ID: "/phase_0", Order: 0, Tasks: tasks}}}
}

func TestTaskPreload_TheBriefsNamedElementsAndTargetArriveWithRevisions(t *testing.T) {
	ws := preloadWorkspace(t)
	c := preloadCampaign(Task{
		ID: "/task_fix", Type: TaskTypeFileModify, Status: TaskPending,
		Description: "Make `Widget.Render` escape the name; NewWidget must reject an empty name",
		WriteSet:    []string{filepath.ToSlash(filepath.Join(ws, "pkg", "widget", "widget.go"))},
	})
	o := evidenceOrchestrator(t, ws, c, nil)
	section, err := o.taskContextSection(context.Background(), &c.Phases[0].Tasks[0])
	if err != nil {
		t.Fatal(err)
	}
	forms, err := o.askTaskPreload("/task_fix")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"pkg/widget/widget.go":     preloadOutline, // the task's own target
		"pkg/widget.Widget.Render": preloadElement, // named in backticks
		"pkg/widget.NewWidget":     preloadElement, // named as a MixedCase word
	}
	for target, form := range want {
		if forms[target] != form {
			t.Errorf("task_preload(%s) = %q, want %q (all: %v)", target, forms[target], form, forms)
		}
	}
	if len(forms) != len(want) {
		t.Errorf("derived %d preload rows, want %d: %v", len(forms), len(want), forms)
	}
	// The named elements' source is in the brief: no tool call needed to see them.
	for _, line := range []string{`return "<b>" + w.name + "</b>"`, "func NewWidget(name string) *Widget"} {
		if !strings.Contains(section, line) {
			t.Errorf("the brief lacks %q:\n%s", line, section)
		}
	}
	// Every preloaded element carries the index's revision, the edit verbs'
	// precondition.
	syms, _, err := world.SharedStructureIndex(ws).Resolve(context.Background(), "pkg/widget.Widget.Render")
	if err != nil || len(syms) != 1 {
		t.Fatalf("resolve Render: %v %v", syms, err)
	}
	if rev := syms[0].Revision; rev == "" || !strings.Contains(section, "pkg/widget.Widget.Render  method  pkg/widget/widget.go:9-12  rev "+rev) {
		t.Errorf("Render is not keyed by its element revision %q:\n%s", rev, section)
	}
	if n := len(regexp.MustCompile(`(?m)^pkg/widget/widget\.go:\d+-\d+  \S+  \S+  rev \S+`).FindAllString(section, -1)); n != 3 {
		t.Errorf("want the target's 3 declarations as outline rows with revisions, got %d:\n%s", n, section)
	}

	// An edit elsewhere in the file leaves Render's revision alone; an edit
	// to Render moves it, and the next brief carries the new one.
	edited := strings.Replace(widgetSource, `"<b>" + w.name + "</b>"`, `"<i>" + w.name + "</i>"`, 1)
	writeEvidenceDoc(t, ws, "pkg/widget/widget.go", edited)
	again, err := o.taskContextSection(context.Background(), &c.Phases[0].Tasks[0])
	if err != nil {
		t.Fatal(err)
	}
	syms2, _, _ := world.SharedStructureIndex(ws).Resolve(context.Background(), "pkg/widget.Widget.Render")
	if len(syms2) != 1 || syms2[0].Revision == syms[0].Revision || !strings.Contains(again, "rev "+syms2[0].Revision) || strings.Contains(again, `"<b>"`) {
		t.Errorf("the brief after an edit to Render must carry its new source and revision:\n%s", again)
	}
}

// Over the configured sizes, a file is named with its count and an element
// with its outline row: never a cut body.
func TestTaskPreload_OverTheSizesIsACountOrASignature(t *testing.T) {
	ws := preloadWorkspace(t)
	c := preloadCampaign(Task{
		ID: "/task_fix", Type: TaskTypeFileModify, Status: TaskPending,
		Description: "Make `Widget.Render` escape the name",
		WriteSet:    []string{filepath.ToSlash(filepath.Join(ws, "pkg", "widget", "widget.go"))},
	})
	o := evidenceOrchestrator(t, ws, c, func(cc *config.CampaignConfig) {
		cc.PreloadOutlineMaxRows = 2
		cc.PreloadElementMaxLines = 3
	})
	section, err := o.taskContextSection(context.Background(), &c.Phases[0].Tasks[0])
	if err != nil {
		t.Fatal(err)
	}
	forms, err := o.askTaskPreload("/task_fix")
	if err != nil {
		t.Fatal(err)
	}
	if forms["pkg/widget/widget.go"] != preloadCount || forms["pkg/widget.Widget.Render"] != preloadSignature {
		t.Fatalf("want /count for a 3-declaration file over 2 rows and /signature for a 4-line element over 3 lines, got %v", forms)
	}
	if strings.Contains(section, `"<b>"`) || !strings.Contains(section, "pkg/widget/widget.go: 3 declarations") {
		t.Fatalf("over-size targets must be named, not printed:\n%s", section)
	}
}

// The code a task's dependency wrote is preloaded even when its brief does not
// name it; a directory the brief names is preloaded as the package outline.
func TestTaskPreload_ADependencysCodeAndANamedPackageArrive(t *testing.T) {
	ws := preloadWorkspace(t)
	c := preloadCampaign(
		Task{ID: "/task_util", Type: TaskTypeFileCreate, Status: TaskCompleted, Description: "Create the escape helper",
			Artifacts: []TaskArtifact{{Type: "/source_file", Path: "pkg/widget/util.go"}}},
		Task{ID: "/task_audit", Type: TaskTypeResearch, Status: TaskPending, DependsOn: []string{"/task_util"},
			Description: "Audit how the helper is used"},
		Task{ID: "/task_survey", Type: TaskTypeResearch, Status: TaskPending, Description: "Survey pkg/widget for markup escaping"},
	)
	o := evidenceOrchestrator(t, ws, c, nil)
	if _, err := o.taskContextSection(context.Background(), &c.Phases[0].Tasks[1]); err != nil {
		t.Fatal(err)
	}
	forms, err := o.askTaskPreload("/task_audit")
	if err != nil {
		t.Fatal(err)
	}
	if forms["pkg/widget/util.go"] != preloadOutline || len(forms) != 1 {
		t.Errorf("the dependency's code file should be outlined for /task_audit, got %v", forms)
	}
	section, err := o.taskContextSection(context.Background(), &c.Phases[0].Tasks[2])
	if err != nil {
		t.Fatal(err)
	}
	forms, err = o.askTaskPreload("/task_survey")
	if err != nil {
		t.Fatal(err)
	}
	if forms["pkg/widget"] != preloadOutline || len(forms) != 1 {
		t.Errorf("the named package should be outlined for /task_survey, got %v", forms)
	}
	for _, ref := range []string{"pkg/widget.escape", "pkg/widget.Widget.Render", "pkg/widget.NewWidget"} {
		if !strings.Contains(section, ref) {
			t.Errorf("the package outline lacks %s:\n%s", ref, section)
		}
	}
}

// A short name two packages declare is an element only where the brief's own
// files decide which; otherwise it is a search, and nothing is preloaded.
func TestTaskPreload_AnAmbiguousNameResolvesInTheBriefsFilesOnly(t *testing.T) {
	ws := preloadWorkspace(t)
	writeEvidenceDoc(t, ws, "pkg/other/other.go", "package other\n\n// NewWidget is another constructor.\nfunc NewWidget() int { return 1 }\n")
	c := preloadCampaign(
		Task{ID: "/task_scoped", Type: TaskTypeResearch, Status: TaskPending, Description: "Check NewWidget in pkg/widget/widget.go"},
		Task{ID: "/task_loose", Type: TaskTypeResearch, Status: TaskPending, Description: "Check NewWidget"},
	)
	o := evidenceOrchestrator(t, ws, c, nil)
	for i, want := range []map[string]string{
		{"pkg/widget/widget.go": preloadOutline, "pkg/widget.NewWidget": preloadElement},
		{},
	} {
		task := &c.Phases[0].Tasks[i]
		if _, err := o.taskContextSection(context.Background(), task); err != nil {
			t.Fatal(err)
		}
		forms, err := o.askTaskPreload(task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(forms) != len(want) {
			t.Errorf("%s: task_preload = %v, want %v", task.ID, forms, want)
		}
		for target, form := range want {
			if forms[target] != form {
				t.Errorf("%s: task_preload(%s) = %q, want %q", task.ID, target, forms[target], form)
			}
		}
	}
}

func TestBriefIdentifiers(t *testing.T) {
	got := briefIdentifiers("Fix `envBool` and features.IsProvenanceEnabled in internal/features/features.go; " +
		"Current Target ADR GAP-FEAT-01 task_evidence agents.md StructureIndex.Refresh escape, then NewWidget and `evaluate`")
	want := []string{"envBool", "features.IsProvenanceEnabled", "task_evidence", "StructureIndex.Refresh", "NewWidget", "evaluate"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("briefIdentifiers = %v, want %v", got, want)
	}
}

func TestBriefPaths(t *testing.T) {
	ws := preloadWorkspace(t)
	if err := os.MkdirAll(filepath.Join(ws, ".nerd", "campaigns", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeEvidenceDoc(t, ws, ".nerd/campaigns/x/a.md", "a")
	got := briefPaths("See pkg/widget/widget.go:9-12, the pkg/widget package, .nerd/campaigns/x/a.md. and pkg/missing.go or ../outside.go", ws)
	want := []string{"pkg/widget/widget.go", "pkg/widget", ".nerd/campaigns/x/a.md"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("briefPaths = %v, want %v", got, want)
	}
}
