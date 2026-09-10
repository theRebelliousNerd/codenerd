package codedom

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/observation/precondition"
	"codenerd/internal/tools"
)

// preconditionFixtureSeq keeps each fixture's content unique across runs.
//
// Preconditions are content-addressed, so a fixture with identical bytes yields
// the identical handle on a second pass in the same process. Without this,
// `go test -count=2` could pass on a handle minted by the first pass rather
// than by the code under test.
var preconditionFixtureSeq atomic.Int64

const lineFixture = `package widget

func Encode(v int) int {
	return v + 1
}

func caller() int {
	return Encode(1)
}
`

// seedLineFixture writes the fixture and mints a precondition over the lines a
// caller is about to edit, exactly as read_file would have.
func seedLineFixture(t *testing.T, start, end int) (root, path, handle string) {
	t.Helper()
	root = t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	name := fmt.Sprintf("widget_%d.go", preconditionFixtureSeq.Add(1))
	full := filepath.Join(root, name)
	if err := os.WriteFile(full, []byte(lineFixture), 0o600); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}
	// Path is the resolved absolute, exactly as read_file mints it: the identity
	// a precondition is keyed on cannot be a workspace-relative name, because
	// the producer and the consumer resolve against roots that are allowed to
	// differ.
	return root, full, precondition.Shared().Mint(precondition.Read{
		Path:    full,
		Display: name,
		Content: lineFixture,
		Start:   start,
		End:     end,
	})
}

// TestEditLines_WhenTextMovedAboveTheRegion_ShouldRefuse is the defect this
// mechanism was built for, driven through the live verb.
//
// lineShiftNotice on these same verbs warns a caller after ITS OWN mutation
// moved the file. It cannot see a mutation made by anything else — another
// agent, a formatter, a build step — and edit_lines is addressed by nothing but
// line numbers, so that is exactly the mutation that turns a correct call into
// a destructive one. Observed live before any of this existed: two edits from
// stale offsets produced duplicate SpawnSpecialist and SetProjectDoc
// declarations in one file.
func TestEditLines_WhenTextMovedAboveTheRegion_ShouldRefuse(t *testing.T) {
	t.Parallel()
	root, path, handle := seedLineFixture(t, 3, 5)

	// Three lines inserted above the region by somebody else. Every byte the
	// caller read still exists; only its address moved.
	moved := strings.Replace(lineFixture, "package widget\n", "package widget\n\n// added\n// elsewhere\n", 1)
	if err := os.WriteFile(path, []byte(moved), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	_, err := executeEditLines(wsCtxFor(t, filepath.Join(root, "x")), map[string]any{
		"path":           path,
		"start_line":     3,
		"end_line":       5,
		"new_content":    "func Encode(v int) int {\n\treturn v + 2\n}",
		precondition.Arg: handle,
	})
	if err == nil {
		t.Fatal("edit_lines wrote at coordinates that had moved under it; the replacement would have landed three lines above the function it was meant for")
	}
	if !strings.Contains(err.Error(), "+3") {
		t.Errorf("the refusal must name the shift so the caller can retry rather than restart:\n%v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(after) != moved {
		t.Errorf("the refused edit still modified the file:\n%s", after)
	}
}

func TestEditLines_WhenNothingMoved_ShouldApplyTheEdit(t *testing.T) {
	t.Parallel()
	root, path, handle := seedLineFixture(t, 3, 5)

	out, err := executeEditLines(wsCtxFor(t, filepath.Join(root, "x")), map[string]any{
		"path":           path,
		"start_line":     3,
		"end_line":       5,
		"new_content":    "func Encode(v int) int {\n\treturn v + 2\n}",
		precondition.Arg: handle,
	})
	if err != nil {
		t.Fatalf("an edit resting on an unchanged read must proceed: %v", err)
	}
	if !strings.Contains(out, "Replaced lines 3-5") {
		t.Errorf("edit did not report what it did:\n%s", out)
	}

	after, _ := os.ReadFile(path)
	if !strings.Contains(string(after), "return v + 2") {
		t.Errorf("edit did not land:\n%s", after)
	}
}

func TestInsertLines_WhenThePreconditionIsStale_ShouldRefuse(t *testing.T) {
	t.Parallel()
	root, path, handle := seedLineFixture(t, 3, 5)

	changed := strings.Replace(lineFixture, "return v + 1", "return v * 9", 1)
	if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	if _, err := executeInsertLines(wsCtxFor(t, filepath.Join(root, "x")), map[string]any{
		"path":           path,
		"after_line":     4,
		"content":        "\t// inserted",
		precondition.Arg: handle,
	}); err == nil {
		t.Fatal("insert_lines placed content by line number into a file whose lines had changed under it")
	}
}

func TestDeleteLines_WhenThePreconditionIsStale_ShouldRefuse(t *testing.T) {
	t.Parallel()
	root, path, handle := seedLineFixture(t, 3, 5)

	changed := strings.Replace(lineFixture, "return v + 1", "return v * 9", 1)
	if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	if _, err := executeDeleteLines(wsCtxFor(t, filepath.Join(root, "x")), map[string]any{
		"path":           path,
		"start_line":     3,
		"end_line":       5,
		precondition.Arg: handle,
	}); err == nil {
		t.Fatal("delete_lines removed lines by number from a file whose lines had changed under it; a delete cannot be undone by a retry")
	}
}

func TestLineVerbs_WhenNoPreconditionIsGiven_ShouldStillWork(t *testing.T) {
	t.Parallel()
	root, path, _ := seedLineFixture(t, 3, 5)

	// The argument is optional by design: an agent that never read the file has
	// nothing to check, and making it required would break every existing
	// caller for no gain in safety.
	if _, err := executeEditLines(wsCtxFor(t, filepath.Join(root, "x")), map[string]any{
		"path":        path,
		"start_line":  3,
		"end_line":    5,
		"new_content": "func Encode(v int) int {\n\treturn v + 3\n}",
	}); err != nil {
		t.Fatalf("an edit with no precondition must proceed: %v", err)
	}
}

// TestRegistry_LineVerbsAdvertiseThePrecondition guards the half of the wiring
// a behaviour test cannot see. The check is only ever reached if the model is
// told the argument exists, and the model is only told through the schema the
// registry hands out.
func TestRegistry_LineVerbsAdvertiseThePrecondition(t *testing.T) {
	t.Parallel()

	registry := tools.NewRegistry()
	if err := RegisterAll(registry); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	for _, name := range []string{"edit_lines", "insert_lines", "delete_lines"} {
		tool := registry.Get(name)
		if tool == nil {
			t.Fatalf("%s is not registered, so nothing the agent does can reach it", name)
		}
		prop, ok := tool.Schema.Properties[precondition.Arg]
		if !ok {
			t.Errorf("%s does not advertise %q; the model is never told it may pass one, so the check never runs and the capability is dead",
				name, precondition.Arg)
			continue
		}
		if !strings.Contains(prop.Description, "read_file") {
			t.Errorf("%s's precondition description never says where a handle comes from:\n%s", name, prop.Description)
		}
	}
}
