package codedom_test

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

const roundTripSource = `package calc

func Add(a, b int) int {
	return a + b
}

func Sub(a, b int) int {
	return a - b
}
`

type elementSnapshot struct {
	ref      string
	revision string
	text     string
}

// readElementSnapshot goes through the registered tool, then retains the
// address and revision exactly as the model receives them.
func readElementSnapshot(t *testing.T, reg *tools.Registry, path, ref string) elementSnapshot {
	t.Helper()
	out := run(t, reg, "get_element", map[string]any{"path": path, "ref": ref})
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 5 && strings.HasPrefix(fields[2], path+":") && fields[3] == "rev" {
			return elementSnapshot{ref: fields[0], revision: fields[4], text: out}
		}
	}
	t.Fatalf("get_element output has no address/revision header for %s:\n%s", path, out)
	return elementSnapshot{}
}

func TestRegistryRoundTrip_ElementReadEditReadAndStaleRevision(t *testing.T) {
	reg, root := structWorkspace(t, map[string]string{
		"go.mod":       "module fixture\n\ngo 1.24\n",
		"calc/calc.go": roundTripSource,
	})
	path := "calc/calc.go"

	before := readElementSnapshot(t, reg, path, "Add")
	if before.ref != "calc.Add" || !strings.Contains(before.text, "return a + b") {
		t.Fatalf("initial read returned unexpected element: %+v", before)
	}

	editOut := run(t, reg, "edit_element", map[string]any{
		"path": path, "ref": before.ref, "revision": before.revision,
		"old": "return a + b", "new": "return a + b + 1",
	})
	if !strings.Contains(editOut, "return a + b + 1") {
		t.Fatalf("edit answer does not carry the changed source:\n%s", editOut)
	}

	after := readElementSnapshot(t, reg, path, before.ref)
	if !strings.Contains(after.text, "return a + b + 1") {
		t.Fatalf("read after edit does not show the committed source:\n%s", after.text)
	}
	if after.revision == before.revision {
		t.Fatalf("revision did not advance after the element changed: %s", after.revision)
	}
	if !strings.Contains(editOut, "rev "+after.revision) {
		t.Fatalf("edit answer revision does not match the subsequent read (%s):\n%s", after.revision, editOut)
	}

	committed := readRel(t, root, path)
	_, err := reg.Execute(context.Background(), "edit_element", map[string]any{
		"path": path, "ref": before.ref, "revision": before.revision,
		"old": "return a + b + 1", "new": "return a + b + 2",
	})
	if err == nil {
		t.Fatal("an old revision must be refused after an element edit")
	}
	for _, want := range []string{"precondition failed", "changed since it was read", "at rev " + after.revision, "not " + before.revision, "It is now:", "return a + b + 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("stale revision refusal must include %q, got: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "malformed") {
		t.Errorf("a genuine stale revision is not malformed: %v", err)
	}
	if got := readRel(t, root, path); got != committed {
		t.Fatal("a refused stale-revision edit changed the file")
	}
}

func TestRegistryRoundTrip_LineEditInvalidatesElementRevision(t *testing.T) {
	reg, root := structWorkspace(t, map[string]string{
		"go.mod":       "module fixture\n\ngo 1.24\n",
		"calc/calc.go": roundTripSource,
	})
	path := "calc/calc.go"

	before := readElementSnapshot(t, reg, path, "Add")
	if _, err := reg.Execute(context.Background(), "edit_lines", map[string]any{
		"path": path, "start_line": 4, "end_line": 4,
		"new_content": "\treturn a + b + 1",
	}); err != nil {
		t.Fatalf("edit_lines: %v", err)
	}

	after := readElementSnapshot(t, reg, path, before.ref)
	if !strings.Contains(after.text, "return a + b + 1") {
		t.Fatalf("read after line edit does not show the committed source:\n%s", after.text)
	}
	if after.revision == before.revision {
		t.Fatalf("revision did not advance after a line edit changed the element: %s", after.revision)
	}

	committed := readRel(t, root, path)
	_, err := reg.Execute(context.Background(), "edit_element", map[string]any{
		"path": path, "ref": before.ref, "revision": before.revision,
		"old": "return a + b + 1", "new": "return a + b + 2",
	})
	if err == nil || !strings.Contains(err.Error(), "precondition failed") {
		t.Fatalf("an old revision must be refused after a line edit, got %v", err)
	}
	if got := readRel(t, root, path); got != committed {
		t.Fatal("a refused stale-revision edit changed the file")
	}
}
